## Sprint 45 — Hexagonale Architektur: Use-Case-Extraktion fleet-service/auth-service (HEX-06/HEXAUTH-04)

**Freigabe (2026-07-20):** Nutzerentscheidung gegenüber zwei Alternativen (Hexagonal-Migration
safety-service/webrtc-sfu/recording; Vorbereitung control-server-Migration per neuem ADR +
Testaufbau). Begründung: HEX-06/HEXAUTH-04 sind bereits seit Sprint 33/37 als optionale
Entscheidungspunkte im Backlog vorbereitet, brauchen kein neues ADR (ADR-031 deckt die Strategie
ab) und sind klein genug für einen kompakten Sprint. Die beiden Alternativen bleiben offene
Entscheidungspunkte für einen Folge-Sprint.

**Scope-Vorrecherche (2026-07-20):** `internal/fleetservice/handler.go` (355 Zeilen) und
`internal/authservice/handler.go` (319 Zeilen) gelesen. Beide Handler folgen bereits konsequent
dem Muster decode → validate → store/tokens-Aufruf → encode — die meisten Endpunkte (`ListVehicles`,
`ListZones`, `ListStations`, `ListTasks`, `ListAlerts`, `GetTaskStatusHistory`,
`GetVehiclePositionHistory`, `VehicleRegister`, `ListUsers`, `CreateUser`) sind reine
1:1-Pass-Throughs ohne eigene Entscheidungslogik — für die lohnt sich keine Use-Case-Schicht
(würde nur Indirektion ohne Testbarkeits- oder Klarheitsgewinn erzeugen, Rule of Three /
CLAUDE.MD-Prinzip "keine Abstraktion ohne Konsument"). Der Sprint extrahiert deshalb **nur** die
Endpunkte mit echter Entscheidungslogik in reine, HTTP-unabhängige Funktionen:

- fleet-service: `CreateTask` (Persistenz + Fire-and-forget-Dispatch, ADR-027), `UpdateTaskStatus`
  (Status-Übergangsvalidierung, ADR-030), `AcknowledgeAlert`
- auth-service: `OperatorLogin` (Auth + Token-Ausstellung), `HandoverToken` (Token-Validierung +
  Ziel-Validierung + befristete ACTIVE_OPERATOR-Ausstellung), `RefreshToken` (Parse + Reissue),
  sowie die in `DeleteUser`/`UpdateUserRole` **duplizierte** "cannot modify own account"-Guard-Logik
  (Rule of Three: zweite Duplizierung, jetzt in eine gemeinsame Funktion extrahieren)

Kein Verhaltens-, kein HTTP-Signatur-Wechsel. `Handler`-Methoden bleiben bestehen und werden zu
dünnen Wrappern (decode → Use-Case-Aufruf → broadcast/encode), analog zum Präzedenzfall
HEX-02/HEXAUTH-02 (Konstruktorsignaturen unverändert).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEXUC-01 | fleet-service: neue Datei `internal/fleetservice/usecase.go` — `createAndDispatchTask(store FleetStore, gw Dispatcher, t Task) (Task, error)` extrahiert aus `CreateTask` (Persistenz + Fire-and-forget-Dispatch, Dispatch-Fehler bewusst nicht propagiert wie bisher). `Handler.CreateTask` ruft die Funktion auf, behält decode/validate/broadcast/encode. | S | ✅ Sprint 45 | — |
| HEXUC-02 | fleet-service: `transitionTaskStatus(store FleetStore, hub *Hub, id, status, changedBy string) (Task, error)` und `acknowledgeAlert(store FleetStore, hub *Hub, id, ackBy string) error` in derselben Datei — bündeln Store-Aufruf **und** Dashboard-Broadcast als eine Einheit (Broadcast ist Domänen-, nicht Transport-Belang), Fehler-Sentinels (`ErrTaskNotFound`/`ErrInvalidTransition`) unverändert durchgereicht. Anpassung ggü. Sprint-Plan: Broadcast wurde mit in die Funktion gezogen statt nur den reinen Store-Call zu extrahieren — sonst wäre die Funktion ein wertloser 1:1-Passthrough gewesen. | S | ✅ Sprint 45 | HEXUC-01 |
| HEXUC-03 | fleet-service: Unit-Tests für die 3 neuen Funktionen direkt gegen `FakeFleetStore` + Fake-`Dispatcher` (kein `httptest` nötig, aber Broadcast-Verifikation nutzt die bestehenden `broadcast_test.go`-Helper `newTestHubServer`/`dialTestClient`/`waitForClientCount`, da `Hub.Broadcast` nur über einen echten verbundenen Client beobachtbar ist) — 7 neue Tests in `usecase_test.go`, inkl. Gegenprobe "Dispatch-Fehler lässt Task-Erstellung trotzdem erfolgreich sein" (ADR-027) und "Fehlerpfad broadcastet nichts" (Marker-Broadcast-Technik). Bestehende `handler_test.go`-Tests bleiben unverändert grün (Verhalten identisch). Verifiziert: `go test ./internal/fleetservice/... -race -count=2` grün. | S | ✅ Sprint 45 | HEXUC-01, HEXUC-02 |
| HEXUC-04 | auth-service: neue Datei `internal/authservice/usecase.go` — `login(ctx, userStore UserStore, tokens TokenIssuer, username, password string) (string, error)` und `refreshToken(tokens TokenIssuer, tokenStr string) (string, error)` extrahiert aus `OperatorLogin`/`RefreshToken`, mit Sentinel-Fehlern `ErrInvalidCredentials`/`ErrInvalidToken` damit `Handler` weiterhin 401 vs. 500 unterscheiden kann. | S | ✅ Sprint 45 | — |
| HEXUC-05 | auth-service: `handoverToken(tokens TokenIssuer, currentToken, targetID string) (string, error)` extrahiert aus `HandoverToken` (Validierung des optionalen aktuellen Tokens + Pflicht-`target_id` + befristete 1h-`ACTIVE_OPERATOR`-Ausstellung bleibt exakt wie bisher, Sentinel-Fehler `ErrHandoverUnauthorized`/`ErrTargetIDRequired` für die 401/400-Unterscheidung). Plus: gemeinsame `canModifyUser(callerUsername string, target User) bool`-Funktion, ersetzt die in `DeleteUser` und `UpdateUserRole` bisher duplizierte "cannot modify own account"-Prüfung. | S | ✅ Sprint 45 | HEXUC-04 |
| HEXUC-06 | auth-service: Unit-Tests für `login`/`refreshToken`/`handoverToken`/`canModifyUser` direkt gegen den echten `JWTTokenIssuer` (reine Berechnung, kein Fake nötig — mirrors HEXAUTH-01's Doku-Begründung) + internem `ucStubUserStore` (Package-lokale Kopie, da `handler_test.go`s `stubUserStore` im externen `authservice_test`-Package unerreichbar ist) — 10 neue Tests in `usecase_test.go`. Bestehende `handler_test.go`-Tests bleiben unverändert grün. Verifiziert: `go test ./internal/authservice/... -race -count=2` grün. | S | ✅ Sprint 45 | HEXUC-04, HEXUC-05 |
| HEXUC-07 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./internal/fleetservice/... ./internal/authservice/... -race -count=2` (Flakiness-Check, CLAUDE.MD §17) — alles grün. ADR-031-Status-Update (HEX-06/HEXAUTH-04 abgeschlossen, mit explizitem Hinweis welche Endpunkte bewusst als reine Pass-Throughs unverändert blieben und warum) + `DECISIONS.MD` + `tasks/backlog.md`-Status-Update (HEX-06-Zeile 360, HEXAUTH-04-Zeile 410). | S | ✅ Sprint 45 | HEXUC-01..06 |

**Nicht Teil dieses Sprints:** Use-Case-Extraktion der reinen CRUD-Pass-Through-Endpunkte (siehe
Begründung oben), Hexagonal-Migration von `safety-service`/`webrtc-sfu`/`internal/recording`
(eigener, noch offener Entscheidungspunkt), `control-server`-Vorbereitung (ADR + Testaufbau,
eigener noch offener Entscheidungspunkt) — beide bleiben mögliche Kandidaten für einen
Folge-Sprint.

**Geschätzter Umfang:** 7 Tasks, alle Typ S — deutlich unter dem ~200k-Token-Sprintbudget.

**Ergebnis:** Neue Dateien `internal/fleetservice/usecase.go` + `usecase_test.go` (7 neue Tests),
`internal/authservice/usecase.go` + `usecase_test.go` (10 neue Tests). `Handler`-Methoden in
beiden Services unverändert in Signatur/HTTP-Verhalten, rufen jetzt nur noch die extrahierten
reinen Funktionen auf. Damit ist die Hexagonal-Migration für fleet-service/auth-service/
telemetry-service inkl. beider optionaler Folgeschritte (HEX-06, HEXAUTH-04) vollständig
abgeschlossen — `safety-service`/`webrtc-sfu`/`internal/recording` und `control-server` bleiben
die einzigen noch nicht migrierten Services (beide eigene, noch offene Entscheidungspunkte).
Verifiziert: `go build ./...`, `go vet ./...` sauber; `go test ./internal/fleetservice/...
./internal/authservice/... -race -count=2` zweimal grün, keine Flakiness. Doku aktualisiert:
`docs/adr/031-hexagonal-architecture-migration.md` (Status-Zeile + neuer Update-Block, dabei auch
eine stale gewordene Status-Zeile für Schritt 3/telemetry-service korrigiert), `DECISIONS.MD`
(ADR-031-Zeile + neue Zeile in "Offene Folge-Entscheidungen"), `tasks/backlog.md` (HEX-06/
HEXAUTH-04 auf ✅ Sprint 45).

---

Vorgänger: Sprint 44 ✅ (Backup-Strategie Audit Store, siehe
`tasks/sprints/44-backup-strategie-audit-store.md`)
