> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 50 — TelemetryWatchdog (DRIFT-K3-TELEMETRY Teil 2)

**Freigabe (2026-07-21):** Nutzer wählt Sprint 50 (TelemetryWatchdog) vor Sprint 51
(Testabdeckungs-Gesamtaudit Teil 1) — siehe Kickoff-Entscheidung, vorher an dieser Stelle
dokumentiert. Typ L (Kernsystem State Machine/Sicherheitsmodell, CLAUDE.MD §1.1/§5). Baut auf
Sprint 47 (`tasks/sprints/47-multi-cause-degraded-fundament.md`, Multi-Cause-DEGRADED-Fundament)
auf. Architekturskizze bereits in [docs/adr/009-failure-model.md](../docs/adr/009-failure-model.md)
Update 2026-07-20 vorbereitet:

1. **Poll-basierter Watchdog in `control-server`** (nicht: `telemetry-service` meldet sich aktiv) —
   analog `SafetyBusWatchdog`/`AuthWatchdog`. `telemetry-service` bleibt vollständig unwissend über
   Sessions/Fahrzeuge.
2. Neues Paket `internal/controlserver/telemetrycheck`, `TelemetryWatchdog` mit Lifecycle
   `Start(sessionID, vehicleID)`/`Stop()`, per-`VehicleContext` (wie `AuthWatchdog`, nicht global
   wie `SafetyBusWatchdog` — ADR-026).
3. Pollt `telemetry-service` per HTTP (neue `TELEMETRY_SERVICE_URL`-Env-Var).
4. Feuert über `enterDegraded(DegradedReasonTelemetry)`/`exitDegraded(...)` aus Sprint 47 — nicht
   über einen zweiten `transitionSystemLocked`-Aufruf.

## Grill-Me-Session (2026-07-21) — Ergebnisse

Vier offene Schwellwert-Fragen vorab per `AskUserQuestion` geklärt (CLAUDE.MD §1.1, vor jeder
Codezeile):

| Frage | Entscheidung | Begründung |
|---|---|---|
| Poll-Interval | **2s** | Schnellere Erkennung als `AuthWatchdog`s 5s — Telemetrie ist die Hauptquelle für Situationsbewusstsein des Operators, abweichend von der 5s-Standardvorlage bewusst gewählt. |
| Fail-Threshold | **2** consecutive | Gleiches Muster wie `SafetyBusWatchdog`/`AuthWatchdog` (2 aufeinanderfolgende Fehlschläge) — mit 2s-Intervall macht das ein Budget von **4s** bis DEGRADED. |
| "Noch nie/nicht aktuell empfangen" | **Timestamp-Alter zählt** | Vorrecherche-Fund: `GET /telemetry/latest/{vehicleID}` liefert den letzten je empfangenen Wert über Session-Grenzen hinweg (in-memory `map[string]*TelemetryEvent`, nicht session-gebunden, siehe `internal/telemetryservice/client.go:103`) — ein HTTP 200 mit veraltetem `timestamp` von einer früheren Session könnte einen echten Ausfall in der aktuellen Session verdecken. Beschluss: sowohl HTTP 404 (nie empfangen) als auch HTTP 200 mit `timestamp`-Alter > `maxAge` zählen gleichwertig als ein Fehlschlag Richtung Threshold. `maxAge := interval × threshold` (= 4s) — konsistent mit dem Fehlschlag-Budget oben, kein zweiter unabhängiger Schwellwert. |
| Timeout/HTTP-Fehler | **3s Timeout, analog `SafetyBusWatchdog`** | `http.Client{Timeout: 3 * time.Second}`. Jeder Netzwerkfehler/Timeout/unerwarteter Status (≠404) zählt gleich wie ein Freshness-Fehlschlag — keine Sonderbehandlung, gleiches Prinzip wie oben (404 vs. Stale-200 vs. Netzwerkfehler sind für den Watchdog alle nur "keine frischen Daten diesen Poll"). |

## Vorrecherche (2026-07-21, vor Task-Planung durchgeführt)

- `internal/controlserver/safety/auth_watchdog.go` und `bus_watchdog.go` sind die direkten Vorbilder
  (Lifecycle-Muster, Fehlschlag-Zählung, Publisher/AuditWriter-Verdrahtung, Log-Events). `TelemetryWatchdog`
  übernimmt dieselbe Grundstruktur, landet aber laut ADR-009-Skizze im **eigenen** Paket
  `internal/controlserver/telemetrycheck` (nicht `safety`) — dort auch der HTTP-Checker.
- `cmd/telemetry-service/main.go:45-72`: `GET /telemetry/latest/{vehicleID}` existiert bereits
  produktiv (BE-05), liefert `404` bei `client.GetLatest()`-Miss, sonst JSON inkl.
  `"timestamp"` (int64 Unix-ms, aus `proto/common.proto:14` `Header.timestamp`). Kein Code-Änderung
  am `telemetry-service` selbst nötig — der Watchdog konsumiert nur den bestehenden Endpoint.
- `internal/controlserver/statemachine/state.go`: `enterDegraded`/`exitDegraded` (Sprint 47) sind
  **private** Methoden (Caller muss `m.mu` bereits halten). `TelemetryWatchdog` liegt in einem
  anderen Package und braucht daher eine neue **exportierte** Methode `TransitionTelemetry(healthy bool)`
  — analog `TransitionMedia`, aber ohne Sub-State (kein `MediaState`-Äquivalent für Telemetrie nötig,
  nur healthy/unhealthy). Rein additiv: `enterDegraded`/`exitDegraded`-Signaturen bleiben unverändert,
  bestehende Sprint-47-Logik unberührt (Vorgabe eingehalten).
- `internal/controlserver/vehiclecontext/registry.go`: `AuthWatchdog` ist **optional** (nil bis
  `WithUserChecker` gesetzt), weil es von einer DB-Verbindung abhängt, die nicht jeder Testkontext
  hat. `TelemetryWatchdog` hat dieses Problem nicht — `TELEMETRY_SERVICE_URL` ist eine reguläre
  konfigurierte URL wie `SAFETY_SERVICE_URL`/`SFU_SERVICE_URL`/`AUTH_SERVICE_URL`
  (`cmd/control-server/main.go:65-67`, `env.OptionalOr`). `TelemetryWatchdog` wird deshalb analog
  `Deadman`/`VehicleACKWatchdog` **immer** erzeugt (kein Nil-Check an den Call-Sites nötig).
- Wiring-Stellen identisch zu `AuthWatchdog`s drei Call-Sites in `cmd/control-server/main.go`:
  `advanceVehicleToActiveOperator` (Start), `handleSessionEnd` — sowohl der `session_id`-Pfad als
  auch der Legacy-Fleet-wide-Pfad (beide Stop()).
- `csafety.Publisher` (`internal/controlserver/safety/publisher.go`) kann direkt importiert werden
  — anders als `VehicleStates`/`SessionSource` in `bus_watchdog.go`, die bewusst lokal redefiniert
  sind um einen Zyklus mit `vehiclecontext` zu vermeiden. Für `telemetrycheck` besteht dieses
  Zyklusrisiko nicht (`vehiclecontext` importiert `telemetrycheck`, nicht umgekehrt), daher direkter
  Import von `csafety.Publisher`/`audit.SafetyAuditWriter` ohne Duplikat (Rule 4.1).
- SAFE_MODE-Verhalten (CLAUDE.MD §0/ADR-009 Invariante 1): der Watchdog läuft während SAFE_MODE
  unverändert weiter (Start/Stop ist an die Session gebunden, nicht an SYSTEM STATE) — seine Aufrufe
  `sm.TransitionTelemetry(...)` werden aber während SAFE_MODE automatisch zu No-Ops, weil die Guards
  in `TransitionTelemetry` (analog `TransitionMedia`) nur bei `StateConnected`/`StateDegraded`
  greifen. Kein Sonderfall im Watchdog-Code nötig — reine Konsequenz der bestehenden Guards. Deckt
  den in der Aufgabenstellung geforderten Testfall ab ("was passiert, wenn `telemetry-service`
  während einer aktiven SAFE_MODE-Phase weiter gepollt wird").

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| TW-01 | ADR-009 Update-Block (Abschnitt "Update 2026-07-20" ergänzen bzw. neuen "Update 2026-07-21"-Block anfügen): dokumentiert die vier Grill-Me-Entscheidungen (Tabelle oben) inkl. Begründung für den Cross-Session-Staleness-Fund, bevor Code entsteht. | S | ✅ | — |
| TW-02 | `internal/controlserver/statemachine/state.go`: neue exportierte Methode `TransitionTelemetry(healthy bool)` — Guards analog `TransitionMedia`, nutzt bestehende `enterDegraded`/`exitDegraded` unverändert. Erweiterung von `state_test.go` (Sprint 47): Telemetrie als zweiter, unabhängiger Grund neben Media. **Ungeplanter Fund während der Umsetzung (mit Nutzer abgestimmt, siehe unten):** `TransitionMedia`s/`TransitionTelemetry`s äußerer Guard prüfte bislang nur `System == StateConnected` vor dem Aufruf von `enterDegraded` — ein zweiter, unabhängiger Grund, der eintrifft *während* SYSTEM bereits wegen des ersten Grundes DEGRADED ist, wurde dadurch nie ins `degradedReasons`-Set aufgenommen; die spätere Recovery des ERSTEN Grundes hätte DEGRADED dann fälschlich komplett aufgehoben, obwohl der zweite Grund noch aktiv war. Guard auf `System == StateConnected \|\| System == StateDegraded` erweitert (beide Funktionen), `enterDegraded` selbst brauchte keine Änderung. Regressionstest `TestSecondCause_ArrivingAfterFirstCauseAlreadyDegraded`. | S | ✅ | — |
| TW-03 | Neues Paket `internal/controlserver/telemetrycheck/checker.go`: `Checker.HasFreshTelemetry(ctx, vehicleID, maxAge) (bool, error)` — GET `{baseURL}/telemetry/latest/{vehicleID}`, 404→`(false,nil)`, Netzwerkfehler/unerwarteter Status→`(false,err)`, 200→`timestamp`-Alter gegen `maxAge` geprüft. 7 Unit-Tests mit `httptest.Server` (fresh/stale/404/500/malformed JSON/unreachable/Pfad-Escaping). | M | ✅ | — |
| TW-04 | `internal/controlserver/telemetrycheck/watchdog.go`: `TelemetryWatchdog` mit Lifecycle `Start(sessionID, vehicleID)`/`Stop()`, Fehlschlag-Zählung, feuert `sm.TransitionTelemetry(...)`; Loop läuft nach Trigger weiter (reversibel, anders als `AuthWatchdog`). **Design-Abweichung vom ursprünglichen Taskzuschnitt (bewusst, kein Rückfrage-Bedarf):** kein `csafety.Publisher`/`audit.SafetyAuditWriter` — Vorrecherche ergab, dass DEGRADED-only-Ursachen (bislang nur Media) in diesem Code weder auf den Safety Event Bus noch in den Audit-Trail geschrieben werden (`cmd/control-server/main.go`s `MediaDegraded`/`MediaFailed`-Aufrufe tun das auch nicht) — dieser Kanal ist laut bestehendem Muster CRITICAL/SAFE_MODE-Events vorbehalten. `TransitionTelemetry`s eigenes strukturiertes Logging (TW-02) reicht, analog `TransitionMedia`. Neuer Log-Event-Typ `EventTelemetryWatchdogTriggered`. 9 Unit-Tests in `tests/unit/watchdog_test.go` (Fresh/Stale/Fehler/Recovery/Stop/SAFE_MODE-No-Op/Restart/Race). | M | ✅ | TW-02, TW-03 |
| TW-05 | Wiring in `vehiclecontext.Registry` + `cmd/control-server/main.go`. **Design-Abweichung vom ursprünglichen Taskzuschnitt:** statt "immer gesetzt, kein Nil-Check" (ursprüngliche Annahme) folgt `TelemetryWatchdog` demselben optionalen `With*`-Builder-Muster wie `AuthWatchdog`/`WithUserChecker` (`WithTelemetryChecker`, Nil-Check an den drei Call-Sites) — vermeidet eine Signaturänderung von `vehiclecontext.NewRegistry`, die 6 bestehende Testaufrufstellen betroffen hätte; in Produktion (`main.go`) ist der Checker trotzdem immer gesetzt, Verhalten dadurch identisch zum ursprünglichen Plan. Neue `TELEMETRY_SERVICE_URL`-Env-Var (Default `http://telemetry-service:8083`). | S | ✅ | TW-04 |
| TW-06 | Integration-Test `tests/integration/telemetry_watchdog_test.go`: `TestIntegration_TelemetryLoss_TriggersDegrade_ThenRecovers`, echter Docker-Teststack. Publiziert direkt per MQTT (eigener Test-Client, `pkg/mqtttls` + `paho`) statt über `vehicle-mock`s Dauerschleife — volle Kontrolle über Timing ("nie empfangen" vs. "einmal frisch empfangen"). **Zusätzlich nötig (nicht im ursprünglichen Taskzuschnitt explizit genannt, aber Voraussetzung):** `telemetry-service` fehlte komplett in `tests/docker-compose.test.yml` — neuer Service-Block ergänzt (Port 18083, MQTT-TLS-Setup analog `fleet-service`), `control-server`s `TELEMETRY_SERVICE_URL`+`depends_on` ergänzt. Gegen echten Stack verifiziert: alle 33 Integrationstests grün (`make test-integration`), keine Regression. | S | ✅ | TW-05 |
| TW-07 | Verifikation: `go build ./...`, `go vet ./...`, `gofmt -l .` sauber, `go test ./internal/... ./tests/unit/... ./pkg/... -race -count=2` grün, vollständiger `make test-integration`-Lauf grün (33/33). DoD (CLAUDE.MD §11): `DECISIONS.MD`, `tasks/backlog.md`, Sprint-Datei-Verschiebung, `tasks/done.md`-Eintrag. | S | ✅ | TW-01..06 |

**Nicht Teil dieses Sprints:** Änderungen an `telemetry-service` selbst über den neuen Compose-Service-Block hinaus; Persistenz/Historie von Telemetrie-Ausfällen über die bestehende Audit-Kette hinaus (ohnehin nicht genutzt, siehe TW-04); Frontend-Anzeige eines eigenen "Telemetrie DEGRADED"-Badges (bestehendes DEGRADED-Banner deckt es generisch ab); Sprint 51 (Testabdeckungs-Gesamtaudit).

**Tatsächlicher Umfang:** 7 Tasks (wie geplant), S/M, ein neues Paket (`telemetrycheck`) + ein
Bugfix in bestehendem Sprint-47-Code (mit Nutzer abgestimmt) + Erweiterung des Integrations-Teststacks
um `telemetry-service` — innerhalb des ~200k-Token-Sprintbudgets (siehe
`feedback_sprint_token_budget`-Memory).

---

Vorgänger: Sprint 47 ✅ (Multi-Cause-DEGRADED-Fundament, DRIFT-K3-TELEMETRY Teil 1), siehe
`tasks/sprints/47-multi-cause-degraded-fundament.md`.
