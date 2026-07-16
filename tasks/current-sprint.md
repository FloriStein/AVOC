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


---

# Sprint 25 — Audio-Benachrichtigungen

Ziel: Audio-Benachrichtigung bei neuen Fleet-Alerts ergänzen — die ursprüngliche
Notification-Anforderungsliste war in Sprint 22 (`FleetAlertsPanel.tsx`) nur zu drei Vierteln
umgesetzt (Echtzeit-Zustellung, Severity-Farbcodierung, Acknowledgment), Sound fehlte komplett.
Eigener Worktree/Branch (`feature/fleet-service-foundation-audio`, abgezweigt von Sprint 22s
Commit), da parallel weitere Sessions an Sprint 23 (Karten-/Zonen-Visualisierung) und Sprint 24
(Task-Management-UI) auf eigenen Branches arbeiten — Zusammenführen aller drei Branches übernimmt
der Nutzer später.

Grill-Me (2026-07-16), drei Fragen, alle mit der jeweils empfohlenen Option beantwortet:

- **Severity-Scope:** Ton nur für `warning`/`critical`, `info` bleibt stumm (zu niedrigschwellig
  für einen Ton in einer 24/7-Leitstelle).
- **Mute-Toggle:** wird bereits in diesem Sprint mitgebaut (nicht zurückgestellt) — im
  24/7-Dauerbetrieb-Kontext ohne Mute-Option müssten Operatoren den Ton dauerhaft ertragen.
- **Soundquelle:** synthetischer Ton per Web Audio API (`AudioContext`/`OscillatorNode`), kein
  Audio-Asset — keine Lizenzfrage, keine zusätzliche Bundle-Größe.

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 22 ✅ (dieser Branch zweigt von Sprint 22s Commit ab, unabhängig von Sprint
23/24, die auf eigenen Parallel-Branches laufen)
Branch: `feature/fleet-service-foundation-audio`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUDIO-01 | Grill-Me-Session (Severity-Scope, Mute-Toggle-Scope, Soundquelle) | S | ✅ |
| AUDIO-02 | `fleet-alert-sound.ts` — reine Logik (Severity-Filter, Debounce/Throttle-Prädikat, Tonerzeugung über Minimal-Interface statt vollem `AudioContext`-Typ) | S | ✅ |
| AUDIO-03 | `useFleetAlertSound.ts` — Hook: Baseline-Erkennung (kein Ton beim initialen REST-Snapshot), Mute-State inkl. `localStorage`-Persistenz, `AudioContext`-Lazy-Creation mit Fehlerbehandlung | M | ✅ |
| AUDIO-04 | `FleetAlertsPanel.tsx`/`FleetOverview.tsx` — Mute-Toggle-Button in der Alerts-Panel-Kopfzeile verdrahtet | S | ✅ |
| AUDIO-05 | Tests: `fleet-alert-sound.test.ts`, `useFleetAlertSound.test.ts`, `FleetAlertsPanel.test.tsx` erweitert | M | ✅ |
| AUDIO-06 | Verifikation: `tsc -b`/`eslint`/`vite build`/`vitest run` (2× hintereinander) + Browser-MCP | S | ✅ |

## Ergebnisse

**AUDIO-01 — Grill-Me ✅**
Drei offene Punkte aus der Aufgabenstellung per `AskUserQuestion` geklärt (siehe oben). Vierter und
fünfter Punkt (Autoplay-Policy, Sound-Storm) waren bereits als Implementierungsvorgaben klar, keine
echten Entscheidungsfragen.

**AUDIO-02 — fleet-alert-sound.ts ✅**
Neues Modul, analog zu `fleet-merge.ts`/`fleet-map.ts` reine Funktionen von React/WebSocket/DOM
getrennt: `isAudibleSeverity` (warning/critical), `shouldPlayNow(lastPlayedAtMs, nowMs,
minIntervalMs = 2000)` (reines Throttle-Prädikat, kein eigener Timer), `playAlertTone(ctx)`
(synthetischer Zwei-Ton-Piepton mit Gain-Hüllkurve gegen Knack-Artefakte). `playAlertTone` nimmt
absichtlich ein selbst definiertes `ToneAudioContext`-Minimal-Interface statt des vollen DOM-Typs
entgegen — jsdom (Testumgebung) implementiert die Web Audio API nicht, ein echter `AudioContext`
erfüllt das schlankere Interface strukturell, ein Test-Fake genügt für die Tests.

**AUDIO-03 — useFleetAlertSound.ts ✅**
Bewusst als eigener, dedizierter Hook (nicht in `useFleetOverview.ts` integriert) — hält die
Datenmerge-Logik (DASH-05) und die Sound-Seiteneffekt-Logik getrennt, keine Änderung an den 45
bestehenden DASH-08-Tests nötig. "Neu" heißt: eine Alert-id, die der Hook noch nie gesehen hat.
Die Baseline wird an den Übergang `loading: true → false` gekoppelt, nicht an den ersten
Hook-Aufruf — `alerts` ist beim Mount zunächst `[]` (State-Default in `useFleetOverview.ts`), erst
nach dem REST-Fetch die echte Liste; ohne diese Unterscheidung hätte der Übergang von `[]` auf die
echte Liste selbst wie "lauter neue Alerts" ausgesehen und beim Öffnen des Dashboards ein
Sound-Feuerwerk für alle bereits bestehenden, unquittierten Alerts ausgelöst — genau das von der
Aufgabenstellung benannte Risiko. Danach löst jede zusätzliche id einen Ton aus, bewusst nicht nur
exakt beim `alert_created`-WS-Event, sondern für jede neu auftauchende id in der von
`useFleetOverview` gelieferten Liste — deckt zusätzlich den Resync-nach-Reconnect-Fall ab
(`Hub.Broadcast` hat kein Backlog, siehe DASH-05): ein Alert, der während eines WS-Aussetzers
entstand und erst durch den Resync sichtbar wird, ist für den Operator genauso neu und relevant.

Mute-State über `localStorage` (`fleet-alert-sound-muted`) persistiert, `try/catch` um
`getItem`/`setItem` (privater Modus/deaktiviertes `localStorage` darf nicht crashen, Fallback
"nicht stumm"). `AudioContext` wird lazy und nur einmal pro Hook-Instanz erzeugt (`undefined` =
noch nicht versucht, `null` = Erzeugung fehlgeschlagen/nicht verfügbar); Konstruktor-Aufruf und
`playAlertTone` beide in `try/catch` — ein blockierter/fehlender `AudioContext` darf das Dashboard
nicht crashen, sondern lässt den Ton einfach ausfallen. `resume()?.catch(() => {})` fängt eine
mögliche Promise-Rejection ab (Browser-Autoplay-Policy), bevor der eigentliche Ton gespielt wird.
Mehrere neue hörbare Alerts im selben Update sowie mehrere Updates innerhalb von 2s lösen nur
einen Ton aus (Sound-Storm-Schutz über `shouldPlayNow`).

**AUDIO-04 — UI-Integration ✅**
Mute-Toggle-Button in der Kopfzeile von `FleetAlertsPanel.tsx` ("Ton an"/"Stumm",
`aria-pressed`-Attribut). `FleetOverview.tsx` ruft `useFleetAlertSound(alerts, loading)` auf und
reicht `muted`/`toggleMuted` durch — kein neuer globaler State, keine Context-Einführung für ein
einzelnes Panel.

**AUDIO-05/06 — Tests + Verifikation ✅**
28 neue/erweiterte Tests: `fleet-alert-sound.test.ts` (14, reine Funktionen — Grenzwerte für
`shouldPlayNow` inkl. exaktem Intervall-Grenzwert und Uhr-Anomalie mit `nowMs` vor
`lastPlayedAtMs`; `playAlertTone` inkl. Fehlerpfad, wenn ein Node-Aufruf wirft), 13 in
`useFleetAlertSound.test.ts` (Baseline-Erkennung ohne Ton trotz vorhandener Alerts, echter neuer
Ton, info bleibt stumm, Mute unterdrückt Ton, Persistenz über einen zweiten Hook-Mount,
Sound-Storm-Schutz sowohl innerhalb eines Updates als auch über zwei Updates hinweg,
Idempotenz bei erneut gesehener id, `AudioContext` fehlt/wirft/`playAlertTone` wirft — jeweils
kein Crash, `localStorage.getItem` wirft — kein Crash), 1 neuer Test in `FleetAlertsPanel.test.tsx`
für den Mute-Button. Bestehende `FleetAlertsPanel.test.tsx`-Tests um die neuen Pflicht-Props
(`muted`/`onToggleMuted`) ergänzt, keine Verhaltensänderung. `FleetOverview.test.tsx` unverändert
grün — nutzt den echten (nicht gemockten) `useFleetAlertSound`-Hook, der ohne `AudioContext` in
jsdom sauber auf `null` degradiert.

Alle 194 Frontend-Tests (davon 28 neu/geändert) zweimal hintereinander grün, keine Flakiness.
`tsc -b`, `eslint .` (nur bereits vor diesem Sprint bestehende, unveränderte Warnungen in
`StreamSenderPanel.tsx`/`UserManagementPanel.tsx`/`src/gen/*`) und `vite build` sauber.

**Browser-Verifikation (Chrome-DevTools-MCP):** isolierter lokaler `npm run dev` (Port 5175) gegen
den bereits laufenden, geteilten Docker-Dev-Stack (temporärer `/fleet`-Proxy-Eintrag in
`vite.config.ts`, nach der Verifikation wieder entfernt — bewusst kein Rebuild/Redeploy des
geteilten `avoc-frontend-1`-Containers, um Sprint 23/24 in parallelen Sessions nicht zu stören).
Initialer Dashboard-Load mit ~50 bereits bestehenden, unquittierten `critical`-Alerts blieb still
(Baseline-Logik greift, kein Sound-Feuerwerk beim Öffnen — die zentrale Sorge der Aufgabenstellung).
Danach vier echte Alerts über MQTT (`fleet/{vehicle_id}/alert`) eingespielt und per WS live im
Dashboard beobachtet: alle vier kamen korrekt und in der richtigen Reihenfolge an, die Konsole
blieb über die gesamte Session fehlerfrei (keine Errors, keine unhandled promise rejections), der
Mute-Button war durchgehend korrekt beschriftet/klickbar. Der erste, sauber instrumentierte
Durchlauf hat den kompletten Mechanismus gegen echte Backend-Daten bewiesen: `AudioContext` wurde
genau einmal erzeugt und wiederverwendet, `osc.start()` lief für zwei tatsächlich neue Alerts (mein
Testalert plus ein unabhängiger echter FLEET-07-Schwellwert-Alert 6s später, beide außerhalb des
2000ms-Debounce-Fensters zueinander — beide Töne korrekt). Bei den drei folgenden Testalerts wurde
über meine eigene Ad-hoc-Instrumentierung (wiederholtes Patchen von `window.AudioContext` über
separate `evaluate_script`-Aufrufe der DevTools-Protocol-Grenze hinweg) keine weitere
Oscillator-Aktivität mehr erfasst — nach mehreren Diagnoseversuchen (Re-Patching, globale
Error-/Rejection-Listener, React-Fiber-Introspektion) konnte die Ursache nicht abschließend geklärt
werden. Bewertung: sehr wahrscheinlich ein Artefakt der Instrumentierung selbst, kein Produktfehler
— es gab in keinem der vier Durchläufe einen Konsolenfehler, die UI/State-Schicht funktionierte
jedes Mal einwandfrei, und die exakt gleiche Debounce-/Neu-Erkennungs-Logik ist bereits
deterministisch und vollständig durch die 13 Unit-Tests in `useFleetAlertSound.test.ts` (u. a. mit
Fake-Timern und gemocktem `playAlertTone`) abgedeckt, die als maßgebliche Nachweisquelle für dieses
Verhalten gelten. Lokaler Dev-Server und temporärer Proxy-Eintrag wurden nach der Verifikation
entfernt.

**Umgebungsnotiz (kein Bug):** frisch angelegter Worktree hatte weder `node_modules`
(`.gitignore`, `npm install` frisch durchgeführt) noch `src/gen/*_pb.ts` (`.gitignore`,
`protoc`-generiert — `protoc` in dieser Sandbox nicht installiert, stattdessen die bereits im
Haupt-Checkout generierten Dateien 1:1 herüberkopiert, identischer Inhalt). Der Production-Build
enthält erwartungsgemäß kein `leaflet` (240 KB statt der 396 KB aus Sprint 23) — dieser Worktree
zweigt vom letzten *committeten* Stand (Sprint 22) ab, Sprint 23s Karten-Arbeit liegt bisher nur
uncommitted im Haupt-Checkout und ist in einem frischen Worktree korrekt nicht enthalten. Kein
Bezug zu diesem Sprint, keine Regression.

**Bewusst nicht in diesem Sprint:** kein konfigurierbarer Ton (Lautstärke/Tonhöhe), keine
Snooze-Funktion über die reine Mute-Persistenz hinaus, kein separater Ton je Severity-Stufe (ein
einheitlicher Ton für warning/critical) — alles Folge-Tasks, falls konkret gebraucht.

---

# Sprint 23 — Outdoor Karten-/Zonen-Visualisierung im Fleet Overview

Ziel: Eine SVG-Karte mit Live-Fahrzeugpositionen im Fleet-Overview-Dashboard, aufbauend auf
ADR-029s Datenmodell. Backend (`GET/POST /fleet/zones`, `GET/POST /fleet/stations`) existiert seit
Sprint 21 (FLEET-05), war im Frontend bisher komplett ungenutzt (Sprint 22 hat bewusst keine
Karte gebaut). Grill-Me (2026-07-16) hat den Scope auf **Outdoor-Zonen** begrenzt — Fahrzeuge
haben für Indoor-Positionierung keine `position_x/y` (nur `position_zone_id`), eine echte
Datenmodell-Lücke, die eine Backend-Erweiterung bräuchte und auf einen Folge-Sprint verschoben
wird. Format von `svg_geometry`/`geo_bounds` (bisher in ADR-029 nur als "wird als Beispiel neu
erstellt" umschrieben) wurde in diesem Sprint verbindlich definiert und dokumentiert (ADR-029-
Update, MAP-02). Bewusst frontend-lastiger Sprint — kein neuer Go-Code außer für das
Demo-Seed-Skript, das die bereits bestehenden REST-CRUD-Endpoints nutzt.

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 22 ✅
Branch: `feature/fleet-service-foundation` (unverändert fortgeführt)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MAP-01 | Demo-Seed-Skript (`scripts/seed-fleet-demo.sh`) — 1 Outdoor-Zone + 5 Stationen über bestehende REST-API, Koordinaten passend zu `vehicle-mock`s hartcodiertem Demo-Pfad | M | ✅ |
| MAP-02 | ADR-029 datiertes Update (`svg_geometry`/`geo_bounds`-Format) + `DECISIONS.MD`/Sprint-23-Skelett | S | ✅ |
| MAP-03 | `api-client.ts`: `Zone`/`Station`-Interfaces + `listFleetZones`/`listFleetStations` | S | ✅ |
| MAP-04 | `leaflet`+`react-leaflet`-Dependencies, `fleet-map.ts` (reine Helper-Funktionen) | M | ✅ |
| MAP-05 | `FleetMap.tsx` — SVG-Zonen-Overlay, Stations-Marker, Live-Fahrzeug-Marker, Empty-State | L | ✅ |
| MAP-06 | `useFleetZones.ts` — einmaliger REST-Fetch (kein WS, dokumentiert) | S | ✅ |
| MAP-07 | `FleetOverview.tsx`-Integration (Karten-Zeile über unverändertem 3-Spalten-Grid) | M | ✅ |
| MAP-08 | Tests reine Funktionen (`fleet-map.test.ts`) | M | ✅ |
| MAP-09 | Tests `FleetMap.tsx` (jsdom/Leaflet-Spike + Komponententests) | M | ✅ |
| MAP-10 | Tests `useFleetZones.ts` | S | ✅ |
| MAP-11 | Verifikation gegen echten Dev-Stack + Browser-MCP, Ergebnis dokumentiert | S | ✅ |

## Ergebnisse

**MAP-01 — Demo-Seed-Skript ✅**
`scripts/seed-fleet-demo.sh` (neu, bash+curl+python3 für JSON-Encoding, kein `jq` im Repo): legt
Zone `Betriebshof Nord` (`zone-betriebshof-nord`, outdoor, `geo_bounds` umschließt
`vehicle-mock`s hartcodierten Demo-Pfad `52.130100,11.640100`–`52.130500,11.641200` mit Marge) +
5 Stationen an (zwei davon exakt deckungsgleich mit `vehicle-mock`s `demo-station-a/b`, drei
weitere plausible Punkte außerhalb des simulierten Pfads — dokumentierte, akzeptierte Lücke).
Idempotent (prüft `GET /fleet/zones` auf existierende Zonen-ID vor dem Anlegen, Exit 0 statt
Fehler bei Wiederholung — verifiziert: zweiter Lauf gegen den echten Dev-Stack meldet korrekt
"already exists", legt nichts doppelt an).

Login via `POST /auth/operator/login` (Admin-Credentials aus `ADMIN_PASSWORD`-Env-Var, Default
`admin_dev_secret`), `BASE_URL` überschreibbar (Default `http://localhost:3000`, durch nginx).
`geo_bounds` wird als JSON-**String** (nicht verschachteltes Objekt) gesendet — `Zone.GeoBounds`
ist Go `*string`, ein verschachteltes JSON-Objekt im Request-Body hätte `json.Decode` zum
Scheitern gebracht (siehe ADR-029-Update, MAP-02). Nach dem Seed wird `GET /fleet/zones` erneut
abgefragt und `geo_bounds` per `json.loads` re-geparst, um das Round-Tripping real zu bestätigen,
nicht nur anzunehmen.

**Echten Bug beim ersten Lauf gefunden:** `GET /fleet/zones` lieferte bei leerer Tabelle JSON
`null` statt `[]` (Go: `var zones []Zone` bleibt `nil` bei 0 Zeilen, `json.Marshal` rendert das
als `null`) — das Idempotenz-Check-Skript crashte beim ersten Durchlauf
(`TypeError: 'NoneType' object is not iterable`), da eine leere Zonen-Liste vorher nie real
abgefragt worden war. Fix im Skript (`zones = json.load(sys.stdin) or []`) und zusätzlich
defensiv in `api-client.ts`s `listFleetZones`/`listFleetStations` (`?? []`), damit `FleetMap.tsx`
nie gegen `null.length` crasht. Kein Backend-Fix in diesem Sprint (siehe `DECISIONS.MD`, Zeile zu
`GET /fleet/zones`/`/fleet/stations` — betrifft potenziell auch `ListVehiclesWithStatus`/
`ListAlerts`, dort aber bisher folgenlos, da nie leer abgefragt).

Gegen den echten laufenden Dev-Stack verifiziert (nicht nur angenommen): Skript zweimal
hintereinander ausgeführt (erster Lauf legt an, zweiter Lauf idempotent kein-op), `GET
/fleet/stations` per `curl` zeigt alle 5 Stationen mit korrekten Koordinaten inkl. deutscher
Umlaute (`Verwaltungsgebäude`, korrekt UTF-8/JSON-escaped).

**MAP-02 — ADR-029-Update + Doku ✅**
`docs/adr/029-fleet-vehicle-data-model.md`: datierter Update-Block nach dem
"Kartendarstellung"-Abschnitt angehängt (kein Überschreiben, CLAUDE.MD Abschnitt 6) —
Format-Definition für `svg_geometry`/`geo_bounds`, der Doppel-JSON-Encoding-Gotcha, der
`null`-statt-`[]`-Fund, und die Outdoor-only-Scope-Entscheidung. `DECISIONS.MD`: ADR-029-Zeile um
Update-Hinweis ergänzt, zwei neue Zeilen in "Offene Folge-Entscheidungen" (Indoor-Fahrzeugposition-
Lücke, `null`-statt-`[]`-Fund).

---
**MAP-04 — Dependencies + fleet-map.ts ✅**
`leaflet@^1.9.4`, `react-leaflet@^4.2.1` (React-18-kompatibel, v5 bräuchte React 19),
`@types/leaflet` (Dev) installiert. `frontend/src/lib/fleet-map.ts` (neu): `parseGeoBounds`
(nie werfend, `null` bei fehlendem/kaputtem JSON/falscher Shape/nicht-endlichen Zahlen),
`parseSvgGeometry` (via `DOMParser`, auch unter jsdom nutzbar), `autonomyMarkerColor`,
`vehiclesWithPosition`. `AUTONOMY_DOT` aus `FleetVehicleList.tsx` hierher verschoben (reiner
Import-Change, bestehende Tests unverändert grün) — eine Farbquelle für Listen-Punkte und
Karten-Marker statt Duplikation.

**Echtes Environment-Problem während MAP-04 gefunden:** `node_modules/@types` (und, wie sich beim
Reparaturversuch herausstellte, große Teile von `node_modules` insgesamt) gehörten `root`, nicht
dem Nutzer — `npm install -D @types/leaflet` scheiterte mit `EACCES`. Ein einfacher `rm -rf
node_modules` lief nur teilweise durch (root-eigene Dateien blieben stehen, inkonsistenter
Zwischenzustand). Nutzer hat `sudo rm -rf node_modules && npm install` selbst im Terminal
ausgeführt (`sudo` erforderte eine interaktive Passworteingabe, die aus dieser Session heraus nicht
möglich war) — danach sauber, `@types/leaflet` ließ sich installieren.

**MAP-05 — FleetMap.tsx ✅**
Neue Komponente `frontend/src/components/FleetMap.tsx`. Kein `<TileLayer>` (ADR-029: keine
externen Kartenkacheln) — nur `MapContainer` mit Bounds aus allen Zonen mit gültigem
`geo_bounds`. `svg_geometry` wird nicht über react-leaflets JSX-`SVGOverlay` gerendert (der
erwartet React-Children, nicht rohes Markup), sondern über Leaflets Core-API `L.svgOverlay`
(seit 1.0, kein Plugin nötig), imperativ via `useMap()`+`useEffect` mit explizitem Cleanup
(`map.removeLayer`) — unter `StrictMode` (aktiv in `main.tsx`) sonst doppelte Layer im
Dev-Modus. Eine Zone, die `parseGeoBounds`/`parseSvgGeometry` nicht besteht, wird übersprungen
(`console.warn`, kein Crash) statt die ganze Karte zu blockieren. Stations-Marker als
`L.divIcon`-Quadrate, Fahrzeug-Marker als `L.divIcon`-Kreise (Farbe über `autonomyMarkerColor`,
ausgewähltes Fahrzeug mit `ring-2 ring-white`), Klick ruft dieselbe `onSelectVehicle`-Callback wie
`FleetVehicleList`, sodass Auswahl über Karte oder Liste denselben `FleetVehicleDetail`-State
treibt.

**MAP-06 — useFleetZones.ts ✅**
Einmaliger REST-Fetch (`Promise.all([listFleetZones, listFleetStations])`), kein WS — `FLEET-06`s
Broadcast-Hub kennt keine Zonen-/Stations-Events. Bewusst als Entscheidung dokumentiert (Kommentar
im Code), nicht als übersehene Lücke.

**MAP-07 — FleetOverview.tsx-Integration ✅**
Bestehendes `grid grid-cols-3 gap-4` unverändert gelassen (minimiert Risiko für die 45
bestehenden DASH-07/08-Tests), neue `FleetMap` in einer Zeile darüber
(`h-[45vh] min-h-80 shrink-0`). Alle 167 bestehenden Frontend-Tests bleiben grün nach der
Änderung — verifiziert vor dem Weiterbauen, nicht erst am Sprintende.

**MAP-08/09/10 — Tests ✅**
33 neue Tests: `fleet-map.test.ts` (19, reine Funktionen — Grenzwerte für `parseGeoBounds`
inkl. eines echten Edge Cases mit `1e400`, das als gültiges JSON-Zahlenliteral zu `Infinity`
overflowt; `parseSvgGeometry`; `autonomyMarkerColor`; `vehiclesWithPosition` inkl.
`vehicle-001`-Fall), `FleetMap.test.tsx` (8, Komponententests inkl. Klick-Interaktion),
`useFleetZones.test.ts` (6, REST-Erfolg/-Fehler/leere Antwort/Unmount-Guard). Alle zweimal
hintereinander gelaufen, keine Flakiness. `tsc -b`/`eslint`/`vite build` sauber, 200 Tests
insgesamt grün.

**MAP-09 — jsdom/Leaflet-Spike-Ergebnis: voller Erfolg, kein Fallback nötig.** Vor dem
eigentlichen Testfile wurde geprüft, ob `MapContainer` unter dem bestehenden jsdom-Setup
überhaupt mountet (kein `ResizeObserver`-Polyfill vorhanden) — es mountete sauber, ohne
Konsolenwarnungen, **kein** Polyfill in `test/setup.ts` nötig. Anders als im Plan als Risiko
vorgesehen, ließ sich sogar die Klick-Interaktion auf einem Fahrzeug-Marker
(`fireEvent.click(screen.getByTitle(...))` → `onSelectVehicle`) direkt unter jsdom testen — der
im Plan vorgesehene Fallback ("nur renders-without-throwing, Rest auf MAP-11 verschieben") war
nicht nötig.

**MAP-11 — Verifikation: vollständig abgeschlossen ✅**

**Update 2026-07-16 (Fortsetzung) — interaktiver Browser-Test durchgeführt, alle 5 Prüfpunkte
bestanden.** `chrome-devtools-mcp` war in dieser Session (nach dem in der vorherigen Session
bereits eingetragenen `--isolated`-Flag, siehe unten) sofort ohne Blockade nutzbar — ein
`ToolSearch` nach den Tool-Namen lieferte direkt funktionsfähige Schemas, `new_page` gegen
`http://localhost:3000` verband ohne "browser is already running"-Fehler, obwohl zeitgleich
mehrere andere `chrome-devtools-mcp`/`firefox-devtools-mcp`-Prozesse aus parallelen Sessions auf
derselben Maschine liefen (`ps aux` zeigte drei separate Prozessgruppen, je mit eigenem isolierten
Profil). Das bestätigt: der MCP-Reconnect, der in der letzten Session noch ausstand, hat
stattgefunden, `--isolated` löst das Mehrfach-Instanzen-Problem wie erwartet.

**Vorbedingung Dev-Stack:** Beim Sessionstart liefen nur die beiden `vehicle-mock`-Container;
Frontend/Backend-Services waren seit 4 Minuten `Exited (0)`/`Exited (2)` (vermutlich durch einen
`docker compose down` oder Neustart zwischen den Sessions). Mit `docker compose ... up -d` (ohne
`--build`, um den in der letzten Session bereits gefixten Layer-Cache-Zustand nicht erneut zu
riskieren) neu gestartet — alle Container liefen danach `Up`/`healthy`. Frontend-Bundle erneut
stichprobenartig gegen den bekannten Leaflet-Bug geprüft: `index-CA_ZWEK1.js` enthält `leaflet`,
388 KB (konsistent mit dem in der letzten Session verifizierten ~396-KB-Bundle, keine Regression
auf den 244-KB-Bug). `scripts/seed-fleet-demo.sh` erneut ausgeführt — Zone `zone-betriebshof-nord`
existierte bereits (idempotent bestätigt, drittes Mal insgesamt).

Konkret geprüft (Login: `admin`/`admin_dev_secret`, `http://localhost:3000`):

1. **Login → Fleet Overview ohne Konsolenfehler:** erfolgreich, `list_console_messages` zeigte
   ausschließlich zwei vorbestehende Accessibility-Hinweise vom Login-Formular (`No label
   associated with a form field`, `A form field element should have an id or name attribute` —
   beide bereits vor MAP-01 im Login-Formular vorhanden, außerhalb des Sprint-23-Scopes, nicht neu
   eingeführt), keine `error`/`warn`-Einträge.
2. **Karte zeigt Zonen-Umriss + 5 Stations-Marker:** Screenshot bestätigt gestrichelten
   Zonen-Rahmen (`Betriebshof Nord`) mit drei benannten Gebäude-Rechtecken ("Halle 1", "Halle 2",
   "Ladezone") — das sind `<text>`-Elemente aus dem Zonen-`svg_geometry` selbst (Hallenplan-Grafik
   aus dem Seed-Skript, Zeilen 57–61), **nicht** die Stations-Marker. Die eigentlichen
   Stations-Marker (kleine `10×10px`-Quadrate, `stationIcon()` in `FleetMap.tsx:106`) sind separat
   sichtbar; über die Accessibility-Snapshot-Buttonzahl (7 = 5 Stationen + 2 Fahrzeuge) verifiziert,
   dass alle 5 Stationen aus dem Seed (`Ladezone A/B`, `Wartungsbereich`, `Verwaltungsgebäude`,
   `Einfahrtstor`) tatsächlich gerendert werden, nicht nur die drei mit sichtbarem Gebäude-Label.
   Auf den ersten Blick sah die Diskrepanz zwischen den drei großen beschrifteten Rechtecken und
   den kleinen unbeschrifteten Quadraten wie ein potenzieller Leaflet-CSS-Bug aus (Verdacht laut
   Aufgabenstellung) — durch Code-Lesen (`FleetMap.tsx`, `scripts/seed-fleet-demo.sh`) als
   beabsichtigtes Verhalten bestätigt, kein Bug.
3. **Simulierte Fahrzeuge als farbige Kreis-Marker, sichtbare Bewegung:** zwei Screenshots im
   Abstand mehrerer Sekunden zeigen messbar unterschiedliche Marker-Positionen für
   `lastenrad-01`/`lastenzug-01` sowie sinkende Batteriewerte (85→82 %, 87→86 % zwischen den ersten
   beiden Aufnahmen). Live-Update-Mechanismus zusätzlich explizit verifiziert (nicht nur vermutet):
   ein per `evaluate_script`-`initScript` injizierter `WebSocket`-Proxy protokollierte nach einem
   Seiten-Reload `WS_OPEN_ATTEMPT`/`WS_OPEN_SUCCESS` für
   `ws://localhost:3000/fleet/ws?token=...` — bestätigt, dass die Bewegung über den echten
   FLEET-06-WS-Broadcast läuft, nicht über Polling. (Nebenbefund: `list_network_requests` des
   `chrome-devtools-mcp`-Tools zeigt WS-Handshakes generell nicht in seiner Liste an, auch nicht
   mit `resourceTypes: ["websocket"]` — eine Werkzeug-Einschränkung, kein App-Verhalten; die
   REST-Aufrufe `GET /fleet/vehicles|zones|stations|alerts` erscheinen dort korrekt mit Status 200.)
4. **Klick auf Fahrzeug-Marker aktualisiert Detail-Panel:** zweimal verifiziert (vor und nach
   Reload). Klick auf den `lastenrad-01`-Kreis-Marker öffnet dasselbe `FleetVehicleDetail`-Panel
   wie ein Klick in der Liste (Batterie/Autonomie-Modus/Teleoperate-Button), und die
   Fahrzeugliste hebt den gleichen Eintrag lila hervor — bestätigt den gemeinsamen
   `onSelectVehicle`-State aus MAP-05. Ein Klick auf einen Stations-Marker (kein
   `onSelectVehicle`-Handler in `FleetMap.tsx`) ändert das Detail-Panel erwartungsgemäß nicht.
5. **Konsole fehlerfrei:** über die gesamte Interaktion (Login, Kartenrendering, zwei
   Marker-Klicks, ein Reload mit WS-Proxy-Injection) traten zu keinem Zeitpunkt `WS_ERROR`/
   `WS_CLOSED` oder sonstige Konsolenfehler auf — nur die zwei vorbestehenden, unveränderten
   Login-Formular-Hinweise aus Punkt 1.

Kein neuer Bug gefunden. Der in der letzten Session dokumentierte Docker-Layer-Cache-Bug (MAP-11,
vorheriger Absatz) ist nicht erneut aufgetreten (Bundle weiterhin korrekt mit Leaflet). Damit ist
Sprint 23 vollständig abgeschlossen: 200 Frontend-Tests (davon 33 neu, MAP-08/09/10) plus der
jetzt nachgeholte interaktive Browser-Test decken sowohl die reine Funktions-/Komponentenebene als
auch das tatsächliche Rendering/Live-Verhalten im echten Browser ab.

**DoD-Nachtrag (CLAUDE.MD Abschnitt 11) — `docs/architecture.md` nachgezogen:** Bei der
Abschlussprüfung fiel auf, dass `docs/architecture.md` `fleet-service` seit dessen Einführung in
Sprint 21 an keiner Stelle dokumentierte (weder REST-API noch WS-Broadcast noch, jetzt in Sprint
23, `FleetMap.tsx`) — eine seit zwei Sprints bestehende, nicht durch Sprint 23 verursachte Lücke.
Auf Nutzerrückfrage ("ist der Sprint vollends abgeschlossen") nachgezogen statt offen gelassen:
neuer Abschnitt "Fleet System (ADR-027/028/029, Sprint 21–23)" (Datenmodell, FleetGateway-
Abstraktion, REST-Endpoint-Tabelle, WS-Broadcast, Frontend-Komponenten/Hooks-Tabelle), Container-
Architecture-Tabelle um `fleet-service`/`vehicle-mock` ergänzt, Projekt-Verzeichnisstruktur um
`cmd/fleet-service`, `cmd/vehicle-mock`, `internal/fleetservice`, `internal/fleetgateway` sowie die
neuen Frontend-Komponenten/Hooks/Lib-Dateien ergänzt.

---

**MAP-11 — Frühere Sessions: Backend-/Build-Ebene verifiziert, Browser-Test noch offen (historisch,
durch obigen Absatz abgelöst).**

Backend-/Build-Ebene vollständig verifiziert:
- `seed-fleet-demo.sh` erneut gegen den Dev-Stack gelaufen (idempotent, Zone bereits vorhanden).
- Frontend-Container neu gebaut und deployed. Dabei ein **echtes, umgebungsbedingtes Problem
  gefunden**: ein zwischenzeitlicher Rebuild des `frontend`-Images (durch einen parallelen
  Prozess außerhalb dieser Session, vermutlich `make up --build`) lieferte ein Bundle **ohne**
  `leaflet` (244 KB statt der erwarteten ~396 KB) — obwohl der Quellcode zu diesem Zeitpunkt
  bereits alle MAP-04/05-Änderungen enthielt. Ursache nicht abschließend geklärt (vermutlich eine
  inkonsistente Docker-Layer-Cache-Wiederverwendung), aber real reproduzierbar beobachtet
  (`docker history` zeigte Layer aus unterschiedlichen Build-Zeitpunkten gemischt). Fix: Rebuild
  mit `--no-cache` (`docker build --no-cache -f infrastructure/docker/frontend.Dockerfile ...`)
  erzeugte wieder das korrekte ~396-KB-Bundle; verifiziert sowohl direkt im Image
  (`docker run ... grep -c leaflet`) als auch nach Redeploy über `curl` gegen
  `http://localhost:3000`. **Für zukünftige Deployments vermerkt:** bei unerklärlich kleinen/
  fehlenden Bundles zuerst `--no-cache` probieren, nicht dem Cache vertrauen.
- `docker compose ... build` (ohne `--pull=false`) schlug wegen fehlendem Netzwerkzugriff auf
  `registry-1.docker.io` (DNS-Timeout) fehl — Umgebungseinschränkung dieser Sandbox, umgangen über
  direktes `docker build --pull=false` gegen bereits lokal vorhandene Base-Images.

**Ehemals "bewusst nicht abgeschlossen" (historisch):** der interaktive Browser-Test war zum
Zeitpunkt dieses Absatzes noch offen, weil `chrome-devtools`-/`firefox-devtools`-MCP durch
parallele Sessions blockiert waren und der zum Fix eingetragene `--isolated`-Flag noch einen
MCP-Reconnect brauchte. Dieser Reconnect hat inzwischen stattgefunden — siehe den Absatz "Update
2026-07-16 (Fortsetzung)" oben: der Browser-Test wurde nachgeholt und ist vollständig
abgeschlossen, alle 5 Prüfpunkte bestanden, kein neuer Bug.

---

# Sprint 22 — Fleet Overview Dashboard

Ziel: Erste Slice des Web-Dashboards (AP2) — eine Fleet-Overview-Seite (Fahrzeugliste,
Status-/Detail-Panel, Alerts) gegen das in Sprint 21 fertiggestellte `fleet-service`-Backend.
Bewusst kein Karten-/Zonen-Rendering, kein Task-Management-UI, kein Dark-Mode-Umschalter, keine
System-Steuerung/-Override — reine Fahrzeugübersicht als erste Slice, analog zu Sprint 21s
Aufteilung in kleine, einzeln abnehmbare Tasks. Grill-Me (2026-07-15) hat zwei echte
Architekturentscheidungen ergeben, beide bereits als datierte ADR-Ergänzungen dokumentiert statt
nur implizit umgesetzt:

- Live-Updates nutzen den echten WS-Broadcast (`GET /fleet/ws`, FLEET-06) statt Polling —
  konsistent mit `ADR-028`s eigenem Ziel (Live-Updates ohne Polling für Multi-Workstation-Betrieb).
- Der "Teleoperate"-Button darf zusätzlich zum Notfall-Trigger auch **proaktiv** auf jedem
  Fahrzeug ohne aktiven Operator genutzt werden — dokumentiert als `ADR-028`-Update
  (2026-07-15) statt stillschweigend über die ursprüngliche Notfall-only-Regel hinaus gebaut.

Datum: 2026-07-15 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 21 ✅
Branch: `feature/fleet-service-foundation` (unverändert fortgeführt)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| DASH-01 | nginx-Routing für `/fleet/` REST + `/fleet/ws` (dev + prod), Docker-DNS-`resolver`-Muster, **kein** Prefix-Stripping (anders als `/api/`) | S | ✅ |
| DASH-02 | `docker-compose.yml`: `frontend` `depends_on: fleet-service` | S | ✅ |
| DASH-03 | `api-client.ts`: `listFleetVehicles`/`listFleetAlerts`/`acknowledgeFleetAlert` + `FleetVehicle`/`FleetAlert`-Interfaces | S | ✅ |
| DASH-04 | `fleet-ws-client.ts` + `fleet-ws-events.ts` — JSON-WS-Client mit typisiertem `FleetWSEvent`, Backoff-Reconnect | M | ✅ |
| DASH-05 | `fleet-merge.ts` (reine Merge-Funktionen) + `useFleetOverview.ts` (REST-Snapshot + WS-Deltas) | L | ✅ |
| DASH-06 | `useActiveSessions.ts` (Extraktion aus `App.tsx`) + `App.tsx` View-Switching (`FleetOverview` vs. Cockpit, gated auf `sessionId`) | M | ✅ |
| DASH-07 | `FleetOverview.tsx` + `FleetVehicleList.tsx` + `FleetVehicleDetail.tsx` + `FleetAlertsPanel.tsx` (ADR-028-Gating: Teleoperate nur ohne aktiven Operator, sonst Beobachten) | L | ✅ |
| DASH-08 | Tests: reine Merge-/Parsing-Funktionen, Hook-Tests, Komponenten-Tests, `App.test.tsx` (neu) — 45 neue Tests; manuelle Verifikation gegen laufenden Dev-Stack | M | ✅ |

## Ergebnisse

**DASH-01/02 — Infrastruktur ✅**
`infrastructure/docker/nginx.dev.conf` und `nginx.conf` (Prod, für Parität) bekamen je zwei neue
`location`-Blöcke — `/fleet/` (REST, `proxy_pass` ohne Rewrite, da `fleet-service`s Mux die Pfade
bereits wörtlich als `/fleet/...` registriert, anders als `/api/` → `control-server`, das den
Prefix strippt) und `/fleet/ws` (WS-Upgrade, gleiches Muster wie `/ws`/`/vehicle/ws`). Ein
Rewrite hier hätte jede Anfrage 404en lassen — der mit Abstand größte Stolperstein dieser
Aufgabe, gegen den echten Stack verifiziert (siehe DASH-08-Ergebnis unten), nicht nur
angenommen. `docker-compose.yml`: `frontend` bekam `fleet-service` als zusätzliches
`depends_on`.

**DASH-03 — REST-Client ✅**
`api-client.ts` um `FleetVehicle`/`FleetAlert`-Interfaces und drei Funktionen erweitert, statt
eine neue Datei zu eröffnen — die bestehende Datei mischt bereits mehrere Backend-Services in
einer Datei (Auth/Control-Server/User-Management als eigene Abschnitte), drei Fleet-Funktionen
rechtfertigen noch keinen Split. Feld-Optionalität 1:1 aus `internal/fleetservice/store.go`
übernommen — `FleetVehicle.autonomy_mode` ist optional (Go: `*string, omitempty`), während das
strukturell ähnliche `VehicleStatus.autonomy_mode` aus dem WS-Kanal **nicht** optional ist. Diese
Asymmetrie ist real und wurde bewusst nicht "vereinheitlicht".

**DASH-04 — Fleet-WS-Client ✅**
`fleet-ws-events.ts` (reine Parsing-Funktion `parseFleetWSMessage`, kein `WebSocket`-Bezug — wirft
nie, kaputtes JSON/fehlendes `type`/`data` kollabiert zu einem expliziten `'unknown'`-Tag statt
den Union-Typ auf `string` aufzuweiten) plus `fleet-ws-client.ts` (`FleetWSClient`, an
`ws-client.ts`s Form angelehnt, aber JSON statt Protobuf). Reconnect-Backoff lebt bewusst
**innerhalb** des Clients (anders als bei `ws-client.ts`, wo `useSession.ts` das wegen der sich
ändernden `sessionId` extern übernimmt) — der Fleet-WS hat nur einen stabilen Verbindungsparameter
(Token), daher ist selbstständiges Reconnect hier einfacher. Bestehende `FE_WS_CONNECTED`/
`FE_WS_RECONNECT`-Logkonstanten wiederverwendet, keine neuen nötig.

**DASH-05 — Merge-Logik + Hook ✅**
`fleet-merge.ts`: `mergeVehicleStatus` überschreibt nur im Event tatsächlich vorhandene Felder
(**nicht-destruktives Overlay**, Grill-Me-Entscheidung) — ein `vehicle_status`-Event, das z. B.
kurzzeitig kein GPS liefert (Go `omitempty`), blendet den zuletzt bekannten Wert nicht auf `—`
aus. Kein Treffer für die `vehicle_id` lässt die Liste unverändert (dokumentierte Lücke, keine
erfundene Teilzeile). `useFleetOverview.ts`: REST-Snapshot zuerst, dann WS-Deltas; **Resync bei
jedem Reconnect nach dem ersten** (`Hub.Broadcast` im Backend hat kein Backlog/Replay — ohne
Resync bliebe das Dashboard nach einem kurzen WS-Aussetzer dauerhaft veraltet); `error` wird
explizit gesetzt statt wie `useVehicles.ts` still den alten Stand zu behalten (Fleet Overview ist
Primärinhalt, keine Randspalte); `acknowledgeAlert` aktualisiert optimistisch lokal und wirft bei
REST-Fehlern weiter (sichtbares Feedback für die bewusste Nutzeraktion).

**DASH-06 — Extraktion + View-Switching ✅**
`useActiveSessions.ts` aus `App.tsx`s bisher inline liegendem `GET /api/sessions`-Polling
extrahiert (reiner Refactor, kein Verhaltensunterschied für `AppContent` — der
Session-Restore-nach-Reload-Effekt nutzt jetzt `hasPolled` als State statt als Ref, was ihn sogar
korrekter macht: er reagiert jetzt auch wirklich auf den Zeitpunkt, an dem `hasPolled` kippt,
statt sich indirekt auf einen gleichzeitigen `activeSessions`-Update-Tick zu verlassen). `App.tsx`
route jetzt dreistufig ohne Router (kein Router nötig für 2 Post-Login-Views): kein Token →
`LoginPanel`; Token ohne `sessionId` → `FleetOverview` (neue Landing-View); `sessionId` gesetzt →
unverändertes Cockpit (`AppContent`).

**DASH-07 — Komponenten ✅**
`FleetOverview.tsx` (Container) verknüpft `fleet-service`-Daten mit `control-server`s
`activeSessions` clientseitig (ADR-029: "Frontend führt Services clientseitig zusammen") — hier,
nicht im Backend, wird bestimmt, ob ein Fahrzeug einen aktiven Operator hat. `FleetVehicleList.tsx`
(Autonomie-Status-Punkt, gestrichelt/grau für Fahrzeuge ohne jegliche Statusdaten — deckt
`vehicle-001` ab, das nie eine `vehicle_type`/Status-Meldung bekommen hat), `FleetVehicleDetail.tsx`
(ADR-028-Gating: aktiver Operator → **kein** Teleoperate-Button, stattdessen Badge + Beobachten-
Button, der denselben `session.startSession`-Fluss nutzt wie `ConnectionPanel`s bestehendes
`onJoinSession`; kein aktiver Operator → Teleoperate-Button, außer bei OBSERVER-Rolle), `FleetAlertsPanel.tsx`
(Severity-farbig, Bestätigen-Status pro Zeile statt global — ein hängender Acknowledge-Request
für Alert A darf den Button für Alert B nicht sperren).

**DASH-08 — Tests + Verifikation ✅**
45 neue Tests (reine Funktionen `fleet-merge.test.ts`/`fleet-ws-events.test.ts` — Grenzwerte wie
kaputtes JSON, fehlendes `type`-Feld, kein Fahrzeug-Treffer beim Merge, Idempotenz bei
Alert-Upsert; Hook-Tests `useActiveSessions.test.ts`/`useFleetOverview.test.ts` mit gemocktem
`FleetWSClient` — Reconnect-Resync, REST-Fehlerpfad, optimistisches Acknowledge inkl.
Fehlerfall; Komponententests inkl. des `vehicle-001`-Falls (alle Felder `undefined`, darf nicht
crashen/`NaN` anzeigen) und der wichtigsten Einzel-Assertion dieser Aufgabe — Teleoperate-Button
ist **abwesend**, nicht nur disabled, sobald ein aktiver Operator existiert; `App.test.tsx`, neu,
als Regressionswächter für die DASH-06-Routing-Entscheidung). Alle 167 Tests des gesamten
Frontends (nicht nur die neuen) grün, `tsc --noEmit`/`eslint`/`vite build` sauber.

Gegen den echten laufenden Dev-Stack verifiziert (Container neu gebaut, nicht nur kompiliert):
`GET /fleet/vehicles`/`/fleet/alerts` über `http://localhost:3000/fleet/...` (durch nginx, mit
echtem JWT) liefert echte Daten inkl. `vehicle-001` (nur `id`/`display_name`, alles andere fehlt
tatsächlich) und der simulierten `lastenzug-01`/`lastenrad-01`; `POST
/fleet/alerts/{id}/acknowledge` durch nginx bestätigt (204); ein echter WebSocket-Client (Node,
kein Mock) gegen `ws://localhost:3000/fleet/ws?token=...` empfing binnen Sekunden mehrere echte
`vehicle_status`-Events aus der laufenden `vehicle-mock`-Simulation — bestätigt sowohl die
No-Rewrite-Entscheidung (DASH-01) als auch, dass die reale Backend-JSON-Form zu den TS-Interfaces
passt. Testdaten aus früheren Ad-hoc-Verifikationsläufen (`mqtt-test-*`) aus der Dev-DB bereinigt.

**Bewusst nicht vollständig verifiziert:** ein echter interaktiver Browser-Klick-Test (Playwright)
war in dieser Sandbox-Umgebung nicht möglich — kein zur installierten Playwright-Version
passender Chromium-Build ladbar (`ERROR: Playwright does not support chromium on
ubuntu26.04-x64`), kein Workaround gefunden. Das ist eine echte, offene Lücke, keine stillschweigend
übersprungene: das eigentliche Rendering/State-Management ist über die 167 Komponenten-/Hook-Tests
abgedeckt (inklusive exakt der Szenarien, die ein Klick-Test auch prüfen würde — Teleoperate-Button-
Gating, `vehicle-001`-Rendering), und die komplette Netzwerk-/Datenebene (nginx-Routing, REST, WS,
reale JSON-Formen) wurde end-to-end gegen die echten Container verifiziert — nur der letzte Schritt
"echter Browser rendert das ohne Konsolenfehler" fehlt. Empfehlung für eine Folge-Session mit
funktionierendem Playwright-Setup (oder manuell durch den Nutzer im eigenen Browser).

**Update 2026-07-15 — Lücke weiterhin offen, aber Ursache jetzt anders diagnostiziert:** Für einen
neuen Anlauf wurden zwei Browser-MCP-Server in `.mcp.json` eingetragen (`firefox-devtools`,
`chrome-devtools`, siehe Repo-Root). Ergebnis dieses Versuchs: Die MCP-Tools kamen in der
Session nicht als nutzbare Tools an (leere Trefferliste bei jeder `ToolSearch`-Abfrage nach
Browser-/DevTools-Funktionen). Ursache manuell isoliert, keine Vermutung: `~/.claude.json` zeigt
für dieses Projektverzeichnis `hasTrustDialogAccepted: false` und `enabledMcpjsonServers: []` —
Claude Code hat die in `.mcp.json` deklarierten Server nie geladen, weil das projektbezogene
MCP-Trust-Gate (normalerweise ein interaktiver Zustimmungs-Dialog beim Start einer Terminal-
Session) in dieser Session nie durchlaufen wurde. Das ist unabhängig von den Browsern selbst:
`npx -y chrome-devtools-mcp@latest --executablePath=/snap/bin/chromium --headless` manuell in der
Shell gestartet läuft an, gibt sein Start-Banner aus und beendet sich nur, weil kein Client an
stdin hängt — kein Absturz, kein fehlender Chromium-Build. `/usr/bin/firefox --version` läuft
ebenfalls ohne Fehler durch. D.h. beide Browser-Binaries und der Chrome-DevTools-MCP-Server sind
grundsätzlich startfähig; blockiert ist einzig die Trust-Freigabe der projektweiten `.mcp.json`
durch den Nutzer. Diese Freigabe kann nur interaktiv (Trust-Dialog beim Start von `claude` in
diesem Verzeichnis, oder manuelle Aufnahme der Servernamen in
`enabledMcpjsonServers` durch den Nutzer selbst) erteilt werden — wurde in dieser Session bewusst
nicht selbst umgangen, da das ein Sicherheits-/Zustimmungsschritt ist. Dev-Stack lief zum
Testzeitpunkt bereits vollständig (`docker ps`: alle `avoc-*`-Container inkl. beider
`vehicle-mock`-Instanzen seit Stunden up), stand also bereit — der eigentliche
Browser-Klick-Test (Login, Live-Update via WS, `vehicle-001`-Rendering, Teleoperate-Button-Gating
über zwei parallele Operator-Sessions) konnte dadurch wieder nicht durchgeführt werden. Nächster
Schritt: Nutzer startet einmal `claude` interaktiv in diesem Repo und bestätigt den MCP-Trust-
Dialog für `firefox-devtools`/`chrome-devtools` (oder gibt ausdrücklich frei, dies programmatisch
in `~/.claude.json` einzutragen) — danach ist der eigentliche Test in einer Folge-Session
voraussichtlich ohne weitere Hindernisse möglich.

**Update 2026-07-15 (Fortsetzung) — Trust-Dialog bestätigt, Lücke trotzdem noch offen, jetzt aber
präziser eingegrenzt:** Nutzer hat den Workspace-Trust-Dialog für dieses Projekt inzwischen
interaktiv bestätigt — `~/.claude.json` zeigt für den Projektpfad jetzt `hasTrustDialogAccepted:
true` (vorher `false`). Erneuter Test in einer neuen Subagent-Session direkt danach: `ToolSearch`
nach Browser-/DevTools-Funktionen ("browser screenshot navigate", danach zusätzlich "firefox
chrome devtools mcp page click evaluate") liefert weiterhin ausschließlich das eingebaute
`WebFetch` zurück — kein einziges `firefox-devtools`- oder `chrome-devtools`-Tool taucht als
Deferred Tool auf. Zweite Prüfung von `~/.claude.json` direkt danach: `enabledMcpjsonServers` UND
`disabledMcpjsonServers` sind beide weiterhin leere Arrays (`[]`) — keine der beiden in `.mcp.json`
deklarierten Server-IDs ist dort eingetragen, weder positiv noch negativ. Das belegt: Workspace-
Trust (`hasTrustDialogAccepted`) und die Freigabe einzelner projektbezogener `.mcp.json`-Server
(`enabledMcpjsonServers`) sind zwei getrennte Gates in Claude Code — die Bestätigung des einen löst
das andere nicht automatisch mit aus. Zusätzlich geprüft, ob sich das Gate von dieser Session aus
selbst schließen ließe: in der Bash-Umgebung dieses Subagents ist kein `claude`-Binary im PATH
(`command not found: claude`) — der Slash-Befehl `/mcp`, über den die interaktive
Server-Zustimmung normalerweise abläuft, ist ausschließlich an die interaktive TUI-Session
gebunden und von einer Hintergrund-/Subagent-Session aus nicht aufrufbar. Es gibt also aktuell
keinen Weg, diese Freigabe aus einer solchen Session heraus zu erteilen. Wie in der Aufgabenstellung
vorgegeben, wurde `~/.claude.json` NICHT selbst verändert, um die Freigabe zu erzwingen. Schritte
2–4 (Browser starten, Dev-Stack-Check, eigentlicher Login-/Live-Update-/Teleoperate-Gating-Test)
wurden entsprechend nicht durchgeführt, da sie zwingend funktionierende Browser-MCP-Tools
voraussetzen. Damit ist das ursprünglich vermutete Henne-Ei-Problem bestätigt und präzisiert: Der
fehlende letzte Verifikationsschritt ("echter Browser rendert Login → Fleet Overview → Live-Update
→ Teleoperate-Gating ohne Konsolenfehler") lässt sich mit dem aktuellen Session-Modell (Subagent /
Hintergrund-Job) grundsätzlich nicht schließen — das ist eine Umgebungsgrenze, kein im Rahmen
dieser Aufgabe lösbares Problem. Um weiterzukommen, muss der Nutzer selbst einmal interaktiv
`claude` in diesem Repo-Verzeichnis starten und dort — falls der Zustimmungs-Dialog für
`firefox-devtools`/`chrome-devtools` erscheint — bestätigen, oder die beiden Servernamen manuell in
`enabledMcpjsonServers` in `~/.claude.json` eintragen. Bis dahin bleibt der Browser-Klick-Test
offen; alle bisher per curl/rohem WebSocket-Client verifizierten Backend-Ebenen und die 167
Komponenten-/Hook-Tests sind davon unberührt und weiterhin gültig.

**Bewusst nicht in diesem Sprint:** Karten-/Zonen-Visualisierung (`GET /fleet/zones`/`/fleet/stations`
existieren bereits, aber keine SVG-Rendering-Komponente), Task-Management-UI (`GET/POST
/fleet/tasks` ebenfalls schon vorhanden), Dark-Mode-Umschaltung (UI bleibt fest dunkel wie
bisher), System-Steuerung/-Override-Panel, Performance-Monitoring jenseits der vier gezeigten
Telemetriewerte (Batterie/Geschwindigkeit/Autonomie-Modus/Zone). Alles Folge-Tasks für einen
späteren Sprint, sobald konkret gebraucht.

---

# Sprint 21 — Fleet Backend Foundation (fleet-service, Multi-Vehicle-Simulation)

Ziel: Das in `ADR-027/028/029` entworfene Fleet-Backend real aufsetzen, damit AP2
(Web-Dashboard-Frontend) gegen echte Bewegtdaten statt gegen nichts entwickelt werden kann.
Bewusst kein Frontend-Task in diesem Sprint — Backend-Fundament zuerst, Dashboard-UI folgt in
Sprint 22.

Datum: 2026-07-14 | **Status: Alle Tasks ✅ (Branches gemergt, 2026-07-15)**
Vorgänger: Sprint 20 ✅
Branch: `feature/fleet-service-foundation` (Basis: `docs/ibatour-pivot`)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| FLEET-01 | DB-Migration: `vehicle_type` Spalte zu `vehicles` (`ADR-029`); neue Tabellen `vehicle_status`, `zones`, `stations`, `tasks`, `alerts` mit FK auf `vehicles.id` | S | ✅ |
| FLEET-02 | `fleet-service` Skeleton — neuer Go-Service nach bestehendem Muster (`cmd/fleet-service/main.go`, `/health`, eigener DB-Connection-Pool auf `avoc`), Docker-Integration (`docker-compose.yml`, `Makefile` GO_SERVICES) | M | ✅ |
| FLEET-03 | `FleetGateway`-Interface + Mock-Implementierung (`ADR-027`) — abstraktes Go-Interface definieren, Mock liefert simulierte Fahrzeugdaten (Position/Batterie/Status/Alerts) | M | ✅ |
| FLEET-04 | Multi-Vehicle-Simulation — `vehicle-mock` erweitern: mehrere simulierte Fahrzeuge gleichzeitig (Typen `lastenrad`/`lastenzug`), bewegen sich zwischen Stationen, Batterie sinkt/lädt, publizieren über `FleetGateway`-Mock | L | ✅ |
| FLEET-05 | `fleet-service` konsumiert `FleetGateway`-Mock, schreibt `vehicle_status`; REST-API (`GET /fleet/vehicles`, `/fleet/zones`, `/fleet/stations`, `/fleet/tasks`, `/fleet/alerts`) inkl. einfacher Zonen-/Stationen-/Task-CRUD | M | ✅ |
| FLEET-06 | WS-Broadcast für Live-Updates (Multi-Workstation-Unterstützung, `ADR-028`) — alle verbundenen Dashboard-Clients erhalten Zustandsänderungen ohne Polling | M | ✅ |
| FLEET-07 | Alert-Engine — Schwellenwert-Logik in `fleet-service` (Beispiel: Batterie-Warnung), getrennt von fahrzeug-initiierten Alerts (kommen bereits fertig über `FleetGateway`-Mock) | S | ✅ |
| FLEET-08 | Unit-Tests `fleet-service` (Schema, API-Handler, Alert-Engine) analog bestehendem Testmuster (`testing`+`testify`) | S | ✅ |

**Abhängigkeitspfad:** FLEET-01 → FLEET-02 → FLEET-03 → FLEET-04 → FLEET-05 → FLEET-06/FLEET-07 (parallel möglich) → FLEET-08

## Ergebnisse

**FLEET-01 — DB-Migration ✅**
`internal/fleetservice/store.go`: `ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS vehicle_type`
(idempotent) + `CREATE TABLE IF NOT EXISTS` für `zones`, `stations`, `tasks`, `vehicle_status`,
`alerts` — alle FK-Referenzen wie in `ADR-029` spezifiziert. `PostgresFleetStore` mit
Basis-CRUD (Zones/Stations/Tasks/VehicleStatus-Upsert/Alerts+Acknowledge). Verifiziert gegen
echte lokale Postgres-Instanz (nicht nur kompiliert): 3 Durchläufe hintereinander grün, bestätigt
sowohl Schema-Idempotenz als auch korrektes CRUD/Upsert-Verhalten. `go build ./...`/`go vet ./...`
für das gesamte Repo sauber. Test: `internal/fleetservice/store_test.go` (Skip ohne
`DATABASE_URL`, analog bestehendem Integrationstest-Muster).

**FLEET-01 — Erweitertes Testmodell (Edge Cases, Integration, E2E-Vorstufe) ✅**
Auf Nutzerwunsch vertieft, bevor es weiterging:
- `edgecases_test.go`: FK-Verletzungen (Task/Station auf unbekannte IDs), PK-Duplikate (Zone
  zweimal), CHECK-Verletzungen (ungültiger `environment`/`autonomy_mode`/`severity`), Not-Found-
  Pfade (`SetVehicleType`/`AcknowledgeAlert`), NULL-Handling (Indoor-Station ohne GPS), deutsche
  Umlaute/Sonderzeichen in Namen/Alert-Texten
- `integration_test.go`: `fleetservice` + `vehicleregistry` koexistieren nachweislich auf
  derselben `vehicles`-Tabelle (Kernanspruch aus `ADR-029` jetzt verifiziert, nicht nur behauptet)
  — inkl. Nachweis, dass `vehicleregistry.List()` durch die neue Spalte nicht bricht. Zusätzlich:
  FK-RESTRICT-Verhalten bestätigt (referenzierte Zone kann nicht gelöscht werden)
- `lifecycle_test.go`: kompletter Autonomy-First-Ablauf aus `ADR-028` einmal end-to-end auf
  Datenebene durchgespielt (Fahrzeug registriert → Typ → Zone/Stationen → Task → autonomous →
  Alert → teleoperated → Task completed → zurück zu autonomous) — E2E-Vorstufe, echtes HTTP-E2E
  folgt mit FLEET-02/05

9 Testfunktionen, alle grün, 2x hintereinander wiederholt (Wiederholbarkeit auf geteilter
Dev-DB bestätigt), `gofmt`/`go vet`/`go build` sauber.

**FLEET-02 — fleet-service Skeleton ✅**
`cmd/fleet-service/main.go` nach `auth-service`-Muster (DB-Connection via `pkg/db`, `WaitForReady`
gegen Crash-Restart-Race, `/health`). Docker-Integration: neuer Service-Block in
`docker-compose.yml` (Port 8085, `DATABASE_URL` gegen `avoc`), `fleet-service` zu `GO_SERVICES`
in `Makefile` ergänzt (Variable + `build`-Target-Duplikat — beide Stellen, da im Makefile bisher
nicht über eine gemeinsame Quelle gepflegt), README-Serviceliste ergänzt.

Verifiziert im echten laufenden Docker-Stack (nicht nur `go build`): Container gebaut und
gestartet, `GET /health` → `200 {"status":"ok"}`, und — wichtiger — das Schema wurde beim echten
Container-Start korrekt gegen die **bereits von `control-server`/`auth-service` genutzte**
`vehicles`-Tabelle initialisiert (`vehicle_type`-Spalte ergänzt, alle 5 neuen Tabellen angelegt,
FK-Constraints aktiv). Das ist der in `ADR-029` behauptete Koexistenz-Fall jetzt auch im echten
Deployment bestätigt, nicht nur im isolierten Test.

**FLEET-02 — Integrationstest-Suite (`tests/integration/`) ✅**
Auf Nutzerwunsch analog zu FLEET-01 vertieft: `fleet-service` in `tests/docker-compose.test.yml`
aufgenommen (Port 18085, Healthcheck), `fleetURL`-Konstante in `setup_test.go`, zwei neue Tests
in `services_test.go` (`TestIntegration_FleetService_Healthy`,
`TestIntegration_FleetService_CoexistsWithControlServer_VehicleRegistration` — Fahrzeug-
Registrierung über `control-server`s echte HTTP-API bleibt funktionsfähig, während
`fleet-service` im selben Netzwerk läuft).

**Dabei einen echten Bug gefunden und behoben:** `fleet-service` und `control-server` haben im
Test-Stack (wie im Dev-Stack) bewusst kein `depends_on` zueinander — beide hängen nur an Postgres.
Startet `fleet-service` zuerst, existierte `vehicles` noch nicht, und `ALTER TABLE vehicles ADD
COLUMN` schlug fehl (`relation "vehicles" does not exist`) — die in `ADR-029` behauptete
"Startreihenfolge ist egal"-Eigenschaft war schlicht nie getestet worden. Fix in
`internal/fleetservice/store.go`: `fleet-service` legt die Basistabelle jetzt selbst per
`CREATE TABLE IF NOT EXISTS` an (identisch zu `vehicleregistry`s Schema), bevor es sie erweitert.
Dauerhafter Regressionstest ergänzt (`TestNewPostgresFleetStore_SucceedsWhenVehiclesTableDoesNotExistYet`,
isolierte Postgres-Schema-Simulation einer wirklich leeren DB). `ADR-029` mit einem datierten
Update-Hinweis korrigiert statt stillschweigend umgeschrieben.

Volle Suite (`make test-integration`, alle Services inkl. `fleet-service`) und die komplette
`internal/fleetservice`-Testsuite (10 Testfunktionen) laufen nach dem Fix durch.

**FLEET-03 — FleetGateway-Interface + Mock ✅**
Neues Paket `internal/fleetgateway` (ADR-027): `FleetGateway`-Interface mit
`SubscribeVehicleStatus`/`SubscribeVehicleAlerts` (Pub/Sub, analog zum bestehenden
`safetyservice.Bus`-Muster) + `DispatchTask` (Leitstelle → Fahrzeug, Richtung noch unklar bis
AP1-Workshop). Event-Typen (`VehicleStatusEvent`, `VehicleAlertEvent`, `TaskAssignment`) bewusst
von den Persistenz-Typen aus `fleetservice` getrennt, damit das Interface stabil bleibt, falls
sich das DB-Schema später ändert.

`MockGateway`: erfüllt "liefert simulierte Fahrzeugdaten" bereits eigenständig — eingebauter
Simulationsloop (`StartSimulation`, Random-Walk-Position, sinkende Batterie, seltene
fahrzeug-initiierte Alerts) läuft ohne Abhängigkeit von FLEET-04. Zusätzlich `SimulateVehicleStatus`/
`SimulateVehicleAlert` exportiert, damit FLEET-04s `vehicle-mock`-Erweiterung und FLEET-05 den
Mock auch von außen treiben können.

12 Testfunktionen: Pub/Sub-Grundverhalten, Mehrfach-Subscriber, Edge Cases (keine Subscriber,
Zero-Timestamp-Autofill, explizite Timestamps erhalten), Nebenläufigkeit (`-race`, keine Data
Races bei gleichzeitigem Subscribe+Simulate), Simulationsloop (produziert Events pro Fahrzeug,
Batterie sinkt nachweislich über Zeit, bleibt im 0–100-Bereich, `Stop()` beendet den Loop
tatsächlich, `Stop()` ohne vorherigen `Start()` crasht nicht). `go build`/`go vet`/`gofmt` für
das gesamte Repo sauber.

**FLEET-04 — Multi-Vehicle-Simulation ✅**
Architekturentscheidung vorab: `vehicle-mock` läuft als eigener Container/Prozess, getrennt von
`fleet-service` — der In-Process-`MockGateway` aus FLEET-03 kann also nicht direkt genutzt
werden, es braucht einen echten Transport über Prozessgrenzen. Passend zur Annahme aus `ADR-027`
("MQTT für WAN") und weil Mosquitto bereits im Stack läuft: `vehicle-mock` publiziert jetzt JSON
(`fleetgateway.VehicleStatusEvent`/`VehicleAlertEvent`, JSON-Tags direkt im Interface-Typ) auf
`fleet/{id}/status` und `fleet/{id}/alert` (`internal/fleetgateway/mqtt_topics.go` — gemeinsamer
Vertrag für den künftigen Subscriber in FLEET-05).

Neue testbare Simulationslogik (`cmd/vehicle-mock/fleet_simulator.go`, getrennt von der
bestehenden, nicht unit-getesteten `main.go`): pro simuliertem Fahrzeug Bewegung zwischen zwei
Demo-Stationen (Platzhalter-Koordinaten — echte Zonen/Stationen folgen erst mit FLEET-05s
REST-API, siehe Backlog), Batterie sinkt während der Fahrt und lädt beim Stehen unter 30 %,
seltene fahrzeug-initiierte Alerts (`ADR-028`). `lastenzug` bewusst langsamer mit größerer
Batteriekapazität (relativ langsamerer Verbrauch/Ladevorgang) als `lastenrad`. Neuer
`FLEET_VEHICLES`-Env-Var (`docker-compose.yml`, Format `id:typ,id:typ`) — Default im Dev-Stack:
`lastenzug-01`, `lastenrad-01`, unabhängig vom bestehenden Direct-Teleop-Fahrzeug.

9 Unit-Tests für die reine Simulationslogik (Bewegung Richtung Ziel, Batterie-Grenzen 0–100,
Lade-/Fahrverhalten je nach Batteriestand, Typ-Unterschiede). Zusätzlich end-to-end gegen den
echten Mosquitto-Broker verifiziert (nicht nur Unit-Tests): `vehicle-mock`-Container gestartet,
über `mosquitto_sub` von außen mitgelesen — korrekt formatierte JSON-Nachrichten auf beiden
Topics, Batterie sinkt nachweislich über mehrere echte Ticks, beide Fahrzeugtypen liefen parallel.
(Ein unabhängiger Java-Prozess auf dem Host blockierte währenddessen Port 8080 für
`control-server` — nicht angetastet; die Fleet-Simulation lief unabhängig davon trotzdem korrekt,
da sie nicht an `control-server` hängt.)

**FLEET-04 — Integrationstest-Suite (`tests/integration/`) ✅**
Auf Nutzerwunsch analog zu FLEET-01/02 dauerhaft in die echte Suite überführt statt nur manuell
verifiziert zu lassen: `vehicle-mock` in `tests/docker-compose.test.yml` aufgenommen
(`FLEET_VEHICLES` mit zwei Test-Fahrzeugen), neuer Test
`TestIntegration_FleetSimulation_PublishesRealMQTTMessages` (erste MQTT-basierte Prüfung in
`tests/integration/`, bisher gab es dafür kein Muster) — verbindet sich als echter MQTT-Client
zum realen Mosquitto-Container, abonniert `fleet/+/status`, wartet auf ≥2 Nachrichten pro
Fahrzeug, prüft Struktur (Batterie 0–100, Position gesetzt, `autonomous`-Modus, Timestamp) und
verifiziert, dass sich die Batterie über echte Ticks hinweg tatsächlich ändert (kein statischer
Mock-Wert). 2x hintereinander gelaufen (identische Laufzeit, 5,51s) — kein Flackern.

Volle Suite (`make test-integration`, jetzt 6 Services inkl. `vehicle-mock` + `fleet-service`)
läuft grün durch.

**FLEET-05 — MQTT-Konsum, Status/Alert-Persistenz, REST-API ✅**
`internal/fleetgateway/mqtt.go` (`MQTTGateway`) ist die konkrete `FleetGateway`-Realisierung —
Gegenstück zu FLEET-04s Publisher: abonniert `StatusTopicWildcard`/`AlertTopicWildcard`, parst
JSON, reicht an registrierte Callbacks weiter (identisches Verhalten zu `MockGateway` aus Sicht
der Aufrufer). `cmd/fleet-service/main.go` verdrahtet: Store → `NewMQTTGateway` →
`SubscribeVehicleStatus`/`SubscribeVehicleAlerts` schreiben in Postgres → `fleetservice.Handler`
→ REST-Routen. Neue Structs (`Zone`, `Station`, `Task`, `VehicleStatus`, `Alert`, `FleetVehicle`)
bekamen JSON-Tags (snake_case, konsistent zu `fleetgateway`s Event-Typen) — vorher hätte die API
Go-Default-Feldnamen (PascalCase) geliefert.

`internal/fleetservice/handler.go`: REST-Handler analog zu `authservice.Handler` (`RequireAuth`
prüft nur Signatur/Gültigkeit, keine Rollenprüfung — Fleet-Daten sind operator-facing, nicht
rollenspezifisch wie AP3-Admin-Aktionen). `CreateTask` dispatcht fire-and-forget an die Gateway
(`ADR-027`: Ack-Semantik der echten Anbindung ist noch offen) — Dispatch-Fehler schlagen die
Anfrage nicht fehl, der Task bleibt persistiert/sichtbar/wiederholbar über das Dashboard.

Zwei reale Bugs erst bei der End-to-End-Verifikation gegen echte Infrastruktur gefunden (nicht
durch `go build`/Unit-Tests):

1. **Fleet-Fahrzeuge kamen nie in `vehicles` an.** Direct-Teleop-Fahrzeuge werden von
   `control-server` bei WS-Connect automatisch registriert (`ADR-029`) — Fleet-Fahrzeuge sprechen
   aber nur MQTT und stellen nie eine WS-Verbindung her. Ergebnis: jedes Status-/Alert-Event für
   ein neues Fahrzeug schlug an der FK-Constraint auf `vehicle_status`/`alerts` fehl, live
   beobachtet gegen den echten Dev-Stack (`docker logs` voller FK-Violation-Warnungen für die
   simulierten `lastenzug`/`lastenrad`-Fahrzeuge). Fix: neue Store-Methode
   `EnsureVehicleExists` (`INSERT ... ON CONFLICT DO NOTHING`), aufgerufen vor jedem
   Status-/Alert-Schreibvorgang — spiegelt `control-server`s eigenes Auto-Register-Verhalten,
   nur für den MQTT-only-Pfad. Regressionstest:
   `TestEnsureVehicleExists_UnblocksStatusForNeverConnectedVehicle`.
2. **Feste MQTT-ClientID.** `MQTTGateway` verband sich immer als `"fleet-service-gateway"` — beim
   parallelen Testlauf gegen den bereits laufenden echten `fleet-service`-Container kämpften beide
   Verbindungen um dieselbe Broker-Session (jeder Reconnect kickt den anderen), was
   `TestMQTTGateway_ReceivesStatusPublishedByExternalClient` reproduzierbar zum Timeout brachte.
   Fix: ClientID bekommt einen `ulid`-Suffix, damit jede Prozessinstanz eindeutig bleibt.

22 neue/erweiterte Tests in `internal/fleetservice`/`internal/fleetgateway`, gegen echtes
Postgres + Mosquitto verifiziert — bewusst auch mit einem parallel laufenden echten
`fleet-service`-Container, um genau den ClientID-Bug oben zu reproduzieren. Manuelle
E2E-Verifikation zusätzlich über die REST-API selbst (`curl` mit echtem JWT gegen den
Dev-Stack): Health, 401 ohne Token, Zone/Task-CRUD, Live-Status simulierter Fahrzeuge sichtbar.

**FLEET-05 — Integrationstest-Suite (`tests/integration/`) ✅**
Neue `tests/integration/fleet_service_test.go`: `TestIntegration_FleetService_RequiresAuth` (401
ohne Token), `TestIntegration_FleetService_ConsumesRealMQTTStatus_AcrossProcessBoundary` (die
eigentliche End-to-End-Kette über echte Prozessgrenzen: `vehicle-mock`-Container →
Mosquitto-Container → `fleet-service`-Container → Postgres → REST — bisherige Tests prüften nur
Teilstücke davon isoliert), `TestIntegration_FleetService_ZoneStationTaskCRUD` (voller CRUD-Zyklus
über die echte HTTP-API), `TestIntegration_FleetService_AcknowledgeAlert_UnknownID_Returns404`.
`tests/docker-compose.test.yml`: `fleet-service` bekam `JWT_SECRET`/`MQTT_BROKER` (fehlten bisher
— FLEET-02s Skeleton brauchte sie noch nicht) sowie `depends_on: mosquitto`.

Volle Suite (`make test-integration`, jetzt inkl. 4 neuer Fleet-REST-Tests) 2x hintereinander
gegen frisch gestartete Container gelaufen (`-count=1`, um Gos Test-Cache zu umgehen) — beide
Male grün, keine Flakiness.

**FLEET-05 — Testlücke nachträglich geschlossen (Nutzerrückfrage während FLEET-06) ✅**
Auf Nachfrage "ist FLEET-05 vollständig getestet?" geprüft statt geglaubt: `Handler.ListStations`
und `Handler.ListAlerts` wurden bis dahin **nie** über HTTP aufgerufen (nur die darunterliegenden
Store-Methoden), `Handler.CreateStation` hatte keinen Validierungstest, und keiner der vier
POST-Handler (`CreateZone`/`CreateStation`/`CreateTask`/`AcknowledgeAlert`) hatte einen
malformed-JSON-400-Test — der Decode-Error-Zweig war nur implizit durch den Code, nie durch einen
Test belegt. Zusätzlich: die Store-Fehlerpfade aus `edgecases_test.go` (PK-Duplikat,
FK-Verletzung) waren nur auf Store-Ebene verifiziert, nie durch den HTTP-Handler hindurch (bildet
`store error` korrekt auf 500 ab, statt zu paniken oder die Anfrage fälschlich als Erfolg zu
melden?).

10 neue Tests in `internal/fleetservice/handler_test.go`: `TestCreateZone_MalformedJSON_Returns400`,
`TestCreateZone_DuplicateID_Returns500`, `TestCreateStation_MissingFields_Returns400`,
`TestCreateStation_MalformedJSON_Returns400`, `TestCreateStation_UnknownZoneID_Returns500`,
`TestCreateStation_Valid_Returns201AndListable`, `TestCreateTask_MalformedJSON_Returns400`,
`TestListTasks_IncludesCreatedTask`, `TestAcknowledgeAlert_MalformedJSON_Returns400`,
`TestListAlerts_IncludesCreatedAlert`. Da weder der Dev- noch der Test-Postgres-Container einen
Host-Port exponiert, gegen den echten laufenden Dev-Stack-Postgres über einen temporären
`golang:1.23`-Container im selben Docker-Netzwerk (`avoc_avoc-net`) statt direkt vom Host
verifiziert — 2x gelaufen (zweiter Lauf `-count=1`), beide Male grün, keine Regression in den
bestehenden ~30 Paket-Tests, keine übrig gebliebenen Testzeilen in Postgres danach (`t.Cleanup`
greift wie erwartet).

**FLEET-06 — WS-Broadcast für Live-Updates ✅**
Neuer `Hub` (`internal/fleetservice/broadcast.go`) — fasst jede verbundene Dashboard-Verbindung
als `wsClient` (gepufferter `send`-Channel + eigener `writePump`, da `gorilla/websocket`-
Connections keine nebenläufigen Writer erlauben und `Broadcast` sowohl aus den MQTT-Gateway-
Callbacks als auch aus REST-Handler-Goroutinen aufgerufen wird). Bewusst **kein** Wiederverwenden
von `internal/controlserver/transport.WSHandler` — geprüft, aber verworfen: das ist ein
Fahrzeug↔Server-Protobuf-Command-Channel mit Session-Bindung, State-Machine-Kopplung und
Deadman/ACK-Watchdogs; `fleet-service`s WS ist ein reiner Dashboard-Client↔Server-JSON-Broadcast
ohne Gegenstück-Semantik. Einziges übernommenes Muster: der `extractToken`-Trick (Token optional
als `?token=`-Query-Param, da Browser-WebSocket-Clients beim Handshake keinen
`Authorization`-Header setzen können) — als eigene Kopie in `fleetservice` (`wsToken`), um die
beiden WS-Schichten nicht zu koppeln.

Neuer Endpoint `GET /fleet/ws` (`Handler.ServeWS`, `internal/fleetservice/handler.go`) — Auth
über denselben JWT-Secret-Check wie `RequireAuth` (in `validateToken` extrahiert, jetzt von
beiden geteilt). Vier Broadcast-Auslöser, alle zusätzlich zum bestehenden Store-Write (nicht
statt dessen):
- `vehicle_status` — `gw.SubscribeVehicleStatus`-Callback in `cmd/fleet-service/main.go`, nach
  `UpsertVehicleStatus`
- `alert_created` — `gw.SubscribeVehicleAlerts`-Callback, nach `CreateAlert` (fahrzeug-initiiert
  über MQTT)
- `alert_acknowledged` — `Handler.AcknowledgeAlert`, nach erfolgreichem Store-Update (eigener
  `AlertAcknowledgedEvent`-Typ statt vollem `Alert`, da `store.AcknowledgeAlert` nur
  Erfolg/Not-Found zurückgibt, nicht die Zeile — `GET /fleet/alerts` bleibt die autoritative
  Quelle für den exakten Server-Timestamp)
- `task_created` — `Handler.CreateTask`, nach erfolgreicher Persistierung (kein `task_updated`,
  da es aktuell keinen Task-Update-Endpoint gibt — nur Erstellung existiert bislang)

Slow-Consumer-Handling: `Broadcast` sendet non-blocking (`select`+`default`) in den
32-Element-Puffer jedes Clients — ein hängender Client verliert einzelne Events, blockiert aber
nie die Zustellung an alle anderen. Kein automatisches Disconnect bei vollem Puffer (bewusst
einfach gehalten, kein Killer-Client-Mechanismus für diesen ersten Slice).

5 neue Unit-Tests (`internal/fleetservice/broadcast_test.go`, `-race`-sauber, 2x wiederholt):
Zustellung an einen/mehrere echte WS-Clients (via `httptest.Server` + echtem
`gorilla/websocket`-Handshake), Unregister bei Disconnect, Slow-Consumer blockiert andere Clients
nachweislich nicht, `ClientCount()`-Konsistenz inkl. doppeltem Unregister ohne Panic.

Gegen den echten laufenden Dev-Stack verifiziert (Container neu gebaut/gestartet, nicht nur
kompiliert) — alle vier Broadcast-Pfade einzeln mit einem echten WS-Client (`gorilla/websocket`,
echtes JWT von `auth-service`) beobachtet:
1. `vehicle_status` — lief bereits allein durch `vehicle-mock`s Simulationsloop (FLEET-04) an,
   mehrere Events innerhalb weniger Sekunden empfangen
2. `task_created` — via echtem `POST /fleet/tasks` ausgelöst, Event mit korrektem Payload
   empfangen
3. `alert_acknowledged` — Alert direkt in Postgres eingefügt, via echtem
   `POST /fleet/alerts/{id}/acknowledge` bestätigt, Event empfangen
4. `alert_created` — Alert direkt per `mosquitto_pub` auf `fleet/{id}/alert` publiziert (simuliert
   fahrzeug-initiierten Alert ohne auf den seltenen Zufalls-Alert aus `vehicle-mock` zu warten),
   Event empfangen
5. Multi-Workstation-Fanout: zwei gleichzeitige WS-Clients verbunden, beide erhielten denselben
   `task_created`-Broadcast — die eigentliche Kernanforderung aus `ADR-028`
6. `GET /fleet/ws` ohne Token → Handshake schlägt fehl (401), wie bei den REST-Endpoints

Dauerhaft in `tests/integration/fleet_service_test.go` überführt (kein rein manueller Test) —
auf Nutzerrückfrage nachgezogen, nachdem die ersten drei Tests nur `task_created`/`vehicle_status`
abdeckten und die beiden MQTT-/Alert-Pfade nur manuell (curl/mosquitto_pub) verifiziert waren, was
der eigenen Verifikationsdisziplin widersprach:
- `TestIntegration_FleetService_WSBroadcast_RequiresAuth` (kein Token → Handshake schlägt fehl)
- `TestIntegration_FleetService_WSBroadcast_InvalidToken_Rejected` (Edge Case: syntaktisch
  vorhandener, aber ungültiger Token — separater Codepfad in `validateToken` als "kein Token")
- `TestIntegration_FleetService_WSBroadcast_DeliversTaskCreated` (verbindet zuerst per WS, erstellt
  danach einen Task über die echte REST-API, prüft den empfangenen Broadcast gegen die Task-Daten)
- `TestIntegration_FleetService_WSBroadcast_MultiWorkstationFanout` (zwei WS-Clients, beide müssen
  ein `vehicle_status`-Event aus dem laufenden `vehicle-mock`-Simulationsstream empfangen — kein
  REST-Trigger nötig, deckt den Dauerbetriebs-Fall ab)
- `TestIntegration_FleetService_WSBroadcast_DeliversAlertCreatedAndAcknowledged` (publiziert einen
  Alert per echtem MQTT-Client auf `fleet/{id}/alert` — analog zu FLEET-04s
  `TestIntegration_FleetSimulation_PublishesRealMQTTMessages`s Verbindungsmuster, nur als
  Publisher statt Subscriber — prüft `alert_created`-Broadcast, quittiert den Alert danach über
  die echte REST-API und prüft zusätzlich `alert_acknowledged`; damit ist die einzige noch nicht
  über den REST-/WS-Pfad abgedeckte Kombination geschlossen)

Neuer Test-Helper `readWSEventOfType` (überliest interleavte `vehicle_status`-Rauschen aus dem
laufenden `vehicle-mock`-Simulationsstream bis zum gesuchten Event-Typ) — von
`DeliversTaskCreated` und dem neuen Alert-Test gemeinsam genutzt, ersetzt eine anfangs pro Test
duplizierte Skip-Schleife.

Volle Suite (`make test-integration`, jetzt 24 Tests, davon 5 neu für FLEET-06) 2x hintereinander
gegen frisch gestartete Container gelaufen (zweiter Lauf mit `-count=1`, um Gos Test-Cache zu
umgehen) — beide Male grün, keine Flakiness.

Kein echter Bug bei der Infrastruktur-Verifikation gefunden (anders als FLEET-01/02/05) — die
Wiederverwendung von `EnsureVehicleExists`/`CreateAlert`s Rückgabewert aus FLEET-05 hat den
FK-/Timing-Fall hier bereits sauber abgedeckt.

**FLEET-07 — Alert-Engine (Batterie-Schwellenwert) ✅**
Neuer `AlertEngine`-Typ (`internal/fleetservice/alertengine.go`) — bewusst **getrennt** vom
vehicle-initiierten Alert-Pfad (`gw.SubscribeVehicleAlerts` in `cmd/fleet-service/main.go`, kommt
unverändert weiter über `FleetGateway`). `AlertEngine.Evaluate` hängt stattdessen im
`gw.SubscribeVehicleStatus`-Callback, direkt nach `UpsertVehicleStatus`/`hub.Broadcast
("vehicle_status", ...)` — läuft also bei jedem Status-Tick mit, nicht nur bei Alerts.

Kernproblem, das die Umsetzung eigentlich ausmacht: bei einem Status-Update alle ~2s (FLEET-04s
Simulationsintervall) würde ein naiver `battery_pct < 20 → Alert` bei jedem einzelnen Tick unter
der Schwelle einen neuen Alert erzeugen — Spam statt Signal. Gelöst über eine
Tier-Zustandsmaschine pro Fahrzeug (`normal`/`warning`/`critical`, `map[vehicleID]int` in
`AlertEngine`, `sync.Mutex`-geschützt) mit **Hysterese**: Eintritt in `warning` bei <20 %,
Rückkehr zu `normal` erst ab ≥25 %; Eintritt in `critical` bei <10 %, Rückkehr zu `warning` erst
ab ≥15 %. Ein Alert wird nur beim Überschreiten in eine *schlechtere* Tier ausgelöst — bleibt das
Fahrzeug in derselben Tier oder erholt es sich, gibt es keinen (weiteren) Alert. Der 5-Prozentpunkte-
Puffer zwischen Eintritts- und Austrittsschwelle verhindert Flackern exakt an der Grenze (z. B.
19,8 % → 20,5 % → 19,5 % würde ohne Puffer zwei Alerts erzeugen). Direkter Sprung von gesund auf
kritisch (ein einzelner Tick) erzeugt korrekt nur den kritischen Alert, keinen zusätzlichen
Warning-Alert dazwischen.

11 Unit-Tests (`internal/fleetservice/alertengine_test.go`, `-race`-sauber, 3x wiederholt):
gesunde Batterie kein Alert, `nil`-Batterie kein Alert (Fahrzeug ohne Telemetrie), Warning-/
Critical-Auslösung, Wiederholungssperre über mehrere Ticks in derselben Tier, direkter
Sprung gesund→kritisch, stille Erholung (kein Alert), Reset nach voller Erholung (erneuter Abfall
löst wieder aus), Hysterese-Flatter-Test an der 20-%-Grenze, Erholung critical→warning mit
erneutem Abfall→critical, Unabhängigkeit zwischen Fahrzeugen, Nebenläufigkeit.

Gegen den echten Dev-Stack verifiziert (nicht nur Unit-Tests) — Status mit `battery_pct: 5.0` per
`mosquitto_pub` auf `fleet/{id}/status` publiziert (simuliert ein reales Fahrzeug, nicht über
`vehicle-mock`s eigene Simulation): kritischer Alert "Batterie kritisch (5.0%)" erschien korrekt
in `GET /fleet/alerts`. Zweiter Tick bei 4.0 % erzeugte nachweislich **keinen** zweiten Alert
(Wiederholungssperre live bestätigt). Tick auf 90 % (Erholung) erzeugte ebenfalls keinen
zusätzlichen Alert. Alte Testdaten aus vorherigen Ad-hoc-Verifikationsläufen (`mqtt-test-*`, die
über den geteilten Dev-Mosquitto versehentlich vom echten `fleet-service`-Container mitgelesen
und autoregistriert wurden, `EnsureVehicleExists` aus FLEET-05) aus der Dev-DB bereinigt.

Neuer Integrationstest
`TestIntegration_FleetService_AlertEngine_LowBatteryTriggersThresholdAlert`
(`tests/integration/fleet_service_test.go`) — publiziert echten Status per MQTT-Client (nicht
`vehicle-mock`), prüft `alert_created`-WS-Broadcast UND `GET /fleet/alerts`, prüft danach explizit
die Wiederholungssperre über einen zweiten Tick. Bewusst als eigener Test von
`TestIntegration_FleetService_WSBroadcast_DeliversAlertCreatedAndAcknowledged` abgegrenzt: der
bestehende Test publiziert direkt auf den Alert-Topic (vehicle-initiiert, umgeht `AlertEngine`
komplett) — dieser hier geht über den Status-Topic, den einzigen Weg, `AlertEngine` tatsächlich
zu treffen. `make test-integration` (jetzt 25 Tests) 2x hintereinander gegen frisch gestartete
Container gelaufen (`-count=1`) — beide Male grün.

**FLEET-07 — Nachtrag: Audit gegen `CLAUDE.MD` Abschnitt 17 (Teststandard) ✅**
Nach Einführung des neuen Teststandards rückwirkend gegen die Fallgruppen-Checkliste geprüft.
9 zusätzliche Grenzwert-Tests ergänzt (`internal/fleetservice/alertengine_test.go`,
`-race`-sauber, 3x wiederholt): exakte Schwellenwerte (20.0/25.0/10.0/15.0 — sperrt die
`<`-vs-`<=`-Semantik explizit fest, vorher nur implizit über Werte deutlich über/unter der
Grenze getestet), 0 % und 100 % Batterie, negative Batteriewerte (physikalisch unmöglich, aber
kein Absturz/Fehlklassifizierung bei fehlerhaftem Upstream), leere `VehicleID` (gültiger
Map-Key, keine Zustandsvermischung mit anderen Fahrzeugen).

Ein Fehlerpfad bewusst ungetestet gelassen statt stillschweigend übersehen: schlägt
`store.CreateAlert` in `cmd/fleet-service/main.go` fehl, nachdem `AlertEngine.Evaluate` einen
Alert zurückgegeben hat, wird das nur geloggt (`log.Warn`), nicht erneut versucht — analog zum
bereits bestehenden, ebenfalls ungetesteten Verhalten beim vehicle-initiierten Alert-Pfad
direkt darüber. Konsistent mit der Projektkonvention, `cmd/*/main.go`-Verdrahtung nicht direkt
zu unit-testen (dafür sorgen die Integrationstests für den Erfolgsfall); ein DB-Fehler exakt in
diesem Moment zu erzwingen wäre nur mit Aufwand deutlich über das reguläre Test-Setup hinaus
möglich und stand in keinem Verhältnis zum Risiko (identisches Verhalten wie der bereits
akzeptierte Nachbar-Pfad).

**FLEET-08 — Unit-Tests fleet-service: Lückenanalyse Schema/API-Handler ✅ (Alert-Engine-Teil offen)**
Gegenstand war eine Lückenanalyse, kein Neuschreiben — der bestehende Teststand
(`store_test.go`, `handler_test.go`, `broadcast_test.go`, `edgecases_test.go`, `integration_test.go`,
`lifecycle_test.go`, insgesamt bereits ~30 Testfunktionen aus FLEET-01/02/05/06) deckte Schema/CRUD/
FK-Constraints/Auth/CRUD-über-HTTP bereits weitgehend ab. Style-Hinweis aus der Aufgabenstellung
("`testing`+`testify`") bewusst nicht befolgt: die bestehenden Tests in `internal/fleetservice`
nutzen durchgängig reine `testing`-Stdlib (`t.Fatalf`), nicht `testify` — neue Tests bleiben
konsistent zum bereits etablierten Paketstil statt eine zweite Test-Bibliothek einzuführen.

Sechs neue Tests geschlossen die von der Aufgabenstellung benannten Kandidaten:
- `TestListMethods_DBConnectionClosed_ReturnsError` (`edgecases_test.go`) — Fehlerpfad aller sechs
  Store-Lesemethoden (`ListZones`/`ListStations`/`ListTasks`/`ListAlerts`/`ListVehicleStatus`/
  `ListVehiclesWithStatus`) bei einer geschlossenen DB-Verbindung; bisher war nur der Erfolgsfall
  getestet. Bewusst per `db.Close()` reproduziert statt z.B. eine Tabelle zu droppen — Letzteres
  hätte die geteilte Dev-Postgres-Instanz beeinträchtigt, an der die parallele FLEET-07-Session
  gerade arbeitet.
- `TestCreateTask_UnknownVehicleID_Returns500` / `TestCreateTask_UnknownStationID_Returns500`
  (`handler_test.go`) — `edgecases_test.go` bewies die FK-Verletzung bisher nur auf Store-Ebene;
  jetzt auch über den HTTP-Handler bestätigt: aktuelles Ist-Verhalten ist ein generischer 500
  ("store error"), wie bei `CreateStation`/unbekannter `zone_id` (FLEET-05). Bewusst kein Fix auf
  aussagekräftigere 400/404 — das ist die gleiche, bereits in FLEET-05 getroffene
  Design-Entscheidung (Scope klein halten), hier nur zusätzlich für `CreateTask` als Ist-Verhalten
  festgeschrieben statt nur für `CreateStation`.
- `TestAcknowledgeAlert_AlreadyAcknowledged_SecondCallSucceedsAndOverwrites` (`handler_test.go`) —
  dokumentiert das bisher unverifizierte Ist-Verhalten: ein zweites `AcknowledgeAlert` auf denselben
  Alert wird nicht abgelehnt, sondern überschreibt `acknowledged_by`/`acknowledged_at` mit dem
  Wert des zweiten Aufrufs (die UPDATE-Query matched über `id`, unabhängig vom bisherigen
  Quittierungsstatus). Kein Bug — es gibt aktuell keine Anforderung für
  Idempotenzschutz/First-Writer-Wins —, aber bisher nur implizit durchs Code, nie durch einen Test
  belegt.

Zwei von der Aufgabenstellung genannte Kandidaten wurden geprüft und bewusst **nicht** in einen
neuen Test überführt:
- **nil vs. leeres Array bei `List*`:** Code-Inspektion bestätigt, dass alle `List*`-Methoden
  (`store.go`) das Muster `var x []T` ohne Vorinitialisierung nutzen — bei null Zeilen bleibt die
  Slice `nil`, was `encoding/json` als `null` statt `[]` serialisiert (Standardverhalten von Go,
  kein Bug in diesem Paket). Nicht mit einem eigenen Test gegen die echte Dev-DB verifiziert: keine
  der fünf Fleet-Tabellen ist in der geteilten Dev-Postgres-Instanz je zuverlässig leer (andere
  Tests/`vehicle-mock`s Simulationsloop schreiben kontinuierlich), und ein künstliches Leeren
  (`DELETE`/`TRUNCATE`) hätte die parallele FLEET-07-Session riskiert. **Für Sprint 22 (Dashboard-
  Frontend) festgehalten:** `GET /fleet/zones` etc. können `null` statt `[]` liefern, wenn keine
  Zeilen existieren — das Frontend darf sich nicht auf ein Array verlassen, ohne das zu behandeln.
- **main.go-Wiring (fehlendes `JWT_SECRET`/`DATABASE_URL`):** `cmd/fleet-service/main.go` beendet
  den Prozess bei fehlenden Env-Vars über `log.Fatal` (→ `os.Exit`), was ohne Umbau von `main()` in
  eine testbare Funktion (z.B. Extraktion der Konfigurationsvalidierung) nicht sinnvoll unit-testbar
  ist — analog zur bestehenden Konvention in diesem Repo, `main()` nicht künstlich für Tests zu
  verbiegen. Bewusste Lücke, hier dokumentiert statt stillschweigend ignoriert.

6 neue Tests, 2x hintereinander gegen den echten laufenden Dev-Stack-Postgres verifiziert (zweiter
Lauf `-count=1`, um Gos Test-Cache zu umgehen) — beide Male grün, keine Flakiness, keine
Beeinträchtigung der geteilten Dev-DB (nur eigene, per `t.Cleanup` aufgeräumte Testdaten plus ein
harmloses `db.Close()` auf einer eigenen Verbindung). `gofmt`/`go vet`/`go build` für das gesamte
Repo sauber.

**FLEET-08 — Fortsetzung Alert-Engine-Teil ✅ (nach Merge von FLEET-07)**
Zum Zeitpunkt der obigen Schema/API-Handler-Runde war FLEET-07 noch nicht abgeschlossen (`git
fetch` auf den GitLab-Remote war in dieser Umgebung nicht möglich, lokal auch in keinem
Branch/Worktree sichtbar — daher der separate Branch `feature/fleet-service-foundation-fleet08`/
Worktree `../controlcenter-aws-fleet08`). FLEET-07 wurde inzwischen fertiggestellt und per
`git merge feature/fleet-service-foundation` in diesen Branch übernommen (Konflikt nur in
`tasks/current-sprint.md`, inhaltlich aufgelöst — kein Konflikt in Code/Tests).

Gegenstand dieser Runde war erneut eine Lückenanalyse, kein Neuschreiben — und die Analyse fiel
diesmal kurz aus: FLEET-07 wurde bereits **unter dem neuen `CLAUDE.MD`-Abschnitt-17-Teststandard**
umgesetzt (der Standard entstand während FLEET-07, siehe Commit `019d5a0`) und in einem eigenen
Nachtrag (`4d85a03`) rückwirkend gegen dessen komplette Fallgruppen-Checkliste geprüft, bevor
dieser FLEET-08-Teil überhaupt begann. `internal/fleetservice/alertengine_test.go` deckt bereits:
Grenzwerte (0 %/100 %/negative Werte/exakte Schwellenwerte mit expliziter `<`-vs-`<=`-Semantik),
Zustandsübergänge (Tier-Hysterese, direkter Sprung gesund→kritisch, Erholung, Flatter-Test an der
Grenze), Wiederholungssperre/Idempotenz (mehrere Ticks in derselben Tier), Nebenläufigkeit
(`-race`), und Isolation zwischen Fahrzeugen (inkl. leerer `VehicleID` als Sonderfall) — 20
Testfunktionen insgesamt.

Eigener Beitrag dieser Runde: **Verifikation statt Duplikation**. Den bereits gemergten
Teststand (Schema/API-Handler-Tests aus der ersten FLEET-08-Runde + FLEET-07s Alert-Engine-Tests)
als Ganzes gegenlaufen lassen, um sicherzustellen, dass beide unabhängig entstandenen
Testerweiterungen nach dem Merge zusammen funktionieren:
- `go test ./internal/fleetservice/...` gegen den echten Dev-Stack-Postgres: 2x grün (59
  Testfunktionen/Subtests, keine Flakiness).
- `go test ./internal/fleetservice/... -race -count=3` (nativ auf dem Host mit `CGO_ENABLED=1`,
  nicht im Alpine-Container, da dort kein `cgo` verfügbar ist) für die nebenläufigkeitsrelevanten
  Pakete (`AlertEngine`, `Hub`): 3x grün, keine Race-Funde.
- Volle `make test-integration`-Suite (frisch gebauter Teststack, 25 Tests inkl.
  `TestIntegration_FleetService_AlertEngine_LowBatteryTriggersThresholdAlert`): grün. Die
  3 übrig bleibenden Skips (`TestIntegration_SessionLifecycle_StartAndEnd` u.a.) sind vorbestehend
  und dokumentiert erwartet ("WebSocket dial failed (expected in minimal test stack)") — keine
  Fleet-bezogene Regression.
- `gofmt`/`go vet`/`go build` für das gesamte Repo weiterhin sauber (die von `gofmt -l` gemeldeten
  Dateien liegen alle außerhalb von `internal/fleetservice`/`cmd/fleet-service` und sind
  vorbestehende Formatierungsabweichungen, nicht durch diese Aufgabe verursacht).

Keine zusätzlichen Alert-Engine-Tests ergänzt — die Fallgruppen-Checkliste aus Abschnitt 17 war
bei Übernahme bereits vollständig abgedeckt, ein weiterer Test hätte nur eine bestehende
Fallgruppe dupliziert. Der eine bewusst offene Fehlerpfad (`store.CreateAlert` schlägt nach
`Evaluate` fehl) war bereits in FLEET-07s eigenem Nachtrag (`4d85a03`) als Lücke dokumentiert und
wird hier nicht erneut aufgegriffen — Begründung dort deckungsgleich mit der Projektkonvention zu
`main.go`-Verdrahtung.

Damit ist FLEET-08 vollständig: Schema (FLEET-01), API-Handler (FLEET-05/06) und Alert-Engine
(FLEET-07) haben je eine dedizierte, gegen echte Infrastruktur verifizierte Testabdeckung, dokumentierte bewusste Lücken statt stillschweigender Annahmen, und sind nach dem Merge gemeinsam
grün.

**Bewusst nicht in diesem Sprint:** Dashboard-Frontend (Fleet Overview, Karten, Task-Management-UI, Alert-UI, "Teleoperate"-Button-Wiring) — das ist Sprint 22, sobald hier eine echte API zum Entwickeln gegen existiert, statt gegen Annahmen zu bauen. Admin-Konsole (AP3, User Management/System-Konfiguration/Maintenance-Tracking) ist ein eigener, späterer Sprint. `GET /fleet/ws` ist ebenfalls noch nicht über `nginx.dev.conf` geroutet — wie schon die `/fleet/*`-REST-Routen aus FLEET-05 (dort ebenfalls nicht ergänzt) bewusst zurückgestellt, bis das Dashboard-Frontend in Sprint 22 tatsächlich einen Browser-Client dagegen braucht. Weitere Schwellenwert-Regeln (z. B. Geschwindigkeit, Zonenverlassen) sind nicht Teil von FLEET-07 — die Aufgabenbeschreibung nennt explizit nur Batterie als Beispiel; `AlertEngine` ist aber bewusst so strukturiert (eigener Tier-Mechanismus pro Regel-Dimension denkbar), dass weitere Regeln später ergänzt werden können, ohne den Aufrufer in `main.go` umzubauen.

---

# Sprint 20 — Bugfix: Session-Neustart nach Session-Ende blockiert

Ziel: Nach Beenden einer Session mit verbundenem Fahrzeug ließ sich keine neue Session mehr starten (`VehicleSelector` erschien nicht wieder).

Datum: 2026-07-10 | **Status: Abgeschlossen ✅**
Vorgänger: Sprint 19 ✅
Branch: `fix/session-restart-after-end` (Basis: `feature/devlokal`)

---

## Task

| ID | Task | Typ | Status |
|----|------|-----|--------|
| BUG-SESSION-01 | Session-Restore-Race in `App.tsx` beheben — nach `endSession()` verhindern, dass der Page-Reload-Recovery-Effect die gerade beendete Session aus stale `activeSessions`-Poll-Daten sofort wieder herstellt | S | ✅ |

---

## Root Cause

`frontend/src/App.tsx`: Der Page-Reload-Recovery-`useEffect` (Zweck: nach echtem Browser-Reload die verlorene `sessionId` aus `GET /sessions` wiederherstellen) hatte keinen "schon versucht"-Guard und lief bei **jedem** `activeSessions`-Poll-Tick (alle 3s) erneut, solange `sessionId` leer war. `activeSessions` wird nur alle 3s neu abgefragt — direkt nach einem manuellen `endSession()` enthielt der lokale State deshalb bis zu 3s lang noch die gerade beendete Session. Der Effect fand darin sofort einen Treffer für den eigenen `operatorId` und rief `restoreFromServerState()` erneut auf — die UI sprang zurück in die (serverseitig bereits beendete) Session, `VehicleSelector` blieb dauerhaft verborgen.

Backend war zu keinem Zeitpunkt betroffen — `GET /sessions` lieferte direkt nach Session-Ende korrekt `[]` (verifiziert per curl während Sprint 19).

## Fix

Zwei Refs ergänzt:
- `hasPolledSessionsRef` — markiert, sobald der erste echte Poll-Response eingetroffen ist (verhindert, dass der Restore-Effect mit dem initialen leeren State vorschnell "nichts zu tun" entscheidet)
- `restoreAttemptedRef` — der Restore-Versuch läuft jetzt maximal **einmal pro Mount**. Das reicht für den eigentlichen Zweck (Reload-Recovery beim App-Start) und verhindert, dass ein späterer, absichtlicher `endSession()`-Aufruf durch denselben Effect rückgängig gemacht wird.

## Verifikation

Playwright-Regressionstest (Login → Session starten → ≥1 Poll-Zyklus abwarten → Session beenden → `VehicleSelector` muss wieder sichtbar sein → neue Session erfolgreich starten):
- **Gegen alten Code:** Test reproduziert den Bug zuverlässig (Schritt 3 schlägt fehl, `select` bleibt verborgen)
- **Gegen gefixten Code:** Test grün
- `npm test` (103 Tests), `tsc --noEmit`, `npm run lint`: alle grün, keine Regressionen

---

# Sprint 19 — Lokaler Dev-Stack: Verifikation & Robustheit

Ziel: Sicherstellen, dass das Projekt zuverlässig **lokal** läuft (nicht nur auf der AWS-EC2-Instanz) — reproduzierbar auf einem frischen Checkout, mit dokumentierten Stolpersteinen und geklärter SSL/HTTPS-Frage.

Datum: 2026-07-10 | **Status: Abgeschlossen ✅ (1 Folge-Task an Backlog übergeben)**
Vorgänger: Sprint 18 (pausiert — 3 Tasks offen: AUTH-18-01, OBS-01, MV-11-ADR, siehe unten)

**Vorab-Recherche (2026-07-10):** Ein lokales Compose-Setup existiert bereits (`make up` → `infrastructure/compose/docker-compose.yml`, README-Schnellstart). Verifiziert: mit `.env` aus `.env.example` kopiert und den vorhandenen (7 Tage alten) Images startet der komplette Stack sauber durch — `frontend` (HTTP 200), `control-server /health` (ok), `vehicle-001` online, kein SSL-Fehler.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| LOCAL-01 | Sauberer Full-Rebuild-Test: `docker compose down -v` + `up --build` von Grund auf | S | ✅ |
| LOCAL-02 | `CONTEXT.MD`-Eintrag „Dev-Stack SSL-Fix" verifizieren | S | ✅ |
| LOCAL-03 | Frontend-Hot-Reload-Pfad (`npm run dev`) end-to-end verifizieren | S | ✅ |
| LOCAL-04 | WebRTC/Video-Verbindung lokal real testen (Playwright, Fake-Kamera via WHIP/WHEP) | M | ✅ (Blocker gefixt, Folge-Bug dokumentiert) |
| LOCAL-05 | README/`CONTEXT.MD` um neu gefundene Stolpersteine ergänzen | S | ✅ |

---

## Ergebnisse

**LOCAL-01 — Full-Rebuild ✅**
`docker compose down -v` + alle `avoc-*`-Images gelöscht + `up --build` von Grund auf: alle 7 Images bauen sauber, alle 14 Container gesund. Ein Sandbox-spezifisches Problem dabei gefunden und dokumentiert (README-Troubleshooting): der `buildx`-Builder-Container cacht `/etc/resolv.conf` beim Start und aktualisiert es nicht bei Netzwerkwechseln — nach langer Laufzeit zeigt es ggf. auf eine tote DNS-IP (`failed to resolve source metadata`). Fix: `docker restart buildx_buildkit_<projekt>-builder0`. Echter Zero-Cache-Rebuild (Base-Images neu von Docker Hub) in dieser Sandbox nicht vollständig verifizierbar (Registry-Zugriff über einen anderen Netzpfad als der Buildx-Builder), aber alle projekteigenen Layer wurden nachweislich neu gebaut.

**LOCAL-02 — SSL-Frage ✅ geschlossen**
Mit echtem Chromium verifiziert: `window.isSecureContext === true` und `getUserMedia()` funktionieren über `http://localhost:3000` ohne Zertifikat (Browser-Ausnahme für `localhost`). `nginx.dev.conf` (HTTP-only) deckt das bereits ab. Der `CONTEXT.MD`-Eintrag „Dev-Stack SSL-Fix … offen" war stale und wurde entfernt.

**LOCAL-03 — Hot-Reload-Pfad ✅**
`make proto-gen-ts` → `npm run dev` end-to-end getestet inkl. echtem Login-Flow über den Vite-Proxy (`/auth/operator/login` → 200 + JWT). Neues `make dev-frontend`-Target ergänzt (Makefile + README) — läuft konsistent zu allen anderen Befehlen vom Repo-Root aus, statt dass `npm run dev` root-versehentlich mit `ENOENT` fehlschlägt (realer Vorfall während des Sprints).

**LOCAL-04 — WebRTC/Video ✅ Blocker behoben, ein Folge-Bug dokumentiert**
Playwright-Test (Login → Fahrzeug wählen → Session starten → WHIP-Publish mit Fake-Kamera → WHEP-Empfang) geschrieben und schrittweise durchgetestet:
1. **Gefunden + gefixt:** `/whip/` und `/whep/` in `nginx.dev.conf` zeigten auf `http://mediamtx:8889` — aber `mediamtx` läuft mit `network_mode: host` und hat keinen Docker-DNS-Eintrag auf `avoc-net` → **502 Bad Gateway bei jedem lokalen Video-Versuch über den Docker-Frontend**. Fix: `host.docker.internal` + `extra_hosts` (analog zum bestehenden `control-server`-Pattern) + statisches `proxy_pass` (der `resolver`-Trick der anderen Locations fragt nur Docker-DNS, das kennt `host.docker.internal` nicht).
2. **Gefunden, nicht gefixt (Nutzer-Entscheidung: dokumentieren statt anfassen):** Nach dem Fix erreicht der WHIP-Publish den SDP-Austausch mit MediaMTX, scheitert dort aber an `setRemoteDescription`: *"Offerer must use actpass value for setup attribute"*. Ursache: `useWebRTC.ts`/`useWHIPSender.ts` erzwingen absichtlich `a=setup:active` im Offer (dokumentierter Pion-v1.19.0-Workaround, `docs/webrtc.md`) — aktuelles Chromium lehnt das als Spec-Verstoß ab. Da `useWebRTC.ts` (WHEP/Video-Empfang) auch produktiv auf AWS läuft, potenziell **kein reines Lokal-Problem** — braucht eigene Untersuchung, siehe `CONTEXT.MD` „Offene Fragen" und Backlog-Eintrag.

**LOCAL-05 — Doku ✅**
README: `make dev-frontend`, WHIP/WHEP-502-Troubleshooting, buildx-DNS-Troubleshooting ergänzt. `CONTEXT.MD`: SSL-Eintrag entfernt, neuer Eintrag zum SDP-`actpass`-Fund.

---

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

---

# Sprint 17 — Multi-Vehicle State Isolation (ADR-026)

Ziel: Drei Safety-kritische Prozess-Singletons (State Machine, DeadmanWatchdog, VehicleACKWatchdog) werden pro Fahrzeug isoliert, damit zwei Operatoren zwei unterschiedliche Fahrzeuge wirklich unabhängig steuern können. SafetyBusWatchdog bleibt global, fächert bei Ausfall aber korrekt auf alle aktiven Fahrzeuge auf. `GET /state` wird durch `GET /vehicles/{id}/state` ersetzt.

Datum: 2026-06-14 | **Status: Backend + Frontend fertig (TDD) ✅ · Deployed 2026-06-16 ✅**
Vorgänger: Sprint 16 ✅
Voraussetzung: [ADR-026](../docs/adr/026-multi-vehicle-state-isolation.md) (Grill-Me-Session 2026-06-14 abgeschlossen)

Arbeitsweise: test-driven auf Wunsch des Nutzers — pro Task zuerst Szenarien/Edge-Cases durchdacht und Tests geschrieben (Red), danach implementiert (Green). Beim Implementieren stellte sich heraus, dass MV-03/04/05 nicht unabhängig voneinander gehen (State Machine ist eine gemeinsam genutzte Safety-Ressource — Teilmigration hätte einen Split-Brain zwischen altem globalem `sm` und neuer Registry erzeugt), daher in einem kohärenten Durchgang umgesetzt.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MV-01 | `vehiclecontext.Registry` — `VehicleContext` (SM + Deadman + ACKTimeoutWatcher + VehicleACKWatchdog), lazy `Get(vehicleID)`, Mutex-safe | M | ✅ |
| MV-02 | `SafetyBusWatchdog` umbauen — global, iteriert `sessionMgr.ActiveVehicleIDs()` bei Ausfall, SAFE_MODE pro betroffenem Fahrzeug | M | ✅ |
| MV-03 | `main.go` — `session/start`, `session/end`, `media/event`, `emergency-stop` auf `vehicleContexts.Get(...)` umgestellt; E-Stop ohne `vehicle_id` = fleet-weit (via `ActiveVehicleIDs()`), mit `vehicle_id` = gezielt | M | ✅ |
| MV-04 | WS-Handler + Command Engine — `sm`/`deadman`/`ackWatcher`-Felder durch Registry-Lookup über `sess.VehicleID` ersetzt | M | ✅ |
| MV-05 | `vehicleconnection.Handler` — Deadman/ACK-Watchdog-Start über `VehicleContext` (Lookup über `claims.Subject`) statt globaler Felder | M | ✅ |
| MV-06 | `GET /vehicles/{id}/state` Endpoint ergänzt. `GET /state` **bewusst nicht entfernt** — bleibt als Compat-Shim (k6 latency.js, ältere Clients), bis MV-07 alle Frontend-Konsumenten migriert hat | S | ✅ (Teil 2 verschoben) |
| MV-07 | Frontend `useSystemState(vehicleId, token)` — Live-Polling vs. Page-Reload-Recovery (`GET /sessions`) getrennt; Unreachable-Banner nutzt `GET /sessions` ohne Fahrzeug | M | ✅ |
| MV-08 | Edge-Case-Tests: 2 Fahrzeuge parallel unabhängiges SAFE_MODE, SafetyBusWatchdog fleet-wide, E-Stop mit/ohne `vehicle_id` | M | ✅ |

**Test-Ergebnis Backend:** 13 neue Unit-Tests (`vehiclecontext_test.go`) + 11 neu geschriebene SafetyBusWatchdog-Tests (`watchdog_test.go`, alte Single-Session-API durch Fleet-Wide-API ersetzt) + 4 neue Integrationstests (`multivehicle_test.go`, gegen echten Docker-Teststack) — alle grün, `-race`-sauber. Gesamte bestehende Unit-Suite (112 Tests) bleibt grün.

**Test-Ergebnis Frontend (MV-07):** 12 neue Tests für `useSystemState` (Vehicle-Polling, Reachability-Probe ohne Fahrzeug, Fehlerzähler-Reset bei Fahrzeugwechsel, Unmount-Cleanup) + 1 neuer Test für `UserManagementPanel` (mehrere aktive Operatoren gleichzeitig gesperrt). Gesamte Frontend-Suite (103 Tests, 10 Dateien) grün, `tsc --noEmit` sauber, Produktionsbuild erfolgreich.

**MV-07 Detailänderungen:**
- `api-client.ts`: `getState()` entfernt (kein Konsument mehr im Frontend), neu `getVehicleState(vehicleId)` → `GET /api/vehicles/{id}/state`; `SystemStateResponse` auf die 4 Layer reduziert (kein `session_id`/`vehicle_id`/`role`/`operator_id` mehr — das kam ohnehin nur aus dem alten globalen `GET /state`)
- `useSystemState(vehicleId, token)`: mit `vehicleId` → pollt das Fahrzeug; ohne → pollt `GET /sessions` rein als Reachability-Probe (Ergebnis verworfen); Fehlerzähler wird bei Fahrzeugwechsel zurückgesetzt
- `App.tsx`: Page-Reload-Recovery nutzt jetzt den ohnehin laufenden `activeSessions`-Poll (eigene Session per `operator_id` finden), entkoppelt von `isSafeMode` (vorher zirkulär: brauchte den State, um den State zu bekommen)
- `UserManagementPanel`: `activeOperatorId?: string` → `activeOperatorIds?: string[]` — mehrere Fahrzeuge können gleichzeitig einen ACTIVE_OPERATOR haben, ein einzelner Wert hätte nur einen davon gesperrt

**Nebenbei gefunden + behoben (unabhängig von ADR-026, eigener Commit):** `tests/integration/services_test.go` + `tests/performance/latency_test.go` + `latency.js` nutzten noch das alte Login-Feld `id` statt `username` und fehlende Auth-Header — Regression aus der Nutzerverwaltungs-Aufgabe, nie mit `go vet`/`go test` geprüft.

**Bewusst nicht angefasst (Scope-Grenze):** `HandoverManager` behält eine eigene, dedizierte einzelne State Machine (`handoverSM`) — er transitioniert nur die OPERATOR-Schicht, nie SAFE_MODE, daher kein Split-Brain-Risiko. Multi-Vehicle-Handover ist nicht Teil von ADR-026 (Folge-Task).

**Abhängigkeitspfad:** MV-01 → MV-02, MV-03, MV-04, MV-05 (mussten kohärent zusammen erfolgen) → MV-06 (teilweise, `GET /state` bewusst erhalten) → MV-07 ✅ → MV-08 ✅

---

# Sprint 16 — Safety Hardening (ADR-009 Lücken geschlossen)

Ziel: Zwei fehlende ADR-009-CRITICAL-Trigger als Watchdog implementieren: VehicleACKWatchdog (Fahrzeug antwortet nicht auf Steuerbefehle) und SafetyBusWatchdog (Safety-Service nicht erreichbar). Außerdem drei Bugfixes im Session-Lifecycle: WS-Disconnect-Race, fehlende CONNECTED→IDLE-Transition, Page-Reload-Recovery.

Datum: 2026-06-14 | **Status: Alle Tasks ✅ · Edge-Case-Tests dokumentiert**
Vorgänger: Sprint 15 ✅

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| SAF-01 | VehicleACKWatchdog — 1s Timeout nach ForwardCommand ohne ACK → SAFE_MODE (ADR-009) | M | ✅ |
| SAF-02 | SafetyBusWatchdog — 2×5s polling /health → SAFE_MODE bei 2 Failures (ADR-009) | M | ✅ |
| BUG-01 | WS-Disconnect-Race: saubes session/end löst kein SAFE_MODE mehr aus | S | ✅ |
| BUG-02 | CONNECTED→IDLE fehlte in validSystemTransitions | S | ✅ |
| BUG-03 | Page-Reload-Recovery: GET /state gibt vehicle_id+role zurück; App restoriert session | S | ✅ |
| TEST-01 | 18 Unit-Tests für beide Watchdogs (alle Edge Cases) | M | ✅ |

---

## Scope-Details

### SAF-01 — VehicleACKWatchdog ✅
- `internal/controlserver/safety/detector.go`: `VehicleACKWatchdog` Struct
  - `Start(sessionID, vehicleID)` — setzt stopped=false, speichert Session-Kontext
  - `Stop()` — stopped=true + cancelt pending Timer (via `sync.Mutex` + `*time.Timer`)
  - `CommandForwarded()` — startet/resettet `AfterFunc(1s, fire)` wenn nicht stopped
  - `ACKReceived()` — cancelt pending Timer
  - `fire()` — prüft stopped-Flag, schreibt Audit, transitioniert → SAFE_MODE, publiziert `EventVehicleACKTimeout`
- `internal/controlserver/command/engine.go`: nach erfolgreichem `ForwardCommand()` → `vehicleACKWatchdog.CommandForwarded()`
- `internal/vehicleconnection/handler.go`: nach `ackStore.Store()` → `vehicleACKWatchdog.ACKReceived()`
- `cmd/control-server/main.go`: erstellt + verdrahtet Watchdog; Start bei `session/start`, Stop bei beiden `session/end`-Pfaden
- `DefaultVehicleACKTimeout = 1 * time.Second`

### SAF-02 — SafetyBusWatchdog ✅
- `internal/controlserver/safety/bus_watchdog.go`: eigenständige Datei
  - `Start()` — cancelt vorherigen Context, startet neue Goroutine via `context.WithCancel`
  - `Stop()` — ruft `cancel()` auf, Goroutine endet beim nächsten `<-ctx.Done()`
  - `run(ctx, ...)` — Ticker-Loop: Failure-Counter; bei `>= threshold` → `triggerSafeMode()`
  - `ping()` — `GET healthURL` mit 3s Timeout; nur HTTP 200 = ok
  - `triggerSafeMode()` — IdempotenzCheck (`sys == StateSafeMode`), Transition, Publish `EventSafetyBusDown`
- `DefaultBusCheckInterval = 5 * time.Second`, `DefaultBusFailThreshold = 2` (= 10s total)

### BUG-01 — WS-Disconnect-Race ✅
- `internal/controlserver/transport/websocket.go`: readLoop defer prüft jetzt `sessionStillActive`
- War: jeder WS-Disconnect → SAFE_MODE; Neu: nur wenn Session noch aktiv (nicht nach sauberem `session/end`)

### BUG-02 — CONNECTED→IDLE fehlte ✅
- `internal/controlserver/statemachine/state.go`: `StateConnected → {StateDegraded, StateSafeMode, StateIdle}`
- War: `session/end` konnte State nicht von CONNECTED auf IDLE setzen → Session-Lifecycle broken bei mehreren Zyklen

### BUG-03 — Page-Reload-Recovery ✅
- `cmd/control-server/main.go`: `GET /state` gibt jetzt `vehicle_id` und `role` zurück (aus aktiver Session)
- `frontend/src/hooks/useSession.ts`: `restoreFromServerState(sessionId, vehicleId, role)` setzt Refs + State
- `frontend/src/App.tsx`: useEffect erkennt "orphaned SAFE_MODE" (Server hat Session, Local-State verloren) → stellt wieder her

### TEST-01 — Edge-Case-Tests ✅
Siehe **Edge-Case-Dokumentation** unten.

---

## Edge-Case-Dokumentation

### VehicleACKWatchdog (`tests/unit/watchdog_test.go`) — 9 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| `TestVehicleACK_TimeoutFires_SafeMode` | CommandForwarded ohne ACK → SAFE_MODE nach Timeout |
| `TestVehicleACK_ACKPrevents_SafeMode` | CommandForwarded + ACKReceived → kein SAFE_MODE |
| `TestVehicleACK_SlidingWindow` | Zweites CommandForwarded resettet Timer |
| `TestVehicleACK_StopCancelsTimer` | Stop() verhindert Transition auch wenn Timer läuft |
| `TestVehicleACK_NoOpAfterStop` | CommandForwarded nach Stop() startet keinen Timer |
| `TestVehicleACK_NoDuplicateTransition` | Bereits SAFE_MODE → kein zweiter Übergang |
| `TestVehicleACK_FreshSessionAfterStop` | Start() nach Stop() funktioniert sauber |
| `TestVehicleACK_ConcurrentRaceSafety` | 10 Goroutinen parallel → kein Data Race |
| `TestVehicleACK_SpuriousACKSafe` | ACKReceived ohne pending Timer → kein Crash |

### SafetyBusWatchdog (`tests/unit/watchdog_test.go`) — 9 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| `TestSafetyBus_ThresholdTriggers_SafeMode` | 2 Failures → SAFE_MODE |
| `TestSafetyBus_PartialFailure_NoTrigger` | 1 Failure + Recovery → kein SAFE_MODE |
| `TestSafetyBus_CounterResets_OnSuccess` | Success resettet Failure-Counter |
| `TestSafetyBus_StopCancels` | Stop() beendet Polling-Goroutine |
| `TestSafetyBus_NoDuplicate_WhenAlreadySafeMode` | Bereits SAFE_MODE → kein zweiter Übergang |
| `TestSafetyBus_503_CountsAsFailure` | HTTP 503 zählt als Failure |
| `TestSafetyBus_ConnectionRefused_CountsAsFailure` | Connection Refused zählt als Failure |
| `TestSafetyBus_StartAfterStop_FreshStart` | Start() nach Stop() startet sauber neu |
| `TestSafetyBus_AlwaysHealthy_NeverTriggers` | 200-Responses → kein SAFE_MODE |

**Testergebnis:** `ok avoc/tests/unit 3.482s` — alle 18 Tests grün.

---

# Sprint 15 — PostgreSQL-Migration + Nutzerverwaltung

Ziel: SQLite vollständig durch PostgreSQL ersetzen. Echte Authentifizierung mit bcrypt. ADMIN-Rolle + Nutzerverwaltung im Frontend. Login-Overlay statt Auto-Connect.

Datum: 2026-06-14 | **Status: Alle Tasks ✅ · Edge-Case-Tests dokumentiert**
Vorgänger: Sprint 14 ✅

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| PG-01 | PostgreSQL als primäre DB (ADR-023) | L | ✅ |
| AUTH-02 | Echte Nutzerverwaltung mit bcrypt + ADMIN-Rolle (ADR-024) | L | ✅ |
| UI-02 | LoginPanel + UserManagementPanel + App-Umstellung | M | ✅ |
| TEST-01 | Edge-Case-Tests: Auth-Handler, parseTokenRole, LoginPanel, UserManagementPanel | M | ✅ |

---

## Scope-Details

### PG-01 — PostgreSQL-Migration ✅
- `pkg/db/postgres.go`: `Open(url)` Factory mit Pool (5 open, 2 idle, 30s lifetime)
- `pkg/audit/postgres_writer.go`: ersetzt `sqlite_writer.go`; `ON CONFLICT (event_id) DO NOTHING` statt `INSERT OR IGNORE`; `$N`-Placeholder
- `internal/vehicleregistry/postgres_store.go`: ersetzt `sqlite_store.go`; `SeedDefault()` idempotent via `ON CONFLICT`
- `go.mod`: `github.com/lib/pq v1.10.9` direkt; `modernc.org/sqlite` + Transitive entfernt
- `docker-compose.yml` + `docker-compose.prod.yml`: postgres:16-alpine Service; `depends_on: service_healthy`
- Durabilität: PostgreSQL `synchronous_commit=on` (Default) ≡ `PRAGMA wal_checkpoint(FULL)` — WriteSync() garantiert ohne extra PRAGMA

### AUTH-02 — Nutzerverwaltung ✅
- `internal/authservice/userstore.go`: `UserStore` Interface + `PostgresUserStore`; bcrypt cost=12 (~300ms/Login)
- `SeedAdmin()` idempotent via `ON CONFLICT (id) DO NOTHING` — sicher bei jedem Restart
- `RoleAdmin` neue Rolle im JWT-Claim `role`
- Auth-Service Endpoints: `GET/POST/DELETE/PATCH /auth/users` — alle hinter `RequireAdmin` Middleware
- `OperatorLogin`: kein "accept any" mehr — `Authenticate()` gegen DB, Rolle aus DB in Token

### UI-02 — Frontend-Umstellung ✅
- `LoginPanel.tsx`: Vollbild-Overlay; Submit-Button disabled bis beide Felder ausgefüllt; Loading-State; Fehleranzeige bei 401
- `UserManagementPanel.tsx`: Tabelle + Formular; Löschen eigenen Accounts gesperrt; Confirm-Dialog vor Löschen; Rolle inline per Dropdown
- `useSession.ts`: `connect(id, password)` statt `connect()` ohne Parameter; `OPERATOR_ID`-Konstante entfernt; `operatorId` als State
- `App.tsx`: kein `useEffect`-Auto-Connect mehr; `LoginPanel` wenn `!token`; "Benutzerverwaltung"-Button nur bei Admin-Token; "Abmelden"-Button
- `parseTokenRole(token)`: Base64-Dekodierung des JWT-Payloads ohne externe Library

### TEST-01 — Edge-Case-Tests ✅
Siehe **Edge-Case-Dokumentation** unten.

---

## Edge-Case-Dokumentation

### Auth-Handler (`internal/authservice/handler_test.go`) — 20 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| Valide Credentials | 200 + JWT |
| Falsches Passwort | 401 |
| Unbekannter User | 401 |
| Deaktivierter User | 401 |
| Malformed JSON | 400 |
| Rolle aus DB im Token | `role`-Claim entspricht DB-Rolle (ADMIN bleibt ADMIN) |
| RequireAdmin: kein Token | 401 |
| RequireAdmin: ungültiger Token | 401 |
| RequireAdmin: OBSERVER-Token | 403 |
| RequireAdmin: ADMIN-Token | 200 (passiert durch) |
| CreateUser: fehlende Felder | 400 |
| CreateUser: doppelte ID | 409 |
| CreateUser: Role leer | 201 + Default OBSERVER |
| DeleteUser: eigener Account | 403 |
| DeleteUser: nicht existenter User | 404 |
| UpdateRole: leere Rolle | 400 |
| SeedAdmin: zweimal aufgerufen | idempotent, kein Fehler, 1 User |
| ValidateToken: Garbage | `{"valid":false}` |
| RefreshToken: ungültiger Token | 401 |
| VehicleRegister: beliebige ID | 200 (kein DB-Check) |

### parseTokenRole (`frontend/src/lib/api-client.test.ts`) — 8 Tests

| Eingabe | Erwartetes Verhalten |
|---------|---------------------|
| JWT mit `role: "ADMIN"` | `"ADMIN"` |
| JWT mit `role: "OBSERVER"` | `"OBSERVER"` |
| JWT ohne `role`-Claim | `""` |
| Kein gültiges JWT (3 Teile, kein JSON) | `""` |
| Leerer String | `""` |
| Nur ein Token-Teil | `""` |
| Ungültiges Base64 im Payload | `""` |
| Payload ist kein JSON | `""` |

### LoginPanel (`frontend/src/components/LoginPanel.test.tsx`) — 8 Tests

| Szenario | Erwartetes Verhalten |
|----------|---------------------|
| Render | ID-Feld, Passwort-Feld, Button vorhanden |
| Beide Felder leer | Button disabled |
| Nur ID ausgefüllt | Button disabled |
| Nur Passwort ausgefüllt | Button disabled |
| Beide Felder ausgefüllt | Button enabled |
| Submit mit korrekten Daten | `onLogin(id, password)` aufgerufen |
| Login-Fehler (onLogin wirft) | Fehlermeldung "Ungültige Zugangsdaten" |
| Während Login | Button zeigt "Anmelden…", ist disabled |
| Zweiter Versuch nach Fehler | Fehlermeldung verschwindet beim neuen Versuch |

### UserManagementPanel (`frontend/src/components/UserManagementPanel.test.tsx`) — 10 Tests

| Szenario | Erwartetes Verhalten |
|----------|---------------------|
| Laden | Alle API-User werden angezeigt |
| Eigener Account | Kein Löschen-Button |
| Fremder Account | Löschen-Button vorhanden |
| Löschen + Bestätigung | `deleteUser()` aufgerufen |
| Löschen + Ablehnung | `deleteUser()` NICHT aufgerufen |
| Anlegen-Button leer | disabled |
| Anlegen mit Daten | `createUser()` mit korrekten Parametern |
| API-Fehler | Fehlermeldung angezeigt |
| Rolle ändern | `updateUserRole()` aufgerufen |
| Panel schließen | `onClose()` aufgerufen |

---

# Sprint 14 — Security & Observability (Archiv)

Ziel: REST-Endpoints JWT-geschützt. Control- und Video-Kanal werden separat mit Status + Latenz angezeigt. Frontend signalisiert wenn das Backend nicht erreichbar ist.

Datum: 2026-06-13 | **Status: Auth/UI/ROB fertig ✅ · OBS-01 offen**
Vorgänger: Sprint 13 ✅ (Dev-Stack Stabilisierung & Log-Korrelation)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUTH-01 | JWT-Pflicht auf REST-Endpoints im control-server | M | ✅ |
| UI-01 | Dual-Channel Status: Control + Video separat mit Latenz in ConnectionPanel | M | ✅ |
| ROB-01 | Backend-nicht-erreichbar-Zustand im Frontend (Banner + Zustandsschutz) | S | ✅ |
| OBS-01 | Vehicle "zuletzt gesehen" Heartbeat-Timestamp (Bonus, wenn Zeit bleibt) | S | 🔲 |

### Nachtrag (Bugfix vor Sprint-Start)
- **E-Stop Race Condition**: `WSClient.disconnect()` setzt `ws.onclose = null` vor `ws.close()` — verhindert unbeabsichtigten Reconnect bei absichtlichem Disconnect (Emergency Stop, Session End)

---

## Scope-Details

### AUTH-01 — JWT-Pflicht REST-Endpoints ✅
- `requireJWT(secret []byte)` Middleware in `cmd/control-server/main.go` (curried `http.HandlerFunc`-Wrapper)
- **Geschützt (11 Endpoints):** `POST /session/start`, `POST /session/end`, `POST /handover/request`, `POST /handover/confirm`, `POST /handover/cancel`, `POST /media/event`, `POST /emergency-stop`, `GET /audit/events`, `GET /recording/`, `POST /vehicles`, `DELETE /vehicles/{id}`
- **Bewusst offen:** `GET /state`, `GET /health`, `GET /vehicles`, `GET /ice-config`, `GET /vehicle/ack/latest/{id}`, `POST /log` (Fire-and-forget Logger, muss auch vor Login feuern können)
- Frontend: `Authorization: Bearer <token>` in `api-client.ts` für `startSession`, `endSession`, `emergencyStop`, `reportMediaState`
- `SafetyPanel.tsx` erhält `token: string | null` Prop aus `App.tsx`

### UI-01 — Dual-Channel Status ✅
- `useWebRTC.ts`: `RTCPeerConnection.getStats()` alle 1s, `candidate-pair` mit `r.nominated === true` → `currentRoundTripTime × 1000` → `videoLatencyMs`
- `VideoPanel.tsx`: `onVideoLatency?: (ms: number | null) => void` Callback-Prop
- `ConnectionPanel`: zwei Zeilen — **Control** (WS-ACK-RTT) + **Video** (ICE-RTT); `— ms` solange kein Stream aktiv
- `App.tsx`: `useState<number | null>(null)` für `videoLatency`, weitergegeben über VideoPanel-Callback

### ROB-01 — Backend nicht erreichbar ✅
- `useSystemState.ts`: `failCount` Ref + `UNREACHABLE_THRESHOLD = 3` (1,5s) → `unreachable: boolean` im Return
- `App.tsx`: rotes Banner bei `state.unreachable`; `ControlPanel` disabled wenn `isUnreachable`
- Verhindert dass Operator glaubt zu steuern während Backend tot ist

### OBS-01 — Vehicle Heartbeat (Bonus) 🔲
- MQTT-Telemetry kommt schon alle ~100ms — letzter Timestamp reicht
- `AckBadge` → "Fahrzeug aktiv vor 2s" auch ohne aktive Steuerbefehle
