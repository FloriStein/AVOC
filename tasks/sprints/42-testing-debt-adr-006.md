> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 42 — Testing-Debt aus ADR-006-Bestandsaufnahme schließen

**Freigabe (2026-07-19):** Nutzer priorisiert bei der Testing-Strategie-Bestandsaufnahme
(siehe EPIC "CI-Gates einführen") CI-Gates zuerst (Sprint 41), lässt aber zwei kleinere Funde als
offene Folgepunkte in `DECISIONS.MD` stehen (Zeilen 82/83). Sprint 42 schließt genau diese zwei
Funde. Ausgeführt in separatem Worktree (`feature/fleet-service-foundation-testdebt`), parallel zu
Sprint 41 (separater Worktree, keine Dateiüberschneidung im Code — nur die Doku-Dateien
`current-sprint.md`/`backlog.md`/`DECISIONS.MD`/`done.md` werden in beiden Strängen aktualisiert;
Zusammenführung erfolgt manuell später).

**Vorrecherche (2026-07-19):**
- **Fund 1 — Concurrency-Test-Lücke `internal/webrtcsfu/sfu_test.go`:** `SFU` (`internal/webrtcsfu/
  sfu.go:44-50`) hat ein `sync.RWMutex` (`s.mu`), das `peers`/`routing`/`state`-Maps schützt —
  echter geteilter Zustand, alle Zugriffe laufen bereits korrekt durch `s.mu.Lock()`/`RLock()`.
  `sfu_test.go` (Sprint 39, 10 Tests) prüft aber nur sequenzielles Verhalten, kein Test ruft
  `HandleSessionEvent`/`registerOperatorSubscription`/`removePeer` aus mehreren Goroutinen
  gleichzeitig auf — anders als `internal/safetyservice/bus_test.go` (selber Sprint 39), das mit
  `TestBus_ConcurrentPublishAndRead` genau so einen Test hat (WaitGroup + Timeout-Channel-Helfer
  `waitOrTimeout`). Fix: identisches Testmuster in `sfu_test.go` nachbauen (Helfer lokal dupliziert,
  analog zum akzeptierten Duplikat-Präzedenzfall aus Sprint 38 `MQTTAUTH-05`, drei identische
  MQTT-Test-Helfer in separaten Testdateien).
- **Fund 2 — `tests/unit/safety_test.go` testet nicht den echten `safetyservice.Bus`:** Die 20
  Tests der "Safety Test Suite" (Datei-Header nennt sie explizit "the safety gate in CI") bauen
  `statemachine.Machine` + `mocks.MockSafetyPublisher` zusammen (`newTestSetup`, Zeile 21-33) und
  prüfen nur, dass `Publisher.PublishEvent(...)` mit dem richtigen `SafetyEventType` aufgerufen
  wird (z.B. Zeile 89 `assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEventType())`).
  Der eigentliche Bus (`internal/safetyservice.Bus`, Produktivcode in `cmd/safety-service/main.go`)
  wird dabei nie erreicht — `internal/controlserver/safety.HTTPPublisher` (Produktiv-Implementierung
  von `Publisher`, `internal/controlserver/safety/http_publisher.go`) schickt Events per HTTP-POST
  an `/safety/event`, das erst serverseitig `bus.PublishSafetyEvent(event)` aufruft
  (`cmd/safety-service/main.go:38-48`, `newSafetyMux`). `newSafetyMux` liegt in `package main` und
  ist daher aus `tests/unit` (package `unit_test`) nicht importierbar — Fix baut keinen
  Produktivcode um, sondern verdrahtet in einer neuen Testdatei einen minimalen lokalen
  `httptest.Server`-Handler für `POST /safety/event` (identische zwei Zeilen wie in `newSafetyMux`,
  bewusst dupliziert statt Produktivcode zu exportieren — kein Scope für einen Hexagonal-Schritt
  hier) und lässt `HTTPPublisher` (mit `baseURL` = Test-Server-URL) echte Events an einen echten
  `safetyservice.NewBus()` schicken, verifiziert über `bus.GetSafetyState()`.
- **CLAUDE.MD-Leitplanken**: Abschnitt 17 (Teststandard) fordert für Typ M/L explizit
  "Nebenläufigkeit" als Fallgruppe (Fund 1) sowie Integrationstests gegen reale Abhängigkeiten statt
  In-Memory-Mocks wo sinnvoll (Fund 2 — `HTTPPublisher` gegen echten `Bus` statt nur gegen
  `MockSafetyPublisher`).

Datum: 2026-07-19 | **Status: ✅ Abgeschlossen**
Vorgänger: Sprint 39 ✅ (Testabdeckung `safety-service`/`webrtc-sfu`/`internal/recording`, siehe
`tasks/sprints/39-testabdeckung-sicherheitsrelevanter-services.md`); Sprint 40/41 laufen parallel in
anderen Worktrees, nicht Teil dieses Sprints.
Branch/Worktree: `feature/fleet-service-foundation-testdebt` (Basis:
`feature/fleet-service-foundation-sprint35`).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| SFUCONC-01 | `internal/webrtcsfu/sfu_test.go`: neuer Concurrency-Test (mehrere Goroutinen rufen `HandleSessionEvent`/`registerOperatorSubscription`/`removePeer` gleichzeitig auf), lokaler `waitOrTimeout`-Helfer analog `bus_test.go`, `go test -race` grün. | S | ✅ | — |
| SAFETYBUS-01 | Neue Testdatei `tests/unit/safety_bus_integration_test.go`: minimaler lokaler `httptest.Server`-Handler für `POST /safety/event` (dupliziert `newSafetyMux`s zwei Zeilen, kein Produktivcode-Umbau), `HTTPPublisher` gegen echten `safetyservice.NewBus()` verdrahtet, mind. 2 ADR-006-CRITICAL-Szenarien (Dead-man-Timeout, ACK-Timeout) verifiziert über `bus.GetSafetyState()`. | S/M | ✅ | — |
| SAFETYBUS-02 | Datei-Header-Kommentar in `tests/unit/safety_test.go` präzisieren: bestehende 20 Tests decken Trigger-Logik (State-Machine → `Publisher`) ab, nicht den Bus selbst; Verweis auf die neuen Bus-Integrationstests aus `SAFETYBUS-01`. `docs/adr/006-testing-strategy.md` falls dort die Suite beschrieben wird, ebenfalls präzisieren. | S | ✅ | SAFETYBUS-01 |
| TESTDEBT-VERIFY-01 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./... -race` (mind. 2x gegen Flakiness, CLAUDE.MD §17), `DECISIONS.MD`-Zeilen 82/83 auf ✅, `tasks/backlog.md`-Status-Update. | S | ✅ | SFUCONC-01, SAFETYBUS-01, SAFETYBUS-02 |

**Nicht Teil dieses Sprints:** Umbau von `cmd/safety-service/main.go`s `newSafetyMux` in ein
exportiertes/testbares Konstrukt (Hexagonal-artiger Schritt, eigener Entscheidungspunkt falls
später gewünscht), Abdeckung aller 5 ADR-006-CRITICAL-Szenarien gegen den echten Bus (nur die 2
wichtigsten Dead-man/ACK-Timeout, Rest bleibt Mock-basiert), Concurrency-Tests für weitere Packages
über `webrtcsfu` hinaus.

## Ergebnisse

**SFUCONC-01 (`internal/webrtcsfu/sfu_test.go`):** Neuer Test
`TestSFU_ConcurrentSessionEventsAndPeerOps` — 20 parallele Trios von Goroutinen rufen
`HandleSessionEvent`, `registerOperatorSubscription` und `removePeer` gleichzeitig auf derselben
`SFU`-Instanz auf (60 Goroutinen gesamt). Lokaler `waitOrTimeout`-Helfer (5s Timeout) analog
`internal/safetyservice/bus_test.go`. Assertion beschränkt sich bewusst auf den deterministischen
Teil des Ergebnisses (`s.state["session-1"] == EventOperatorAssigned` — alle `HandleSessionEvent`-
Aufrufe publizieren denselben Event-Typ), da `peers`/`routing` legitim vom Goroutinen-Interleaving
abhängen. Ursprünglich mit `n = 50` implementiert; das hat unter `go test ./... -race` (volle Suite,
alle Pakete parallel) einen seltenen, nicht-reproduzierbaren Flake in einem unveränderten Nachbarpaket
(`internal/fleetgateway`, zeitbasierte Assertions) ausgelöst — vermutlich CPU/Speicher-Druck durch
50 echte `webrtc.PeerConnection`-Instanzen gleichzeitig unter `-race`-Instrumentierung. Auf `n = 20`
reduziert (spart >50% der echten PeerConnection/ICE-Ressourcen, exercised `s.mu` weiterhin unter
echter Nebenläufigkeit) — danach 3 aufeinanderfolgende volle `go test ./... -race`-Läufe grün, kein
Flake mehr reproduzierbar.

**SAFETYBUS-01 (`tests/unit/safety_bus_integration_test.go`, neu):** `newBusIntegrationTestServer`-
Helfer baut einen echten `safetyservice.NewBus()` hinter einem lokalen `httptest.Server`, der
`newSafetyMux`s `POST /safety/event`-Handler dupliziert (zwei Zeilen, kein Produktivcode-Umbau,
`cmd/safety-service/main.go` unverändert). Drei Tests:
- `TestSafetyBusIntegration_DeadmanTimeout_ReachesRealBusOverHTTP` — echter `HTTPPublisher` +
  `csafety.NewDeadmanWatchdog` gegen eine `CONNECTED`-`statemachine.Machine`, verifiziert per
  `assert.Eventually`/`require.Eventually` (Timer/HTTP-Roundtrip laufen asynchron zum Testgoroutine)
  über `bus.GetSafetyState()`.
- `TestSafetyBusIntegration_ACKTimeout_ReachesRealBusOverHTTP` — analog mit
  `csafety.NewACKTimeoutWatcher`, `CommandACKed()` bewusst nicht aufgerufen (Timeout muss feuern).
- `TestSafetyBusIntegration_MalformedEventBody_Returns400AndBusUnaffected` — Fehlerpfad-Ergänzung
  (CLAUDE.MD §17 Fallgruppe "Fehlerhafte Eingaben"): kaputter Body → 400, Bus-Zustand unverändert.
- `connectedStateMachine`-Helfer treibt die Maschine erst durch `CONNECTING`→`AUTHENTICATED`→
  `CONNECTED` (wie `connectSession` in `safety_test.go`) — ein bare `statemachine.New()` hätte die
  `SAFE_MODE`-Transition der Watchdogs aus `IDLE` zurückgewiesen (nicht falsch, aber irreführendes
  Warn-Log; das Ziel dieses Tests ist der Bus, nicht die State-Machine-Transitionsvalidierung selbst).

Bewusst nicht abgedeckt (siehe "Nicht Teil dieses Sprints" oben): die übrigen 3 ADR-006-CRITICAL-
Szenarien (No-Operator, Emergency-Stop, Auth-Invalid/Safety-Bus-Down) bleiben Mock-basiert in
`safety_test.go` — kein Bedarf, da die HTTP→Bus-Kopplung bereits an den zwei wichtigsten Szenarien
verifiziert ist und identisch für alle Event-Typen funktioniert (derselbe Handler, dieselbe
`PublishSafetyEvent`-Methode).

**SAFETYBUS-02 (`tests/unit/safety_test.go`, `docs/adr/006-testing-strategy.md`):**
Datei-Header-Kommentar präzisiert — benennt jetzt explizit, dass die 20 Tests nur die Trigger-Logik
(State Machine → `Publisher`-Interface, gegen `MockSafetyPublisher`) abdecken, nicht den echten Bus,
mit Verweis auf `safety_bus_integration_test.go`. ADR-006 Teil 3 um denselben Hinweis ergänzt
("Ergänzung (SAFETYBUS-02, Sprint 42)").

**TESTDEBT-VERIFY-01:**
- `go build ./...` — sauber.
- `go vet ./...` — sauber.
- `go test ./... -race -count=1` — **3 aufeinanderfolgende volle Läufe grün** für alle Pakete außer
  `avoc/tests/integration` (vorbestehend, benötigt laufenden Docker-Test-Stack auf Ports
  18080-18085, nicht Teil dieses Sprints — durch Vergleichslauf auf dem unveränderten Basis-Commit
  bestätigt: identische Fehlschläge dort ohne jede Sprint-42-Änderung).
- `DECISIONS.MD` Zeilen 83/84 (Concurrency-Test-Lücke, Safety-Test-Suite-Fund) auf ✅ Sprint 42;
  Zeile 82 (ADR-006/§17-Gesamtbefund) präzisiert — nur der CI-Gates-Teil (Sprint 41, separater
  Strang) bleibt dort offen.
- `tasks/backlog.md`: EPIC-Überschrift und alle 4 Tasks auf ✅ Sprint 42.

**Bewusst nicht getestet / außerhalb des Scopes** (siehe auch "Nicht Teil dieses Sprints"):
Umbau von `newSafetyMux` in ein exportierbares Konstrukt; Abdeckung aller 5 statt 2
ADR-006-CRITICAL-Szenarien gegen den echten Bus; Concurrency-Tests für weitere Packages über
`webrtcsfu` hinaus; die vorbestehenden Docker-abhängigen Integrationstests (unverändert, siehe oben).
