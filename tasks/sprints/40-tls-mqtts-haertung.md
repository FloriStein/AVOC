> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 40 — TLS/MQTTS-Härtung für Mosquitto

Ziel: Sprint 38 hat MQTT-Authentifizierung (Mosquitto `password_file`) aktiviert, aber
Transportverschlüsselung bewusst ausgeklammert — Credentials und Payloads reisen weiterhin im
Klartext über das interne Docker-Bridge-Netzwerk `avoc-net`. Dieser Sprint schließt diese Lücke:
Mosquitto TLS-Listener (Port 8883) statt Klartext-Port 1883, alle 4 MQTT-Consumer-Services
verbinden sich per `tls://` mit Server-Zertifikatsprüfung gegen eine projekteigene CA. Nutzer
wählt diesen Fokus am 2026-07-19 explizit gegenüber zwei Alternativen (Hexagonal-Migration Schritt
3 telemetry-service, Session-Recording-Storage-Entscheidung), mit Verweis auf CLAUDE.MD §0
Priorität 1 ("Sicherheit schlägt alles").

**Architektur-/Trust-Modell-Entscheidung (getroffen bei der Planung, nicht mehr offen):**
- **Serverseitiges TLS mit echter CA-Zertifikatsprüfung** (`RootCAs` im `tls.Config`, keine
  `InsecureSkipVerify`) — reine Verschlüsselung ohne Zertifikatsprüfung würde MITM-Angriffe im
  `avoc-net` nicht verhindern und widerspräche CLAUDE.MD §0. Eine projekteigene, selbstsignierte CA
  (analog zum bestehenden nginx-Zertifikatsmuster, aber mit `CN=mosquitto`/`SAN DNS:mosquitto`
  statt IP-basiertem SAN, da der Container-DNS-Name stabil ist) signiert das Mosquitto-Server-Zert.
- **Kein mTLS (Client-Zertifikate)** — würde die bestehende Username/Passwort-Authentifizierung aus
  Sprint 38 duplizieren, ohne zusätzlichen Sicherheitsgewinn in diesem Vertrauensmodell (ein
  Angreifer mit Netzwerkzugriff auf `avoc-net` hat ohnehin Zugriff auf alle Container-Volumes).
- **Harter Cutover auf Port 8883, kein Parallelbetrieb mit 1883** — Rule aus Abschnitt 15
  (Refactoring-Regeln): keine Feature-Flags/Backwards-Compat-Shims, wenn der Code einfach
  geändert werden kann. Alle 4 Consumer sind interne Go-Services im selben Repo/Deploy, es gibt
  keine externen MQTT-Clients, die auf 1883 angewiesen wären.

**Bewusster Nicht-Scope:**
- mTLS/Client-Zertifikate (siehe Trust-Modell-Entscheidung oben).
- CA-Rotationsstrategie für den Produktivbetrieb (Zertifikat gilt 365 Tage, analog zum
  bestehenden nginx-Zertifikat aus Sprint 10 — Rotation ist ein eigener, noch nicht dringender
  Folgepunkt, identisches Muster wie beim HTTPS-Zertifikat).
- Neues ADR — analog zur Sprint-38-Begründung (`tasks/backlog.md` Zeile ~526-531): ADR-003 legt
  Mosquitto bereits als Broker fest, TLS ist ein eingebauter Broker-Mechanismus (`listener`/
  `cafile`/`certfile`/`keyfile`), keine neue Architekturentscheidung. `DECISIONS.MD` wird nur um
  die neue Zeile "TLS/MQTTS-Härtung" ergänzt (bereits erledigt bei der Planung).
- SSM-Verteilung der CA — anders als `MQTT_USERNAME`/`MQTT_PASSWORD` (die für Sprint 38 aus SSM
  gezogen werden, weil sie *geheim* bleiben müssen) ist die CA nur ein **öffentliches**
  Zertifikat. Sie wird wie das bestehende nginx-Zertifikat lokal auf dem EC2-Host bei jedem
  Deploy generiert/wiederverwendet (`scripts/deploy.sh`) und per Compose-Volume an Mosquitto +
  die 4 Consumer-Container verteilt — kein Cross-Host-Bedarf, da alles auf demselben EC2-Host
  läuft.

**Nutzer-Budget:** ca. 200.000 Token (Standardvorgabe). Sechs klein geschnittene Tasks (S/M).

Vorrecherche (2026-07-19, vor Sprint-Start durchgeführt):

- **`infrastructure/mosquitto/mosquitto.conf`** und **`tests/mosquitto-test.conf`** (je 5 Zeilen,
  identisch): `listener 1883` / `allow_anonymous false` / `password_file
  /mosquitto/config/passwd` / `persistence false` / `log_dest stdout`. Kein TLS-Listener, keine
  `cafile`/`certfile`/`keyfile`-Direktiven, keine Vorarbeit vorhanden — Sprint 40 ergänzt/ersetzt
  `listener 1883` durch `listener 8883` + die drei TLS-Direktiven in beiden Dateien (Prod-Analogon
  liegt auf dem EC2-Host unter `$APP_DIR/mosquitto/mosquitto.conf`, identischer Inhalt wie Dev).
- **Drei Compose-Dateien**, Mosquitto-Service-Block: `infrastructure/compose/docker-compose.yml:128-136`
  (Dev), `infrastructure/compose/docker-compose.prod.yml:144-153` (Prod),
  `tests/docker-compose.test.yml:90-98` (Test) — überall `image: eclipse-mosquitto:2`, Port
  `1883:1883`, mountet `mosquitto.conf`+`passwd` read-only. Alle `MQTT_BROKER`-Werte sind reines
  `"mosquitto:1883"` (Go-Code prependt `tcp://`) — u.a. `docker-compose.yml:119,167,265,296`,
  `docker-compose.prod.yml:113,131,262`, `tests/docker-compose.test.yml:112,136`. Für Sprint 40:
  Port-Mapping auf `8883:8883`, neue Volume-Mounts für `ca.pem`/`cert.pem`/`key.pem` an Mosquitto,
  neuer read-only CA-Mount (`ca.pem`) an die 4 Consumer-Services (`telemetry-service`,
  `fleet-service`, `vehicle-mock`, `vehicle-mock-2`), `MQTT_BROKER` auf Port 8883 sowie neue
  `MQTT_CA_CERT`-Env-Var (Pfad zur gemounteten CA) an denselben 4 Stellen.
  In `tests/docker-compose.test.yml` zusätzlich `fleet-service`+`vehicle-mock` betroffen
  (siehe Test-Broker-Referenzen Zeile 112/136).
- **Drei Go-MQTT-Verbindungsstellen** (alle aus Sprint 38 MQTTAUTH-02 bekannt, Bibliothek
  `github.com/eclipse/paho.mqtt.golang v1.4.3`, unterstützt TLS nativ via
  `mqtt.NewClientOptions().SetTLSConfig(*tls.Config)` — im Repo bisher **nirgends** verwendet,
  `grep -r "SetTLSConfig\|tls.Config\|InsecureSkipVerify"` liefert 0 Treffer):
  - `internal/telemetryservice/client.go:45-58` (`Connect()`) — `AddBroker("tcp://" + c.broker)`
    Zeile 46, `SetUsername`/`SetPassword` Zeilen 48-49.
  - `internal/fleetgateway/mqtt.go:39-46` (`NewMQTTGateway`) — `AddBroker(broker)` Zeile 40
    (Broker kommt hier bereits mit `tcp://`-Präfix vom Aufrufer, `cmd/fleet-service/main.go`),
    `SetUsername`/`SetPassword` Zeilen 42-43.
  - `cmd/vehicle-mock/main.go:262-275` (`connectMQTT`) — `AddBroker("tcp://" + broker)` Zeile 264,
    `SetUsername`/`SetPassword` Zeilen 267-268.
  Für TLS: `tcp://` → `tls://` an allen drei Präfix-Stellen (bzw. beim Aufrufer für
  `fleetgateway`), plus `.SetTLSConfig(&tls.Config{RootCAs: pool})` mit `pool` aus
  `x509.NewCertPool()` + `AppendCertsFromPEM(caCertBytes)`, `caCertBytes` aus einem neuen
  `caCertPath`-Parameter (Env `MQTT_CA_CERT`) gelesen — analog zum bestehenden
  `username`/`password`-Parameter-Muster aus Sprint 38, kein Rule-2.3-Bundling nötig (bleibt bei 4
  Parametern: `broker, username, password, caCertPath`).
- **Bestehendes TLS-Muster (nginx/EC2, ADR-019/Sprint 10)**: `scripts/deploy.sh:98-115` generiert
  einmalig ein selbstsigniertes Zertifikat (`openssl req -x509 -newkey rsa:2048 -days 365 -nodes
  -subj "/CN=${TURN_EXTERNAL_IP}" -addext "subjectAltName=IP:${TURN_EXTERNAL_IP}"`) unter
  `$APP_DIR/ssl/`, wiederverwendet bei jedem erneuten Deploy (`if [ ! -f cert.pem ]`). Direkt im
  Anschluss (`scripts/deploy.sh:117-127`, MQTTAUTH-04) wird die Mosquitto-Passwd-Datei nach
  demselben "einmalig generieren, danach wiederverwenden"-Muster unter `$APP_DIR/mosquitto/`
  erzeugt. Sprint 40 fügt einen dritten Block nach demselben Muster ein: CA-Key + CA-Cert
  (selbstsigniert, `-subj "/CN=avoc-mosquitto-ca"`) einmalig erzeugen, danach Server-Cert für
  `CN=mosquitto`/`SAN DNS:mosquitto` mit dieser CA signieren — alles unter `$APP_DIR/mosquitto/`,
  analog zu `passwd`. Unterschied zum nginx-Zertifikat: dort vertraut der Browser interaktiv
  (Klick-Warnung), hier muss der Paho-Client dem Zertifikat programmatisch über `RootCAs`
  vertrauen — daher CA-Ansatz statt Ad-hoc-Self-Signed-Cert pro Verbindung.
- **`scripts/setup-ssm.sh:66-73`**: legt `MQTT_USERNAME`/`MQTT_PASSWORD` in SSM ab (Sprint 38,
  MQTTAUTH-04) — **kein** TLS-Cert-Handling nötig in SSM, da die CA öffentlich ist und lokal auf
  dem EC2-Host generiert wird (siehe Nicht-Scope oben, kein Cross-Host-Bedarf).
  `scripts/setup-ssm.sh` bleibt in diesem Sprint unverändert.
- **ADR-Lage**: `docs/adr/003-mqtt-broker.md` legt nur die Broker-Wahl fest (Mosquitto vs. EMQX),
  keine Transport-Security-Aussage. `docs/adr/019-deployment-strategy.md` listet TLS/MQTT-Auth als
  "Offen — für Testphase akzeptabel" (durch Sprint 10 bzw. 38 je einzeln geschlossen). Kein neues
  ADR nötig, siehe Nicht-Scope.
- **Testinfrastruktur**: `tests/docker-compose.test.yml:90-98` mountet aktuell
  `mosquitto-test.conf` + `mosquitto-passwd-test` (Sprint 38, committed, Credentials
  `avoc`/`mqtt_test_secret`, unkritisch da Test-only). Sprint 40 ergänzt ein analog committetes
  Test-CA/Zertifikat-Paar (z.B. `tests/mosquitto-certs/`, ebenfalls unkritisch, da nur für den
  CI-Testlauf). `internal/fleetgateway/mqtt_test.go:16-28` (`mqttTestBroker`/
  `mqttTestCredentials`-Helper) sowie die davon abhängigen Testdateien `fleet_simulation_test.go`
  und `fleet_service_test.go` (identische 3 Dateien wie in Sprint 38 MQTTAUTH-05) müssen auf
  `tls://`+CA umgestellt werden.
- **CLAUDE.MD-Leitplanken**: §0 (Sicherheit schlägt alles) rechtfertigt den Sprint-Fokus, analog
  zu Sprint 38. Abschnitt 17 (Teststandard) verlangt für Typ M/L auch Fehlerpfade — hier zusätzlich
  zum Sprint-38-Präzedenzfall ("anonyme Verbindung abgelehnt") ein Gegenprobe-Test/-Check, dass
  eine Verbindung ohne gültige CA-Prüfung (z.B. falscher/fehlender CA-Pfad) fehlschlägt, statt
  stillschweigend auf Klartext zurückzufallen.

Datum: 2026-07-19 (geplant) / 2026-07-20 (umgesetzt) | **Status: ✅ Abgeschlossen**
Vorgänger: Sprint 39 ✅ (Testabdeckung `safety-service`/`webrtc-sfu`/`internal/recording`, siehe
`tasks/sprints/39-testabdeckung-sicherheitsrelevanter-services.md`)
Branch/Worktree: `feature/fleet-service-foundation-sprint35` (bestehender Worktree, kein neuer
Branch).

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MQTTS-01 | Zertifikatserzeugung: selbstsignierte CA + Mosquitto-Server-Zertifikat (`CN=mosquitto`/`SAN DNS:mosquitto`, 365 Tage). Dev/Test: committete Zertifikate (analog zu `passwd`/`mosquitto-passwd-test`, unkritisch da self-signed + nur intern). Prod: `scripts/deploy.sh` erzeugt CA+Server-Cert einmalig unter `$APP_DIR/mosquitto/` nach demselben "generieren, dann wiederverwenden"-Muster wie das SSL-Zertifikat (Zeile 98-115) und die Passwd-Datei (Zeile 117-127). | S/M | ✅ |
| MQTTS-02 | `infrastructure/mosquitto/mosquitto.conf` + `tests/mosquitto-test.conf`: `listener 1883` → `listener 8883` + `cafile`/`certfile`/`keyfile`-Direktiven (harter Cutover, kein Parallelbetrieb mit 1883, siehe Nicht-Scope). Prod-Konfiguration (identischer Inhalt, liegt auf dem EC2-Host) analog anpassen. | S | ✅ |
| MQTTS-03 | Go-Client-TLS an allen 3 Verbindungsstellen (`internal/telemetryservice/client.go`, `internal/fleetgateway/mqtt.go`, `cmd/vehicle-mock/main.go`): `tcp://` → `tls://`, neuer `caCertPath`-Parameter, `SetTLSConfig(&tls.Config{RootCAs: pool})` mit `pool` aus `x509.NewCertPool()`+`AppendCertsFromPEM`. Neue `MQTT_CA_CERT`-Env-Var an den 4 Aufrufstellen (`cmd/telemetry-service/main.go`, `cmd/fleet-service/main.go`, `cmd/vehicle-mock/main.go`). | M | ✅ |
| MQTTS-04 | Drei Compose-Dateien: Port `1883:1883` → `8883:8883`, CA/Cert/Key-Volume-Mounts an Mosquitto, CA-Volume-Mount (read-only) an die 4 Consumer-Services, `MQTT_BROKER` auf Port 8883, neue `MQTT_CA_CERT`-Env-Var an denselben 4 Stellen. | S/M | ✅ |
| MQTTS-05 | Testinfrastruktur: committetes Test-CA/Zertifikat-Paar (`tests/mosquitto-certs/` o.ä.), `internal/fleetgateway/mqtt_test.go`-Helper (`mqttTestBroker`/`mqttTestCredentials`) + `fleet_simulation_test.go`/`fleet_service_test.go` auf TLS umstellen. Zusätzlich Regressionstest/-Check: Verbindung ohne gültige CA-Prüfung wird abgelehnt (Gegenprobe analog Sprint 38 "anonym abgelehnt"). | S/M | ✅ |
| MQTTS-06 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./...`, `make test-integration` (echter TLS-Broker im Docker-Test-Stack), manuelle Dev-Stack-Verifikation (Roundtrip über 8883 UND Ablehnung bei fehlender/falscher CA). Doku: `DECISIONS.MD`-Zeile "TLS/MQTTS-Härtung" auf ✅, `docs/architecture.md` falls dort MQTT-Verbindungsdetails dokumentiert sind, `tasks/backlog.md`-Status-Update. | S | ✅ |

**Nicht Teil dieses Sprints:** siehe "Bewusster Nicht-Scope" oben (mTLS/Client-Zertifikate,
CA-Rotationsstrategie, neues ADR, SSM-Verteilung der CA).

## Ergebnis (2026-07-20)

Umgesetzt wie geplant, mit einer Korrektur gegenüber der ursprünglichen Vorrecherche:

- **SAN-Erweiterung (Abweichung von der Planung):** Das Server-Zertifikat wurde ursprünglich mit
  `SAN DNS:mosquitto` generiert. Der erste `make test-integration`-Lauf schlug für die 3 Go-Tests
  fehl, die direkt vom Host aus verbinden (`connectTestMQTTClient` in `tests/integration/`,
  Broker-Adresse `localhost:18883`): `tls: failed to verify certificate: x509: certificate is
  valid for mosquitto, not localhost`. Go's TLS-Stack prüft den Hostnamen strikt gegen den SAN,
  anders als der erste manuelle `openssl s_client`-Check ohne `-verify_hostname`, der das Problem
  nicht aufdeckte. Fix: Zertifikate (Dev + Test) neu generiert mit
  `SAN DNS:mosquitto,DNS:localhost,IP:127.0.0.1` — funktioniert sowohl für Container-interne
  Verbindungen (`mosquitto:8883`) als auch für host-seitige Testclients (`localhost:18883`).
  `scripts/deploy.sh`s Prod-Zertifikatserzeugung entsprechend mitgezogen (kostet nichts, macht
  Ad-hoc-Debugging über Port-Forward robuster).
- **Neues `pkg/mqtttls`** (GOSTYLE Rule 3.1 — dritte identische Verwendungsstelle): zentrale
  `LoadClientConfig(caCertPath string) (*tls.Config, error)`, genutzt von `telemetryservice`,
  `fleetgateway` und `vehicle-mock`. Eigene Unit-Tests (`pkg/mqtttls/mqtttls_test.go`: valide CA,
  fehlende Datei, kein valides PEM).
- **`internal/telemetryservice`** (nach Sprint 43 bereits hexagonal migriert, ADR-031):
  `PahoConnectionConfig` um `CACertPath` erweitert, `NewPahoConnection` gibt jetzt `(*PahoConnection,
  error)` zurück (TLS-Config-Aufbau kann fehlschlagen), `Client.Connect()` propagiert den Fehler.
  `NewClient` um 4. Parameter `caCertPath` erweitert.
- **`internal/fleetgateway`** (noch nicht hexagonal migriert — Rohzugriff auf `mqtt.Client`
  bleibt unverändert): `NewMQTTGateway` um 4. Parameter `caCertPath` erweitert, lädt TLS-Config
  vor dem Connect.
- **`cmd/vehicle-mock/main.go`**: `connectMQTT` um `caCertPath`-Parameter erweitert, TLS-Config-Fehler
  führt zu `log.Fatal` (Konfigurationsfehler, kein Retry-Fall wie ein unerreichbarer Broker).
- **Gegenprobe-Test** `TestMQTTGateway_ConnectionRejectedWithoutValidCA`
  (`internal/fleetgateway/mqtt_test.go`): verbindet mit der *Dev*-CA gegen den *Test*-Broker (echte,
  unabhängige CA-Paare, kein synthetischer Mismatch) — Connect schlägt fehl wie erwartet. Manuell
  gegen einen laufenden Test-Broker verifiziert (`MQTT_BROKER=localhost:18883 ... go test -run
  TestMQTTGateway`), alle 5 Tests inkl. Gegenprobe grün.
- **`tests/integration`**: neuer gemeinsamer Helper `connectTestMQTTClient` in `setup_test.go`
  (GOSTYLE Rule 3.1 — dritte identische Connect-Boilerplate-Stelle in `fleet_simulation_test.go`/
  `fleet_service_test.go`), TLS-Config aus `tests/mosquitto-certs/ca.pem`.

**Verifikation:**
- `go build ./...`, `go vet ./...`, `gofmt -l .` — alle sauber.
- `go test ./internal/... ./cmd/... ./pkg/... -race -count=1` — grün (inkl. neuem
  `pkg/mqtttls`-Paket).
- `make test-integration` (echter TLS-Broker im Docker-Test-Stack, alle Services) — 32/32 Tests
  grün nach der SAN-Korrektur.
- Manuell: `openssl s_client` gegen `localhost:18883` mit korrekter Test-CA → `Verify return code:
  0 (ok)`; mit der (falschen) Dev-CA → `Verify return code: 19 (self-signed certificate in
  certificate chain)`. Echter `mosquitto_pub`/`mosquitto_sub`-Roundtrip über TLS im Docker-Test-Netz
  erfolgreich.
- Bekannter, vorbestehender Flake (nicht durch diesen Sprint verursacht — `internal/fleetgateway/
  mock.go`/`mock_test.go` unverändert, siehe `git diff`):
  `TestMockGateway_Stop_StopsProducingEvents` schlägt gelegentlich unter Last fehl (2/3 grün bei
  isolierten Wiederholungsläufen), isoliert 4/4 grün. Bereits in Sprint 41-44-Merge-Verifikation
  dokumentiert, außerhalb des Scopes dieses Sprints.

**Nicht Teil dieses Sprints (wie geplant):** mTLS/Client-Zertifikate, CA-Rotationsstrategie, neues
ADR, SSM-Verteilung der CA.

Doku aktualisiert: `DECISIONS.MD` (Zeile "TLS/MQTTS-Härtung für Mosquitto" ✅), `docs/architecture.md`
(MQTT-Layer-Abschnitt), `docs/adr/019-deployment-strategy.md` (Folge-Entscheidungen-Tabelle,
inkl. Korrektur der veralteten "MQTT-Authentifizierung offen"-Zeile auf ✅ Sprint 38),
`tasks/backlog.md` (EPIC + alle 6 Tasks ✅).
