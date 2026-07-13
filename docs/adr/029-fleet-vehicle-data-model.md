# ADR-029: Fleet Vehicle Data Model & Service-Grenze

Status: Accepted

## Kontext

Für das Web-Dashboard (AP2) und die Admin-Konsole (AP3) wird ein Fahrzeug-/Flotten-Datenmodell
benötigt (Typ, Batterie, Position, Autonomie-Modus, Zonen, Stationen, Tasks, Alerts), das im
bestehenden System nicht existiert. Gleichzeitig gibt es bereits ein **Vehicle**-Konzept in
`control-server`/`internal/vehicleregistry` (`vehicles`-Tabelle: `id, display_name, description,
created_at`, plus live berechnetes `online` über `vehicleconnection.Registry.Connected()`) — das
für die Direct-Teleop-/Safety-Domäne existiert (ADR-021/022/026).

Diese zwei „Vehicle"-Konzepte müssen sauber verknüpft werden, ohne die Safety-Domäne
(`control-server`) mit der neuen, nicht-sicherheitskritischen Fleet-Domäne zu koppeln
(Grundprinzip aus `ADR-007`: Control Hub bleibt fokussiert; vgl. auch `ADR-028`:
Fleet-Monitoring ist explizit nicht safety-enforcement-verantwortlich).

## Optionen

### Option A: Gemeinsame Vehicle-Tabelle, ein Service besitzt alles

**Vorteile:** Keine Verknüpfungslogik nötig
**Nachteile:** Koppelt Safety-Domäne (control-server) und Fleet-Domäne (Task/Alert/Zonen) in einem
Service — widerspricht dem vom Auftraggeber vorgegebenen Microservices-Prinzip und vergrößert die
Angriffs-/Änderungsfläche des safety-kritischen Control Servers unnötig

### Option B: Zwei komplett unabhängige Registries, verknüpft nur über gleichlautende IDs

**Vorteile:** Maximale Entkopplung
**Nachteile:** Kein Konsistenzmechanismus — ein Fahrzeug könnte in einem Service existieren und im
anderen nicht, keine referenzielle Integrität, Drift-Risiko

### Option C: Gemeinsame `vehicles`-Identitätstabelle (bestehend), neuer Service erweitert sie über Fremdschlüssel (gewählt)

**Vorteile:**
- Nutzt die bereits bestehende `vehicles`-Tabelle als einzige Quelle der Wahrheit für
  Fahrzeug-Identität — keine Duplikation
- `fleet-service` (neuer Service) bekommt einen eigenen Connection-Pool auf dieselbe geteilte
  Postgres-DB `avoc` (bestehendes Muster seit `ADR-023`) und referenziert `vehicles.id` per
  Fremdschlüssel aus seinen eigenen Tabellen — referenzielle Integrität durch Postgres garantiert
- `control-server` bleibt unverändert für Identität/Online-Status zuständig; `fleet-service`
  besitzt ausschließlich die neuen Fleet-Konzepte
- Konsistent mit Microservices-Prinzip (unabhängige Tabellen pro Service), ohne auf referenzielle
  Integrität zu verzichten (beide greifen auf dieselbe DB zu, nicht auf getrennte DBs)

**Nachteile:**
- Zwei Services schreiben potenziell auf dieselbe Tabelle (`control-server`: Auto-Register bei
  erstem WS-Connect; `fleet-service`: vollständige Admin-Konsole-Konfiguration inkl. Typ) —
  erfordert klare Schreibzuständigkeit pro Spalte (siehe unten)
- Schema-Änderungen an `vehicles` betreffen potenziell beide Services — Migration muss koordiniert werden

## Entscheidung

Wir wählen **Option C**.

### Schema-Erweiterung (Migration)

```sql
ALTER TABLE vehicles ADD COLUMN vehicle_type TEXT; -- 'lastenrad' | 'lastenzug'
```

`vehicles` bleibt strukturell wie bisher (`id, display_name, description, created_at`), erweitert
um `vehicle_type`. Kein neues `online` — das bleibt live-berechnet in `control-server`, wird nicht
in die DB geschrieben (unverändert aus `ADR-022`).

### Neue, von `fleet-service` besessene Tabellen (alle mit `vehicle_id REFERENCES vehicles(id)`, wo zutreffend)

```
vehicle_status  (vehicle_id FK, battery_pct, speed, position_lat, position_lon,
                  position_zone_id FK, autonomy_mode, current_task_id FK, updated_at)
zones           (id, name, environment ['indoor'|'outdoor'], svg_geometry, geo_bounds)
stations        (id, zone_id FK, name, position_x, position_y, position_lat, position_lon)
tasks           (id, vehicle_id FK, from_station_id FK, to_station_id FK,
                  status, priority, created_at, completed_at)
alerts          (id, vehicle_id FK, severity, message, created_at,
                  acknowledged_at, acknowledged_by)
```

`vehicle_status` ist bewusst von `vehicles` getrennt (nicht in dieselbe Zeile geschrieben) — hochfrequente
Telemetrie-Updates (Position/Batterie) sollen nicht dieselbe Tabellenzeile beschreiben wie die
selten geänderte Fahrzeug-Identität.

### Schreibzuständigkeit

- **`control-server`:** `vehicles.id`, `vehicles.display_name` (Auto-Register bei erstem
  `/vehicle/ws`-Connect, minimal — unverändert aus `ADR-021`)
- **`fleet-service`:** `vehicles.vehicle_type`, `vehicles.description` sowie alle neuen Tabellen
  (Admin-Konsole-CRUD, AP3: Registrierung, Konfiguration, Zonenzuweisung, Maintenance Tracking)
- Bestehende `POST /vehicles`/`DELETE /vehicles/{id}`-Endpunkte in `control-server` bleiben vorerst
  bestehen (Kompatibilität), werden aber langfristig vermutlich durch die reichhaltigere
  `fleet-service`-Admin-API abgelöst — kein Bruch in diesem Schritt, nur vorgemerkt

### Frontend-seitige Zusammenführung statt Backend-Kopplung

Das Dashboard fragt **beide Services separat ab** und führt clientseitig über `vehicle_id`
zusammen (`control-server` für `online`-Status, `fleet-service` für Typ/Batterie/Position/Task/
Alerts) — keine synchrone Backend-zu-Backend-Kopplung zwischen `control-server` und
`fleet-service`. Konsistent mit der vertraglichen Betonung auf der clientseitigen
Visualisierungsschicht (Leistungsbeschreibung AP2).

### Kartendarstellung (Indoor + Outdoor, beide SVG-basiert)

- **Indoor:** eigenständige SVG-Karte (wird als Beispiel neu erstellt), Koordinaten in
  `stations.position_x/y`
- **Outdoor:** ebenfalls SVG-Karte, aber **geo-referenziert** — die SVG wird als Overlay über ein
  Koordinatensystem gelegt, dessen Eckpunkte bekannten GPS-Koordinaten entsprechen (Leaflet
  `imageOverlay`/`svgOverlay` mit `bounds`, keine externen Kartenkacheln/SaaS-Abhängigkeit nötig).
  `stations.position_lat/lon` sind die primäre Quelle, `zones.geo_bounds` verankert die
  SVG-Überlagerung
- Route-Darstellung: geplante Route und gefahrene Historie beide darstellbar, mit
  unterschiedlicher visueller Markierung (z. B. gestrichelt vs. durchgezogen) — Datenquelle:
  geplante Route aus `tasks` (from/to Station), Historie aus `vehicle_status`-Zeitreihe
  (Persistenzform für Historie ist noch offen, siehe `CONTEXT.MD` Offene Fragen)

## Konsequenzen

### Positiv
- Kein Duplikat der Fahrzeug-Identität, referenzielle Integrität durch Postgres
- Safety-Domäne (`control-server`) bleibt unverändert und unberührt von Fleet-Feature-Entwicklung
- Neuer Service ist unabhängig deploybar/skalierbar (Microservices-Prinzip des AG erfüllt)

### Negativ
- Geteilte Tabelle zwischen zwei Services erfordert Migrations-Koordination bei Schema-Änderungen
- Frontend muss zwei Datenquellen zusammenführen (etwas mehr Client-Komplexität als eine einzelne API)
- Historie-Persistenz für "gefahrene Route" ist noch nicht spezifiziert (Zeitreihen-DB? Einfache
  Tabelle? — siehe `CONTEXT.MD`)
