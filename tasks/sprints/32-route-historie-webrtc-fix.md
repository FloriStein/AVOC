# Sprint 32 — Klärungsrunde + Route-Historie, WebRTC-Fix, Meilenstein-Dokumente

Ziel: Anders als in Sprint 27–31 gab es diesmal keine bequeme Menge fertig geschnittener
Backlog-Tasks — fast alle offenen Punkte brauchten zuerst eine Klärung (Grill-Me-Session oder
explizite Nutzerentscheidung), bevor sie überhaupt geschnitten werden konnten. Triage
2026-07-18 (CLAUDE.MD Abschnitt 10):

- **FLEET-01** (Handshake-Autonomie-Rückgabe): geprüft, ob es seit `ADR-028` einen neuen Anlass
  zur Revision gibt — Nutzerantwort: nein, weiter zurückgestellt (kein Code-Task).
- **FLEET-02/AP2-03** (Persistenzform "gefahrene Route"): Grill-Me-Ergebnis — einfache
  Postgres-Tabelle statt Zeitreihen-DB (Flottengröße rechtfertigt keine neue
  Infrastrukturkomponente). Typ L (neue Datenstruktur) → `ADR-033` vor Umsetzung.
- **WEBRTC-10** (SDP-Workaround inkompatibel mit aktuellem Chromium): technische Vorab-Prüfung
  ergab, dass `mediamtx:latest` inzwischen eine komplett andere Pion-Version einsetzt als beim
  ursprünglichen Bug — Nutzerentscheidung: umsetzen und lokal verifizieren.
- **AP1-04/AP2-05** (Abnahme-Dokument-Bedarf für Meilenstein 1/2): explizite Nutzerentscheidung
  (kein Code-Task, CLAUDE.MD Abschnitt 4) — beide Meilensteine sollen ein eigenständiges Dokument
  bekommen.
- **AP1-01..03** (Workshop Professur Logistik), **HEX-01..06**, **GOSTYLE-IF-01..06**: weiterhin
  extern blockiert bzw. nachgelagert, nicht angefasst.
- **AP2-02** (Indoor-Kartenrendering), **MV-10** (VehicleContext-GC): weiterhin Typ L bzw. geringe
  Priorität ohne sichtbaren Bedarf — nicht in diesen Sprint aufgenommen.

Ergebnis: ein kleinerer, fokussierter 4-Task-Sprint statt künstlichem Auffüllen (vom Nutzer im
Rahmen der Grill-Me-Antworten bestätigt).

**Keine Signaturänderung nach außen ohne Zweck** (CLAUDE.MD Abschnitt 15) — Ausnahme ist
`WEBRTC-10` (Entfernen des SDP-Hacks ist der Taskzweck: der Offer verhält sich danach wieder wie
der Browser-Standard, nicht wie vorher). Bei den Go-Tasks (`FLEET-02`/`AP2-03`) gilt zusätzlich
`docs/go-style-guide.md`.

Datum: 2026-07-18 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 31 ✅ (dieser Branch zweigt direkt von `feature/fleet-service-foundation` ab,
nicht von einem der alten Sprint-Worktrees — Sprint 27–31 sind zum Startzeitpunkt bereits gemergt)
Branch: `feature/fleet-service-foundation-sprint32`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| FLEET-02 / AP2-03 | Persistenzform "gefahrene Route" entscheiden + implementieren | M/L | ✅ |
| WEBRTC-10 | SDP-Workaround entfernen, lokal gegen echten mediamtx-Container verifizieren | M | ✅ |
| AP1-04 | Meilenstein-1-Abnahmedokument konsolidieren | S/M | ✅ |
| AP2-05 | Meilenstein-2-Abnahmedokument konsolidieren | S | ✅ |

## Ergebnisse

**FLEET-02 / AP2-03 — Persistenzform "gefahrene Route" ✅ ([ADR-033](../docs/adr/033-vehicle-position-history.md))**
Typ L bestätigt (neue Tabelle = Datenstruktur-Änderung), Grill-Me-Session vor Umsetzung
durchgeführt (siehe oben) — Nutzer wählte "einfache Postgres-Tabelle" gegenüber einer dedizierten
Zeitreihen-DB. Implementierung in `internal/fleetservice/store.go`:
- Neue `vehicle_position_history`-Tabelle (`id, vehicle_id, position_lat, position_lon,
  recorded_at`), `vehicle_id REFERENCES vehicles(id) ON DELETE CASCADE` (gleiche Begründung wie
  `task_status_history`/`ADR-032`: kein Produktions-Lösch-Endpoint für Fahrzeuge, aber
  History-Zeilen sollen ihr Fahrzeug nicht überleben).
- `RecordPositionHistory` (neue Methode) schreibt gedrosselt — ein bedingtes
  `INSERT ... WHERE NOT EXISTS` (eine atomare Anweisung), das einen neuen Punkt nur einfügt, wenn
  seit dem letzten aufgezeichneten Punkt für dieses Fahrzeug mindestens 10s vergangen sind
  (`positionHistoryMinInterval`). `vehicle-mock`s Fleet-Simulation publiziert alle 3s
  (`fleetSimulationTick`) — ungedrosselt wären das ~1.200 Zeilen/Stunde/Fahrzeug für eine
  Kartenlinie, die diese Auflösung nicht braucht. Positionslose Updates (`nil` lat/lon) erzeugen
  keine Zeile.
- Retention: `pruneVehiclePositionHistory` löscht Zeilen älter als 30 Tage
  (`positionHistoryRetention`) — einmal beim Start (`NewPostgresFleetStore`, gleiches Muster wie
  `backfillTaskStatusHistory`) und zusätzlich per 24h-Ticker während der Laufzeit
  (`startPositionHistoryRetentionLoop`, `cmd/fleet-service/main.go`), damit ein dauerhaft
  laufender Prozess nicht auf einen Neustart wartet.
- Neuer Endpoint `GET /fleet/vehicles/{id}/history` (`handler.go`, `cmd/fleet-service/main.go`),
  404 bei unbekanntem Fahrzeug, `[]` (nicht `null`) bei einem Fahrzeug ohne aufgezeichnete Punkte.
- Frontend: `useVehiclePositionHistory`-Hook (One-Shot-Fetch pro Fahrzeugauswahl, analog
  `useFleetZones.ts`), `FleetMap.tsx` zeichnet die Historie nur für das aktuell ausgewählte
  Fahrzeug als durchgezogene Polylinie (`react-leaflet`s `<Polyline>`, cyan, nur ab ≥2 Punkten).
- `go build`/`go vet ./...` reposweit sauber.
- `go test ./internal/fleetservice/... -race` (gegen echten Postgres-Container, zweimal
  hintereinander gegen frischen Zustand, CLAUDE.MD Abschnitt 17): 10 neue Fälle — Schreibpfad
  (Erfolgsfall, `nil`-Position no-op, Throttling innerhalb des Intervalls, Schreiben nach
  Ablauf des Intervalls via zurückdatiertem Testfixture statt echtem Sleep), Auslesen (leeres
  nicht-`null`-Array, 404 bei unbekanntem Fahrzeug, chronologische Reihenfolge), `ON DELETE
  CASCADE`-Regressionstest (analog zum in Sprint 31 gefundenen Bug), Retention (löscht nur
  Zeilen älter als das Retention-Fenster), sowie 2 Handler-Tests (404, End-to-End über den echten
  HTTP-Layer). Alle bestehenden Fleet-Service-Tests weiterhin grün (272/272 inkl. Frontend).
- **Gegen den echten Docker-Test-Stack verifiziert** (`docker compose -f
  tests/docker-compose.test.yml up --build -d`, danach `down`): `vehicle-mock`s
  Fleet-Simulation (`test-lastenzug-01`) lief automatisch mit, `GET
  /fleet/vehicles/test-lastenzug-01/history` zeigte nach ~25s fünf chronologisch aufsteigende,
  korrekt gedrosselte Punkte (~12s statt 3s Abstand); 404 für unbekanntes Fahrzeug bestätigt;
  `fleet-service`-Neustart bestätigt: Tabellenerstellung idempotent, alle 7 bis dahin
  aufgezeichneten Punkte überlebten den Neustart unverändert.
- Frontend: `npx tsc -b --noEmit` sauber, `npx vitest run` komplett grün (272/272, 10 neue Fälle:
  3 `FleetMap`-Tests für die Polyline — kein Rendering ohne Prop, kein Rendering bei <2 Punkten,
  Rendering ab 2 Punkten — sowie 7 `useVehiclePositionHistory`-Hook-Tests, Muster identisch zu
  `useFleetZones.test.ts`).
- **Bewusst nicht abgedeckt:** kein Live-Update der Historie während eine Session läuft (One-Shot-
  Fetch pro Fahrzeugauswahl, kein WS-Event für neue Historie-Punkte — analog zur bereits
  akzeptierten Grenze von `useFleetZones.ts`, dokumentiert im Hook-Kommentar); keine geplante
  Route (gestrichelte Linie aus `tasks`) — bleibt offen, siehe `ADR-033`.

**WEBRTC-10 — SDP-Workaround entfernt und lokal verifiziert ✅**
Technische Vorab-Prüfung (vor jeder Code-Änderung, wie im Sprint-Kickoff gefordert): `go.mod` von
`bluenviron/mediamtx` (GitHub, `main`-Branch und konkret Tag `v1.18.1`, was `mediamtx:latest`
tatsächlich ausliefert) zeigt `github.com/pion/webrtc/v4 v4.2.12` — eine komplett andere
Codebasis als das am 2026-06-18 referenzierte, längst abgelöste `v1.19.0`. Nutzerentscheidung auf
dieser Grundlage: umsetzen.
- `frontend/src/hooks/useWebRTC.ts` und `useWHIPSender.ts`: `actpass → active`-Regex-Ersetzung vor
  `setLocalDescription()` entfernt, Offer bleibt Standard-`actpass` (RFC 8842).
- `npx tsc -b --noEmit` sauber, `npx vitest run` komplett grün (272/272, keine Regression — keine
  bestehenden Tests hingen an der alten SDP-Mutation).
- **Lokal gegen den echten `mediamtx:latest`-Container verifiziert** (nicht Teil der Repo-Test-
  Suite — Browser-vs-Pion-DTLS-Spezifika unterscheiden sich von Pion-vs-Pion, daher ein
  Scratch-Verifikationsskript statt eines committeten Tests): ein isolierter WHIP-Publisher-Client
  in Go (`pion/webrtc v4.2.12`, exakt passend zur mediamtx-Version — eine ältere Client-Version,
  `v4.0.14` wie sonst im Repo gepinnt, hing im ersten Testlauf selbst und wäre ein Testartefakt
  statt eines echten mediamtx-Befunds gewesen) sendet einen unveränderten `actpass`-Offer an
  MediaMTX' WHIP-Endpoint. Ergebnis: MediaMTX beantwortet mit `a=setup:active` (RFC-8842-
  Empfehlung für den Answerer — exakt die Rolle, die 2026-06-18 als fehlerhaft dokumentiert war),
  der DTLS/ICE-Handshake erreicht zuverlässig `PeerConnectionStateConnected`. 6 von 6 Läufen
  erfolgreich bei kontrollierter, auf die Loopback-Schnittstelle beschränkter ICE-Kandidatenwahl
  (die Verifikationsumgebung hat ungewöhnlich viele Netzwerk-Interfaces — echter Host, IPv6,
  libvirt-Bridge, drei Docker-Bridges —, was in ungefilterten Läufen zu ICE-Kandidatenpaar-
  Flakiness führte, unabhängig von der eigentlichen DTLS-Frage; mit sauberer Kandidatenwahl war
  das Ergebnis reproduzierbar stabil). Kein eigenes ADR nötig (reiner Implementierungs-Hack, keine
  Architekturentscheidung nach ADR-014/020 berührt) — Doku-Update stattdessen in
  `docs/webrtc.md`, `CONTEXT.MD`, `DECISIONS.MD`.
- **Bewusst nicht abgedeckt:** Produktiv-Video-Empfang (WHEP) auf AWS mit echtem Chromium — bleibt
  offen (neue Zeile in `CONTEXT.MD` "Offene Fragen"), da dieser Worktree keinen AWS-Zugriff hat
  und ein Pion-Testclient die Chromium-DTLS-Implementierung nicht exakt nachbildet (andere
  Codebasis). Kein lokaler Browser-E2E-Test (getUserMedia braucht Kamera-Hardware bzw.
  `--use-fake-device-for-media-stream`, zusätzlich ist der lokale Dev-Stack laut `DECISIONS.MD`
  aktuell an einem separaten, unabhängigen SSL-Problem blockiert) — durch das gezielte
  Pion-WHIP-Verifikationsskript ersetzt, das die kritische, tatsächlich strittige Frage (mediamtx-
  seitiges DTLS-Verhalten als "active" Answerer) direkt beantwortet, ohne diese Umgebungslücken zu
  brauchen.

**AP1-04 — Meilenstein-1-Abnahmedokument ✅**
Explizite Nutzerentscheidung (kein Code-Task, CLAUDE.MD Abschnitt 4): eigenständiges Dokument
gewünscht für beide Meilensteine. Neu:
[docs/milestones/meilenstein-1-architektur.md](../docs/milestones/meilenstein-1-architektur.md) —
konsolidiert `vision.md`/`requirements.md`/`architecture.md`/`CONTEXT.MD`/`ADR-027/028/029` zu
einem auftraggebertauglichen Dokument. Weist explizit darauf hin, dass Meilenstein 1 zum
aktuellen Stand **nicht vollständig abnahmefähig** ist — Teil 1 der Leistungsbeschreibung
("Einarbeitung in vorhandene Programmierschnittstellen") hängt am noch ausstehenden
Workshop-Termin mit der Professur Logistik (`AP1-01`, externe Abhängigkeit).

**AP2-05 — Meilenstein-2-Abnahmedokument ✅**
Analog zu `AP1-04`. Neu:
[docs/milestones/meilenstein-2-dashboard.md](../docs/milestones/meilenstein-2-dashboard.md) —
Statusübersicht aller vier AP2-Anforderungsbereiche (6 von 7 Einzelpunkten fertig, siehe
Tabelle im Dokument), inkl. Kurzbeschreibung der Sprint-31/32-Ergänzungen (Task-Status-Historie,
Routenübersicht). Weist `AP2-02` (Indoor-Kartenrendering) als einzigen noch offenen Punkt aus.

**Nebenbei erledigt (keine eigenen Tasks, beim Ohnehin-Bearbeiten der Dateien):** veraltete
`OBS-01`/"Vollständige Task-Status-Audit-Historie"-Zeilen in `tasks/backlog.md` ("Offene
Entscheidungen"-Tabelle + Phasen-Übersicht), `CONTEXT.MD` ("Offene Fragen") und
`docs/adr/README.md` ("Offene Folge-Entscheidungen") auf ✅ nachgezogen — beide waren durch
Sprint 31 bereits erledigt, aber in diesen separaten Archiv-Tabellen nicht aktualisiert worden.
`docs/adr/README.md`s ADR-Index war zudem bei ADR-031 stehengeblieben (fehlte `ADR-032` komplett)
— `ADR-032` und `ADR-033` ergänzt, Zähler auf 31 korrigiert.
