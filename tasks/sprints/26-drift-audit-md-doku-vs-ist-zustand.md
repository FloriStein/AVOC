# Sprint 26 — Drift-Audit: MD-Dokumentation vs. Ist-Zustand

Ziel: Vor der nächsten Feature-Entwicklung prüfen, ob zwischen den maßgeblichen Markdown-Dokumenten
(`docs/vision.md`, alle 34 ADRs in `docs/adr/`, `docs/code-patterns.md`/`CLAUDE.MD` §12,
Qualitätsziele aus `vision.md` §8, Sicherheitsregeln aus `CLAUDE.MD` §13/`CONTEXT.MD`/
`requirements.md`, Performance-Ziele aus `requirements.md`/`architecture.md`) und dem tatsächlichen
Ist-Zustand des Projekts (Code, Tests, Deployment, `README.md`/`Doku.md`/`DECISIONS.MD`) Drifts
entstanden sind. Dieser Sprint entwickelt keine neuen Features und ändert bewusst keinen
Produktivcode — reine Bestandsaufnahme.

Grill-Me-Session (2026-07-16), vier Fragen, jeweils die empfohlene Option gewählt:

- **Scope:** nur Audit (identifizieren + dokumentieren), keine Fixes in diesem Sprint — gefundene
  Drifts werden als eigene Folgetasks in `tasks/backlog.md` aufgenommen. Begründung: 30-180-Min-
  Taskgrößenregel (§10) und sicherheitsrelevante Fixes sollen ihren eigenen Grill-Me-/ADR-Prozess
  durchlaufen statt nebenbei mitgepatcht zu werden.
- **Granularität:** ein Task pro Kategorie (~7 Tasks) statt Aufsplittung pro Domäne (Core-Teleop/
  Fleet/Frontend) oder ein einzelner Typ-L-Recherche-Task.
- **Output:** neues datiertes Audit-Dokument (`docs/drift-audit-2026-07.md`), bestätigte Drifts
  zusätzlich als konkrete Einträge in `tasks/backlog.md`. Kein separates ADR pro Befund (nur falls
  ein Befund tatsächlich eine Architekturentscheidungs-Lücke aufdeckt, s. §6).
- **Branch:** eigener neuer Worktree/Branch (`feature/docs-drift-audit`, Basis:
  `feature/fleet-service-foundation`), konsistent zum bisherigen Muster (Sprint 23/24/25 liefen
  ebenfalls in eigenen Worktrees).

**Schweregrad-Skala für alle Befunde** (konsistent über AUDIT-01–07 hinweg zu verwenden):

- **Kritisch:** Doku behauptet ein Sicherheits-/Safety-Verhalten, das der Code nicht (mehr) erfüllt,
  oder umgekehrt (z. B. Invariante aus `CONTEXT.MD` ohne Entsprechung im Code).
  Sicherheit schlägt alles (§0) — sofort auch mündlich/im Chat hervorheben, nicht nur im Report.
- **Mittel:** veraltete/irreführende, aber nicht sicherheitsrelevante Doku (z. B. ADR beschreibt
  einen Zustand, der durch einen späteren ADR überholt, aber nicht verlinkt wurde).
- **Niedrig:** kosmetische Abweichung ohne Entscheidungsrelevanz (Tippfehler, veraltete Zahl ohne
  Konsequenz).

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 25 ✅ / Sprint 24 ✅ / Sprint 23 ✅ (alle bereits in
`feature/fleet-service-foundation` gemerged)
Branch: `feature/docs-drift-audit` (eigener Worktree `controlcenter-aws-driftaudit`, Basis:
`feature/fleet-service-foundation`)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUDIT-01 | Vision-Drift: `docs/vision.md` (Grundprinzipien §7, Qualitätsziele §8, Zielgruppen, Erfolgskriterien §10, Nicht-Ziele §6) gegen Ist-Zustand abgleichen | M | ✅ |
| AUDIT-02 | ADR-Gesamtkonsistenz: alle 34 ADRs (`docs/adr/*`) gegen aktuellen Code/Architektur prüfen — überholte, aber nicht verlinkt als "updated" markierte ADRs; sich widersprechende ADRs; ADRs ohne Code-Entsprechung | M | ✅ |
| AUDIT-03 | Coding Guidelines: `docs/code-patterns.md` + `CLAUDE.MD` §12 (KISS, keine versteckte Logik, keine Magic Values, explizite Abhängigkeiten) gegen tatsächlichen Code-Stil (Stichproben Backend/Frontend) abgleichen | M | ✅ |
| AUDIT-04 | Qualitätsziele: `vision.md` §8 gegen Ist — Teststandard (`CLAUDE.MD` §17) tatsächlich eingehalten, Wartbarkeit, Dokumentationsvollständigkeit | M | ✅ |
| AUDIT-05 | Sicherheitsregeln: `CLAUDE.MD` §13, `CONTEXT.MD` (Safety Concepts, Invarianten 1–3, Failure Classification), `requirements.md` "Safety Requirements" gegen tatsächliche Implementierung (State Machine, Watchdogs, SAFE_MODE, Auth) abgleichen | M | ✅ |
| AUDIT-06 | Performance-Ziele: `requirements.md` "Performance Requirements", `architecture.md` "Latency Targets", `CONTEXT.MD` Control-Loop-Zielwert (<100ms) gegen tatsächlich gemessene/getestete Werte (CI-Latenztests, echte Messungen) abgleichen | M | ✅ |
| AUDIT-07 | Ist-Zustand-Abgleich: `requirements.md` "Projektstatus", `README.md`, `Doku.md`, `DECISIONS.MD` gegen tatsächlichen Code-/Feature-Stand (was ist wirklich implementiert/getestet/deploybar vs. nur dokumentiert oder umgekehrt) | M | ✅ |
| AUDIT-08 | Konsolidierung: `docs/drift-audit-2026-07.md` aus AUDIT-01–07 zusammenführen, bestätigte Drifts als konkrete Tasks in `tasks/backlog.md` aufnehmen, Sprint-Dokumentation abschließen | S | ✅ |

## Ergebnisse

**AUDIT-01 — Vision-Drift (`docs/vision.md`) ✅**

Abgleich der fünf im Task genannten Abschnitte gegen den tatsächlichen Code-/Doku-Zustand
(`docs/requirements.md`, `tasks/backlog.md`, `CONTEXT.MD`, Frontend/Backend-Struktur). Keine
Kritisch-Befunde (kein Sicherheits-/Safety-Verhalten, das Doku und Code widersprüchlich
beschreiben — die bestehende Safety-Architektur des Direct-Teleop-Fundaments ist unangetastet und
konsistent dokumentiert).

**Mittel-Befunde:**

1. **§11 „Offene strategische Fragen" veraltet:** vision.md (Stand 2026-07-10) führt „Projektstatus:
   Ist die Ausschreibung bereits beauftragt...?" weiterhin als offene Grill-Me-Frage.
   `docs/requirements.md` §„Projektstatus" (Zeile 134-136) beantwortet das bereits seit 2026-07-13
   („Ausschreibung ist beauftragt, nicht mehr Angebotsstadium") — drei Tage nach vision.md-Stand,
   aber seither nicht zurückgespiegelt/verlinkt. Analog zur Sprint-Definition „ADR beschreibt
   überholten Zustand, nicht verlinkt".
2. **§7 Grundprinzip „Interface over Implementation" nur teilweise erfüllt:** bereits durch die
   eigene ADR-031-Bestandsaufnahme (`CONTEXT.MD`, Offene Fragen, Eintrag vom 2026-07-16) belegt:
   `safety-service`s Bus-Interface (ADR-002) existiert nicht als Go-Interface-Typ, `SessionRecorder`
   (ADR-005) existiert, wird aber nirgends als Interface-Typ verwendet; `fleetgateway` (ADR-027) ist
   der einzige Service-Port, der das Prinzip tatsächlich durchsetzt. vision.md §7 nennt das Prinzip
   uneingeschränkt als Architektur-Leitprinzip — kein Hinweis auf diese Lücke. Kein neuer Fund,
   aber hiermit explizit auch als Vision-Drift verortet (bisher nur in `CONTEXT.MD`/ADR-031
   sichtbar).
3. **§5/§10/§4 „Admin-Konsole" (AP3) faktisch nicht begonnen, aber nicht als solches
   gekennzeichnet:** vision.md führt AP3 (Nutzer-, Fahrzeug-, Systemkonfigurationsverwaltung,
   System-Gesundheit) als eines von drei Erfolgskriterien-definierenden Meilensteinen (§10) und in
   §4/§5 explizit auf. Im Unterschied zu AP1 und AP2, die beide ein eigenes `## EPIC:`-Kapitel in
   `tasks/backlog.md` mit Task-Liste und offenen Punkten haben, existiert **kein AP3-Epic** in
   `tasks/backlog.md` — keine Tasks, keine offenen Fragen dokumentiert. Im Code existiert nur
   `frontend/src/components/UserManagementPanel.tsx` (in `App.tsx` eingebunden) — dieses Panel
   stammt jedoch nachweislich aus der Zeit vor dem Fleet-Pivot (`git log --follow` zeigt nur den
   initialen Commit, keine ADR-027/028/029-Historie) und deckt nur einfache Nutzerkontenverwaltung
   ab, nicht Rollen/Rechte/Activity-Monitoring (vision.md §5) oder Fahrzeugregistrierung/-
   konfiguration, Zonenzuweisung-UI, Maintenance-Tracking, System-Health/Service-Status (alle
   ebenfalls vision.md §5 AP3-Bullets). `tasks/backlog.md` DOC-01 hat dieses Panel bereits als
   veraltet in `frontend/README.md` dokumentiert vermerkt, aber ohne AP3-Bezug. Empfehlung für
   AUDIT-08/Backlog: eigenes `## EPIC: AP3` analog AP1/AP2 anlegen, damit der Meilenstein-Status
   ehrlich sichtbar ist (aktuell: **0 % begonnen**, nicht nur „lückenhaft").

**Kein Drift festgestellt (geprüft, konsistent):**

- §6 Nicht-Ziele (kein Consumer-Produkt, kein Offline-System, kein rein lokales Embedded-System
  ohne Cloud/Netzwerk) — `infrastructure/AWS/cdk_server-stack.ts` (EC2/VPC/SG) und
  `docs/architecture.md` Zeile 430 („Keine Kubernetes-Abhängigkeit") bestätigen konsistent
  Docker-Compose-auf-AWS-EC2, kein Widerspruch zu `CONTEXT.MD`s „Deployment: Docker Compose (kein
  Kubernetes)".
- §10 „Sicherheitsmechanismen... sowohl auf Flotten- als auch auf Einzelfahrzeugebene": geprüft,
  `fleet-service` enthält keinerlei Safety-/Emergency-/Deadman-Logik (nur CRUD für
  Zones/Stations/Tasks/Alerts) — aber das ist **kein neuer Drift**, da vision.md §11 selbst das
  zugrunde liegende „Sicherheitskonzept für das Betriebsgelände" bereits explizit als offene,
  unentschiedene Frage führt. Erfolgskriterium ist aspirational und bewusst an eine noch
  ausstehende Entscheidung gekoppelt, keine Fehlbehauptung über den Ist-Zustand.
- §8 Qualitätsziele „klare Trennung von Sicherheits-/Steuerungssystemen" — `fleet-service` bleibt
  als separater, nicht-safety-kritischer Dienst von `control-server` getrennt, konsistent mit
  Hub-Hierarchie (ADR-007).

**Niedrig:** keine kosmetischen Befunde mit Entscheidungsrelevanz gefunden.

---

**AUDIT-02 — ADR-Gesamtkonsistenz (alle 32 ADRs in `docs/adr/`) ✅**

Systematisch geprüft: `docs/adr/README.md` als Index, danach alle 32 ADR-Dateien (001–031, inkl.
012b) vollständig gelesen und jeweils gegen Code (grep/Datei-Existenz) sowie gegen
`docs/architecture.md`/`docs/requirements.md`/`CONTEXT.MD`/`DECISIONS.MD` abgeglichen. Arbeitsteilung
auf drei parallele Rechercheagenten (ADR 001–010 / 011–020 / 021–031), Bewertung/Schweregrad und
die sicherheitsrelevante Kernaussage unten selbst nachverifiziert (nicht nur aus Agentenberichten
übernommen).

**Kritisch (1 Befund — sofort hervorgehoben):**

1. **ADR-009 CRITICAL-Trigger „Auth Invalidation" ohne Produktivimplementierung.** `docs/adr/009-failure-model.md`
   Zeile 21 sowie `CONTEXT.MD` „Failure Classification" (Zeile 100) behaupten: JWT-Widerruf während
   einer laufenden Session löst `Auto-Stop → SAFE_MODE` aus („Implementierung: Auth Service,
   ADR-004"). Tatsächlich existiert dafür **kein Produktivpfad**: `EventAuthInvalid`
   (`internal/safetyservice/bus.go:22`) wird ausschließlich in `tests/unit/safety_test.go:183/189`
   synthetisch erzeugt — keine Stelle in `internal/authservice/` oder `cmd/auth-service/` erkennt
   einen widerrufenen/invalidierten Token und löst das Event aus. `internal/controlserver/transport/
   websocket.go:251` validiert das JWT nur einmalig beim WS-Handshake (`jwt.ParseWithClaims`), danach
   keine erneute Prüfung/kein Revocation-Mechanismus während der laufenden Verbindung. Selbst
   nachverifiziert (nicht nur Agentenbefund): `grep -rn "revoke" -i internal/authservice/
   cmd/auth-service/` → keine Treffer; `grep -n "ExpiresAt\|Valid()" internal/controlserver/
   transport/websocket.go` → keine Treffer. **Sicherheitsrelevant**, da `CLAUDE.MD` §13 und
   `CONTEXT.MD`s Failure Classification (Core-Load-Dokument) dieses Verhalten als bestehende
   Sicherheitsgarantie darstellen, die real nicht besteht. Relevant auch für AUDIT-05
   (Sicherheitsregeln-Audit) — dort erneut aufgreifen.

**Mittel (8 Befunde):**

2. **ADR-018 (Audit Trail, SQLite WAL) vs. ADR-023 (PostgreSQL-Migration) — unverlinkter
   Widerspruch.** `docs/adr/023-postgresql-migration.md` Zeile 23: „PostgreSQL 16 ersetzt SQLite
   vollständig als einzige Datenbankinfrastruktur", Zeile 81 verwirft explizit die Alternative
   „SQLite für Audit behalten" („Safety-kritischer Pfad bleibt auf SQLite, kein Gewinn"). ADR-018
   bleibt aber `Status: Accepted`, referenziert ADR-023 nirgends, Volltext beschreibt weiterhin
   SQLite/`modernc.org/sqlite` als aktuell. Code bestätigt die Migration (`pkg/audit/postgres_writer.go`,
   kein `sqlite`-Treffer in `go.mod`/Repo) — die Safety-Garantie selbst (fsync vor SAFE_MODE) scheint
   über `synchronous_commit=on` erhalten, aber ADR-018 ist unverändert als „Accepted" markiert ohne
   Supersession-Hinweis. Exaktes Beispiel für die im Sprint-Text genannte Mittel-Definition.
3. **ADR-011 (System State Machine) ohne Verweis auf ADR-026 (Per-Vehicle-Isolation).** Volltext
   (232 Zeilen) beschreibt nur den ursprünglichen globalen Singleton-Ansatz; einziger
   Status-Zusatz ist „erweitert durch ADR-014" (Zeile 3), kein Hinweis auf ADR-026, obwohl
   `docs/architecture.md:149` und `CONTEXT.MD:61/63` den Per-Fahrzeug-Wechsel bereits korrekt
   dokumentieren. Die „lebenden" Dokumente sind aktuell, das ADR selbst (laut `CLAUDE.MD` §3
   ranghöher als `architecture.md`/`CONTEXT.MD`) ist es nicht.
4. **ADR-022 (Vehicle Registry, SQLite) ohne Verweis auf ADR-023.** Beschreibt durchgängig
   `avoc_audit.db`/SQLite als aktuell, keine Erwähnung von PostgreSQL/ADR-023 im gesamten Text.
   Code (`internal/vehicleregistry/postgres_store.go:12`) nutzt bereits vollständig Postgres.
   `DECISIONS.MD:29` hat die Diskrepanz bereits stillschweigend geglättet („SQLite/Postgres"), aber
   nur dort — nicht im ADR selbst.
5. **ADR-021 (Vehicle Connectivity) ohne Hinweis auf `vehicle-mock`s Doppelrolle.** Beschreibt
   `vehicle-mock` nur als Single-Vehicle-Direct-Teleop-Mock. Seit Sprint 21 (ADR-027/029) simuliert
   derselbe Service zusätzlich eine Multi-Vehicle-Flotte (`cmd/vehicle-mock/fleet_simulator.go`,
   `lastenzug`/`lastenrad`) — im Code sauber kommentiert (`main.go:67-69`, referenziert ADR-027/029),
   aber ADR-021 selbst wurde nie um einen Hinweis ergänzt.
6. **ADR-025 (`vehicleController`-Map) und ADR-026 (`vehiclecontext.Registry`) ohne
   Cross-Referenz.** Kein Rename, echte Koexistenz zweier verschiedener Mechanismen
   (Control-Locking vs. Safety-Isolation) — beide korrekt im Code vorhanden und funktional. Risiko:
   ohne Cross-Referenz entsteht leicht die falsche Annahme, ADR-026 habe ADR-025 abgelöst.
7. **ADR-026 eigene Behauptung „`GET /state` entfällt als Polling-Endpunkt" (Zeile 85)
   stimmt nicht mit dem Code überein** — `cmd/control-server/main.go:643` behält den Endpoint
   bewusst als Compat-Shim (Kommentar: „kept for backward compat"). Der Ist-Zustand ist bereits
   korrekt in `architecture.md`/`DECISIONS.MD` (Backlog-Punkt MV-12) dokumentiert — nur ADR-026s
   eigener Text wurde nie korrigiert.
8. **`DECISIONS.MD` Status-Spalte für ADR-014 inkonsistent zur ADR-Datei selbst.**
   `DECISIONS.MD:21` führt ADR-014 als „Accepted", während `docs/adr/014-video-streaming.md:3`
   selbst korrekt „Status: Superseded by ADR-020" trägt (inkl. sauberem In-File-Hinweis,
   Zeilen 5-8 — ADR-014 selbst macht also alles richtig, nur die Live-Übersicht zieht es nicht nach).
9. **ADR-012b („generierter Protobuf-Code wird per `.gitignore` ausgeschlossen") stimmt nicht.**
   `gen/go/*.pb.go` sind tatsächlich in Git getrackt (`git ls-files gen/` liefert Treffer), keine
   `gen/`-Regel in `.gitignore`. Kein Sicherheitsrisiko, aber Widerspruch zur „Single Source of
   Truth ist `.proto`, Code wird zur Build-Zeit generiert"-Entscheidung (ADR-008/012b).

**Niedrig (5 Befunde, kosmetisch/ohne Entscheidungsrelevanz):**

10. `docs/adr/README.md` Zeile 9: Kopfzeile „ADR-Index (29 ADRs)" — tatsächlich 32 Tabellenzeilen
    (`grep -c "^| \[ADR-" docs/adr/README.md` → 32).
11. ADR-002s Pseudocode-Methodennamen/-Signaturen sind vom Code abgedriftet
    (`SubscribeSafetyEvents` vs. tatsächlich `Subscribe`; `TriggerEmergencyStop(reason)` vs.
    tatsächlich 3 Parameter) — stützt den bereits bekannten „Prinzip 6 lückenhaft"-Befund
    (`CONTEXT.MD` Offene Fragen, ADR-031-Bestandsaufnahme), kein neues Sicherheitsproblem, da der
    `Bus` weiterhin korrekt funktioniert.
12. ADR-005s `SessionRecorder`-Interface wird nirgends als Interface-Typ verwendet (nur
    `*MemoryRecorder`-Konkrettyp) — ebenfalls bereits bekannter Prinzip-6-Befund, hier nur mit
    zusätzlicher Codestelle (`cmd/control-server/main.go:102`) belegt.
13. ADR-009 nennt falsche Dateipfade für zwei von neun Triggern (`safety/deadman.go` existiert
    nicht, tatsächlich `safety/detector.go:30`; „No Active Operator" tatsächlich in
    `statemachine/state.go:182`, nicht `session/manager.go`) sowie eine falsche Testanzahl („18
    Unit-Tests" vs. tatsächlich 20 in `tests/unit/watchdog_test.go`) — Verhalten korrekt
    implementiert, nur die Pfad-/Zahlangaben im ADR-Text sind veraltet.
14. ADR-017s `AuditWriter`-Interface-Text nennt nicht die zusätzliche Methode `QueryBySession`, die
    im Code (`pkg/audit/writer.go`) bereits existiert — additive, folgenlose Abweichung.

**Kein Drift festgestellt (geprüft, konsistent):**

- ADR-014 → ADR-020 Supersession ist sauber in beiden Richtungen dokumentiert (ADR-014 trägt
  „Superseded by ADR-020" inkl. Erklärungstext, ADR-020 erklärt umgekehrt das Verhältnis).
- ADR-006s „Update (2026-07-15)"-Zusatz zu `CLAUDE.MD` §17 ist wortgleich sowohl im ADR selbst als
  auch in `DECISIONS.MD` vorhanden — sauber dupliziert, kein Drift.
- ADR-028/ADR-029 nutzen konsequent datierte In-File-Updates statt neuer ADR-Nummern für
  Folgeentscheidungen am selben Thema (`## Update (2026-07-15)` bzw. „Update 2026-07-13/2026-07-16")
  — entspricht dem in `CLAUDE.MD` §6 vorgesehenen Muster.
- ADR-030/ADR-031-Nummernkollision (siehe `DECISIONS.MD` Historie): keine Rückstände in den
  Dateien selbst geprüft — keine falschen Binnenverweise gefunden.
- ADR-027s „Schnittstellenparameter vorläufig"-Status ist weiterhin akkurat — AP1-Workshop mit der
  Professur Logistik laut `tasks/backlog.md` (`AP1-01`) noch nicht durchgeführt.
- ADR-019s Abweichung bei der Image-Benennung (kein `docker.io/`-Präfix) ist bereits im
  `docker-compose.prod.yml` selbst dokumentiert (Verweis auf
  `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`) — kein verstecktes Drift.

---

**AUDIT-03 — Coding Guidelines (`docs/code-patterns.md` + CLAUDE.MD §12) ✅**

Stichproben: `internal/controlserver/{statemachine,safety,command,transport,session}` (Core-Teleop),
`internal/controlserver/vehiclecontext` (ADR-026), `internal/fleetservice/{handler,store,
alertengine,broadcast}.go` (Fleet), `internal/authservice/handler.go`, sowie
`frontend/src/hooks/{useDeadmanSwitch,useControls}.ts`, `frontend/src/lib/fleet-alert-sound.ts`.
Kein Kritisch-Befund — die gefundenen Stilverstöße sind Tech-Debt, keiner widerlegt eine
dokumentierte Safety-Garantie.

**Mittel:**

1. **Magic Values — `OperatorRole` als Rohstring statt Typ, obwohl der Typ bereits existiert.**
   `internal/authservice/handler.go:15-20` definiert korrekt `type OperatorRole string` mit
   `RoleActiveOperator`/`RoleObserver`-Konstanten (genau das von CLAUDE.MD §12 geforderte Pattern).
   `internal/controlserver/session/manager.go:18` deklariert `OperatorRole string` erneut lose und
   literalisiert `"ACTIVE_OPERATOR"`/`"OBSERVER"` an über einem Dutzend Stellen (u. a. Zeilen 57,
   60, 71, 75, 98, 111, 138, 154, 176, 207, 278); `command/engine.go:136` und
   `transport/websocket.go:100` vergleichen ebenfalls gegen den Rohstring `"OBSERVER"`. Ein
   Tippfehler in einem dieser Literale würde die ADR-025-Rollenprüfung (OBSERVERs dürfen keine
   Bewegungsbefehle senden) lautlos aushebeln — Verstoß gegen CLAUDE.MD §12, kein Doku-Drift.
2. **Magic Values — Task-Status als Rohstrings in `fleetservice`.** `internal/fleetservice/
   store.go:345-348` (`taskTransitionSources`-Map), Zeile 387, Zeile 301 verwenden
   `"pending"/"in_progress"/"completed"/"cancelled"` als Strings statt eines benannten Typs — im
   Gegensatz zum vorbildlichen, typisierten Muster in
   `internal/controlserver/statemachine/state.go:17-58`, das `docs/code-patterns.md` §1 selbst als
   Referenzmuster zeigt. Verstoß, kein Drift.
3. **`docs/code-patterns.md` §1–3 veraltet gegenüber ADR-026 (Per-Vehicle-Isolation).** Die Doku
   zeigt `Machine`/`DeadmanWatchdog`/`Engine` noch mit einzelnen injizierten Feldern (Singleton-
   Pattern, Zeilen 29-45, 83-104, 144-175). Der reale Code
   (`internal/controlserver/vehiclecontext/registry.go:1-6,25-33`,
   `internal/controlserver/command/engine.go:39-46,91,95,106,122`) wurde per ADR-026 bereits auf
   eine `Registry` mit einem `VehicleContext` pro Fahrzeug umgestellt — die Doku lehrt aktuell das
   überholte, schlechtere Singleton-Pattern (verursachte laut Code-Kommentar in `registry.go:3-6`
   genau das Bug-Muster "zwei Operatoren auf zwei Fahrzeugen überschreiben sich"). Irreführend,
   aber nicht sicherheitsrelevant.
4. **`docs/code-patterns.md` §3 unterschlägt sicherheitsrelevante Zweige des echten
   `Engine.Handle`.** Der reale Code (`engine.go:104-120,135-138`) schreibt vor dem
   SAFE_MODE-Übergang synchron einen Audit-Eintrag (ADR-018) und blockt OBSERVER-Rollen explizit
   (ADR-025) — beides fehlt im Doku-Snippet. Unvollständige, nicht falsche Doku.
5. **Stale Kommentar in `frontend/src/hooks/useDeadmanSwitch.ts:34`:** nennt 2s
   Watchdog-Timeout (ADR-009-Verweis), tatsächlicher Server-Timeout ist 10s
   (`internal/controlserver/safety/detector.go:17`, `DefaultDeadmanTimeout = 10 * time.Second`,
   korrekt auch in `docs/code-patterns.md:397`). Nur der Code-Kommentar ist der Ausreißer — keine
   funktionale Auswirkung (Wert wird server-seitig durchgesetzt), aber irreführend.

**Niedrig:**

6. **Magic-Number-Fallbacks in `frontend/src/hooks/useControls.ts:99-101,105-106,121-122,
   137-138,142-143`:** `CommandType?.STEER ?? 1` usw. spiegeln Proto-Enum-Ordinalzahlen
   (`STEER=1, THROTTLE=2, BRAKE=3`, `proto/control.proto:340-342`) hart codiert, ohne Konstante —
   nur ein Edge-Case-Fallback-Pfad, aber ein Umsortieren der Proto-Enum würde dies lautlos
   brechen.

**Kein Drift festgestellt (geprüft, konsistent):**

- `internal/controlserver/statemachine/state.go:14-58` — vorbildliche typisierte
  Zustandskonstanten, exakt das CLAUDE.MD-§12-Pattern gegen Magic Values.
- `internal/controlserver/safety/detector.go:16-20` — alle Timeout-Werte als benannte
  `Default*`-Konstanten.
- `internal/controlserver/command/engine.go:26,45,57,176-208` — Rate-Limit als benannte
  Konstante, Token-Bucket ohne externe Dependency (KISS).
- `internal/fleetservice/alertengine.go:8-24` — Batterie-Tier-Schwellenwerte als benannte,
  kommentierte Konstanten inkl. Hysterese-Begründung.
- `internal/fleetservice/handler.go:21-26,31-36` — `Dispatcher`-Interface als schmale, explizite
  Abhängigkeit — entspricht CLAUDE.MD §12 „explizite Abhängigkeiten".
- `frontend/src/lib/fleet-alert-sound.ts:1-70` — reine, getestete Entscheidungslogik getrennt von
  React/WebSocket-State, mit expliziten Minimal-Interfaces statt voller DOM-Typen.

---

**AUDIT-04 — Qualitätsziele (vision.md §8, Teststandard CLAUDE.MD §17) ✅**

Geprüft: `vision.md` §8 gegen `CLAUDE.MD` §17, Stichproben in `internal/fleetservice/*_test.go`
(~60 Testfunktionen), `internal/fleetgateway/*_test.go`, `tests/unit/safety_test.go`/
`watchdog_test.go` (Core-Teleop), `frontend/src/**/*.test.ts(x)` (25 Dateien), sowie
`docs/architecture.md` Container-/Komponenten-Tabellen gegen `docker-compose.yml` und
`current-sprint.md`/`backlog.md` auf unverfolgte "bewusst nicht getestet"-Lücken.

Kein Kritisch-Fund (die einzige sicherheitsrelevante Testlücke, `EventAuthInvalid` ohne
Produktivpfad, ist bereits unter AUDIT-02 dokumentiert, kein neuer Fund hier).

**Mittel:**

1. **`docs/architecture.md` wiederholt genau die Sprint-23-Lücke, die es angeblich behoben hat.**
   Der Sprint-23-DoD-Nachtrag ergänzte eine Frontend-Komponenten-Tabelle "Fleet Overview Dashboard
   (Sprint 22–23)" (`docs/architecture.md:274-283`). Seither nicht fortgeführt: `FleetTaskPanel.tsx`
   (Sprint 24) und `fleet-alert-sound.ts`/`useFleetAlertSound.ts` (Sprint 25) — beide mit eigenen
   Testdateien — fehlen komplett. Der Sprint-23-Fix war ein Einmal-Catch-up, keine etablierte
   Praxis.
2. **`postgres`-Container fehlt vollständig in der Container-Services-Tabelle.**
   `docs/architecture.md:434-449` listet 14 Services (inkl. `fleet-service`/`vehicle-mock` seit dem
   Sprint-23-Fix), aber der tatsächlich laufende `postgres:16-alpine`-Container
   (`docker-compose.yml:23-24`, zentrale Datenhaltung) taucht dort an keiner Stelle als eigene
   Zeile auf — nur beiläufig im Fließtext (Zeile 441). Sprint-23-Fix hat den Drift nur teilweise
   behoben.
3. **§17-Nebenläufigkeits-Vorgabe (Race-Detector) nirgends strukturell verankert.** Keine
   CI-Konfiguration im Repo; `Makefile:59-89` ruft `-race` an keiner Stelle auf. Die zahlreichen
   `-race`-Läufe, die §17 verlangt und die in Sprint-Notizen dokumentiert sind, passieren
   ausschließlich manuell/ad-hoc pro Entwickler-Session — ein Entwickler, der nur `make test`
   ausführt, bekommt nie eine Race-Prüfung. Teststandard wird gelebt, aber nicht erzwungen.
4. **Dokumentierte Testlücke ohne Backlog-Folge-Task.** `store.CreateAlert`-Fehlerpfad nach
   erfolgreichem `AlertEngine.Evaluate()` ist bewusst ungetestet (FLEET-07-Nachtrag) — in
   `tasks/backlog.md` existiert dazu kein Eintrag, obwohl zweimal in Sprint-Notizen vermerkt.

**Niedrig:**

5. ADR-006 Teil 1 schreibt `testing`+`testify` fest; `internal/fleetservice`-Tests nutzen
   durchgängig reines `testing`-Stdlib (bewusst dokumentierte Abweichung) — ADR selbst nicht
   nachgezogen.

**Kein Drift festgestellt:**

- `internal/fleetservice`-Tests decken die §17-Fallgruppen tatsächlich vollständig ab: Grenzwerte,
  fehlerhafte Eingaben, Fehlerpfade, Idempotenz, Zustandsübergänge, Nebenläufigkeit (dokumentiert,
  wenn auch nicht automatisiert), Zugriffsgrenzen. Integrationstests gegen echte Postgres/MQTT
  vorhanden, 2–3x-Wiederholung gegen Flakiness durchgängig dokumentiert.
- §8 "klare Trennung von Sicherheits-/Steuerungssystemen" bereits unter AUDIT-01 bestätigt, hier
  erneut verifiziert.
- Core-Teleop-Safety-Tests decken Grenzwerte/Zustandsübergänge/Nebenläufigkeit breit ab.

---

**AUDIT-05 — Sicherheitsregeln (CLAUDE.MD §13, CONTEXT.MD Safety/Invarianten, requirements.md Safety Requirements) ✅**

Geprüft: `CLAUDE.MD` §13, `CONTEXT.MD` (Safety Concepts, 4-Layer State Machine, Failure
Classification, Invarianten 1–3, Prinzipien 1/7/8/11/13), `docs/requirements.md` „Safety
Requirements"/„State Machine Requirements" gegen `internal/controlserver/statemachine/state.go`,
`internal/controlserver/safety/{detector.go,bus_watchdog.go}`,
`internal/controlserver/vehiclecontext/registry.go`, `internal/controlserver/transport/
websocket.go`, `cmd/control-server/main.go`, `internal/authservice/`, `internal/fleetservice/`,
`internal/fleetgateway/`. Jede Failure-Classification-Zeile einzeln per grep auf Produktivpfad
geprüft (nicht nur Tests).

**Kritisch (2 weitere Befunde, zusätzlich zum AUDIT-02-Fund zu ADR-009/Auth Invalidation):**

1. **„No Active Operator" (OPERATOR_STATE=NO_OPERATOR) hat keinen Produktivpfad.**
   `docs/adr/009-failure-model.md:25`, `CONTEXT.MD:70/81/100` und `docs/requirements.md:191/217`
   behaupten den CRITICAL-Trigger „OPERATOR_STATE = NO_OPERATOR → SAFE_MODE", ADR-009 nennt
   explizit `session/manager.go` als Implementierung. Tatsächlich ruft `TransitionOperator
   (statemachine.OpNoOperator)` — der einzige Pfad, der den SAFE_MODE-Zweig in `state.go:187-192`
   auslösen könnte — im gesamten Produktivcode niemand auf; einziger Aufrufer ist
   `tests/unit/safety_test.go:150`. Produktiv wird `TransitionOperator` nur mit
   `OpActive`/`OpHandoverPending` aufgerufen (`websocket.go:119`, `session/handover.go:66/93/114`).
   Der praktische Sicherheits-Outcome wird zwar über einen anderen Mechanismus erreicht
   (WS-Disconnect-Handler `websocket.go:177` transitioniert SYSTEM STATE direkt, ohne den
   OPERATOR-Layer zu berühren) — die dokumentierte „4 unabhängige Layer"-Architektur stimmt für den
   OPERATOR-Layer real nicht: er bleibt nach Disconnect bei `ACTIVE_OPERATOR` hängen, obwohl SYSTEM
   bereits SAFE_MODE ist. Die dokumentierte, eigenständige Safety-Invariante funktioniert nur durch
   Redundanz (WS-Disconnect) — fällt diese in einem anderen Szenario weg (z. B. Operator-Session-
   Ende ohne WS-Close), gibt es keinen Fallback.
2. **DEGRADED-Tier (Media/Video/Telemetrie) komplett unverdrahtet.** `CONTEXT.MD:101`,
   `docs/requirements.md` (State Machine Requirements MEDIA STATE) und ADR-009 (DEGRADED-Tabelle:
   Video Stream Lost, Qualitätsverlust, Secondary Camera Failure, Partial Telemetry Loss)
   behaupten, Video-/Telemetrie-Ausfälle führten zu `SYSTEM DEGRADED`. Einzige Aufrufer von
   `TransitionMedia(...)` sind `tests/unit/safety_test.go:214/227` — kein Treffer in
   `internal/webrtcsfu`, `internal/mediamtx` oder sonst im Produktivcode für
   `MediaFailed`/`MediaDegraded`. Die MEDIA-Schicht verharrt in Produktion dauerhaft bei
   `MEDIA_INIT` — DEGRADED wird nie durch echte Video-/Telemetrie-Fehler ausgelöst. Invarianten 1/2
   sind zwar knapp trivial erfüllt (nichts triggert überhaupt etwas), aber die dokumentierte
   Warnfunktion im UI bei Videoverlust hat keine Backend-Grundlage. Gleiches Muster wie
   AUDIT-02/Fund 1: Doku/ADR beschreibt eine Sicherheitsmechanik, die nur in Unit-Tests existiert.

**Mittel:**

3. **OBSERVATION-Trigger „Auth Service down → neue Sessions blockiert" ohne Implementierung.**
   `CONTEXT.MD:102`, ADR-009. `POST /session/start` (`cmd/control-server/main.go:153-209`) prüft
   nur JWT-Rolle und Vehicle-Registry-Connected-Status, ruft nirgends einen
   Auth-Service-Health-Endpoint auf. „JWT-Validierung lokal weiter" stimmt, „neue Sessions
   blockiert" ist unimplementiert. Niedrigste Failure-Klasse (kein Auto-Stop betroffen), daher
   Mittel statt Kritisch.
4. **CONTEXT.MD Failure-Classification-Tabelle listet „Vehicle ACK Timeout" nicht separat.**
   ADR-009 und Code (`ACKTimeoutWatcher` vs. `VehicleACKWatchdog`, unterschiedliche Defaults
   100ms/1s, unterschiedliche Trigger-Quelle) unterscheiden zwei eigenständige
   CRITICAL-Mechanismen; `CONTEXT.MD:100` fasst beide fälschlich unter „Command ACK Timeout"
   zusammen. Dokumentationslücke, kein Verhaltens-Drift (Code deckt real beide Fälle ab).
5. **ADR-011 „System State Machine" fehlt Verweis auf ADR-026 auch in `requirements.md`**
   (Zeile 200-217) — beschreibt nur die globale Variante, ohne Per-Vehicle-Hinweis, obwohl
   `vehiclecontext/registry.go` das korrekt pro Fahrzeug implementiert. Gleicher Drift wie
   AUDIT-02 Fund #3, hier zusätzlich in `requirements.md` sichtbar.

**Kein Drift festgestellt (geprüft, konsistent):**

- Invariante 1/2 (Media darf SAFE_MODE nie triggern): `state.go:170-180` mappt
  MEDIA_FAILED/DEGRADED ausschließlich auf `StateDegraded`, kein Pfad zu `StateSafeMode` —
  Code-seitig korrekt, auch wenn nie aufgerufen.
- Invariante 3 (Control Hub = Single Source of Truth): GSA-Modell bestätigt, keine parallele
  Zustandsquelle gefunden.
- Dead-man Switch, Command ACK Timeout, Vehicle ACK Timeout, Safety Bus Failure, Emergency Stop,
  WS Disconnect: alle mit echtem Produktivpfad, `WriteSync`-Audit vor Transition (ADR-018),
  korrektem `TransitionSystem(StateSafeMode)`-Aufruf verifiziert.
- Per-Vehicle Safety Isolation (Prinzip 13/ADR-026): `VehicleContextRegistry` isoliert
  SM/Deadman/ACKTimeoutWatcher/VehicleACKWatchdog korrekt pro Fahrzeug; `SafetyBusWatchdog` bleibt
  bewusst global, fächert korrekt auf alle aktiven Fahrzeuge auf — exakt wie dokumentiert.
- Fleet-Service-Isolation: keine Treffer für SAFE_MODE/State-Machine-Referenzen in
  `internal/fleetservice`/`internal/fleetgateway` — CONTEXT.MDs Behauptung „fleet-service nicht
  safety-kritisch" bestätigt.
- CLAUDE.MD §13 Stichproben: JWT-Secret/TURN-Credentials via `os.Getenv` (kein Hardcoding), keine
  Passwort-Klartext-Logs in `authservice/handler.go` gefunden.

**Zusammenfassung für AUDIT-08:** Insgesamt bestehen damit **drei** von neun dokumentierten
CRITICAL/DEGRADED-Failure-Classification-Zeilen ohne echten Produktivpfad (Auth Invalidation —
AUDIT-02, No Active Operator, Media-DEGRADED-Wiring — beide AUDIT-05). Gemeinsamer Backlog-Task
empfohlen: „Failure-Classification-Zeilen ohne Produktivpfad schließen oder Doku korrigieren".

---

**AUDIT-06 — Performance-Ziele (requirements.md/architecture.md Latenzziele, CONTEXT.MD Control Loop <100ms) ✅**

**Kritisch:**

1. **„CI Build-Fail" existiert nicht als automatisierte Pipeline.** `docs/requirements.md:280`,
   `docs/architecture.md:313` und `CONTEXT.MD:204,248` behaupten übereinstimmend, das <100ms-Ziel
   führe zu einem „CI Build-Fail" bei Verletzung. Im Repo existiert keine CI-Konfiguration (kein
   `.github/workflows/`, kein `.gitlab-ci.yml`, kein Jenkinsfile — geprüft per Suche, keine
   Treffer). Die Latenztests sind ausschließlich manuelle Makefile-Targets (`Makefile:79`
   `test-latency`, `Makefile:91` `test-k6`). Ohne CI-Anbindung gibt es keinen automatischen
   Build-Fail-Mechanismus — die Doku suggeriert eine Absicherung, die faktisch nicht greift.
2. **Der reale WS-ACK-Roundtrip-Test läuft im Default-Testlauf gar nicht.**
   `tests/performance/latency_test.go:58` (`BenchmarkControlACKRoundtrip`) ist ein Go-*Benchmark*,
   kein `Test` — der Standard-Target `make test` (`go test ./...`) führt ihn nicht aus, nur
   `test-latency` (`Makefile:84`), das nirgends automatisiert getriggert wird (s. o.). Zusätzlich
   lässt `b.Skip(...)` bei fehlendem Auth-Service/WebSocket (`latency_test.go:61,67`) den Benchmark
   bei fehlender Infrastruktur klaglos „grün" durchlaufen, ohne real gemessen zu haben.
3. **`TestLatencyBudget_DocumentedRequirement` (`latency_test.go:117-121`) ist tautologisch** —
   prüft nur, ob die im selben File hartkodierte Konstante `latencyBudget` gleich `100ms` ist
   (immer wahr), keine Aussage über tatsächliche Latenz. Risiko: kann fälschlich als Beleg für
   laufende Latenzüberwachung gelesen werden.

**Mittel:**

1. **k6-Test misst nicht den dokumentierten Control-Loop-Roundtrip.** `tests/performance/latency.js`
   behauptet „Control Loop ACK Latency" zu testen (Schwelle `p(99)<100`), misst laut eigenem
   Kommentar aber `GET /state` als „HTTP proxy for ACK latency" (Zeilen 46-51) — nicht den in
   CONTEXT.MD:204 definierten Messpunkt über den WebSocket-Kanal. Schwellwert stimmt, gemessenes
   Signal ist ein anderes als dokumentiert.
2. **Video-Latenzziel (100–300ms, `requirements.md:281`, `architecture.md:314`) hat keinen
   automatisierten Schwellwert-Test.** `useWebRTC.ts:165` liefert `videoLatencyMs` nur zur
   UI-Anzeige, `ConnectionPanel.test.tsx` prüft nur Farbcodierung anhand der Control-Loop-Schwellen
   (30/75/120ms), keine Video-QoS-Prüfung gegen 100–300ms. Als Mittel eingestuft, da laut ADR-014
   explizit kein Safety-Hartziel — aber ein dokumentiertes QoS-Ziel ohne jede Messung.

**Niedrig:** keine weiteren rein kosmetischen Abweichungen — die Zahlenwerte selbst sind über
`requirements.md`/`architecture.md`/`CONTEXT.MD` konsistent.

**Kein Drift festgestellt:** die Latenz-Zielwerte (<100ms Control, 100–300ms Video) sind
dokumentenübergreifend konsistent; der Schwellwert im Go-Benchmark und im k6-Skript stimmt
zahlenmäßig mit der Doku überein — die Kritik betrifft die fehlende CI-Einbindung und die falsche
Messgröße im k6-Fall, nicht den Schwellwert selbst.

---

**AUDIT-07 — Ist-Zustand-Abgleich (requirements.md Projektstatus, README.md, Doku.md, DECISIONS.MD) ✅**

Kein Kritisch-Befund (Fleet-Service ist laut eigenem Requirements-Text explizit
„Monitoring/Dispatch-Ebene, nicht Safety-Enforcement").

**Mittel:**

1. **`Doku.md` (EC2-Produktions-Deployment-Guide) deckt `fleet-service` überhaupt nicht ab,
   obwohl seit Sprint 21 (2026-07-13) gemerged.** `Doku.md:221`
   (`GO_SERVICES="control-server auth-service safety-service telemetry-service webrtc-sfu"`),
   `Doku.md:300` ("Sechs Go-Binaries unter `cmd/`") und die Image-Build-/Save-Listen (Zeilen
   224-237) enthalten `fleet-service` nicht — tatsächlich existieren 7 Go-Binaries. Bestätigt durch
   `infrastructure/compose/docker-compose.prod.yml`: kein `fleet-service`-Eintrag (14 Services
   total, exakt die von `Doku.md:29/69/278` behaupteten „14 Container"), während
   `infrastructure/compose/docker-compose.yml` (Dev-Stack) `fleet-service` an Zeile 152 führt.
   `Doku.md` wurde am 2026-07-10 erstellt, also vor dem Fleet-Pivot (erster Fleet-Commit
   2026-07-11) — seither nie nachgezogen. Konsequenz: die als „validiert am 2026-07-09"
   beschriebene Produktions-Deployment-Anleitung liefert real keinen lauffähigen Fleet-Stack auf
   EC2, obwohl das Feature laut `README.md:34`/`DECISIONS.MD` längst „Accepted"/gemerged ist.
2. **`README.md`-Projektstruktur-Baum (Zeilen 182-206) nennt `fleet-service` nicht**, obwohl
   `cmd/fleet-service`, `internal/fleetservice`, `internal/fleetgateway` existieren — im
   Gegensatz zur Service-Tabelle weiter oben (Zeile 34), die korrekt aktualisiert wurde. Baum ist
   damit ggü. dem gleichen Dokument veraltet.

**Niedrig:**

3. **`README.md:84`: „Vitest Component-Tests (41 Tests)" veraltet** — tatsächlich 252
   `it`/`test`-Treffer über 25 Test-Dateien, u. a. sechs neue Fleet-Test-Dateien aus Sprint 22-24.
   Kosmetisch, aber deutliche Unterrepräsentation der Testabdeckung.
4. **`docs/requirements.md` Kopfzeile („Stand: 2026-07-10") vs. eigener Abschnitt „Projektstatus"
   („Stand 2026-07-13") und weitere Inhalts-Updates bis 2026-07-16** — Datumsstempel am Kopf wird
   nicht mitgepflegt, obwohl der Inhalt laufend aktualisiert wird.

**Kein Drift festgestellt (geprüft, konsistent):**

- `DECISIONS.MD` erfüllt CLAUDE.MD §9 vollständig: ADR-027 bis ADR-031 alle mit korrektem Status
  und Datums-Updates eingetragen, passend zu `docs/adr/027…031`.
- `README.md` Service-Tabelle (Zeile 34, Port 8085) korrekt und aktuell.
- `README.md`s „Implementierungsstand & Sprint-Stand"-Abschnitt verweist bewusst nur auf
  `tasks/current-sprint.md`/`done.md`/`backlog.md` statt eigene Angaben zu duplizieren — richtiges
  Muster, hier kein Fund.
- AP3/Admin-Konsole: weder `README.md` noch `Doku.md` noch `DECISIONS.MD` behaupten einen
  Fertig-/In-Arbeit-Status (0 Treffer) — konsistent mit AUDIT-01s „0 % begonnen".
- `DECISIONS.MD` „Offene Folge-Entscheidungen"-Tabelle bildet den Sprint-23/24-Ist-Zustand
  akkurat und aktuell ab.

---

**AUDIT-08 — Konsolidierung ✅**

Alle Befunde aus AUDIT-01–07 in [`docs/drift-audit-2026-07.md`](../docs/drift-audit-2026-07.md)
konsolidiert: **6 Kritisch**, **17 Mittel Typ S**, **8 Mittel Typ M/L**, **7 Niedrig** — insgesamt
38 bestätigte Einzelbefunde. Alle sechs Kritisch-Befunde folgen demselben Muster: eine im
Core-Load-Dokument (`CONTEXT.MD`) oder einem ADR als bestehend behauptete Sicherheits- oder
Latenz-Garantie hat keinen echten Produktivpfad, sondern existiert nur in Unit-Tests/Benchmarks
(drei Failure-Classification-Trigger ohne Produktivauslöser: Auth Invalidation, No Active
Operator, Media-DEGRADED; drei Latenz-Infrastruktur-Lücken: fehlende CI, nicht mitlaufender
Benchmark, tautologischer Test).

Jeder Befund als eigene `DRIFT-*`-Zeile in `tasks/backlog.md` (neues `## EPIC: Drift-Audit-Fixes`)
aufgenommen, inkl. Schweregrad-, Typ- und Quell-Spalte. Kritisch-Befunde ausdrücklich als
„Grill-Me ausstehend" markiert, nicht als normale Backlog-Tasks — `CLAUDE.MD` §0 verbietet einen
direkten Fix. Zusätzlich `DRIFT-AP3` als strukturelle Backlog-Ergänzung (neues `## EPIC: AP3`
anlegen) vorgemerkt, kein Einzel-Drift.

**Bewusst nicht in diesem Sprint:** keine der 38 Befunde wurde behoben — reine Bestandsaufnahme
+ Konsolidierung laut Sprint-Scope-Entscheidung (Grill-Me, s. o.). Die Behebung (Grill-Me je
Kritisch-Themenblock, danach Fast-Track/volle Phasenfolge je nach Typ) ist eigener Folge-Scope.

---


## Sprint 26 (Fortsetzung) — DRIFT-K1/K2/K3: Safety-Trigger ohne Produktivpfad

Abgeschlossen: 2026-07-17

Drei Kritisch-Befunde aus dem Sprint-26-Drift-Audit (`docs/drift-audit-2026-07.md`,
AUDIT-02/AUDIT-05): ADR-009 behauptete Safety-Trigger, die nur in `tests/unit/safety_test.go`
synthetisch erzeugt wurden, ohne echten Produktivpfad. Typ L (Kernsystem/Safety-Modell) — je
Befund eigene Grill-Me-Session vor Umsetzung (§1.1/§5 CLAUDE.MD), gefolgt von einer zweiten
Grill-Me-Runde zu konkreten Implementierungs-Trade-offs (Erkennungsumfang, Watchdog-Timing,
Schwellwert-Kalibrierung, Scope-Split), da die Konsequenzen für bestehende Watchdogs/
State-Machine-Übergänge (ADR-026 Per-Vehicle-Isolation, Race mit dem WS-Disconnect-Pfad) vor
der Implementierung geklärt werden mussten. Nutzer entschied sich in allen drei Fällen für
Implementierung statt reiner Doku-Korrektur.

Zusätzlich vorgelagert, im selben Worktree/Branch: **Fast-Track (2026-07-16)** — 24 reine
Doku-Korrekturen ohne Architekturbezug (DRIFT-M01..M17, N01..N07) — ADR-Update-Blöcke,
Cross-Referenzen, zwei Code-Kommentar-/Magic-Value-Fixes. Siehe Commit `e12a6ca`.

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| DRIFT-K1 | Auth Invalidation → SAFE_MODE | L | ✅ Neuer `AuthWatchdog` (`internal/controlserver/safety/auth_watchdog.go`), per-`VehicleContext` (ADR-026), 5s×2-Poll wie `SafetyBusWatchdog`. Liest `users.is_active` direkt aus der geteilten `avoc`-DB (`internal/controlserver/authcheck`, kein neuer HTTP-Dienst). Feuert über `TransitionOperator(OpNoOperator)` |
| DRIFT-K2 | No Active Operator → SAFE_MODE | L | ✅ WS-Disconnect-Handler ruft jetzt zusätzlich `TransitionOperator(OpNoOperator)` — OPERATOR-Layer hängt nicht mehr dauerhaft bei `ACTIVE_OPERATOR`. `EventNoOperator` wird neu auf dem Safety Event Bus publiziert (`WSHandler.WithPublisher`) |
| DRIFT-K2-FIX | `TransitionOperator`-Guard-Bypass | S | ✅ Nebenbefund: interner SAFE_MODE-Zweig umging bislang `validSystemTransitions` (funktional bisher folgenlos, da Guard-Bedingung zufällig deckungsgleich). Behoben via `transitionSystemLocked`-Extraktion, jetzt von `TransitionSystem` und `TransitionOperator` gemeinsam genutzt |
| DRIFT-K3 | MEDIA_DEGRADED-Schwellwerte + Recovery | L | ✅ Audit-Prämisse korrigiert: MEDIA_FAILED war bereits verdrahtet (`useWebRTC.ts`). Neu: Paketverlust-/Bitrate-Schwellwerte aus `getStats()` (initiale, nicht feldvalidierte Werte, 3-Sample-Hysterese) → `MEDIA_DEGRADED`; `TransitionMedia` bekommt einen bislang fehlenden Rückweg DEGRADED→CONNECTED |
| DRIFT-K3-TELEMETRY | Partial Telemetry Loss | — | 🔲 Bewusst zurückgestellt, eigener Backlog-Task (`tasks/backlog.md`) — neuer Cross-Service-Watchdog gegen `telemetry-service` nötig, andere Risikoklasse als die Video-Änderung |

### Test-Ergebnis

- Backend: 8 neue `AuthWatchdog`-Tests (`watchdog_test.go`: Normalfall, Trigger, Fehlerpfad,
  Recovery-nach-1-Fehler, Stop/Restart, Nebenläufigkeit) + 8 neue `safety_test.go`-Tests
  (`TransitionOperator`-Guard-Grenzfälle, `TransitionMedia`-Recovery) + 2 neue
  `vehiclecontext_test.go`-Tests (`Registry.WithUserChecker` verdrahtet `AuthWatchdog` korrekt,
  bleibt `nil` ohne Checker). Alle Go-Pakete außer `tests/integration` zweimal hintereinander mit
  `-race -count=1` grün (keine Flakiness).
- Integration: 3 neue Tests gegen den echten Docker-Teststack, **vollständig End-to-End grün**
  (`TestIntegration_MediaDegraded_TriggersDegrade_ThenRecovers`,
  `TestIntegration_WSDisconnect_OperatorLayerReflectsNoOperator`,
  `TestIntegration_AuthWatchdog_DeletedAccount_TriggersSafeMode` — letzterer inkl. echter
  Postgres-`DELETE`-Operation und tatsächlichem SAFE_MODE-Eintritt nach ~10,8s, passend zum
  5s×2-Watchdog-Timing). Zweimal komplett frisch (`docker compose down/up --build`, `-count=1`,
  kein Go-Test-Cache) durchlaufen, beide Male stabil grün.

  **Nachträglich zwei echte Bugs beim Debuggen des ursprünglich als „Sandbox-Limitation"
  eingeordneten WS-Dial-Fehlschlags gefunden und behoben** (Nutzerfrage, ob wirklich alle Tests
  die geänderten Module abdecken, deckte auf, dass die erste Verifikation zu oberflächlich war):
  1. `/ws` verlangt zwingend `?session_id=...` (aus `POST /session/start`) — alle drei neuen Tests
     (und, bereits vorher bestehend, 3 unveränderte Alttests) dialten die WS **vor** `session/start`
     bzw. ganz ohne `session_id`, was serverseitig korrekt mit 400 abgelehnt wird und bei gorilla als
     „bad handshake" ankommt. Kein Docker-/Sandbox-Problem — per manuellem `curl`/Go-Probe gegen den
     laufenden Teststack verifiziert (101 Switching Protocols bei korrekter Reihenfolge).
  2. Zusätzlich war im Teststack nur `vehicle-int-mock` tatsächlich online (`vehicle-mock`-Service);
     jeder andere `vehicle_id` ließ `/session/start` mit 409 „vehicle not connected" scheitern —
     stillschweigend, da die Alttests den Status-Code nie prüften.
  3. Beim Beheben von (1)/(2) ein dritter, eigener Test-Bug gefunden: der neue Test-Helper
     `startSessionAndDialWS` hatte `operator_id: "admin"` hartkodiert — das ursprünglich für
     DRIFT-K1 gelöschte Test-Konto war dadurch nie das tatsächlich vom `AuthWatchdog` beobachtete
     Konto (`admin` existiert weiter), wodurch der Test 20s lang wartete, ohne dass der Watchdog je
     etwas zu melden hatte. Fix: `operator_id` als Parameter, pro Aufrufer korrekt übergeben.
  Alle drei Punkte sind Test-Infrastruktur-Bugs (2 davon bereits vor dieser Änderung in den
  unveränderten Alttests vorhanden, 1 selbst eingeführt), keine Bugs im produktiven
  DRIFT-K1/K2/K3-Code — die Fixes sind ausschließlich in `tests/integration/services_test.go`.
- Frontend: 22 neue Tests für die reine Schwellwert-/Hysterese-Logik
  (`useWebRTC.test.ts`) — keine `RTCPeerConnection`-Mock-Infrastruktur im Repo vorhanden, daher
  Extraktion in pure, exportierte Funktionen statt Hook-Integrationstest. Gesamte Suite 274
  Tests/26 Dateien, zweimal hintereinander grün, `tsc -b` sauber.

### Bewusst nicht abgedeckt

- `useWebRTC.ts`s tatsächlicher `useEffect`/`getStats()`-Polling-Code (nur die daraus extrahierte
  reine Entscheidungslogik ist getestet) — keine `RTCPeerConnection`-Mock-Infrastruktur im Repo.
- MEDIA_DEGRADED-Schwellwerte sind nicht feldvalidiert (keine reale Flotte zum Kalibrieren) —
  bewusste Grill-Me-Entscheidung, im ADR-009-Update-Block als „initial, nicht feldvalidiert"
  markiert.
- Partial Telemetry Loss — siehe `DRIFT-K3-TELEMETRY`.
- Die 3 unveränderten Alttests (`TestIntegration_SessionLifecycle_StartAndEnd`,
  `TestIntegration_MediaFailed_TriggersDegrade_NeverSafeMode`,
  `TestIntegration_EmergencyStop_TriggersSafeMode`) haben denselben WS-Dial-Bug wie oben unter (1)
  beschrieben und skippen weiterhin — außerhalb des DRIFT-K1/K2/K3-Scopes, nicht mitgefixt, aber
  jetzt als eigenständiger, verstandener Befund dokumentiert statt als vermutete Umgebungsgrenze.

### Neue/geänderte Dateien (Stand Branch-Tip, vor Merge)

- `internal/controlserver/authcheck/checker.go` — NEU
- `internal/controlserver/safety/auth_watchdog.go` — NEU
- `internal/controlserver/statemachine/state.go` — `transitionSystemLocked`-Extraktion; `TransitionMedia`-Recovery
- `internal/controlserver/transport/websocket.go` — `WithPublisher`; `TransitionOperator(OpNoOperator)` im Disconnect-Handler; `AuthWatchdog` Start/Stop
- `internal/controlserver/vehiclecontext/registry.go` — `AuthWatchdog`-Feld (optional, nil-safe), `WithUserChecker`/`WithAuthWatchdogTiming`
- `cmd/control-server/main.go` — `authcheck.NewChecker`, `WithUserChecker`, `WithPublisher`, `AuthWatchdog` Start/Stop an allen Session-Lifecycle-Stellen
- `pkg/logger/event_types.go` — `EventAuthWatchdogTriggered`
- `frontend/src/hooks/useWebRTC.ts` — `getStats()`-Schwellwertlogik als pure Funktionen; MEDIA_DEGRADED-Detection
- `frontend/src/hooks/useWebRTC.test.ts` — NEU
- `tests/unit/safety_test.go`, `tests/unit/watchdog_test.go`, `tests/unit/vehiclecontext_test.go` — neue Testfälle
- `tests/integration/services_test.go` — 3 neue Integrationstests + `getJSONListAuth`-Fix (fehlende Auth) + `startSessionAndDialWS`/`endAllSessions`-Helper (session_id-Reihenfolge, `vehicle-int-mock`, korrekte `operator_id`)
- `docs/adr/009-failure-model.md` — Update-Block (2026-07-17), Implementierung-Spalten korrigiert
- `CONTEXT.MD`, `DECISIONS.MD`, `docs/drift-audit-2026-07.md`, `tasks/backlog.md` — nachgezogen

## Merge-Nachtrag (2026-07-20)

Der Branch `feature/drift-k1-k3-safety-model` (Sprint 26 + Fast-Track +
DRIFT-K1/K2/K3) wurde erst am 2026-07-20 in `feature/fleet-service-foundation` gemerged — 33
Commits nach seinem Abzweigpunkt. Dabei mussten `AuthWatchdog`/`vehiclecontext.Registry` auf die
inzwischen (Sprint 28/29, GOSTYLE-IF-03) verschlankte `audit.SafetyAuditWriter`-Schnittstelle
umgestellt werden (statt der vollen `audit.AuditWriter`), und die DRIFT-K1/K2-Anpassungen an
`cmd/control-server/main.go`/`internal/controlserver/transport/websocket.go` mussten in die dort
inzwischen (Sprint 29, Rule 2.3/2.4) extrahierten Methoden (`handleSessionStart`,
`advanceVehicleToActiveOperator`, `handleSessionEnd`, `recoverFromSafeMode`,
`handleWSDisconnect`) übertragen werden, statt in den ursprünglichen (inzwischen aufgelösten)
Inline-Handlern in `main()`. `go build ./...`/`go vet ./...`/`go test ./...` nach dem Merge grün
(bis auf die erwartbaren `tests/integration`-Fehlschläge ohne laufenden Docker-Teststack).
