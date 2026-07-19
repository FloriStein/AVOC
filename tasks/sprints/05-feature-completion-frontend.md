# Sprint 5 — Feature Completion Frontend

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| FE-05 | Control Panel UI — Keyboard + Virtual Joystick + Gamepad | M | ✅ `useControls.ts` (20 Hz, Keyboard WASD/Pfeiltasten, Virtual Joystick, Gamepad API); `ControlPanel.tsx` (SVG Joystick, Speed Slider, Steer/Throttle Bars, Mode-Anzeige) |
| FE-06 | Video Stream Panel — WebRTC RTCPeerConnection | M | ✅ `useWebRTC.ts` (RTCPeerConnection, SDP Signaling via `/sfu/subscribe/`, MEDIA STATE Tracking, `reportMediaState()` → Control Server); `VideoPanel.tsx` (video Element, MEDIA STATE Badge, Overlays, Retry-Button); SFU Track-Forwarding Fix (`TrackLocalStaticRTP` in `SubscribeOperator`) |
| FE-07 | Teleoperation Dashboard — Integration + Telemetrie | M | ✅ `useTelemetry.ts` (1 Hz Polling `/telemetry/latest/{vehicleId}`); `App.tsx` (VideoPanel + ControlPanel + Telemetrie + Operator-Rolle im Header); `ConnectionPanel.tsx` (Speed/Battery/Status); `ws-client.ts` (Protobuf ControlAck parsen, `onAckError` Callback); `websocket.go` (JSON-Fallback → Protobuf Binary) |

### Fixes während Implementierung

1. **`nginx.conf`**: Docker DNS Caching → 502 nach Container-Rebuild. Fix: `resolver 127.0.0.11 valid=5s` + variable-basiertes `proxy_pass`. **Achtung:** `set $var` muss **vor** `rewrite...break` stehen (beide im Rewrite-Modul — `break` stoppt nachfolgende Set-Direktiven).
2. **`ws-client.ts`**: `fromBinary()` gibt `Message`-Basistyp zurück → `as any` Cast erforderlich für `.success` / `.errorMsg` Zugriff.
3. **`internal/webrtcsfu/sfu.go`**: `SubscribeOperator` registrierte Operator-Peer ohne `Tracks` → RTP-Forwarding schrieb in leeren Slice. Fix: `TrackLocalStaticRTP` erstellen und in `peer.Tracks` speichern.

### Testprotokoll Integration (2026-06-03)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | Go Build (alle Sprint-5-Änderungen) | `OK` | ✅ |
| T02 | Safety Regression (19/19) | Alle grün | ✅ 19/19 |
| T03 | Service Health (alle 8 Services) | HTTP 200 je Service | ✅ |
| T04 | Frontend Bundle — neue Strings | 16 minifizierungs-stabile Strings im Bundle | ✅ alle 16 |
| T05 | nginx Routing (6 Routen) | auth→200, state→200, sfu-health→200, telemetry→404, sfu-subscribe→500, /ws→401 | ✅ alle |
| T06 | Session-Lifecycle + Protobuf Commands | 101 WS Upgrade, ULID-Session, 7 cmd-Typen mit Protobuf-ACK success | ✅ |
| T07 | Command Engine Log | `[CMD] COMMAND_TYPE_STEER/THROTTLE/BRAKE/SPEED` im Log | ✅ |
| T08 | Protobuf-Fallback (kein JSON) | Fallback-ACK: Protobuf binary, success=false, error_msg='no active session' | ✅ 28 bytes Protobuf |
| T09 | MEDIA STATE — alle 5 Übergänge | NEGOTIATING/CONNECTED/DEGRADED (2×)/INIT je HTTP 202 | ✅ alle |
| T10 | SFU TrackLocalStaticRTP Fix | String 'avoc-vehicle' im SFU-Binary, `NewTrackLocalStaticRTP` in sfu.go:214 | ✅ |
| T11 | reportMediaState im Bundle | 'media/event' im JS-Bundle | ✅ |
| T12 | useTelemetry im Bundle | 'speed_kmh' im JS-Bundle | ✅ |
| T13 | onAckError im Bundle | 'onAckError' im JS-Bundle | ✅ |
| T14 | nginx DNS-Resolver (set vor rewrite) | Zeile 14: `set $upstream_cs` vor Zeile 15: `rewrite` | ✅ |
| T15 | 20Hz Rate-OK + Burst Rate-Limiter | 10/10 ACK bei 20Hz (0.4ms avg); 10/110 rejected bei Burst | ✅ |
| T16 | nginx kein 502 nach Restart | Auth nach auth-service-Restart: HTTP 200; SFU nach SFU-Restart: HTTP 200 | ✅ |
| T17 | Recovery Flow | SAFE_MODE → RECOVERING → AUTHENTICATED nach neuem WS-Connect | ✅ |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| ACK Latenz 20Hz | Ø 0.4 ms (localhost) | < 100ms ✅ |
| Rate Limiter Schwelle | 100 cmd/s | 100 cmd/s ✅ |
| Bundle-Größe | 2 JS-Dateien (index + proto) | — |
| Safety Tests | 19/19 | 19/19 ✅ |

### Protokollierte Log-Ausgaben (Auszug)

```
[CMD] COMMAND_TYPE_STEER value=0.75 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_THROTTLE value=0.50 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_BRAKE value=1.00 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_SPEED value=0.60 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] EMERGENCY_STOP → SAFE_MODE (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[STATE] MEDIA MEDIA_FAILED → SYSTEM DEGRADED (Invariant 1: never SAFE_MODE)
[STATE] SYSTEM: SAFE_MODE → RECOVERING (via WS reconnect)
[RECORDING] session started: id=01KT7JH1C46ZPB2J1W5Y61HK9G vehicle=vehicle-1 operator=operator-1
```

### Neue Dateien

- `frontend/src/hooks/useControls.ts` — 20 Hz Keyboard/Joystick/Gamepad Command Loop (FE-05)
- `frontend/src/components/ControlPanel.tsx` — Virtual Joystick SVG + Speed Slider (FE-05)
- `frontend/src/hooks/useWebRTC.ts` — RTCPeerConnection + SDP Signaling (FE-06)
- `frontend/src/components/VideoPanel.tsx` — Video Element + MEDIA STATE Badge (FE-06)
- `frontend/src/hooks/useTelemetry.ts` — 1 Hz Telemetrie-Polling (FE-07)

### Geänderte Dateien

- `frontend/src/App.tsx` — vollständiges Dashboard (VideoPanel, ControlPanel, Telemetrie, Operator-Rolle)
- `frontend/src/lib/api-client.ts` — `reportMediaState()` ergänzt
- `frontend/src/lib/ws-client.ts` — Protobuf ControlAck parsen, `onAckError` Callback
- `frontend/src/components/ConnectionPanel.tsx` — Speed/Battery/Status-Felder
- `internal/controlserver/transport/websocket.go` — JSON-Fallback → Protobuf Binary
- `internal/webrtcsfu/sfu.go` — `TrackLocalStaticRTP` Track-Forwarding Fix
- `infrastructure/docker/nginx.conf` — Docker DNS Resolver + variable proxy_pass
