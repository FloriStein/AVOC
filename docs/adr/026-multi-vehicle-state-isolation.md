# ADR-026: Multi-Vehicle State Isolation

**Status:** Accepted
**Date:** 2026-06-14
**Sprint:** 17 (geplant)

## Context

ADR-025 führte Multi-Operator-Support ein: `session.Manager` verwaltet mehrere parallele Sessions mit `vehicleController map[string]string` (vehicleID → Controller-Session). Auf den ersten Blick erlaubt das bereits, dass Operator A Vehicle-1 steuert während Operator B gleichzeitig Vehicle-2 steuert.

> **Hinweis zur Koexistenz mit ADR-025:** Dieses ADR ersetzt nicht das Rollenmodell
> (`ACTIVE_OPERATOR`/`OBSERVER`), `session.Manager` oder die Session-Lifecycle-Entscheidungen aus
> ADR-025 — es ersetzt ausschließlich den darunterliegenden, bis dahin global-singleton
> `statemachine.Machine` durch eine Instanz pro Fahrzeug (`VehicleContextRegistry`, siehe unten).
> Beide ADRs gelten weiterhin gemeinsam.

Bei der Prüfung dieser Annahme (Nutzerfrage: *"kann Operator B Fahrzeug 2 steuern, während Operator A Fahrzeug 1 steuert?"*) zeigte sich, dass drei Safety-kritische Komponenten weiterhin als **Singletons pro Prozess** existieren, nicht pro Fahrzeug:

| Komponente | Datei | Problem |
|---|---|---|
| `statemachine.Machine` (`sm`) | `cmd/control-server/main.go:77` | EIN globaler SYSTEM/CONTROL/MEDIA/OPERATOR-State für den ganzen Prozess. SAFE_MODE für Vehicle-1 blockiert via `ControlBlocked` auch Vehicle-2. |
| `DeadmanWatchdog` | `internal/controlserver/safety/detector.go:30-38` | Felder `sessionID`, `vehicleID` werden bei jedem `Start()` überschrieben. Startet Operator B eine zweite Session, verliert Vehicle-1 lautlos seine Deadman-Überwachung. |
| `VehicleACKWatchdog` | gleiche Datei | Identisches Muster — ein Timer-Kontext pro Prozess, nicht pro Fahrzeug. |
| `SafetyBusWatchdog` | `internal/controlserver/safety/bus_watchdog.go` | Pollt EINEN globalen `/health`-Endpunkt des Safety-Service (das ist korrekt, da geteilte Infrastruktur) — `triggerSafeMode()` kennt aber nur die zuletzt gestartete `sessionID`/`vehicleID`, nicht alle aktiven Fahrzeuge. |

**Konsequenz heute:** Zwei Operatoren können über die API beide eine Session für unterschiedliche Fahrzeuge starten — sobald die zweite Session ihren Watchdog startet, verliert die erste Session ihre Deadman-/ACK-Überwachung **ohne Fehler oder Log-Warnung**. Das ist kein UX-Gap, sondern ein Sicherheitsdefekt: ein Operator kann glauben, sicher zu fahren, während die Timeout-Überwachung für sein Fahrzeug inaktiv ist.

Zusätzlich: `GET /state` liefert einen einzigen globalen Snapshot — das Frontend kann nicht nach dem State eines bestimmten Fahrzeugs fragen.

### Klärung per Grill-Me-Session (2026-06-14)

| Frage | Entscheidung |
|---|---|
| API-Form für vehicle-scoped State | **Pfad-Segment** `GET /vehicles/{id}/state` (konsistent zu `GET /vehicle/ack/latest/{id}`) |
| Lifecycle der Per-Vehicle-Instanzen | **Lazy-Create, dauerhaft behalten** für die Prozesslaufzeit (kleine, bekannte Flotte — kein GC-Aufwand) |
| Scope-Schnitt | **Ein ADR, vollständig**, zerlegt in mehrere Typ-M-Sprint-Tasks (kein Phasen-Split) |
| Vehicle-Dropdown UI mit Live-State-Badge | **Bewusst ausgeklammert** → Folge-Task im Backlog |

## Decision

### 1. `VehicleContext` — gebündelte Per-Fahrzeug-Safety-Komponenten

Neuer Typ in `internal/controlserver/vehiclecontext/`:

```go
type VehicleContext struct {
    SM                 *statemachine.Machine
    Deadman            *csafety.DeadmanWatchdog
    VehicleACKWatchdog *csafety.VehicleACKWatchdog
}
```

`VehicleContextRegistry` (`map[vehicleID]*VehicleContext` + `sync.RWMutex`) erzeugt Instanzen lazy:

```go
func (r *Registry) Get(vehicleID string) *VehicleContext
```

Jedes Fahrzeug bekommt beim ersten Zugriff (egal ob über `/vehicle/ws`-Connect oder `POST /session/start`) eine eigene, frische `statemachine.Machine` (Start-Zustand `IDLE`), einen eigenen `DeadmanWatchdog`, einen eigenen `VehicleACKWatchdog`. Instanzen bleiben für die Prozesslaufzeit bestehen (Grill-Me-Entscheidung).

### 2. `SafetyBusWatchdog` bleibt global — fächert aber auf alle Fahrzeuge auf

Der Safety-Service ist geteilte Infrastruktur (ADR-002): ein `/health`-Endpunkt, kein Per-Fahrzeug-Konzept. Ein Ausfall ist also korrekt ein **fleet-weites** Ereignis — die heutige Implementierung trifft das nur zufällig (weil sie ohnehin nur eine Session kennt), aber falsch begründet.

Änderung:
- `sessionID`/`vehicleID`-Felder entfernt.
- Konstruktor erhält den `session.Manager` (für `ListSessions()`) statt einzelner IDs.
- `Start()` läuft **einmalig beim Prozessstart**, nicht mehr pro `session/start`.
- `triggerSafeMode()` iteriert `sessionMgr.ListSessions()`, dedupliziert nach `VehicleID`, holt sich für jedes betroffene Fahrzeug den `VehicleContext` aus der Registry und transitioniert dessen `SM` einzeln nach `SAFE_MODE`. Pro Fahrzeug wird ein eigenes `SafetyEvent` publiziert (Audit-Korrektheit: jedes betroffene Fahrzeug erscheint separat im Log).

### 3. Alle Aufrufstellen von `sm` wechseln auf `vehicleContexts.Get(vehicleID).SM`

Betroffen in `cmd/control-server/main.go`:

| Endpoint/Stelle | Heute | Neu |
|---|---|---|
| `POST /session/start` | `sm.TransitionSystem(...)` | `vehicleContexts.Get(req.VehicleID).SM.TransitionSystem(...)` |
| `POST /session/end` | `sm.TransitionSystem(StateIdle)` | `vehicleContexts.Get(sess.VehicleID).SM...` |
| `POST /media/event` | `sm.TransitionMedia(...)` | über `claimsKey`/Session → `vehicleContexts.Get(sess.VehicleID).SM` |
| `POST /emergency-stop` | `sm.TransitionSystem(StateSafeMode)` | aus Request `vehicle_id` → `vehicleContexts.Get(...)`. E-Stop bleibt aber zusätzlich **fleet-weit auslösbar** wenn kein `vehicle_id` übergeben wird (Sicherheitsnetz, siehe Konsequenzen) |
| WS-Handler (`transport/websocket.go`) | `h.sm` (ein Feld) | `h.vehicleContexts.Get(sess.VehicleID).SM` pro Connection |
| Command Engine (`command/engine.go`) | `e.sm` | Lookup über `sess.VehicleID` bei jedem `Handle()`-Aufruf |
| `vehicleconnection.Handler` (Deadman/ACK-Start) | globale Watchdog-Felder | `vehicleContexts.Get(vehicleID).Deadman` / `.VehicleACKWatchdog` |

### 4. API: `GET /vehicles/{id}/state` ersetzt `GET /state` für Live-Polling

```
GET /vehicles/{id}/state → { system, control, media, operator }  (4-Layer-Snapshot des EINEN Fahrzeugs)
```

`GET /state` (ohne ID) entfällt als Polling-Endpunkt für die zwei unten migrierten Nutzungen.
> **Update (2026-07-16):** In der Umsetzung wurde `GET /state` bewusst nicht entfernt, sondern als
> Compat-Shim beibehalten — noch von `tests/performance/latency.js` und
> `tests/integration/services_test.go` genutzt, kein Frontend-Konsument mehr (geplante Entfernung:
> Backlog MV-12). Bereits korrekt dokumentiert in `architecture.md` und `DECISIONS.MD` (Zeile
> MV-12); dieser Absatz beschrieb nur die ursprüngliche Planung. Die zwei bisherigen Nutzungen wurden migriert:

- **Live-Polling während aktiver Session** (`useSystemState`): sobald `vehicleId` bekannt ist (Session gestartet/wiederhergestellt) → Wechsel auf `GET /vehicles/{id}/state`.
- **Page-Reload-Recovery** (Sprint 16 BUG-03 — Frontend kennt nach Reload noch keine `vehicleId`): nutzt stattdessen den bereits existierenden `GET /sessions`-Endpoint (Sprint "Observer-Einschränkungen" Sept. 2026-06-14), gefiltert auf `operator_id === eigene JWT-Subject` → liefert `vehicle_id` + `role` der eigenen, evtl. verwaisten Session.
- **Unreachable-Banner** (`ROB-01`, Sprint 14): braucht nur irgendeinen Health-Beweis, kein Fahrzeugbezug — pollt künftig `GET /sessions` (liefert ohnehin 200 auch bei leerer Liste) statt `GET /state`.

`useSystemState(vehicleId: string | null)` bekommt damit einen Parameter und wählt intern den Endpoint.

### 5. Auto-Registrierung bleibt unverändert

Die in der vorherigen Iteration eingeführte Auto-Registrierung neuer Fahrzeuge (`vehicleconnection.Handler.WithVehicleAdder`) bleibt bestehen — sie befüllt nur die `vehicles`-DB-Tabelle, nicht die `VehicleContextRegistry`. Die Registry erzeugt ihren Eintrag unabhängig davon beim ersten `/vehicle/ws`-Connect oder `session/start`.

## Consequences

### Positiv
- Zwei Operatoren auf zwei Fahrzeugen sind ab jetzt sicherheitstechnisch wirklich isoliert: Deadman-Timeout, ACK-Timeout und SAFE_MODE eines Fahrzeugs beeinflussen kein anderes Fahrzeug mehr.
- Safety-Bus-Ausfall bleibt korrekt fleet-weit (das war schon immer der fachlich richtige Anspruch, jetzt auch korrekt implementiert statt zufällig).
- `GET /vehicles/{id}/state` ist konsistent mit dem bestehenden `GET /vehicle/ack/latest/{id}`-Muster.
- Kein GC/Cleanup-Code nötig (Grill-Me-Entscheidung: dauerhaft behalten) — reduziert Komplexität für die aktuelle Flottengröße.

### Negativ / Risiken
- **Breaking Change für `GET /state`**: alle Frontend-Aufrufstellen (`useSystemState`, Page-Reload-Recovery) müssen gleichzeitig migriert werden — kein schrittweiser Rollout möglich, da der globale State sonst leer/falsch wäre.
- **E-Stop-Sonderfall**: `POST /emergency-stop` ohne `vehicle_id` muss explizit auf "alle aktiven Fahrzeuge" erweitert werden, sonst regressiert ein bestehendes Sicherheitsfeature (globaler Notaus). Muss in Sprint 17 explizit getestet werden.
- **Mehr State im Speicher**: pro Fahrzeug eine `Machine` + 2 Watchdog-Strukte statt einmalig — bei einer Flotte im zwei- bis dreistelligen Bereich vernachlässigbar, bei sehr großen Flotten (Tausende Fahrzeuge) müsste die "dauerhaft behalten"-Entscheidung revidiert werden (siehe Backlog-Folge-Eintrag).
- **Tests**: Alle bestehenden Watchdog-/State-Machine-Unit-Tests (Sprint 16, `tests/unit/watchdog_test.go`) instanziieren `sm`/Watchdog direkt — bleiben gültig, da sie weiterhin eine einzelne Instanz testen. Neue Tests nötig für `VehicleContextRegistry.Get()` (Idempotenz, Concurrency) und für das fleet-weite `SafetyBusWatchdog`-Verhalten (mehrere Fahrzeuge gleichzeitig in SAFE_MODE).

### Out of Scope (Folge-Tasks)
- Vehicle-Dropdown mit Live-State-Badge pro Fahrzeug im Frontend (Grill-Me-Entscheidung: separater Backlog-Task).
- Garbage Collection inaktiver `VehicleContext`-Instanzen bei großer/dynamischer Flotte.
- MediaMTX-Routing ist bereits Pfad-basiert pro `vehicleId` (ADR-022) und nicht von diesem ADR betroffen.
