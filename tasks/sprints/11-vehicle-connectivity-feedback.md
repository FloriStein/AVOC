# Sprint 11 — Vehicle Connectivity & Feedback (ADR-021)

Abgeschlossen: 2026-06-11

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| VEH-01 | ADR-021 — Vehicle Connectivity & Feedback Architecture | L | ✅ `docs/adr/021-vehicle-connectivity-feedback.md` |
| VEH-02 | `proto/vehicle.proto` — VehicleCommandAck | S | ✅ header + command_event_id + received + received_at_ms |
| VEH-03 | `proto/telemetry.proto` — Actuation Fields 7–11 | S | ✅ steer/throttle/brake commanded + actual |
| VEH-04 | `internal/vehicleconnection/registry.go` — ForwardCommand | M | ✅ VehicleForwarder-Impl.; kritischer Gap geschlossen |
| VEH-05 | `internal/vehicleconnection/ackstore.go` — AckStore | S | ✅ Latest-ACK je vehicleID |
| VEH-06 | `internal/controlserver/command/engine.go` — VehicleForwarder | M | ✅ Interface + WithVehicleForwarder() |
| VEH-07 | `cmd/control-server/main.go` — ACK-Endpoint | M | ✅ `GET /vehicle/ack/latest/{vehicleID}` |
| VEH-08 | `cmd/vehicle-mock/main.go` — Docker-Fahrzeug-Simulation | L | ✅ JWT self-gen, WS, Protobuf, ACK, MQTT-Lerp 15% |
| VEH-09 | `vehicle-mock.Dockerfile` + Compose + nginx | M | ✅ `/vehicle/` Proxy, dev+prod Compose |
| VEH-10 | `useVehicleAck.ts` — Frontend Hook | S | ✅ 500ms-Polling |
| VEH-11 | `InputIndicatorPanel.tsx` — Lenkrad + ActuationBars + AckBadge | M | ✅ Im Footer integriert |
| VEH-12 | Tests: 7 Go + 7 TypeScript | M | ✅ 26/26 Go Unit + 41/41 Frontend grün |

### Neue/geänderte Dateien

- `proto/vehicle.proto` — VehicleCommandAck
- `proto/telemetry.proto` — Actuation Fields 7–11
- `internal/vehicleconnection/registry.go` — neu
- `internal/vehicleconnection/ackstore.go` — neu
- `internal/vehicleconnection/handler.go` — rewritten (register/unregister/readLoop)
- `internal/controlserver/command/engine.go` — VehicleForwarder Interface
- `cmd/control-server/main.go` — Registry/AckStore verdrahtet, ACK-Endpoint
- `cmd/vehicle-mock/main.go` — neu
- `cmd/telemetry-service/main.go` — Actuation Fields in JSON
- `infrastructure/docker/vehicle-mock.Dockerfile` — neu
- `infrastructure/compose/docker-compose.yml` — vehicle-mock Service
- `infrastructure/compose/docker-compose.prod.yml` — vehicle-mock Image
- `infrastructure/docker/nginx.conf` — `/vehicle/` + `/vehicle/ws` Proxy
- `frontend/src/hooks/useVehicleAck.ts` — neu
- `frontend/src/hooks/useTelemetry.ts` — Interface erweitert
- `frontend/src/components/InputIndicatorPanel.tsx` — neu
- `frontend/src/App.tsx` — useVehicleAck + InputIndicatorPanel
- `tests/unit/vehicleconnection_test.go` — neu
- `frontend/src/components/InputIndicatorPanel.test.tsx` — neu
- `docs/adr/021-vehicle-connectivity-feedback.md` — neu
- `docs/sprints/sprint-11-vehicle-connectivity.md` — neu
- `docs/sprints/sprint-11-verification.md` — neu

### Verification

E2E-Verification PASS: `steer_commanded=0.75 → steer_actual=0.6375 (Lerp 15%) → ACK <500ms`
Finding: `make up` startet Frontend lokal nicht ohne SSL-Cert (Sprint-10-Regression, kein Sprint-11-Bug)
