# Sprint 33 — Indoor-Kartenrendering (ADR-034) + Hexagonal-Pilot-Start (HEX-01..05)

Ziel: Sprint 32 hat AP2 auf 6 von 7 Anforderungsbereichen gebracht — `AP2-02`
(Indoor-Kartenrendering) blieb als einziger frischer, nicht extern blockierter Kandidat übrig.
Triage 2026-07-18 (CLAUDE.MD Abschnitt 10):

- **AP2-02** (Indoor-Kartenrendering, Typ L): Grill-Me-Session vor Umsetzung durchgeführt (siehe
  unten) — neues [ADR-034](../docs/adr/034-indoor-vehicle-position.md), danach in 4 kleine
  S/M-Teilaufgaben zerlegt (Vorbild: HEX-01..06s "bewusst kleinteilig geschnitten"-Muster), statt
  als ein großer L-Task durchgezogen (Sprint-33-Vorgabe: Tasks strikt im S/M-Rahmen halten, siehe
  WEBRTC-10-Erfahrung aus Sprint 32).
- **HEX-01..06** (Hexagonale Architektur-Migration, Pilot `fleet-service`): Backlog-Text sagt
  "nachgelagert nach AP2/AP3" — AP2 ist zu diesem Zeitpunkt zu 6/7 fertig, AP3 existiert im Repo
  weiterhin nicht als eigenes EPIC. Frage explizit dem Nutzer vorgelegt (kein Code-Task, CLAUDE.MD
  Abschnitt 4): Nutzerentscheidung — jetzt starten. HEX-01..05 (bereits klein geschnitten, S/M)
  in diesen Sprint aufgenommen; HEX-06 bleibt optionaler Entscheidungspunkt nach HEX-05 im
  Backlog.
- **FLEET-01** (Handshake-Autonomie-Rückgabe), **MV-10** (VehicleContext-GC): weiterhin ohne
  neuen Anlass zurückgestellt, nicht erneut geprüft (bereits in Sprint 32 geprüft).
- **AP1-01..03** (Workshop Professur Logistik), **GOSTYLE-IF-01..06** (abhängig von HEX-05, damit
  erst nächsten Sprint relevant): weiterhin extern blockiert bzw. nachgelagert, nicht angefasst.

**Grill-Me AP2-02 (2026-07-18):** Schema-Vorabprüfung ergab einen Befund, der über die reine
Backlog-Beschreibung hinausging — `vehicle_status.position_zone_id` wird von **keinem** Schreiber
gesetzt (`cmd/vehicle-mock/fleet_simulator.go` liefert nur Outdoor-GPS, keine Zonen-Logik). Zwei
Fragen an den Nutzer, beide mit der empfohlenen Option beantwortet:
- **Datenmodell:** freie Koordinaten `vehicle_status.position_x/y` (analog `stations.position_x/y`,
  ADR-029-Muster) statt stationsgebundener Position.
- **Scope:** dieser Sprint liefert nur Datenmodell + Backend + Rendering, verifiziert mit manuell
  gesetzten Testdaten/Seed-Werten. Simulator-Bewegungslogik für Indoor-Zonen ist bewusst
  ausgeklammert und als Folge-Task in `tasks/backlog.md`/`DECISIONS.MD` vorgemerkt — hält den Task
  klein und unabhängig verifizierbar.

**Keine Signaturänderung nach außen ohne Zweck** (CLAUDE.MD Abschnitt 15) — Ausnahme ist HEX-02
(Typ-Wechsel `Handler.store` auf Interface ist der Taskzweck, HTTP-Verhalten bleibt unverändert).
`docs/go-style-guide.md` gilt für alle Go-Tasks in diesem Sprint.

Datum: 2026-07-18 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 32 ✅ (dieser Branch zweigt von `feature/fleet-service-foundation-sprint32` ab,
da Sprint 32 zum Startzeitpunkt noch nicht in `feature/fleet-service-foundation` gemergt war)
Branch: `feature/fleet-service-foundation-sprint33`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AP2-02-01 | `vehicle_status.position_x/y`-Migration | S | ✅ |
| AP2-02-02 | `UpsertVehicleStatus`/`GET`-Endpunkte um `position_x/y` erweitern | S | ✅ |
| AP2-02-03 | Indoor-Beispielzone + Stationen im Seed-Skript | S | ✅ |
| AP2-02-04 | Frontend Indoor-Kartendarstellung (SVG-Koordinatensystem) | M | ✅ |
| HEX-01 | `FleetStore`-Interface definieren + Compile-Time-Check | S | ✅ |
| HEX-02 | `Handler.store` auf `FleetStore`-Interface umstellen | S | ✅ |
| HEX-03 | `FakeFleetStore` (In-Memory) implementieren | M | ✅ |
| HEX-04 | Handler-Tests auf `FakeFleetStore` migrieren | M | ✅ |
| HEX-05 | Verifikation ohne `DATABASE_URL` + ADR-031/DECISIONS.MD-Update | S | ✅ |

## Ergebnisse

**HEX-01..05 — Hexagonal-Pilot `fleet-service` abgeschlossen ✅ ([ADR-031](../docs/adr/031-hexagonal-architecture-migration.md) Update)**
Nutzerentscheidung (Sprint-Kickoff): trotz "nachgelagert nach AP2/AP3" jetzt starten, da AP2 zu
6/7 fertig war und AP3 im Repo weiterhin nicht als EPIC existiert. Umsetzung exakt im
ADR-031-Zielbild, eine bewusste Abweichung (kein `context.Context` in den Methoden — Konsistenz
mit dem Rest der Codebasis, siehe ADR-031-Update):
- `internal/fleetservice/store.go`: neues `FleetStore`-Interface (19 Methoden, deckt
  `*PostgresFleetStore` vollständig ab — Interface-Segregation auf Handler-Bedarf ist bewusst
  GOSTYLE-IF-*-Scope, nicht Teil dieses Piloten), Compile-Time-Check
  `var _ FleetStore = (*PostgresFleetStore)(nil)`.
- `internal/fleetservice/handler.go`: `Handler.store`/`NewHandler` von `*PostgresFleetStore` auf
  `FleetStore` umgestellt — reiner Typwechsel, kein HTTP-Verhalten geändert.
- `internal/fleetservice/fake_store.go` (neu): `FakeFleetStore`, In-Memory, thread-safe
  (`sync.Mutex`). Wiederverwendet die package-internen Sentinel-Fehler/Transitionstabelle
  (`ErrTaskNotFound`, `ErrInvalidTransition`, `taskTransitionSources`,
  `allowedTaskTransitions`) direkt aus `store.go`, statt sie zu duplizieren — dadurch verhält sich
  `Handler` unabhängig vom konkreten Store identisch (ADR-030-Zustandsmaschine, Duplikat-/
  FK-Validierung für `AddZone`/`AddStation`/`CreateTask`).
- `internal/fleetservice/handler_test.go`: alle 30 zuvor `DATABASE_URL`-gebundenen Tests (Ist-Zahl,
  ursprüngliche Schätzung im Backlog war ~20 von 24) auf `newFakeStore(t)` migriert; die
  Postgres-spezifische Fixture-Cleanup-Logik in `newTaskStatusTestFixture` entfällt (In-Memory-Store
  stirbt mit dem Test, kein Cross-Test-State). `requirePostgresStore`-Helper entfernt (unbenutzt
  nach der Migration), damit `database/sql`/`os`/`lib/pq`-Imports.
- **Store-Layer-Integrationstests bleiben bewusst Postgres-gebunden** (`store_test.go`,
  `integration_test.go`, `edgecases_test.go`, `lifecycle_test.go`, `positionhistory_test.go`) —
  sie testen `PostgresFleetStore` selbst (echte SQL-Constraints, `ON DELETE CASCADE`,
  Race-Verhalten), nicht `Handler`, und erfüllen weiterhin CLAUDE.MD Abschnitt 17s
  "mindestens ein Test über die echte Prozessgrenze" für dieses Paket.
- `go build ./...`/`go vet ./...` reposweit sauber. `go test ./internal/fleetservice/...`
  **läuft jetzt tatsächlich durch (nicht nur übersprungen) ohne `DATABASE_URL`** — das eigentliche
  Erfolgskriterium des Piloten (vorher: alle 30 Tests `SKIP`). Zusätzlich gegen echten Postgres
  verifiziert (`docker compose -f tests/docker-compose.test.yml up -d postgres`, danach `down`):
  komplettes Paket zweimal hintereinander grün, keine Regression an den weiterhin
  Postgres-gebundenen Tests.
- ADR-031 aktualisiert (Status "Pilot abgeschlossen"), `DECISIONS.MD` nachgezogen,
  `tasks/backlog.md` HEX-01..05 auf ✅ Sprint 33.
- **Entscheidungspunkt für Folgesession:** ob/in welcher Reihenfolge auth-service/
  telemetry-service folgen (ADR-031 "Offene Punkte") — bewusst nicht in diesem Sprint entschieden.

**AP2-02 — Indoor-Kartenrendering abgeschlossen ✅ ([ADR-034](../docs/adr/034-indoor-vehicle-position.md))**
Typ L bestätigt (Datenstruktur-Änderung), Grill-Me-Session vor Umsetzung (siehe oben) — Nutzer
wählte freie Koordinaten (`vehicle_status.position_x/y`, analog `stations`) statt
stationsgebundener Position, und Backend/Rendering-Scope ohne Simulator-Bewegung. In 4
Teilaufgaben zerlegt:
- **AP2-02-01 (Datenmodell):** `vehicle_status.position_x/position_y` (`DOUBLE PRECISION`,
  nullable) — idempotente `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (gleiches Muster wie
  `vehicleTypeColumn`/`taskStatusChangedByColumn`) plus direkte Deklaration in `schema`'s
  `CREATE TABLE` für frische Installationen. `VehicleStatus`/`FleetVehicle`-Structs erweitert.
- **AP2-02-02 (Backend):** `UpsertVehicleStatus` (INSERT + `ON CONFLICT DO UPDATE`),
  `ListVehicleStatus`, `ListVehiclesWithStatus` (LEFT-JOIN-Query) um `position_x/y` erweitert;
  `FakeFleetStore.ListVehiclesWithStatus` nachgezogen. Kein neuer HTTP-Endpoint nötig — die
  Felder laufen durch die bereits bestehende `GET /fleet/vehicles`-Antwort mit.
- **AP2-02-03 (Beispieldaten):** `scripts/seed-fleet-demo.sh` um eine zweite, unabhängig
  idempotente Indoor-Zone (`zone-lager-indoor`, 3 Stationen mit `position_x/y`) erweitert. Dabei
  einen Idempotenz-Bug im ursprünglichen Skript-Aufbau gefunden und behoben: das bestehende
  Outdoor-Zonen-`exit 0` bei "Zone existiert bereits" hätte das gesamte Skript beendet, bevor die
  neue Indoor-Zone geprüft/angelegt wird, sobald die Outdoor-Zone schon existiert. Skript auf zwei
  unabhängige Funktionen (`seed_outdoor_zone`/`seed_indoor_zone`, je eigene Existenzprüfung)
  umgebaut. Gegen den echten Dev-Stack verifiziert (`make up`, siehe unten): beide Zonen legen sich
  beim ersten Lauf an, zweiter Lauf überspringt beide unabhängig voneinander korrekt.
- **AP2-02-04 (Frontend):** neue `FleetIndoorMap.tsx` + `lib/fleet-indoor-map.ts` (reine,
  testbare Filter-/Farb-Helper, mirrored an `fleet-map.ts`). Anders als `FleetMap.tsx` (Leaflet,
  geo-referenziert) rendert die Indoor-Karte zwei gestapelte `<svg viewBox>`-Ebenen im
  zoneneigenen Koordinatensystem — Hintergrundgeometrie (`zone.svg_geometry`, wiederverwendet
  `parseSvgGeometry` aus `fleet-map.ts`) plus eine Marker-Ebene aus nativen `<circle>`-Elementen
  für Stationen/Fahrzeuge. Kein Leaflet, kein `geo_bounds` — Indoor-Zonen sind nicht
  geo-referenziert. In `FleetOverview.tsx` unterhalb der Outdoor-Karte eingehängt; rendert
  `null`, wenn keine Indoor-Zone existiert (kein leerer Platzhalter für ein optionales Extra).
  `FleetVehicle`-TS-Interface um `position_x/y` ergänzt (`frontend/src/lib/api-client.ts`).
- `npx tsc -b --noEmit` sauber (nach `make proto-gen-ts` — frischer Checkout hatte `src/gen/`
  noch nicht generiert, siehe `.gitignore`-Kommentar dort, kein Zusammenhang mit diesem Task).
  `npx vitest run`: 288/288 grün (14 neue Fälle: 8 `fleet-indoor-map.test.ts` für die reinen
  Helper — Zonen-/Stationen-/Fahrzeugfilterung inkl. Negativfällen ohne `position_x/y` bzw. ohne
  passende `position_zone_id`, Farb-Mapping inkl. unbekanntem Modus —, 7 `FleetIndoorMap.test.tsx`
  für Empty-State ohne Indoor-Zone, fehlende Kartengeometrie, Stationsfilterung nach Zone,
  Fahrzeug-Filterung, Klick-Handler, Auswahl-Hervorhebung, Mehrfachzonen-Rendering). Zweimal
  hintereinander stabil.
- **Gegen den echten Dev-Stack verifiziert** (`make up`, danach `down` — nicht
  `tests/docker-compose.test.yml`, da der Seed-Skript-Login `/auth/...` und `/fleet/...` über
  denselben nginx-`BASE_URL` braucht, den nur der Dev-Stack bereitstellt): `seed-fleet-demo.sh`
  zweimal ausgeführt (Idempotenz bestätigt, siehe AP2-02-03), Inhalt über die echte
  `GET /fleet/zones`/`GET /fleet/stations`-API geprüft. **Mit echtem Chromium via
  chrome-devtools-Tooling** eingeloggt und die Fleet-Overview-Seite geladen: "Indoor-Zonen"-Sektion
  rendert die Lager-Zone mit allen drei Stationen an den korrekten Koordinaten, keine
  Konsolenfehler/-warnungen. Kein Fahrzeug-Marker sichtbar — erwartet, da `position_zone_id`/
  `position_x/y` von keinem Schreiber gesetzt werden (siehe "Bewusst nicht abgedeckt").
- **Bewusst nicht abgedeckt:** Befüllung von `position_zone_id`/`position_x/y` durch eine
  Simulator- oder reale Gateway-Anbindung (Grill-Me-Entscheidung, siehe oben) — als Folge-Task in
  `tasks/backlog.md`/`DECISIONS.MD` vorgemerkt. Kein automatisierter Browser-Test (Playwright) für
  die Indoor-Karte — manuell gegen den echten Dev-Stack verifiziert, analog zum bereits etablierten
  Muster bei `FleetMap.tsx`/`seed-fleet-demo.sh` (Sprint 23).

**Nebenbei behoben (kein eigener Task, während der Arbeit an anderen Dateien gefunden):**
`docs/milestones/meilenstein-2-dashboard.md` (Sprint 32, `AP2-05`) wies `AP2-02` noch als
einzigen offenen Punkt aus — auf den neuen Stand nachgezogen (Abschnitt 2 Statustabelle,
Abschnitt 6 von "Offener Punkt" auf "Umgesetzt" umformuliert, ADR-034 in der Referenzentabelle
ergänzt).

**Zwischenfall während der Verifikation (transparent dokumentiert):** Beim ersten Hochfahren des
Dev-Stacks für die AP2-02-04-Browser-Verifikation wurde versehentlich `docker compose` direkt statt
`make up` aufgerufen (fehlendes `COMPOSE_PROJECT_NAME`) — dadurch wurden kurzzeitig Container unter
dem geteilten Projektnamen `avoc` neu gebaut, der laut `docker compose ls` zuvor an den
Basis-Worktree `controlcenter-aws` gebunden war. Sofort bemerkt, Stack wieder heruntergefahren,
danach korrekt mit `make up` (isolierter `controlcenter-aws-sprint33`-Projektname) neu gestartet.
Keine Daten verloren, aber falls im Basis-Worktree parallel `make up` lief, wurde dessen Stack
kurzzeitig ersetzt und gestoppt — dem Nutzer im Gespräch bereits mitgeteilt.
