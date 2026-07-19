> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 38 — MQTT-Authentifizierung (Mosquitto Passwort-File)

Ziel: den seit Projektbeginn offenen Sicherheits-Gap aus `tasks/backlog.md` ("Offene
Entscheidungen") schließen — Mosquitto (Port 1883) läuft aktuell mit `allow_anonymous true`, jeder
Prozess im `avoc-net`-Docker-Netzwerk kann ohne Credentials publizieren/subscriben. Nutzer wählt
diesen Fokus am 2026-07-19 explizit gegenüber der Alternative "Hexagonal-Migration Schritt 3
(telemetry-service)", mit Verweis auf CLAUDE.MD §0 Priorität 1 ("Sicherheit schlägt alles").

**Bewusster Nicht-Scope:** Transportverschlüsselung (TLS/MQTTS) ist **nicht** Teil dieses Sprints —
der Backlog-Eintrag ist explizit auf "Mosquitto Passwort-File" (Authentifizierung) begrenzt, nicht
auf Verschlüsselung. Passwörter reisen damit weiterhin im Klartext über das interne
`avoc-net`-Bridge-Netzwerk — identisches Vertrauensmodell wie `DATABASE_URL`
(Postgres-Passwort ebenfalls Klartext über dasselbe Netzwerk). Kein neues ADR nötig: ADR-003
("MQTT Broker Wahl") legt Mosquitto bereits fest, dieser Sprint aktiviert nur dessen eingebauten
Auth-Mechanismus (`password_file`) — keine neue Architekturentscheidung, kein Bruch mit ADR-003.
Analog zu SEC-01 (Sprint 34, JWT-Alg-Confusion-Fix), das ebenfalls ohne neues ADR auskam.

**Nutzer-Budget:** ca. 200.000 Token (Standardvorgabe). Sechs klein geschnittene Tasks (S/M),
keine neue Domänenlogik — reine Auth-Aktivierung an einer bestehenden Infrastrukturkomponente.

Vorrecherche (2026-07-19, vor Sprint-Start durchgeführt):

- **Aktueller Zustand:** `infrastructure/mosquitto/mosquitto.conf` (von Dev **und** Prod-Compose
  gemountet, siehe `infrastructure/compose/docker-compose.yml:131` bzw.
  `docker-compose.prod.yml:145`) sowie `tests/mosquitto-test.conf` (Docker-Test-Stack,
  `tests/docker-compose.test.yml:95`) haben identisch nur 4 Zeilen: `listener 1883`,
  `allow_anonymous true`, `persistence false`, `log_dest stdout`. Kein `password_file`.
- **Drei Go-MQTT-Verbindungsstellen in Produktionscode**, alle über `github.com/eclipse/
  paho.mqtt.golang`, keine setzt `SetUsername`/`SetPassword`:
  1. `internal/telemetryservice/client.go:41-52` (`Client.Connect`) — `telemetry-service`,
     abonniert `vehicle/+/telemetry`.
  2. `internal/fleetgateway/mqtt.go:39-44` (`NewMQTTGateway`) — `fleet-service`, abonniert
     `StatusTopicWildcard`/`AlertTopicWildcard`.
  3. `cmd/vehicle-mock/main.go:259-269` (`connectMQTT`) — `vehicle-mock`/`vehicle-mock-2`,
     publiziert Telemetrie **und** (via `fleet_simulator.go`, gleicher `mqtt.Client`) Fleet-Status/
     -Alerts. Ein einziger Verbindungspunkt für beide Zwecke.
  Alle drei lesen den Broker bereits aus einer Env-Var (`MQTT_BROKER`, Default `mosquitto:1883`)
  — Muster für `MQTT_USERNAME`/`MQTT_PASSWORD` liegt also schon vor.
- **Drei Compose-Dateien** reichen `MQTT_BROKER: "mosquitto:1883"` an total 4 Services durch
  (`telemetry-service`, `fleet-service`, `vehicle-mock`, `vehicle-mock-2` — dev:
  `docker-compose.yml:119,164,260,289`; prod: `docker-compose.prod.yml` analog; Test-Stack:
  `docker-compose.test.yml:111,133`, dort nur `fleet-service`+`vehicle-mock`, kein
  `telemetry-service`/`vehicle-mock-2`).
- **SSM/Deploy-Präzedenzfall:** `TURN_USER`/`TURN_PASSWORD` ist bereits ein Username+Passwort-Paar
  für ein Nicht-HTTP-Protokoll, exakt analog zum Zielzustand hier. `scripts/setup-ssm.sh:36-76`
  legt es per `put-parameter` an (Klartext-User via `put`, Passwort via `put-parameter
  --type SecureString`), `scripts/deploy.sh:66-71` liest beides per `get`/`get_secure` zur
  Deploy-Zeit. `.env.example` zeigt das Dev-Äquivalent (`TURN_USER=avoc`, `TURN_PASSWORD=changeme`)
  — `JWT_SECRET`/`DB_PASSWORD` folgen demselben Muster (`${VAR}`-Interpolation in
  `docker-compose.yml`, Wert aus `.env`).
- **Mosquitto-Mechanismus:** `eclipse-mosquitto:2`-Image enthält das `mosquitto_passwd`-CLI-Tool
  zum Erzeugen einer gehashten Passwort-Datei (`mosquitto_passwd -b -c <file> <user> <pass>`).
  `mosquitto.conf` referenziert sie über `password_file <pfad>` + `allow_anonymous false`. Die
  Datei muss vor Broker-Start existieren — für Dev/Test kann ein fester, dokumentierter
  Dev-Credential-Satz analog zu `TURN_PASSWORD=changeme` verwendet und die gehashte Datei committed
  werden (Hash ist nicht umkehrbar, unkritisch für Dev/Test-Secrets); für Prod muss die Datei
  **zur Deploy-Zeit** aus dem SSM-Passwort generiert werden (nicht committen).
- **Testabdeckung, die von der Umstellung betroffen ist:** `internal/fleetgateway/mqtt_test.go`
  (Unit, verbindet direkt gegen einen lokalen Test-Broker), `tests/integration/
  fleet_simulation_test.go:23` + `tests/integration/fleet_service_test.go:324,377` (Integration,
  verbinden direkt gegen `tcp://localhost:11883` bzw. den Test-Compose-Broker) — alle drei müssen
  nach der Umstellung ebenfalls Credentials setzen, sonst schlägt die Verbindung fehl.

Datum: 2026-07-19 | **Status: Geplant, noch nicht begonnen**
Vorgänger: Sprint 37 ✅ (auth-service TokenIssuer-Port, siehe `tasks/sprints/
37-auth-service-hexagonal-jwt-port.md`)
Branch/Worktree: `feature/fleet-service-foundation-sprint35` (bestehender Worktree, kein neuer
Branch).

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MQTTAUTH-01 | Mosquitto-Configs umstellen: `infrastructure/mosquitto/mosquitto.conf` (Dev+Prod) und `tests/mosquitto-test.conf` auf `allow_anonymous false` + `password_file /mosquitto/config/passwd`. Gehashte Dev-/Test-Passwd-Dateien erzeugen (feste, dokumentierte Credentials, analog `TURN_PASSWORD=changeme`) und committen. | S | 🔲 |
| MQTTAUTH-02 | Go-Client-Anpassung: `SetUsername`/`SetPassword` an den 3 Verbindungsstellen (`telemetryservice.Client.Connect`, `fleetgateway.NewMQTTGateway`, `cmd/vehicle-mock/main.go connectMQTT`) ergänzen. Neue `MQTT_USERNAME`/`MQTT_PASSWORD`-Env-Vars in den jeweiligen `main.go`/Konstruktoren, analog zum bestehenden `MQTT_BROKER`-Muster. | M | 🔲 |
| MQTTAUTH-03 | `MQTT_USERNAME`/`MQTT_PASSWORD` an alle 4 MQTT-Consumer-Services in `docker-compose.yml` (Dev), `docker-compose.prod.yml` und `docker-compose.test.yml` durchreichen; `.env.example` um die Dev-Defaults ergänzen. | S | 🔲 |
| MQTTAUTH-04 | `scripts/setup-ssm.sh`: `MQTT_USERNAME`(`put`)/`MQTT_PASSWORD`(`put-parameter --type SecureString`) ergänzen, analog `TURN_USER`/`TURN_PASSWORD`. `scripts/deploy.sh`: Parameter laden + Passwd-Datei auf dem EC2-Host zur Deploy-Zeit generieren (`docker run --rm ... mosquitto_passwd -b -c ...`), bevor `docker compose up` den Broker startet. | M | 🔲 |
| MQTTAUTH-05 | Bestehende Tests auf neue Credentials umstellen: `internal/fleetgateway/mqtt_test.go` (Unit), `tests/integration/fleet_simulation_test.go`, `tests/integration/fleet_service_test.go` (Integration) — `SetUsername`/`SetPassword` an den dortigen direkten `mqtt.NewClient`-Aufrufen ergänzen. | S | 🔲 |
| MQTTAUTH-06 | Verifikation gegen echten Docker-Test-Stack (`make test-integration`) + lokalen Dev-Stack (`make up`, MQTT-Roundtrip Telemetrie + Fleet-Status manuell/über bestehende Tests geprüft) + `go vet`/`go test ./...`. Doku: `DECISIONS.MD`-Zeile "MQTT-Authentifizierung" auf ✅, `docs/architecture.md` (Communication-Layers-Tabelle), `tasks/backlog.md`-Status-Update. | S | 🔲 |

**Nicht Teil dieses Sprints:** TLS/MQTTS (Transportverschlüsselung) — siehe "Bewusster
Nicht-Scope" oben. Bleibt als möglicher Folge-Punkt im Backlog, falls später ein Anlass entsteht
(z. B. Deployment außerhalb eines vertrauenswürdigen internen Netzwerks).
