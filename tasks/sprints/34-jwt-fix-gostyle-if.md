# Sprint 34 — JWT-Alg-Confusion-Fix (SEC-01) + GOSTYLE-IF-01/02/05/06

Ziel: Sprint 33 hat den Hexagonal-Pilot (HEX-05) abgeschlossen, damit sind GOSTYLE-IF-01..06
(Phase 2 Interface-Segregation) entsperrt. Bei der Sprint-34-Planung zusätzlich nachrecherchiert:
die in Sprint 29 als Nebenbefund vermerkte JWT-Alg-Confusion-Lücke betrifft tatsächlich 3 von 5
`jwt.Parse*`-Stellen (nicht 2 von 4 wie ursprünglich notiert) — echte, bisher unbehobene
Sicherheitslücke, jetzt als `SEC-01` formal im Backlog erfasst.

**Explizites Nutzer-Budget für diesen Sprint:** ca. 200.000 Token, um den Verbrauch sprintübergreifend
zu regulieren (zusätzlich zur bestehenden Typ-S/M-Vorgabe aus CLAUDE.MD Abschnitt 10) — Grund für
den bewusst kleineren Zuschnitt unten (4 von 6 GOSTYLE-IF-Tasks, die zwei größeren M-Tasks auf
Sprint 35 verschoben).

Triage 2026-07-18 (CLAUDE.MD Abschnitt 10):
- **SEC-01** (JWT-Alg-Confusion, 3 Stellen ohne Signaturmethoden-Prüfung): Sicherheitsrelevant,
  höchste Priorität dieses Sprints — dem Nutzer als Sprint-Schwerpunkt vorgeschlagen und bestätigt.
- **GOSTYLE-IF-01/02** (ungenutzte `SessionRecorder`/`FleetGateway`-Interfaces): Typ S, geringes
  Risiko (reine Auflösung toter Abstraktion, Rule 1.2) — aufgenommen.
- **GOSTYLE-IF-05/06** (`VehicleStore`-Bootstrap-Trennung, `safety.Publisher`-Aufspaltung): Typ S,
  aufgenommen.
- **GOSTYLE-IF-03/04** (`AuditWriter`/`UserStore`, beide Typ M, mehr Konsumenten betroffen):
  bewusst auf Sprint 35 verschoben, um das ~200k-Token-Budget nicht zu sprengen.
- **HEX-06** (optionale Use-Case-Schicht-Extraktion): explizit dem Nutzer vorgelegt (kein
  Code-Task, CLAUDE.MD Abschnitt 4) — Nutzerentscheidung: weiterhin zurückstellen, kein neuer
  Anlass.
- **Hexagonal-Fortsetzung auf auth-service/telemetry-service**: ebenfalls explizit vorgelegt —
  Nutzerentscheidung: zurückstellen. Zusätzlich inhaltlich sinnvoll, da `auth-service` ohnehin
  Ziel von `SEC-01` in diesem Sprint ist (keine zwei große Umbauten am selben Code parallel).
- **AP1-01..03** (Workshop Professur Logistik), **FLEET-01**, **MV-10**: weiterhin extern
  blockiert bzw. ohne neuen Anlass zurückgestellt, nicht erneut geprüft.

**Keine Signaturänderung nach außen ohne Zweck** (CLAUDE.MD Abschnitt 15) — Ausnahme sind die
GOSTYLE-IF-Tasks selbst dort, wo eine Interface-Methode entfernt wird (Zweck des Tasks); der
JWT-Fix ändert kein öffentliches Verhalten für gültige Tokens, nur für zuvor fälschlich akzeptierte
(falscher Algorithmus). `docs/go-style-guide.md` gilt für alle Go-Tasks in diesem Sprint.

Datum: 2026-07-18 | **Status: Code + Doku fertig, noch uncommitted** (kein Commit ohne explizite Nutzeraufforderung)
Vorgänger: Sprint 33 ✅ (Code + Doku, aber **noch uncommitted** zum Zeitpunkt des Sprint-34-Kickoffs)
Branch/Worktree: **kein eigener** — läuft bewusst im bestehenden Worktree
`controlcenter-aws-sprint33` (Branch `feature/fleet-service-foundation-sprint33`) weiter, da
Sprint 34 (GOSTYLE-IF-*) auf Sprint 33s `HEX-05`-Ergebnis (`FleetStore`-Interface,
`FakeFleetStore`) aufbaut, das nur uncommitted in diesem Arbeitsverzeichnis existiert. Ein neuer
`git worktree add` würde davon nichts enthalten (nur den letzten Commit `b0e902a`). Kein Commit
ohne explizite Nutzeraufforderung — daher kein Branch-Wechsel, keine neue Worktree-Anlage.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| SEC-01 | JWT-Alg-Confusion-Check ergänzen (`authservice`, `control-server`/`websocket.go`, `vehicleconnection`) | S/M | ✅ |
| GOSTYLE-IF-01 | `recording.SessionRecorder`: Interface auflösen oder tatsächlich verwenden | S | ✅ |
| GOSTYLE-IF-02 | `fleetgateway.FleetGateway`: gleiche Frage wie IF-01 | S | ✅ |
| GOSTYLE-IF-05 | `vehicleregistry.VehicleStore`: `SeedDefault` aus Interface lösen | S | ✅ |
| GOSTYLE-IF-06 | `controlserver/safety.Publisher`: `PublishEvent`-only Interface aufspalten | S | ✅ |

## Ergebnisse

- **SEC-01**: Alle 5 `jwt.Parse*`-Stellen im Repo geprüft. 2 waren bereits korrekt
  (`internal/fleetservice/handler.go:validateToken`, `cmd/control-server/main.go` Auth-Middleware).
  3 fehlten den `t.Method.(*jwt.SigningMethodHMAC)`-Check und wurden ergänzt:
  `internal/authservice/handler.go:parseToken`, `internal/controlserver/transport/websocket.go:validateJWT`
  (fehlenden `fmt`-Import ergänzt), `internal/vehicleconnection/handler.go:validateJWT` (ebenfalls
  `fmt`-Import ergänzt). Regressionstests mit gefälschtem `alg:none`-Token ergänzt: 2 neue Tests in
  `internal/authservice/handler_test.go` (End-to-End über `ValidateToken`/`RequireAdmin`), neue
  Testdateien `internal/controlserver/transport/websocket_test.go` und
  `internal/vehicleconnection/handler_test.go` (beide vorher ohne Tests) mit je einem
  Alg-None-Reject- und einem Valid-HMAC-Accept-Test. Alle Tests 2x hintereinander grün
  (Flakiness-Check gemäß CLAUDE.MD Abschnitt 17).
- **GOSTYLE-IF-01**: `SessionRecorder`-Interface hatte genau eine Implementierung
  (`MemoryRecorder`), einzige Konsumentin (`cmd/control-server/main.go`) referenziert bereits den
  konkreten Typ `*recording.MemoryRecorder` direkt, nirgends als Interface-Typ verwendet — tote
  Abstraktion. Interface aus `internal/recording/recorder.go` entfernt, Package-Kommentar
  angepasst.
- **GOSTYLE-IF-02**: `FleetGateway`-Interface wird von ADR-027 (aktiv, nicht überschrieben)
  bewusst für die künftige Auswechselbarkeit gegen das noch unbestätigte ROS2/DDS-Backend
  vorgeschrieben — anders als IF-01 **nicht** gelöscht (Konflikt mit aktivem ADR wäre stillschweigendes
  Überschreiben, CLAUDE.MD Abschnitt 3/6). Stattdessen tatsächlich nutzbar gemacht: die beiden
  Helper `subscribeVehicleStatus`/`subscribeVehicleAlerts` in `cmd/fleet-service/main.go` nahmen
  bisher den konkreten Typ `*fleetgateway.MQTTGateway` entgegen, obwohl sie nur
  `SubscribeVehicleStatus`/`SubscribeVehicleAlerts` aus dem Interface brauchen — Parametertyp auf
  `fleetgateway.FleetGateway` geändert (Rest von `main()` unverändert, `gw.Close()` bleibt auf dem
  konkreten Typ).
- **GOSTYLE-IF-05**: `SeedDefault` wird ausschließlich einmalig auf dem konkreten
  `*PostgresVehicleStore` in `cmd/control-server/main.go:newVehicleStore` aufgerufen, bevor der
  Store als `VehicleStore`-Interface zurückgegeben wird — nie über das Interface selbst. Aus dem
  Interface entfernt (`internal/vehicleregistry/registry.go`); `NoopVehicleStore.SeedDefault`
  dadurch tot geworden und mit gelöscht (`noop_store.go`); `PostgresVehicleStore.SeedDefault`
  bleibt als konkrete Methode bestehen.
- **GOSTYLE-IF-06**: Geprüft, wer `safety.Publisher` interface-typisiert hält:
  `command.Engine`, `vehiclecontext.Registry`, `DeadmanWatchdog`/`ACKTimeoutWatcher`/
  `VehicleACKWatchdog` (alle `detector.go`) und `BusWatchdog` (`bus_watchdog.go`) — alle rufen
  ausschließlich `PublishEvent` auf, nie `TriggerEmergencyStop`. `TriggerEmergencyStop` wird nur
  direkt auf dem konkreten `*HTTPPublisher` aufgerufen (`cmd/control-server/main.go:561`), nie über
  das Interface. `TriggerEmergencyStop` daher aus `Publisher` entfernt (`internal/controlserver/
  safety/publisher.go`), Interface ist jetzt PublishEvent-only; `HTTPPublisher`/
  `MockSafetyPublisher` behalten die Methode konkret weiter.

**Verifikation gesamt**: `go build ./...`, `go vet ./...` sauber. `go test ./...` — alle Unit-/
Package-Tests grün (inkl. sicherheitsrelevanter Pakete 2x hintereinander gegen Flakiness geprüft);
Integrationstests (`tests/integration`) schlagen wie erwartet mit `connection refused` fehl, da
kein docker-compose-Stack läuft — laut Sprint-Vorgabe für diesen rein Backend-Interface-Task ohne
Schema-/API-Verhaltensänderung nicht nötig. Kein Frontend-/Browser-Check, da keine der fünf
Änderungen client-sichtbares Verhalten oder API-Response-Shapes ändert.
