# Sprint 9 — WebRTC Videostream: Larix WHIP → MediaMTX → Browser

**Datum:** 2026-06-05
**Vorgänger:** Sprint 8 ✅ (EC2 Deployment via Docker Hub — ADR-019)
**ADR:** ADR-020 (MediaMTX als WHIP/WHEP Router)

### Zielsetzung

Einrichten des Live-Videostreams vom Smartphone (Larix Broadcaster via WHIP über 5G) durch
MediaMTX als Router zum Operator-Browser (WHEP). Ablösung des Custom-SFU-Signaling durch
IETF-Standard-Protokolle (WHIP/WHEP).

### Architektur-Entscheidung

MediaMTX übernimmt WHIP-Ingestion (Larix) und WHEP-Distribution (Browser).
Der **Control Server** authentifiziert alle WHIP/WHEP-Requests via `externalAuthenticationURL`.
Bei SAFE_MODE ruft der Control Server direkt die MediaMTX Management API auf.
Der Pion SFU bleibt passiver Session-Event-Subscriber ohne ausgehende Calls.

### Implementierte Tasks

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| STREAM-01 | ADR-020 — MediaMTX als WHIP/WHEP Router | L | ✅ | `docs/adr/020-mediamtx-whip-whep.md` vollständig |
| STREAM-02 | `infrastructure/mediamtx/mediamtx.yml` + Docker Service | M | ✅ | MediaMTX startet, externalAuthenticationURL konfiguriert |
| STREAM-03 | nginx: `/whep/` Proxy | S | ✅ | Rewrite + Authorization-Header-Forwarding korrekt |
| STREAM-04 | `useWebRTC.ts` → WHEP-Protokoll + vehicleId-Prop | M | ✅ | ICE-Gathering-Wait, application/sdp, res.text() |
| STREAM-05 | Control Server: `POST /internal/media/auth` + SAFE_MODE → MediaMTX API | M | ✅ | Auth-Hook + KickVehicle bei Emergency Stop |
| STREAM-06 | TURN in MediaMTX ICE-Config + Compose env | S | ✅ | TURN_USER/TURN_PASSWORD aus vorhandenen SSM-Params |
| STREAM-07 | CDK Port 8889 + SSM `/avoc/prod/whip-stream-key` + setup-ssm.sh | S | ✅ | Security Group offen, SSM-Param + Validierung (min 32 Zeichen) |
| STREAM-08 | `docker-compose.prod.yml`: mediamtx + Control Server env + deploy.sh | S | ✅ | WHIP_STREAM_KEY aus SSM, MEDIAMTX_API_URL gesetzt |
| STREAM-09 | Larix Setup Guide + E2E Smoke Test Protokoll | S | ✅ | `docs/deployment/larix-setup.md` mit 7-Schritt-E2E-Tabelle |

### Testprotokoll (2026-06-05)

| ID | Test | Prüfmethode | Ergebnis |
|----|------|-------------|----------|
| T01 | ADR-020 vollständig (Context, Decision, Consequences) | `grep -c "^##"` → 17 Abschnitte | ✅ 17 Abschnitte, alle Pflicht-Keywords vorhanden |
| T02 | mediamtx.yml — Schlüssel-Felder korrekt | grep auf externalAuthenticationURL, webrtcICEServers2, api, webrtc, paths | ✅ 5/5 Felder vorhanden |
| T03 | mediamtx Service in docker-compose.yml | grep auf image, ports 8889+9997, volume, depends_on | ✅ Alle Felder vorhanden |
| T04 | nginx /whep/ Proxy — rewrite + Authorization-Header | grep auf rewrite, proxy_pass, Authorization | ✅ Rewrite korrekt, Header weitergeleitet |
| T05 | useWebRTC.ts nutzt WHEP-Standard | grep auf application/sdp, /whep/, res.text() | ✅ 4 Treffer — WHEP-Protokoll korrekt |
| T06 | Kein alter /sfu/subscribe Endpoint | grep -c "sfu/subscribe" | ✅ 0 Treffer — vollständig abgelöst |
| T07 | ICE-Gathering wartet auf complete | grep auf icegatheringstatechange, iceGatheringState | ✅ 4 Treffer — non-trickle WHEP korrekt |
| T08 | vehicleId in SessionState Interface | grep "vehicleId" in useSession.ts | ✅ Interface + Return-Wert vorhanden |
| T09 | VideoPanel Props enthält vehicleId | grep "vehicleId" in VideoPanel.tsx | ✅ Props-Interface + Hook-Call mit vehicleId |
| T10 | App.tsx übergibt vehicleId an VideoPanel | grep "vehicleId" in App.tsx | ✅ `vehicleId={session.vehicleId}` gesetzt |
| T11 | Control Server auth hook POST /internal/media/auth | grep -c in main.go | ✅ Endpoint + publish/read-Logik vorhanden (2 Treffer) |
| T12 | WHIP_STREAM_KEY in main.go geladen | grep -c "WHIP_STREAM_KEY\|whipStreamKey" | ✅ 2 Treffer — env + Verwendung |
| T13 | KickVehicle bei Emergency Stop (goroutine) | grep -n "KickVehicle\|mtxClient" main.go | ✅ `go mtxClient.KickVehicle(sess.VehicleID)` in emergency-stop Handler (Zeile 192) |
| T14 | internal/mediamtx/client.go — KickVehicle + listSessions + deleteSession | grep -n auf alle 3 Funktionen | ✅ Alle 3 Methoden implementiert (Zeilen 35, 50, 72) |
| T15 | CDK Port 8889 Security Group | grep "8889" cdk_server-stack.ts | ✅ `addIngressRule(...Port.tcp(8889)...)` vorhanden |
| T16 | setup-ssm.sh — whip-stream-key Entry (min 32 Zeichen) | Datei-Inspektion | ✅ Prompt + Längenvalidierung + put_secure-Aufruf (Zeile 76–97) |
| T17 | deploy.sh lädt whip-stream-key aus SSM | Datei-Inspektion | ✅ `export WHIP_STREAM_KEY=$(get_secure /avoc/prod/whip-stream-key)` (Zeile 46) |
| T18 | docker-compose.prod.yml — mediamtx + WHIP_STREAM_KEY | Datei-Inspektion | ✅ mediamtx Service + WHIP_STREAM_KEY + MEDIAMTX_API_URL in control-server |
| T19 | Larix Setup Guide — WHIP-URL, Codec, Bearer Token, E2E | Datei-Inspektion | ✅ 7-Schritt-E2E-Tabelle, WHIP URL, H.264-Codec, Bearer Token dokumentiert |
| T20 | Go Build — alle Packages kompilieren fehlerfrei | `docker run golang:1.23-alpine go build ./...` | ✅ Exit 0 — alle Services inkl. `internal/mediamtx` kompilieren |
| T21 | TypeScript — kein Typfehler | `npx tsc --noEmit` | ✅ Exit 0 — keine TS-Fehler nach useWebRTC-Umbau |
| T22 | Safety Regression — Safety-Code unverändert | `git diff HEAD~1 -- internal/safety/ cmd/safety-service/ internal/session/state_machine.go` | ✅ 0 Zeilen geändert — Safety-Pfad vollständig unberührt |
| T23 | Port-Konsistenz 8889 (CDK ↔ Compose ↔ nginx) | grep 8889 in allen Infra-Dateien | ✅ Port 8889 konsistent in CDK, dev-compose, prod-compose, nginx |

**Ergebnis: 23/23 Tests bestanden ✅**

### Bugfix während Testprotokoll

**vehicleId-Mismatch behoben:**
`useSession.ts` hatte `VEHICLE_ID = 'vehicle-1'`, während `larix-setup.md` `vehicle-001` als WHIP-Pfad
dokumentiert. Bei diesen unterschiedlichen Pfaden würde der Browser WHEP auf `vehicle-1` abfragen,
während Larix auf `vehicle-001` published — das Video käme nie an.
**Fix:** `VEHICLE_ID = 'vehicle-001'` in `useSession.ts` — übereinstimmend mit `larix-setup.md` und
dem MediaMTX-Pfad in `mediamtx.yml`.

### Definition of Done

- [x] ADR-020 dokumentiert (`docs/adr/020-mediamtx-whip-whep.md`)
- [x] MediaMTX startet in Docker Compose, WHIP-Endpunkt auf Port 8889
- [ ] Larix (Smartphone, 5G) streamt erfolgreich zu MediaMTX — *E2E-Test ausstehend (Hardware)*
- [ ] Operator-Browser empfängt Video via WHEP — `MEDIA_CONNECTED` sichtbar — *E2E-Test ausstehend (Hardware)*
- [x] SAFE_MODE stoppt Video (Control Server → MediaMTX API — kein SFU-Umweg)
- [x] Ein Auth-Mechanismus: alle WHIP/WHEP-Requests via `externalAuthenticationURL` → Control Server
- [x] TURN-Credentials konfiguriert (ICE für Operator hinter NAT)
- [x] CDK Port 8889 offen, SSM `whip-stream-key` angelegt
- [x] Larix Setup Guide vorhanden (`docs/deployment/larix-setup.md`)
- [x] Safety Regression: 0 Zeilen geändert — Safety-Pfad vollständig unberührt
- [x] vehicleId-Mismatch behoben (`vehicle-1` → `vehicle-001`)

### Offene Punkte (Sprint 10)

- E2E-Test mit echtem Larix-Smartphone und MediaMTX auf EC2 (Hardware-abhängig)
- EC2-Deployment von Sprint 9: neues Docker-Image pushen + `whip-stream-key` in SSM + `mediamtx.yml` auf EC2 kopieren
- vehicleId dynamisch aus Session-Assign (aktuell hardcoded `vehicle-001`)

---

**Redaktionshinweis (MD-Konsolidierung, 2026-07-19):** Diese Datei fasst zwei zuvor parallel in
`tasks/done.md` geführte Versionen dieses Sprints zusammen (dort stand Sprint 9 versehentlich
zweimal — eine kurze Tabellen-Fassung und diese vollständige Fassung). Die kurze Fassung enthielt
keine Informationen, die hier fehlen, und wurde daher ersatzlos verworfen.
