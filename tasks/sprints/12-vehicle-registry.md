# Sprint 12 — Vehicle Registry (ADR-022)

Abgeschlossen: 2026-06-12

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| VEH-REG-01 | ADR-022 — Vehicle Registry Architecture | L | ✅ `docs/adr/022-vehicle-registry.md` |
| VEH-REG-02 | `pkg/audit/sqlite_writer.go` — `DB() *sql.DB` getter | S | ✅ Shared WAL-Connection für vehicleregistry |
| VEH-REG-03 | `internal/vehicleregistry/` — VehicleStore + SQLiteVehicleStore + NoopVehicleStore | M | ✅ ErrNotFound-Sentinel; ConnectionChecker Interface |
| VEH-REG-04 | `cmd/control-server/main.go` — Store-Init + `GET/POST/DELETE /vehicles` | M | ✅ SeedDefault vehicle-001; korrekte Status-Codes |
| VEH-REG-05 | `frontend/src/lib/api-client.ts` + `useVehicles.ts` | S | ✅ `VehicleInfo`, `listVehicles()`, 2s-Polling |
| VEH-REG-06 | `frontend/src/hooks/useSession.ts` — `startSession(vehicleId)` | M | ✅ VEHICLE_ID-Hardcoding vollständig entfernt |
| VEH-REG-07 | `frontend/src/components/VehicleSelector.tsx` | M | ✅ Dropdown + Online-Indikator + "Session starten" |
| VEH-REG-08 | `SafetyPanel.tsx` + `ControlPanel.tsx` + `ConnectionPanel.tsx` + `App.tsx` | M | ✅ vehicleId-Prop-Chain; Auto-Start entfernt |
| VEH-REG-12 | Bugfix: `DELETE /vehicles/{id}` → 404 (ErrNotFound) | S | ✅ `RowsAffected()` Check + errors.Is() im Handler |
| VEH-REG-13 | Bugfix: `POST /vehicles` malformed JSON → "invalid JSON" (400) | S | ✅ Decode von Feldvalidierung getrennt |

### Neue/geänderte Dateien

- `pkg/audit/sqlite_writer.go` — `DB() *sql.DB` getter
- `internal/vehicleregistry/registry.go` — neu (Vehicle, VehicleStore Interface, ConnectionChecker, ErrNotFound)
- `internal/vehicleregistry/sqlite_store.go` — neu (SQLiteVehicleStore, vehicles-Tabelle, SeedDefault)
- `internal/vehicleregistry/noop_store.go` — neu (NoopVehicleStore)
- `cmd/control-server/main.go` — vehicleregistry import, Store-Init, 3 neue Endpoints
- `frontend/src/lib/api-client.ts` — VehicleInfo + listVehicles()
- `frontend/src/hooks/useVehicles.ts` — neu
- `frontend/src/hooks/useSession.ts` — startSession(vehicleId) statt startSessionIfNeeded()
- `frontend/src/components/VehicleSelector.tsx` — neu
- `frontend/src/components/SafetyPanel.tsx` — vehicleId prop
- `frontend/src/components/ControlPanel.tsx` — vehicleId prop
- `frontend/src/components/ConnectionPanel.tsx` — VehicleSelector + onStartSession prop
- `frontend/src/App.tsx` — Auto-Start entfernt, vehicleId-Prop-Chain
- `frontend/src/components/SafetyPanel.test.tsx` — vehicleId={null} ergänzt
- `frontend/src/components/ControlPanel.test.tsx` — vehicleId={null} ergänzt
- `docs/adr/022-vehicle-registry.md` — neu

### Verification

E2E-Verification PASS: vehicle-001 auto-geseedet · Online-Flag live aus WS-Registry · alle CRUD-Endpoints korrekte Status-Codes · SQL-Injection-Probe neutralisiert · Persistenz nach Neustart bestätigt
