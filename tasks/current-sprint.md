> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 37 — Hexagonale Architektur-Migration, Schritt 2: auth-service (JWT-Port)

Ziel: Fortsetzung von ADR-031 (Hexagonale Architektur-Migration) über den `fleet-service`-Piloten
(Sprint 33) hinaus. Nutzer gibt am 2026-07-19 explizit die Fortsetzung frei — der seit Sprint 34
zurückgestellte Entscheidungspunkt ("ob und wann Schritt 2/3 folgen") ist damit aufgelöst. Scope
folgt der in ADR-031 festgelegten Priorisierung: Schritt 2 ist `auth-service` (Storage-Seite über
`UserStore` bereits gelöst, nur der JWT-Port fehlt). `telemetry-service` (Schritt 3) und die
optionale Use-Case-Extraktion (analog `HEX-06` beim Piloten) bleiben bewusst eigene, separat zu
entscheidende Folgeschritte — ein Service pro Sprint, wie beim Piloten, hält den Sprint klein.

**Nutzer-Budget:** ca. 200.000 Token (Standardvorgabe). Erwartet deutlich unterhalb davon: reine
Interface-Extraktion an bereits vollständig getesteter, DB-unabhängiger Logik (`handler_test.go`
läuft schon ohne externe Ressource), kein Docker-Stack nötig — anders als Sprint 36 (DEPLOY-08/
TESTGAP-01) gibt es hier keinen Grund für einen echten Verifikationslauf gegen den Test-Stack,
`go test ./internal/authservice/...` genügt.

Vorrecherche (2026-07-19, vor Sprint-Start durchgeführt):

- **HEXAUTH-01/02**: `internal/authservice/handler.go:305-332` — `issueToken`/`parseToken` sind
  private `Handler`-Methoden, die direkt gegen `golang-jwt/jwt/v5` arbeiten
  (`jwt.NewWithClaims`/`jwt.ParseWithClaims`), `Handler.secret []byte` als Feld (Zeile 33). Anders
  als beim `FleetStore`-Piloten bringt der Port hier **keine** neue DB-Unabhängigkeit — der Nutzen
  ist Dependency Inversion: `Handler` importiert `golang-jwt/jwt/v5` nach der Migration nicht mehr
  direkt, sondern nur noch über den `TokenIssuer`-Port. `cmd/auth-service/main.go:35` ruft
  `authservice.NewHandler(secret, userStore)` auf (`secret` als `string`) — Konstruktorsignatur
  bleibt unverändert, wenn `NewHandler` intern einen `JWTTokenIssuer` aus dem `secret`-String
  baut (identisches Muster zu HEX-02: `cmd/fleet-service/main.go:59` musste bei der
  `FleetStore`-Umstellung ebenfalls nicht angepasst werden, da `*PostgresFleetStore` das neue
  Interface strukturell bereits erfüllte — kein Fake/Mock in `main.go` nötig).
- **HEXAUTH-03**: `go test ./internal/authservice/...` läuft bereits jetzt ohne `DATABASE_URL`
  (Stub-`UserStore`) — nach der Migration ändert sich daran nichts Beobachtbares, nur
  `TokenIssuer` tritt zwischen `Handler` und `golang-jwt` (kein neuer Fake nötig, `handler_test.go`
  kann unverändert gegen den echten `JWTTokenIssuer` laufen, da JWT-Signierung reine Berechnung
  ist, keine externe Ressource wie Postgres).
- **Nebenbefund, bewusst außerhalb dieses Sprints:** `golang-jwt/jwt/v5` mit identischem
  Alg-Confusion-Guard (SEC-01) direkt in 6 Paketen importiert (`cmd/control-server/main.go`,
  `cmd/vehicle-mock/main.go`, `internal/authservice/handler.go`, `internal/fleetservice/handler.go`,
  `internal/vehicleconnection/handler.go`, `internal/controlserver/transport/websocket.go`) —
  potenzielles Rule-3.1-Duplikat (gemeinsames `pkg/authtoken` denkbar), aber paketübergreifende
  Extraktion ist nicht Teil des ADR-031-Scopes für diesen Schritt (nur `auth-service`s eigener
  Handler-Code). Nur dokumentiert, nicht in diesem Sprint aufgegriffen.

Datum: 2026-07-19 | **Status: Geplant, noch nicht begonnen**
Vorgänger: Sprint 36 ✅ (committed, Commit `b487f15`)
Branch/Worktree: noch nicht festgelegt.

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| HEXAUTH-01 | `TokenIssuer`-Port + `JWTTokenIssuer`-Adapter definieren (`internal/authservice/tokenissuer.go`), Compile-Time-Check. Kein `Handler`-Wechsel in diesem Schritt. | S | 🔲 |
| HEXAUTH-02 | `Handler.secret` → `Handler.tokens TokenIssuer` umstellen, alle Aufrufer auf `h.tokens.IssueToken`/`h.tokens.ParseToken` migrieren, `NewHandler`-Signatur bleibt unverändert. | S | 🔲 |
| HEXAUTH-03 | Verifikation (`go build`/`go vet`/`go test ./internal/authservice/...`) + ADR-031-/`DECISIONS.MD`-/Backlog-Status-Update (Schritt 2 abgeschlossen). | S | 🔲 |

**Nicht Teil dieses Sprints (eigene Entscheidungspunkte danach):** HEXAUTH-04 (optionale
Use-Case-Extraktion, analog `HEX-06`), Fortsetzung auf `telemetry-service` (ADR-031 Schritt 3).
