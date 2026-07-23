> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 61 — Drift-Audit-Restposten Teil 2 (Umsetzung)

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "Drift-Audit-Fixes (Sprint 26,
`docs/drift-audit-2026-07.md`)". Zweiter und letzter der beiden Sprints, die die seit Sprint 26
offenen `DRIFT-M*`-Punkte abschließen (Sprint-59-Nachbesprechung, 2026-07-22 — Aufteilung wegen
Token-Budget). Sprint 60 hat die drei kritischen Grill-Me-Entscheidungen (`DRIFT-K4/K6/M18`)
geliefert; alle sechs hier verbliebenen Punkte (`DRIFT-M19/M20/M21/M22/M24/M25`) sind laut
Backlog-Einstufung "reine Umsetzung, keine offenen Entscheidungen mehr" — kein Grill-Me nötig.

**Vorrecherche (2026-07-23, gegen echten Code verifiziert statt nur die Backlog-/Drift-Audit-
Einschätzung zu übernehmen):**
- **DRIFT-M19** — `tests/performance/latency.js` misst laut eigenem Kommentar bewusst `GET
  /sessions` als HTTP-Proxy für die ACK-Latenz, weil "WebSocket binary framing in k6 requires
  additional setup". k6s stabiles `k6/ws`-Modul unterstützt binäre Frames aber bereits seit
  Jahren nativ (kein Experimental-Feature, keine Docker-Image-Änderung nötig) — die im
  Kommentar genannte Einschränkung ist überholt. Login (`setup()`) und Schwellwerte (`p(99)<100`)
  bleiben unverändert, nur der eigentliche Request wird von `http.get` auf einen echten
  `ws.connect`-Roundtrip mit einer minimalen Protobuf-`DEADMAN_HOLD`-Nachricht umgestellt (analog
  `BenchmarkControlACKRoundtrip` in `tests/performance/latency_test.go`) — braucht vorher
  `session/start` gegen `vehicle-int-mock` (einziges Fahrzeug mit echter WS-Verbindung im
  Test-Stack, siehe Memory zu E2E-Backend-Eigenheiten).
- **DRIFT-M20** — Video-Latenzziel (100–300ms) ist laut ADR-014 explizit **kein**
  Safety-Hartziel (anders als der 100ms-ACK-Wert, ADR-010) und hat aktuell nur einen
  Farbcodierungs-Test der UI-Anzeige, keinen automatisierten Latenz-Messwert. Reine
  Ermessensentscheidung ohne Sicherheitsbezug — kein Grill-Me nötig, aber eine bewusste Wahl:
  echten Test ergänzen (WebRTC-`getStats()`-basiert, aufwendig gegen einen echten
  MediaMTX-Container) vs. Doku als "unverifiziert/aspirational" kennzeichnen.
- **DRIFT-M21** — Der im Drift-Audit beschriebene Zustand ("`docker-compose.prod.yml` hat keinen
  `fleet-service`-Eintrag") ist **überholt**: `DEPLOY-08` (Sprint 36) hat den Service-Block bereits
  ergänzt (`infrastructure/compose/docker-compose.prod.yml:126-127`), `infrastructure/docker/
  nginx.conf:107-116` proxied `/fleet/` bereits auf `fleet-service:8085`, und `GO_SERVICES` in
  `docs/deployment/ec2-bootstrap.md:262` baut `fleet-service` bereits mit. Fleet-service braucht
  auch **keinen** eigenen Security-Group-Port (wie `auth-service:8081` läuft es rein intern hinter
  nginx — `infrastructure/AWS/cdk_server-stack.ts` öffnet konsequenterweise auch für
  `auth-service` keinen Port). Der einzige reale Rest-Gap: `ec2-bootstrap.md` selbst behauptet an
  zwei Stellen (Zeile 107-109, Zeile 371) noch fälschlich, die Lücke bestehe weiterhin — reiner
  Doku-Korrektur-Task (S statt M/L, kein Compose-/CDK-/nginx-Code betroffen).
- **DRIFT-M22** — Kein `Makefile`-Target ruft `-race` auf (nur ad-hoc in `tasks/sprints/*.md`
  dokumentierte manuelle Läufe). Sicherheitsnah nur indirekt (deckt Data Races in
  sicherheitsrelevantem Code auf, ist selbst aber keine Sicherheitslücke) — Entscheidungspunkt:
  welche(s) Target(s) bekommen `-race` (`test-unit` betrifft `go test $(go list ./... | grep -v
  /tests/integration)`, `test-integration` den Docker-Stack) und ob `-race` das Default-`test`
  ersetzt oder als zusätzliches `test-race`-Target danebensteht (Laufzeit-Overhead beachten).
- **DRIFT-M24** — `authservice.OperatorRole` (`internal/authservice/handler.go:17-25`, benannte
  Konstanten `RoleAdmin/RoleActiveOperator/RoleObserver/RoleStandby/RoleVehicle`) existiert
  bereits, wird aber **nicht** wiederverwendet: `control-server` importiert `authservice` an
  keiner Stelle (separate deploybare Services, nur über JWT-Claims gekoppelt — ein Cross-Service-
  Import wäre eine neue, unerwünschte Kopplung). Der Fix ist deshalb kein Type-Reuse, sondern ein
  eigener, package-lokaler Typ in `internal/controlserver/session/manager.go` (analog zum
  bestehenden Muster in `statemachine/state.go`), der die drei betroffenen Rohstring-Stellen
  (`session/manager.go:18,57,60,71,75,98,111,138,154,156,176,207,274,278`,
  `command/engine.go:108`, `transport/websocket.go:158,172`) ersetzt. Sicherheitsnah (ADR-025
  Rollenprüfung) — volle §17-Testabdeckung (`-race -count=2`) bei Umsetzung Pflicht.
- **DRIFT-M25** — `internal/fleetservice/store.go`s `Task.Status` (Zeile 210) sowie
  `taskTransitionSources`/`allowedTargetsFrom` (Zeile 572ff.) arbeiten durchgängig mit
  Rohstrings (`"pending"|"in_progress"|"completed"|"cancelled"`). Analog `DRIFT-M24`: neuer
  package-lokaler `TaskStatus`-Typ (Vorbild `statemachine/state.go`), keine Verhaltensänderung an
  der Übergangslogik selbst.

**Gemini-mcp-Einsatz (2026-07-23, Konnektivität live neu verifiziert):** Sprint 60 konnte den
`gemini-mcp`-Server wegen eines serverseitigen Modell-Fallback-Bugs (404 auf ein nicht mehr
existierendes Modell) nicht nutzen (s. `tasks/sprints/60-drift-audit-restposten-1.md`). Vor
diesem Sprint erneut geprüft, nicht nur angenommen: `gemini_subagent_list` antwortet, und ein
echter Modell-Call (`gemini_summarize`) liefert ein korrektes Ergebnis — der Server läuft aktuell
sauber. Dieser Sprint plant Gemini deshalb gezielt dort ein, wo ein Werkzeug wirklich passt (nicht
pauschal für jeden Task):
- **Zweitmeinung (`gemini_brainstorm`)** für die beiden echten Ermessensentscheidungen
  `DRIFT-M20`/`DRIFT-M22` — Entscheidung selbst bleibt bei Claude/Nutzer, Gemini liefert nur
  Pro/Contra.
- **Entwurf (`gemini_simple_code_generator`, Gemini Flash)** für den k6-`ws.connect`-Rewrite in
  `DRIFT-M19` — mechanische, gut abgegrenzte Übersetzung eines bestehenden Musters
  (`BenchmarkControlACKRoundtrip`) in eine andere Sprache; Claude integriert den Entwurf und
  verifiziert ihn gegen den echten Docker-Teststack (`make test-k6`), übernimmt ihn nicht
  ungeprüft.
- **Zweitmeinung-Review (`gemini_code_review_refactor`, Gemini Pro)** für `DRIFT-M24`/`M25` — beide
  sind sicherheitsnahe bzw. strukturelle Typ-Refactorings; Claude implementiert (ADR-025-Rollen-
  prüfung bleibt bei Claude, `CLAUDE.MD` §0), Gemini liefert danach eine unabhängige zweite
  Code-Review-Meinung auf den fertigen Diff, bevor der PR aufgemacht wird.
- **Abschließendes Review (`gemini_code_review_refactor`)** in `DRIFT61-07` über den gesamten
  Sprint-Diff als zusätzliches Gate vor dem PR — ergänzt, ersetzt nicht die eigene Verifikation.
- **`DRIFT-M21`** (reiner Doku-Zeilenfix) bekommt bewusst **keinen** Gemini-Einsatz — kein
  Werkzeug aus dem Katalog passt für eine triviale Zwei-Zeilen-Korrektur, Einsatz wäre Ritual statt
  Mehrwert.
- `gemini_unit_test_generator`/`gemini_type_converter_formatter` passen für keinen der sechs Tasks
  (Vitest/Jest/PyTest bzw. JSON/SQL/YAML→TS/Pydantic — dieser Sprint ist reines Go/k6) und werden
  deshalb nicht eingeplant.

## Tasks

| ID | Task | Typ | Gemini-mcp-Rolle | Status | Abhängigkeiten |
|----|------|-----|-------------------|--------|-----------------|
| DRIFT61-01 | `DRIFT-M19`: `tests/performance/latency.js` von `GET /sessions`-Proxy auf echten WS-ACK-Roundtrip umstellen (`k6/ws`, `session/start` gegen `vehicle-int-mock`, minimale Protobuf-`DEADMAN_HOLD`-Nachricht wie im Go-Benchmark). Schwellwerte (`p(99)<100`) unverändert. | M | Entwurf via `gemini_simple_code_generator`, Claude integriert + verifiziert (`make test-k6`) | ✅ | — |
| DRIFT61-02 | `DRIFT-M20`: Entscheidung + Umsetzung — Video-Latenzziel (100–300ms) entweder um einen `getStats()`-basierten automatisierten Test ergänzen, oder `docs/requirements.md`/`CONTEXT.MD` explizit als "unverifiziert/aspirational" kennzeichnen (ADR-014: kein Safety-Hartziel, daher reine Ermessensfrage ohne Grill-Me). | S/M | Zweitmeinung via `gemini_brainstorm` vor der Entscheidung | ✅ | — |
| DRIFT61-03 | `DRIFT-M21`: `docs/deployment/ec2-bootstrap.md` korrigieren — Zeile 107-109 (Security-Group-Hinweis) und Zeile 371 (Service-Tabelle) korrigieren: `fleet-service` ist seit Sprint 36 (`DEPLOY-08`) in `docker-compose.prod.yml` + nginx-Proxy verdrahtet, braucht keinen eigenen Port (analog `auth-service`). Kein Compose-/CDK-Code betroffen, reiner Doku-Fix. | S | keiner (trivialer Doku-Fix, kein Werkzeug passt) | ✅ | — |
| DRIFT61-04 | `DRIFT-M22`: `-race` strukturell in mindestens einem Makefile-Standardziel verankern (Entscheidung: eigenes `test-race`-Target vs. `-race` in `test-unit` selbst) statt nur ad-hoc manuell. Bei Umsetzung: `go test ./... -race` einmal vollständig gegen den aktuellen Stand laufen lassen, um neu auffällige Races (nicht nur den Task selbst) sofort sichtbar zu machen. | M | Zweitmeinung via `gemini_brainstorm` vor der Entscheidung | ✅ | — |
| DRIFT61-05 | `DRIFT-M24`: Package-lokalen `OperatorRole`-Typ in `internal/controlserver/session/manager.go` einführen (analog `statemachine/state.go`, **kein** Import von `authservice` — separate Services). Alle Rohstring-Stellen in `session/manager.go`, `command/engine.go:108`, `transport/websocket.go:158,172` umstellen. Sicherheitsnah (ADR-025) — volle Testabdeckung inkl. `-race -count=2`. | M | Claude implementiert; `gemini_code_review_refactor` als unabhängige Zweitmeinung auf den fertigen Diff vor PR | ✅ | — |
| DRIFT61-06 | `DRIFT-M25`: Package-lokalen `TaskStatus`-Typ in `internal/fleetservice/store.go` einführen (analog `statemachine/state.go`), `Task.Status`, `taskTransitionSources` und alle Vergleichsstellen umstellen. Keine Verhaltensänderung an der Übergangsmatrix. | M | Claude implementiert; `gemini_code_review_refactor` als unabhängige Zweitmeinung auf den fertigen Diff vor PR | ✅ | DRIFT61-04 (falls `-race`-Target vorher steht, direkt mitverifizieren) |
| DRIFT61-07 | Verifikation: `go build ./...`, `go vet ./...`, `gofmt -l` sauber; `make test-latency`/`make test-k6` weiterhin grün (inkl. neuem WS-Roundtrip aus `DRIFT61-01`); `go test ./... -race -count=2` grün (§17); Doku-Updates (`DECISIONS.MD`, `tasks/backlog.md` Drift-Audit-Fixes-Tabelle — 6 Zeilen, ggf. `docs/requirements.md`/`CONTEXT.MD` für `DRIFT-M20`). | S | Abschließendes `gemini_code_review_refactor` über den gesamten Sprint-Diff als zusätzliches Pre-PR-Gate | ✅ | DRIFT61-01..06 |

**Grundsatz (gilt für alle Gemini-Einsätze in diesem Sprint):** Gemini liefert Entwürfe/Zweit-
meinungen, niemals die finale Entscheidung oder unüberprüfte sicherheitsrelevante Umsetzung —
Claude verifiziert jeden Gemini-Output gegen echten Code/Teststack, bevor er übernommen wird
(`CLAUDE.MD` §0: Sicherheit schlägt alles). Bei erneutem Serverausfall (s. Sprint-60-Präzedenzfall)
wird ohne Gemini weitergearbeitet und das transparent im Ergebnis-Abschnitt dokumentiert, statt den
Sprint zu blockieren.

**Nicht Teil dieses Sprints:** `DRIFT-AP3` (neues EPIC „AP3" anlegen, strukturell, kein
Drift-Fix), `CIHARD-04` (`buf breaking`, weiterhin offener Diskussionspunkt ohne neuen Anlass) —
beide bereits in Sprint 60 als bewusst ausgeklammert dokumentiert und weiterhin ohne neuen Anlass.
Mit Abschluss dieses Sprints sind alle in der Sprint-59-Nachbesprechung (2026-07-22) benannten
`DRIFT-K*`/`DRIFT-M18..M25`-Restposten abgearbeitet.

## Ergebnis

**DRIFT61-01 — `tests/performance/latency.js` auf echten WS-ACK-Roundtrip umgestellt.** Entwurf via
`gemini_simple_code_generator` (k6/ws-API-Form: `sendBinary`, `binaryMessage`-Event), von Claude
integriert und substanziell angepasst: `setup()` ruft jetzt zusätzlich `/session/start` gegen
`vehicle-int-mock` auf, `default()` öffnet einen echten `ws.connect`-Roundtrip mit der binären
`DEADMAN_HOLD`-Nachricht (`[0x10, 0x06]`, wie `BenchmarkControlACKRoundtrip`), `teardown()` ruft
`/session/end`. **`vus` bewusst von 10 auf 1 reduziert** — nur ein `ACTIVE_OPERATOR`-Slot pro
Fahrzeug (ADR-025), konkurrierende VUs hätten nur Lock-Contention statt ACK-Latenz gemessen; die
`recoverFromSafeMode`-Logik in `websocket.go` erlaubt das wiederholte Reconnect-nach-SAFE_MODE-Muster
pro Iteration sauber. Verifiziert: `make test-k6` 2× grün, p99=1ms, 581/581 bzw. 583/583 Iterationen
erfolgreich, `ack_latency_ms` misst nachweislich nur Send→ACK (getrennt von k6s eigener
`ws_connecting`-Metrik, die den Handshake separat ausweist).

**DRIFT61-02 — Video-Latenzziel: Doku statt Test.** `gemini_brainstorm`-Zweitmeinung hing zweimal
500s+ (siehe Gemini-Abschnitt unten) — Entscheidung ohne Gemini getroffen, eigenständig begründet.
Entscheidung: `docs/requirements.md` und `CONTEXT.MD` markieren das 100–300ms-Ziel jetzt explizit als
"unverifiziert/aspirational" statt einen `getStats()`-basierten Test nachzurüsten. Begründung: ein
Mock von `RTCPeerConnection.getStats()` würde nur beweisen, dass der eigene Code den Mock-Wert liest,
nicht dass echte Netzwerklatenz im Zielkorridor liegt; ein aussagekräftiger echter Test bräuchte
einen echten MediaMTX-Container plus Browser-Automatisierung — unverhältnismäßiger Aufwand für ein
laut ADR-014 nicht-sicherheitsrelevantes, nicht CI-Build-Fail-auslösendes QoS-Ziel (anders als die
100ms-Control-Loop-Latenz, die bereits real erzwungen wird). Die Farbcodierungs-/Schwellwertlogik der
UI bleibt unit-getestet (`useWebRTC.test.ts`).

**DRIFT61-03 — `docs/deployment/ec2-bootstrap.md` korrigiert.** 3 Stellen statt der 2
vorrecherchierten (eine dritte, gleichlautende Falschbehauptung in einem Kommentarblock bei den
Build-Instruktionen wurde beim Durcharbeiten zusätzlich gefunden und mitkorrigiert) —
`fleet-service` ist seit Sprint 36 (`DEPLOY-08`) längst in `docker-compose.prod.yml`/nginx
verdrahtet, braucht keinen eigenen Security-Group-Port (analog `auth-service:8081`, bestätigt gegen
`infrastructure/AWS/cdk_server-stack.ts`). Reiner Doku-Fix, kein Gemini-Einsatz (wie geplant).

**DRIFT61-04 — `-race` strukturell verankert.** `gemini_brainstorm`-Zweitmeinung empfahl ein
parametrisiertes `RACE=1`-Makefile-Pattern + CI-Restrukturierung — als für diesen Codebase-Stil zu
komplex verworfen (bestehende Targets sind durchgängig einfache, hartkodierte Ziele ohne
Parametrisierung). Entscheidung: neues eigenständiges `test-race`-Target (`go test ./... -race`,
startet den Docker-Teststack wie `test-integration`), **nicht** in `test-unit` gebacken (würde den
schnellen Dev-Loop verlangsamen), **nicht** mit eingebautem `-count=2` (Laufzeit-Overhead). Neuer
blockierender `race`-Job in `test-go.yml`. Voller `-race`-Lauf gegen den Gesamtcode (vor jeder
Sprint-Änderung) fand **keine** bis dahin unentdeckten Races.

**DRIFT61-05 — `session.OperatorRole`-Typ (sicherheitsnah, ADR-025).** Package-lokaler Typ
(`RoleActiveOperator`/`RoleObserver`), analog `statemachine/state.go`, **kein** Import von
`authservice.OperatorRole` (separate Services, nur JWT-gekoppelt). Alle Rohstring-Vergleiche ersetzt
in `session/manager.go` (inkl. `UpdateOperator`/`CreateSession`-Parametertypen), `handover.go`,
`command/engine.go:108`, `transport/websocket.go:158`, sowie drei Stellen in
`cmd/control-server/main.go` (nicht in der ursprünglichen Vorrecherche-Liste, aber vom selben Feld
betroffen — `handleSessionStart`/`handleSessionEnd`/`handleSessions`, JSON-Serialisierung braucht
dort explizite `string(...)`-Konvertierung, da `map[string]string`/`string`-Felder keine implizite
Konvertierung von benannten Typen erlauben). Keine Verhaltensänderung, `-race -count=2` auf allen
berührten Paketen grün. **Testfalle gefunden und behoben:** mehrere `assert.Equal(t, "ACTIVE_OPERATOR",
sess.OperatorRole)`-Aufrufe in `manager_test.go`/`multioperator_test.go` hätten nach der Typumstellung
`testify`-intern (`reflect.DeepEqual` auf unterschiedliche konkrete Typen) lautlos zur Laufzeit
fehlgeschlagen — 18 Stellen auf den typisierten Konstantenvergleich umgestellt, danach grün.
`gemini_code_review_refactor`-Zweitmeinung auf den fertigen Diff: ein echter, vorbestehender Fund
(`PushSFUEvent` liest `*Session`-Pointer nach `RUnlock()` — potenzielle Data Race mit `UpdateOperator`,
unabhängig vom Typ-Refactor, dokumentiert in `CONTEXT.MD`, bewusst nicht mitgefixt); ein falscher Fund
(behauptete "Handover-Bugfix"-Notwendigkeit — gegen echten Code verifiziert und widerlegt,
`statemachine.OpActive` war bereits exakt `"ACTIVE_OPERATOR"`); ein abgelehnter Vorschlag (Fail-closed
statt Fail-open bei `command/engine.go`s Rollenprüfung — legitime Verteidigungslinie, aber
Verhaltensänderung für einen aktuell unerreichbaren Zustand, widerspricht dem Sprint-Scope "keine
Verhaltensänderung"); IsActive()/IsObserver()-Hilfsmethoden abgelehnt (inkonsistent mit dem
`statemachine/state.go`-Vorbild, das keine solchen Methoden hat).

**DRIFT61-06 — `fleetservice.TaskStatus`-Typ.** Package-lokaler Typ (`TaskPending`/`TaskInProgress`/
`TaskCompleted`/`TaskCancelled`), analog `statemachine/state.go`. `Task.Status`,
`Task.AllowedTransitions`, `taskTransitionSources`, `taskTransitionOrder`, `allowedTaskTransitions`
umgestellt. `UpdateTaskStatus`s Parameter bleibt bewusst `string` (kommt roh aus dem
HTTP-PATCH-Body — externe, unvalidierte Eingabe; die bestehende Map-Lookup-Validierung
(`taskTransitionSources[TaskStatus(newStatus)]`) deckt die Boundary bereits ab, kein `ParseTaskStatus`
nötig). Auch `FakeFleetStore` (Test-Double) und `usecase.go`s `TaskStatusChangedEvent`-Broadcast
(explizite `string(...)`-Konvertierung) angepasst. Keine Verhaltensänderung, `-race -count=2` grün;
echte Postgres-Integrationstests (nicht nur Unit-Tests) bestätigen, dass `pq.Array([]TaskStatus)` und
`Scan(&t.Status, ...)` mit dem neuen Typ korrekt funktionieren (`database/sql`s reflect-basierter
Kind==String-Fallback). `gemini_code_review_refactor`-Zweitmeinung: Vorschläge für
`driver.Valuer`/`sql.Scanner`-Interfaces und ein getrenntes `ParseTaskStatus` abgelehnt (unnötige
Abstraktion, empirisch nicht nötig, inkonsistent mit dem existierenden Codebase-Muster); ein
behaupteter "Bug" (`newStatus == "completed"`-Vergleich) war nur unter Geminis eigenem — nicht
übernommenem — Vorschlag ein Bug, im tatsächlichen Code (Parameter bleibt `string`) korrekt; ein
Fund war reines Prompt-Artefakt (von Gemini fälschlich als "Code-Korruption im Repo" interpretierter
Textrest aus dem eigenen Review-Prompt, gegen die echten Dateien verifiziert und widerlegt).

**DRIFT61-07 — Abschließende Verifikation.** `go build ./...`/`go vet ./...`/`gofmt -l` sauber.
`make test-latency` grün (p50/p95/p99 0-1ms bei 100ms-Budget). `make test-k6` grün (s. DRIFT61-01).
`go test $(go list ./... | grep -v /tests/integration) -race -count=2` grün über alle berührten UND
unberührten Pakete. `tests/integration` läuft weiterhin mit `-race -count=1` (Makefile-Konvention)
grün — **echter, vorbestehender Fund dabei:** ein `go test ./... -race -count=2` (erstmals in diesem
Sprint strukturell versucht, vorher gab es nur Ad-hoc-Einzelläufe) deckte auf, dass mehrere
`tests/integration`-Tests hartkodierte literale Fixture-IDs verwenden (z. B. Zone-ID
`"itg-rest-zone"`) — ein zweiter Durchlauf im selben Prozess kollidiert mit dem ersten
(Unique-Constraint, HTTP 500 statt 201). **Verifiziert als vorbestehend, keine Sprint-61-Regression:**
identisches Verhalten reproduziert auf unverändertem `main` (`git stash` + Test gegen den
Pre-Sprint-Stand). Dokumentiert in `CONTEXT.MD`s Offene-Punkte-Tabelle als eigener Folge-Task, nicht
in diesem Sprint mitgefixt (Scope wäre eine breite Änderung an vielen Testdateien). Doku-Updates:
`DECISIONS.MD` (neue Zeile "Drift-Audit-Restposten `DRIFT-M19/M20/M21/M22/M24/M25`"),
`tasks/backlog.md` (alle 6 Zeilen ✅), `docs/requirements.md`/`CONTEXT.MD` (DRIFT-M20), `CONTEXT.MD`
(2 neue Nebenbefunde: `PushSFUEvent`-Race, `tests/integration`-`-count=2`-Lücke). Abschließendes
`gemini_code_review_refactor` über den gesamten Sprint-Diff: bestätigte den 1-VU-k6-Rewrite als
fachlich korrekt, fand zwei tatsächlich falsche Kritikpunkte (behauptete "falsche Messgröße"/
"Session-Lifecycle-Risiko" — durch die bereits vorliegenden echten `make test-k6`-Läufe (581/583
saubere Iterationen) empirisch widerlegt) sowie zwei brauchbare, risikofreie Härtungsvorschläge
(dangling `setTimeout` nach frühem `close`, fehlender `close`-Handler für unerwartete
Server-Disconnects) — beide übernommen, `make test-k6` danach erneut grün verifiziert. Makefile-/
CI-Vorschläge (`pg_isready` statt `sleep 5`, `trap` für Cleanup) abgelehnt — identisches,
vorbestehendes Muster in allen anderen Docker-basierten Makefile-Targets, Änderung nur für
`test-race` wäre inkonsistent und außerhalb des Scopes von `DRIFT-M22`.

**Gemini-mcp — tatsächlicher Einsatz vs. Plan:** `gemini_subagent_list` + `gemini_summarize`
(Konnektivitätscheck zu Sprint-Beginn) funktionierten. `gemini_simple_code_generator` (DRIFT61-01)
funktionierte, lieferte einen brauchbaren, aber substanziell zu überarbeitenden Entwurf.
`gemini_code_review_refactor` (DRIFT61-05/06/07, 3 Aufrufe) funktionierte jedes Mal, mit gemischter
Trefferquote (siehe oben) — jeder Fund musste einzeln gegen echten Code/echte Testläufe verifiziert
werden, mehrere Befunde waren falsch oder nicht übernehmbar. `gemini_brainstorm` (DRIFT61-02) hing
zweimal 500s+ ohne Antwort, obwohl der Server laut Eingangscheck erreichbar war — per-Tool-Flakiness,
nicht Server-weiter Ausfall (siehe aktualisierte `gemini-mcp`-Referenz-Memory); für DRIFT61-04 war
derselbe Tool-Aufruf dagegen erreichbar. Realistische Bilanz (auf Nutzerfrage mitten im Sprint
gegeben): tokenökonomisch in dieser Session ungefähr ausgeglichen bis leicht negativ — die
Verifikationsarbeit gegen jede Gemini-Aussage hat ungefähr so viel gekostet wie die eigenständige
Analyse gekostet hätte, mit Ausnahme des einen echten `PushSFUEvent`-Race-Fundes, der den Aufwand
wert war.

**Bewusst nicht behoben (neue, in diesem Sprint gefundene Nebenbefunde, dokumentiert statt gefixt):**
- `session.Manager.PushSFUEvent` liest einen `*Session`-Pointer nach `RUnlock()` — potenzielle Data
  Race mit gleichzeitigem `UpdateOperator` auf derselben Session. Vorbestehend, unabhängig vom
  `OperatorRole`-Typ-Refactor. Siehe `CONTEXT.MD`.
- `tests/integration` ist nicht `-count=2`-fest (hartkodierte Fixture-IDs). Vorbestehend, erstmals
  durch diesen Sprints `-race -count=2`-Verifikationsversuch sichtbar geworden. Siehe `CONTEXT.MD`.

Mit Abschluss dieses Sprints sind alle in der Sprint-59-Nachbesprechung (2026-07-22) benannten
`DRIFT-K*`/`DRIFT-M18..M25`-Restposten abgearbeitet.
