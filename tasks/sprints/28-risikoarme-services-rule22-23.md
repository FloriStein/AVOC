# Sprint 28 — Risikoarme Services: Rule 2.2 + 2.3

Ziel: Zweiter von drei Sprints (27/28/29) zum Rollout des Go Coding Style Guide
(`docs/go-style-guide.md`, EPIC "Go Coding Style Guide Rollout" in `tasks/backlog.md`). Sprint 28
zerlegt alle verbleibenden `main()`-Funktionen >50 Zeilen (Rule 2.2) und Funktionen/Methoden >4
Parameter (Rule 2.3) außerhalb von `control-server` — bewusst getrennt vom sicherheitskritischen
Kern (eigener Sprint 29 mit Pflicht-Grill-Me, siehe `tasks/backlog.md`), damit dieser Sprint ohne
Extra-Grill-Me und mit Standard-Sorgfalt (Typ S/M, Normal-Track) durchlaufen kann. Baut auf den in
Sprint 27 eingeführten Helpern `pkg/db.OpenAndWait`/`pkg/env.Require` auf (`auth-`/`fleet-service`
nutzen diese bereits in `main()`).

**Keine Signaturänderung nach außen, kein Verhaltenswechsel** (CLAUDE.MD Abschnitt 15) — jeder
Task endet mit vollem Testlauf des betroffenen Service/Pakets + Diff-Review gegen genau diese
Vorgabe. Bei jeder `main()`-Zerlegung: Reihenfolge-Semantik 1:1 erhalten, Helper in derselben Datei
(`docs/go-style-guide.md` Rule 2.2). Explizit nicht Teil dieses Sprints: `control-server` (Sprint
29, höchstes Risiko, eigene Grill-Me-Pflicht), Interface-Änderungen (Phase 2, wartet auf
ADR-031/HEX-05).

Datum: 2026-07-17 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 27 ✅ (dieser Branch zweigt von `feature/fleet-service-foundation-gostyle`
ab, NICHT von `feature/fleet-service-foundation` direkt, da GOSTYLE-03/GOSTYLE-06 auf
`pkg/db.OpenAndWait`/`pkg/env.Require` aufbauen)
Branch: `feature/fleet-service-foundation-gostyle28`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| GOSTYLE-03 | `auth-service`: `main()` (67 Zeilen) auf <50 Zeilen zerlegen | S | ✅ |
| GOSTYLE-04 | `safety-service`: `main()` (57 Zeilen) zerlegen | S | ✅ |
| GOSTYLE-05 | `telemetry-service`: `main()` (57 Zeilen) zerlegen | S | ✅ |
| GOSTYLE-06 | `fleet-service`: `main()` (129 Zeilen) zerlegen | M | ✅ |
| GOSTYLE-07 | `webrtc-sfu`: `main()` (76 Zeilen) zerlegen + `internal/webrtcsfu/sfu.go` `SFU.SubscribeOperator` (74)/`CreateVehicleOffer` (56) prüfen/zerlegen | M | ✅ |
| GOSTYLE-08 | `vehicle-mock`: `runConnection` (73 Zeilen, 5 Parameter) zerlegen + Parameter in Struct bündeln (Rule 2.2 + 2.3) | S | ✅ |
| GOSTYLE-09 | `internal/recording/memory_recorder.go`: 3 `Record*`-Methoden mit je 6 Parametern auf Parameter-Struct umstellen (Rule 2.3) | S | ✅ |
| GOSTYLE-10 | `internal/controlserver/safety/bus_watchdog.go`: `NewSafetyBusWatchdog` (6 Parameter) auf Options-Struct umstellen | S | ✅ |
| GOSTYLE-11 | `internal/vehicleconnection/handler.go`: `NewHandler` (5 Parameter) prüfen/ggf. bündeln | S | ✅ |

## Ergebnisse

**GOSTYLE-03 — `auth-service` ✅ (kein Codeeingriff nötig)**
`main()` liegt nach den in Sprint 27 eingeführten Helpern (`pkgdb.OpenAndWait`, `env.Require`/
`env.OptionalOr`) bereits bei 42 Zeilen (`golangci-lint`/`funlen` bestätigt: keine Findung mehr für
diese Datei) — die ursprünglich in der Bestandsaufnahme gezählten 67 Zeilen waren die Vor-Sprint-27-
Größe. Rule 1.2 ("Justify Every Abstraction") verbietet eine Zerlegung ohne Größenproblem, daher
keine weitere Änderung; nur verifiziert (Build/Lint/Test).

**GOSTYLE-04 — `safety-service` ✅**
`main()` (55 Zeilen) in `newSafetyMux(bus) *http.ServeMux` (Routenregistrierung, Reihenfolge 1:1)
und `logSafetyEvent` (vormals inline Subscribe-Closure) zerlegt. `main()` jetzt ~15 Zeilen.

**GOSTYLE-05 — `telemetry-service` ✅**
Analog zu GOSTYLE-04: `main()` (55 Zeilen) in `newTelemetryMux(client) *http.ServeMux` extrahiert.

**GOSTYLE-06 — `fleet-service` ✅**
`main()` (92 Zeilen nach Sprint-27-Helpern, vorher 129) in drei Helper zerlegt: `newFleetMux
(handler)` (Routenregistrierung), `subscribeVehicleStatus(gw, store, hub, alertEngine)` und
`subscribeVehicleAlerts(gw, store, hub)` (die beiden MQTT-Gateway-Callback-Registrierungen, FLEET-
06/07). `connectGatewayWithRetry` blieb unverändert (war bereits ein separater Helper).
Reihenfolge der Registrierungen/Aufrufe 1:1 erhalten.

**GOSTYLE-07 — `webrtc-sfu` ✅**
`main()` (73 Zeilen) in vier Route-Handler-Funktionen zerlegt (`handleSessionEvent`,
`handleVehicleOffer`, `handleOperatorSubscribe`, `handleHealth`) plus `newSFUMux(sfu)` für die
Registrierung — analog zum Safety-/Telemetry-Muster, aber mit einer Zwischenstufe (erste Version
von `newSFUMux` lag mit reinen Closures noch bei 64 Zeilen, daher zusätzlich in benannte Funktionen
aufgeteilt). `internal/webrtcsfu/sfu.go`: `CreateVehicleOffer` (54→~27 Zeilen) und
`SubscribeOperator` (72→~30 Zeilen) teilen sich jetzt `s.newPeerConnection()` (ICE-Konfiguration)
und `negotiateAnswer(pc, sdpOffer)` (Offer/Answer-Austausch inkl. ICE-Gathering) — beide Methoden
hatten exakt denselben Verhandlungsblock dupliziert. Zusätzlich `registerOperatorSubscription`
extrahiert; dabei bewusst die bestehende (leicht überraschende) Semantik erhalten, dass die Peer-
Connection eines Operators **immer** ersetzt wird, auch im "already subscribed"-Fall — nur der
Eintrag in der Routing-Liste wird in diesem Fall übersprungen (Diff-Review gegen CLAUDE.MD
Abschnitt 15 hat das explizit verglichen, um keine Verhaltensänderung einzuschleusen).

**GOSTYLE-08 — `vehicle-mock` ✅**
`runConnection` (73 Zeilen, 5 Parameter) auf `connectionParams`-Struct umgestellt (Rule 2.3) und in
`startTelemetryLoop(mqttClient, vehicleID, st)`, `receiveCommands(conn, vehicleID, st)` und
`sendCommandAck(conn, vehicleID, cmd, st)` zerlegt (Rule 2.2). Bei `sendCommandAck` wurde die
ursprüngliche Continue-vs-Return-Unterscheidung bewusst erhalten: ein Marshal-Fehler wird geloggt
und wie zuvor übersprungen (Helper gibt `nil` zurück, Aufrufer läuft weiter), ein Write-Fehler
bricht die Verbindung wie zuvor fatal ab. `envOr` bewusst unangetastet gelassen — GOSTYLE-08s
Aufgabenbeschreibung umfasst nur die `runConnection`-Zerlegung/Parameter-Struct, keine
`pkg/env`-Migration; das wäre eine Scope-Erweiterung über den in `tasks/backlog.md` definierten
Task hinaus gewesen.

**GOSTYLE-09 — `internal/recording/memory_recorder.go` ✅**
Drei neue Parameter-Structs (`ControlEventParams`, `StateSnapshotParams`, `SafetyEventParams`) in
`recorder.go` neben dem `SessionRecorder`-Interface ergänzt (das Interface musste mitgeändert
werden, da es dieselben drei Methoden mit denselben 6-Parameter-Signaturen deklariert). Zwei
Call-Sites in `cmd/control-server/main.go` (`RecordStateSnapshot`, `RecordSafetyEvent`) mechanisch
auf die neuen Structs umgestellt — reine Signaturanpassung, keine `control-server`-main()-
Zerlegung (die bleibt Sprint 29). `RecordControlEvent` hat aktuell keinen Call-Site (bereits vor
diesem Sprint so, siehe Bestandsaufnahme "Dead Ports" in `tasks/backlog.md" — außerhalb des
Scopes, das ist Phase-2/GOSTYLE-IF-01-Thema).

**GOSTYLE-10 — `internal/controlserver/safety/bus_watchdog.go` ✅**
Neue `SafetyBusWatchdogOptions`-Struct (6 Felder) ersetzt die 6 Positionsparameter von
`NewSafetyBusWatchdog`. Beide Call-Sites angepasst: `cmd/control-server/main.go` (mechanische
Anpassung, keine main()-Zerlegung) und `tests/unit/watchdog_test.go`.

**GOSTYLE-11 — `internal/vehicleconnection/handler.go` ✅**
Geprüft: `NewHandler` (5 Parameter: `jwtSecret`, `vehicleContexts`, `publisher`, `registry`,
`ackStore`) überschreitet Rule 2.3 um 1 Parameter — analog zu GOSTYLE-10 auf `HandlerOptions`-
Struct umgestellt. Einziger externer Call-Site (`cmd/control-server/main.go`, mechanisch
angepasst, Builder-Chain `.WithVehicleAdder(...)` bleibt unverändert erhalten).

**Verifikation (alle neun Tasks zusammen) ✅**
`go build ./...`, `go vet ./...` sauber. `gofmt -l .` findet weiterhin genau dieselben 13
vorbestehenden unformatierten Dateien wie vor diesem Sprint (per `git stash`/`gofmt -l`-Vergleich
gegen den Branch-Ausgangspunkt bestätigt, exakt identische Liste — keine neuen Abweichungen durch
diese Änderungen). `golangci-lint run --issues-exit-code=0` zeigt für alle neun bearbeiteten
Dateien keine `funlen`/`argument-limit`-Findings mehr; verbleibende Findings betreffen ausschließlich
`control-server` (Sprint 29) und Testdateien (nicht Teil des Sprint-28-Scopes). `go test ./...`:
alle Unit-Test-Pakete grün; `tests/integration/...` schlägt ohne laufenden Docker-Stack erwartungs-
gemäß mit "connection refused" fehl (kein Code-Problem). Sicherheitsrelevante Watchdog-Tests
zusätzlich zweimal hintereinander mit `-race` gelaufen (CLAUDE.MD Abschnitt 17) — beide Male grün,
keine Data Races.

Vollständiger `make test-integration`-Lauf gegen den echten Docker-Test-Stack
(`tests/docker-compose.test.yml`, enthält `control-server`, `auth-`, `safety-`, `fleet-service`,
`vehicle-mock`, `mosquitto`, `postgres`): alle 29 Integrationstests PASS, 3 vorbestehende
WebSocket-Skips (identisch zu Sprint 27 — minimaler Test-Stack hat kein echtes WebRTC/SFU).
`telemetry-service` und `webrtc-sfu` sind **nicht** Teil dieses Test-Stacks (begründet ausgelassen,
per Vorgabe bei Nichtvorhandensein) — für beide wurden Build/Vet/Lint/Unit-Verifikation lokal
durchgeführt, ein Integrationstest gegen den echten Prozess war für diese zwei Services mangels
Stack-Anbindung nicht möglich.

Ein zusätzlicher, aus dem Initial-Commit bereits eingecheckter Kompilat-Artefakt (`vehicle-mock`-
Binary im Repo-Root, unrelated zum Style-Guide-Scope) wurde durch einen `go build ./...`-Lauf
versehentlich überschrieben und vor dem Abschluss wieder auf den committeten Stand zurückgesetzt
(`git checkout -- vehicle-mock`) — kein Bestandteil dieses Sprints, nur zur Sauberhaltung des
Diffs vermerkt.

**Bewusst nicht in diesem Sprint:** `control-server`-`main()`-Zerlegung (Sprint 29, höchstes
Risiko, eigene Grill-Me-Pflicht), Interface-Segregation nach Rule 4.2/4.3 (Phase 2, wartet auf
ADR-031/HEX-05), `gofmt -w .` (separater Bonus-Task außerhalb des Style-Guide-Scopes),
`vehicle-mock`s `envOr` (nicht Teil von GOSTYLE-08s Aufgabenbeschreibung).
