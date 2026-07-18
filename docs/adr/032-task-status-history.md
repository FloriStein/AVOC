# ADR-032: Vollständige Task-Status-Audit-Historie

Status: Accepted

## Kontext

`ADR-030` hat bewusst nur den *letzten* Status-Übergang gespeichert (`tasks.status_changed_by`)
und eine eigene Audit-Tabelle als Scope-Creep für den ersten Slice abgelehnt — mit dem
expliziten Vermerk, dass eine vollständige Historie über mehrere Übergänge hinweg "ein separates
Folge-ADR, kein Bruch dieses hier" wäre, falls sie später gebraucht wird (`TASKUI-03`).

Sprint-31-Triage (2026-07-18): der Bedarf ist da (Task-Detailansicht soll den vollständigen
Werdegang eines Tasks zeigen, nicht nur "wer hat zuletzt geändert"). Da dies eine neue Tabelle
und damit eine Datenstruktur-Änderung ist, gilt CLAUDE.MD Abschnitt 1.1 Typ L — Grill-Me-Session
vor Umsetzung. Grill-Me-Session (2026-07-18) hat folgende Fragen geklärt.

## Optionen

### Option A: `tasks.status_changed_by` um weitere Spalten erweitern (z.B. JSON-Array-Spalte)

**Vorteile:** Kein neues Tabellen-Join nötig
**Nachteile:** Postgres-JSON-Spalten sind für eine wachsende, unbegrenzte Liste von Einträgen
schlecht geeignet (kein effizientes Filtern/Sortieren einzelner Einträge, keine referenzielle
Integrität pro Eintrag). Widerspricht dem in `ADR-029`/`ADR-030` etablierten Muster (eigene
Tabelle pro fachlichem Konzept).

### Option B: Eigene `task_status_history`-Tabelle, ein Eintrag pro Übergang (gewählt)

**Vorteile:**
- Konsistent mit dem bestehenden Schema-Stil (`vehicle_status`, `alerts` sind ebenfalls eigene
  Tabellen statt eingebetteter Felder)
- Beliebig viele Übergänge ohne Schema-Änderung, effizientes Filtern/Sortieren per SQL
- `tasks.status_changed_by` bleibt unverändert bestehen (ADR-030 nicht gebrochen, nur ergänzt) —
  schnelle "letzter Änderer"-Abfrage ohne Join bleibt möglich

**Nachteile:**
- Neue Tabelle, neue Migration, neuer Endpoint — mehr Implementierungsaufwand als Option A
- Leichte Redundanz zwischen `tasks.status_changed_by` (letzter Übergang) und dem jeweils
  letzten Eintrag in `task_status_history` (bewusst akzeptiert, siehe Entscheidung)

## Entscheidung

Wir wählen **Option B**.

### Schema (additiv, kein Bruch von ADR-030)

```sql
CREATE TABLE IF NOT EXISTS task_status_history (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES tasks(id),
    from_status TEXT,
    to_status   TEXT NOT NULL,
    changed_by  TEXT NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_task_status_history_task_id ON task_status_history(task_id);
```

`from_status` ist nullable — siehe Backfill-Abschnitt für den einzigen Fall, in dem das genutzt
wird. IDs werden wie bei allen anderen Tabellen in diesem Service anwendungsseitig per
`ulid.Generate()` erzeugt, kein DB-seitiger Default (konsistent mit `zones`/`stations`/`tasks`).

Nur tatsächliche `PATCH .../status`-Übergänge werden als Zeile erfasst — die Erzeugung eines
Tasks (impliziter Status `pending`) erzeugt bewusst **keine** Historien-Zeile; `tasks.created_at`
deckt das bereits ab. `pending` taucht deshalb (wie schon in `ADR-030`s
`taskTransitionSources`) nie als `to_status` auf.

### Backfill bestehender Tasks (Rollout, einmalig, idempotent)

Bestehende Tasks kennen nur ihren letzten Übergang (`tasks.status_changed_by`,
`tasks.completed_at` nur bei `completed` gesetzt). Beim ersten Start nach diesem ADR wird für
jeden Task mit `status != 'pending'`, der noch keine Historien-Zeile hat, eine Zeile
nachgetragen:

- `to_status` = `tasks.status`, `changed_by` = `tasks.status_changed_by`
- `changed_at` = `COALESCE(tasks.completed_at, tasks.created_at)` — für `completed`-Tasks ist
  `completed_at` der tatsächliche Übergangszeitpunkt; für `in_progress`/`cancelled` existiert
  keine gespeicherte Übergangszeit, `created_at` ist eine bekannt ungenaue Näherung (**dokumentierte
  Einschränkung**, kein Anspruch auf Exaktheit für Zeilen vor diesem ADR)
- `from_status`: nur gesetzt, wenn eindeutig herleitbar aus `ADR-030`s Übergangstabelle
  (`in_progress` kam garantiert von `pending`, `completed` garantiert von `in_progress`).
  Für `cancelled` ist die Herkunft mehrdeutig (`pending` **oder** `in_progress` möglich) — dort
  bleibt `from_status` bewusst `NULL` statt eines geratenen Werts (**dokumentierte
  Einschränkung**, keine stillschweigende Erfindung von Daten, die nicht rekonstruierbar sind)

Idempotenz: das Backfill-`INSERT` läuft nur für `task_id`s, die noch keine
`task_status_history`-Zeile besitzen (`NOT EXISTS`-Guard) — ein erneuter Service-Start dupliziert
nichts, konsistent mit dem `taskuiDemoSeedCleanup`-Muster aus `ADR-030`.

### API

Neuer Endpoint `GET /fleet/tasks/{id}/history` (nicht in `GET /fleet/tasks` eingebettet — die
Task-Liste bleibt schlank, Historie wird nur bei Bedarf geladen, z.B. beim Öffnen einer
Task-Detailansicht). Antwort: Array, chronologisch aufsteigend sortiert (`changed_at ASC`) für
eine Timeline-Darstellung. Existiert der Task nicht, `404` (wiederverwendet `ErrTaskNotFound`
aus `ADR-030`) statt eines irreführenden leeren Arrays — ein leeres Array bedeutet stattdessen
"Task existiert, ist aber noch `pending`, hatte also noch keinen Übergang".

### Schreibpfad

`store.UpdateTaskStatus` (bestehend, `ADR-030`) schreibt zusätzlich zum bisherigen `UPDATE
tasks ...` einen neuen `INSERT INTO task_status_history` mit dem tatsächlichen `from_status`
(dem vor dem UPDATE gelesenen `status`) und `to_status` (dem neuen Status) — kein separates
Sentinel-/Fehlerverhalten nötig, da dieser INSERT nur nach erfolgreichem, bereits validiertem
UPDATE läuft.

## Konsequenzen

### Positiv
- Task-Detailansicht kann den vollständigen Werdegang zeigen, nicht nur den letzten Übergang
- `ADR-030` bleibt unverändert gültig (`status_changed_by` weiterhin gepflegt) — rein additive
  Erweiterung, kein Bruch
- Gleiches Migrations-/Idempotenz-Muster wie alle bisherigen Schema-Änderungen in
  `internal/fleetservice/store.go`

### Negativ
- Backfill-Daten für Tasks vor diesem ADR sind bei `changed_at` (für `in_progress`/`cancelled`)
  und `from_status` (für `cancelled`) unvollständig/approximiert — dokumentierte, akzeptierte
  Lücke, keine rückwirkende Erfindung exakter historischer Daten
- Leichte Redundanz zwischen `tasks.status_changed_by` und dem jüngsten
  `task_status_history`-Eintrag (bewusst in Kauf genommen für O(1)-Zugriff auf den letzten
  Änderer ohne Join)
