# Sprint 23 — Outdoor Karten-/Zonen-Visualisierung im Fleet Overview

Ziel: Eine SVG-Karte mit Live-Fahrzeugpositionen im Fleet-Overview-Dashboard, aufbauend auf
ADR-029s Datenmodell. Backend (`GET/POST /fleet/zones`, `GET/POST /fleet/stations`) existiert seit
Sprint 21 (FLEET-05), war im Frontend bisher komplett ungenutzt (Sprint 22 hat bewusst keine
Karte gebaut). Grill-Me (2026-07-16) hat den Scope auf **Outdoor-Zonen** begrenzt — Fahrzeuge
haben für Indoor-Positionierung keine `position_x/y` (nur `position_zone_id`), eine echte
Datenmodell-Lücke, die eine Backend-Erweiterung bräuchte und auf einen Folge-Sprint verschoben
wird. Format von `svg_geometry`/`geo_bounds` (bisher in ADR-029 nur als "wird als Beispiel neu
erstellt" umschrieben) wurde in diesem Sprint verbindlich definiert und dokumentiert (ADR-029-
Update, MAP-02). Bewusst frontend-lastiger Sprint — kein neuer Go-Code außer für das
Demo-Seed-Skript, das die bereits bestehenden REST-CRUD-Endpoints nutzt.

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 22 ✅
Branch: `feature/fleet-service-foundation` (unverändert fortgeführt)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MAP-01 | Demo-Seed-Skript (`scripts/seed-fleet-demo.sh`) — 1 Outdoor-Zone + 5 Stationen über bestehende REST-API, Koordinaten passend zu `vehicle-mock`s hartcodiertem Demo-Pfad | M | ✅ |
| MAP-02 | ADR-029 datiertes Update (`svg_geometry`/`geo_bounds`-Format) + `DECISIONS.MD`/Sprint-23-Skelett | S | ✅ |
| MAP-03 | `api-client.ts`: `Zone`/`Station`-Interfaces + `listFleetZones`/`listFleetStations` | S | ✅ |
| MAP-04 | `leaflet`+`react-leaflet`-Dependencies, `fleet-map.ts` (reine Helper-Funktionen) | M | ✅ |
| MAP-05 | `FleetMap.tsx` — SVG-Zonen-Overlay, Stations-Marker, Live-Fahrzeug-Marker, Empty-State | L | ✅ |
| MAP-06 | `useFleetZones.ts` — einmaliger REST-Fetch (kein WS, dokumentiert) | S | ✅ |
| MAP-07 | `FleetOverview.tsx`-Integration (Karten-Zeile über unverändertem 3-Spalten-Grid) | M | ✅ |
| MAP-08 | Tests reine Funktionen (`fleet-map.test.ts`) | M | ✅ |
| MAP-09 | Tests `FleetMap.tsx` (jsdom/Leaflet-Spike + Komponententests) | M | ✅ |
| MAP-10 | Tests `useFleetZones.ts` | S | ✅ |
| MAP-11 | Verifikation gegen echten Dev-Stack + Browser-MCP, Ergebnis dokumentiert | S | ✅ |

## Ergebnisse

**MAP-01 — Demo-Seed-Skript ✅**
`scripts/seed-fleet-demo.sh` (neu, bash+curl+python3 für JSON-Encoding, kein `jq` im Repo): legt
Zone `Betriebshof Nord` (`zone-betriebshof-nord`, outdoor, `geo_bounds` umschließt
`vehicle-mock`s hartcodierten Demo-Pfad `52.130100,11.640100`–`52.130500,11.641200` mit Marge) +
5 Stationen an (zwei davon exakt deckungsgleich mit `vehicle-mock`s `demo-station-a/b`, drei
weitere plausible Punkte außerhalb des simulierten Pfads — dokumentierte, akzeptierte Lücke).
Idempotent (prüft `GET /fleet/zones` auf existierende Zonen-ID vor dem Anlegen, Exit 0 statt
Fehler bei Wiederholung — verifiziert: zweiter Lauf gegen den echten Dev-Stack meldet korrekt
"already exists", legt nichts doppelt an).

Login via `POST /auth/operator/login` (Admin-Credentials aus `ADMIN_PASSWORD`-Env-Var, Default
`admin_dev_secret`), `BASE_URL` überschreibbar (Default `http://localhost:3000`, durch nginx).
`geo_bounds` wird als JSON-**String** (nicht verschachteltes Objekt) gesendet — `Zone.GeoBounds`
ist Go `*string`, ein verschachteltes JSON-Objekt im Request-Body hätte `json.Decode` zum
Scheitern gebracht (siehe ADR-029-Update, MAP-02). Nach dem Seed wird `GET /fleet/zones` erneut
abgefragt und `geo_bounds` per `json.loads` re-geparst, um das Round-Tripping real zu bestätigen,
nicht nur anzunehmen.

**Echten Bug beim ersten Lauf gefunden:** `GET /fleet/zones` lieferte bei leerer Tabelle JSON
`null` statt `[]` (Go: `var zones []Zone` bleibt `nil` bei 0 Zeilen, `json.Marshal` rendert das
als `null`) — das Idempotenz-Check-Skript crashte beim ersten Durchlauf
(`TypeError: 'NoneType' object is not iterable`), da eine leere Zonen-Liste vorher nie real
abgefragt worden war. Fix im Skript (`zones = json.load(sys.stdin) or []`) und zusätzlich
defensiv in `api-client.ts`s `listFleetZones`/`listFleetStations` (`?? []`), damit `FleetMap.tsx`
nie gegen `null.length` crasht. Kein Backend-Fix in diesem Sprint (siehe `DECISIONS.MD`, Zeile zu
`GET /fleet/zones`/`/fleet/stations` — betrifft potenziell auch `ListVehiclesWithStatus`/
`ListAlerts`, dort aber bisher folgenlos, da nie leer abgefragt).

Gegen den echten laufenden Dev-Stack verifiziert (nicht nur angenommen): Skript zweimal
hintereinander ausgeführt (erster Lauf legt an, zweiter Lauf idempotent kein-op), `GET
/fleet/stations` per `curl` zeigt alle 5 Stationen mit korrekten Koordinaten inkl. deutscher
Umlaute (`Verwaltungsgebäude`, korrekt UTF-8/JSON-escaped).

**MAP-02 — ADR-029-Update + Doku ✅**
`docs/adr/029-fleet-vehicle-data-model.md`: datierter Update-Block nach dem
"Kartendarstellung"-Abschnitt angehängt (kein Überschreiben, CLAUDE.MD Abschnitt 6) —
Format-Definition für `svg_geometry`/`geo_bounds`, der Doppel-JSON-Encoding-Gotcha, der
`null`-statt-`[]`-Fund, und die Outdoor-only-Scope-Entscheidung. `DECISIONS.MD`: ADR-029-Zeile um
Update-Hinweis ergänzt, zwei neue Zeilen in "Offene Folge-Entscheidungen" (Indoor-Fahrzeugposition-
Lücke, `null`-statt-`[]`-Fund).

---
**MAP-04 — Dependencies + fleet-map.ts ✅**
`leaflet@^1.9.4`, `react-leaflet@^4.2.1` (React-18-kompatibel, v5 bräuchte React 19),
`@types/leaflet` (Dev) installiert. `frontend/src/lib/fleet-map.ts` (neu): `parseGeoBounds`
(nie werfend, `null` bei fehlendem/kaputtem JSON/falscher Shape/nicht-endlichen Zahlen),
`parseSvgGeometry` (via `DOMParser`, auch unter jsdom nutzbar), `autonomyMarkerColor`,
`vehiclesWithPosition`. `AUTONOMY_DOT` aus `FleetVehicleList.tsx` hierher verschoben (reiner
Import-Change, bestehende Tests unverändert grün) — eine Farbquelle für Listen-Punkte und
Karten-Marker statt Duplikation.

**Echtes Environment-Problem während MAP-04 gefunden:** `node_modules/@types` (und, wie sich beim
Reparaturversuch herausstellte, große Teile von `node_modules` insgesamt) gehörten `root`, nicht
dem Nutzer — `npm install -D @types/leaflet` scheiterte mit `EACCES`. Ein einfacher `rm -rf
node_modules` lief nur teilweise durch (root-eigene Dateien blieben stehen, inkonsistenter
Zwischenzustand). Nutzer hat `sudo rm -rf node_modules && npm install` selbst im Terminal
ausgeführt (`sudo` erforderte eine interaktive Passworteingabe, die aus dieser Session heraus nicht
möglich war) — danach sauber, `@types/leaflet` ließ sich installieren.

**MAP-05 — FleetMap.tsx ✅**
Neue Komponente `frontend/src/components/FleetMap.tsx`. Kein `<TileLayer>` (ADR-029: keine
externen Kartenkacheln) — nur `MapContainer` mit Bounds aus allen Zonen mit gültigem
`geo_bounds`. `svg_geometry` wird nicht über react-leaflets JSX-`SVGOverlay` gerendert (der
erwartet React-Children, nicht rohes Markup), sondern über Leaflets Core-API `L.svgOverlay`
(seit 1.0, kein Plugin nötig), imperativ via `useMap()`+`useEffect` mit explizitem Cleanup
(`map.removeLayer`) — unter `StrictMode` (aktiv in `main.tsx`) sonst doppelte Layer im
Dev-Modus. Eine Zone, die `parseGeoBounds`/`parseSvgGeometry` nicht besteht, wird übersprungen
(`console.warn`, kein Crash) statt die ganze Karte zu blockieren. Stations-Marker als
`L.divIcon`-Quadrate, Fahrzeug-Marker als `L.divIcon`-Kreise (Farbe über `autonomyMarkerColor`,
ausgewähltes Fahrzeug mit `ring-2 ring-white`), Klick ruft dieselbe `onSelectVehicle`-Callback wie
`FleetVehicleList`, sodass Auswahl über Karte oder Liste denselben `FleetVehicleDetail`-State
treibt.

**MAP-06 — useFleetZones.ts ✅**
Einmaliger REST-Fetch (`Promise.all([listFleetZones, listFleetStations])`), kein WS — `FLEET-06`s
Broadcast-Hub kennt keine Zonen-/Stations-Events. Bewusst als Entscheidung dokumentiert (Kommentar
im Code), nicht als übersehene Lücke.

**MAP-07 — FleetOverview.tsx-Integration ✅**
Bestehendes `grid grid-cols-3 gap-4` unverändert gelassen (minimiert Risiko für die 45
bestehenden DASH-07/08-Tests), neue `FleetMap` in einer Zeile darüber
(`h-[45vh] min-h-80 shrink-0`). Alle 167 bestehenden Frontend-Tests bleiben grün nach der
Änderung — verifiziert vor dem Weiterbauen, nicht erst am Sprintende.

**MAP-08/09/10 — Tests ✅**
33 neue Tests: `fleet-map.test.ts` (19, reine Funktionen — Grenzwerte für `parseGeoBounds`
inkl. eines echten Edge Cases mit `1e400`, das als gültiges JSON-Zahlenliteral zu `Infinity`
overflowt; `parseSvgGeometry`; `autonomyMarkerColor`; `vehiclesWithPosition` inkl.
`vehicle-001`-Fall), `FleetMap.test.tsx` (8, Komponententests inkl. Klick-Interaktion),
`useFleetZones.test.ts` (6, REST-Erfolg/-Fehler/leere Antwort/Unmount-Guard). Alle zweimal
hintereinander gelaufen, keine Flakiness. `tsc -b`/`eslint`/`vite build` sauber, 200 Tests
insgesamt grün.

**MAP-09 — jsdom/Leaflet-Spike-Ergebnis: voller Erfolg, kein Fallback nötig.** Vor dem
eigentlichen Testfile wurde geprüft, ob `MapContainer` unter dem bestehenden jsdom-Setup
überhaupt mountet (kein `ResizeObserver`-Polyfill vorhanden) — es mountete sauber, ohne
Konsolenwarnungen, **kein** Polyfill in `test/setup.ts` nötig. Anders als im Plan als Risiko
vorgesehen, ließ sich sogar die Klick-Interaktion auf einem Fahrzeug-Marker
(`fireEvent.click(screen.getByTitle(...))` → `onSelectVehicle`) direkt unter jsdom testen — der
im Plan vorgesehene Fallback ("nur renders-without-throwing, Rest auf MAP-11 verschieben") war
nicht nötig.

**MAP-11 — Verifikation: vollständig abgeschlossen ✅**

**Update 2026-07-16 (Fortsetzung) — interaktiver Browser-Test durchgeführt, alle 5 Prüfpunkte
bestanden.** `chrome-devtools-mcp` war in dieser Session (nach dem in der vorherigen Session
bereits eingetragenen `--isolated`-Flag, siehe unten) sofort ohne Blockade nutzbar — ein
`ToolSearch` nach den Tool-Namen lieferte direkt funktionsfähige Schemas, `new_page` gegen
`http://localhost:3000` verband ohne "browser is already running"-Fehler, obwohl zeitgleich
mehrere andere `chrome-devtools-mcp`/`firefox-devtools-mcp`-Prozesse aus parallelen Sessions auf
derselben Maschine liefen (`ps aux` zeigte drei separate Prozessgruppen, je mit eigenem isolierten
Profil). Das bestätigt: der MCP-Reconnect, der in der letzten Session noch ausstand, hat
stattgefunden, `--isolated` löst das Mehrfach-Instanzen-Problem wie erwartet.

**Vorbedingung Dev-Stack:** Beim Sessionstart liefen nur die beiden `vehicle-mock`-Container;
Frontend/Backend-Services waren seit 4 Minuten `Exited (0)`/`Exited (2)` (vermutlich durch einen
`docker compose down` oder Neustart zwischen den Sessions). Mit `docker compose ... up -d` (ohne
`--build`, um den in der letzten Session bereits gefixten Layer-Cache-Zustand nicht erneut zu
riskieren) neu gestartet — alle Container liefen danach `Up`/`healthy`. Frontend-Bundle erneut
stichprobenartig gegen den bekannten Leaflet-Bug geprüft: `index-CA_ZWEK1.js` enthält `leaflet`,
388 KB (konsistent mit dem in der letzten Session verifizierten ~396-KB-Bundle, keine Regression
auf den 244-KB-Bug). `scripts/seed-fleet-demo.sh` erneut ausgeführt — Zone `zone-betriebshof-nord`
existierte bereits (idempotent bestätigt, drittes Mal insgesamt).

Konkret geprüft (Login: `admin`/`admin_dev_secret`, `http://localhost:3000`):

1. **Login → Fleet Overview ohne Konsolenfehler:** erfolgreich, `list_console_messages` zeigte
   ausschließlich zwei vorbestehende Accessibility-Hinweise vom Login-Formular (`No label
   associated with a form field`, `A form field element should have an id or name attribute` —
   beide bereits vor MAP-01 im Login-Formular vorhanden, außerhalb des Sprint-23-Scopes, nicht neu
   eingeführt), keine `error`/`warn`-Einträge.
2. **Karte zeigt Zonen-Umriss + 5 Stations-Marker:** Screenshot bestätigt gestrichelten
   Zonen-Rahmen (`Betriebshof Nord`) mit drei benannten Gebäude-Rechtecken ("Halle 1", "Halle 2",
   "Ladezone") — das sind `<text>`-Elemente aus dem Zonen-`svg_geometry` selbst (Hallenplan-Grafik
   aus dem Seed-Skript, Zeilen 57–61), **nicht** die Stations-Marker. Die eigentlichen
   Stations-Marker (kleine `10×10px`-Quadrate, `stationIcon()` in `FleetMap.tsx:106`) sind separat
   sichtbar; über die Accessibility-Snapshot-Buttonzahl (7 = 5 Stationen + 2 Fahrzeuge) verifiziert,
   dass alle 5 Stationen aus dem Seed (`Ladezone A/B`, `Wartungsbereich`, `Verwaltungsgebäude`,
   `Einfahrtstor`) tatsächlich gerendert werden, nicht nur die drei mit sichtbarem Gebäude-Label.
   Auf den ersten Blick sah die Diskrepanz zwischen den drei großen beschrifteten Rechtecken und
   den kleinen unbeschrifteten Quadraten wie ein potenzieller Leaflet-CSS-Bug aus (Verdacht laut
   Aufgabenstellung) — durch Code-Lesen (`FleetMap.tsx`, `scripts/seed-fleet-demo.sh`) als
   beabsichtigtes Verhalten bestätigt, kein Bug.
3. **Simulierte Fahrzeuge als farbige Kreis-Marker, sichtbare Bewegung:** zwei Screenshots im
   Abstand mehrerer Sekunden zeigen messbar unterschiedliche Marker-Positionen für
   `lastenrad-01`/`lastenzug-01` sowie sinkende Batteriewerte (85→82 %, 87→86 % zwischen den ersten
   beiden Aufnahmen). Live-Update-Mechanismus zusätzlich explizit verifiziert (nicht nur vermutet):
   ein per `evaluate_script`-`initScript` injizierter `WebSocket`-Proxy protokollierte nach einem
   Seiten-Reload `WS_OPEN_ATTEMPT`/`WS_OPEN_SUCCESS` für
   `ws://localhost:3000/fleet/ws?token=...` — bestätigt, dass die Bewegung über den echten
   FLEET-06-WS-Broadcast läuft, nicht über Polling. (Nebenbefund: `list_network_requests` des
   `chrome-devtools-mcp`-Tools zeigt WS-Handshakes generell nicht in seiner Liste an, auch nicht
   mit `resourceTypes: ["websocket"]` — eine Werkzeug-Einschränkung, kein App-Verhalten; die
   REST-Aufrufe `GET /fleet/vehicles|zones|stations|alerts` erscheinen dort korrekt mit Status 200.)
4. **Klick auf Fahrzeug-Marker aktualisiert Detail-Panel:** zweimal verifiziert (vor und nach
   Reload). Klick auf den `lastenrad-01`-Kreis-Marker öffnet dasselbe `FleetVehicleDetail`-Panel
   wie ein Klick in der Liste (Batterie/Autonomie-Modus/Teleoperate-Button), und die
   Fahrzeugliste hebt den gleichen Eintrag lila hervor — bestätigt den gemeinsamen
   `onSelectVehicle`-State aus MAP-05. Ein Klick auf einen Stations-Marker (kein
   `onSelectVehicle`-Handler in `FleetMap.tsx`) ändert das Detail-Panel erwartungsgemäß nicht.
5. **Konsole fehlerfrei:** über die gesamte Interaktion (Login, Kartenrendering, zwei
   Marker-Klicks, ein Reload mit WS-Proxy-Injection) traten zu keinem Zeitpunkt `WS_ERROR`/
   `WS_CLOSED` oder sonstige Konsolenfehler auf — nur die zwei vorbestehenden, unveränderten
   Login-Formular-Hinweise aus Punkt 1.

Kein neuer Bug gefunden. Der in der letzten Session dokumentierte Docker-Layer-Cache-Bug (MAP-11,
vorheriger Absatz) ist nicht erneut aufgetreten (Bundle weiterhin korrekt mit Leaflet). Damit ist
Sprint 23 vollständig abgeschlossen: 200 Frontend-Tests (davon 33 neu, MAP-08/09/10) plus der
jetzt nachgeholte interaktive Browser-Test decken sowohl die reine Funktions-/Komponentenebene als
auch das tatsächliche Rendering/Live-Verhalten im echten Browser ab.

**DoD-Nachtrag (CLAUDE.MD Abschnitt 11) — `docs/architecture.md` nachgezogen:** Bei der
Abschlussprüfung fiel auf, dass `docs/architecture.md` `fleet-service` seit dessen Einführung in
Sprint 21 an keiner Stelle dokumentierte (weder REST-API noch WS-Broadcast noch, jetzt in Sprint
23, `FleetMap.tsx`) — eine seit zwei Sprints bestehende, nicht durch Sprint 23 verursachte Lücke.
Auf Nutzerrückfrage ("ist der Sprint vollends abgeschlossen") nachgezogen statt offen gelassen:
neuer Abschnitt "Fleet System (ADR-027/028/029, Sprint 21–23)" (Datenmodell, FleetGateway-
Abstraktion, REST-Endpoint-Tabelle, WS-Broadcast, Frontend-Komponenten/Hooks-Tabelle), Container-
Architecture-Tabelle um `fleet-service`/`vehicle-mock` ergänzt, Projekt-Verzeichnisstruktur um
`cmd/fleet-service`, `cmd/vehicle-mock`, `internal/fleetservice`, `internal/fleetgateway` sowie die
neuen Frontend-Komponenten/Hooks/Lib-Dateien ergänzt.

---

**MAP-11 — Frühere Sessions: Backend-/Build-Ebene verifiziert, Browser-Test noch offen (historisch,
durch obigen Absatz abgelöst).**

Backend-/Build-Ebene vollständig verifiziert:
- `seed-fleet-demo.sh` erneut gegen den Dev-Stack gelaufen (idempotent, Zone bereits vorhanden).
- Frontend-Container neu gebaut und deployed. Dabei ein **echtes, umgebungsbedingtes Problem
  gefunden**: ein zwischenzeitlicher Rebuild des `frontend`-Images (durch einen parallelen
  Prozess außerhalb dieser Session, vermutlich `make up --build`) lieferte ein Bundle **ohne**
  `leaflet` (244 KB statt der erwarteten ~396 KB) — obwohl der Quellcode zu diesem Zeitpunkt
  bereits alle MAP-04/05-Änderungen enthielt. Ursache nicht abschließend geklärt (vermutlich eine
  inkonsistente Docker-Layer-Cache-Wiederverwendung), aber real reproduzierbar beobachtet
  (`docker history` zeigte Layer aus unterschiedlichen Build-Zeitpunkten gemischt). Fix: Rebuild
  mit `--no-cache` (`docker build --no-cache -f infrastructure/docker/frontend.Dockerfile ...`)
  erzeugte wieder das korrekte ~396-KB-Bundle; verifiziert sowohl direkt im Image
  (`docker run ... grep -c leaflet`) als auch nach Redeploy über `curl` gegen
  `http://localhost:3000`. **Für zukünftige Deployments vermerkt:** bei unerklärlich kleinen/
  fehlenden Bundles zuerst `--no-cache` probieren, nicht dem Cache vertrauen.
- `docker compose ... build` (ohne `--pull=false`) schlug wegen fehlendem Netzwerkzugriff auf
  `registry-1.docker.io` (DNS-Timeout) fehl — Umgebungseinschränkung dieser Sandbox, umgangen über
  direktes `docker build --pull=false` gegen bereits lokal vorhandene Base-Images.

**Ehemals "bewusst nicht abgeschlossen" (historisch):** der interaktive Browser-Test war zum
Zeitpunkt dieses Absatzes noch offen, weil `chrome-devtools`-/`firefox-devtools`-MCP durch
parallele Sessions blockiert waren und der zum Fix eingetragene `--isolated`-Flag noch einen
MCP-Reconnect brauchte. Dieser Reconnect hat inzwischen stattgefunden — siehe den Absatz "Update
2026-07-16 (Fortsetzung)" oben: der Browser-Test wurde nachgeholt und ist vollständig
abgeschlossen, alle 5 Prüfpunkte bestanden, kein neuer Bug.
