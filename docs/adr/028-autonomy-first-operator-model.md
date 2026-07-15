# ADR-028: Autonomy-First Operator Model — Notfall-Trigger statt Dauerbetrieb

Status: Accepted

## Kontext

Sprint 1–19 haben ein System gebaut, das implizit von **kontinuierlicher Fernsteuerung als
Normalfall** ausgeht: `ADR-009` (Failure Model) und `ADR-011` (State Machine) legen fest, dass
`NO_OPERATOR` ein CRITICAL-Trigger ist, der `SAFE_MODE` auslöst — sinnvoll, wenn ein Fahrzeug
üblicherweise gerade gefahren wird und ein verschwindender Operator ein Sicherheitsproblem ist.

Grill-Me-Session 2026-07-13/14 hat das Grundparadigma für IBATOUR geklärt: Die Fahrzeuge
(Lastenrad, Lastenzug) fahren **autonom als Normalfall** auf dem Betriebsgelände. Teleoperation
ist die **Ausnahme**, ausgelöst durch vom Fahrzeug selbst erkannte Probleme (z. B. Hindernis auf
der Strecke), die einen Operator-Eingriff erfordern. Damit ist "kein aktiver Operator" nicht mehr
der Ausnahmefall, sondern der **Normalzustand** der Flotte — die bestehende Regel würde in dieser
Lesart jedes autonom fahrende Fahrzeug dauerhaft in `SAFE_MODE` versetzen. Das ist nicht intendiert
und muss präzisiert werden, ohne `ADR-009`/`ADR-011` stillschweigend zu überschreiben.

Geklärte Eckpunkte aus der Grill-Me-Session:

- Immer mindestens ein Operator im Dienst, wenn die Flotte autonom fährt (Aufsichtspflicht)
- Alle Fahrzeuge halten eine dauerhafte WebSocket-Verbindung zum Control Server — **unabhängig**
  von einer aktiven Teleop-Session (verifiziert: `/vehicle/ws` ist bereits heute eine eigene Route,
  entkoppelt von `session/start` — keine Codeänderung nötig, nur explizite Dokumentation dieser
  bereits bestehenden Eigenschaft als Invariante)
- Fahrzeuge erkennen selbstständig Probleme, die einen Eingriff erfordern, und senden das als
  Alert (Weg: künftige FleetGateway-Schnittstelle → `fleet-service`, `ADR-027/029`)
- Operator sieht den Alert im Notification-System und wählt das betroffene Fahrzeug aus einem
  Dropdown aus (bestehendes `VehicleSelector`-Muster) — kein automatisches Queue/Claim-System
- Mehrere Operatoren an unterschiedlichen Arbeitsplätzen möglich, jeder kontrolliert jeweils
  genau ein Fahrzeug gleichzeitig (deckt sich mit der bestehenden Ein-Active-Operator-pro-
  Fahrzeug-Regel, jetzt operator-seitig zusätzlich bestätigt)
- Nach Problemlösung (z. B. Hindernis umfahren) gibt der Operator die Kontrolle zurück — **für den
  Anfang reicht dafür das bestehende `endSession()`**, ein expliziter Handshake mit dem Fahrzeug
  ist bewusst zurückgestellt (siehe `tasks/backlog.md`)

## Entscheidung

**Die `NO_OPERATOR`→`SAFE_MODE`-Regel aus `ADR-009`/`ADR-011` gilt weiterhin, aber ausschließlich
innerhalb einer bereits aktiven Control Session** — nicht als Dauerzustand für Fahrzeuge, die sich
gar nicht in einer Teleop-Session befinden. Konkret:

- Ein Fahrzeug ohne aktive Control Session befindet sich in **eigenständigem Autonomiebetrieb** —
  das ist außerhalb des Geltungsbereichs der bestehenden 4-Layer State Machine des Control Servers.
  Diese State Machine beschreibt weiterhin ausschließlich den **Lebenszyklus einer Teleop-Session**,
  nicht den Autonomiebetrieb selbst (der wird von `fleet-service`/der externen ROS2-Automatisierung
  verwaltet, `ADR-027/029`)
- Sobald eine Control Session gestartet wird (Operator übernimmt nach Alert), gilt die bestehende
  Regel unverändert: verschwindet der Operator während einer **aktiven** Session, ist das weiterhin
  CRITICAL → `SAFE_MODE`. Das Sicherheitsniveau während einer Intervention ändert sich nicht.
- Notfall-Trigger: Fahrzeug → Alert (`fleet-service`) → Operator wählt Fahrzeug aus Dropdown →
  bestehender `session/start`-Fluss (unverändert) → Operator löst Problem → `endSession()`
  (unverändert) → Fahrzeug erhält Autonomie zurück

## Begründung

Diese Lesart erfordert **keine Änderung an der bestehenden State-Machine-Implementierung** — die
Regel war technisch nie "global", sondern immer im Kontext einer Session formuliert
(`internal/controlserver/statemachine`). Die Präzisierung ist in erster Linie eine
**Dokumentationskorrektur**: `ADR-009`/`ADR-011`/`requirements.md` beschrieben den Geltungsbereich
bisher nicht explizit genug, weil "kein Operator" beim ursprünglichen Direct-Teleop-PoC praktisch
immer session-bezogen gemeint war und dieser Fall nie auftrat. Jetzt, wo "kein Operator" der
Normalfall ist, muss die Abgrenzung explizit gemacht werden, damit niemand fälschlich annimmt, ein
autonom fahrendes Fahrzeug müsse dauerhaft eine Session halten.

Der Notfall-Trigger-Fluss (Alert → Dropdown-Auswahl → bestehender Session-Start) erfordert ebenfalls
**keine neue UI-Interaktionsform** — das bestehende `VehicleSelector`-Muster wird wiederverwendet,
nur der Auslöser (Alert statt freie Wahl) ist neu. Das reduziert Implementierungsrisiko gegenüber
einem komplett neuen Zuweisungsmodell (z. B. Queue/Claim-System), das explizit nicht gewünscht ist.

## Konsequenzen

### Positiv
- Keine Änderung an bestehender, bereits getesteter Safety-Architektur nötig (Deadman-Switch, SAFE_MODE, Emergency Stop bleiben wie sie sind)
- Vehicle-WS-Konnektivität (`ADR-021`) erfüllt die "immer verbunden"-Anforderung bereits ohne Codeänderung
- Bestehendes `VehicleSelector`/Session-Start/-Ende-UI-Muster wird wiederverwendet statt neu gebaut

### Negativ
- `ADR-009`/`ADR-011`/`requirements.md` müssen um den Geltungsbereichs-Hinweis ergänzt werden (Dokumentation, keine ADRs überschreiben — Verweis auf dieses ADR ergänzen)
- Der Handshake-basierte Autonomie-Rückgabe (statt einfachem Session-Ende) bleibt offen — Risiko: Fahrzeug könnte Kontrolle "zurückerhalten", bevor es tatsächlich sicher ist, dies zu verarbeiten. Für den aktuellen Stand akzeptiert, siehe `tasks/backlog.md`
- Notfall-Alert-Zustellung hängt jetzt vollständig an der noch unspezifizierten FleetGateway-Schnittstelle (`ADR-027`) — kein Notfall-Trigger ohne diese Schnittstelle nutzbar, bis der AP1-Workshop stattgefunden hat

---

## Update (2026-07-15) — Proaktive Übernahme zusätzlich zum Notfall-Trigger

Grill-Me zum Fleet-Overview-Dashboard (Sprint 22) hat den Notfall-Trigger-Fluss oben um einen
zweiten, gleichberechtigten Auslöser ergänzt: ein Operator darf ein Fahrzeug auch **proaktiv**
übernehmen — nicht nur als Reaktion auf einen zugestellten Alert —, solange das Fahrzeug aktuell
**keinen aktiven Operator** hat. Der Alert-Fluss aus der ursprünglichen Entscheidung bleibt
unverändert das primäre Signal, das einem Operator anzeigt, *welches* Fahrzeug Aufmerksamkeit
braucht; die proaktive Übernahme ist eine zusätzliche, alertunabhängige Handlungsmöglichkeit
(z. B. wenn ein Operator im Fleet Overview ohnehin sieht, dass ein Fahrzeug in einer Zone
feststeckt, ohne dass die Schwellenwert-/Vehicle-Alert-Logik das bereits als Alert erkannt hat).

**Konkretisierung:** "kein aktiver Operator" heißt hier — wie im Rest dieses ADRs — bezogen auf
den Geltungsbereich der 4-Layer State Machine des Control Servers: der `OPERATOR`-Layer für dieses
Fahrzeug steht auf `NO_OPERATOR` (kein laufender `session/start`). Der bestehende `session/start`-
Fluss (unverändert, kein neues Zuweisungsmodell) läuft in beiden Fällen identisch — nur der
Auslöser unterscheidet sich (Alert-Klick vs. direkter "Teleoperate"-Klick im Fleet Overview auf ein
Fahrzeug ohne Operator). Die bestehende Ein-Active-Operator-pro-Fahrzeug-Regel gilt unverändert:
ein Fahrzeug, das bereits einen aktiven Operator hat, kann nicht zusätzlich proaktiv übernommen
werden (der Button ist für dieses Fahrzeug in diesem Fall nicht verfügbar).

Keine Änderung an der Safety-Architektur, an `session/start`/`endSession()` oder an der
Handshake-Rückstellung (weiterhin `tasks/backlog.md`) — reine Erweiterung, *wie* eine Session
ausgelöst werden darf, nicht *was* während einer Session gilt.
