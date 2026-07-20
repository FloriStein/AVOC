# ADR-035: control-server Hexagonal-Migration — Vorbereitung (Testaufbau vor Refactor)

Status: Accepted, **Testaufbau abgeschlossen (Sprint 46, 2026-07-20)** — Migration selbst noch
nicht begonnen, siehe "Nächste Schritte".

## Kontext

ADR-031 (Hexagonale Architektur-Migration) hat `control-server` seit der ursprünglichen
Bestandsaufnahme (2026-07-16) explizit ausgeklammert:

> `control-server` | Ältester, größter, safety-kritischster Service. ... sicherheitskritische
> Orchestrierung (State-Transition/Audit/Publish) inline in HTTP-Handler-Closures an ~6 Stellen
> mit Reihenfolge-Semantik nur in Kommentaren. | Höchstes Risiko, geringste Testabdeckung genau
> dort, wo Migration am meisten Schaden anrichten könnte

Mit fleet-service (Pilot, Sprint 33), auth-service (Schritt 2, Sprint 37), telemetry-service
(Schritt 3, Sprint 43) und den optionalen Use-Case-Extraktionen (Sprint 45) sind alle Services mit
einer echten Infrastruktur-Abhängigkeit (Postgres/JWT-Lib/MQTT) migriert. `control-server` blieb
als letzter, größter und riskantester Kandidat übrig. DECISIONS.MD hielt dafür fest: "bräuchte
eigenes ADR + Testabdeckung-Aufbau zuerst". Dieses ADR ist dieser erste Schritt.

## Korrigierte Bestandsaufnahme (2026-07-20)

Die ursprüngliche Einschätzung "geringste Testabdeckung" war bei genauerer Prüfung **nur zur
Hälfte richtig** — eine reine Per-Package-Coverage-Messung (`go test ./internal/controlserver/...
-cover`) zeigt 0% für so gut wie jedes Unterpaket, weil die umfangreiche bestehende Testsuite in
`tests/unit/*.go` (2711 Zeilen: `watchdog_test.go`, `safety_test.go`, `multioperator_test.go`,
`vehiclecontext_test.go`) diese Pakete von außen testet und in dieser einfachen Messung nicht
mitgezählt wird. Mit `-coverpkg` sichtbar gemacht (`go test ./tests/unit/...
-coverpkg=./internal/controlserver/...`):

| Bereich | Tatsächliche Coverage | Bewertung |
|---|---|---|
| `internal/controlserver/statemachine` | ~100% (bis auf eine kleine 0%-Methode) | Gut abgedeckt — State Machine (SAFE_MODE-Übergänge, 4 orthogonale States) ist der sicherheitskritischste Einzelbaustein und bereits solide getestet. |
| `internal/controlserver/safety` (5 Watchdogs: Deadman/ACKTimeout/VehicleACK/Auth/SafetyBus) | Größtenteils 85-100% | Gut abgedeckt. |
| `internal/controlserver/session` (Manager, HandoverManager) | Größtenteils 90-100% | Gut abgedeckt. |
| `internal/controlserver/command` (Engine) | ~70-90% | Gut abgedeckt. |
| `internal/controlserver/vehiclecontext` (Registry) | ~85-100% | Gut abgedeckt. |
| **`cmd/control-server/main.go` (919 Zeilen, `package main`)** | **0% — jede einzelne Funktion**, inkl. Bootstrap (`loadConfig`/`newAuditWriter`/`newVehicleStore`/`newControlServer`/`buildHandlers`/`newMux`/`main`) und aller ~25 HTTP-Handler | **Die eigentliche, echte Lücke.** |
| `internal/controlserver/authcheck` (46 Zeilen, DB-Query gegen `users`) | 0% | Kleine, aber reale Lücke. |
| `internal/controlserver/transport/websocket.go` (370 Zeilen, Fahrzeug-facing WS) | ~5% (nur die kleine eigene `websocket_test.go`) | Reale Lücke, eigene große Sicherheitsfläche. |

Gesamtbild: Das eigentliche Risiko liegt nicht in der Domain-Logik (State Machine, Watchdogs,
Session-Verwaltung — alle bereits solide getestet), sondern in der **HTTP-Orchestrierungsschicht**
selbst — `cmd/control-server/main.go` verdrahtet diese bereits getesteten Bausteine, aber diese
Verdrahtung (welcher Handler ruft was in welcher Reihenfolge auf) war bis zu diesem Sprint nie
selbst getestet. Das deckt sich mit der ursprünglichen Beobachtung ("Orchestrierung inline in
HTTP-Handler-Closures ... Reihenfolge-Semantik nur in Kommentaren") — nur dass die Handler
inzwischen (Sprint 29, GOSTYLE Rule 2.3/2.4) bereits aus `main()` in `controlServer`-Methoden
extrahiert wurden, was sie strukturell testbar macht, ohne dass das bisher genutzt wurde.

## Entscheidung: Testaufbau zuerst, kein Big-Bang-Refactor

Analog zum Präzedenzfall Sprint 39 (Testabdeckung für `safety-service`/`webrtc-sfu`/`recording`
vor jeder Migrationsdiskussion dort): **kein Produktivcode-Refactor in diesem Sprint.** Erst wenn
die HTTP-Orchestrierungsschicht durch Tests abgesichert ist, lässt sich eine
Handler-Struct-Extraktion (analog `fleetservice.Handler`/`authservice.Handler`) risikoarm
durchführen — vorher würde jede Umstrukturierung blind gegen eine ungetestete
Sicherheitsfläche laufen.

**Testaufbau in diesem Sprint (Sprint 46):** neues `cmd/control-server/main_test.go` mit einer
Fixture (`newTestControlServer`), die einen echten `*controlServer` aus den bereits getesteten
Bausteinen konstruiert (`session.NewManager`, `vehiclecontext.NewRegistry`, `command.NewEngine`
usw. — keine Postgres-Verbindung nötig), `safetyPub` zeigt auf einen lokalen `httptest.Server`
statt auf einen echten safety-service (analog `tests/unit/safety_bus_integration_test.go`). Damit
abgedeckt:

- `requireJWT` (Auth-Gate: fehlender Header, malformed Token, falsches Secret, abgelaufener Token,
  gültiger Token inkl. Rollen-Weiterreichung)
- `handleSessionStart`/`advanceVehicleToActiveOperator` (Fahrzeug nicht verbunden → 409, OBSERVER
  darf keine neue Session claimen → 403, ACTIVE_OPERATOR-Pfad schaltet Watchdogs scharf und fährt
  die State Machine bis CONNECTED hoch, OBSERVER-Join-Pfad lässt sie unangetastet)
- `handleSessionEnd` (Beendigung per `session_id`, unbekannte `session_id` als No-Op, Legacy-Pfad
  ohne `session_id` für die fleet-weite Beendigung)
- `handleEmergencyStop` (fahrzeugspezifisch vs. fleet-weit, `TriggerEmergencyStop` beim
  Test-Server angekommen)
- `internal/controlserver/authcheck/checker_test.go` — `DATABASE_URL`-gated gegen die echte
  `users`-Tabelle (aktiv/inaktiv/gelöscht)

**Echter Fund während des Testaufbaus:** Ein Fahrzeug ohne aktive Session steht in SYSTEM=IDLE.
`validSystemTransitions` (`internal/controlserver/statemachine/state.go`) erlaubt `IDLE→SAFE_MODE`
**nicht** (nur `IDLE→CONNECTING`) — `handleEmergencyStop`s `TransitionSystem(StateSafeMode)` wird
für so ein Fahrzeug lautlos abgelehnt (nur als Warn-Log sichtbar), obwohl `TriggerEmergencyStop`
weiterhin an safety-service gemeldet wird und der Endpoint weiterhin 202 antwortet. Dieses
Verhalten existierte schon vor diesem Sprint — es wurde nur erstmals durch einen Test sichtbar.
**Kein Fix in diesem Sprint** (Scope-Entscheidung: Vorbereitung, kein Produktivcode-Refactor) —
siehe "Nächste Schritte" für die Einordnung als Kandidat.

## Nicht Teil dieses Sprints

- `internal/controlserver/transport/websocket.go` (eigene, große Sicherheitsfläche — verdient
  einen eigenen Folge-Sprint, nicht nebenbei mitgezogen)
- Die restlichen ~20 HTTP-Handler in `main.go` (Handover-Request/Confirm/Cancel, Media-Event,
  Log, Audit-Events, Recording, MediaMTX-Auth, Vehicle-ACK, Vehicles-CRUD, Sessions, ICE-Config,
  Logout) — Folge-Sprints nach Priorität, gleiche Fixture wiederverwendbar
- Die DB-touchende Bootstrap-Logik selbst (`loadConfig`/`newAuditWriter`/`newVehicleStore`/
  `newControlServer`/`main()`) — bräuchte eigene Postgres-Test-Infrastruktur (z. B. containerisiert
  in CI), anderer Scope als reine Handler-Orchestrierung
- Der Fix für den oben dokumentierten `IDLE→SAFE_MODE`-Fund — eigener Entscheidungspunkt, ob/wie
  `validSystemTransitions` oder `handleEmergencyStop` angepasst werden soll
- Die eigentliche Hexagonal-Migration (Handler-Struct-Extraktion) selbst

## Nächste Schritte (Entscheidungspunkte, kein Automatismus)

1. Testabdeckung für die restlichen `main.go`-Handler und `transport/websocket.go` fortsetzen
   (eigene Folge-Sprints) — je vollständiger die Orchestrierungsschicht getestet ist, desto
   risikoärmer wird jede spätere Umstrukturierung.
2. Erst danach: Entscheidungspunkt, ob eine Handler-Struct-Extraktion (analog
   `fleetservice.Handler`/`authservice.Handler`, `controlServer`-Methoden bleiben strukturell
   bereits vor) tatsächlich lohnend ist, oder ob der bestehende Zustand (dünne Methoden auf einem
   bereits gut entkoppelten `controlServer`-Struct) für dieses safety-kritische System ausreicht.
3. Getrennter Entscheidungspunkt: ob/wie der `IDLE→SAFE_MODE`-Fund behoben wird (z. B.
   `validSystemTransitions` um `StateIdle: {StateConnecting, StateSafeMode}` erweitern, oder
   `handleEmergencyStop` so anpassen, dass IDLE-Fahrzeuge übersprungen statt lautlos ignoriert
   werden) — sicherheitsrelevant genug, um nicht nebenbei im nächsten Testsprint mit entschieden
   zu werden.
