# Sprint 24 — Task-Management-UI

Ziel: Task-Management-UI im Fleet-Overview-Dashboard (AP2) — Aufgaben erstellen/zuweisen,
Status verfolgen/setzen, Priorität anzeigen, Task-Historie (= vollständige Task-Liste über alle
Status). Baut auf dem bereits fertigen `fleet-service`-Backend (Sprint 21) und dem Fleet-Overview-
Dashboard (Sprint 22) auf, die Task-Management bewusst ausgeklammert hatten (`GET/POST
/fleet/tasks` existierte, aber kein Status-Update-Mechanismus, kein UI).

Grill-Me-Session (2026-07-16) hat zwei echte Architekturentscheidungen ergeben, dokumentiert als
`ADR-030` statt stillschweigend umgesetzt:

- **Backend-Status-Endpoint wird ergänzt** (nicht nur Anzeige) — ohne Mechanismus wäre jeder Task
  für immer auf `"pending"` geblieben, "Status verfolgen" wäre wirkungslos gewesen.
- **Eigene, klar isolierte Demo-Stationen** (`-taskui`-Namensraum) statt auf die parallele
  Sprint-23-Session zu warten — bewusst akzeptiertes Merge-Risiko (siehe Ergebnisse unten: das
  Risiko ist real eingetreten, aber folgenlos geblieben, siehe TASK-14).
- Dritte Entscheidung während der Umsetzung: **Task-Historie = flache Liste + `status_changed_by`-
  Spalte**, kein separates Audit-Log über mehrere Übergänge (Scope-Entscheidung, s. `ADR-030`).

Ein Plan-Agent hat den Entwurf vor der Umsetzung gegengeprüft und einen echten, sonst unbemerkt
gebliebenen Bug im Entwurf gefunden: `vehicle_status.current_task_id` wurde von nichts genullt,
wenn ein Task terminal wird — ohne Fix hätte die Fahrzeug-Detailansicht dauerhaft einen längst
beendeten "Geister-Task" gezeigt. Mit `ADR-030` gefixt (TASK-03).

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 22 ✅ (Sprint 23 — Karten-/Zonen-Visualisierung — läuft parallel in einem
eigenen Worktree/Branch, siehe Ergebnisse unten)
Branch: `feature/fleet-service-foundation-taskui` (eigener Worktree
`controlcenter-aws-taskui`, Basis: `feature/fleet-service-foundation`)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| TASK-01 | `ADR-030`: Task-Status-Lifecycle & Status-Übergangs-Endpoint (Zustandsmaschine, Race-Safety, Demo-Seed-Entscheidung) | M | ✅ |
| TASK-02 | Schema-Erweiterung (`tasks.status_changed_by`) + idempotenter Demo-Seed (`demo-zone-taskui` + 2 Stationen) | S | ✅ |
| TASK-03 | `UpdateTaskStatus`-Store-Methode — atomares herkunftsbeschränktes UPDATE (race-safe), `current_task_id`-Clearing-Fix | M | ✅ |
| TASK-04 | Handler `PATCH /fleet/tasks/{id}/status` + Route + WS-Broadcast `task_status_changed` | M | ✅ |
| TASK-05 | Backend-Unit-Tests: Übergangsmatrix, Terminal-Status, Idempotenz, Nebenläufigkeit (`-race`), `current_task_id`-Clearing | M | ✅ |
| TASK-06 | Integrationstest (`tests/integration/fleet_service_test.go`) gegen echten Test-Stack | S | ✅ |
| TASK-07 | `api-client.ts`: `Task`/`Station`-Interfaces, `listFleetTasks`/`createFleetTask`/`updateFleetTaskStatus`/`listFleetStations` | S | ✅ |
| TASK-08 | `fleet-ws-events.ts`: `task_created` korrekt typisiert (war `unknown`), neuer `task_status_changed`-Event | S | ✅ |
| TASK-09 | `fleet-merge.ts`: `upsertTask`/`applyTaskStatusChanged` + Tests | S | ✅ |
| TASK-10 | `useFleetOverview.ts`: `tasks`-State, WS-Wiring, `createTask`/`updateTaskStatus`-Callbacks + Tests | M | ✅ |
| TASK-11 | `FleetTaskPanel.tsx` (neu): Liste/Anlage-Formular/Status-Buttons | L | ✅ |
| TASK-12 | `FleetOverview.tsx`-Integration (additive zweite Grid-Zeile) | S | ✅ |
| TASK-13 | Komponententests `FleetTaskPanel.test.tsx` | M | ✅ |
| TASK-14 | Verifikation gegen echten Dev-Stack (Backend voll, Frontend UI-Build+Route bestätigt, interaktiver Browser-Test dokumentierte Lücke) | M | ✅ |
| TASK-15 | `tasks/current-sprint.md` Sprint-24-Abschnitt | S | ✅ |
| TASK-16 | `DECISIONS.MD`/`docs/adr/README.md`/`CONTEXT.MD`/`docs/requirements.md`/`docs/architecture.md`/`tasks/backlog.md` nachgezogen | S | ✅ |

## Ergebnisse

**TASK-01 — ADR-030 ✅**
`docs/adr/030-task-status-lifecycle.md`: Zustandsmaschine `pending→in_progress→completed`/
`cancelled` (beide terminal, `pending→cancelled` explizit erlaubt — Stornieren eines nie
gestarteten Tasks). `PATCH /fleet/tasks/{id}/status` statt `POST .../acknowledge`-Vorbild gewählt
— näher an `PATCH /auth/users/{id}` (`ADR-024`), bewusste dokumentierte Abweichung. Neue Spalte
`tasks.status_changed_by` statt vollem Audit-Log (Scope-Entscheidung). `current_task_id`-Clearing
bei Terminal-Status als Nebeneffekt-Fix festgehalten.

**TASK-02/03 — Schema + Store-Methode ✅**
`status_changed_by TEXT` sowohl direkt im `CREATE TABLE` (Neuinstallation) als auch per
idempotentem `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (bereits laufende Dev-DB) — identisches
Muster zu `vehicleTypeColumn` aus `ADR-029`. `UpdateTaskStatus`: atomares `UPDATE ... WHERE
id=$1 AND status = ANY($2)` (kein read-then-write), bei 0 betroffenen Zeilen Follow-up-`SELECT`
zur 404-vs-409-Unterscheidung. Neue Sentinel-Errors `ErrTaskNotFound`/`ErrInvalidTransition`
(anders als `AcknowledgeAlert`, das nur einen Fehlerfall kennt). Demo-Seed (`demo-zone-taskui`,
`demo-station-a/b-taskui`) idempotent per `ON CONFLICT DO NOTHING` in `NewPostgresFleetStore`.

**TASK-04 — Handler + Route + Broadcast ✅**
`Handler.UpdateTaskStatus` mapt `ErrTaskNotFound`→404, `ErrInvalidTransition`→409, malformed
JSON/fehlende Felder→400 (Muster: `CreateTask`/`AcknowledgeAlert`). Neuer, schlanker
`TaskStatusChangedEvent`-Broadcast (nicht die volle Task-Zeile, Muster:
`AlertAcknowledgedEvent`).

**TASK-05 — Backend-Unit-Tests ✅**
`internal/fleetservice/handler_test.go`: volle Übergangs-Matrix inkl. beider Terminal-Status
(`TestUpdateTaskStatus_TerminalStates_RejectAnyFurtherTransition` prüft alle drei Zielstatus aus
beiden Terminal-Zuständen, nicht nur den "offensichtlichen" Rückwärtsfall), Idempotenz
(`TestUpdateTaskStatus_Idempotency_RepeatingSameTransition_SecondCallReturns409` — dokumentiert
bewusst abweichendes Verhalten zu `AcknowledgeAlert`, das Wiederholungen erlaubt: ein
Status-Übergang ist ein Einweg-Zustandswechsel, keine settable Value), `current_task_id`-Clearing
verifiziert. `internal/fleetservice/store_test.go`:
`TestUpdateTaskStatus_ConcurrentTransitions_ExactlyOneSucceeds` — 20 Goroutinen racen denselben
Übergang, exakt 1 Erfolg + 19 Konflikte, `-race`-sauber, 2x hintereinander gegen die echte
Dev-Postgres gelaufen (identisches Ergebnis, keine Flakiness). Volle `internal/fleetservice`-Suite
(inkl. aller Vorsprint-Tests) grün. Ein vorbestehender, unabhängiger Flake in
`internal/fleetgateway` (`TestMockGateway_Stop_StopsProducingEvents`, Timing-bedingt) — verifiziert
via `git status`/`git diff`, dass dieses Paket in diesem Branch nicht angefasst wurde, also nicht
Teil dieses Sprints.

**TASK-06 — Integrationstest ✅**
`TestIntegration_FleetService_UpdateTaskStatus_DeliversBroadcastAndPersists` (Task anlegen → PATCH
→ WS-Broadcast `task_status_changed` empfangen → `GET /fleet/tasks` bestätigt persistierten
Status) und `TestIntegration_FleetService_UpdateTaskStatus_InvalidTransition_Returns409` (frischer
Task direkt `pending→completed` → 409, beweist die Zustandsmaschine über den echten HTTP-Pfad, nicht
nur auf Store-Ebene). `make test-integration` (jetzt 27 Tests) 2x gegen frisch gestartete
Container gelaufen (`docker compose down`+`up` zwischen den Läufen, nicht nur `-count=1` auf
demselben Stand — sonst kollidieren feste Test-IDs wie `itg-rest-zone` mit sich selbst) — beide
Male grün.

**TASK-07/08/09 — Frontend-Basis ✅**
`api-client.ts`: `Task`/`Station`-Interfaces 1:1 aus dem Go-Modell (inkl. `status_changed_by?`).
`fleet-ws-events.ts`: `task_created` von `unknown` auf `data: Task` korrigiert (Sprint-22-Lücke
geschlossen), neuer `task_status_changed`-Zweig. `fleet-merge.ts`: `upsertTask` (Muster
`upsertAlert`, newest-first, idempotent), `applyTaskStatusChanged` (Muster
`applyAlertAcknowledged`, No-Op bei unbekannter ID). 27 neue Unit-Tests für `fleet-merge.ts`/
`fleet-ws-events.ts`, alle grün.

**TASK-10 — Hook-Erweiterung ✅**
`useFleetOverview.ts`: dritter Snapshot-Fetch (`listFleetTasks`), `task_created`/
`task_status_changed` jetzt echt verarbeitet (vorher tote No-Op-Zweige). `createTask` übernimmt
die REST-Antwort direkt (nicht erst über den WS-Roundtrip) — der später eintreffende
`task_created`-Broadcast wendet sich dank `upsertTask`s Idempotenz harmlos ein zweites Mal an.
`updateTaskStatus` wirft bei Fehlern (insb. 409) weiter, analog `acknowledgeAlert`. 12 neue
Hook-Tests (Snapshot-Laden, beide neuen Event-Typen, `createTask`/`updateTaskStatus` inkl.
Fehlerpfad und No-Op ohne `operatorId`).

**TASK-11/12/13 — UI-Komponente + Integration + Tests ✅**
`FleetTaskPanel.tsx` (neu): Liste (= Historie, alle Status) + Anlage-Formular (Fahrzeug-/
Stations-Dropdowns, gespeist aus `vehicles`-Prop + eigenem `listFleetStations`-Fetch, da Stationen
Referenzdaten ohne WS-Kanal sind — bewusst nicht in `useFleetOverview`s Live-Loop gemischt) +
Status-Buttons, die nur laut Client-seitigem Spiegel der `ADR-030`-Übergangsmatrix sichtbare
Ziele zeigen (dokumentierte Duplikation, s. `DECISIONS.MD`). Pro-Zeile-Pending-State (Muster
`FleetAlertsPanel`). `FleetOverview.tsx`: additiv als viertes Element im bestehenden
`grid-cols-3` (füllt per `col-span-full` automatisch eine neue Zeile) statt die Grid-Klasse
umzubauen — minimaler Diff, da die parallele Sprint-23-Session dieselbe Datei ebenfalls anfasst.
11 neue Komponententests (Leer-/Listenzustand, Formular-Validierung/-Fehler, Button-Sichtbarkeit
je Status inkl. beider Terminal-Status, Statuswechsel-Erfolg/-Fehler, Nebenläufigkeits-Isolation
zwischen zwei Task-Zeilen). Bestehende `FleetOverview.test.tsx` musste angepasst werden: die neue
`vehicle_id`-Dropdown-Option im Formular kollidierte mit einem bestehenden `getByText('V1')`
(mehrdeutig geworden) — auf `getByRole('button', {name: /V1/})` präzisiert, sowie alle
`useFleetOverview`-Mock-Rückgaben um `tasks`/`createTask`/`updateTaskStatus` ergänzt.

**Echter Bug im eigenen Verifikationsvorgehen gefunden:** `npx tsc --noEmit` (ohne `-b`) gegen das
Root-`tsconfig.json` (nur `references`, `files: []`) prüft de facto **nichts** — ein echter
Typfehler in `FleetTaskPanel.test.tsx` (Mock-Rückgabe `Promise<Task>` statt `Promise<void>`) wurde
dadurch erst beim `npm run build` (`tsc -b`) im Docker-Image-Build sichtbar, nicht vorher. Fix im
Test + für den Rest der Verifikation konsequent `npx tsc -b` (bzw. `npm run build`) statt `tsc
--noEmit` genutzt.

**TASK-14 — Verifikation gegen echten Dev-Stack ✅ (mit zwei offenen Punkten)**
Backend vollständig gegen den laufenden Dev-Stack (nicht nur Tests) verifiziert: `fleet-service`
neu gebaut, Demo-Seed sichtbar (`GET /fleet/stations`), Task angelegt (`POST /fleet/tasks`),
gültiger Übergang `pending→in_progress` (200), identischer Übergang wiederholt → korrekt 409,
echter WebSocket-Client (Node, kein Mock) im Docker-Netz verbunden → `task_status_changed`-Event
mit korrektem Payload (`completed_at`, `status_changed_by`) beim `in_progress→completed`-Übergang
empfangen. Zusätzlich explizit über **nginx** (Port 3000, derselbe Pfad wie das Frontend) statt
nur direkt gegen Port 8085 verifiziert — `PATCH` ist der erste PATCH-Verb-Endpoint in
`fleet-service`, No-Rewrite-Routing (`DASH-01`) hätte hier ebenfalls brechen können, tat es nicht.
Frontend: `tsc -b`/`eslint`/`vitest` (192 Tests)/`vite build` sauber, Docker-Image baut und
Container läuft.

Zwei Punkte bewusst offen dokumentiert, nicht stillschweigend übersprungen:

1. **Interaktiver Browser-Test nicht möglich:** Chrome-DevTools-MCP war durch die parallele
   Sprint-23-Session belegt (`chrome-profile` exklusiv gesperrt — bewusst nicht übernommen, um die
   andere Sitzung nicht zu unterbrechen), Firefox-DevTools-MCP schlug mit "Failed to read
   marionette port" fehl (kein Firefox-Binary in dieser Sandbox verfügbar) — dieselbe Art Lücke wie
   bereits in Sprint 22 (DASH-08, dort Playwright/Chromium) dokumentiert. Kompensiert durch die
   volle REST-/WS-Verifikation oben (Netzwerk-/Datenebene identisch zu dem, was ein Browser auch
   nutzen würde) plus 192 grüne Komponenten-/Hook-Tests inklusive genau der UI-Szenarien, die ein
   Klick-Test prüfen würde. Empfehlung: Folge-Verifikation durch den Nutzer im eigenen Browser oder
   in einer Folge-Session, sobald der Chrome-Profile-Konflikt nicht mehr besteht.

   **Update (Folge-Session, 2026-07-16, TASK-14b):** Chrome-DevTools-MCP war frei, interaktive
   Browser-Verifikation nachgeholt — und hat einen echten, durch keinen der 192 Tests abgedeckten
   Absturz gefunden: `GET /fleet/tasks` liefert bei null Zeilen JSON `null` statt `[]` (Go:
   `var tasks []Task` bleibt ein nil-Slice, `encoding/json` marshalt das als `null`), und
   `FleetTaskPanel.tsx`s `tasks.length`-Check crasht darauf ohne Error Boundary — reißt die
   komplette Fleet-Overview-Seite ab, nicht nur das Task-Panel. Unentdeckt geblieben, weil jeder
   bisherige Test (Unit/Integration/manuelle REST-Checks) vorher mindestens einen Task angelegt
   hatte — der Fall "Tasks-Tabelle ist leer" kam nie vor. Fix: `internal/fleetservice/store.go`
   `ListTasks` initialisiert jetzt `tasks := []Task{}` statt `var tasks []Task`. Regressionstest
   `TestListTasks_ReturnsEmptySliceNotNull_WhenNoTasksExist` (neu, `store_test.go`) nutzt das
   Fresh-Schema-Isolationsmuster aus `integration_test.go` (`DROP`/`CREATE SCHEMA` +
   `search_path`), da die geteilte `public`-Schema-Tabelle nie garantiert leer ist — geprüft beide
   JSON-Serialisierung (`[]`, nicht `null`) und `ListTasks() != nil`. 2x hintereinander grün, volle
   `internal/fleetservice`-Suite mit `-race` ebenfalls grün. Dieselbe nil-Slice-Machart existiert
   auch in `ListZones`/`ListStations`/`ListVehicleStatus`/`ListVehiclesWithStatus`/`ListAlerts` —
   dort bisher folgenlos (Demo-Seed sorgt dafür, dass diese Listen nie leer sind), aber ein
   latentes Risiko; bewusst nicht mitgefixt (außerhalb des Scopes dieses Sprints), siehe
   `tasks/backlog.md`.

   **Zweiter, unabhängiger Befund während derselben Verifikation:** der geteilte
   `docker-compose.yml` trägt `name: avoc` — jeder Worktree (Haupt-Checkout, Sprint-23-Worktree,
   dieser `-taskui`-Worktree) landet dadurch im selben Compose-Projekt und denselben Containern,
   nicht in isolierten Kopien. Der zu Beginn dieser Folge-Session laufende `avoc-frontend-1`-
   Container enthielt `FleetMap` (Sprint 23), aber kein `FleetTaskPanel` — vermutlich von der
   parallelen Sprint-23-Session zwischenzeitlich mit deren eigenem `docker compose up --build`
   überschrieben. Um den gemeinsamen Container nicht ein weiteres Mal zu überschreiben, wurde
   stattdessen mit einem isolierten `vite`-Dev-Server (Port 5183) plus einer isolierten lokalen
   `fleet-service`-Instanz (Port 18086, über einen temporären `socat`-Proxy an dieselbe Postgres
   angebunden) verifiziert — beide danach sauber gestoppt, der Verifikations-Task in der
   Postgres-Tabelle blieb als bekannter, trivial identifizierbarer Rest zurück (kein Cleanup-
   Zugriff auf den Container erteilt). `frontend/vite.config.ts` bekam dabei dauerhaft einen
   fehlenden `/fleet`- und `/fleet/ws`-Proxy-Eintrag (Port 8085) ergänzt — dieselbe Lücke wie
   DASH-01s nginx-Routing, nur nie in die Vite-Dev-Config nachgezogen; ohne diesen Eintrag kann
   `npm run dev` gegen Fleet-Endpunkte grundsätzlich nicht funktionieren, unabhängig vom
   Container-Namensrisiko. Das Container-Namensrisiko selbst bleibt ungelöst (`name: avoc` in
   `infrastructure/compose/docker-compose.yml` ist über alle Worktrees hinweg identisch) — als
   Backlog-Punkt vorgemerkt, siehe `tasks/backlog.md`.

2. **Doppelte Demo-Stationsanlage real eingetreten:** die parallele Sprint-23-Session hat
   unabhängig bereits eine eigene, deutlich reichhaltigere Zone (`zone-betriebshof-nord`, echte
   SVG-Karte + `geo_bounds`, 5 benannte Stationen inkl. GPS-Koordinaten) angelegt — ca. 90 Sekunden
   vor diesem Sprints Seed. Kein technischer Konflikt (dank `-taskui`-Namensraum, `ADR-030`
   akzeptiertes Risiko trat wie erwartet ein, blieb aber folgenlos), aber beim späteren Merge der
   beiden Branches sollte entschieden werden, welche Stationsdaten die "echten" werden — die
   `demo-*-taskui`-Zeilen sind bewusst trivial identifizierbar/löschbar gehalten.

**Wichtiger Vorfall während der Verifikation, transparent dokumentiert:** der erste
`docker compose up --build` -Versuch für `fleet-service`/`frontend` hat `auth-service`,
`control-server` und `fleet-service` im gemeinsamen Dev-Stack kurzzeitig zum Absturz gebracht
(`JWT_SECRET environment variable is required`) — dieser Worktree (`controlcenter-aws-taskui`,
per `git worktree add` neu angelegt) hatte keine eigene `.env`-Datei (gitignored, wird von `git
worktree` nicht mitkopiert). Sofort erkannt und behoben: `.env` aus dem Haupt-Checkout kopiert
(reine lokale Secrets-Datei, kein Git-Tracking betroffen) und alle drei Services mit `--env-file
.env` neu gestartet — Ausfallzeit für den gemeinsam genutzten Dev-Stack ca. 3 Minuten. Kein
Datenverlust (Postgres/Mosquitto/vehicle-mock liefen währenddessen unverändert weiter). Für künftige
Worktree-Sessions in diesem Projekt vorgemerkt: `docker compose up` aus einem neuen Worktree heraus
braucht entweder eine eigene `.env`-Kopie oder ein explizites `--env-file <Pfad-zum-Haupt-Checkout>/.env`
*bevor* Services (neu-)gestartet werden, nicht danach.

**TASK-15/16 — Dokumentation ✅**
Dieser Abschnitt sowie `DECISIONS.MD`, `docs/adr/README.md`, `CONTEXT.MD` (jeweils ADR-030-Zeile +
zwei neue Folge-Entscheidungen: Übergangstabellen-Duplikation Backend/Frontend,
begrenzte Audit-Historie), `docs/requirements.md` (Task-Modell um Status-Lifecycle ergänzt) auf
Nutzeranfrage zwischendurch aktualisiert. Dabei einen vorbestehenden, unabhängigen Fehler in
`docs/architecture.md` gefunden und behoben: die Container-Services-Tabelle listete
`fleet-service`/`vehicle-mock` seit Sprint 21 gar nicht (nicht durch diesen Sprint verursacht,
opportunistisch mitkorrigiert). Bewusst nicht angefasst: `CONTEXT.MD`s Glossar/Domänenmodell und
`architecture.md`s Fließtext enthalten seit dem Fleet-Pivot (`ADR-027/028/029`) noch keine
Fleet-Begriffe (Zone/Station/Task) — deutlich größere, vorbestehende Lücke über drei Sprints
hinweg, die vermutlich mit der parallelen Sprint-23-Session kollidieren würde; explizit geflaggt,
nicht übernommen.

**Bewusst nicht in diesem Sprint:** vollständige Task-Audit-Historie über mehrere Übergänge
(Scope-Entscheidung, `ADR-030`), Auflösung der Backend/Frontend-Übergangstabellen-Duplikation
(Backlog-Folge-Task), Bereinigung der doppelten Demo-Stationsanlage (Merge-Zeitpunkt-Entscheidung),
automatische Fahrzeug-Rückmeldung für Task-Status (wartet auf AP1-Workshop, `ADR-027`).
