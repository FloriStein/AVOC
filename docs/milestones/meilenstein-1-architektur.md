# Meilenstein 1 — Einarbeitung & Architekturplanung (AP1)

Stand: 2026-07-18 (Sprint 32) | Referenz: Leistungsbeschreibung AP1, `ADR-027`/`028`/`029`

Konsolidiertes Abnahme-Dokument für den Auftraggeber. Fasst die über `docs/vision.md`,
`docs/requirements.md`, `docs/architecture.md`, `CONTEXT.MD` und mehrere ADRs verteilte
Architekturarbeit für AP1 zusammen (`AP1-04` in `tasks/backlog.md`). Ersetzt diese Dokumente
nicht — sie bleiben die technische Arbeitsgrundlage — sondern ordnet ihre Ergebnisse dem
vertraglichen Leistungsumfang von AP1 zu.

---

## 1. Leistungsumfang laut Leistungsbeschreibung

AP1 umfasst laut Vertrag zwei Teile:

1. **Einarbeitung** in die vorhandenen Programmierschnittstellen der ausgewählten Fahrzeuge
   (Lastenrad, Lastenzug)
2. **Design der Softwarearchitektur und Schnittstellen** der Leitstellensoftware

## 2. Status: Teil 2 abgeschlossen, Teil 1 extern blockiert

**Wichtiger Vorbehalt:** Dieses Dokument deckt ausschließlich Teil 2 vollständig ab. Teil 1 ist
**nicht abgeschlossen** — er hängt vertraglich vom Workshop mit der Professur Logistik ab
(Klärung der konkreten ROS2-Topics, Nachrichtenformate, DDS-vs-ROSbridge-Frage sowie der
tatsächlichen Fahrzeugschnittstellen). Dieser Workshop-Termin ist zum Stand dieses Dokuments noch
nicht erfolgt (`AP1-01` in `tasks/backlog.md`, externe Abhängigkeit, kein Code-Task). Bis dahin
läuft die gesamte Architektur gegen eine bewusst als vorläufig markierte Annahme (`FleetGateway`/
`MockGateway`-Abstraktion, `ADR-027`) — nicht gegen die reale Schnittstelle. `AP1-02` (Abgleich
gegen die reale Schnittstelle) und `AP1-03` (konkreter Adapter) folgen erst nach dem Workshop.

Meilenstein 1 ist damit zum Stand dieses Dokuments **nicht vollständig abnahmefähig** — dieses
Dokument beschreibt den Architekturstand, ersetzt aber nicht die vertraglich vorgesehene
Einarbeitung in die reale Fahrzeugschnittstelle.

## 3. Zentraler Kurswechsel (2026-07-10 bis 2026-07-14)

Das Projekt begann als Proof-of-Concept für Direct-Teleoperation eines Einzelfahrzeugs
(Joystick-Fernsteuerung mit eigenem Sicherheits-Stack, Sprint 1–19). Nach Klärung mit dem
Auftraggeber (`docs/vision.md` Abschnitt "Historie") entwickelt sich das Projekt gezielt zu einer
**Leitstellensoftware für Fahrzeugflotten** im Rahmen von IBATOUR weiter. Der bestehende
Direct-Teleop-Stack (Control Server, Safety Event Bus, Deadman-Switch, 4-Layer State Machine,
WebRTC-Video-Hub) bleibt vollständig erhalten — kein Rewrite — und wird als **Notfall-Eingriffsmodus**
in die neue Fleet-Leitstelle eingebunden, nicht ersetzt.

## 4. Grundparadigma: Autonomy-First (`ADR-028`)

Die Flotte fährt im Normalfall autonom. Teleoperation ist die Ausnahme für Notfälle:

```
Fahrzeug erkennt Problem → sendet Alert → Operator sieht Alert im Notification-System
  → wählt Fahrzeug per Dropdown → übernimmt über die bestehende, unveränderte
  Direct-Teleop-Oberfläche → löst Problem → endSession() gibt Autonomie zurück
```

Die Leitstelle ist **Monitoring/Dispatch**, nicht Safety-Enforcement — das Fahrzeug verantwortet
seine eigene Sicherheit selbst (Sicherheitskonzept geklärt, Grill-Me 2026-07-13/14). Eine
Handshake-basierte Autonomie-Rückgabe (statt des einfachen `endSession()`) wurde bewusst
zurückgestellt (`FLEET-01`) — `endSession()` ist für den aktuellen Umfang ausreichend; das Risiko
(Fahrzeug erhält Kontrolle zurück, bevor es sicher verarbeitet ist) ist dokumentiert.

## 5. Fleet-Datenmodell & Service-Grenze (`ADR-029`)

Neuer, eigenständiger Go-Service `fleet-service`, orthogonal zum bestehenden `control-server`:

- **Geteilte Identität, getrennte Zuständigkeit:** Die bestehende `vehicles`-Tabelle bleibt
  einzige Quelle der Fahrzeug-Identität, erweitert um `vehicle_type`. `fleet-service` besitzt
  eigene, per Fremdschlüssel angehängte Tabellen (`vehicle_status`, `zones`, `stations`, `tasks`,
  `alerts`, seit Sprint 31/32 zusätzlich `task_status_history` und `vehicle_position_history`).
- **Keine Backend-zu-Backend-Kopplung:** Das Frontend führt `control-server` (Online-Status) und
  `fleet-service` (Typ/Batterie/Position/Tasks/Alerts) clientseitig zusammen — konsistent mit der
  vertraglichen Betonung auf der clientseitigen Visualisierungsschicht.
- **Kartendarstellung ohne externe Kartendienste:** Indoor und Outdoor beide SVG-basiert; Outdoor
  zusätzlich geo-referenziert per Leaflet-Overlay über bekannte GPS-Eckpunkte — bewusst keine
  SaaS-Kartenkacheln (Kostenbewusstsein, Vendor-Lock-in-Vermeidung, CLAUDE.MD Abschnitt 14).

## 6. Architektur-Zielprinzipien

Vom Auftraggeber vorgegeben und im Design konsequent umgesetzt:

- **Microservices** — `fleet-service` als eigenständiger, unabhängig deploybarer Service statt
  Erweiterung des sicherheitskritischen `control-server`
- **Event-Driven** — MQTT als Fahrzeug-zu-Leitstelle-Kanal (Fleet-Fahrzeuge sprechen nur MQTT,
  keine WebSocket-Verbindung zum Control Server); Live-Updates ans Dashboard per WebSocket-Broadcast
  ohne Polling
- **Interface over Implementation** — `FleetGateway`-Interface trennt `fleet-service` von der
  konkreten Fahrzeug-Anbindung; `MockGateway` (Simulation) und `MQTTGateway` (Produktivpfad)
  austauschbar, ohne dass die reale ROS2/DDS-Schnittstelle zum Blocker für den restlichen Aufbau wird
- **Security-First** — JWT-Authentifizierung für alle `/fleet/*`-Endpunkte (gleicher Auth-Service
  wie der bestehende Direct-Teleop-Stack)

## 7. Verhältnis zum bestehenden Sicherheits-Fundament

Die `NO_OPERATOR → SAFE_MODE`-Regel und die übrige 4-Layer State Machine (`ADR-009/011`) bleiben
unverändert gültig — aber nur innerhalb einer bereits aktiven Teleop-Session, nicht als
Dauerzustand für autonome Fahrzeuge ohne Session. Die bestehende Fahrzeug-WebSocket-Verbindung
(`/vehicle/ws`) ist bereits unabhängig von Sessions dauerhaft aktiv und erfüllt damit die
Anforderung "Fahrzeug ist immer mit der Leitstelle verbunden" ohne Codeänderung am bestehenden
Fundament.

## 8. Referenzen

| Thema | Dokument |
|---|---|
| Vision, Zielgruppen, Erfolgskriterien | `docs/vision.md` |
| Fachliche Anforderungen (Fleet + bestehend) | `docs/requirements.md` |
| Vollständige technische Architektur | `docs/architecture.md` |
| Domänenglossar, offene Fragen | `CONTEXT.MD` |
| Autonomy-First-Paradigma, Notfall-Trigger | `docs/adr/028-autonomy-first-operator-model.md` |
| Fleet-Datenmodell, Service-Grenze | `docs/adr/029-fleet-vehicle-data-model.md` |
| Externe Fahrzeugschnittstelle (vorläufig) | `docs/adr/027-fleet-gateway-interface.md` |
| Aktueller Sprint-/Task-Stand | `tasks/current-sprint.md`, `tasks/backlog.md` |

## 9. Offene Punkte für die vollständige Abnahme von Meilenstein 1

| Punkt | Status | Referenz |
|---|---|---|
| Workshop mit der Professur Logistik | ausstehend, externe Abhängigkeit | `AP1-01` |
| Abgleich `FleetGateway` gegen reale Schnittstelle | blockiert durch Workshop | `AP1-02` |
| Konkreter ROS2/DDS-Adapter | blockiert durch Workshop | `AP1-03` |
