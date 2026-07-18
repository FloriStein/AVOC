# ADR-034: Indoor-Fahrzeugposition (`vehicle_status.position_x/y`)

Status: Accepted

## Kontext

`ADR-029` hat für Indoor-Zonen `stations.position_x/y` als lokales Koordinatensystem definiert,
aber bewusst offengelassen, wie die *Fahrzeug*-Position innerhalb einer Indoor-Zone dargestellt
wird: `vehicle_status` hat nur `position_lat/lon` (GPS, für Outdoor) und `position_zone_id`
(Fremdschlüssel auf `zones`), aber kein Äquivalent zu `stations.position_x/y` für eine
Punktposition innerhalb der Zone. Sprint 23 (MAP-01..11) hat die Kartenvisualisierung deshalb
bewusst auf Outdoor-Zonen beschränkt (`ADR-029`-Update 2026-07-16, `DECISIONS.MD` "Offene
Fragen"). Seitdem als `AP2-02` in `tasks/backlog.md` als einziger noch offener AP2-Anforderungsbereich
geführt (siehe [Meilenstein-2-Dokument](../milestones/meilenstein-2-dashboard.md)).

Da dies eine Datenstruktur-Änderung an `vehicle_status` ist, gilt CLAUDE.MD Abschnitt 1.1 Typ L —
Grill-Me-Session vor Umsetzung. Grill-Me-Session (2026-07-18, Sprint-33-Kickoff) hat zwei Fragen
geklärt.

**Zusätzlicher Befund während der Grill-Me-Vorbereitung, nicht aus dem Backlog-Text ersichtlich:**
`position_zone_id` wird aktuell von **keinem** Schreiber gesetzt — `cmd/vehicle-mock/fleet_simulator.go`
liefert ausschließlich `position_lat/lon` (Outdoor-GPS-Simulation), keine Zonen-Logik. Das
Fahrzeug-Datenmodell hat also schon vor diesem ADR eine unbenutzte Spalte; dieses ADR fügt eine
zweite hinzu, die ebenfalls erst durch einen späteren Task (Simulator- oder reale
Gateway-Integration) tatsächlich befüllt wird. Das ist eine bewusste Entscheidung, kein Versehen
(siehe "Scope" unten).

## Optionen

### Option A: Freie Koordinaten auf `vehicle_status` (gewählt)

Neue Spalten `position_x`/`position_y` auf `vehicle_status`, analog zu `stations.position_x/y` und
zum bestehenden `position_lat/lon`-Muster für Outdoor.

**Vorteile:**
- Konsistente Spaltenbenennung/-semantik mit `stations` (bereits etabliertes Muster aus `ADR-029`)
  — kein neues Konzept, nur dieselbe Spalte auf einer weiteren Tabelle
- Erlaubt kontinuierliche Positionsdarstellung innerhalb einer Zone (nicht nur "an Station X"),
  konsistent mit der Outdoor-Darstellung (`position_lat/lon` ist auch kontinuierlich, nicht auf
  Stationen beschränkt)
- Zukunftsoffen für eine spätere Simulator- oder reale-Gateway-Anbindung, die kontinuierlich
  Positionsupdates liefert (analog zur bestehenden Outdoor-Simulation in `fleet_simulator.go`)

**Nachteile:**
- Weitere Spalte, die vorerst von keinem Schreiber befüllt wird (siehe Kontext oben) — Nutzen
  hängt vom Folge-Task für die Simulator-/Gateway-Anbindung ab

### Option B: Stationsgebunden (kein neues Feld)

Fahrzeug übernimmt für die Kartendarstellung die `position_x/y` der Station, auf die
`current_task_id` bzw. die zuletzt erreichte Station verweist — kein neues Feld auf
`vehicle_status`.

**Vorteile:** Keine Schema-Änderung nötig
**Nachteile:** Keine freie Bewegung innerhalb der Zone darstellbar (Fahrzeug "springt" zwischen
Stationen), inkonsistent mit der kontinuierlichen Outdoor-Darstellung, verknüpft
Kartendarstellung fest mit Task-Zuweisung (funktioniert nicht für ein Fahrzeug ohne aktiven Task,
das sich trotzdem sichtbar in einer Indoor-Zone befindet)

## Entscheidung

Wir wählen **Option A**.

### Schema

```sql
ALTER TABLE vehicle_status ADD COLUMN IF NOT EXISTS position_x DOUBLE PRECISION;
ALTER TABLE vehicle_status ADD COLUMN IF NOT EXISTS position_y DOUBLE PRECISION;
```

Beide Spalten nullable, analog zu `position_lat/lon` — ein Fahrzeug ohne Indoor-Position liefert
`null`. `position_x/y` ist nur im Koordinatensystem der über `position_zone_id` referenzierten Zone
sinnvoll interpretierbar (keine eigene FK-Constraint dafür nötig, analog zur bereits bestehenden
Beziehung zwischen `stations.position_x/y` und `stations.zone_id`).

### Scope dieses ADRs — bewusst nur Datenmodell + Darstellung, keine Befüllung

Analog zum in `ADR-033` etablierten Muster (Schema zuerst, Schreibpfad-Details separat dokumentiert)
deckt dieses ADR nur ab:
1. Schema-Erweiterung (oben)
2. API: `UpsertVehicleStatus`/`GET`-Endpunkte nehmen/liefern `position_x/y` wie die übrigen
   `vehicle_status`-Felder
3. Frontend-Rendering: neue Indoor-Kartendarstellung (SVG-Viewport im Zonen-eigenen
   Koordinatensystem, **kein** Leaflet/`geo_bounds` — Indoor-Zonen sind nicht geo-referenziert)

**Nicht Teil dieses ADRs:** wie `position_x/y`/`position_zone_id` tatsächlich befüllt werden
(Simulator-Erweiterung um Indoor-Bewegungslogik oder reale Gateway-Anbindung nach dem AP1-Workshop).
Grill-Me-Entscheidung (2026-07-18): das hält den Sprint-33-Task klein und verifizierbar (Backend/
Rendering lässt sich mit manuell gesetzten Testdaten/Seed-Werten vollständig verifizieren, ohne auf
eine Simulator-Erweiterung zu warten). Als Folge-Task in `tasks/backlog.md` vorgemerkt.

### Indoor-Kartendarstellung

Anders als bei Outdoor-Zonen (`ADR-029`/Sprint-23-Update: `L.svgOverlay` über echten GPS-Koordinaten)
gibt es für Indoor-Zonen kein geografisches Koordinatensystem — `zones.geo_bounds` bleibt für
`environment = 'indoor'`-Zonen `NULL`. Die Indoor-Karte ist stattdessen ein reines SVG-Dokument im
eigenen Koordinatensystem der Zone (dieselbe Interpretation wie `stations.position_x/y`, das dieses
Koordinatensystem bereits seit `ADR-029` implizit nutzt, ohne dass es bisher visualisiert wurde).
Frontend-seitig eine eigene Komponente (kein Leaflet), die `zones.svg_geometry` direkt als
Hintergrund rendert und Stationen/Fahrzeuge über `position_x/y` als SVG-Koordinaten positioniert.

### Beispiel-Indoor-Zone

Für Verifikation und Demo wird eine neue Beispiel-Indoor-Zone im Seed-Skript
(`scripts/seed-fleet-demo.sh`) angelegt (`environment = 'indoor'`, eigenes `svg_geometry`
Grundriss-Beispiel, mindestens zwei Stationen mit `position_x/y`), analog zu den bestehenden
Outdoor-Beispieldaten aus Sprint 23.

## Begründung

Die Spaltenbenennung/-semantik folgt exakt dem bereits in `ADR-029` etablierten Muster für
`stations` — keine neue Konvention, nur konsistente Anwendung auf eine weitere Tabelle. Die
bewusste Trennung von Datenmodell/Rendering (dieses ADR) und Befüllung (Folge-Task) hält den
Sprint-33-Task im S/M-Rahmen (CLAUDE.MD Abschnitt 10) statt eines einzelnen großen L-Tasks, der
Schema, Backend, Simulator und Frontend in einem Schritt bündelt.

## Konsequenzen

### Positiv
- Schließt die "Kartenansicht — Indoor"-Lücke aus der AP2-Epic-Tabelle strukturell (Datenmodell +
  Rendering-Fähigkeit vorhanden)
- Konsistentes Koordinatensystem-Muster mit `stations` (`ADR-029`), keine Sonderlogik
- Kleine, unabhängig verifizierbare Teilaufgaben statt eines großen Tasks

### Negativ
- `position_x/y` bleibt nach diesem ADR ungenutzt, bis ein Folge-Task sie tatsächlich befüllt
  (Simulator- oder reale Gateway-Integration) — die AP2-Epic-Zeile "Kartenansicht — Indoor" ist
  damit technisch möglich, aber ohne Folge-Task nicht mit Live-Daten demonstrierbar
- Zwei parallele, unterschiedliche Kartendarstellungen (Leaflet/geo-referenziert für Outdoor,
  reines SVG-Koordinatensystem für Indoor) — mehr Frontend-Code als eine einheitliche Lösung, aber
  durch die fundamental unterschiedliche Koordinatengrundlage (GPS vs. lokal) nicht vermeidbar
