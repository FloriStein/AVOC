# Meilenstein 2 — Web-Dashboard (AP2)

Stand: 2026-07-18 (Sprint 32) | Referenz: Leistungsbeschreibung AP2, `ADR-029`/`030`/`032`/`033`

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
| **Kartenansicht — Indoor** | 🔲 Offen | Fahrzeuge haben aktuell keine Punktposition innerhalb einer Indoor-Zone (nur die Zonen-Zugehörigkeit selbst) — braucht eine eigene Datenmodell-Erweiterung, siehe Abschnitt 5 |
| **Routenübersicht (gefahrene Historie)** | ✅ Fertig (Sprint 32) | Durchgezogene Polylinie der zuletzt gefahrenen Route für das ausgewählte Fahrzeug, siehe Abschnitt 4 |
| **Task Management** | ✅ Fertig | Aufgaben erstellen/zuweisen, Status-Workflow (pending → in_progress → completed/cancelled), Prioritätenmanagement (Sortierung + visuelle Hervorhebung), vollständige Status-Verlaufshistorie |
| **Alert System — Basis** | ✅ Fertig | Echtzeit-Benachrichtigungen, Prioritätsklassen, Bestätigung (Acknowledge) pro Alert |
| **Alert System — Audio** | ✅ Fertig | Akustische Benachrichtigung bei neuen Alerts, stummschaltbar |

Sechs von sieben Anforderungsbereichen sind vollständig umgesetzt. Die Indoor-Kartenansicht ist
der einzige noch offene Punkt (siehe Abschnitt 6).

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

## 6. Offener Punkt: Indoor-Kartenrendering (`AP2-02`)

Fahrzeuge tragen für Outdoor-Positionen `position_lat`/`position_lon`. Für eine Position
*innerhalb* einer Indoor-Zone fehlt das Äquivalent (`position_x`/`position_y`, wie es Stationen
bereits haben) — eine reine Zonen-Zugehörigkeit reicht nicht für eine Punktdarstellung auf der
Karte. Diese Datenmodell-Erweiterung wurde bewusst noch nicht vorgenommen, um `ADR-029` nicht
kommentarlos zu überschreiben — sie braucht eine eigene kurze Architekturentscheidung (Typ L nach
CLAUDE.MD, vergleichbar mit `ADR-033` oben), aufgenommen als `AP2-02` in `tasks/backlog.md`.

## 7. Referenzen

| Thema | Dokument |
|---|---|
| Fachliche Anforderungen | `docs/requirements.md`, Abschnitt "Web Dashboard Requirements" |
| Vollständige technische Architektur | `docs/architecture.md`, Abschnitt "Fleet System" |
| Fleet-Datenmodell, Service-Grenze | `docs/adr/029-fleet-vehicle-data-model.md` |
| Task-Status-Lifecycle | `docs/adr/030-task-status-lifecycle.md` |
| Task-Status-Audit-Historie | `docs/adr/032-task-status-history.md` |
| Routenübersicht-Persistenz | `docs/adr/033-vehicle-position-history.md` |
| Aktueller Sprint-/Task-Stand | `tasks/current-sprint.md`, `tasks/backlog.md` |
