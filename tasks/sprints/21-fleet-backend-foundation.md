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
