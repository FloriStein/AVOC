# ADR-033: Persistenzform für gefahrene Route (Vehicle Position History)

Status: Accepted

## Kontext

`ADR-029` hat die Historie-Persistenz für die "gefahrene Route" (Kartendarstellung, durchgezogene
Linie im Gegensatz zur gestrichelten geplanten Route aus `tasks`) bewusst offengelassen: "Zeitreihen-DB
vs. einfache Tabelle noch nicht entschieden" (`FLEET-02`/`AP2-03` in `tasks/backlog.md`). Bis dahin
wird `vehicle_status` als reine "letzter bekannter Zustand"-Zeile geführt (`UpsertVehicleStatus`,
`ON CONFLICT (vehicle_id) DO UPDATE` — überschreibt bei jedem Update, keine Historie). Die
"Routenübersicht (gefahrene Historie)"-Zeile in der AP2-Epic-Tabelle blockiert seit Sprint 23
darauf.

Da dies eine neue Tabelle und damit eine Datenstruktur-Änderung ist, gilt CLAUDE.MD Abschnitt 1.1
Typ L — Grill-Me-Session vor Umsetzung. Grill-Me-Session (2026-07-18, Sprint-32-Kickoff) hat die
Kernfrage geklärt.

## Optionen

### Option A: Dedizierte Zeitreihen-DB (z. B. TimescaleDB-Extension, InfluxDB als separater Dienst)

**Vorteile:**
- Für hochfrequente Telemetrie mit langer Aufbewahrung und Analytics-Anforderungen (Downsampling,
  Continuous Aggregates) die technisch passendere Lösung
- Skaliert auf deutlich größere Flotten/Aufbewahrungszeiträume ohne manuelles Tuning

**Nachteile:**
- Neue Infrastrukturkomponente (CLAUDE.MD Abschnitt 14 — Betriebs-/Wartungsaufwand, ggf.
  Vendor-Lock-in bei einem separaten Dienst wie InfluxDB)
- Durch die aktuell kleine Flotte (siehe `MV-10`-Begründung: "aktuell nicht relevant, kleine
  Flotte") und die rein clientseitige Kartendarstellung (`ADR-029`) nicht durch einen konkreten
  Bedarf gedeckt — Overengineering ohne belegten Anlass
- TimescaleDB wäre zusätzlich eine PostgreSQL-Extension, die im bestehenden `avoc`-Deployment
  (Standard-`postgres`-Image, `pkg/db`) nicht vorhanden ist — eigener Migrationsschritt

### Option B: Einfache Tabelle in der bestehenden `avoc`-Postgres-DB (gewählt)

**Vorteile:**
- Kein neuer Dienst, keine neue Abhängigkeit — nutzt dasselbe Verbindungsmuster wie alle anderen
  `fleet-service`-Tabellen (`ADR-029`/`ADR-023`)
- Ausreichend für die tatsächliche Zugriffsform: ein Zeitfenster von Punkten pro Fahrzeug, zum
  Zeichnen einer Polylinie auf der Karte — keine Aggregationsfunktionen nötig
- Konsistent mit dem bereits etablierten Muster für Audit-/Verlaufsdaten in diesem Projekt
  (`task_status_history`, `ADR-032`)

**Nachteile:**
- Kein eingebautes Downsampling/Retention — muss selbst gebaut werden (siehe unten)
- Bei signifikantem Flottenwachstum ggf. Migration auf Option A nötig (kein Blocker heute, siehe
  `MV-10`-Präzedenzfall für "bewusst zurückgestellt, bis Bedarf sichtbar ist")

## Entscheidung

Wir wählen **Option B**.

### Schema

```sql
CREATE TABLE IF NOT EXISTS vehicle_position_history (
    id           TEXT PRIMARY KEY,
    vehicle_id   TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    position_lat DOUBLE PRECISION NOT NULL,
    position_lon DOUBLE PRECISION NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vehicle_position_history_vehicle_id_recorded_at
    ON vehicle_position_history(vehicle_id, recorded_at);
```

`ON DELETE CASCADE` analog zu `task_status_history` (`ADR-032`) — es gibt keinen
Produktions-Lösch-Endpoint für Fahrzeuge, aber History-Zeilen haben keinen Grund, ihr Fahrzeug zu
überleben (Test-Cleanup, künftige Admin-Tooling-Fälle). Ohne `ON DELETE CASCADE` würde eine
RESTRICT-Constraint dieselbe Klasse von Testcleanup-Bug reproduzieren, die `ADR-032` bereits einmal
gefunden hat.

### Schreibpfad — gedrosselt, nicht bei jedem Telemetrie-Update

`vehicle-mock`s Fleet-Simulation publiziert Status alle 3s (`fleetSimulationTick`,
`cmd/vehicle-mock/fleet_simulator.go`). Ungedrosselt geschrieben ergäbe das ~1.200 Zeilen/Stunde
pro Fahrzeug — für die Kartendarstellung (Polylinie) unnötig fein aufgelöst, treibt aber
Tabellenwachstum unnötig hoch. Gewählt: ein neuer Punkt wird nur aufgezeichnet, wenn seit dem
letzten aufgezeichneten Punkt für dieses Fahrzeug mindestens 10 Sekunden vergangen sind
(`positionHistoryMinInterval`), umgesetzt als bedingtes `INSERT ... WHERE NOT EXISTS` — eine
einzelne atomare Anweisung, kein separates read-then-write (gleiches Race-Safety-Prinzip wie
`UpdateTaskStatus` in `ADR-032`, auch wenn hier kein echter Nebenläufigkeitskonflikt zu erwarten
ist, da `subscribeVehicleStatus` MQTT-Callbacks pro Fahrzeug sequenziell verarbeitet).

Positionslose Status-Updates (`position_lat`/`position_lon` = `nil`, z. B. das bestehende
Direct-Teleop-Fahrzeug ohne Fleet-Simulation) erzeugen keine History-Zeile.

### Retention

Zeilen älter als 30 Tage (`positionHistoryRetention`) werden bereinigt — einmal beim Start von
`fleet-service` (`NewPostgresFleetStore`, gleiches Muster wie `backfillTaskStatusHistory`) und
zusätzlich per Ticker alle 24h während der Laufzeit (`cmd/fleet-service/main.go`), damit ein
dauerhaft laufender Prozess nicht auf einen Neustart wartet, um alte Daten loszuwerden. 30 Tage ist
eine bewusst grobe erste Annahme (kein Auftraggeber-Anforderung dazu bekannt) — spätere Anpassung
über eine Env-Variable ist möglich, aber für den aktuellen Bedarf (Demo-/Pilotbetrieb) nicht
notwendig, daher als Konstante statt Konfigurationsoption umgesetzt (KISS, CLAUDE.MD Abschnitt 12).

### API

Neuer Endpoint `GET /fleet/vehicles/{id}/history` — chronologisch aufsteigend, analog zu
`GET /fleet/tasks/{id}/history` (`ADR-032`). 404 bei unbekanntem Fahrzeug, `[]` bei einem
Fahrzeug ohne aufgezeichnete Punkte (z. B. noch nie Position gemeldet).

### Kartendarstellung

`FleetMap.tsx` zeichnet die Historie nur für das aktuell ausgewählte Fahrzeug (`selectedVehicleId`)
als durchgezogene Polylinie — nicht für alle Fahrzeuge gleichzeitig, um die Karte nicht zu
überladen und um unnötige Fetches für nicht angesehene Fahrzeuge zu vermeiden. Die geplante Route
(gestrichelt, aus `tasks`) ist nicht Teil dieses ADRs — dafür existiert noch keine Task-ID
(offene Zeile in `CONTEXT.MD`), bleibt also weiterhin offen.

## Begründung

Die kleine Flotte und der alleinige Konsument (eine Polylinie auf der Karte, kein Analytics/
Reporting-Anforderung) rechtfertigen keine neue Infrastrukturkomponente. Eine einfache Tabelle mit
gedrosseltem Schreibpfad und expliziter Retention liefert das benötigte Verhalten ohne
Betriebsaufwand, konsistent mit dem bereits etablierten `task_status_history`-Muster (`ADR-032`).

## Konsequenzen

### Positiv
- Kein neuer Dienst, keine neue Abhängigkeit
- Konsistentes Muster mit `ADR-032` (Audit-/Verlaufstabelle, `ON DELETE CASCADE`, idempotente
  Migration beim Start)
- Schließt die "Routenübersicht (gefahrene Historie)"-Lücke aus der AP2-Epic-Tabelle

### Negativ
- Kein eingebautes Downsampling — bei starkem Flottenwachstum müsste die Retention-Konstante
  angepasst oder auf Option A migriert werden (kein aktueller Blocker, analog `MV-10`)
- 30-Tage-Retention ist eine unbestätigte Annahme, kein Auftraggeber-Vorgabe-Beleg im Repo
