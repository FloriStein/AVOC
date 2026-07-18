# WebRTC im AVOC-System — Vollständige technische Dokumentation

Stand: 2026-06-18 | Referenz-ADRs: ADR-014, ADR-020 | Sprint 10

---

## 1. WebRTC — Grundprinzip

WebRTC (Web Real-Time Communication) ist ein offener W3C/IETF-Standard für Echtzeit-Medienübertragung direkt im Browser ohne Plugin. Technisch besteht es aus drei unabhängigen Schichten:

```
┌──────────────────────────────────────────────┐
│  Signaling (nicht standardisiert)            │  ← Wie tauschen Peers SDP aus?
│  In AVOC: WHIP/WHEP (HTTP POST)              │
├──────────────────────────────────────────────┤
│  ICE (Interactive Connectivity Establishment)│  ← Wie finden sich Peers im Netz?
│  STUN / TURN                                 │  ← coturn im AVOC-Stack
├──────────────────────────────────────────────┤
│  DTLS-SRTP (Medien-Transport)                │  ← Verschlüsselter RTP-Kanal
│  SDP (Codec-Aushandlung)                     │
└──────────────────────────────────────────────┘
```

WebRTC trennt Signaling und Media bewusst: Das Signaling-Protokoll ist nicht standardisiert — jede Anwendung wählt selbst (WebSocket, HTTP, SIP, etc.). Nur das ICE/DTLS/RTP-Protokoll ist normiert.

---

## 2. ICE — Interactive Connectivity Establishment

ICE löst das NAT-Traversal-Problem: Zwei Peers hinter unterschiedlichen NATs finden einen gemeinsamen Kommunikationspfad.

### 2.1 ICE Candidate-Typen

Ein **ICE Candidate** ist eine Netzwerkadresse, über die ein Peer erreichbar sein könnte.

| Typ | Beschreibung | Beispiel |
|-----|-------------|---------|
| `host` | Direkte Netzwerk-Interface-Adresse | `192.168.1.10:54321` |
| `srflx` (server-reflexive) | Öffentliche IP, vom STUN-Server gespiegelt | `18.196.24.10:54321` |
| `relay` | Adresse auf dem TURN-Server (Relay) | `18.196.24.10:49200` |

### 2.2 STUN — Session Traversal Utilities for NAT

STUN ist ein simples Request/Response-Protokoll. Der Client fragt den STUN-Server: „Wie sehe ich von außen aus?"

```
Browser                    coturn (STUN)
   │── STUN Binding Request ──▶│
   │◀── Response: 18.196.24.10:54321 ──│
```

Der Browser kennt jetzt seine öffentliche IP+Port → `srflx` Candidate. Das reicht für **Full-Cone NAT** (Heimrouter), nicht für symmetrisches NAT (typisch in 5G/LTE-Netzen, CGNAT).

### 2.3 TURN — Traversal Using Relays around NAT

TURN ist der Fallback wenn P2P scheitert. Der TURN-Server wird zum **Relay-Punkt**: Beide Peers senden Medien nicht direkt aneinander, sondern über den TURN-Server.

```
Browser (5G/CGNAT)         coturn (TURN Relay)         MediaMTX
       │── ALLOCATE ──────────▶│                           │
       │◀── RelayAddress ──────│ (18.196.24.10:49200)     │
       │── Media (UDP) ────────▶│── Media (UDP) ──────────▶│
       │◀─────────────────────────────── Media (UDP) ──────│
```

TURN erfordert Credentials (Long-Term Credential Mechanism, `lt-cred-mech`). In AVOC: `TURN_USER` + `TURN_PASSWORD` aus AWS SSM Parameter Store.

### 2.4 ICE Gathering Phase

Bevor ein Peer einen SDP-Offer sendet, sammelt er alle seine Candidates:

```
1. host-Candidates:  alle lokalen Netzwerk-Interfaces durchsuchen
2. srflx-Candidates: STUN-Anfrage an coturn → öffentliche IP erfahren
3. relay-Candidates: TURN-Allocation bei coturn anfragen → Relay-Adresse erhalten
```

In AVOC wartet der Browser explizit bis `iceGatheringState === 'complete'` bevor der WHIP/WHEP-POST abgesendet wird (`frontend/src/hooks/useWebRTC.ts`, Zeile 99–109):

```typescript
await new Promise<void>(resolve => {
  if (pc.iceGatheringState === 'complete') { resolve(); return }
  const tid = setTimeout(resolve, 2000)   // 2s Timeout als Safety
  pc.addEventListener('icegatheringstatechange', function handler() {
    if (pc.iceGatheringState === 'complete') {
      clearTimeout(tid)
      pc.removeEventListener('icegatheringstatechange', handler)
      resolve()
    }
  })
})
```

Das ist **Vanilla ICE** (alle Candidates im initialen SDP-Offer) — MediaMTX erwartet keine nachträglichen Trickle-ICE-Candidates.

### 2.5 ICE Connectivity Checks & Candidate Pairs

Wenn beide Seiten ihre Candidate-Listen ausgetauscht haben (via SDP), startet ICE Connectivity Checks: STUN Binding Requests werden auf jeder möglichen Candidate-Paar-Kombination ausprobiert.

```
Browser Candidates × MediaMTX Candidates = Candidate Pairs

Beispiel:
  browser-srflx(18.196.24.10:54321) ↔ mediamtx-host(18.196.24.10:8189)  → SUCCESS
  browser-relay(18.196.24.10:49200) ↔ mediamtx-host(18.196.24.10:8189)  → SUCCESS
  browser-host(192.168.1.10:xxxxx)  ↔ mediamtx-host(18.196.24.10:8189)  → FAIL (NAT)
```

ICE nominiert das **beste erfolgreiche Pair** (niedrigste Latenz, Präferenz: host > srflx > relay). In AVOC auf WiFi ist das typischerweise `browser-srflx ↔ mediamtx-host`, auf 5G (CGNAT) `browser-relay ↔ mediamtx-host`.

---

## 3. SDP — Session Description Protocol

SDP beschreibt eine Mediasitzung: Codecs, Transport-Protokoll, ICE Candidates, DTLS-Fingerprint. Es ist kein Binärformat, sondern ein Textformat.

Relevanter Ausschnitt eines WHEP-Offers aus `useWebRTC.ts`:

```
v=0
o=- 123456 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0
m=video 9 UDP/TLS/RTP/SAVPF 96
a=rtpmap:96 H264/90000
a=setup:actpass         ← RFC 8842: der Offerer MUSS actpass senden (Sprint 32/WEBRTC-10 — kein
                           erzwungenes "active" mehr, siehe unten)
a=ice-ufrag:abc123
a=ice-pwd:xyz789
a=candidate:1 1 udp 2122260223 192.168.1.10 54321 typ host
a=candidate:2 1 udp 1686052607 18.196.24.10 54321 typ srflx raddr 192.168.1.10 rport 54321
a=candidate:3 1 udp 33562623   18.196.24.10 49200 typ relay raddr 18.196.24.10 rport 3478
```

**Update Sprint 32 (WEBRTC-10):** Der frühere `actpass→active`-SDP-Zwang in `useWebRTC.ts`/
`useWHIPSender.ts` (Fix für einen Pion-v1.19.0-DTLS-Client-Bug, Sprint 10) wurde entfernt.
`createOffer()`/`setLocalDescription()` läuft jetzt unverändert (Standard-`actpass`-Offer,
RFC-8842-konform). Grund: `mediamtx:latest` baut inzwischen gegen `pion/webrtc v4.2.x` — eine
komplett andere Codebasis als das damalige `v1.19.0` — und der Zwang selbst war der aktuelle
Fehler geworden (modernes Chromium lehnt einen Offer mit `a=setup:active` als Spec-Verstoß ab,
"Offerer must use actpass"). Lokal gegen den echten `mediamtx:latest`-Container verifiziert
(Sprint 32, isolierter Pion-WHIP-Client mit `pion/webrtc v4.2.12`, passend zur mediamtx-Version):
ein Standard-`actpass`-Offer wird von MediaMTX mit `a=setup:active` beantwortet (RFC-8842-
Empfehlung für den Answerer) und der DTLS/ICE-Handshake erreicht zuverlässig
`PeerConnectionStateConnected` — genau die Rolle, die 2026-06-18 (Sprint 10) als fehlerhaft
dokumentiert war, funktioniert mit der aktuellen MediaMTX-Version. Details, inkl. der zuerst
irreführenden Ergebnisse durch die ungewöhnliche Multi-Interface-Netzwerktopologie der
Verifikationsumgebung: `tasks/current-sprint.md` Sprint 32.

---

## 4. WHIP & WHEP — HTTP-basiertes Signaling

WHIP (WebRTC-HTTP Ingestion Protocol, RFC 9725) und WHEP (WebRTC-HTTP Egress Protocol, RFC 9728) definieren HTTP als Signaling-Kanal — kein WebSocket nötig.

### WHIP (Publish — Browser/Fahrzeug → MediaMTX)

```
POST /vehicle-001/whip
Authorization: Bearer <WHIP_STREAM_KEY>
Content-Type: application/sdp

[SDP Offer mit allen ICE Candidates]

← 201 Created
   Content-Type: application/sdp
   Location: /vehicle-001/whip/abc123   ← Ressource-URL für DELETE (Session beenden)

   [SDP Answer von MediaMTX]
```

### WHEP (Subscribe — Browser → MediaMTX)

```
POST /whep/vehicle-001/whep     (via nginx: /whep/ → mediamtx:8889)
Authorization: Bearer <JWT>
Content-Type: application/sdp

[SDP Offer — nur recvonly Video]

← 200 OK
   Content-Type: application/sdp

   [SDP Answer von MediaMTX]
```

---

## 5. Systemarchitektur im AVOC-Stack

```
                    ┌──────────────────────────────────────────────────┐
                    │           Operator Browser                        │
                    │                                                   │
                    │  useWHIPSender.ts   useWebRTC.ts                  │
                    │  (Senden)           (Empfangen)                   │
                    └──────────┬──────────────┬────────────────────────┘
                               │ WHIP POST    │ WHEP POST
                               │ (SDP+ICE)   │ (SDP+ICE)
                               ▼              ▼
┌────────────────┐   ┌─────────────────────────────────────────────────┐
│ Fahrzeug (5G)  │──▶│              nginx (Port 443/80)                 │
│ WHIP Publisher │   │  /whip/ → mediamtx:8889   /api/ → control:8080  │
└────────────────┘   │  /whep/ → mediamtx:8889                         │
                     └──────────────┬──────────────┬────────────────────┘
                                    │              │ GET /api/ice-config
                                    │              │ POST /internal/media/auth
                                    ▼              ▼
                     ┌──────────────────┐   ┌──────────────────────────┐
                     │    MediaMTX      │   │    Control Server        │
                     │  (Port 8889/TCP) │──▶│  (Port 8080/TCP)         │
                     │  (Port 8189/UDP) │   │  JWT-Validierung         │
                     │  WHIP/WHEP       │   │  SAFE_MODE → KickVehicle │
                     │  Auth-Hook       │   │  GET /ice-config          │
                     └────────┬─────────┘   └──────────────────────────┘
                              │ UDP RTP
                              │ (ICE ausgehandelt)
                     ┌────────▼─────────┐
                     │    coturn        │
                     │  (Port 3478)     │
                     │  STUN + TURN     │
                     │  Relay 49152–    │
                     │  65535/UDP       │
                     └──────────────────┘

                     ┌──────────────────────────────────────────────────┐
                     │  Pion WebRTC SFU (Port 8084)                     │
                     │  Session-Event-Consumer (ADR-015)                │
                     │  Kein Media-Routing mehr (seit ADR-020)          │
                     └──────────────────────────────────────────────────┘
```

---

## 6. Komponenten im Detail

### 6.1 MediaMTX — WHIP/WHEP Router

**Datei:** `infrastructure/mediamtx/mediamtx.yml`

MediaMTX ist der zentrale Media-Router. Er spricht nativ WHIP und WHEP und delegiert alle Auth-Entscheidungen via Hook an den Control Server.

```yaml
webrtcAddress: :8889            # HTTP Signaling — WHIP/WHEP POST
webrtcLocalUDPAddress: :8189    # ICE UDP Mux — separater Port
webrtcIPsFromInterfaces: false  # Docker-interne IPs (172.x) unterdrücken
webrtcAdditionalHosts:          # Einziger Host-Candidate: EC2 Elastic IP
  - "$TURN_EXTERNAL_IP"
webrtcHandshakeTimeout: 60s     # TURN-Relay braucht mehr Zeit als direkte Verbindung

authMethod: http
authHTTPAddress: http://control-server:8080/internal/media/auth

paths:
  "~^vehicle-.*":               # Alle vehicle-* Pfade erlaubt
```

**Dev vs. Prod:**

In der Dev-Konfiguration (`infrastructure/mediamtx/mediamtx.dev.yml`) läuft MediaMTX im `network_mode: host` und setzt `webrtcIPsFromInterfaces: true` — kein AWS-NAT-Problem lokal, alle Interface-Adressen sind echte LAN-IPs.

**ICE-Strategie (die entscheidende Designentscheidung aus Sprint 10):**

MediaMTX gathert **keine** eigenen STUN/TURN-Candidates. Es annonciert nur einen einzigen Host-Candidate: die EC2 Elastic IP auf UDP-Port 8189. Der **Browser** gathert seinen eigenen Candidate-Satz (host + srflx + relay) über `/api/ice-config`.

Funktionierende ICE-Pairs:
- `browser-srflx ↔ mediamtx-host` — WiFi/DSL (kein CGNAT)
- `browser-relay ↔ mediamtx-host` — 5G/LTE (symmetrisches NAT, via coturn TURN-Relay)

### 6.2 coturn — STUN/TURN Server

**Datei:** `infrastructure/coturn/turnserver.conf`

```ini
listening-port=3478
lt-cred-mech               # Long-Term Credential Mechanism
fingerprint                # Pflicht für WebRTC-Kompatibilität
min-port=49152
max-port=65535             # Relay-Port-Range (muss in AWS Security Group offen sein)

external-ip=${TURN_EXTERNAL_IP}   # Prod: EC2 Elastic IP
realm=${TURN_REALM}
user=${TURN_USER}:${TURN_PASSWORD}
```

**Das AWS NAT-Problem (Sprint 10 Root Cause #5):**

AWS EC2 hat kein Interface mit der öffentlichen Elastic IP — die Instanz sieht nur ihre private IP (`10.0.33.191`). coturn muss Relay-Sockets an die **private** IP binden, aber nach außen die **öffentliche** IP ankündigen.

Lösung in `docker-compose.prod.yml`:

```yaml
stun-turn:
  network_mode: host   # kein Bridge-Networking — direkt an Host-Interfaces gebunden
  command: >
    --relay-ip=${TURN_PRIVATE_IP}                        # Socket bindet an 10.0.33.191
    --external-ip=${TURN_EXTERNAL_IP}/${TURN_PRIVATE_IP} # Advertised: 18.196.24.10/10.0.33.191
    --min-port=49152
    --max-port=65535
```

`network_mode: host` ist nötig weil `49152-65535:49152-65535/udp` (16.384 Port-Mappings) im Bridge-Netz nicht praktikabel ist.

`TURN_PRIVATE_IP` wird zur Laufzeit via EC2 Instance Metadata Service (IMDSv2) gesetzt:

```bash
# scripts/deploy.sh
_IMDS_TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
export TURN_PRIVATE_IP=$(curl -sf -H "X-aws-ec2-metadata-token: ${_IMDS_TOKEN}" \
  http://169.254.169.254/latest/meta-data/local-ipv4)
```

Amazon Linux 2023 erzwingt IMDSv2 — ohne Token-Header liefert IMDS eine leere Response und `TURN_PRIVATE_IP` wäre leer (Sprint 10 Bug B1).

### 6.3 Control Server — WebRTC-Endpunkte

**Datei:** `cmd/control-server/main.go`

#### GET /ice-config

```go
mux.HandleFunc("GET /ice-config", func(w http.ResponseWriter, _ *http.Request) {
    type iceServer struct {
        URLs       []string `json:"urls"`
        Username   string   `json:"username,omitempty"`
        Credential string   `json:"credential,omitempty"`
    }
    host := turnExternalIP
    servers := []iceServer{
        {URLs: []string{"stun:" + host + ":" + turnPort}},
        {URLs: []string{"turn:" + host + ":" + turnPort},
            Username: turnUser, Credential: turnPassword},
        {URLs: []string{"turn:" + host + ":" + turnPort + "?transport=tcp"},
            Username: turnUser, Credential: turnPassword},
    }
    json.NewEncoder(w).Encode(map[string]any{"iceServers": servers})
})
```

Kein Auth erforderlich — TURN-Credentials sind per Design sichtbar: Sie werden ohnehin in den WebRTC ICE-Kandidaten über SDP ausgetauscht. Der API-Endpunkt (statt Build-Time-Env) ermöglicht Credential-Rotation ohne Frontend-Rebuild.

nginx routet: Browser → `GET /api/ice-config` → (Strip `/api/`) → Control Server `GET /ice-config`.

#### POST /internal/media/auth

MediaMTX ruft diesen Hook für **jeden** eingehenden WHIP/WHEP-Request auf. Der Control Server ist damit Single Point of Control für alle Video-Auth-Entscheidungen:

```go
mux.HandleFunc("POST /internal/media/auth", func(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Action string `json:"action"`
        Path   string `json:"path"`
        Token  string `json:"token"`
    }
    // ...
    switch req.Action {
    case "publish":
        // WHIP: Fahrzeug-Client authentifiziert sich mit Stream Key
        if whipStreamKey == "" || req.Token != whipStreamKey {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
    case "read":
        // WHEP: Operator-Browser authentifiziert sich mit JWT + aktive Session
        if req.Token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        _, hasSession := sessionMgr.GetCurrentSession()
        if !hasSession {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
    }
    w.WriteHeader(http.StatusOK)
})
```

#### KickVehicle bei SAFE_MODE

```go
// Beim Auslösen von SAFE_MODE:
go mtxClient.KickVehicle(vehicleID)
```

### 6.4 mediamtx.Client — SAFE_MODE-Integration

**Datei:** `internal/mediamtx/client.go`

```go
func (c *Client) KickVehicle(vehicleID string) {
    // 1. Alle aktiven WebRTC-Sessions für vehicle-001 von der Management API holen
    sessions, _ := c.listSessions(vehicleID)
    // 2. Jede Session einzeln terminieren
    for _, s := range sessions {
        c.deleteSession(s.ID)  // POST /v3/webrtcsessions/kick/{id}
    }
}

func (c *Client) listSessions(vehicleID string) ([]webrtcSession, error) {
    url := fmt.Sprintf("%s/v3/webrtcsessions/list", c.apiURL)
    // Filtert Items nach Path == vehicleID
}
```

Die Management API (`mediamtx:9997`) ist nur Docker-intern erreichbar — nie nach außen exponiert.

### 6.5 useWebRTC.ts — Browser WHEP Consumer

**Datei:** `frontend/src/hooks/useWebRTC.ts`

Vollständiger WHEP-Verbindungsaufbau in der `connect()`-Funktion:

```typescript
const connect = useCallback(async () => {
  // 1. ICE-Server-Liste laden
  const iceServers = await fetchIceServers()   // GET /api/ice-config
  const pc = new RTCPeerConnection({ iceServers })
  pcRef.current = pc

  // 2. Nur Video empfangen (kein Senden)
  pc.addTransceiver('video', { direction: 'recvonly' })

  // 3. Callbacks
  pc.ontrack = (event) => {
    videoRef.current.srcObject = event.streams[0]
    updateState('MEDIA_CONNECTED')
  }
  pc.oniceconnectionstatechange = () => {
    if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
      updateState('MEDIA_FAILED')
    }
  }

  // 4. SDP Offer erstellen
  const offer = await pc.createOffer()

  // 5. Pion DTLS-Bug-Fix
  const fixedSdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
  await pc.setLocalDescription({ type: 'offer', sdp: fixedSdp })
  // → ICE Gathering startet jetzt

  // 6. Warten bis alle Candidates gesammelt (Vanilla ICE)
  await new Promise<void>(resolve => {
    if (pc.iceGatheringState === 'complete') { resolve(); return }
    const tid = setTimeout(resolve, 2000)
    pc.addEventListener('icegatheringstatechange', function handler() {
      if (pc.iceGatheringState === 'complete') {
        clearTimeout(tid); resolve()
      }
    })
  })

  // 7. WHEP POST mit vollständigem SDP (inkl. aller ICE Candidates)
  const res = await fetch(`/whep/${vehicleId}/whep`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/sdp',
      'Authorization': `Bearer ${token}`,
    },
    body: pc.localDescription!.sdp,
  })

  // 8. SDP Answer von MediaMTX verarbeiten
  const answerSdp = await res.text()
  await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
  // → ICE Connectivity Checks starten
  // → DTLS Handshake
  // → RTP-Stream → ontrack → MEDIA_CONNECTED
}, [sessionId, vehicleId, token, disconnect, updateState])
```

**RTT-Messung für ConnectionPanel** (`useWebRTC.ts` Zeile 153–171):

```typescript
// Poll ICE candidate-pair RTT jede Sekunde während MEDIA_CONNECTED
const stats = await pc.getStats()
stats.forEach(r => {
  if (r.type === 'candidate-pair' && r.nominated &&
      typeof r.currentRoundTripTime === 'number' && r.currentRoundTripTime > 0) {
    setVideoLatencyMs(Math.round(r.currentRoundTripTime * 1000))
  }
})
// currentRoundTripTime ist in Sekunden → * 1000 → Millisekunden
// 0-Guard: Wert ist 0 bis zum ersten STUN Consent Check (~5s nach ICE-Establishment)
```

**ICE-Server-Validierung** (schützt gegen leere TURN_EXTERNAL_IP):

```typescript
function isValidIceServer(s: RTCIceServer): boolean {
  const urls = Array.isArray(s.urls) ? s.urls : [s.urls]
  // Reject "stun::3478" — entsteht wenn TURN_EXTERNAL_IP leer ist
  return urls.every(u => !/^(stun|turn):(?::|\?)/.test(u))
}
```

**Auto-Retry bei MEDIA_FAILED:**

```typescript
useEffect(() => {
  if (mediaState !== 'MEDIA_FAILED' || !enabled || !sessionId || !token) return
  const tid = setTimeout(connect, 3000)  // 3s Pause, dann neu verbinden
  return () => clearTimeout(tid)
}, [mediaState, enabled, sessionId, token])
```

Nach SAFE_MODE schlägt der Retry an der WHEP-Auth fehl (keine aktive Session) — das ist gewolltes Verhalten (ADR-009: kein Auto-Resume).

### 6.6 useWHIPSender.ts — Browser WHIP Publisher

**Datei:** `frontend/src/hooks/useWHIPSender.ts`

Der Browser kann selbst als Fahrzeug-Simulator fungieren (Sprint 10 Nachtrag, `StreamSenderPanel.tsx`):

```typescript
const start = useCallback(async (whipUrl: string, sourceType: SourceType, streamKey: string) => {
  // 1. Medienquelle capturen (HTTPS Pflicht!)
  let stream: MediaStream
  if (sourceType === 'screen') {
    stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true })
  } else {
    stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true })
  }

  // 2. ICE-Server + PeerConnection
  const iceServers = await fetchIceServers()
  const pc = new RTCPeerConnection({ iceServers })

  // 3. Tracks hinzufügen (senden, nicht empfangen)
  stream.getTracks().forEach(track => pc.addTrack(track, stream))

  // 4. Offer + DTLS-Fix + ICE Gathering (identisch zu useWebRTC)
  const offer = await pc.createOffer()
  const sdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
  await pc.setLocalDescription({ type: 'offer', sdp })
  // ... ICE Gathering wait ...

  // 5. WHIP POST
  const res = await fetch(whipUrl, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/sdp',
      'Authorization': `Bearer ${streamKey}`,
    },
    body: pc.localDescription!.sdp,
  })

  // 6. Location für späteres DELETE merken
  const location = res.headers.get('Location')
  if (location) {
    locationRef.current = location.startsWith('http')
      ? location
      : `/whip${location.startsWith('/') ? '' : '/'}${location}`
  }

  await pc.setRemoteDescription({ type: 'answer', sdp: await res.text() })
}, [previewRef, stop])
```

**Session beenden:**

```typescript
const stop = useCallback(() => {
  if (locationRef.current) {
    // WHIP DELETE — signalisiert MediaMTX dass der Publisher die Session beendet
    fetch(locationRef.current, { method: 'DELETE' }).catch(() => {})
  }
  pcRef.current?.close()
  streamRef.current?.getTracks().forEach(t => t.stop())
}, [previewRef])
```

**HTTPS-Pflicht:** `navigator.mediaDevices.getUserMedia()` und `getDisplayMedia()` sind nur in Secure Contexts (HTTPS oder `localhost`) verfügbar. Deshalb: Self-Signed-Zertifikat in Prod + nginx auf Port 443.

### 6.7 Pion WebRTC SFU

**Datei:** `internal/webrtcsfu/sfu.go`

Der Pion-SFU hat seit ADR-020 **keine Media-Routing-Funktion** mehr — MediaMTX übernimmt das. Der SFU ist heute ein reiner **Session-Event-Consumer** (ADR-015: "Dumb Media Router with State Subscription").

```go
// Session-Event vom Control Server empfangen (push-basiert, nie aktiv gefragt)
func (s *SFU) HandleSessionEvent(event SessionEvent) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.state[event.SessionID] = event.Type

    switch event.Type {
    case EventCreated:
        s.routing[event.SessionID] = []string{}
    case EventOperatorAssigned, EventOperatorHandover:
        s.routing[event.SessionID] = []string{event.OperatorID}
    case EventSafeMode:
        s.dropStreams(event.SessionID)  // SFU-interne Peer Connections schließen
    case EventEnded:
        s.dropStreams(event.SessionID)
        delete(s.routing, event.SessionID)
    }
}
```

**Track-Forwarding-Logik** (zeigt das Architekturprinzip — SAFE_MODE blockiert Forwarding):

```go
func (s *SFU) forwardTrack(sessionID string, track *webrtc.TrackRemote) {
    rtpBuf := make([]byte, 1400)
    for {
        n, _, err := track.Read(rtpBuf)
        if err != nil { return }

        s.mu.RLock()
        safeModeActive := s.state[sessionID] == EventSafeMode
        s.mu.RUnlock()

        if safeModeActive {
            continue  // SESSION_SAFE_MODE → kein Forwarding (ADR-015 Invariante)
        }
        // RTP-Pakete an alle subscribed Operators schicken
    }
}
```

### 6.8 Session-Event Push: Control Server → SFU

**Datei:** `internal/controlserver/session/sfu_publisher.go`

Der Control Server pushiert Session-Events asynchron via HTTP POST an den SFU — der SFU fragt nie aktiv Zustand ab (ADR-007: Dumb Media Router):

```go
func (p *HTTPSFUPublisher) PublishSessionEvent(eventType, sessionID, operatorID string) {
    body, _ := json.Marshal(map[string]string{
        "type":        eventType,    // z.B. "SESSION_SAFE_MODE"
        "session_id":  sessionID,
        "operator_id": operatorID,
    })
    p.client.Post(p.baseURL+"/session/event", "application/json", bytes.NewReader(body))
}
```

### 6.9 nginx als WebRTC-Proxy

**Datei:** `infrastructure/docker/nginx.dev.conf`

nginx ist kein aktiver Teilnehmer in WebRTC-Verbindungen — er proxyt nur das HTTP-Signaling (WHIP/WHEP POST). Die eigentlichen RTP-Media-Pakete fließen direkt zwischen Browser und MediaMTX über UDP (Port 8189):

```nginx
# WHIP: Browser → nginx → MediaMTX (Präfix /whip/ wird gestripped)
# Bewusst OHNE $upstream-Variable (Sprint 19): mediamtx läuft im Dev-Stack mit
# network_mode: host (ICE-Stabilität) — kein Docker-DNS-Eintrag "mediamtx" auf
# avoc-net. Der `resolver`-Trick der anderen Locations fragt nur Docker-DNS
# (127.0.0.11) und kennt host.docker.internal nicht (reiner /etc/hosts-Eintrag
# via extra_hosts). Statisches proxy_pass wird einmalig beim Config-Load über
# den System-Resolver aufgelöst, der /etc/hosts respektiert.
location /whip/ {
    rewrite ^/whip/(.*) /$1 break;
    proxy_pass http://host.docker.internal:8889;
    proxy_set_header Authorization $http_authorization;
}

# WHEP: Browser → nginx → MediaMTX (Präfix /whep/ wird gestripped)
location /whep/ {
    rewrite ^/whep/(.*) /$1 break;
    proxy_pass http://host.docker.internal:8889;
    proxy_set_header Authorization $http_authorization;
}
```

`/whip/vehicle-001/whip` → nginx strippt → `/vehicle-001/whip` → MediaMTX.

> **Prod-Unterschied:** In `docker-compose.prod.yml` läuft `mediamtx` *ohne* `network_mode: host`
> (normaler Bridge-Service) — dort ist `http://mediamtx:8889` weiterhin korrekt und per
> Docker-DNS auflösbar. Der `host.docker.internal`-Fix betrifft nur `nginx.dev.conf`.

---

## 7. Vollständiger Datenfluss — Ende zu Ende

### 7.1 Video-Ingestion (Browser WHIP Sender → MediaMTX)

```
useWHIPSender.start()
  │
  ├─ 1. getUserMedia() → Webcam-Stream (HTTPS Pflicht!)
  ├─ 2. GET /api/ice-config → [STUN, TURN UDP, TURN TCP]
  ├─ 3. new RTCPeerConnection({ iceServers })
  ├─ 4. stream.getTracks() → pc.addTrack() (sendonly)
  ├─ 5. pc.createOffer() → SDP Offer
  ├─ 6. (entfällt seit Sprint 32/WEBRTC-10 — kein SDP-Fix mehr, Offer bleibt actpass)
  ├─ 7. pc.setLocalDescription() → ICE Gathering startet
  │      Browser → coturn STUN: "Was ist meine öffentliche IP?"
  │        → srflx Candidate: 18.196.24.10:54321
  │      Browser → coturn TURN: "Reserviere Relay-Adresse"
  │        → relay Candidate: 18.196.24.10:49200
  ├─ 8. Warten: iceGatheringState === 'complete' (max 2s)
  ├─ 9. POST /whip/vehicle-001/whip
  │      Authorization: Bearer <WHIP_STREAM_KEY>
  │      Body: SDP Offer (host + srflx + relay Candidates)
  │
  │      nginx → MediaMTX:8889
  │      MediaMTX → POST /internal/media/auth
  │        { action: "publish", token: WHIP_STREAM_KEY } → 200 OK
  │
  ├─ 10. Response: 201 Created
  │       SDP Answer (MediaMTX ICE Candidate: 18.196.24.10:8189)
  │       Location: /vehicle-001/whip/abc123
  ├─ 11. pc.setRemoteDescription(answer)
  ├─ 12. ICE Connectivity Checks
  │       browser-srflx:54321 ↔ mediamtx-host:8189 → SUCCESS ✓
  ├─ 13. DTLS Handshake (Browser=Client, MediaMTX=Server)
  └─ 14. RTP-Pakete: Browser UDP → 18.196.24.10:8189
         Status: 'live'
```

### 7.2 Video-Distribution (MediaMTX → Operator Browser WHEP)

```
useWebRTC.connect()
  │
  ├─ 1–8: (identisch zu WHIP — ICE Gathering)
  │
  ├─ 9. POST /whep/vehicle-001/whep
  │      Authorization: Bearer <JWT>
  │      Body: SDP Offer (recvonly video)
  │
  │      nginx → MediaMTX:8889
  │      MediaMTX → POST /internal/media/auth
  │        { action: "read", token: JWT } → aktive Session? → 200 OK
  │
  ├─ 10. Response: 200 OK
  │       SDP Answer (MediaMTX ICE Candidate: 18.196.24.10:8189)
  ├─ 11. pc.setRemoteDescription(answer)
  ├─ 12. ICE Connectivity Checks (identisch)
  ├─ 13. DTLS Handshake
  ├─ 14. RTP-Stream: MediaMTX UDP → Browser
  ├─ 15. pc.ontrack → videoRef.current.srcObject = stream
  └─ 16. updateState('MEDIA_CONNECTED')
         VideoPanel zeigt Live-Video
```

### 7.3 SAFE_MODE → Video-Kick

```
SAFE_MODE ausgelöst (Deadman / VehicleACKWatchdog / Safety Bus)
  │
  ├─ Control Server: go mtxClient.KickVehicle("vehicle-001")
  │     GET mediamtx:9997/v3/webrtcsessions/list → alle aktiven Sessions
  │     POST mediamtx:9997/v3/webrtcsessions/kick/{id} → Session terminiert
  │
  ├─ MediaMTX: WebRTC-Verbindung getrennt
  │
  ├─ Browser: pc.oniceconnectionstatechange → 'disconnected'
  │     useWebRTC: updateState('MEDIA_FAILED')
  │
  ├─ Control Server: PublishSessionEvent("SESSION_SAFE_MODE", ...)
  │     → Pion SFU: HandleSessionEvent → dropStreams
  │
  ├─ Auto-Retry nach 3s: connect() → WHEP POST
  │     MediaMTX → /internal/media/auth → keine aktive Session → 401
  │     → MEDIA_FAILED bleibt (gewollt — kein Auto-Resume bis Operator-Ack)
  │
  └─ Nach Operator-Ack + Session-Recovery:
     connect() → WHEP Auth → 200 OK → MEDIA_CONNECTED
```

---

## 8. Port-Übersicht

| Port | Protokoll | Dienst | Zweck |
|------|-----------|--------|-------|
| 443 | TCP (TLS) | nginx | HTTPS — Pflicht für `getUserMedia` |
| 80 | TCP | nginx | HTTP (Dev) |
| 8889 | TCP | MediaMTX | WHIP/WHEP HTTP Signaling |
| 8189 | UDP | MediaMTX | ICE Media-Mux (RTP-Pakete) |
| 9997 | TCP | MediaMTX | Management API (nur Docker-intern) |
| 3478 | TCP+UDP | coturn | STUN + TURN Signaling |
| 49152–65535 | UDP | coturn | TURN Relay-Ports |
| 8084 | TCP | Pion SFU | Session-Event-Empfang (intern) |

---

## 9. Sprint-10 Root Causes — Zusammenfassung

Alle 5 Root Causes aus der ICE-Migration und ihre Lösungen:

| # | Problem | Ursache | Lösung |
|---|---------|---------|--------|
| 1 | **Candidate Explosion** — ICE-Timeout 30–60s | `webrtcIPsFromInterfaces` fehlte → MediaMTX annoncierte alle Docker-Interfaces (172.x, 10.x, 127.x) | `webrtcIPsFromInterfaces: false` in `mediamtx.yml` |
| 2 | **Srflx auf gesperrten Ports** — alle ICE-Pairs fail | `webrtcICEServers2` → MediaMTX gatherte eigene srflx-Candidates auf ephemeren Ports die nicht in der AWS Security Group offen waren | `webrtcICEServers2` komplett entfernt; Browser übernimmt vollständig das ICE-Gathering |
| 3 | **Pion DTLS-Client-Bug** (Sprint 10, `pion/webrtc v1.19.0`-Ära) — Retransmit-Loop bis Timeout | Browser sendet `a=setup:actpass` → Pion wählt "active" → verarbeitet ServerHello nicht korrekt | Ursprünglich: SDP-Fix `actpass → active` erzwingen. **Seit Sprint 32 (WEBRTC-10) entfernt** — siehe Update unten |
| 4 | **TURN-Relay nicht erreicht** auf 5G/CGNAT | `useWebRTC.ts` hatte nur STUN (falscher Port 3479), kein TURN UDP/TCP | `GET /api/ice-config` Endpoint im Control Server; liefert STUN + TURN UDP + TURN TCP |
| 5 | **coturn relay-Adresse falsch** auf AWS | `external-ip=PUBLIC` ohne Private-Mapping → coturn advertised falsche Relay-Adresse | `--relay-ip=PRIVATE --external-ip=PUBLIC/PRIVATE` + `network_mode: host` + IMDSv2 für `TURN_PRIVATE_IP` |

> **Update Sprint 19 (2026-07-10):** Der `actpass → active`-Fix aus Zeile 3 wurde lokal mit
> aktuellem Chromium erneut getestet — `setRemoteDescription` schlägt jetzt fehl mit
> *"Offerer must use actpass value for setup attribute"*. Der ursprüngliche Pion-v1.19.0-Bug
> ist damit möglicherweise durch ein neueres, gegenteiliges Problem ersetzt worden (modernes
> Chrome erzwingt Spec-Konformität, die der Workaround verletzt). Nicht behoben — betrifft
> potenziell auch den Produktiv-Video-Empfang auf AWS (`useWebRTC.ts`/WHEP), nicht nur lokale
> Tests. Siehe `CONTEXT.MD` „Offene Fragen" und Backlog-Task `WEBRTC-10`.
>
> **Update Sprint 32 (2026-07-18, WEBRTC-10 — behoben):** Technische Vorab-Prüfung ergab:
> `mediamtx:latest` (aktuell `v1.18.1`) baut gegen `pion/webrtc v4.2.12` — eine komplett andere
> Codebasis als das 2026-06-18 referenzierte, längst abgelöste `v1.19.0`. Der `actpass → active`-
> Zwang wurde entfernt (`useWebRTC.ts`, `useWHIPSender.ts`); der Offer bleibt jetzt Standard-
> `actpass` (RFC 8842). Lokal gegen den echten `mediamtx:latest`-Container verifiziert (isolierter
> WHIP-Publisher mit `pion/webrtc v4.2.12`, exakt passend zur mediamtx-Version, da eine ältere
> Client-Version — `v4.0.14`, wie sonst im Repo gepinnt — im ersten Testlauf selbst hing und damit
> ein Testartefakt statt eines echten mediamtx-Bugs gewesen wäre): MediaMTX beantwortet den
> Standard-Offer mit `a=setup:active` (RFC-8842-Empfehlung für den Answerer — exakt die Rolle, die
> 2026-06-18 als fehlerhaft dokumentiert war) und der DTLS/ICE-Handshake erreicht zuverlässig
> `PeerConnectionStateConnected` (6/6 erfolgreiche Läufe bei kontrollierter, auf die
> Loopback-Schnittstelle beschränkter ICE-Kandidatenauswahl — die Verifikationsumgebung hat
> ungewöhnlich viele Netzwerk-Interfaces, was in ungefilterten Läufen zu ICE-Kandidatenpaar-
> Flakiness führte, unabhängig von der eigentlichen DTLS-Frage). Damit ist der ursprüngliche
> Pion-Bug in der aktuellen MediaMTX-Version nicht mehr reproduzierbar. Produktiv-Video-Empfang auf
> AWS (echtes Chromium statt des Pion-Testclients) bleibt außerhalb dieses Sprints unverifiziert —
> siehe `tasks/current-sprint.md` Sprint 32 für die vollständige Herleitung.

---

## 10. Architektur-Invarianten (Safety)

Aus ADR-009, ADR-014, ADR-020:

1. **Video = Awareness only** — `MEDIA_FAILED` → `DEGRADED`, nie `SAFE_MODE`. Video-Ausfall blockiert nie die Steuerung.
2. **Control Hub > Video Hub** — Control Server allein entscheidet über SAFE_MODE; MediaMTX führt nur aus (kein eigenes Safety-Urteil).
3. **Kein Auto-Resume** — nach SAFE_MODE schlägt WHEP-Auth fehl (keine aktive Session) bis Operator-Ack erfolgt ist.
4. **Pion SFU = Dumb Router** — konsumiert Session-Events, ruft niemals externe Services an, beeinflusst nie den System-State.
5. **Control Server = Single Auth-Instanz** — sowohl WHIP (Stream Key) als auch WHEP (JWT) laufen durch denselben `/internal/media/auth` Hook. MediaMTX hat keine eigenen Credentials.
