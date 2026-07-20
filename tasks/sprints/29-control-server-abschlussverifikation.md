# Sprint 29 — `control-server` (hohes Risiko) + Abschlussverifikation

Ziel: Dritter und letzter Sprint (27/28/29) zum Rollout des Go Coding Style Guide
(`docs/go-style-guide.md`, EPIC "Go Coding Style Guide Rollout" in `tasks/backlog.md`). Sprint 29
zerlegt `main()` (683 Zeilen) im sicherheitskritischsten Service (`control-server`, laut ADR-031
"höchstes Risiko, geringste Testabdeckung") sowie zwei weitere >50-Zeilen-Funktionen
(`WSHandler.readLoop`/`ServeWS`, `Engine.Handle`), und schließt mit einer Abschlussverifikation
über alle drei Sprints ab.

**Pflicht-Grill-Me vor GOSTYLE-12** (CLAUDE.MD Abschnitt 5, Typ L) am 2026-07-18 durchgeführt, vier
Fragen, alle empfohlenen Optionen bestätigt:
- **Extraktionsmuster:** `controlServer`-Struct + Methoden statt eigenständiger Funktionen mit
  Parametern — konsistent mit bereits bestehenden Mustern im selben Package (`transport.WSHandler`,
  `command.Engine`, `vehicleconnection.Handler` folgen alle Builder+Methoden), vermeidet
  Rule-2.3-Konflikte bei den ~15 geteilten Abhängigkeiten.
- **Granularität:** Alle ~20 Routen einheitlich extrahiert, auch triviale (GET /health,
  /ice-config, /dev/whip-key) — main() muss von ~680 auf <50 Zeilen, das geht nur vollständig.
- **Reihenfolge-Doku:** Konsolidierter Kommentarblock über der Bootstrap-Sequenz in
  `newControlServer` (zusätzlich zu den bereits vorhandenen Einzelkommentaren) — macht die von
  ADR-031 als Kernrisiko benannte, bisher nur in Kommentaren dokumentierte Reihenfolge explizit.
- **Testkadenz:** `make test-integration` nach jedem der vier großen Blöcke (Init/DB/Audit,
  Core-Components, Route-Handler-Extraktion, Server-Start) statt nur am Sprint-Ende — main.go hat
  keine direkten Unit-Tests (package `main`, nicht importierbar), einzige Absicherung ist der
  Docker-Integrationstest.

**Keine Signaturänderung nach außen, kein Verhaltenswechsel** (CLAUDE.MD Abschnitt 15) — jeder Task
endet mit vollem Testlauf + Diff-Review gegen genau diese Vorgabe. Bei der `main()`-Zerlegung:
Reihenfolge-Semantik 1:1 erhalten, Helper in derselben Datei (`docs/go-style-guide.md` Rule 2.2 —
projektspezifische Anmerkung zu `control-server`).

Datum: 2026-07-18 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 28 ✅ (dieser Branch zweigt von `feature/fleet-service-foundation-gostyle28` ab,
NICHT von `feature/fleet-service-foundation` direkt, da GOSTYLE-16 alle drei Sprints 27/28/29
zusammen prüfen muss und `control-server`s `main.go` bereits die in Sprint 28 umgestellten
Call-Sites — `recording.StateSnapshotParams`/`SafetyEventParams`, `SafetyBusWatchdogOptions`,
`vehicleconnection.HandlerOptions` — enthält)
Branch: `feature/fleet-service-foundation-gostyle29`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| GOSTYLE-12 | `control-server`: `main()` (683 Zeilen) zerlegen | L | ✅ |
| GOSTYLE-13 | `internal/controlserver/transport/websocket.go`: `WSHandler.readLoop`/`ServeWS` zerlegen | M | ✅ |
| GOSTYLE-14 | `internal/controlserver/command/engine.go`: `Engine.Handle` zerlegen | M | ✅ |
| GOSTYLE-16 | Abschlussverifikation Phase 1 gesamt (Sprint 27+28+29) | S | ✅ |

## Ergebnisse

**GOSTYLE-12 — `control-server`: `main()` zerlegt ✅**
`main()` (683 Zeilen) auf 17 Zeilen reduziert. Struct+Methoden-Muster wie im Grill-Me festgelegt:
- `serverConfig`-Struct + `loadConfig()` bündelt alle Umgebungsvariablen (Rule 2.4 — zu viele
  Einzelwerte für Direct-Return). Reihenfolge der beiden `env.Require`-Aufrufe (`JWT_SECRET` vor
  `DATABASE_URL`) 1:1 erhalten — bestimmt, welche fehlende Pflichtvariable zuerst gemeldet wird.
- `newAuditWriter(db)` und `newVehicleStore(db, conn)` kapseln die beiden Postgres-mit-Fallback-
  Blöcke (Audit Writer, Vehicle Registry) inkl. aller Log-Zeilen unverändert.
- `controlServer`-Struct bündelt alle ~15 geteilten Abhängigkeiten (Rule 2.3 — vermeidet
  Parameterlimit-Konflikte bei ~20 Route-Handlern); `newControlServer()` verdrahtet sie in exakt
  der bisherigen Reihenfolge (dokumentiert jetzt zusätzlich in einem konsolidierten GoDoc-
  Kommentarblock über der Funktion — die im Grill-Me beschlossene Reihenfolge-Doku-Maßnahme).
  `startSafetyBusWatchdog()` und `(*controlServer).buildHandlers()` als weitere Unter-Helper
  ausgelagert, da `newControlServer()` sonst selbst >50 Zeilen geblieben wäre (53→~40).
- Alle ~20 Routen einheitlich zu `(*controlServer)`-Methoden extrahiert (auch triviale wie
  `handleHealth`, `handleICEConfig`), `(*controlServer) newMux()` registriert sie nur noch.
  `handleSessionStart` (58 Zeilen) zusätzlich in `advanceVehicleToActiveOperator` aufgeteilt.
- `requireJWT` bewusst unverändert als eigenständige Funktion belassen (kein struct-Zugriff nötig,
  Rule 1.1 — keine Abstraktion ohne Grund).
- Alle 12 Handler-Methoden + 5 Konstruktor-/Helper-Funktionen unter 50 Zeilen
  (`golangci-lint run` — `funlen`/`argument-limit` — 0 Findings für `cmd/control-server/`).

**GOSTYLE-13 — `websocket.go`: `ServeWS`/`readLoop` zerlegt ✅**
`ServeWS` (61→~24 Zeilen): `authenticateWS()` (JWT+Session-Auflösung, schreibt bei Fehler die
HTTP-Response selbst und gibt `ok=false` zurück) und `recoverFromSafeMode()` (SAFE_MODE-Reconnect-
Pfad) extrahiert. `readLoop` (104→~18 Zeilen): `handleWSDisconnect()` (deferred Disconnect-Logik,
inkl. `auditWSDisconnect()`-Unter-Helper für den Audit-Write) und `processWSMessage()` (Message-
Loop-Body) extrahiert. Neuer `wsConn`-Struct bündelt `conn`/`vc`/`claims`/`sess`/`isObserver`
(Rule 2.3 — sonst 5 Parameter für `processWSMessage`/`handleWSDisconnect`). Die
Continue-vs-Return-Verzweigung in `handleWSDisconnect` (aktiv/inaktive Session,
SAFE_MODE-Transition ja/nein) wurde als Guard-Clause-Kette umgeschrieben, aber auf exakt dieselbe
Fallunterscheidung geprüft (leerer sessionStillActive=false-Zweig verhält sich identisch zum
Original-`else`).

*Testlücke transparent gemacht (CLAUDE.MD Abschnitt 17):* `internal/controlserver/transport` hat
keine eigenen Unit-Tests, und alle drei Integrationstests, die `/ws` anfahren
(`TestIntegration_SessionLifecycle_StartAndEnd`, `_MediaFailed_...`, `_EmergencyStop_...`), skippen
im minimalen Test-Stack — **nicht** wegen fehlendem WebRTC/SFU (wie bei den bereits bekannten
3 SFU-bedingten Skips), sondern weil ihre eigene Dial-URL nur `?token=` statt `?token=&session_id=`
mitgibt und dadurch am `session_id`-Required-Check scheitert (vorbestehender Test-Setup-Gap, im
Testcode selbst kommentiert: "skip full WS in integration test"). Damit hatte `readLoop` vor UND
nach diesem Sprint keinerlei automatisierte Abdeckung. Da dies die riskanteste Datei im Scope ist,
zusätzlich manuell end-to-end gegen den echten Docker-Test-Stack verifiziert (Scratch-Skript, nicht
Teil des Repos): Login → `POST /session/start` → WS-Dial **mit** `session_id` → Nachricht senden →
Ack empfangen (116 Bytes, korrekt geparst) → Verbindung schließen → `GET /vehicles/{id}/state`
bestätigt `SAFE_MODE` (beweist `handleWSDisconnect`s SAFE_MODE-Transition inkl. Audit-Write-Pfad
funktioniert unverändert). Diese Testlücke selbst zu schließen (dauerhafter Testfix in
`services_test.go`) ist außerhalb des Scopes von GOSTYLE-13 (reine Strukturaufgabe) — als
Folge-Task für `tasks/backlog.md` vorgemerkt.

**GOSTYLE-14 — `engine.go`: `Engine.Handle` zerlegt ✅**
`Handle` (82→~42 Zeilen): `handleEmergencyStop(vc, sess)` (Audit-Write-vor-Transition + SAFE_MODE-
Transition + Safety-Event-Publish, 2 Parameter) und `forwardMovementCommand(vc, sess, cmd, rawMsg)`
(4 Parameter) extrahiert. Bei `forwardMovementCommand` wurde die ursprüngliche
`if forwarder != nil && VehicleID != "" { if err != nil {log} else {watchdog} }`-Verschachtelung
als Guard-Clause-Kette umgeschrieben (De-Morgan-äquivalent: früher Return, wenn Forwarder fehlt
oder VehicleID leer ist) — exakt dieselbe Bedingung, nur ohne Verschachtelung.

**Verifikation (alle vier Tasks zusammen) ✅**
`go build ./...`, `go vet ./...` sauber. `gofmt -l .` findet 12 unformatierte Dateien — Vergleich
gegen den exakten Branch-Ausgangspunkt (`e21f6d6`, per temporärem `git worktree add --detach`
geprüft) bestätigt: alle 12 sind Teilmenge der dortigen 13 vorbestehenden Dateien, keine neue
hinzugekommen. Die 13. (`cmd/control-server/main.go`) ist als Nebeneffekt der vollständigen
Neufassung jetzt zufällig `gofmt`-clean — keine gezielte Formatierungsänderung, nur eine
Beobachtung. `golangci-lint run --issues-exit-code=0 ./...`: 0 Findings mehr für `control-server`
oder einen der drei bearbeiteten Pfade — alle verbleibenden `funlen`/`argument-limit`-Findings
betreffen ausschließlich Testdateien (`internal/fleetservice/*_test.go`,
`tests/integration/fleet_*_test.go`, `tests/performance/latency_test.go`), die in keinem der drei
Sprints (27/28/29) im Scope waren. Damit ist die Rule-2.2/2.3-Bereinigung für den gesamten
Produktionscode (`cmd/`, `internal/`, `pkg/`) abgeschlossen.

`go test ./...`: alle Unit-Test-Pakete grün; `tests/integration/...` schlägt ohne laufenden
Docker-Stack erwartungsgemäß mit "connection refused" fehl. Sicherheitsrelevante/nebenläufige
Tests (`Watchdog`/`Safety`/`StateMachine`/`Deadman`/`ACK`-Suiten, 34 Tests) zusätzlich zweimal
hintereinander mit `-race` gelaufen (CLAUDE.MD Abschnitt 17) — beide Male grün, keine Data Races.

Vollständiger `make test-integration`-Lauf gegen den echten Docker-Test-Stack: zweimal ausgeführt
(einmal nach GOSTYLE-12 allein, einmal final nach allen vier Tasks zusammen) — beide Male identisch
26/29 PASS + 3 vorbestehende WebSocket-Skips (identisch zu Sprint 27/28, minimaler Test-Stack hat
kein echtes WebRTC/SFU — siehe oben zur GOSTYLE-13-spezifischen Testlücke für die genaue Ursache
dieser 3 Skips). `telemetry-service` und `webrtc-sfu` sind weiterhin nicht Teil dieses Test-Stacks
(unverändert seit Sprint 27/28) — für `control-server` (alle drei geänderten Dateien) wurde
zusätzlich der manuelle WS-Smoketest aus GOSTYLE-13 durchgeführt.

**Diff-Review gegen "nur Struktur, kein Verhaltenswechsel" (CLAUDE.MD Abschnitt 15):**
Zusätzlich zur manuellen Zeile-für-Zeile-Prüfung während der Zerlegung ein automatisierter
Abgleich aller entfernten/hinzugefügten Zeilen in `main.go`: jeder `http.Error(...)`- und
`w.WriteHeader(...)`-Aufruf kommt nach der Zerlegung in identischer Anzahl und identischem Wortlaut
vor (kein Statuscode/keine Fehlermeldung verloren oder verändert). Bootstrap-Reihenfolge (DB → Audit
→ Core-Components → Handlers → Mux → Server-Start) 1:1 erhalten, wie im konsolidierten GoDoc-Block
auf `newControlServer` dokumentiert.

**Bewusst nicht in diesem Sprint:** dauerhafter Fix der `session_id`-Lücke in den drei
WS-Integrationstests (siehe GOSTYLE-13 — als Folge-Task vorgemerkt), Interface-Segregation nach
Rule 4.2/4.3 (Phase 2, wartet auf ADR-031/HEX-05), `gofmt -w .` für die verbleibenden 12
vorbestehenden Dateien (separater Bonus-Task außerhalb des Style-Guide-Scopes).

**Damit ist Phase 1 des EPICs "Go Coding Style Guide Rollout" (Sprints 27/28/29, Rules 1–3 +
Duplikat-Extraktion + non-blocking Linter-Gate) vollständig abgeschlossen.** Phase 2
(Interface-Segregation, Rule 4.2/4.3) folgt koordiniert mit ADR-031/HEX-05.
