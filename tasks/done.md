# Done

Lifecycle: backlog → sprint → done

Kompakter Index aller abgeschlossenen Sprints. Volltext (Tasks, Testprotokolle, Datei-Listen,
Scope-Details) je Sprint in [tasks/sprints/](sprints/).

---

## Sprint 50 — TelemetryWatchdog (DRIFT-K3-TELEMETRY Teil 2) ✅
2026-07-21 → [tasks/sprints/50-telemetry-watchdog.md](sprints/50-telemetry-watchdog.md)
- Grill-Me-Session (Typ L) klärt die vier in Sprint 47 offen gelassenen Schwellwerte: 2s
  Poll-Interval, 2 Fehlschläge Threshold (4s-Budget), Timestamp-Alter zählt gleichwertig zu HTTP 404
  ("nie/nicht aktuell empfangen" — `telemetry-service`s `/telemetry/latest/{id}`-Cache ist nicht
  session-gebunden, stale Werte aus Vorsessions dürfen einen echten Ausfall nicht verdecken), 3s
  HTTP-Timeout analog `SafetyBusWatchdog`.
- Neues Paket `internal/controlserver/telemetrycheck`: `Checker.HasFreshTelemetry(...)` (HTTP-Poll)
  + `TelemetryWatchdog` (Lifecycle `Start`/`Stop`, per-`VehicleContext` wie `AuthWatchdog`, feuert
  über die neue exportierte `statemachine.Machine.TransitionTelemetry(healthy bool)`). Bewusst ohne
  Safety-Bus/Audit-Anbindung (DEGRADED-only-Ursachen nutzen diesen Kanal in diesem Code nicht, siehe
  Media-Präzedenz) — abweichend vom ursprünglichen Taskzuschnitt, mit Nutzer nicht extra
  rückgefragt (reine Konsistenz mit bestehendem Muster).
- **Ungeplanter Bugfix (mit Nutzer abgestimmt, da er den als "nicht anfassen" markierten
  `TransitionMedia`-Code berührt):** `TransitionMedia`/`TransitionTelemetry`s äußerer Guard prüfte
  nur `System == StateConnected` vor `enterDegraded` — ein zweiter, unabhängiger DEGRADED-Grund,
  der eintraf während SYSTEM bereits wegen eines ersten Grundes DEGRADED war, wurde nie ins
  `degradedReasons`-Set aufgenommen; die Recovery des ersten Grundes hätte DEGRADED dann fälschlich
  komplett aufgehoben. Guard erweitert auf `StateConnected || StateDegraded` in beiden Funktionen,
  Regressionstest `TestSecondCause_ArrivingAfterFirstCauseAlreadyDegraded`.
- Wiring: `vehiclecontext.Registry.WithTelemetryChecker(...)` (optionales `With*`-Muster wie
  `AuthWatchdog`, um `NewRegistry`s Signatur und damit 6 bestehende Testaufrufstellen nicht
  anzufassen — in Produktion trotzdem immer aktiv), `cmd/control-server/main.go` neue
  `TELEMETRY_SERVICE_URL`-Env-Var, Start/Stop an allen drei bestehenden `AuthWatchdog`-Call-Sites.
- `tests/docker-compose.test.yml`: `telemetry-service` fehlte komplett im Integrations-Teststack —
  neuer Service-Block ergänzt (analog `fleet-service`s MQTT-TLS-Setup), `control-server` erhält
  `TELEMETRY_SERVICE_URL` + `depends_on`. Neuer Integrationstest
  `TestIntegration_TelemetryLoss_TriggersDegrade_ThenRecovers` publiziert direkt per MQTT (eigener
  Test-Client) statt über `vehicle-mock`s Dauerschleife — volle Timing-Kontrolle. Gegen echten
  `make test-integration`-Lauf verifiziert: 33/33 grün, keine Regression.
- 25 neue Unit-Tests (`statemachine`, `telemetrycheck`, `tests/unit/watchdog_test.go`) + 1
  Integrationstest. `go build`/`go vet`/`gofmt -l`/`go test ./internal/... ./tests/unit/... ./pkg/...
  -race -count=2` grün.

---

## Sprint 47 — Multi-Cause-DEGRADED-Fundament in der State Machine (DRIFT-K3-TELEMETRY Teil 1) ✅
2026-07-21 → [tasks/sprints/47-multi-cause-degraded-fundament.md](sprints/47-multi-cause-degraded-fundament.md)
- Zwei Grill-Me-Entscheidungen (Typ L, Kernsystem State Machine/Sicherheitsmodell): Poll-basierter
  `TelemetryWatchdog` (analog `SafetyBusWatchdog`/`AuthWatchdog`, Sprint 48) statt aktiver Meldung
  vom `telemetry-service`; Multi-Cause-DEGRADED jetzt beheben statt zurückstellen, da sonst
  `TransitionMedia`s Recovery-Zweig einen unabhängigen Telemetrie-DEGRADED-Trigger fälschlich mit
  aufheben würde, sobald nur das Video sich erholt.
- `internal/controlserver/statemachine/state.go`: neuer Typ `DegradedReason`
  (`DegradedReasonMedia`/`DegradedReasonTelemetry`), Feld `degradedReasons
  map[DegradedReason]bool` auf `Machine`, private Helper `enterDegraded`/`exitDegraded`.
  `TransitionMedia` darauf umgestellt — Single-Cause-Fall (nur Media) bit-identisch zu vorher.
  `transitionSystemLocked`s SAFE_MODE-Zweig leert das Set zusätzlich.
- Neue `internal/controlserver/statemachine/state_test.go` (bisher kein eigenes internes
  Testfile) — Regressionsschutz Media-Fall, Multi-Cause-Szenario (zwei Gründe aktiv, nur beide
  Recovery führt zurück zu CONNECTED), SAFE_MODE-Reset-Verhalten. `go build`/`go vet`/
  `go test ./internal/controlserver/... ./tests/unit/... -race -count=2` grün.
- ADR-009 Update-Block dokumentiert beide Entscheidungen + vollständige Architekturskizze für
  Sprint 48 (`TelemetryWatchdog`-Paket, Lifecycle, `TELEMETRY_SERVICE_URL`), damit Sprint 48 nicht
  erneut recherchieren muss. Der Watchdog selbst ist explizit nicht Teil dieses Sprints.

---

## Sprint 49 — Lokale Ansible-VM als Hetzner-Nachbildung, Teil B: Verifikation (LOCALVM-08a/08b/08c/09) ✅
2026-07-21 → [tasks/sprints/49-lokale-ansible-vm-teil-b.md](sprints/49-lokale-ansible-vm-teil-b.md)
- Fortsetzung von Sprint 48 (reine Autorierung) — jetzt real gegen eine laufende lokale VM
  (`avoc-local-vm`, libvirt/KVM) verifiziert. EPIC "Lokale Ansible-VM als Hetzner-Nachbildung"
  damit vollständig abgeschlossen (Teil A+B).
- LOCALVM-08a gegengecheckt (VM-IP unverändert, Inventory-Platzhalter musste nachträglich befüllt
  werden — Sed-Ersetzung aus dem Vorlauf war nicht persistiert). LOCALVM-08b:
  `ansible-playbook site.yml`/`deploy.yml` real ausgeführt, 5 reale, vorher unbekannte Bugs
  gefunden und behoben: (1) `apt upgrade` bricht SSH ab, sobald `openssh-server` mit upgegradet
  wird (Fix: async-Task + Reconnect-Pattern), (2) Docker-Apt-Repo mit falscher
  Architekturbezeichnung (`ansible_architecture`=`x86_64` statt `dpkg --print-architecture`=
  `amd64` — stiller Fehlschlag, kein Docker-Paket verfügbar), (3) `ufw`-Kommentar mit Apostroph
  bricht das Ansible-Modul-Quoting, (4) Mosquitto-Container-Crash-Loop durch zu strikte
  Dateiberechtigungen (`chmod 600` statt `644` auf Bind-Mounts), (5) MQTT-Test-Passwort passte
  nicht zum committeten `infrastructure/mosquitto/passwd`-Hash. Zusätzlich (kein Ansible-Bug):
  lokal gebaute avoc-*-Images waren älter als der TLS-Client-Support aus Sprint 40 — neu gebaut.
- LOCALVM-08c: Smoke-Test grün — alle 15 Container `Up`/`postgres healthy`, keine Restart-Loops,
  Frontend (`:3000`) und Control-Server-API (`:8080/health`) beide HTTP 200 von der VM-IP aus.
- LOCALVM-09: `docs/deployment/hetzner-setup.md` auf den Ansible-Workflow umgestellt (Schritt
  1/3/4/6/7 durch Kurzbeschreibung + Ansible-/Skript-Verweis ersetzt, Rest unverändert für den
  echten Server gültig); zwei Doku-Lücken aus Sprint 48 mitkorrigiert (MQTT-Port-Tabelle auf
  TLS-8883 statt 1883, `docs/deployment/UEBERGABE-ABWEICHUNGEN.md` neu angelegt); `DECISIONS.MD`
  + `tasks/backlog.md`-Status aktualisiert (EPIC komplett ✅).

---

## Sprint 48 — Lokale Ansible-VM als Hetzner-Nachbildung, Teil A: Autorierung (LOCALVM-01..07) ✅
2026-07-20 → [tasks/sprints/48-lokale-ansible-vm-teil-a.md](sprints/48-lokale-ansible-vm-teil-a.md)
- Nutzerfreigabe 2026-07-20: AWS als lokale Test-/Referenzumgebung durch eine lokale,
  per Ansible provisionierte VM ersetzen, die den zukünftigen Hetzner-Server nachbildet
  (libvirt/KVM + `virt-install` + cloud-init, kein Vagrant/VirtualBox). Teil A deckt reine
  Autorierung ab (kein Zugriff auf eine echte laufende VM nötig) — Teil B (Verifikation gegen die
  echte VM, LOCALVM-08/09) folgt als eigener Sprint.
- Neues `ansible/`-Grundgerüst (Rollen `bootstrap`/`firewall`/`secrets`, Playbooks
  `site.yml`/`deploy.yml`), `scripts/local-vm-create.sh` (virt-install + cloud-init NoCloud-ISO),
  sowie erstmalige Materialisierung der drei bisher nur in `docs/deployment/hetzner-setup.md`
  dokumentierten Dateien als echten Code: `infrastructure/compose/docker-compose.hetzner.yml`,
  `scripts/deploy-hetzner.sh`, `scripts/secrets-setup-hetzner.sh`.
- Eine dokumentierte, minimale Abweichung vom 1:1-Materialisierungsauftrag: `deploy-hetzner.sh`
  bekam einen optionalen `SKIP_REGISTRY_PULL`-Zweig, ohne den die bereits freigegebene
  "kein-Docker-Hub-Roundtrip"-Architektur-Entscheidung für die lokale VM nicht hätte funktionieren
  können (Default-Verhalten für den echten Hetzner-Server bleibt unverändert).
- `ansible`/`ansible-playbook` konnten in dieser Agenten-Session nicht per `sudo apt-get install`
  systemweit installiert werden (kein TTY für interaktive `sudo`-Passwortabfrage) — als Ersatz
  lokal aus denselben Ubuntu-Paketen (`apt-get download` + `dpkg-deb -x`, kein Root nötig)
  verifiziert. Für den Nutzer offen: `sudo apt-get install -y ansible` einmalig manuell
  ausführen.
- Verifiziert: `ansible-playbook site.yml/deploy.yml --syntax-check` + `--list-tasks` (beide
  grün), alle YAML-Dateien einzeln per `yaml.safe_load` geprüft, `bash -n` für alle 3 neuen/
  geänderten Shell-Skripte, `docker compose -f docker-compose.hetzner.yml config` gegen
  Dummy-`.env` (Exit 0, alle 5 dokumentierten Hetzner-Änderungen im aufgelösten YAML bestätigt).
  Kein echter Playbook-Lauf/keine echte VM-Erzeugung (explizit Sprint B). Details, erkannte
  Doku-Lücken (`hetzner-setup.md` teils veraltet seit MQTTS-01) und vollständige Testliste im
  Sprint-Dokument.

## Sprint 46 — control-server: Hexagonal-Migration Vorbereitung (neues ADR-035 + Testaufbau) ✅
2026-07-20 → [tasks/sprints/46-control-server-hexagonal-migration-prep.md](sprints/46-control-server-hexagonal-migration-prep.md)
- Schließt den seit ADR-031 offenen Punkt "control-server bräuchte eigenes ADR + Testabdeckung-Aufbau zuerst". Nutzerentscheidung 2026-07-20 gegenüber 2 Alternativen (Hexagonal-Migration safety-service/webrtc-sfu/recording — verworfen, keine vergleichbare Infrastruktur-Abhängigkeit dort; `newSafetyMux` exportieren — zu klein).
- Vorrecherche korrigierte die ursprüngliche "geringste Testabdeckung"-Annahme: `internal/controlserver`-Subpakete sind über die bestehende `tests/unit`-Suite bereits ~84% getestet (nur mit `-coverpkg` sichtbar). Die echte Lücke ist `cmd/control-server/main.go` selbst (919 Zeilen, 0% Coverage auf jeder Funktion).
- Neues ADR `docs/adr/035-control-server-hexagonal-migration-prep.md` (36 ADRs gesamt). Testaufbau (kein Produktivcode-Refactor) für den sicherheitskritischsten Teil: `requireJWT`, `handleSessionStart`/`advanceVehicleToActiveOperator`, `handleSessionEnd`, `handleEmergencyStop` (neue `cmd/control-server/main_test.go`, 20 Tests) sowie `authcheck.Checker` (neue, `DATABASE_URL`-gated `checker_test.go`, 3 Tests).
- Echter Fund dabei: ein Fahrzeug ohne aktive Session steht in SYSTEM=IDLE, und `IDLE→SAFE_MODE` ist kein gültiger State-Transition — Emergency-Stop auf so ein Fahrzeug wird lautlos ignoriert (nur Warn-Log). Dokumentiert als offener Entscheidungspunkt im ADR, nicht behoben (Scope: Vorbereitung, kein Fix).
- Verifiziert: `go build`/`go vet ./...` sauber, `go test ./cmd/control-server/... ./internal/controlserver/authcheck/... -race -count=2` zweimal grün; `authcheck`-Tests zusätzlich gegen einen ephemeren `postgres:16-alpine`-Container real verifiziert; `go test ./... -race` gesamt geprüft (nur erwartbare `tests/integration`-Fehlschläge ohne Docker-Teststack).

## Sprint 45 — Hexagonale Architektur: Use-Case-Extraktion fleet-service/auth-service (HEX-06/HEXAUTH-04) ✅
2026-07-20 → [tasks/sprints/45-hexagonal-usecase-extraktion.md](sprints/45-hexagonal-usecase-extraktion.md)
- Schließt die beiden seit Sprint 33/37 offenen optionalen ADR-031-Entscheidungspunkte (HEX-06 fleet-service, HEXAUTH-04 auth-service). Nutzerentscheidung 2026-07-20 gegenüber 2 Alternativen (Hexagonal-Migration safety-service/webrtc-sfu/recording; control-server-Vorbereitung per neuem ADR + Testaufbau).
- Vorrecherche ergab: die meisten Endpunkte beider Handler sind reine 1:1-Store-Pass-Throughs ohne Entscheidungslogik — dafür bewusst keine Use-Case-Schicht (Rule of Three / "keine Abstraktion ohne Konsument"). Extrahiert wurden nur Endpunkte mit echter Orchestrierung: `internal/fleetservice/usecase.go` (`createAndDispatchTask`, `transitionTaskStatus`/`acknowledgeAlert` — Store-Aufruf + Dashboard-Broadcast als eine Einheit) und `internal/authservice/usecase.go` (`login`/`refreshToken`/`handoverToken` mit Sentinel-Fehlern für die HTTP-Statuscode-Zuordnung, plus `canModifyUser` löst eine echte Code-Duplizierung zwischen `DeleteUser`/`UpdateUserRole` auf).
- `Handler`-Methoden unverändert in Signatur/HTTP-Verhalten, nur noch dünne Wrapper. 17 neue Unit-Tests (7 fleet-service, 10 auth-service) direkt gegen die reinen Funktionen ohne `httptest`; Broadcast-Verifikation nutzt bestehende `broadcast_test.go`-WebSocket-Helper.
- Damit ist die Hexagonal-Migration für fleet-service/auth-service/telemetry-service inkl. beider optionaler Folgeschritte vollständig abgeschlossen — `safety-service`/`webrtc-sfu`/`internal/recording` und `control-server` bleiben die einzigen noch nicht migrierten Services (beide eigene, noch offene Entscheidungspunkte).
- Verifiziert: `go build`/`go vet ./...` sauber, `go test ./internal/fleetservice/... ./internal/authservice/... -race -count=2` zweimal grün, keine Flakiness.

## Sprint 44 — Backup-Strategie Audit Store (ADR-018/023 Folge) ✅
2026-07-19 → [tasks/sprints/44-backup-strategie-audit-store.md](sprints/44-backup-strategie-audit-store.md)
- Schließt seit ADR-019 offenen Punkt "Audit Store Backup-Strategie" — Backlog-Text war veraltet ("SQLite Volume"), korrigiert auf PostgreSQL (`postgres-data`-Volume, seit ADR-023). Nutzerentscheidung 2026-07-19 gegenüber 2 Alternativen (Migration zu AWS ECR, Session-Recording-Storage-Entscheidung).
- Neues `scripts/backup-audit-store.sh`: täglicher `pg_dump`+`gzip`-Dump gegen den laufenden `postgres`-Container, Upload nach `s3://<AppBucket>/backups/postgres/<Datum>-avoc.sql.gz`. Bucket-Name per neuem CDK-`ssm.StringParameter` (`/avoc/prod/backup-bucket-name`), 30-Tage-S3-Lifecycle-Regel auf dem Prefix. `scripts/deploy.sh` registriert den Cronjob (03:00 UTC) idempotent bei jedem Deploy.
- Bewusst nicht Teil des Scopes: Point-in-Time-Recovery (`pg_basebackup`/WAL), automatisierter Restore-Test, client-seitige Verschlüsselung des Dumps.
- Verifiziert: lokaler Trockenlauf des unveränderten Skripts gegen den echten Dev-`postgres`-Container (Fake-`aws`-Wrapper statt echtem SSM/S3-Call) — erzeugte echten `pg_dump`-Dump, gzip-valide, korrekter S3-Pfad. Cron-Idempotenz-Idiom zusätzlich gegen den echten `crontab`-Befehl der Maschine geprüft (Doppel-Registrierung ausgeschlossen, vorhandene fremde Einträge bleiben erhalten). Kein `cdk deploy`/echter AWS-Call (separater späterer Schritt).

## Sprint 43 — Hexagonale Architektur-Migration Schritt 3: telemetry-service (ADR-031) ✅
2026-07-19 → [tasks/sprints/43-hexagonal-migration-telemetry-service.md](sprints/43-hexagonal-migration-telemetry-service.md)
- Fortsetzung nach Pilot (fleet-service, Sprint 33) und Schritt 2 (auth-service, Sprint 37) — Nutzerfreigabe 2026-07-19 für Schritt 3 (telemetry-service).
- Neuer `MQTTConnection`-Port (`internal/telemetryservice/mqttconnection.go`, 4 Methoden) statt direkter Kopplung an `paho.mqtt.golang`s 14-Methoden-`mqtt.Client`; `PahoConnection`-Adapter kapselt `mqtt.Token`/`mqtt.Message` intern. `Client` importiert paho danach nicht mehr direkt. `NewClient`-Signatur unverändert.
- Erste Testabdeckung für beide Pakete (zuvor 0 Tests): 16 neue Unit-Tests (`FakeMQTTConnection`-Testdoppel, `client_test.go`, `cmd/telemetry-service/main_test.go`), `-race`-sauber, 2x gegen Flakiness verifiziert.

## Sprint 42 — Testing-Debt aus ADR-006-Bestandsaufnahme schließen ✅
2026-07-19 → [tasks/sprints/42-testing-debt-adr-006.md](sprints/42-testing-debt-adr-006.md)
- Schließt die zwei kleineren Funde aus der ADR-006/CLAUDE.MD-§17-Bestandsaufnahme (2026-07-19), die hinter den priorisierten CI-Gates (Sprint 41, separater Worktree) zurückgestellt wurden. Ausgeführt in eigenem Worktree (`feature/fleet-service-foundation-testdebt`), keine Code-Überschneidung mit Sprint 41.
- SFUCONC-01: `TestSFU_ConcurrentSessionEventsAndPeerOps` in `internal/webrtcsfu/sfu_test.go` — Concurrency-Test für `HandleSessionEvent`/`registerOperatorSubscription`/`removePeer`, analog `bus_test.go`s `TestBus_ConcurrentPublishAndRead`. `n` von anfänglich 50 auf 20 reduziert, nachdem 50 echte `webrtc.PeerConnection`-Instanzen unter `-race` einen seltenen Flake in einem unveränderten Nachbarpaket (`fleetgateway`) ausgelöst hatten.
- SAFETYBUS-01/02: neue `tests/unit/safety_bus_integration_test.go` verdrahtet einen echten `HTTPPublisher` gegen einen echten `safetyservice.NewBus()` über einen lokalen `httptest.Server` (dupliziert `newSafetyMux`s zwei Zeilen, kein Produktivcode-Umbau) — Dead-man-Timeout und ACK-Timeout jetzt gegen den echten Bus statt nur gegen `MockSafetyPublisher` verifiziert. `safety_test.go`-Header und ADR-006 Teil 3 präzisiert.
- Verifiziert: `go build`/`go vet` sauber, `go test ./... -race` **3 aufeinanderfolgende volle Läufe grün** (vorbestehende `tests/integration`-Fehlschläge unverändert, Docker-Test-Stack nicht gestartet — per Vergleichslauf auf dem Basis-Commit bestätigt).

## Sprint 41 — CI-Gates einführen (ADR-006-Bestandsaufnahme) ✅
2026-07-20 → [tasks/sprints/41-ci-gates-einfuehren.md](sprints/41-ci-gates-einfuehren.md)
(Branch `feature/fleet-service-foundation-cigates`, Basis `feature/fleet-service-foundation-sprint35`
— zum Zeitpunkt dieses Sprints war Sprint 40 (TLS/MQTTS-Härtung, eigener Fokus direkt im
`sprint35`-Worktree, kein separater Branch) noch nicht umgesetzt; inzwischen nachgeholt, siehe
Sprint 40 unten.)
- Schließt größten Befund einer ADR-006/CLAUDE.MD-§17-Bestandsaufnahme: `.github/workflows/` hatte nur non-blocking `lint.yml`, die dokumentierte Pipeline existierte nicht. 4 neue Workflow-Dateien: `test-go.yml` (3 blockierende Jobs: Unit/Safety/Integration), `test-frontend.yml` (blockierend, Vitest), `test-latency.yml` (bewusst non-blocking, Shared-Runner-Rauschen), `test-e2e.yml` (non-blocking Playwright-Informational-Job). Neuer `Makefile`-Target `test-unit`.
- Zwei vorbestehende Bugs gefunden und behoben: `make test-k6` lief nie (fehlendes `-i` bei `docker run`, Skript kam nie im Container an), `frontend/src/gen/` (gitignored, build-time generiert) fehlte für CI-Vitest-Läufe — inkl. dem bekannten Root-owned-`node_modules`-Problem aus Docker-basierter Proto-Generierung (README.md-Troubleshooting-Muster wiederverwendet).
- Zwei weitere Bugs gefunden, bewusst NICHT gefixt (Testverhalten statt CI-Wiring, siehe `DECISIONS.MD`): `BenchmarkControlACKRoundtrip` skipt immer (fehlender `session_id`-Parameter, ADR-025-Drift), `dashboard.spec.ts` erwartet Dashboard-Inhalt ohne Login-Pflicht.
- Branch-Protection (Required Status Checks) vorbereitet/dokumentiert (`README.md`), bewusst nicht aktiviert — geteiltes Repo-Setting, braucht explizite Nutzerbestätigung.
- Verifiziert: `go build`/`go vet` sauber, alle 3 `test-go.yml`-Jobs + `test-frontend.yml` je 2× frisch lokal grün (keine Flakiness). Alle 5 Workflow-Dateien `actionlint`-sauber. Echte GitHub-Actions-Läufe stehen aus (kein Push in diesem Sprint, siehe Sprint-Dokument).

## Sprint 40 — TLS/MQTTS-Härtung für Mosquitto ✅
2026-07-19 (geplant) / 2026-07-20 (umgesetzt) → [tasks/sprints/40-tls-mqtts-haertung.md](sprints/40-tls-mqtts-haertung.md)
- Schließt den bei Sprint 38 (MQTT-Authentifizierung) bewusst ausgeklammerten Punkt Transportverschlüsselung: Mosquitto TLS-Listener Port 8883 (Port 1883 hart abgeschaltet, kein Parallelbetrieb), projekteigene selbstsignierte CA signiert das Mosquitto-Server-Zertifikat. Alle 3 Go-MQTT-Verbindungsstellen (`telemetryservice`, `fleetgateway`, `vehicle-mock`) verbinden per `tls://` mit echter Zertifikatsprüfung (kein `InsecureSkipVerify`), CA-Lade-Logik zentral in neuem `pkg/mqtttls` (GOSTYLE Rule 3.1). Kein mTLS, kein neues ADR (ADR-003 deckt Mosquitto bereits ab).
- Fund während der Umsetzung: das ursprünglich geplante Zertifikat (`SAN DNS:mosquitto`) ließ host-seitige Go-Integrationstests an Go's Hostname-Verifikation scheitern (`localhost:18883` ≠ `mosquitto`) — SAN um `DNS:localhost,IP:127.0.0.1` erweitert (Dev+Test+Prod-Erzeugung in `scripts/deploy.sh`).
- Neuer Gegenprobe-Test `TestMQTTGateway_ConnectionRejectedWithoutValidCA` (echte Dev-CA gegen Test-Broker, kein synthetischer Mismatch) — verifiziert manuell gegen einen laufenden Broker.
- Verifiziert: `go build`/`go vet`/`gofmt` sauber, `go test ./... -race` grün, `make test-integration` (32/32 gegen echten TLS-Broker) grün, manuelle `openssl s_client`- und `mosquitto_pub`/`sub`-Roundtrip-Verifikation über TLS.

## Sprint 39 — Testabdeckung `safety-service`/`webrtc-sfu`/`internal/recording` ✅
2026-07-19 → [tasks/sprints/39-testabdeckung-sicherheitsrelevanter-services.md](sprints/39-testabdeckung-sicherheitsrelevanter-services.md)
- Schließt seit der ADR-031-Bestandsaufnahme (2026-07-16) offene Lücke: drei Services mit 0 automatisierten Tests, darunter der Safety Event Bus (`safety-service`). Nutzerentscheidung 2026-07-19 gegenüber 3 Alternativen (Hexagonal-Migration Schritt 3, TLS/MQTTS-Härtung, Session-Recording-Storage), Begründung CLAUDE.MD §0 Priorität 1 ("Sicherheit schlägt alles").
- 36 neue Unit-Tests über 5 Testdateien: `internal/recording/memory_recorder_test.go`, `internal/safetyservice/bus_test.go`, `cmd/safety-service/main_test.go`, `internal/webrtcsfu/sfu_test.go`, `cmd/webrtc-sfu/main_test.go`. `-race`-sauber für `safetyservice`/`webrtcsfu`. Reine Testabdeckung, 0 Produktivcode-Änderungen.
- Bewusst nicht Teil des Scopes: E2E-WebRTC-SDP-Negotiation (ADR-006: "zu flaky in CI"), `SFU.forwardTrack`, Hexagonal-Migration der drei Services selbst.
- Verifiziert: `go build`/`go vet` sauber, `go test ./...` grün für alle betroffenen Pakete (vorbestehende `tests/integration/...`-Fehlschläge unverändert, da Docker-Test-Stack nicht gestartet).

## Sprint 38 — MQTT-Authentifizierung (Mosquitto Passwort-File) ✅
2026-07-19 → [tasks/sprints/38-mqtt-authentifizierung.md](sprints/38-mqtt-authentifizierung.md)
- Schließt seit Sprint 9 offenen Sicherheits-Gap: Mosquitto lief mit `allow_anonymous true` — jeder Prozess im `avoc-net` konnte ohne Credentials publizieren/subscriben. Nutzerentscheidung 2026-07-19, Priorität vor Hexagonal-Migration Schritt 3 (CLAUDE.MD §0 "Sicherheit schlägt alles").
- `password_file`-Mechanismus aktiviert (Dev/Test/Prod), alle 3 Go-MQTT-Clients (`telemetryservice`, `fleetgateway`, `vehicle-mock`) setzen `MQTT_USERNAME`/`MQTT_PASSWORD`, Prod-Credential nur über SSM SecureString + deploy-zeitige Passwd-Generierung auf dem EC2-Host (nie committed).
- Bewusst nicht Teil des Scopes: TLS/MQTTS (Transportverschlüsselung) — Klartext-Credentials über das interne Docker-Bridge-Netzwerk, analog `DATABASE_URL`.
- Verifiziert: `go build`/`go vet` sauber, `go test ./...` ohne Regression, `make test-integration` 30/30 grün gegen den echten authentifizierten Broker, manuelle Dev-Stack-Verifikation bestätigt Roundtrip UND Ablehnung anonymer Verbindungen ("Connection Refused: not authorised").

## Sprint 37 — Hexagonale Architektur-Migration, Schritt 2: auth-service (JWT-Port) ✅
2026-07-19 → [tasks/sprints/37-auth-service-hexagonal-jwt-port.md](sprints/37-auth-service-hexagonal-jwt-port.md)
- ADR-031 Schritt 2: `TokenIssuer`-Port + `JWTTokenIssuer`-Adapter (`internal/authservice/tokenissuer.go`) — `Handler.secret` → `Handler.tokens TokenIssuer`, `NewHandler`-Signatur unverändert.
- `handler.go` importiert `golang-jwt/jwt/v5` danach nicht mehr direkt (`Claims`-Typ mit nach `tokenissuer.go` verschoben) — reine Dependency Inversion, keine neue DB-Unabhängigkeit (anders als beim `fleet-service`-Piloten).
- Verifiziert: `go build`/`go vet` sauber, `go test ./internal/authservice/...` 22/22 grün, kein Docker-Stack nötig. HEXAUTH-04 (Use-Case-Extraktion) und `telemetry-service` (Schritt 3) bleiben eigene Folgeschritte.

## Sprint 36 — Restposten-Bereinigung III (Tech Debt, Formatierung, Prod-Lücke, Test-Gap) ✅
2026-07-19 → [tasks/sprints/36-restposten-bereinigung-iii.md](sprints/36-restposten-bereinigung-iii.md)
- TECHDEBT-01: `internal/authservice/noop_userstore.go` (komplett unreferenziert) gelöscht.
- GOSTYLE-FMT-01: `gofmt -w .` für die 9 verbleibenden unformatierten Dateien, reine Whitespace-Änderung.
- DEPLOY-08: `fleet-service` in `infrastructure/compose/docker-compose.prod.yml` ergänzt (Port 8085, Postgres+Mosquitto-Dependencies, Env analog `auth-service`/`telemetry-service`).
- TESTGAP-01: 3 skippende WS-Integrationstests gefixt — zusätzlich zur geplanten Reihenfolge-Umkehr musste die Test-Vehicle-Registrierung über `/vehicle/ws` ergänzt werden (`vehicleRegistry.Connected`-Check in `handleSessionStart`, bisher nicht dokumentiert). Gegen echten Docker-Test-Stack verifiziert: 27/27 grün, 0 Skips.

## Sprint 35 — GOSTYLE-IF-03/04 (Phase-2-Abschluss Interface-Segregation) ✅
2026-07-18 → [tasks/sprints/35-gostyle-if-phase2-abschluss.md](sprints/35-gostyle-if-phase2-abschluss.md)
- `pkg/audit.AuditWriter` in schlankes `SafetyAuditWriter` (`WriteSync`-only) für die 5 Safety-/Command-Consumer aufgespalten, `Close` aus beiden Interfaces entfernt (`NoopWriter.Close` dadurch tot geworden und mit gelöscht).
- `SeedAdmin` aus `authservice.UserStore` entfernt; Nebenbefund `NoopUserStore` komplett ungenutzt bestätigt und als `TECHDEBT-01` im Backlog erfasst.
- Damit ist Phase 2 (Interface-Segregation, Rule 4.2/4.3) des Go-Coding-Style-Guide-EPICs vollständig abgeschlossen.

## Sprint 34 — JWT-Alg-Confusion-Fix (SEC-01) + GOSTYLE-IF-01/02/05/06 ✅
2026-07-18 → [tasks/sprints/34-jwt-fix-gostyle-if.md](sprints/34-jwt-fix-gostyle-if.md)
- SEC-01: fehlender Signaturmethoden-Check an 3 von 5 `jwt.Parse*`-Stellen behoben (Alg-Confusion), Regressionstests mit gefälschtem `alg:none`-Token.
- GOSTYLE-IF-01/02/05/06: `SessionRecorder` als tote Abstraktion entfernt, `FleetGateway` tatsächlich interface-typisiert gemacht, `VehicleStore`/`safety.Publisher` um Bootstrap-Methoden verschlankt.

## Sprint 33 — Indoor-Kartenrendering (ADR-034) + Hexagonal-Pilot-Start (HEX-01..05) ✅
2026-07-18 → [tasks/sprints/33-indoor-kartenrendering-hexagonal-pilot.md](sprints/33-indoor-kartenrendering-hexagonal-pilot.md)
- ADR-034: `vehicle_status.position_x/y` ergänzt, `FleetIndoorMap.tsx` rendert SVG-Koordinatensystem (Befüllung durch Simulator bewusst ausgeklammert).
- HEX-01..05: Hexagonal-Pilot auf `fleet-service` — `FleetStore`-Repository-Port (19 Methoden) + `FakeFleetStore` für Tests.

## Sprint 32 — Klärungsrunde + Route-Historie (ADR-033), WebRTC-Fix, Meilenstein-Dokumente ✅
2026-07-18 → [tasks/sprints/32-route-historie-webrtc-fix.md](sprints/32-route-historie-webrtc-fix.md)
- ADR-033: `vehicle_position_history`-Tabelle, gedrosselter Schreibpfad, 30-Tage-Retention, `GET /fleet/vehicles/{id}/history`.
- WEBRTC-10: `actpass→active`-SDP-Workaround entfernt (inkompatibel mit aktuellem Chrome), Offer bleibt Standard-`actpass`.
- Meilenstein-1/2-Dokumente für den Auftraggeber (IBATOUR) konsolidiert.

## Sprint 31 — AP2-Vervollständigung + Fleet-/Observability-Nacharbeiten ✅
2026-07-18 → [tasks/sprints/31-ap2-vervollstaendigung.md](sprints/31-ap2-vervollstaendigung.md)
- AP2-04 Prioritätenmanagement in der Task-UI, TASKUI-03 Nacharbeiten.
- OBS-01 (Vehicle-Heartbeat, seit Sprint 14 offen) nachgeholt: AckBadge zeigt "zuletzt gesehen vor Xs".

## Sprint 30 — Restposten-Bereinigung (MV-Folge-Tasks, TASKUI-Nacharbeiten, Doku) ✅
2026-07-18 → [tasks/sprints/30-restposten-bereinigung.md](sprints/30-restposten-bereinigung.md)
- MV-09 (Live-State-Badge), MV-11 (Multi-Vehicle-Handover — bei Bearbeitung als bereits erledigt vorgefunden, Commit `f68346a`), MV-12 (`GET /state`-Folgeaufräumung) sowie TASKUI-Nachträge aus Sprint 24.

## Sprint 29 — `control-server` (hohes Risiko) + Abschlussverifikation ✅
2026-07-18 → [tasks/sprints/29-control-server-abschlussverifikation.md](sprints/29-control-server-abschlussverifikation.md)
- Dritter Go-Style-Guide-Rollout-Sprint: `main()` (683 Zeilen) im sicherheitskritischsten Service zerlegt (Rule 2.2), Abschlussverifikation über alle drei Rollout-Sprints (27–29).

## Sprint 28 — Risikoarme Services: Rule 2.2 + 2.3 ✅
2026-07-17 → [tasks/sprints/28-risikoarme-services-rule22-23.md](sprints/28-risikoarme-services-rule22-23.md)
- Zweiter Go-Style-Guide-Rollout-Sprint: `main()`-Funktionen >50 Zeilen und Funktionen >4 Parameter außerhalb von `control-server` zerlegt.

## Sprint 27 — Fundament: `pkg/db`, `pkg/env`, non-blocking Linter-Gate ✅
2026-07-17 → [tasks/sprints/27-fundament-pkg-db-env-linter.md](sprints/27-fundament-pkg-db-env-linter.md)
- Erster Go-Style-Guide-Rollout-Sprint: DB-Open+WaitForReady- und `envOr`-Dreifach-Duplikate in `pkg/db`/`pkg/env` gebündelt; `golangci-lint` als non-blocking Warn-Gate eingerichtet.

## Sprint 26 — Drift-Audit MD-Doku vs. Ist-Zustand + DRIFT-K1/K2/K3-Sicherheitsfixes ✅
2026-07-16 (Audit) / 2026-07-17 (Fixes) / 2026-07-20 (gemerged) → [tasks/sprints/26-drift-audit-md-doku-vs-ist-zustand.md](sprints/26-drift-audit-md-doku-vs-ist-zustand.md)
- Reine Bestandsaufnahme (kein Fix-Scope): 38 bestätigte Drifts zwischen `docs/vision.md`/34 ADRs/`CLAUDE.MD`/`CONTEXT.MD`/`requirements.md` und dem tatsächlichen Code-/Test-/Deployment-Zustand, konsolidiert in `docs/drift-audit-2026-07.md` (6 Kritisch, 25 Mittel, 7 Niedrig), als `DRIFT-*`-Tasks in `tasks/backlog.md` aufgenommen.
- Fast-Track (2026-07-16): 24 reine Doku-Korrekturen (DRIFT-M01..M17, N01..N07) — ADR-Update-Blöcke, Cross-Referenzen, zwei Code-Kommentar-Fixes.
- DRIFT-K1/K2/K3 (2026-07-17): drei CRITICAL/DEGRADED-Safety-Trigger aus ADR-009, die zuvor nur synthetisch in Unit-Tests existierten, bekamen einen echten Produktivpfad — neuer `AuthWatchdog` (Auth Invalidation), `TransitionOperator(OpNoOperator)` im WS-Disconnect-Handler (No Active Operator), `MEDIA_DEGRADED`-Schwellwerte + Recovery-Pfad (Media-DEGRADED-Wiring). Je Befund eigene Grill-Me-Session (Typ L).
- Branch (`feature/drift-k1-k3-safety-model`) wurde erst am 2026-07-20 gemerged, 33 Commits nach seinem Abzweigpunkt — `AuthWatchdog`/`vehiclecontext.Registry` mussten dabei auf die inzwischen (Sprint 28/29) verschlankte `audit.SafetyAuditWriter`-Schnittstelle umgestellt werden, die DRIFT-K1/K2-Anpassungen in die inzwischen (Sprint 29) extrahierten `handleSessionStart`/`handleSessionEnd`/`recoverFromSafeMode`/`handleWSDisconnect`-Methoden übertragen werden statt in die ursprünglichen (inzwischen aufgelösten) Inline-Handler.
- Verifiziert: `go build`/`go vet ./...` sauber, `go test ./... -race` grün für alle Pakete außer den erwartbaren `tests/integration`-Fehlschlägen (kein laufender Docker-Teststack).

## Sprint 25 — Audio-Benachrichtigungen ✅
2026-07-16 → [tasks/sprints/25-audio-benachrichtigungen.md](sprints/25-audio-benachrichtigungen.md)
- Audio-Benachrichtigung bei neuen Fleet-Alerts (`useFleetAlertSound`, Mute-Toggle) — schließt die in Sprint 22 unvollständig umgesetzte Notification-Anforderung.

## Sprint 24 — Task-Management-UI ✅
2026-07-16 → [tasks/sprints/24-task-management-ui.md](sprints/24-task-management-ui.md)
- Task-Erstellung/-Zuweisung, Status-Tracking, Priorität, Task-Historie im Fleet-Overview-Dashboard (`FleetTaskPanel.tsx`), aufbauend auf `fleet-service`.

## Sprint 23 — Outdoor Karten-/Zonen-Visualisierung im Fleet Overview ✅
2026-07-16 → [tasks/sprints/23-outdoor-karten-zonen-visualisierung.md](sprints/23-outdoor-karten-zonen-visualisierung.md)
- SVG-Karte mit Live-Fahrzeugpositionen (`FleetMap.tsx`, Leaflet `svgOverlay`), Scope per Grill-Me auf Outdoor-Zonen begrenzt (Indoor folgte in Sprint 33).

## Sprint 22 — Fleet Overview Dashboard ✅
2026-07-15 → [tasks/sprints/22-fleet-overview-dashboard.md](sprints/22-fleet-overview-dashboard.md)
- Erste Slice des Web-Dashboards (AP2): Fahrzeugliste, Status-/Detail-Panel, Alerts gegen das `fleet-service`-Backend — bewusst ohne Karte/Task-UI (folgten in Sprint 23/24).

## Sprint 21 — Fleet Backend Foundation (fleet-service, Multi-Vehicle-Simulation) ✅
2026-07-14 (Branches gemergt 2026-07-15) → [tasks/sprints/21-fleet-backend-foundation.md](sprints/21-fleet-backend-foundation.md)
- `fleet-service` real aufgesetzt (ADR-027/028/029): Zonen/Stationen/Tasks/Alerts-Endpoints, `FleetGateway`, Multi-Vehicle-Simulator — Fundament für das gesamte AP2-Dashboard.

## Sprint 20 — Bugfix: Session-Neustart nach Session-Ende blockiert ✅
2026-07-10 → [tasks/sprints/20-session-neustart-bugfix.md](sprints/20-session-neustart-bugfix.md)
- Nach Session-Ende mit verbundenem Fahrzeug ließ sich keine neue Session starten (`VehicleSelector` erschien nicht wieder) — behoben.

## Sprint 19 — Lokaler Dev-Stack: Verifikation & Robustheit ✅
2026-07-10 (1 Folge-Task an Backlog übergeben) → [tasks/sprints/19-dev-stack-verifikation.md](sprints/19-dev-stack-verifikation.md)
- Reproduzierbarkeit auf frischem Checkout sichergestellt, SSL/HTTPS-Dev-Frage geklärt, dokumentierte Stolpersteine.

## Sprint 18 — Cleanup & ADR-Vorbereitung
2026-06-18 → [tasks/sprints/18-cleanup-adr-vorbereitung.md](sprints/18-cleanup-adr-vorbereitung.md)
- CI-Blocker (`go vet`-Fehler durch veraltete `handler_test.go`) behoben, Multi-Vehicle-Handover architektonisch vorbereitet (Grill-Me + ADR, kein Code).

## Sprint 17 — Multi-Vehicle State Isolation (ADR-026) ✅
2026-06-14, deployed 2026-06-16 (Commits `32de463`, `d20e9f2`) → [tasks/sprints/17-multi-vehicle-state-isolation.md](sprints/17-multi-vehicle-state-isolation.md)
- State Machine, DeadmanWatchdog, VehicleACKWatchdog von Prozess-Singletons auf `vehiclecontext.Registry` (pro Fahrzeug) umgestellt — zwei Operatoren können zwei Fahrzeuge unabhängig steuern; `SafetyBusWatchdog` bleibt global, fächert aber korrekt auf.

## Sprint 16 — Safety Hardening (ADR-009 Lücken geschlossen) ✅
2026-06-14, Commit `844a6ef` → [tasks/sprints/16-safety-hardening.md](sprints/16-safety-hardening.md)
- `VehicleACKWatchdog` + `SafetyBusWatchdog` (2 fehlende ADR-009-CRITICAL-Trigger) implementiert; 3 Session-Lifecycle-Bugfixes (WS-Disconnect-Race, CONNECTED→IDLE-Transition, Page-Reload-Recovery); 18 neue Unit-Tests.

## Sprint 15 — PostgreSQL-Migration + Nutzerverwaltung ✅
2026-06-14 → [tasks/sprints/15-postgresql-migration-nutzerverwaltung.md](sprints/15-postgresql-migration-nutzerverwaltung.md)
- ADR-023: SQLite vollständig durch PostgreSQL ersetzt. ADR-024: echte Authentifizierung mit bcrypt, ADMIN-Rolle, `LoginPanel`/`UserManagementPanel`.

## Sprint 14 — Security & Observability ✅ (OBS-01 nachgeholt Sprint 31)
2026-06-13 → [tasks/sprints/14-security-observability.md](sprints/14-security-observability.md)
- JWT-Pflicht auf REST-Endpoints (`requireJWT`), Dual-Channel-Latenzanzeige (Control/Video getrennt), Backend-nicht-erreichbar-Banner im Frontend.

## Sprint 13 — Dev-Stack Stabilisierung & Log-Korrelation ✅
2026-06-13 → [tasks/sprints/13-dev-stack-stabilisierung.md](sprints/13-dev-stack-stabilisierung.md)
- HTTP-only Dev-nginx (SSL-Fehler bei `make up` behoben), `vehicle-mock` in `build-prod`/`push` ergänzt, `session_id`-Propagation durch alle Kanäle (vollständige Log-Korrelation).

## Sprint 12 — Vehicle Registry (ADR-022) ✅
2026-06-12 → [tasks/sprints/12-vehicle-registry.md](sprints/12-vehicle-registry.md)
- `VehicleStore`-Interface + SQLite-Implementierung (später Postgres, ADR-023), `GET/POST/DELETE /vehicles`, `VehicleSelector.tsx` — Hardcoded-Vehicle-ID vollständig entfernt.

## Sprint 11 — Vehicle Connectivity & Feedback (ADR-021) ✅
2026-06-11 → [tasks/sprints/11-vehicle-connectivity-feedback.md](sprints/11-vehicle-connectivity-feedback.md)
- `VehicleCommandAck` (WS) + Actuation-Feedback-Felder (MQTT), `vehicleconnection.Registry`/`AckStore`, `vehicle-mock` Docker-Service, `InputIndicatorPanel.tsx`.

## Sprint 10 — Browser WebRTC ICE Migration ✅
2026-06-10 → [tasks/sprints/10-webrtc-ice-migration.md](sprints/10-webrtc-ice-migration.md)
- coturn als eigenständiger STUN/TURN-Service (`network_mode: host`), `GET /ice-config`-Endpoint, DTLS-Fix in `useWebRTC.ts`, Deploy auf EC2.

## Sprint 9 — WebRTC Videostream: Larix WHIP → MediaMTX → Browser ✅
2026-06-05 → [tasks/sprints/09-webrtc-videostream-larix-mediamtx.md](sprints/09-webrtc-videostream-larix-mediamtx.md)
- ADR-020: MediaMTX als WHIP/WHEP-Router löst das Custom-SFU-Signaling ab; Control Server als einzige Auth-Instanz (`/internal/media/auth`) + SAFE_MODE-Kick.

## Sprint 8 — EC2 Deployment via Docker Hub ✅
2026-06-04 → [tasks/sprints/08-ec2-deployment-docker-hub.md](sprints/08-ec2-deployment-docker-hub.md)
- ADR-019: Docker-Hub-private-Repos + EC2 Elastic IP + SSM Parameter Store. `make build-prod`/`push`, `docker-compose.prod.yml`, `deploy.sh`, EC2-Bootstrap-Guide.

## Sprint 7 — Logging & Audit Trail ✅
2026-06-04 → [tasks/sprints/07-logging-audit-trail.md](sprints/07-logging-audit-trail.md)
- `pkg/logger` (strukturiertes slog, ADR-017) über alle Services ausgerollt; `pkg/audit` (`AuditWriter`, ursprünglich SQLite-WAL — seit ADR-023 PostgreSQL); Loki + Grafana + Promtail.

## Sprint 6 — Testing & Quality Gates ✅
2026-06-04 → [tasks/sprints/06-testing-quality-gates.md](sprints/06-testing-quality-gates.md)
- Integration-Test-Stack (`docker-compose.test.yml`), Vitest + RTL + Playwright, Performance/Latency-Tests (Go Benchmark + k6), README-Contributor-Guide.

## Sprint 5 — Feature Completion Frontend ✅
2026-06-03 → [tasks/sprints/05-feature-completion-frontend.md](sprints/05-feature-completion-frontend.md)
- Control Panel (Keyboard/Joystick/Gamepad, 20 Hz Command Loop), Video Panel (WebRTC RTCPeerConnection), Dashboard-Integration mit Telemetrie.

## Sprint 4 — Core Backend Services ✅
2026-06-03 → [tasks/sprints/04-core-backend-services.md](sprints/04-core-backend-services.md)
- Command Engine (Rate Limiting, Protobuf-Routing), MQTT Telemetry Service, Session Recording (abstraktes Interface + `MemoryRecorder`), WebRTC SFU (Pion).

## Sprint 3 — Frontend Core ✅
2026-06-03 → [tasks/sprints/03-frontend-core.md](sprints/03-frontend-core.md)
- Protobuf-Frontend-Adapter + Build-Pipeline, WebSocket-Client mit State-Polling, SAFE-MODE-Overlay + Operator-Ack-Flow, Emergency-Stop + Dead-man-Switch.

## Sprint 2 — Safety & Failure Model ✅
2026-06-03 → [tasks/sprints/02-safety-failure-model.md](sprints/02-safety-failure-model.md)
- Vehicle Connection Service, Session Manager (GSA), Failure Detection (DeadmanWatchdog + ACKTimeoutWatcher), Operator Handover — erste vollständige Safety Test Suite (19/19).

## Sprint 1 — Foundation Layer ✅
2026-06-03 → [tasks/sprints/01-foundation-layer.md](sprints/01-foundation-layer.md)
- Proto-Schema-Repository, React-Projekt-Setup, Auth-Service (JWT), coturn, Safety Event Bus (In-Memory), Control-Server-WebSocket+JWT, Dockerfiles, Docker-Compose-Orchestrierung.
