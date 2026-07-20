# Meilenstein 2 — Web-Dashboard (AP2)

Stand: 2026-07-18 (Sprint 33) | Referenz: Leistungsbeschreibung AP2, `ADR-029`/`030`/`032`/`033`/`034`

Konsolidiertes Abnahme-Dokument für den Auftraggeber. Fasst die über mehrere Sprints (21–32)
entstandenen Dashboard-Funktionen zusammen (`AP2-05` in `tasks/backlog.md`). Ersetzt nicht
`docs/requirements.md`/`docs/architecture.md` als technische Arbeitsgrundlage, sondern ordnet den
Umsetzungsstand dem vertraglichen Leistungsumfang von AP2 zu.

---

## 1. Leistungsumfang laut Leistungsbeschreibung

AP2 umfasst vier Anforderungsbereiche: Flottenübersicht, Kartenansicht/Werkshallenvisualisierung,
Task Management, Alert System.

## 2. Umsetzungsstand je Bereich

| Bereich | Status | Kurzbeschreibung |
|---|---|---|
| **Flottenübersicht** | ✅ Fertig | Echtzeit-Fahrzeugstatus (Batterie, Geschwindigkeit, Autonomie-Modus), Live-Updates ohne Polling über WebSocket-Broadcast |
| **Kartenansicht — Outdoor** | ✅ Fertig | SVG-Zonenkarte, geo-referenziert per Leaflet-Overlay; Stationen und Fahrzeuge als Marker, Fahrzeugauswahl per Klick |
| **Kartenansicht — Indoor** | ✅ Fertig (Sprint 33) | SVG-Grundriss im zoneneigenen Koordinatensystem (kein Leaflet/Geo-Referenzierung); Stationen und Fahrzeuge als Marker über `position_x/position_y`, siehe Abschnitt 6 |
| **Routenübersicht (gefahrene Historie)** | ✅ Fertig (Sprint 32) | Durchgezogene Polylinie der zuletzt gefahrenen Route für das ausgewählte Fahrzeug, siehe Abschnitt 4 |
| **Task Management** | ✅ Fertig | Aufgaben erstellen/zuweisen, Status-Workflow (pending → in_progress → completed/cancelled), Prioritätenmanagement (Sortierung + visuelle Hervorhebung), vollständige Status-Verlaufshistorie |
| **Alert System — Basis** | ✅ Fertig | Echtzeit-Benachrichtigungen, Prioritätsklassen, Bestätigung (Acknowledge) pro Alert |
| **Alert System — Audio** | ✅ Fertig | Akustische Benachrichtigung bei neuen Alerts, stummschaltbar |

Alle sieben Anforderungsbereiche sind vollständig umgesetzt (siehe Abschnitt 6 für die
Indoor-Kartenansicht, zuletzt offener Punkt).

## 3. Architekturprinzipien der Umsetzung

- **Kein Backend-zu-Backend-Kopplung:** Das Dashboard fragt `control-server` (Online-Status) und
  `fleet-service` (alle übrigen Fleet-Daten) getrennt ab und führt sie clientseitig über die
  Fahrzeug-ID zusammen (`ADR-029`).
- **Keine externen Kartendienste:** Beide Kartentypen (Indoor/Outdoor) sind SVG-basiert, keine
  SaaS-Kartenkacheln — vermeidet laufende Kartendienst-Kosten und Vendor-Lock-in.
  Outdoor-SVGs werden geo-referenziert per Leaflet-`svgOverlay` über bekannte GPS-Eckkoordinaten
  dargestellt.
- **Live-Updates ohne Polling:** Ein WebSocket-Broadcast-Hub in `fleet-service` verteilt
  Statusänderungen an alle verbundenen Dashboard-Clients — mehrere Operator-Arbeitsplätze sehen
  denselben Zustand gleichzeitig (Multi-Workstation-fähig, `ADR-028`).

## 4. Routenübersicht (Sprint 32, `ADR-033`)

Persistenzform für die "gefahrene Route" war seit `ADR-029` offen (Zeitreihen-DB vs. einfache
Tabelle). Entscheidung: eine einfache, gedrosselt beschriebene Tabelle in der bestehenden
Postgres-Datenbank (kein zusätzlicher Infrastrukturdienst) — angemessen für die aktuelle
Flottengröße, siehe `ADR-033` für die vollständige Begründung inkl. Alternativenabwägung. Die
Route wird für das im Dashboard ausgewählte Fahrzeug als durchgezogene Linie auf der Karte
angezeigt; eine automatische Bereinigung alter Datenpunkte (30 Tage) verhindert unbegrenztes
Tabellenwachstum.

## 5. Task-Status-Historie (Sprint 31, `ADR-032`)

Über den reinen "wer hat zuletzt geändert"-Stand hinaus (`ADR-030`) zeichnet das System seit
Sprint 31 jeden Status-Übergang eines Tasks vollständig nach — abrufbar für die
Task-Detailansicht, inklusive nachvollziehbarer Näherungswerte für Übergänge, die vor Einführung
dieser Historie stattfanden.

## 6. Indoor-Kartenrendering (Sprint 33, `AP2-02`/`ADR-034`)

Fahrzeuge trugen für Outdoor-Positionen bereits `position_lat`/`position_lon`; für eine Position
*innerhalb* einer Indoor-Zone fehlte das Äquivalent (`position_x`/`position_y`, wie es Stationen
bereits hatten) — eine reine Zonen-Zugehörigkeit reicht nicht für eine Punktdarstellung auf der
Karte. `ADR-034` ergänzt `vehicle_status` um genau diese beiden Spalten, analog zum bestehenden
`stations`-Muster. Die Indoor-Karte selbst ist bewusst **kein** Leaflet-Overlay wie bei Outdoor —
Indoor-Zonen sind nicht geo-referenziert, daher rendert `FleetIndoorMap.tsx` die Zonen-eigene
`svg_geometry` direkt als Koordinatensystem, mit Stationen/Fahrzeugen als SVG-Markern darin.

**Bewusst nicht Teil dieses Sprints:** wie `position_x/y` tatsächlich befüllt werden (Simulator-
oder reale Gateway-Anbindung) — die Fahrzeugsimulation liefert bisher nur Outdoor-GPS-Positionen,
sodass aktuell noch kein Fahrzeug-Marker auf der Indoor-Karte erscheint, nur die Zonen-Geometrie
und Stationen. Datenmodell, Backend-API und Frontend-Rendering sind vollständig vorhanden und mit
manuell gesetzten Testdaten verifiziert; die Befüllung ist als eigener Folge-Task vorgemerkt
(`tasks/backlog.md`, `DECISIONS.MD`), um diesen Task klein und unabhängig verifizierbar zu halten.

## 7. Referenzen

| Thema | Dokument |
|---|---|
| Fachliche Anforderungen | `docs/requirements.md`, Abschnitt "Web Dashboard Requirements" |
| Vollständige technische Architektur | `docs/architecture.md`, Abschnitt "Fleet System" |
| Fleet-Datenmodell, Service-Grenze | `docs/adr/029-fleet-vehicle-data-model.md` |
| Task-Status-Lifecycle | `docs/adr/030-task-status-lifecycle.md` |
| Task-Status-Audit-Historie | `docs/adr/032-task-status-history.md` |
| Routenübersicht-Persistenz | `docs/adr/033-vehicle-position-history.md` |
| Indoor-Fahrzeugposition | `docs/adr/034-indoor-vehicle-position.md` |
| Aktueller Sprint-/Task-Stand | `tasks/current-sprint.md`, `tasks/backlog.md` |
