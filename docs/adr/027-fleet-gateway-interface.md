# ADR-027: Fleet Gateway Interface — Abstraktion für externes Vehicle-Backend (IBATOUR)

Status: Accepted (Strategie) — konkrete Schnittstellenparameter vorläufig, unbestätigt

## Kontext

Das Projekt entwickelt sich von einem Direct-Teleoperation-Proof-of-Concept (Einzelfahrzeug,
manuelle Echtzeit-Steuerung) hin zu einer Leitstellensoftware für das Förderprojekt **IBATOUR**
(Intelligentes Betriebsgelände für Autonomie in Transport, Organisation und Umschlag via
Routenzüge, gefördert vom Bundesministerium für Verkehr).

Laut Leistungsbeschreibung werden fahrzeugseitige Automatisierung, Hardware-Schnittstellen
(ROS/ROS2) und das fahrzeugseitige Backend von der Professur Logistik bereitgestellt — **nicht**
Teil dieses Auftrags. Gegenstand dieses Projekts ist die übergeordnete Leitstellen-Infrastruktur
mit Schwerpunkt auf der clientseitigen Visualisierungsschicht (AP2 Web-Dashboard, AP3
Admin-Konsole).

Zum jetzigen Zeitpunkt (2026-07-10) liegt **keine konkrete Spezifikation** der externen
Schnittstelle vor — keine ROS2-Topic-Namen, kein Nachrichtenformat, keine Aussage ob DDS direkt
oder über eine Bridge angebunden wird. Bekannt sind nur Ziel-Designprinzipien aus einer
Architektur-Vorlage des Auftraggebers:

- Event-Driven Communication via ROS 2 DDS (ROSbridge + WebSocket als Fallback für ROS1)
- Multi-Protocol: DDS lokal, MQTT/Zenoh für WAN, WebSocket für Web-Clients
- Time-Series-Optimierung für hochfrequente Telemetrie
- Security-First (Ende-zu-Ende-Verschlüsselung, Zero-Trust)

Laut Leistungsbeschreibung ist die Klärung der konkreten Schnittstelle explizit Teil von **AP1**
("Einarbeitung in vorhandene Programmierschnittstellen für ausgewählte Fahrzeuge und Design der
Softwarearchitektur und Schnittstellen") — gemeinsame Workshops mit dem Auftraggeber sind
vertraglich vorgesehen. Auf das Ergebnis dieser Workshops zu warten, bevor AP2/AP3 beginnen,
würde beide nachfolgenden Meilensteine (und die daran gekoppelten Zahlungen) verzögern.

## Optionen

### Option A: Warten auf finale Schnittstellenspezifikation

**Vorteile:**
- Kein Risiko einer Fehlannahme, kein späteres Rework

**Nachteile:**
- Blockiert AP2/AP3 vollständig bis zum Workshop-Termin
- Verzögert Meilenstein 2/3 ohne Not — die Leitstellen-eigene Logik (UI, Auth, Task-Datenmodell) hängt nicht zwingend an der externen Schnittstelle

### Option B: Direkt gegen angenommene ROS2/DDS-Schnittstelle fest verdrahten

**Vorteile:**
- Kein zusätzlicher Abstraktionsaufwand

**Nachteile:**
- Bei Abweichung der echten Schnittstelle muss potenziell die gesamte Fleet-Datenschicht (Datenmodell, Parsing, State-Mapping) neu geschrieben werden
- Widerspricht dem im Projekt bereits etablierten Prinzip 6 ("Interface over Implementation", `CONTEXT.MD`)

### Option C: Abstraktes Fleet-Gateway-Interface jetzt definieren, Mock/Adapter dagegen bauen (gewählt)

**Vorteile:**
- Entkoppelt Web-Dashboard (AP2) und Admin-Konsole (AP3) von der noch unbekannten externen Schnittstelle — beide können sofort starten
- Konsistent mit bereits etabliertem Architekturmuster im Projekt (Safety Bus/Session Recording/SFU gegen Interfaces — `ADR-002/005/015`)
- Austausch der konkreten Implementierung später betrifft nur den Adapter, nicht die Leitstellen-Logik
- Zeigt gegenüber dem Auftraggeber Fortschritt in AP2/AP3, ohne den Workshop-Termin zu blockieren

**Nachteile:**
- Mock-Annahmen (DDS/ROSbridge, MQTT/Zenoh) könnten von der echten Schnittstelle abweichen — Rework-Risiko bleibt bestehen, ist aber auf den Adapter begrenzt statt auf die gesamte Fleet-Schicht
- Erfordert Disziplin, die Annahme durchgängig als vorläufig zu kennzeichnen, damit sie nicht versehentlich als bestätigte Spezifikation missverstanden wird

## Entscheidung

Wir wählen **Option C**: Ein abstraktes `FleetGateway`-Interface wird jetzt definiert, mit einer
Mock-Implementierung (Simulationsdaten, analog zum bestehenden `vehicle-mock`-Muster aus
`ADR-021`) dahinter. Die konkrete Anbindung an das externe ROS2/DDS-Backend erfolgt als
austauschbarer Adapter, sobald die Schnittstelle im Rahmen von AP1 geklärt ist.

## Begründung

Das Projekt hat mit `ADR-002` (Safety Bus), `ADR-005` (Session Recording) und `ADR-020`
(MediaMTX als austauschbarer WHIP/WHEP-Router) bereits mehrfach erfolgreich demonstriert, dass
Interface-first-Design Architekturrisiko bei unsicheren externen Abhängigkeiten reduziert, ohne
Fortschritt zu blockieren. Die gleiche Strategie auf die noch unbestätigte IBATOUR-Fahrzeug-
Schnittstelle anzuwenden ist konsistent und risikoärmer als Warten (Option A) oder Festverdrahtung
(Option B).

## Architektur (vorläufige Annahme — unbestätigt bis Workshop mit Professur Logistik)

```
Vehicle (ROS2, DDS)
   │ DDS pub/sub (lokal) — oder ROSbridge + WebSocket, falls ROS1
   ▼
[Externes Fahrzeug-Backend — Professur Logistik, außerhalb Scope dieses Projekts]
   │ MQTT oder Zenoh (WAN-Transport, Annahme)
   ▼
FleetGateway (Interface, in dieser Codebase — Sprint TBD)
   │ Implementierung: Mock (jetzt) → Adapter an reale Schnittstelle (nach AP1-Workshop)
   ▼
Leitstellen-Backend (Fleet-/Task-/Alert-API für AP2, Verwaltungs-API für AP3)
   │ WebSocket / REST
   ▼
Operator Browser (React Dashboard — Web-Dashboard + Admin-Konsole)
```

## Konsequenzen

### Positiv
- AP2 (Web-Dashboard) und AP3 (Admin-Konsole) können sofort gegen einen Mock entwickelt werden, ohne auf den Workshop-Termin zu warten
- Konsistent mit dem bereits etablierten Interface-over-Implementation-Prinzip
- Reales Abweichungsrisiko wird auf den Adapter begrenzt, nicht auf die gesamte Fleet-Datenschicht

### Negativ
- Doppelte Arbeit möglich, falls Mock-Annahmen stark von der echten Schnittstelle abweichen
- Bis zur Bestätigung bleibt ein offenes Architekturrisiko bestehen (siehe `CONTEXT.MD` Offene Fragen)
- Zusätzlicher Koordinationsaufwand (Workshop-Termin mit AG) als Voraussetzung für die endgültige Adapter-Implementierung

## Offene Punkte / Nächste Schritte

- Workshop mit der Professur Logistik zur Klärung der konkreten ROS2-Topics, Nachrichtenformate und DDS-vs-Bridge-Frage (vertraglich Teil von AP1)
- `FleetGateway`-Interface nach dem Workshop ggf. anpassen — bei signifikanter Abweichung: **neues ADR**, dieses ADR nicht überschreiben
- Diese Annahme ist in `requirements.md` (Fleet Gateway Requirements) und `CONTEXT.MD` (Offene Fragen) verlinkt, damit sie nicht in Vergessenheit gerät
