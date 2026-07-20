# ADR-031: Migration zu hexagonaler Architektur (Ports & Adapters) — Strangler-Fig, Pilot fleet-service

Status: Accepted (Strategie), **Pilot abgeschlossen (Sprint 33, 2026-07-18)** — Fortsetzung auf weitere Services ist ein bewusster Entscheidungspunkt nach Pilot-Abschluss, kein Automatismus (siehe "Offene Punkte")

## Kontext

Wartbarkeit und Onboarding (CLAUDE.MD Ziel: Architektur muss "ohne implizites Wissen
rekonstruierbar" sein) leiden darunter, dass mehrere Services Business-Logik, I/O und
HTTP-Handling vermischen, und dass CONTEXT.MD Prinzip 6 ("Interface over Implementation") in der
Praxis lückenhafter ist, als die zitierten ADRs (002/005/015) suggerieren. Auslöser für diesen
Plan (Grill-Me 2026-07-16) ist ausdrücklich **allgemeine Wartbarkeit/Onboarding**, nicht die noch
unspezifizierte ROS2/DDS-Anbindung (ADR-027) — deren Klärung bleibt an den AP1-Workshop mit der
Professur Logistik gebunden und wird hier nicht vorweggenommen.

Vollständige Bestandsaufnahme (parallele Analyse aller 7 Go-Services, 2026-07-16) ergab folgendes
Bild, wie nah jeder Service tatsächlich an "hexagonal" ist (nicht nur, wie es in Kommentaren/ADRs
behauptet wird):

| Service | Befund | Reifegrad |
|---|---|---|
| `fleet-service` | `internal/fleetgateway/gateway.go:51-63` ist ein echter Port (Interface + Mock-/MQTT-Adapter, korrekte Abhängigkeitsrichtung); `AlertEngine` ist bereits reine, I/O-freie Domain-Logik. Aber `Handler.store` ist an den konkreten Typ `*PostgresFleetStore` gebunden (`internal/fleetservice/handler.go:32,37`) — kein Repository-Port. Folge: ~20 von 24 Handler-Tests brauchen eine echte Postgres-Instanz statt eines Mocks. | Am nächsten an hexagonal; ein Gap, klar abgegrenzt |
| `auth-service` | `UserStore`-Interface bereits korrekt vom Handler konsumiert (`internal/authservice/userstore.go:37-45`), Handler-Tests laufen komplett gegen Stub ohne DB — auf dieser Dimension bereits weiter als fleet-service. Aber JWT-Erzeugung/-Validierung liegt direkt im Handler gegen `golang-jwt/jwt/v5` (`internal/authservice/handler.go:305-324`), kein Port, keine Use-Case-Schicht getrennt von HTTP. | Storage-Seite gelöst, JWT-Seite offen |
| `telemetry-service` | `internal/telemetryservice/client.go` — kleinster Service (~180 Zeilen), keine Interfaces, keine Tests, aber triviales reines Passthrough. | Billigster Einzel-Fix, aber wenig Lerneffekt für andere Services |
| `control-server` | Ältester, größter, safety-kritischster Service. `command.VehicleForwarder` (`internal/controlserver/command/engine.go:31-33`) ist der einzige wirklich korrekte Port. Sonst: Interface + HTTP-Adapter im selben Package (`safety/publisher.go`, `session/sfu_publisher.go` — kein Kern/Adapter-Split), MediaMTX-Client ganz ohne Abstraktion (`cmd/control-server/main.go:52,395`), sicherheitskritische Orchestrierung (State-Transition/Audit/Publish) inline in HTTP-Handler-Closures an ~6 Stellen mit Reihenfolge-Semantik nur in Kommentaren. | Höchstes Risiko, geringste Testabdeckung genau dort, wo Migration am meisten Schaden anrichten könnte |
| `safety-service` | Das in ADR-002 beschriebene Interface existiert nicht als Go-Code — `Bus` ist ein konkreter Typ, 0% Testabdeckung. "DDS-Austauschbarkeit" funktioniert nur zufällig, weil der Consumer ohnehin nur HTTP spricht. | Interface nur auf dem Papier |
| `webrtc-sfu`/`recording` | `SessionRecorder`-Interface existiert wörtlich wie in ADR-005 beschrieben (`internal/recording/recorder.go:26-33`), wird aber **nirgends** als Interface-Typ verwendet — `cmd/control-server/main.go:102` nutzt durchgängig den konkreten `*MemoryRecorder`. Null Tests in allen drei Paketen. | Port auf dem Papier, nie erzwungen |
| `vehicle-mock` | Test-Double; `cmd/vehicle-mock/fleet_simulator.go` trennt bereits vorbildlich reine Simulationslogik von I/O. | Kein Migrationsbedarf — bewusst ausgenommen |

Kernbefund: Prinzip 6 ("Interface over Implementation") ist im Projekt bisher eher Absicht als
durchgesetzte Realität. `fleetgateway` (ADR-027) ist der einzige Service-Port, der tatsächlich das
tut, was die Architektur-Prinzipien versprechen — er dient in diesem ADR als Vorbild, nicht die
ADR-002/005-Texte.

**Scope-Entscheidung (Grill-Me 2026-07-16):** Nur die sieben Go-Backend-Services. Das Frontend
folgt einem funktionierenden Hooks/Components/lib-Muster ohne die hier beschriebenen Symptome
(verwobene Business-Logik mit I/O, Interfaces nur auf dem Papier) und bleibt außerhalb dieses ADRs.

**Zeithorizont (Grill-Me 2026-07-16):** Nachgelagert nach AP2/AP3-Meilensteinen. Der
Fördervertrag koppelt Zahlungen an diese Meilensteine (Web-Dashboard/Admin-Konsole); eine
Architekturmigration parallel zu den laufenden Dashboard-Sprints (23ff., bereits mehrere parallele
Worktrees aktiv) würde dieses Risiko unnötig erhöhen. Dieses ADR legt die Strategie und den
Pilot-Sprint-Plan fest, **ohne** einen Startzeitpunkt festzulegen — der Startzeitpunkt wird
gesondert nach AP2/AP3-Abschluss entschieden.

## Zieldefinition: Was "hexagonal" für dieses Projekt konkret bedeutet

Kein abstraktes Ports&Adapters-Lehrbuchmodell, sondern konkret drei Regeln, angewendet auf
bestehenden Code:

1. **Domain-Kern**: Business-Logik (State-Übergänge, Validierung, Berechnung) als reine
   Funktionen/Typen ohne Netzwerk-/DB-Zugriff. Vorbild: `AlertEngine` in `fleet-service` (bereits
   so), `fleet_simulator.go` in `vehicle-mock` (bereits so).
2. **Driving Ports**: HTTP-Handler rufen Anwendungsfälle (Funktionen/Methoden des Domain-Kerns)
   auf, nicht direkt Repositories. Handler bleiben dünn: Request parsen → Use-Case aufrufen →
   Response schreiben.
3. **Driven Ports**: Alles, was nach außen geht (Postgres, MQTT, MediaMTX, externe APIs), wird
   durch ein vom **Consumer** definiertes Go-Interface abstrahiert (Dependency Inversion — die
   Adapter-Implementierung importiert den Domain-Kern, nie umgekehrt), mit Compile-Time-Check
   (`var _ Interface = (*Concrete)(nil)`) wie bereits in `fleetgateway/gateway.go` vorgeführt.

### Konkretes Vorher/Nachher-Beispiel (fleet-service, der bestätigte Pilot)

**Vorher** (`internal/fleetservice/handler.go:32,37`):
```go
type Handler struct {
    store *PostgresFleetStore // konkreter Typ, hart an Postgres gebunden
}
```
Folge: Jeder Handler-Test, der `Handler` instanziiert, braucht eine echte Postgres-Verbindung
(`DATABASE_URL`) — ~20 von 24 Tests.

**Nachher** (Zielbild, analog zu `fleetgateway.FleetGateway`):
```go
type FleetStore interface {
    ListZones(ctx context.Context) ([]Zone, error)
    ListStations(ctx context.Context) ([]Station, error)
    // ... übrige Methoden von *PostgresFleetStore
}

type Handler struct {
    store FleetStore // Port statt konkreter Typ
}

var _ FleetStore = (*PostgresFleetStore)(nil) // Compile-Time-Check, wie in gateway.go
```
Ein `FakeFleetStore` (In-Memory) implementiert dasselbe Interface für Tests — Handler-Tests laufen
danach ohne Postgres. Das ist der komplette Umfang der Handler-seitigen Migration; keine
Business-Logik ändert sich, nur die Abhängigkeitsrichtung.

> **Update (2026-07-18, Sprint 33 — Start-Entscheidung):** entgegen der ursprünglichen
> "nachgelagert nach AP2/AP3"-Planung oben hat der Nutzer den Pilotstart bereits nach Sprint 32
> freigegeben — AP2 war zu diesem Zeitpunkt zu 6 von 7 Anforderungsbereichen fertig (nur `AP2-02`
> noch offen, selbst Teil von Sprint 33), AP3 existiert im Repo weiterhin nicht als eigenes EPIC.
> Begründung: HEX-01..05 sind bereits einzeln klein geschnitten (S/M) und laut Scope-Entscheidung
> oben unabhängig von AP3-Inhalten. Kein Widerspruch zur Meilenstein-Kopplungs-Begründung —
> lediglich eine bewusste Vorziehung, keine Aufweichung der ursprünglichen Argumentation.
>
> **Umsetzung, eine Abweichung vom Code-Beispiel oben:** das "Nachher"-Beispiel zeigte
> `ctx context.Context`-Parameter — rein illustrativ, keine verbindliche Signaturvorgabe.
> `*PostgresFleetStore`s tatsächliche Methoden (`internal/fleetservice/store.go`) verwenden
> projektweit kein `context.Context` (keine der bestehenden Store-Interfaces im Repo tut das,
> z. B. `vehicleregistry.VehicleStore`), daher übernimmt `FleetStore` exakt die bestehenden
> Signaturen ohne `ctx` — Konsistenz mit dem Rest der Codebasis hat Vorrang vor dem Beispiel in
> diesem ADR.
>
> **Ergebnis:** `FleetStore`-Interface (19 Methoden, deckt `*PostgresFleetStore` vollständig ab,
> nicht nur `Handler`s tatsächlichen Bedarf — Interface-Segregation ist bewusst GOSTYLE-IF-*-Scope,
> siehe `tasks/backlog.md`), `Handler.store`/`NewHandler` auf das Interface umgestellt,
> `FakeFleetStore` (In-Memory, wiederverwendet interne Sentinel-Fehler/Transitionstabelle aus
> `store.go` für identisches Verhalten) implementiert. Alle 30 zuvor `DATABASE_URL`-gebundenen
> Handler-Tests migriert und liefen anschließend tatsächlich (nicht nur "übersprungen ohne
> Fehler") ohne `DATABASE_URL` durch. Store-Layer-Integrationstests (`store_test.go`,
> `integration_test.go`, `edgecases_test.go`, `lifecycle_test.go`, `positionhistory_test.go`)
> bleiben bewusst Postgres-gebunden — sie testen `PostgresFleetStore` selbst, nicht `Handler`,
> und bleiben die CLAUDE.MD-Abschnitt-17-Pflichtabdeckung "mindestens ein Test über die echte
> Prozessgrenze" für dieses Paket.

## Risikobewertung & Strangler-Fig-Ansatz

Ein Big-Bang-Rewrite ist angesichts von ADR-006 (umfangreiche Testabdeckung), mehreren bereits
produktiv deployten Services (control-server, auth-service, telemetry-service, webrtc-sfu) und
fehlendem zeitlichen Druck (Auslöser ist Wartbarkeit, keine akute Störung) nicht zu rechtfertigen.
Gewählt wird ein **inkrementeller Strangler-Fig-Ansatz**: ein Service als Pilot, danach expliziter
Entscheidungspunkt vor jeder Fortsetzung — keine automatische Kettenreaktion auf alle sieben
Services.

Priorisierung basierend auf der Bestandsaufnahme (Risiko × Aufwand × Lerneffekt):

1. **fleet-service (Pilot, bestätigt)** — jüngster Service, `fleetgateway` liefert bereits das
   Vorbild im selben Service, einziger Gap ist ein sauber abgegrenzter Repository-Port. Niedrigstes
   Risiko, höchster Übertragungswert für die anderen Services.
2. **auth-service (Kandidat für Folge-Sprint)** — Storage-Seite bereits gelöst, JWT-Port + Use-Case-
   Extraktion wäre eine andersartige, aber ähnlich kleine Übung.
3. **telemetry-service (Kandidat für Folge-Sprint)** — kleinster/billigster Fall, aber triviale
   Logik; eher ein schneller Nebeneffekt als ein Lernschritt.
4. **safety-service, webrtc-sfu/recording** — Interfaces existieren nominell (ADR-002/005), aber
   0 Tests und keine echte Erzwingung. Vor einer Migration hier müsste zuerst überhaupt
   Testabdeckung aufgebaut werden — höherer Aufwand, niedrigere Priorität.
5. **control-server (ausdrücklich zuletzt, wenn überhaupt)** — größter Service, produktiv
   deployed, sicherheitskritische Orchestrierung inline mit nur in Kommentaren festgehaltener
   Reihenfolge-Semantik und dünner Handler-Testabdeckung genau an den riskantesten Stellen. Jede
   Migration hier braucht zuerst eine eigene, gesonderte Risikoanalyse — nicht Teil dieses Plans.

Dieses ADR beauftragt **ausschließlich Schritt 1**. Schritte 2-5 sind Optionen für einen späteren,
gesondert zu treffenden Entscheidungspunkt (siehe "Offene Punkte" unten) — kein Freibrief, alle
sieben Services nacheinander abzuarbeiten.

## Optionen

### Option A: Big-Bang-Rewrite aller 7 Services auf hexagonale Architektur

**Vorteile:** Einheitliches Ergebnis in einem Zug, kein Zwischenzustand mit gemischten Mustern

**Nachteile:** Widerspricht ADR-006 (umfangreiche bestehende Testabdeckung, Safety-kritisch);
mehrere Services sind produktiv deployed — ein Rewrite ohne inkrementelle Verifikation ist bei
diesem Risikoprofil nicht vertretbar; kein Nutzer-sichtbarer Nutzen rechtfertigt das Risiko

### Option B: Keine formale Migration — hexagonale Muster nur "opportunistisch" beim Anfassen von Code einführen

**Vorteile:** Kein dedizierter Aufwand, kein Risiko durch gezielte Umbauten

**Nachteile:** Genau dieses Muster hat bereits zu den "Port auf dem Papier"-Fällen geführt
(ADR-002/005) — ohne konkreten Plan bleibt es bei Absicht statt Durchsetzung; kein messbarer
Fortschritt, keine Grundlage für eine spätere Entscheidung über Fortsetzung/Abbruch

### Option C: Inkrementeller Strangler-Fig, ein Pilot-Service, expliziter Entscheidungspunkt danach (gewählt)

**Vorteile:**
- Risiko auf einen einzigen, bereits am nächsten an hexagonal liegenden Service begrenzt
- Liefert nach dem Piloten eine belastbare Aufwandsschätzung für die übrigen Services statt einer
  Vermutung
- Kein Zwang, alle Services zu migrieren — Fortsetzung ist ein separater, informierter Entscheid
- Konsistent mit dem im Projekt etablierten "Interface over Implementation"-Vorgehen, diesmal aber
  mit Verifikation (Tests laufen ohne DB) statt nur Behauptung

**Nachteile:**
- Für einen Zeitraum bestehen gemischte Muster im Code (fleet-service teilweise migriert, andere
  Services unverändert) — akzeptabel, da klar dokumentiert und lokal auf einen Service begrenzt
- Erfordert Disziplin, den Piloten wirklich abzuschließen und zu verifizieren, bevor über
  Fortsetzung entschieden wird, statt ihn liegen zu lassen

## Entscheidung

Wir wählen **Option C**: `fleet-service` als alleiniger Pilot, beauftragt durch den Sprint-Plan in
`tasks/backlog.md` (EPIC "Hexagonale Architektur-Migration — Pilot fleet-service"). Umfang: nur
der fehlende `FleetStore`-Repository-Port (siehe Vorher/Nachher oben). Start nachgelagert nach
Abschluss von AP2/AP3 (Grill-Me 2026-07-16) — kein fixes Datum in diesem ADR.

## Begründung

Der Pilot ist der Service mit dem kleinsten Abstand zum Zielbild (nur ein Gap: Repository-Port),
hat mit `fleetgateway` bereits einen im selben Service funktionierenden Beweis, dass das Muster
hier funktioniert, und sein Erfolgskriterium ist objektiv verifizierbar (Handler-Tests laufen ohne
`DATABASE_URL`). control-server explizit auszuschließen folgt direkt aus der Bestandsaufnahme:
höchstes Risiko, geringste Testabdeckung an den kritischsten Stellen — eine Migration dort ist
kein Fortsetzungs-Kandidat ohne eigene, gesonderte Risikoanalyse. Die Verschiebung auf "nach
AP2/AP3" folgt aus der vertraglichen Meilenstein-Kopplung: der Auslöser ist Wartbarkeit, kein
akuter Schmerzpunkt, der eine Kollision mit laufenden Dashboard-Sprints rechtfertigen würde.

## Konsequenzen

### Positiv
- Belegbarer Fortschritt bei Prinzip 6 ("Interface over Implementation") an genau der Stelle, die
  aktuell die meisten DB-gebundenen Tests erzeugt
- Objektives Erfolgskriterium (Tests ohne Postgres) statt eines subjektiven "fühlt sich sauberer an"
- Liefert nach Abschluss eine belastbare Aufwandsschätzung für auth-/telemetry-service, falls eine
  Fortsetzung beschlossen wird
- Kein Risiko für bereits deployte, sicherheitskritische Services (control-server bleibt unberührt)

### Negativ
- Gemischte Architektur-Muster zwischen Services (und vorübergehend auch innerhalb von
  fleet-service, falls die Use-Case-Extraktion aus HEX-06 zurückgestellt wird) für die Dauer der
  Migration
- Ohne diszipliniertes Nachfassen (Entscheidungspunkt nach Pilot, siehe unten) bleibt es bei einem
  Einzelfall statt einer projektweiten Verbesserung
- Startzeitpunkt hängt vom AP2/AP3-Fortschritt ab, ist also nicht fest planbar

## Offene Punkte / Nächste Schritte

- Nach Abschluss des Piloten (HEX-01..05, siehe `tasks/backlog.md`): expliziter Entscheidungspunkt,
  ob und in welcher Reihenfolge auth-service/telemetry-service folgen — kein Automatismus, siehe
  Risikobewertung oben
- control-server bleibt bis auf Weiteres explizit ausgeklammert; eine spätere Migration dort
  bräuchte ein eigenes, gesondertes ADR mit eigener Risikoanalyse (Testabdeckung müsste zuerst
  aufgebaut werden)
- safety-service und webrtc-sfu/recording brauchen vor einer Migration zunächst überhaupt
  Testabdeckung — nicht Teil dieses Plans
- Startzeitpunkt (nach AP2/AP3) wird gesondert festgelegt, sobald der Meilenstein-Fortschritt das
  erlaubt — dieses ADR legt nur die Strategie fest, keinen Termin
