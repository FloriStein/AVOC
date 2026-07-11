# vision.md

Stand: 2026-07-10 — überarbeitet nach Kurswechsel Richtung IBATOUR (siehe `ADR-027`, `CONTEXT.MD` Offene Fragen)

> **Hinweis zur Historie:** Dieses Dokument beschrieb ursprünglich ein reines
> Direct-Teleoperation-System (Einzelfahrzeug, manuelle Echtzeit-Steuerung per Joystick). Das war
> laut Auftraggeber bewusst der Anfang, kein Fehlschlag — das Projekt entwickelt sich jetzt gezielt
> in Richtung einer Leitstellensoftware für Fahrzeugflotten weiter. Die bisherigen
> Direct-Teleop-Fähigkeiten (Safety-Architektur, Video-Hub, State Machine) werden nicht verworfen,
> ihr genaues Verhältnis zur neuen Fleet-Leitstelle ist aber noch offen (siehe Abschnitt 11).

## 1. Projektvision

Das Ziel dieses Projekts ist die Entwicklung einer sicheren, modularen Leitstellensoftware zur
Überwachung und Steuerung von automatisierten Fahrzeugflotten in Echtzeit — im Rahmen des
Förderprojekts **IBATOUR** (Intelligentes Betriebsgelände für Autonomie in Transport, Organisation
und Umschlag via Routenzüge, gefördert vom Bundesministerium für Verkehr).

Das System soll es einem Operator ermöglichen, den Zustand einer Fahrzeugflotte (automatisierte
Routenzüge und andere Logistikfahrzeuge) auf einem Betriebsgelände zu überwachen, Prozesse
(Job-Zuweisung, Routenplanung) zu steuern, und bei Bedarf korrigierend oder im Notfall direkt
einzugreifen.

---

## 2. Problemstellung

Automatisierte Logistikfahrzeuge auf einem Betriebsgelände benötigen eine übergeordnete
Leitstelle, die Flottenzustand, Aufgabenverteilung und Sicherheit koordiniert. Die
fahrzeugseitige Automatisierung (ROS/ROS2) selbst ist nicht Gegenstand dieses Projekts — sie wird
von der Professur Logistik bereitgestellt.

Zentrale Herausforderungen:

- Überwachung mehrerer Fahrzeuge gleichzeitig (Status, Batterie, Position) in Echtzeit
- Aufgaben-/Routenmanagement für eine Flotte statt Einzelfahrzeugsteuerung
- Integration einer noch nicht final spezifizierten externen Fahrzeug-/ROS2-Schnittstelle (`ADR-027`)
- Sicherheitsrisiken bei Verbindungs- oder Steuerungsverlust — Konsequenzen für ein
  Betriebsgelände mit automatisierten Fahrzeugen sind eigenständig zu bewerten, nicht unreflektiert
  aus Annahmen für öffentlichen Straßenverkehr zu übernehmen (siehe Abschnitt 11)
- komplexe Echtzeit-Kommunikation (Video + Steuerung + Telemetrie), bei Bedarf für mehrere
  Fahrzeuge gleichzeitig

Dieses Projekt adressiert diese Probleme durch eine strukturierte, mehrschichtige
Systemarchitektur — aufbauend auf einem bereits validierten Fundament für sicherheitskritische
Einzelfahrzeug-Fernsteuerung (Sprint 1–19).

---

## 3. Ziel des Systems

Das System soll folgende Kernziele erfüllen:

- Echtzeit-Flottenüberwachung (Status, Position, Batterie, Aufgaben) über eine Web-Oberfläche
- Task-/Job-Management für automatisierte Routenzüge (Zuweisung, Priorisierung, Nachverfolgung)
- Alert-System mit Prioritätsklassen für kritische Ereignisse
- Administrationsebene für Nutzer-, Fahrzeug- und Systemverwaltung
- garantierte Sicherheitsmechanismen zur Vermeidung gefährlicher Zustände
- nachvollziehbare Session- und Entscheidungsprotokollierung
- Integration mit einer externen ROS2/DDS-basierten Fahrzeugschnittstelle (Details offen, `ADR-027`)

---

## 4. Zielgruppen

- Leitstellen-Operatoren, die eine Flotte automatisierter Routenzüge überwachen und disponieren
- Administratoren, die Nutzer, Fahrzeuge und Systemkonfiguration verwalten
- Der Auftraggeber IBATOUR / Professur Logistik als Betreiber des Betriebsgeländes
- Entwickler von autonomen / semi-autonomen Systemen (Schnittstellen-Integration)
- Forschungs- und Testumgebungen für Robotik und Mobilität

---

## 5. Kernfunktionen (High-Level)

**Web-Dashboard (AP2):**
- Flottenübersicht — Echtzeit-Fahrzeugstatus, Batterielevel mit Alert-System, Aufgabenverteilung
- Interaktive Kartenansicht — Werkshallenvisualisierung, Echtzeit-Positionsverfolgung, Routenübersicht
- Task Management — Aufgaben erstellen/zuweisen, Prozess-Status-Tracking, Prioritätenmanagement, Task-History
- Alert System — Echtzeit-Benachrichtigungen mit Prioritätsklassen, Audio-Alerts, Acknowledgement

**Admin-Konsole (AP3):**
- User Management — zentrale Nutzerverwaltung, Rollen/Rechte, Activity Monitoring
- System-Konfiguration — flottenübergreifende Parameter, Notification Settings, Betriebssicherheits-Einstellungen
- Fahrzeugmanagement — Registrierung, Konfiguration, Zonenzuweisung, Maintenance Tracking
- System-Gesundheit — Service Status, API-Gateway Health, System-Logs

**Bestehendes Fundament (Sprint 1–19, Rolle im Gesamtsystem noch zu klären — siehe Abschnitt 11):**
- Echtzeit-Teleoperation eines Einzelfahrzeugs (Joystick/Keyboard/Gamepad)
- Video-Streaming (WebRTC/WHIP/WHEP)
- Sicherheitsmechanismen (Dead-man Switch, Emergency Stop, SAFE_MODE, Per-Vehicle-Isolation)
- Verbindungsmanagement, Latenzüberwachung, Session Recording

---

## 6. Nicht-Ziele (Explicit Non-Goals)

Dieses System ist NICHT:

- die fahrzeugseitige Automatisierung selbst (ROS/ROS2-Stack, Hardware-Schnittstellen) — das liefert die Professur Logistik
- ein Consumer-Entertainment-Produkt
- ein Offline-Steuerungssystem ohne Netzwerkabhängigkeit
- ein rein lokales Embedded-Control-System ohne Cloud/Netzwerk
- (offen, zu klären) möglicherweise nicht primär ein Direct-Teleop-System — abhängig vom Ergebnis der Klärung in Abschnitt 11

---

## 7. Grundprinzipien

Die Architektur basiert auf folgenden Leitprinzipien:

- Safety First (Sicherheitsmechanismen haben höchste Priorität)
- Real-Time Communication (minimale Latenz ist kritisch für Steuerung und Flottenstatus)
- Multi-Channel Architecture (Trennung von Video, Control, Telemetry, Safety)
- Interface over Implementation (externe Abhängigkeiten — Fahrzeug-Backend, Safety Bus, Video-Router — gegen Interfaces bauen, nicht fest verdrahten)
- Failure Awareness (System muss Ausfälle erkennen und reagieren können)
- Explicit Design Decisions (alle wichtigen Entscheidungen werden dokumentiert, ADRs unveränderlich)

---

## 8. Qualitätsziele

- stabile Kommunikation unter variierenden Netzwerkbedingungen
- Reaktionszeit im Kontrollpfad unter 100ms (für Direct-Control-Anteile)
- robuste Fehler- und Disconnect-Behandlung, auch bei mehreren Fahrzeugen gleichzeitig
- klare Trennung von Sicherheits- und Steuerungssystemen
- nachvollziehbare Systemzustände zu jedem Zeitpunkt, pro Fahrzeug und flottenweit

---

## 9. Architekturleitlinie

Die endgültige Architektur wird nicht vorausgesetzt, sondern entsteht iterativ durch:

- Anforderungen (`docs/requirements.md`, inkl. Fleet Gateway Requirements)
- ADR-basierte Entscheidungen
- kontinuierliche Validierung durch Grill-Me Sessions

Externe Unsicherheiten (insbesondere die noch unbestätigte Fahrzeug-/ROS2-Schnittstelle) werden
explizit als vorläufig markiert und über Interfaces abstrahiert (`ADR-027`), statt stillschweigend
angenommen zu werden.

---

## 10. Erfolgskriterien

Das Projekt gilt als erfolgreich, wenn:

- ein Operator den Zustand der Flotte zuverlässig in Echtzeit überwachen kann
- Aufgaben/Routen für die Flotte disponiert und nachverfolgt werden können
- Sicherheitsmechanismen zuverlässig eingreifen — sowohl auf Flotten- als auch auf Einzelfahrzeugebene
- Verbindungsabbrüche keine unkontrollierten Zustände erzeugen
- die Admin-Konsole eine vollständige Verwaltung von Nutzern, Fahrzeugen und Systemkonfiguration ermöglicht
- das System modular erweiterbar bleibt (insbesondere: Fahrzeug-Backend austauschbar ohne Rewrite der Leitstellen-Logik)
- alle Architekturentscheidungen nachvollziehbar dokumentiert sind
- die drei Meilensteine (AP1 Architektur, AP2 Web-Dashboard, AP3 Admin-Konsole) gemäß Leistungsbeschreibung erfüllt werden

---

## 11. Offene strategische Fragen (bewusst nicht entschieden)

Diese Fragen sind Grill-Me-Stoff für kommende Sessions, nicht in diesem Dokument beantwortet —
sie werden hier bewusst offen gehalten statt stillschweigend entschieden:

- **Verhältnis Direct-Teleop-System zu Fleet-Leitstelle:** Wird das bestehende
  Control-Server/Safety-Bus/Deadman-System (a) als manueller Eingriffsmodus in die Leitstelle
  integriert, (b) bleibt es ein paralleles Subsystem, oder (c) läuft es aus, weil Routenzüge
  überwiegend routengeführt/autonom fahren?
- **Sicherheitskonzept für das Betriebsgelände:** Welches Regelwerk gilt (vermutlich
  Maschinensicherheit/DIN EN ISO 3691-4 für fahrerlose Flurförderzeuge, nicht
  Straßenverkehrsrecht) — und was bedeutet das konkret für Verhalten bei Video-/Verbindungsverlust?
- **Konkrete ROS2/DDS-Schnittstelle:** Ausstehend, Teil des AP1-Workshops mit der Professur Logistik (`ADR-027`)
- **Projektstatus:** Ist die Ausschreibung bereits beauftragt oder wird sie noch als Angebot vorbereitet?

Siehe auch `CONTEXT.MD` „Offene Fragen" für die technische Fortführung dieser Punkte.
