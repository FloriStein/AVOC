> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 43 — Hexagonale Architektur-Migration Schritt 3: telemetry-service (ADR-031)

Strategie/Priorisierung: [ADR-031](../docs/adr/031-hexagonal-architecture-migration.md), Update
"Sprint-43-Kickoff". Fortsetzung nach Pilot (fleet-service, ✅ Sprint 33) und Schritt 2
(auth-service, ✅ Sprint 37) — Nutzer gibt Fortsetzung mit `telemetry-service` (Schritt 3 der
ADR-031-Priorisierung) am 2026-07-19 frei, nachdem dieser Entscheidungspunkt zuvor mehrfach
zurückgestellt wurde.

**Vorrecherche (2026-07-19):**
- `internal/telemetryservice/client.go` (114 Zeilen, gesamter Service): `Client.client` (Zeile 29)
  ist vom Typ `mqtt.Client` — das ist bereits ein Interface, aber eines aus der Drittanbieter-
  Bibliothek `paho.mqtt.golang` (14 Methoden, u.a. `Publish`/`AddRoute`/`OptionsReader`, die
  telemetryservice nie nutzt). Kein projekteigener, schmaler Port — Verstoß gegen GOSTYLE Rule 2.2
  (Interface-Segregation) und ADR-031-Regel 3 (Driven Ports vom Consumer definiert, nicht vom
  Adapter/Anbieter). `Connect()` (Zeile 44-64) baut `mqtt.NewClientOptions()...` und ruft
  `c.client.Connect()`/`token.Wait()`/`token.Error()` direkt auf; `subscribe()` (Zeile 66-74)
  ebenso mit `mqtt.Token`.
- **`internal/telemetryservice` und `cmd/telemetry-service` haben aktuell 0 Tests** (`find
  internal/telemetryservice cmd/telemetry-service -name "*_test.go"` liefert nichts) — derselbe
  Bestandsaufnahme-Befund wie bei `safety-service`/`webrtc-sfu`/`internal/recording` vor Sprint 39.
  Grund: `Connect()`/`subscribe()` sind ohne echten MQTT-Broker nicht sinnvoll testbar, solange sie
  direkt gegen `mqtt.Client` programmieren. Ein projekteigener `MQTTConnection`-Port löst dieses
  Problem als direkten Nebeneffekt der Migration (kein separater Testabdeckungs-Sprint nötig).
- `handleMessage`/`GetLatest` (Zeile 76-107) sind bereits reine Domain-Logik (Proto-Parse,
  Map-Zugriff hinter `sync.RWMutex`) ohne I/O — laut ADR-031-Bestandsaufnahme "triviales reines
  Passthrough". Keine Use-Case-Extraktion nötig (anders als `HEX-06`/`HEXAUTH-04`, die für
  fleet-/auth-service optional zur Debatte stehen).
- `cmd/telemetry-service/main.go:29` (`telemetryservice.NewClient(broker, username, password)`) —
  externe Konstruktorsignatur bleibt unverändert, analog zum Präzedenzfall `HEX-02`/`HEXAUTH-02`
  (`*PostgresFleetStore` bzw. `JWTTokenIssuer` wurden beide intern konstruiert, ohne den Aufrufer
  anzupassen).
- `cmd/telemetry-service/main.go:38-79` (`newTelemetryMux`) — analoges Muster zu den in Sprint 39
  getesteten `newSafetyMux`/`newSFUMux`, aber bisher ungetestet.

Datum: 2026-07-19 | **Status: Abgeschlossen ✅**
Vorgänger: Sprint 37 ✅ (Hexagonal-Migration Schritt 2, auth-service) — Sprint 38-42 laufen
parallel in eigenen Worktrees (andere Services/Themen, keine Code-Überschneidung).
Branch/Worktree: `feature/fleet-service-foundation-hextele` (Basis:
`feature/fleet-service-foundation-sprint35`).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEXTELE-01 | `MQTTConnection`-Port definieren (neue Datei `internal/telemetryservice/mqttconnection.go`): schmales Interface (`Connect() error`, `Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error`, `Disconnect(quiesceMs uint)`, `IsConnected() bool`) statt direkter Kopplung an `paho.mqtt.golang`s `mqtt.Client`. `PahoConnection`-Adapter kapselt `mqtt.Token`/`mqtt.Message`-Handling intern. Compile-Time-Check `var _ MQTTConnection = (*PahoConnection)(nil)`. | S | ✅ | — |
| HEXTELE-02 | `Client.client` (`internal/telemetryservice/client.go:29`) von `mqtt.Client` auf `MQTTConnection`-Port umgestellt. `Connect()`/`subscribe()`/`Disconnect()` rufen den Port statt paho direkt auf. `NewClient`-Signatur unverändert, Adapter wird intern konstruiert (`cmd/telemetry-service/main.go:29` unangetastet, analog `HEX-02`/`HEXAUTH-02`). | S | ✅ | HEXTELE-01 |
| HEXTELE-03 | Erste Testabdeckung `internal/telemetryservice` + `cmd/telemetry-service` (aktuell 0 Tests): `FakeMQTTConnection`-Testdoppel; Tests für `handleMessage` (Proto-Parse, fehlender/leerer `vehicle_id`, Malformed Payload), `GetLatest`, `Connect`/`subscribe`-Wiring über den Fake (kein echter Broker nötig); `main_test.go` für `newTelemetryMux` via `httptest` (analog Sprint-39-Muster `main_test.go` für `safety-service`/`webrtc-sfu`). | M | ✅ | HEXTELE-02 |
| HEXTELE-04 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./internal/telemetryservice/... ./cmd/telemetry-service/... -race` (mind. 2x gegen Flakiness, CLAUDE.MD §17). ADR-031-Status-Update (Schritt 3 abgeschlossen, analog Sprint-37/HEXAUTH-03-Eintrag), `DECISIONS.MD`, `tasks/backlog.md`-Status-Update. | S | ✅ | HEXTELE-03 |

**Nicht Teil dieses Sprints:** Use-Case-Extraktion (kein Bedarf, siehe Vorrecherche —
`handleMessage`/`GetLatest` sind bereits reine Funktionen), `control-server` (Schritt 4/5 der
ADR-031-Priorisierung, ausdrücklich zuletzt/eigenes ADR nötig), `safety-service`/`webrtc-sfu`/
`recording` (Hexagonal-Migration dort, unabhängig von der bereits vorhandenen Sprint-39-
Testabdeckung, ein separater Entscheidungspunkt).

## Ergebnisse

- **HEXTELE-01**: Neuer `MQTTConnection`-Port (`internal/telemetryservice/mqttconnection.go`) —
  4-Methoden-Interface (`Connect() error`, `Subscribe(topic string, qos byte, handler func(topic
  string, payload []byte)) error`, `Disconnect(quiesceMs uint)`, `IsConnected() bool`) statt
  `paho.mqtt.golang`s 14-Methoden-`mqtt.Client`. `PahoConnection`-Adapter kapselt `mqtt.Token`/
  `mqtt.Message` intern — die `Subscribe`-Callback-Signatur wandelt `mqtt.Message` bereits im
  Adapter in `(topic string, payload []byte)` um, die Domain-Seite sieht kein paho-Symbol mehr.
  Konstruktorparameter (`Broker`/`Username`/`Password`/`ClientID`) in `PahoConnectionConfig`
  gebündelt (GOSTYLE Rule 2.3 — sonst 6 Parameter inkl. der beiden Callbacks). Compile-Time-Check
  `var _ MQTTConnection = (*PahoConnection)(nil)`.
- **HEXTELE-02**: `Client.client` (`internal/telemetryservice/client.go:28`) von `mqtt.Client` auf
  `MQTTConnection` umgestellt. `Connect()` konstruiert `PahoConnection` intern und ruft nur noch
  `c.client.Connect()` auf; `subscribe()`/`Disconnect()` ebenso rein über den Port.
  `handleMessage` nimmt jetzt `(topic string, payload []byte)` statt `(mqtt.Client, mqtt.Message)`
  — bleibt dieselbe reine Logik, nur ohne paho-Abhängigkeit in der Signatur. `client.go` importiert
  `github.com/eclipse/paho.mqtt.golang` danach nicht mehr. `NewClient(broker, username, password
  string)` unverändert, `cmd/telemetry-service/main.go:29` nicht angefasst (wie geplant).
- **HEXTELE-03**: Erste Testabdeckung, zuvor 0 Tests in beiden Paketen.
  - `internal/telemetryservice/fake_mqttconnection.go`: `FakeMQTTConnection`-Testdoppel
    (`MQTTConnection`-Interface), inkl. `Deliver(topic, payload)` zur Simulation eingehender
    Broker-Nachrichten an den zuletzt via `Subscribe` registrierten Handler.
  - `internal/telemetryservice/client_test.go` (13 Tests): `handleMessage` — valides Payload
    (Latest wird gesetzt), fehlender `Header` (ignoriert), leerer `vehicle_id` (ignoriert),
    Malformed Payload (Proto-Parse-Fehler, kein Panic), leeres Payload, Überschreiben eines
    bestehenden Eintrags. `GetLatest` für unbekanntes Fahrzeug. `subscribe()`-Wiring über den
    Fake (korrektes Topic/QoS, zugestellte Nachricht landet über den registrierten Handler in
    `latest`) sowie Subscribe-Fehlerpfad (Port liefert Fehler → geloggt, kein Panic). `Disconnect()`
    verbunden/nicht verbunden/`client == nil` (Grenzwerte). Ein Nebenläufigkeitstest
    (`-race`, mehrere Goroutinen rufen `handleMessage`/`GetLatest` gleichzeitig für überlappende
    Vehicle-IDs auf) gegen die bestehende `sync.RWMutex`.
  - `cmd/telemetry-service/main_test.go` (3 Tests, analog Sprint-39-Muster): `newTelemetryMux` via
    `httptest` — `GET /telemetry/latest/` ohne `vehicle_id` (400), unbekanntes Fahrzeug (404),
    `GET /health` (200 + korrektes JSON).
  - **Bewusst nicht abgedeckt:** der 200-OK-Erfolgsfall von `GET /telemetry/latest/{id}` auf
    Mux-Ebene (`cmd/telemetry-service`) — anders als bei `safety-service`s `Bus.TriggerEmergencyStop`
    (Sprint 39) bietet `telemetryservice.Client` keine öffentliche Methode, um `latest` von außerhalb
    des Pakets zu befüllen (Daten kommen ausschließlich über den privaten `handleMessage`-Callback,
    real nur über einen echten Broker erreichbar). Einen Test-only-Setter für `Client` einzuführen,
    nur um diesen einen Pfad zu testen, wäre Scope-Creep über HEXTELE-01..04 hinaus. Der Happy Path
    der zugrundeliegenden Logik (`handleMessage` → `GetLatest`) ist auf `internal/telemetryservice`-
    Ebene bereits vollständig abgedeckt (`TestClient_HandleMessage_ValidPayload_StoresLatest`); die
    JSON-Encoding-Zeile in `newTelemetryMux` selbst ist einfaches, risikoarmes `map[string]any`-
    Encoding ohne eigene Logik.
- **HEXTELE-04**: `go build ./...` und `go vet ./...` (gesamtes Repo) sauber. `go test
  ./internal/telemetryservice/... ./cmd/telemetry-service/... -race -count=1` zweimal hintereinander
  grün (16 Tests: 13 in `internal/telemetryservice`, 3 in `cmd/telemetry-service`), keine Flakiness.
  ADR-031 (`docs/adr/031-hexagonal-architecture-migration.md`), `DECISIONS.MD` (Zeilen 38/79) und
  `tasks/backlog.md` (EPIC-Header + `HEXTELE-01..04`-Tabelle) auf ✅ aktualisiert.

**Ergebnis:** 4/4 Tasks abgeschlossen, neuer `MQTTConnection`-Port + `PahoConnection`-Adapter,
`Client` importiert `paho.mqtt.golang` nicht mehr direkt, 16 neue Unit-Tests über 2 Testdateien
(zuvor 0 Tests in beiden Paketen), keine Änderung der externen `NewClient`-Signatur.
