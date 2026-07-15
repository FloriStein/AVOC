# Sprint 21 — Fleet Backend Foundation (fleet-service, Multi-Vehicle-Simulation)

Ziel: Das in `ADR-027/028/029` entworfene Fleet-Backend real aufsetzen, damit AP2
(Web-Dashboard-Frontend) gegen echte Bewegtdaten statt gegen nichts entwickelt werden kann.
Bewusst kein Frontend-Task in diesem Sprint — Backend-Fundament zuerst, Dashboard-UI folgt in
Sprint 22.

Datum: 2026-07-14 | **Status: In Bearbeitung 🔄**
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
| FLEET-08 | Unit-Tests `fleet-service` (Schema, API-Handler, Alert-Engine) analog bestehendem Testmuster (`testing`+`testify`) | S | 🔲 |

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
