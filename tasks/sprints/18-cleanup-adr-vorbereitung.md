# Sprint 18 — Cleanup & ADR-Vorbereitung

Ziel: CI-Blocker beheben (`go vet` schlägt fehl durch veraltete `handler_test.go`), Vehicle-Heartbeat-Timestamp nachliefern (Sprint-14-Bonus), und Multi-Vehicle-Handover architektonisch klären (Grill-Me + ADR, kein Code).

Datum: 2026-06-18 | **Status: In Bearbeitung 🔄**
Vorgänger: Sprint 17 ✅

Grill-Me-Session: 2026-06-18 — Fokus Cleanup-Sprint; MV-12 vorerst stehen lassen; MV-11 erst ADR; AUTH-TEST-01 vollständig (20 Tests); kein externer Zeitdruck.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUTH-18-01 | `internal/authservice/handler_test.go` vollständig auf neue API migrieren — `User.ID int`, `Username`, `UserStore` Signaturen, Login-Feld `username`; alle 20 Szenarien | M | 🔲 |
| OBS-01 | Vehicle Heartbeat-Timestamp im `AckBadge` — "zuletzt gesehen vor Xs" aus `useVehicleAck` Hook | S | 🔲 |
| MV-11-ADR | Grill-Me-Session + ADR für Multi-Vehicle Handover-Isolation (`HandoverManager` nutzt noch globale State Machine für OPERATOR-Schicht) — kein Implementierungs-Code in diesem Sprint | L | 🔲 |
