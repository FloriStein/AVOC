# Sprint 31 — AP2-Vervollständigung + Fleet-/Observability-Nacharbeiten

Ziel: Fünf offene, bislang keinem Sprint zugeordnete Backlog-Tasks. Aus "AP2 — Web-Dashboard"
(Meilenstein 2, IBATOUR): `AP2-04` (Prioritätenmanagement in der Task-UI über reine
Zahlenanzeige hinaus — Sortierung, visuelle Hervorhebung). Aus "Fleet Dashboard Planung"
(TASKUI-Nachträge, Sprint 24, in Sprint 30 zurückgestellt): `TASKUI-03` (vollständige
Task-Status-Audit-Historie). Aus "Fleet Dashboard Planung" (Sprint-21-Nachträge): `FLEET-03`
(Klärung `control-server`-Vehicle-Endpoints vs. `fleet-service`-Admin-API — reine Dokuaufgabe,
kein Code), `FLEET-04` (`vehicle-mock` nutzt echte Zonen/Stationen statt hartcodierter
Demo-Koordinaten). Aus "Security & Observability" (Sprint 14): `OBS-01` (Vehicle
"zuletzt gesehen"-Heartbeat-Timestamp in AckBadge, Bonus).

**TASKUI-03 Typ-L-Klärung 2026-07-18:** `ADR-030` merkt eine echte `task_status_history`-Tabelle
explizit als "separates Folge-ADR" vor — nach CLAUDE.MD Abschnitt 1.1 damit Typ L (neue
Datenstruktur), zwingend Grill-Me-Session + ADR vor Umsetzung. Nutzer hat sich für sofortige
Grill-Me-Session in diesem Sprint entschieden (statt Zurückstellen). Ergebnis:
[ADR-032](../docs/adr/032-task-status-history.md) — additive neue Tabelle, `ADR-030` bleibt
unverändert gültig, Backfill bestehender Tasks beim Rollout (mit dokumentierten Näherungen für
`changed_at`/`from_status` bei nicht mehr exakt rekonstruierbaren Alt-Übergängen), neuer Endpoint
`GET /fleet/tasks/{id}/history`.

**FLEET-03 Klärung:** `ADR-029` beantwortet die Frage (Koexistenz, kein Bruch im aktuellen
Schritt, langfristig vermutlich Ablösung durch `fleet-service`-Admin-API) bereits vollständig —
reine Bestätigungsaufgabe ohne Code-Änderung, siehe Ergebnisse-Abschnitt.

**Keine Signaturänderung nach außen ohne Zweck, kein ungewollter Verhaltenswechsel** (CLAUDE.MD
Abschnitt 15) — Ausnahme sind `TASKUI-03` (neue Historie ist der Taskzweck) und `AP2-04` (neues
Sortier-/Hervorhebungsverhalten ist der Taskzweck); dort gegen die im Task beschriebene
Zielsemantik getestet, nicht auf Unverändertheit. Bei den Go-Tasks (`TASKUI-03`, `FLEET-04`,
`OBS-01`-Backend-Anteil) gilt zusätzlich `docs/go-style-guide.md` (Funktionslänge, Parameterzahl)
für neuen/geänderten Code.

Datum: 2026-07-18 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 30 ✅ (dieser Branch zweigt von `feature/fleet-service-foundation-cleanup30` ab,
NICHT von `feature/fleet-service-foundation` direkt, da `TASKUI-03`/`AP2-04`
`internal/fleetservice/store.go` bzw. `frontend/src/components/FleetTaskPanel.tsx` anfassen, die
Sprint 30 bereits verändert hat — Nil-Slice-Fix, serverseitig berechnetes
`allowed_transitions`-Feld statt `NEXT_TRANSITIONS`)
Branch: `feature/fleet-service-foundation-backlog31`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| FLEET-03 | Klärung `control-server`-Vehicle-Endpoints vs. `fleet-service`-Admin-API | S | ✅ |
| FLEET-04 | `vehicle-mock` nutzt echte Zonen/Stationen statt hartcodierter Demo-Koordinaten | S | ✅ |
| OBS-01 | Vehicle "zuletzt gesehen"-Heartbeat-Timestamp in AckBadge | S | ✅ |
| AP2-04 | Prioritätenmanagement in der Task-UI ausbauen | M | ✅ |
| TASKUI-03 | Vollständige Task-Status-Audit-Historie (ADR-032) | M | ✅ |

## Ergebnisse

**FLEET-03 — Klärung `control-server`-Vehicle-Endpoints vs. `fleet-service`-Admin-API ✅**
Reine Bestätigungsaufgabe, kein Code geändert. Verifiziert: `POST /vehicles` und
`DELETE /vehicles/{id}` existieren unverändert in `cmd/control-server/main.go:297-298`
(`handleVehiclesCreate`/`handleVehicleDelete`). `ADR-029` (Abschnitt "Schreibzuständigkeit") hat
die Frage bereits abschließend beantwortet: Koexistenz ist die aktuelle, bewusste Entscheidung
("kein Bruch in diesem Schritt, nur vorgemerkt") — `control-server` bleibt für
`vehicles.id`/`vehicles.display_name` (Auto-Register) zuständig, `fleet-service` für
`vehicle_type`/`description` sowie alle Fleet-Tabellen. Eine Ablösung der `control-server`-
Endpoints ist als spätere, nicht terminierte Möglichkeit dokumentiert, keine offene Frage in
diesem Sprint. Kein Folge-ADR nötig, da `ADR-029` diesen Fall bereits abdeckt.

**FLEET-04 — `vehicle-mock` nutzt echte Zonen/Stationen ✅**
Neue `cmd/vehicle-mock/fleet_stations.go`: `resolveSimulationStations()` ruft
`GET /fleet/stations` gegen `fleet-service` auf (Auth per selbst signiertem JWT, gleiches
Shared Secret wie die Telemetrie-Seite, ADR-004 — `RequireAuth` prüft nur die Signatur, keine
Rolle) und nutzt die ersten 2 Stationen mit gesetztem `position_lat/lon`. Fällt automatisch auf
die bisherigen hartcodierten Koordinaten (`fallbackDemoStations`, umbenannt aus `demoStations`)
zurück bei Netzwerkfehler, Non-200, kaputtem JSON oder weniger als 2 geo-verorteten Stationen —
`vehicle-mock` bleibt damit auch ohne laufenden `fleet-service` oder vor
`scripts/seed-fleet-demo.sh` funktionsfähig (Verhalten unverändert in diesem Fall). `docker-
compose.yml`: neue Env `FLEET_SERVICE_URL` (Default im Code: `http://fleet-service:8085`) +
`depends_on: fleet-service` für den `vehicle-mock`-Service; `tests/docker-compose.test.yml`
bewusst unverändert gelassen (fehlender `fleet-service` dort löst den bereits vorgesehenen
Fallback-Pfad aus, kein Test-Stack-Ausbau nötig). `fleet_simulator.go`: `fleetVehicleSimulator`
bekommt ein `stations []fleetStation`-Feld statt des globalen `demoStations`-Zugriffs (reines
Pure-Logic-Verhalten von `tick()` unverändert, nur die Datenquelle ist jetzt injiziert).
`go build`/`go vet`/`go test ./cmd/vehicle-mock/...` grün (15 Tests: 9 bestehende
Simulator-Tests nach Umbenennung unverändert grün, 6 neue Tests für
`resolveSimulationStations` — Erfolgsfall, Verbindungsfehler, <2 geo-verortete Stationen, leere
Liste (JSON `null`, ADR-029-Muster), Non-200, kaputtes JSON). Nicht gegen den echten Docker-Stack
verifiziert (keine Schema-/DB-Änderung, reiner HTTP-Client-Code — durch die httptest-Server-Tests
bereits realistisch abgedeckt).

**OBS-01 — Vehicle "zuletzt gesehen"-Heartbeat-Timestamp in AckBadge ✅**
Gefundener eigentlicher Kern des Bonus-Tasks: die bestehende `AckBadge`-Zeile "ACK vor Xs"
(`ack.ageSinceAckMs`, `useVehicleAck.ts`) beruht ausschließlich auf `VehicleCommandAck` — die
`AckStore` (`internal/vehicleconnection/ackstore.go`) aktualisiert sich nur, wenn der Operator
aktiv Kommandos sendet. Bei einem länger untätigen, aber weiterhin verbundenen Fahrzeug wird "ACK
vor Xs" fälschlich alt, obwohl das Fahrzeug durchgehend Telemetrie sendet (`telemetry-service`
liefert bereits `event.Header.GetTimestamp()` über `GET /telemetry/latest/{id}`, wurde vom
Frontend bisher aber verworfen). Fix: `useTelemetry.ts` übernimmt jetzt `timestamp` als
`timestampMs` + berechnet `ageSinceUpdateMs` (gleiches Zeitpunkt-der-Poll-Berechnungsmuster wie
`useVehicleAck.ts`s `ageSinceAckMs`, keine neue Abstraktion). `InputIndicatorPanel.tsx`:
`AckBadge` bekommt `telemetry`-Prop, neue Zeile "Zuletzt gesehen vor Xs" unterhalb von "ACK vor
Xs" (echtes Heartbeat-Signal, unabhängig vom Kommandofluss), gemeinsame `formatAge()`-Hilfsfunktion
für beide Zeilen extrahiert statt Duplikat. `npx tsc -b --noEmit` sauber (nach `make
proto-gen-ts`, in diesem frischen Worktree noch nicht generiert gewesen — unabhängig von diesem
Task, Docker-basierter Codegen-Schritt), `npx vitest run` komplett grün (259/259, keine
Regression), davon 4 neue Fälle in `InputIndicatorPanel.test.tsx`: Heartbeat sichtbar bei
vorhandenem Timestamp, nicht sichtbar bei `timestampMs=0`/`ageSinceUpdateMs=null`, nicht sichtbar
bei `telemetry=null`, korrekte Sekunden-Formatierung (≥1000ms).

**AP2-04 — Prioritätenmanagement in der Task-UI ausgebaut ✅**
Grill-Me-Rückfrage 2026-07-18 zum Schwellenwert (kein festes Prioritäts-Schema im System, freies
Integer-Feld ohne Min/Max): fester Wert `priority >= 5` als "hoch" gewählt (statt relativem
Top-Quartil, das denselben Wert je nach aktuell offenen Tasks mal hervorheben, mal nicht
hervorheben würde). `FleetTaskPanel.tsx`: neue `sortedTasks` (per `useMemo`, absteigend nach
`priority`, stabiler Sort erhält bei Gleichstand die vom Backend gelieferte
`created_at DESC`-Reihenfolge) ersetzt die Iteration über das rohe `tasks`-Prop. Hohe Priorität
(`>= HIGH_PRIORITY_THRESHOLD`) wird zweifach hervorgehoben: Zeilen-Rahmen/Hintergrund
(amber statt grau) sowie "Priorität N ⚠ Hoch" fett/amber in der Metazeile. `npx tsc -b --noEmit`
sauber, `npx vitest run` komplett grün (262/262, keine Regression), davon 3 neue Fälle in
`FleetTaskPanel.test.tsx`: Sortierreihenfolge unabhängig von der Prop-Reihenfolge, Hervorhebung
bei `priority=5` (Schwelle selbst), keine Hervorhebung bei `priority=4` (Grenzwert-Test knapp
unter der Schwelle). Kein Backend-Bruch — `priority` bleibt unverändert ein reines
Integer-Feld, Sortierung/Hervorhebung sind rein clientseitig.

**TASKUI-03 — Vollständige Task-Status-Audit-Historie ✅ ([ADR-032](../docs/adr/032-task-status-history.md))**
Typ L bestätigt (neue Tabelle = Datenstruktur-Änderung, `ADR-030` verweist selbst explizit auf ein
"separates Folge-ADR"), Grill-Me-Session + ADR-032 vor Umsetzung durchgeführt (siehe oben).
Implementierung in `internal/fleetservice/store.go`:
- Neue `task_status_history`-Tabelle (`id, task_id, from_status, to_status, changed_by,
  changed_at`), `task_id REFERENCES tasks(id) ON DELETE CASCADE` — additiv zu `ADR-030`,
  `tasks.status_changed_by` bleibt unverändert gepflegt.
- `UpdateTaskStatus` liest den echten vorherigen Status jetzt über eine `WITH previous AS (SELECT
  status FROM tasks WHERE id=$3) UPDATE ... FROM previous ...`-CTE — bleibt eine einzige atomare
  Anweisung (kein separates read-then-write, Race-Safety aus ADR-030 unverändert, siehe
  `TestUpdateTaskStatus_ConcurrentTransitions_ExactlyOneSucceeds`, weiterhin grün) — und schreibt
  danach über die neue `recordTaskStatusTransition`-Methode eine Historie-Zeile.
- `backfillTaskStatusHistory` läuft einmalig bei jedem Start (`NewPostgresFleetStore`, `NOT
  EXISTS`-Guard = idempotent, gleiches Muster wie `taskuiDemoSeedCleanup`): `changed_at` =
  `completed_at` falls vorhanden, sonst `created_at` (Näherung, dokumentierte Einschränkung);
  `from_status` nur gesetzt wo eindeutig herleitbar (`in_progress`←`pending`,
  `completed`←`in_progress`), bei `cancelled` bewusst `NULL` (Herkunft mehrdeutig).
- Neuer Endpoint `GET /fleet/tasks/{id}/history` (`handler.go`, `cmd/fleet-service/main.go`),
  404 bei unbekanntem Task, `[]` (nicht `null`) bei noch nie übergegangenem Task.
- `go build`/`go vet ./...` reposweit sauber.
- `go test ./internal/fleetservice/... -race` (gegen echten Postgres-Container, zweimal
  hintereinander gegen frischen Zustand gelaufen, CLAUDE.MD Abschnitt 17
  Flakiness-Anforderung): 6 neue Fälle — Historie-Eintrag mit korrektem from/to-Status,
  chronologische Reihenfolge bei mehreren Übergängen, leeres nicht-`null`-Array bei
  Pending-Task ohne Übergänge, 404 bei unbekanntem Task (Store- und Handler-Ebene), Backfill
  inkl. Idempotenz bei wiederholtem Start (isoliertes Schema, direkt eingefügte "Legacy"-Zeilen
  ohne `CreateTask`/`UpdateTaskStatus`). Alle 45 bestehenden Fleet-Service-Tests weiterhin grün
  (keine Verhaltensänderung an bestehenden Übergängen/Fehlercodes).
- **Gefundener und behobener Bug während der Verifikation:** die neue FK
  `task_status_history.task_id → tasks(id)` brach ohne `ON DELETE CASCADE` die
  Test-Cleanup-Routinen mehrerer bestehender Tests (`DELETE FROM tasks WHERE ...` schlug an der
  RESTRICT-Constraint fehl, sobald für den Task eine Historie-Zeile existierte) — mit `-race`
  reproduzierbar, ohne `-race` beim ersten Lauf nicht aufgefallen. `ON DELETE CASCADE` ergänzt
  (es gibt ohnehin keinen Produktions-Lösch-Endpoint für Tasks); danach zwei aufeinanderfolgende
  volle `-race`-Läufe grün.
- **Gegen den echten Docker-Test-Stack verifiziert** (`docker compose -f
  tests/docker-compose.test.yml up --build -d`, danach `down`): Tabelle korrekt inkl.
  `ON DELETE CASCADE` angelegt, voller Task-Lifecycle per `curl` durchgespielt (Task anlegen →
  leere Historie → zwei Übergänge → Historie zeigt beide chronologisch korrekt), 404 für
  unbekannten Task bestätigt, Backfill-Migration nach Container-Neustart mit direkt eingefügter
  "Legacy"-Zeile bestätigt (`from_status=in_progress`, `changed_at`≈`completed_at`), zweiter
  Neustart bestätigt Idempotenz (weiterhin genau 1 Zeile).
- **Bewusst nicht abgedeckt:** keine Nebenläufigkeits-/Race-Tests speziell für die
  Historie-Schreibung selbst (die zugrunde liegende `UpdateTaskStatus`-Atomarität ist bereits
  durch `TestUpdateTaskStatus_ConcurrentTransitions_ExactlyOneSucceeds` abgedeckt; der
  Historie-INSERT läuft nur nach einem bereits gewonnenen, exklusiven Übergang, hat also keinen
  eigenen Race-Fall).
