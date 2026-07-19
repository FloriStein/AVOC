> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 36 — Restposten-Bereinigung III (Tech Debt, Formatierung, Prod-Lücke, Test-Gap)

Ziel: vier unabhängige, seit mehreren Sprints bekannte, aber nie aufgegriffene Restposten
schließen. Kein EPIC-Fortschritt, reine Aufräumarbeit — bewusst klein gehalten (4× Typ S/M) statt
neue Architektur-Stränge zu öffnen. `HEX-06`, `FLEET-01`, `MV-10` bleiben wie in den
Sprint-32/33/34-Kickoffs entschieden zurückgestellt (kein neuer Anlass).

**Nutzer-Budget:** ca. 200.000 Token (Standardvorgabe). Zwei Tasks (DEPLOY-08, TESTGAP-01)
brauchen echte Verifikation gegen den Docker-Stack — bei der Umsetzung im Auge behalten, ob das
Budget dadurch überschritten wird; wenn ja, TESTGAP-01 mit dokumentiertem Teilergebnis abschließen
statt unbegrenzt nachzuinvestieren (analog WEBRTC-10-Präzedenzfall).

Vorrecherche (2026-07-19, vor Sprint-Start durchgeführt):

- **TECHDEBT-01** (`internal/authservice/noop_userstore.go`): `grep -rn NoopUserStore --include=*.go`
  bestätigt — außer der eigenen Definitionsdatei keine Referenz, auch nicht in Tests
  (`handler_test.go` nutzt `stubUserStore`). Ganze Datei kann gelöscht werden, keine
  Interface-Anpassung nötig (war nie im `UserStore`-Interface referenziert, nur eine unabhängige
  Implementierung davon).
- **GOSTYLE-FMT-01** (neu, Bonus-Folge-Task aus Sprint 28/29, nie aufgegriffen): `gofmt -l .`
  listet aktuell 10 Dateien: `cmd/auth-service/main.go`, `cmd/vehicle-mock/main.go`,
  `internal/authservice/noop_userstore.go` (entfällt durch TECHDEBT-01 — Reihenfolge beachten:
  erst TECHDEBT-01, dann `gofmt -l` erneut prüfen), `internal/authservice/userstore.go`,
  `internal/controlserver/session/manager.go`, `internal/safetyservice/bus.go`,
  `internal/webrtcsfu/sfu.go`, `pkg/db/postgres.go`, `pkg/logger/event_types.go`,
  `tests/unit/multioperator_test.go`. Reines `gofmt -w .`, keine Logikänderung (CLAUDE.MD
  Abschnitt 15 — Verhalten bleibt unverändert, nur Formatierung).
- **DEPLOY-08** (neu, bekannte Produktionslücke, bisher nicht als Task erfasst): `fleet-service`
  ist in `Makefile`s `GO_SERVICES`-Liste (Zeile 21) und wird gebaut, fehlt aber in
  `infrastructure/compose/docker-compose.prod.yml` (17 Services dort, `fleet-service` nicht
  darunter). Bestehendes Muster für DB-/MQTT-gebundene Services (z. B. `auth-service`,
  Zeile 79–93; `telemetry-service`, Zeile 106–118): `image: avoc-<service>:${VERSION}`, Port,
  `environment:`-Block, `depends_on:` mit `condition: service_healthy` für Postgres, `networks:
  - avoc-net`, `restart: unless-stopped`. Env-Variablen-Namen aus der Dev-Compose
  (`infrastructure/compose/docker-compose.yml:152-171`) übernehmbar: `FLEET_PORT`,
  `DATABASE_URL` (Postgres, analog `auth-service`), `JWT_SECRET`, `MQTT_BROKER`. Port `8085`.
- **TESTGAP-01** (neu, Sprint-29-Folge-Task, nie aufgegriffen): die 3 skippenden Tests in
  `tests/integration/services_test.go` (`TestIntegration_SessionLifecycle_StartAndEnd` Zeile 184,
  `TestIntegration_MediaFailed_TriggersDegrade_NeverSafeMode` Zeile 241,
  `TestIntegration_EmergencyStop_TriggersSafeMode` Zeile 276) dialen WS **vor** `POST
  /session/start` und ohne `session_id`-Query-Param. `internal/controlserver/transport/
  websocket.go:93-97` (`authenticateWS`) verlangt `session_id` zwingend (400 ohne). Root Cause
  bestätigt: der Kommentar "Authenticate by connecting WebSocket (transition IDLE →
  AUTHENTICATED)" in den Tests ist veraltet — `handleSessionStart`
  (`cmd/control-server/main.go:318-359`) führt die komplette State-Machine-Transition
  (`advanceVehicleToActiveOperator`, Zeile 364ff., IDLE→CONNECTING→AUTHENTICATED→CONNECTED)
  bereits selbst aus und braucht dafür **keine** vorherige WS-Verbindung, nur einen gültigen JWT
  + `vehicleRegistry.Connected(vehicleID)`. Fix: Reihenfolge in allen 3 Tests umdrehen — erst
  `/session/start` aufrufen und `session_id` aus der Response lesen, danach `ws://.../ws?
  token=...&session_id=...` dialen. Kein Produktionscode-Fix nötig, nur Test-Setup.

Datum: 2026-07-19 | **Status: ✅ abgeschlossen**
Vorgänger: Sprint 35 ✅ (committed, Commit `034fc79`)
Branch/Worktree: `feature/fleet-service-foundation-sprint35`, bestehender Worktree
`controlcenter-aws-sprint35` (kein neuer Worktree/Branch angelegt).

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| TECHDEBT-01 | `internal/authservice/noop_userstore.go` löschen (komplett unreferenziert) | S | ✅ |
| GOSTYLE-FMT-01 | `gofmt -w .` für die 10 aktuell unformatierten Dateien, keine Logikänderung | S | ✅ |
| DEPLOY-08 | `fleet-service` in `infrastructure/compose/docker-compose.prod.yml` ergänzen (analog `auth-service`/`telemetry-service`-Muster) | M | ✅ |
| TESTGAP-01 | 3 skippende WS-Integrationstests fixen: `/session/start` vor WS-Dial aufrufen, `session_id` im WS-URL mitgeben | S/M | ✅ |

## Ergebnisse

- **TECHDEBT-01**: `grep -rn NoopUserStore --include="*.go" .` bestätigte erneut keine Referenz
  außerhalb der eigenen Datei — `internal/authservice/noop_userstore.go` komplett gelöscht, keine
  Interface-Anpassung nötig. `go build ./...` weiterhin sauber.
- **GOSTYLE-FMT-01**: `gofmt -l .` nach der TECHDEBT-01-Löschung neu geprüft — nur noch 9 Dateien
  (die 10. war die gelöschte `noop_userstore.go`). `gofmt -w .` angewendet; `gofmt -l .` läuft
  danach leer. Alle Diffs rein Alignment-/Whitespace-Änderungen (stichprobenartig geprüft, z. B.
  `pkg/logger/event_types.go`: nur Spaltenausrichtung der `const`-Werte), keine Logikänderung.
  `go build ./...`/`go vet ./...` sauber.
- **DEPLOY-08**: `fleet-service`-Service-Block in `infrastructure/compose/docker-compose.prod.yml`
  ergänzt, direkt nach `telemetry-service` — Muster 1:1 aus `auth-service` (DB-Env analog) und
  `telemetry-service` (MQTT-Env/`mosquitto`-Dependency) übernommen, Env-Variablennamen aus der
  Dev-Compose (`FLEET_PORT`, `DATABASE_URL`, `JWT_SECRET`, `MQTT_BROKER`) 1:1 gespiegelt: Image
  `avoc-fleet-service:${VERSION}` (passt zum Makefile-`GO_SERVICES`-Namensmuster), Port `8085`,
  `depends_on` Postgres (`condition: service_healthy`) + Mosquitto (`condition: service_started`),
  `networks: [avoc-net]`, `restart: unless-stopped`. Mit `docker compose -f
  docker-compose.prod.yml config --services` (Dummy-Env-Werte) validiert — `fleet-service`
  erscheint korrekt unter den 15 Services, keine YAML-Syntaxfehler.
- **TESTGAP-01**: Root-Cause aus der Vorrecherche bestätigt, aber beim Umsetzen ein zweiter,
  bisher nicht dokumentierter Blocker gefunden: `handleSessionStart`
  (`cmd/control-server/main.go:339`) lehnt mit 409 ab, wenn `vehicleRegistry.Connected(vehicleID)`
  `false` ist — die drei Test-Vehicle-IDs (`vehicle-int-1`, `vehicle-media`, `vehicle-estop`) waren
  nie über eine echte `/vehicle/ws`-Verbindung registriert (der Test-Stack-`vehicle-mock` verbindet
  nur `vehicle-int-mock`). Reine Reihenfolge-Umkehr allein hätte daher weiterhin mit 409
  fehlgeschlagen. Fix kombiniert beides: erst `connectVehicle(t, vehicleID)` (bereits vorhandener
  Helper aus `multivehicle_test.go`, dialt `/vehicle/ws` mit einem frischen Vehicle-JWT) vor
  `POST /session/start`, danach `session_id` aus der Response lesen und den Operator-WS-Dial
  (`/ws?token=...&session_id=...`) ans Ende verschieben. Die bisherigen `t.Skipf(...)`-Fallbacks
  und der weiche `if resp2.StatusCode == 200`-Guard in `TestIntegration_SessionLifecycle_StartAndEnd`
  wurden durch harte `require.Equal(t, 200, ...)`-Assertions ersetzt — die Tests validieren jetzt
  tatsächlich den vollen Session-Start- und WS-Pfad statt ihn bei Fehlschlag stillschweigend zu
  überspringen. Gegen den echten Docker-Test-Stack verifiziert (`make test-integration`, frischer
  Build): alle 27 Integrationstests grün, keine Skips mehr, insbesondere die drei zuvor
  skippenden (`TestIntegration_SessionLifecycle_StartAndEnd`,
  `TestIntegration_MediaFailed_TriggersDegrade_NeverSafeMode`,
  `TestIntegration_EmergencyStop_TriggersSafeMode`). Kein Produktionscode geändert, nur
  Test-Setup — deckt sich mit der Vorrecherche.

**Verifikation gesamt**: `go build ./...`, `go vet ./...` sauber. `go test $(go list ./... | grep
-v /tests/integration)` — alle Unit-/Package-Tests grün. `make test-integration` (frischer
Docker-Build aller 5 Go-Services + Postgres/Mosquitto/vehicle-mock): 27/27 Tests grün, 0 Skips.
`docker compose -f infrastructure/compose/docker-compose.prod.yml config` zur Syntax-/
Struktur-Validierung von DEPLOY-08 (kein echtes EC2-Deployment, da keine Deploy-Absicht in diesem
Sprint). Budget nicht überschritten — TESTGAP-01 brauchte trotz des zusätzlich gefundenen
`vehicleRegistry.Connected`-Blockers keinen zweiten Anlauf, da der Fix direkt beim ersten
Docker-Stack-Lauf verifiziert wurde.
