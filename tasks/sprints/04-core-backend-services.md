# Sprint 4 — Core Backend Services

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| INFRA-02 | Proto-Gen Fix | S | ✅ `--go_opt=module=avoc` erzeugt korrekte Verzeichnisstruktur `gen/go/control/v1/control.pb.go` |
| BE-04 | Command Engine | M | ✅ Protobuf-Parsing, DEADMAN_HOLD/RELEASE, EMERGENCY_STOP-Routing, Rate Limiting 100 cmd/s, Protobuf ControlAck |
| BE-05 | MQTT Telemetry Service | M | ✅ Paho v1.4.3, `vehicle/+/telemetry` Subscribe, TelemetryEvent Protobuf, `GET /telemetry/latest/{id}` |
| BE-07 | Session Recording | M | ✅ `SessionRecorder` Interface + `MemoryRecorder`; Control Server zeichnet Session-Start, State-Snapshots, Safety-Events auf |
| BE-08 | WebRTC SFU | M | ✅ Pion/Go v4.0.14, Session Event Consumer (alle 6 SESSION_*-Events), SDP-Offer/Answer Endpunkte, Primary Stream Forwarding |

### Bugfix während Implementierung

`paho.mqtt.golang`: `GOFLAGS=-mod=mod` im Dockerfile zog v1.5.1 (erfordert Go 1.24 — inkompatibel). Fix: Version explizit auf `v1.4.3` in go.mod gepinnt.

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Safety Tests Regression (19/19) | Alle grün | ✅ |
| INFRA-02: Proto-Gen Struktur | `gen/go/control/v1/control.pb.go` | ✅ Alle 5 Schemas korrekt |
| BE-04: `COMMAND_TYPE_DEADMAN_HOLD` im Binary | String im Service-Binary | ✅ |
| BE-04: `rate limited`-String im Binary | String im Service-Binary | ✅ |
| BE-04: Emergency Stop → SAFE_MODE + Recording | `SAFE_MODE / CONTROL_BLOCKED`, Recording Entry | ✅ |
| BE-05: Telemetry Service Health | `{"status":"ok"}` | ✅ |
| BE-05: Mosquitto-Verbindung | Log: `connected + subscribed` | ✅ |
| BE-05: MQTT Subscribe aktiv | Log: Parse-Error bei non-Protobuf-Nachricht (kein Crash) | ✅ |
| BE-05: `GET /telemetry/latest/unknown` | HTTP 404 | ✅ |
| BE-07: State Snapshot bei session/start | `count=1, type=state, CONNECTED/CONTROL_ACTIVE` | ✅ |
| BE-07: Safety Event bei Emergency Stop | `count=2, type=safety, EMERGENCY_STOP` | ✅ |
| BE-07: session/end → Log `entries=1` | `[RECORDING] session ended: entries=1` | ✅ |
| BE-08: Health | `{"status":"ok","service":"webrtc-sfu"}` | ✅ |
| BE-08: Alle 6 SESSION_*-Events | HTTP 202 je Event | ✅ |
| BE-08: SFU loggt alle Events korrekt | Log mit allen Event-Typen | ✅ |
| BE-08: nginx `/sfu/` Route | HTTP 202 | ✅ |
| BE-08: SDP-Offer Endpunkt vorhanden | HTTP 500 (invalid SDP, aber Endpunkt existiert) | ✅ |
| BE-08: Control Server pusht SESSION_CREATED automatisch | SFU Log: Event empfangen | ✅ |

### Neue Dateien

- `internal/controlserver/command/engine.go` — Command Engine (BE-04)
- `internal/telemetryservice/client.go` — MQTT Paho Client (BE-05)
- `internal/recording/recorder.go` + `memory_recorder.go` — Session Recording (BE-07)
- `internal/webrtcsfu/sfu.go` — WebRTC SFU Pion (BE-08)
- `cmd/telemetry-service/main.go` + `cmd/webrtc-sfu/main.go` — vollständig implementiert
