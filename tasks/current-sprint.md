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

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| DRIFT61-01 | `DRIFT-M19`: `tests/performance/latency.js` von `GET /sessions`-Proxy auf echten WS-ACK-Roundtrip umstellen (`k6/ws`, `session/start` gegen `vehicle-int-mock`, minimale Protobuf-`DEADMAN_HOLD`-Nachricht wie im Go-Benchmark). Schwellwerte (`p(99)<100`) unverändert. | M | 🔲 | — |
| DRIFT61-02 | `DRIFT-M20`: Entscheidung + Umsetzung — Video-Latenzziel (100–300ms) entweder um einen `getStats()`-basierten automatisierten Test ergänzen, oder `docs/requirements.md`/`CONTEXT.MD` explizit als "unverifiziert/aspirational" kennzeichnen (ADR-014: kein Safety-Hartziel, daher reine Ermessensfrage ohne Grill-Me). | S/M | 🔲 | — |
| DRIFT61-03 | `DRIFT-M21`: `docs/deployment/ec2-bootstrap.md` korrigieren — Zeile 107-109 (Security-Group-Hinweis) und Zeile 371 (Service-Tabelle) korrigieren: `fleet-service` ist seit Sprint 36 (`DEPLOY-08`) in `docker-compose.prod.yml` + nginx-Proxy verdrahtet, braucht keinen eigenen Port (analog `auth-service`). Kein Compose-/CDK-Code betroffen, reiner Doku-Fix. | S | 🔲 | — |
| DRIFT61-04 | `DRIFT-M22`: `-race` strukturell in mindestens einem Makefile-Standardziel verankern (Entscheidung: eigenes `test-race`-Target vs. `-race` in `test-unit` selbst) statt nur ad-hoc manuell. Bei Umsetzung: `go test ./... -race` einmal vollständig gegen den aktuellen Stand laufen lassen, um neu auffällige Races (nicht nur den Task selbst) sofort sichtbar zu machen. | M | 🔲 | — |
| DRIFT61-05 | `DRIFT-M24`: Package-lokalen `OperatorRole`-Typ in `internal/controlserver/session/manager.go` einführen (analog `statemachine/state.go`, **kein** Import von `authservice` — separate Services). Alle Rohstring-Stellen in `session/manager.go`, `command/engine.go:108`, `transport/websocket.go:158,172` umstellen. Sicherheitsnah (ADR-025) — volle Testabdeckung inkl. `-race -count=2`. | M | 🔲 | — |
| DRIFT61-06 | `DRIFT-M25`: Package-lokalen `TaskStatus`-Typ in `internal/fleetservice/store.go` einführen (analog `statemachine/state.go`), `Task.Status`, `taskTransitionSources` und alle Vergleichsstellen umstellen. Keine Verhaltensänderung an der Übergangsmatrix. | M | 🔲 | DRIFT61-04 (falls `-race`-Target vorher steht, direkt mitverifizieren) |
| DRIFT61-07 | Verifikation: `go build ./...`, `go vet ./...`, `gofmt -l` sauber; `make test-latency`/`make test-k6` weiterhin grün (inkl. neuem WS-Roundtrip aus `DRIFT61-01`); `go test ./... -race -count=2` grün (§17); Doku-Updates (`DECISIONS.MD`, `tasks/backlog.md` Drift-Audit-Fixes-Tabelle — 6 Zeilen, ggf. `docs/requirements.md`/`CONTEXT.MD` für `DRIFT-M20`). | S | 🔲 | DRIFT61-01..06 |

**Nicht Teil dieses Sprints:** `DRIFT-AP3` (neues EPIC „AP3" anlegen, strukturell, kein
Drift-Fix), `CIHARD-04` (`buf breaking`, weiterhin offener Diskussionspunkt ohne neuen Anlass) —
beide bereits in Sprint 60 als bewusst ausgeklammert dokumentiert und weiterhin ohne neuen Anlass.
Mit Abschluss dieses Sprints sind alle in der Sprint-59-Nachbesprechung (2026-07-22) benannten
`DRIFT-K*`/`DRIFT-M18..M25`-Restposten abgearbeitet.

## Ergebnis

_Noch offen — Sprint ist geplant, aber nicht umgesetzt._
