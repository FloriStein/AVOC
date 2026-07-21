> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Sprint 47 — Multi-Cause-DEGRADED-Fundament in der State Machine (DRIFT-K3-TELEMETRY Teil 1)

**Freigabe (2026-07-20):** Nutzer wählt DRIFT-K3-TELEMETRY (Kritisch, ADR-009/`docs/drift-audit-2026-07.md`)
gegenüber DRIFT-K4/K5/K6 (CI-/Latenz-Testing-Debt) und den ADR-035-Folgeschritten. Typ L
(Kernsystem State Machine, Sicherheitsmodell) — Grill-Me-Session durchgeführt (2026-07-20), zwei
Architekturentscheidungen getroffen:

1. **Poll-basierter Watchdog in control-server** (nicht: telemetry-service meldet sich aktiv) —
   analog zu `SafetyBusWatchdog`/`AuthWatchdog`, kein neuer Kontrollfluss, `telemetry-service`
   bleibt unwissend über Sessions/Fahrzeuge.
2. **Multi-Cause-DEGRADED jetzt beheben, nicht zurückstellen** — `statemachine.Machine` trackt
   aktuell DEGRADED als einzelnen Zustand ohne Ursache; `TransitionMedia`s Recovery-Zweig
   (`MediaConnected` während `StateDegraded` → `StateConnected`) würde einen zweiten,
   unabhängigen DEGRADED-Trigger (Telemetrie) fälschlich mit aufheben, sobald sich nur das Video
   erholt, obwohl Telemetrie noch gestört ist. Bislang rein hypothetisch (nur eine
   DEGRADED-Quelle existiert produktiv), wird mit dem neuen Watchdog real. Fix: Set aktiver
   Degraded-Gründe statt Einzelzustand, Rückkehr zu CONNECTED nur wenn das Set leer ist.

**Dieser Sprint (47) baut nur das State-Machine-Fundament** — der eigentliche `TelemetryWatchdog`
(neues Package, Poll-Loop, Verdrahtung in `main.go`/`vehiclecontext.Registry`, neue
`TELEMETRY_SERVICE_URL`-Env-Var) ist **Sprint 48**, aufbauend auf diesem. Aufteilung bewusst, um
die sicherheitskritische State-Machine-Änderung isoliert zu verifizieren, bevor der Watchdog
darauf aufsetzt (analog zur Praxis, DRIFT-Themenblöcke einzeln statt vermischt zu behandeln).

**Vorrecherche (2026-07-20):**
- `internal/controlserver/statemachine/state.go` (216 Zeilen, ~100% Coverage über `tests/unit`):
  `TransitionMedia` ist die einzige Stelle, die DEGRADED aktuell betritt/verlässt — Eintritt bei
  `System == StateConnected`, Austritt bei `System == StateDegraded`. Beide Guards bewusst
  erhalten (SAFE_MODE hat Vorrang, ADR-009 Invariante 1: Media/Telemetrie lösen nie SAFE_MODE aus
  und heben es auch nie auf).
- `transitionSystemLocked`s `StateSafeMode`-Zweig setzt bereits `Control = ControlBlocked` — wird
  um das Leeren des neuen Degraded-Reason-Sets ergänzt (SAFE_MODE ist ein vollständiger Stopp
  unabhängig von der Ursache; nach Recovery prüft jeder Watchdog/Media-Poll unabhängig neu, ob
  sein Grund noch besteht, und tritt bei Bedarf erneut in DEGRADED ein — kein Datenverlust, nur
  kein Vorgriff auf einen möglicherweise inzwischen behobenen Zustand).
- `recoverFromSafeMode` (`transport/websocket.go:126`) geht explizit über
  `TransitionSystem(StateRecovering)`→`(StateAuthenticated)`→`TransitionToConnected()` — nicht
  über die neuen Degraded-Helper, unverändert in diesem Sprint.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| SM-01 | ADR-009 Update-Block: dokumentiert beide Grill-Me-Entscheidungen (Poll-Watchdog-Architektur, Multi-Cause-Set) + die Architekturskizze für Sprint 48 (damit Sprint 48 nicht erneut recherchieren muss). | S | 🔲 Backlog | — |
| SM-02 | `internal/controlserver/statemachine/state.go`: neuer Typ `DegradedReason` (`DegradedReasonMedia`, `DegradedReasonTelemetry` — Letzterer bereits jetzt definiert, auch wenn Sprint 48 ihn erst nutzt) + neues Feld `degradedReasons map[DegradedReason]bool` auf `Machine`. Private Helper `enterDegraded(reason)`/`exitDegraded(reason)` (müssen `m.mu` bereits gehalten sehen, analog `transitionSystemLocked`): `enterDegraded` fügt zum Set hinzu + transitioniert zu DEGRADED nur wenn `System == StateConnected` (Guard unverändert); `exitDegraded` entfernt aus dem Set + transitioniert zu CONNECTED nur wenn das Set danach leer UND `System == StateDegraded` ist. | M | 🔲 Backlog | — |
| SM-03 | `TransitionMedia` auf `enterDegraded(DegradedReasonMedia)`/`exitDegraded(DegradedReasonMedia)` umgestellt statt direktem `transitionSystemLocked`-Aufruf. Verhalten für den bestehenden Single-Cause-Fall (nur Media aktiv) bit-identisch zu vorher — reine interne Umleitung. | S | 🔲 Backlog | SM-02 |
| SM-04 | `transitionSystemLocked`s `StateSafeMode`-Fall leert `degradedReasons` (siehe Vorrecherche-Begründung oben). | S | 🔲 Backlog | SM-02 |
| SM-05 | Tests (neue `internal/controlserver/statemachine/state_test.go` — bisher kein eigenes internes Testfile, Coverage kam nur über `tests/unit`; hier gezielt die neue Multi-Cause-Logik direkt am `Machine` ohne Watchdog-Umweg): Media-DEGRADED→CONNECTED unverändert (Regressionsschutz), zwei simulierte Gründe gleichzeitig aktiv → Entfernen nur eines Grundes bleibt DEGRADED → Entfernen des zweiten wechselt zu CONNECTED, SAFE_MODE-Eintritt leert das Set (danach kein Auto-Recovery zu DEGRADED beim nächsten CONNECTED). | M | 🔲 Backlog | SM-02, SM-03, SM-04 |
| SM-06 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./internal/controlserver/... ./tests/unit/... -race -count=2` (bestehende `tests/unit`-Suite für `statemachine`/`safety`/`session` muss unverändert grün bleiben — reiner internes Umrouten, keine Verhaltensänderung für den bisherigen Single-Cause-Fall). `DECISIONS.MD`/`tasks/backlog.md`-Status-Update, Verweis auf Sprint 48 als direkten Folge-Sprint. | S | 🔲 Backlog | SM-01..05 |

**Nicht Teil dieses Sprints:** der `TelemetryWatchdog` selbst, `internal/controlserver/telemetrycheck`
(neues Package für den HTTP-Poll gegen `telemetry-service`), Verdrahtung in `vehiclecontext.Registry`/
`cmd/control-server/main.go`, neue `TELEMETRY_SERVICE_URL`-Env-Var, Schwellwert-Entscheidungen
(Poll-Interval/Fail-Threshold/"noch nie empfangen"-Handling) — alles Sprint 48.

**Geschätzter Umfang:** 6 Tasks, überwiegend S/M — innerhalb des ~200k-Token-Sprintbudgets.

---

Vorgänger: Sprint 46 ✅ (control-server: Hexagonal-Migration Vorbereitung — neues ADR-035 +
Testaufbau, siehe `tasks/sprints/46-control-server-hexagonal-migration-prep.md`)
