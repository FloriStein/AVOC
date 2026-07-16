# Drift-Audit 2026-07 — MD-Dokumentation vs. Ist-Zustand

Konsolidierung der Sprint-26-Befunde (AUDIT-01 bis AUDIT-07). Vollständige Herleitung/Belege je
Befund stehen in `tasks/current-sprint.md` (Sprint-26-Abschnitt, je AUDIT-0X-Unterabschnitt) — hier
nur die konsolidierte, nach Schweregrad und Typ (§1.1 CLAUDE.MD) sortierte Übersicht als
Ausgangspunkt für die Behebung.

Schweregrad-Skala (aus dem Sprint-Grill-Me, konsistent über alle AUDIT-Tasks verwendet):

- **Kritisch:** Doku behauptet ein Sicherheits-/Safety-Verhalten, das der Code nicht (mehr)
  erfüllt, oder umgekehrt.
- **Mittel:** veraltete/irreführende, aber nicht sicherheitsrelevante Doku.
- **Niedrig:** kosmetische Abweichung ohne Entscheidungsrelevanz.

Typ-Klassifizierung (§1.1 CLAUDE.MD): **S** = reine Doku-Korrektur ohne Architekturbezug
(Fast-Track). **M/L** = Code muss an Doku angepasst werden oder umgekehrt, mit Architekturbezug —
eigener Task durch die volle Phasenfolge.

Status-Spalte: `offen` = noch nicht behoben, `behoben` = umgesetzt (Befund bleibt zur
Nachvollziehbarkeit stehen, wird nicht gelöscht).

---

## Kritisch (6 Befunde — vor Umsetzung jeweils eigene Grill-Me-Session, kein direkter Fix)

Alle sechs betreffen dieselbe Grundfrage: eine im Core-Load-Dokument (`CONTEXT.MD`) oder einem ADR
als bestehend behauptete Sicherheits-/Latenz-Garantie hat keinen echten Produktivpfad — nur
Unit-Tests/Benchmarks, die synthetisch das behauptete Verhalten erzeugen bzw. gar nicht laufen.
Drei betreffen die Failure-Classification-Tabelle (Safety-Trigger), drei die <100ms-Control-Loop-
Garantie (Latenz-Trigger). Gemeinsame Empfehlung (AUDIT-05/AUDIT-06): ggf. gebündelte
Grill-Me-Sessions pro Themenblock statt sechs komplett unabhängiger — Entscheidung liegt beim
Nutzer.

| ID | Befund | Quelle | Typ | Status |
|----|--------|--------|-----|--------|
| DRIFT-K1 | ADR-009 CRITICAL-Trigger „Auth Invalidation" (JWT-Widerruf → SAFE_MODE) ohne Produktivpfad — `EventAuthInvalid` nur in `tests/unit/safety_test.go:183/189` erzeugt, kein Revocation-Mechanismus in `authservice`/`websocket.go` | AUDIT-02 | L | offen |
| DRIFT-K2 | ADR-009 CRITICAL-Trigger „No Active Operator" ohne Produktivpfad — `TransitionOperator(OpNoOperator)` wird produktiv nirgends aufgerufen; der praktische Effekt entsteht nur zufällig über den WS-Disconnect-Handler, OPERATOR-Layer bleibt sonst bei `ACTIVE_OPERATOR` hängen | AUDIT-05 | L | offen |
| DRIFT-K3 | DEGRADED-Tier (Media/Video/Telemetrie) komplett unverdrahtet — `TransitionMedia(...)` wird produktiv nie aufgerufen (`internal/webrtcsfu`, `internal/mediamtx` lösen es nicht aus), UI-Warnfunktion bei Videoverlust hat keine Backend-Grundlage | AUDIT-05 | L | offen |
| DRIFT-K4 | Dokumentierter „CI Build-Fail" bei >100ms-Latenzverletzung existiert nicht als automatisierte Pipeline — kein `.github/workflows`/`.gitlab-ci.yml`/Jenkinsfile im Repo, Latenztests nur manuelle Makefile-Targets | AUDIT-06 | L | offen |
| DRIFT-K5 | `BenchmarkControlACKRoundtrip` (der reale WS-ACK-Test) ist ein Go-Benchmark, läuft im Standard-Testlauf (`make test`) nicht mit, wird nirgends automatisiert getriggert; `b.Skip(...)` bei fehlender Infrastruktur liefert zusätzlich stillschweigend „grün" ohne echte Messung | AUDIT-06 | M/L | offen |
| DRIFT-K6 | `TestLatencyBudget_DocumentedRequirement` ist tautologisch (prüft nur die im selben File hartkodierte 100ms-Konstante gegen sich selbst) — kann fälschlich als Beleg für laufende Latenzüberwachung gelesen werden | AUDIT-06 | S/M | offen |

---

## Mittel — Typ S (reine Doku-Korrektur, Fast-Track, direkt umsetzbar)

| ID | Befund | Quelle | Status |
|----|--------|--------|--------|
| DRIFT-M01 | `vision.md` §11 „Ist die Ausschreibung beauftragt?" ist bereits seit 2026-07-13 in `requirements.md` §Projektstatus beantwortet, aber nicht zurückverlinkt | AUDIT-01 | offen |
| DRIFT-M02 | `ADR-018` (Status „Accepted") referenziert die spätere PostgreSQL-Migration (`ADR-023`) nirgends — datierter Update-Block nötig (nicht überschreiben, §6) | AUDIT-02 | offen |
| DRIFT-M03 | `ADR-011` (System State Machine) referenziert `ADR-026` (Per-Vehicle-Isolation) nicht, obwohl `architecture.md`/`CONTEXT.MD` den Wechsel korrekt zeigen | AUDIT-02, AUDIT-05 | offen |
| DRIFT-M04 | `ADR-022` (Vehicle Registry, SQLite) referenziert `ADR-023` (Postgres-Migration) nicht — Code nutzt bereits vollständig Postgres | AUDIT-02 | offen |
| DRIFT-M05 | `ADR-021` (Vehicle Connectivity) erwähnt `vehicle-mock`s seit Sprint 21 bestehende Doppelrolle (Single-Vehicle-Mock + Fleet-Simulator) nicht | AUDIT-02 | offen |
| DRIFT-M06 | `ADR-025` (`vehicleController`-Map) und `ADR-026` (`vehiclecontext.Registry`) ohne Cross-Referenz — Risiko, ADR-026 werde fälschlich als Ablösung von ADR-025 gelesen | AUDIT-02 | offen |
| DRIFT-M07 | `ADR-026` behauptet, `GET /state` entfalle als Polling-Endpunkt (Zeile 85) — Code behält ihn bewusst als Compat-Shim (`main.go:643`), bereits korrekt in `architecture.md`/`DECISIONS.MD` (MV-12) dokumentiert, nur ADR-026-Text selbst nie korrigiert | AUDIT-02 | offen |
| DRIFT-M08 | `DECISIONS.MD` führt `ADR-014` als „Accepted", obwohl die ADR-Datei selbst korrekt „Superseded by ADR-020" trägt | AUDIT-02 | offen |
| DRIFT-M09 | `ADR-012b` behauptet, generierter Protobuf-Code werde per `.gitignore` ausgeschlossen — `gen/go/*.pb.go` sind tatsächlich getrackt, keine `gen/`-Regel in `.gitignore` | AUDIT-02 | offen |
| DRIFT-M10 | `docs/code-patterns.md` §1–3 zeigt noch das per ADR-026 abgelöste Singleton-Pattern (`Machine`/`DeadmanWatchdog`/`Engine` mit Einzelfeldern) statt der tatsächlichen `VehicleContextRegistry` | AUDIT-03 | offen |
| DRIFT-M11 | `docs/code-patterns.md` §3 unterschlägt die sicherheitsrelevanten Zweige des echten `Engine.Handle` (Audit-Eintrag vor SAFE_MODE, OBSERVER-Rollenblock) | AUDIT-03 | offen |
| DRIFT-M12 | Stale Kommentar in `frontend/src/hooks/useDeadmanSwitch.ts:34` nennt 2s Watchdog-Timeout, tatsächlicher Server-Timeout ist 10s (Code selbst korrekt, nur Kommentar falsch) | AUDIT-03 | offen |
| DRIFT-M13 | `docs/architecture.md` Frontend-Komponententabelle (Sprint 22–23 nachgetragen) führt `FleetTaskPanel.tsx` (Sprint 24) und `fleet-alert-sound.ts`/`useFleetAlertSound.ts` (Sprint 25) nicht | AUDIT-04 | offen |
| DRIFT-M14 | `docs/architecture.md` Container-Services-Tabelle führt den laufenden `postgres`-Container an keiner Stelle als eigene Zeile | AUDIT-04 | offen |
| DRIFT-M15 | `CONTEXT.MD` Failure-Classification-Tabelle fasst zwei eigenständige Mechanismen (`ACKTimeoutWatcher` vs. `VehicleACKWatchdog`, unterschiedliche Timeouts/Trigger) fälschlich unter „Command ACK Timeout" zusammen | AUDIT-05 | offen |
| DRIFT-M16 | `docs/requirements.md` „State Machine Requirements" beschreibt nur die globale State-Machine-Variante, ohne Per-Vehicle-Hinweis (ADR-026) | AUDIT-05 | offen |
| DRIFT-M17 | `README.md`-Projektstruktur-Baum (Zeilen 182-206) nennt `cmd/fleet-service`/`internal/fleetservice`/`internal/fleetgateway` nicht, obwohl die Service-Tabelle im selben Dokument korrekt ist | AUDIT-07 | offen |

## Mittel — Typ M/L (Code ⇄ Doku, eigener Task durch volle Phasenfolge)

| ID | Befund | Quelle | Typ | Status |
|----|--------|--------|-----|--------|
| DRIFT-M18 | OBSERVATION-Trigger „Auth Service down → neue Sessions blockiert" ohne Implementierung — `POST /session/start` prüft keinen Auth-Service-Health-Endpoint. Entscheidung nötig: implementieren oder Doku auf Ist-Zustand korrigieren | AUDIT-05 | M | offen |
| DRIFT-M19 | k6-Skript `latency.js` misst laut eigenem Kommentar `GET /state` statt des dokumentierten WS-ACK-Roundtrips — Schwellwert stimmt, Messgröße nicht | AUDIT-06 | M | offen |
| DRIFT-M20 | Video-Latenzziel (100–300ms, ADR-014 „kein Safety-Hartziel") hat keinen automatisierten Test — nur Farbcodierungs-Test der UI-Anzeige. Entscheidung nötig: Test ergänzen oder Doku als „unverifiziert/aspirational" kennzeichnen | AUDIT-06 | S/M | offen |
| DRIFT-M21 | `Doku.md` (EC2-Produktions-Deployment-Guide) deckt `fleet-service` komplett nicht ab — `docker-compose.prod.yml` hat keinen `fleet-service`-Eintrag, Guide vordatiert den Fleet-Pivot und wurde nie nachgezogen. Produktions-Deployment von Fleet-Features ist damit faktisch ungetestet/nicht dokumentiert | AUDIT-07 | M/L | offen |
| DRIFT-M22 | `-race`-Vorgabe aus CLAUDE.MD §17 strukturell nicht erzwungen — kein CI, `Makefile`-Targets rufen `-race` nirgends auf; Race-Checks passieren nur manuell/ad-hoc | AUDIT-04 | M | offen |
| DRIFT-M23 | Dokumentierte Testlücke `store.CreateAlert`-Fehlerpfad nach `AlertEngine.Evaluate()` (FLEET-07-Nachtrag) wurde nie in einen Backlog-Task überführt | AUDIT-04 | S | offen |
| DRIFT-M24 | Magic-Value-Verstoß: `OperatorRole` als Rohstring in `session/manager.go`/`command/engine.go`/`transport/websocket.go` statt Wiederverwendung des bereits existierenden Typs aus `authservice/handler.go` — Tippfehlerrisiko könnte ADR-025-Rollenprüfung lautlos aushebeln | AUDIT-03 | M | offen |
| DRIFT-M25 | Magic-Value-Verstoß: Task-Status als Rohstrings in `internal/fleetservice/store.go` statt eines benannten Typs (Vorbild: `statemachine/state.go`) | AUDIT-03 | M | offen |

---

## Niedrig (Typ S, Fast-Track, kosmetisch — ohne Entscheidungsrelevanz)

| ID | Befund | Quelle | Status |
|----|--------|--------|--------|
| DRIFT-N01 | `docs/adr/README.md` Kopfzeile nennt „29 ADRs", tatsächlich 32 Tabellenzeilen | AUDIT-02 | offen |
| DRIFT-N02 | `ADR-002` Pseudocode-Methodennamen (`SubscribeSafetyEvents`/`TriggerEmergencyStop(reason)`) vom tatsächlichen Code (`Subscribe`, 3 Parameter) abgedriftet | AUDIT-02 | offen |
| DRIFT-N03 | `ADR-009` nennt falsche Dateipfade für zwei Trigger (`safety/deadman.go` existiert nicht, tatsächlich `detector.go`; „No Active Operator" fälschlich in `session/manager.go` verortet) sowie falsche Testanzahl („18" statt 20) | AUDIT-02 | offen |
| DRIFT-N04 | `ADR-017` nennt nicht die zusätzliche, bereits im Code vorhandene `AuditWriter`-Methode `QueryBySession` | AUDIT-02 | offen |
| DRIFT-N05 | `frontend/src/hooks/useControls.ts` Magic-Number-Fallbacks (`CommandType?.STEER ?? 1` usw.) spiegeln Proto-Enum-Ordinalzahlen hart codiert, ohne benannte Konstante (nur Edge-Case-Pfad) | AUDIT-03 | offen |
| DRIFT-N06 | `README.md:84` „Vitest Component-Tests (41 Tests)" veraltet — tatsächlich 252 Tests über 25 Dateien | AUDIT-07 | offen |
| DRIFT-N07 | `docs/requirements.md` Kopfzeilen-Datumsstempel („Stand: 2026-07-10") hinkt dem eigenen, laufend aktualisierten Inhalt hinterher (§Projektstatus bereits „Stand 2026-07-13") | AUDIT-07 | offen |

---

## Bereits bekannte, nicht neu erfasste Punkte (Cross-Referenzen)

Diese Themen wurden während des Audits mehrfach bestätigt, sind aber bereits an anderer Stelle als
offene Punkte erfasst — hier nur verlinkt, kein neuer Backlog-Task:

- **AP3 „Admin-Konsole" hat kein Backlog-Epic, 0 % begonnen** (AUDIT-01) → siehe
  `tasks/backlog.md`, neues `## EPIC: AP3` empfohlen (eigener Punkt, s. u. DRIFT-M-Tabelle nicht
  nötig, da strukturelle Backlog-Änderung statt Einzel-Drift — siehe „Empfehlung AUDIT-08" unten).
- **Prinzip 6 „Interface over Implementation" lückenhaft** (`safety-service`-Bus, `SessionRecorder`)
  — bereits in `CONTEXT.MD` „Offene Fragen" (ADR-031-Bestandsaufnahme) und `vision.md` (AUDIT-01)
  dokumentiert, hier durch AUDIT-02 (ADR-005) erneut bestätigt, kein neuer Task.
- **ADR-030/031-Nummernkollision** — bereits in `DECISIONS.MD`-Historie sauber aufgelöst, AUDIT-02
  bestätigt keine Rückstände.

---

## Empfehlung für die Umsetzung (AUDIT-08)

1. **Kritisch (DRIFT-K1–K6):** je Themenblock (Safety-Trigger K1–K3, Latenz-Infrastruktur K4–K6)
   eine Grill-Me-Session vor jeder Umsetzung — „welche Seite ist richtig" ist hier nicht trivial
   (z. B. K2: der WS-Disconnect-Handler deckt den Fall in der Praxis bereits ab — Frage ist, ob der
   fehlende Operator-Layer-Pfad ein echtes Risiko oder nur eine Doku-Ungenauigkeit ist).
2. **Mittel/Niedrig Typ S (DRIFT-M01–M17, DRIFT-N01–N07):** Fast-Track, direkt umsetzbar ohne
   Architekturentscheidung — reine Doku-Korrekturen bzw. ein Kommentar-Fix.
3. **Mittel Typ M/L (DRIFT-M18–M25):** je eigener Task durch die volle Phasenfolge (§1), nicht
   vermischen. `DRIFT-M24`/`M25` (Magic-Value-Refactorings) sind sicherheitsnah (ADR-025-Rollenprüfung
   bzw. Task-Zustandsmaschine) — volle Teststandard-Abdeckung (§17) bei Umsetzung erforderlich.
4. Neues `## EPIC: AP3` in `tasks/backlog.md` anlegen (AUDIT-01-Empfehlung) — strukturelle
   Backlog-Ergänzung, kein Einzel-Drift-Fix, aber Voraussetzung für ehrlichen Meilenstein-Status.
