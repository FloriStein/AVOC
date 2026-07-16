# ADR-030: Task-Status-Lifecycle & manueller Status-Übergangs-Endpoint

Status: Accepted

## Kontext

`fleet-service` besitzt seit Sprint 21 (FLEET-05, `ADR-029`) ein Task-Datenmodell (`Task{id,
vehicle_id, from_station_id, to_station_id, status, priority, created_at, completed_at}`) und
`POST /fleet/tasks` zum Anlegen. Es gibt aber **keinen Mechanismus**, der den Status nach dem
Anlegen je ändert — jeder Task bleibt technisch dauerhaft auf `"pending"`. Auch die reale
Fahrzeug-Anbindung liefert dazu nichts: laut `ADR-027` ist die ROS2/DDS-Schnittstelle bis zum
AP1-Workshop mit der Professur Logistik unspezifiziert, es gibt keine automatische
Status-Rückmeldung vom Fahrzeug.

Sprint 24 liefert die Task-Management-UI (AP2). "Status verfolgen" wäre ohne einen Weg, den
Status zu ändern, reine Anzeige eines für immer statischen Werts. Gleichzeitig verlangt das
Sprint-Ziel auch "Task-Historie" — die vollständige Liste aller Tasks über alle Status hinweg
(nicht nur aktive), ergänzt um die Frage, wer eine Statusänderung vorgenommen hat.

Grill-Me-Session (2026-07-16) hat entschieden: ein Operator kann den Status manuell setzen, bis
die echte Fahrzeug-Rückmeldung existiert (spätere Ergänzung, kein Bruch dieser Entscheidung nötig
— ein zukünftiger automatischer Pfad kann denselben Store-Mechanismus nutzen).

## Optionen

### Option A: Nur Anzeige, kein Update-Mechanismus

**Vorteile:** Kein neuer Endpoint, kein Nebenläufigkeits-/Validierungsaufwand
**Nachteile:** "Status verfolgen" bleibt bis zum AP1-Workshop faktisch wirkungslos — widerspricht
dem Sprint-Ziel. Vom Nutzer in der Grill-Me-Session explizit abgelehnt.

### Option B: Freies Setzen jedes beliebigen Status (kein Übergangs-Modell)

**Vorteile:** Einfachste Implementierung
**Nachteile:** Erlaubt unsinnige Übergänge (`completed` → `pending`, `cancelled` → `in_progress`)
— keine Garantie, dass `completed_at` konsistent zum Status bleibt; keine Nebenläufigkeits-Semantik
möglich, da es keinen "erwarteten Ausgangsstatus" gibt, gegen den atomar geprüft werden könnte.

### Option C: Explizite Zustandsmaschine mit atomarem, herkunftsbeschränktem Update (gewählt)

**Vorteile:**
- Verhindert unsinnige/widersprüchliche Zustände strukturell (Datenbank-Constraint-artig, nicht
  nur durch UI-Disziplin)
- Race-sicher: zwei gleichzeitige Operator-Aktionen auf denselben Task können nicht beide
  "gewinnen" und einen inkonsistenten Endzustand erzeugen
- Kompatibel mit einer späteren automatischen Fahrzeug-Rückmeldung — dieselbe Store-Methode kann
  dann auch von einem MQTT-Callback aufgerufen werden, nicht nur vom HTTP-Handler

**Nachteile:**
- Mehr Implementierungsaufwand als Option B (Übergangs-Tabelle, Fehler-Differenzierung 404 vs. 409)
- Client (Frontend) muss dieselbe Übergangs-Logik kennen, um sinnvolle UI-Buttons anzuzeigen —
  Duplikation zwischen Go und TypeScript (bewusst in Kauf genommen, siehe Konsequenzen)

## Entscheidung

Wir wählen **Option C**.

### Zustandsmaschine

```
pending ──▶ in_progress ──▶ completed   (terminal)
   │              │
   └──▶ cancelled ◀┘                    (terminal)
```

Erlaubte Ziel-Status per Endpoint: `in_progress`, `completed`, `cancelled` (niemals `pending` —
das ist ausschließlich der Erzeugungs-Default aus `CreateTask`). Erlaubte Herkunfts-Status pro
Ziel:

| Ziel-Status  | Erlaubt von         |
|--------------|---------------------|
| `in_progress`| `pending`           |
| `completed`  | `in_progress`       |
| `cancelled`  | `pending`, `in_progress` |

Jeder andere Übergang — insbesondere jeder Übergang weg von `completed`/`cancelled` (beide
terminal) — wird abgelehnt (`409 Conflict`).

### Endpoint

`PATCH /fleet/tasks/{id}/status`, Body `{"status": "in_progress"|"completed"|"cancelled",
"changed_by": "<operator-id>"}`, beide Felder Pflicht (`changed_by` analog zu
`AcknowledgeAlert`s `acknowledged_by`).

PATCH statt POST+Verb-im-Pfad (wie das bestehende `POST /fleet/alerts/{id}/acknowledge`) gewählt:
näher am bereits im Projekt etablierten `PATCH /auth/users/{id}` (Admin-Konsole, `ADR-024`) —
beides ist semantisch "ein Feld einer Ressource per ID ändern", nicht eine eigenständige Aktion
wie "acknowledge". Bewusste, dokumentierte Abweichung vom `AcknowledgeAlert`-Vorbild, nicht
übersehen — dieses Projekt hat für "Update-by-ID" ohnehin kein einheitliches Verb-Konzept (beide
Präzedenzfälle existieren parallel).

### Nebenläufigkeit (Race-Safety)

Ein atomares, herkunftsbeschränktes `UPDATE`:

```sql
UPDATE tasks
SET status = $1,
    completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
    status_changed_by = $2
WHERE id = $3 AND status = ANY($4)
RETURNING id, vehicle_id, from_station_id, to_station_id, status, priority, created_at, completed_at, status_changed_by
```

`$4` ist die für das Ziel erlaubte Herkunfts-Menge aus der Tabelle oben. Betrifft das Statement 0
Zeilen, folgt ein `SELECT status FROM tasks WHERE id = $1`, um zwei Fälle zu unterscheiden:
- kein Treffer → Task existiert nicht → `404`
- ein Treffer, aber Status nicht in der erlaubten Herkunfts-Menge → ungültiger Übergang → `409`

Zwei sequentielle Statements statt einer Transaktion — konsistent mit dem bisherigen Store-Stil
(`internal/fleetservice/store.go` nutzt bislang keine expliziten Transaktionen). Ein Fehlschlag
zwischen beiden Schritten kann höchstens eine bereits bestehende Ungenauigkeit unverändert lassen
(fehlerhafte 404/409-Unterscheidung bei einem extrem seltenen Zeitfenster), erzeugt aber keine
neue Dateninkonsistenz — das UPDATE selbst bleibt atomar.

### `current_task_id`-Konsistenz

`vehicle_status.current_task_id` (FK auf `tasks.id`, gesetzt vermutlich beim Dispatch) wird
aktuell von nichts genullt, wenn ein Task terminal wird. Ohne Fix würde die Fahrzeug-Detailansicht
dauerhaft einen längst abgeschlossenen/stornierten Task als "aktuell" anzeigen. Fix: nach
erfolgreichem Übergang in `completed` oder `cancelled` zusätzlich

```sql
UPDATE vehicle_status SET current_task_id = NULL WHERE vehicle_id = $1 AND current_task_id = $2
```

(mit `vehicle_id`/`id` aus der zurückgegebenen Task-Zeile) — verhindert, dass ein zwischenzeitlich
neu zugewiesener anderer Task versehentlich überschrieben wird (`WHERE ... AND current_task_id =
$2` statt eines bedingungslosen NULL-Setzens).

### Schema-Änderung

Neue Spalte `tasks.status_changed_by TEXT` (nullable — bei frisch angelegten, noch nicht
übergegangenen Tasks leer). Direkt in `CREATE TABLE IF NOT EXISTS tasks (...)` ergänzt
(Neuinstallation) **plus** `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS status_changed_by TEXT;`
(idempotent, identisches Muster zu `vehicleTypeColumn` aus `ADR-029`, notwendig für die bereits
laufende Dev-DB mit vorhandener `tasks`-Tabelle ohne diese Spalte).

### Task-Historie (Scope-Entscheidung)

"Task-Historie" bedeutet für diesen Sprint: die vollständige Task-Liste (`GET /fleet/tasks`,
bereits vorhanden, sortiert nach `created_at DESC`) über alle Status hinweg, ergänzt um
`status_changed_by`. **Kein** separates Audit-Log/`task_status_history`-Tabelle mit einer Zeile
pro Übergang — das wäre für einen ersten Slice Scope-Creep (Grill-Me-Entscheidung). Wird als
Backlog-Folge-Task vorgemerkt, falls später tatsächlich Nachvollziehbarkeit über mehrere
Übergänge hinweg gebraucht wird (aktuell speichert `status_changed_by` nur den *letzten*
Übergang, frühere gehen verloren).

### Demo-Seed (Zonen/Stationen)

`zones`/`stations` sind in der Dev-DB aktuell leer (verifiziert, 2026-07-16) — Tasks benötigen
aber existierende `from_station_id`/`to_station_id` (FK). Eine minimale, idempotente Demo-Anlage
wird ergänzt: 1 Zone (`demo-zone-taskui`) + 2 Stationen (`demo-station-a-taskui`,
`demo-station-b-taskui`), per `INSERT ... ON CONFLICT (id) DO NOTHING` in
`NewPostgresFleetStore`.

**Bekanntes, akzeptiertes Risiko:** die parallel arbeitende Sprint-23-Session
(Karten-/Zonen-Visualisierung) steht vor demselben Datenproblem und könnte unabhängig eigene
Zonen/Stationen anlegen. Die hier gewählten IDs sind bewusst mit `-taskui`-Suffix eindeutig
namensraum-isoliert, um einen späteren Merge-Konflikt leicht erkennbar/auflösbar zu halten (reine
Datenzeilen-Duplikate, kein Struktur-Konflikt). Nutzer hat dieses Risiko in der Grill-Me-Session
explizit akzeptiert, statt auf die parallele Session zu warten.

### Fehler-Mapping

Neue Sentinel-Errors in `store.go`: `ErrTaskNotFound`, `ErrInvalidTransition` — anders als
`AcknowledgeAlert` (das nur einen generischen "not found"-Fehler kennt, weil es nur einen
Fehlerfall zu unterscheiden braucht), muss der neue Handler zwei HTTP-Codes unterscheiden.
Malformed JSON und fehlende Pflichtfelder → `400` (bestehendes Muster aus `CreateTask`/
`AcknowledgeAlert`).

### WebSocket-Broadcast

Neuer Event-Typ `task_status_changed`, schlanker Payload analog `AlertAcknowledgedEvent` (nicht
die volle Task-Zeile — `GET /fleet/tasks` bleibt die autoritative Quelle):

```go
type TaskStatusChangedEvent struct {
    ID          string     `json:"id"`
    Status      string     `json:"status"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    ChangedBy   string     `json:"status_changed_by"`
}
```

## Konsequenzen

### Positiv
- "Status verfolgen" in der UI wird tatsächlich nutzbar, nicht nur Anzeige eines statischen Werts
- Race-sichere Übergänge — kein durch gleichzeitige Operator-Aktionen erzeugbarer inkonsistenter
  Zustand
- Derselbe Store-Mechanismus (`UpdateTaskStatus`) ist bereits so gebaut, dass eine spätere
  automatische Fahrzeug-Rückmeldung (nach AP1-Workshop) ihn ohne Bruch dieses ADRs wiederverwenden
  kann
- Behebt den vorher unbemerkten `current_task_id`-Staleness-Bug als Nebeneffekt

### Negativ
- Übergangs-Tabelle existiert dupliziert in Go (Backend-Validierung, autoritativ) und TypeScript
  (Frontend, nur für UI-Button-Sichtbarkeit) — bei einer künftigen Änderung der Zustandsmaschine
  müssen beide Stellen angepasst werden. Als bekanntes Tech-Debt im Backlog vermerkt.
- `status_changed_by` speichert nur den letzten Übergang, keine vollständige Historie über
  mehrere Übergänge hinweg — falls später gebraucht, ist eine echte Audit-Tabelle ein separates
  Folge-ADR, kein Bruch dieses hier
- Demo-Seed-Daten sind ein bewusst akzeptiertes Merge-Konflikt-Risiko mit der parallelen
  Sprint-23-Session (siehe oben) — keine automatische Auflösung, muss beim Zusammenführen der
  Branches manuell geprüft werden
