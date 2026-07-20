# Sprint 22 — Fleet Overview Dashboard

Ziel: Erste Slice des Web-Dashboards (AP2) — eine Fleet-Overview-Seite (Fahrzeugliste,
Status-/Detail-Panel, Alerts) gegen das in Sprint 21 fertiggestellte `fleet-service`-Backend.
Bewusst kein Karten-/Zonen-Rendering, kein Task-Management-UI, kein Dark-Mode-Umschalter, keine
System-Steuerung/-Override — reine Fahrzeugübersicht als erste Slice, analog zu Sprint 21s
Aufteilung in kleine, einzeln abnehmbare Tasks. Grill-Me (2026-07-15) hat zwei echte
Architekturentscheidungen ergeben, beide bereits als datierte ADR-Ergänzungen dokumentiert statt
nur implizit umgesetzt:

- Live-Updates nutzen den echten WS-Broadcast (`GET /fleet/ws`, FLEET-06) statt Polling —
  konsistent mit `ADR-028`s eigenem Ziel (Live-Updates ohne Polling für Multi-Workstation-Betrieb).
- Der "Teleoperate"-Button darf zusätzlich zum Notfall-Trigger auch **proaktiv** auf jedem
  Fahrzeug ohne aktiven Operator genutzt werden — dokumentiert als `ADR-028`-Update
  (2026-07-15) statt stillschweigend über die ursprüngliche Notfall-only-Regel hinaus gebaut.

Datum: 2026-07-15 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 21 ✅
Branch: `feature/fleet-service-foundation` (unverändert fortgeführt)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| DASH-01 | nginx-Routing für `/fleet/` REST + `/fleet/ws` (dev + prod), Docker-DNS-`resolver`-Muster, **kein** Prefix-Stripping (anders als `/api/`) | S | ✅ |
| DASH-02 | `docker-compose.yml`: `frontend` `depends_on: fleet-service` | S | ✅ |
| DASH-03 | `api-client.ts`: `listFleetVehicles`/`listFleetAlerts`/`acknowledgeFleetAlert` + `FleetVehicle`/`FleetAlert`-Interfaces | S | ✅ |
| DASH-04 | `fleet-ws-client.ts` + `fleet-ws-events.ts` — JSON-WS-Client mit typisiertem `FleetWSEvent`, Backoff-Reconnect | M | ✅ |
| DASH-05 | `fleet-merge.ts` (reine Merge-Funktionen) + `useFleetOverview.ts` (REST-Snapshot + WS-Deltas) | L | ✅ |
| DASH-06 | `useActiveSessions.ts` (Extraktion aus `App.tsx`) + `App.tsx` View-Switching (`FleetOverview` vs. Cockpit, gated auf `sessionId`) | M | ✅ |
| DASH-07 | `FleetOverview.tsx` + `FleetVehicleList.tsx` + `FleetVehicleDetail.tsx` + `FleetAlertsPanel.tsx` (ADR-028-Gating: Teleoperate nur ohne aktiven Operator, sonst Beobachten) | L | ✅ |
| DASH-08 | Tests: reine Merge-/Parsing-Funktionen, Hook-Tests, Komponenten-Tests, `App.test.tsx` (neu) — 45 neue Tests; manuelle Verifikation gegen laufenden Dev-Stack | M | ✅ |

## Ergebnisse

**DASH-01/02 — Infrastruktur ✅**
`infrastructure/docker/nginx.dev.conf` und `nginx.conf` (Prod, für Parität) bekamen je zwei neue
`location`-Blöcke — `/fleet/` (REST, `proxy_pass` ohne Rewrite, da `fleet-service`s Mux die Pfade
bereits wörtlich als `/fleet/...` registriert, anders als `/api/` → `control-server`, das den
Prefix strippt) und `/fleet/ws` (WS-Upgrade, gleiches Muster wie `/ws`/`/vehicle/ws`). Ein
Rewrite hier hätte jede Anfrage 404en lassen — der mit Abstand größte Stolperstein dieser
Aufgabe, gegen den echten Stack verifiziert (siehe DASH-08-Ergebnis unten), nicht nur
angenommen. `docker-compose.yml`: `frontend` bekam `fleet-service` als zusätzliches
`depends_on`.

**DASH-03 — REST-Client ✅**
`api-client.ts` um `FleetVehicle`/`FleetAlert`-Interfaces und drei Funktionen erweitert, statt
eine neue Datei zu eröffnen — die bestehende Datei mischt bereits mehrere Backend-Services in
einer Datei (Auth/Control-Server/User-Management als eigene Abschnitte), drei Fleet-Funktionen
rechtfertigen noch keinen Split. Feld-Optionalität 1:1 aus `internal/fleetservice/store.go`
übernommen — `FleetVehicle.autonomy_mode` ist optional (Go: `*string, omitempty`), während das
strukturell ähnliche `VehicleStatus.autonomy_mode` aus dem WS-Kanal **nicht** optional ist. Diese
Asymmetrie ist real und wurde bewusst nicht "vereinheitlicht".

**DASH-04 — Fleet-WS-Client ✅**
`fleet-ws-events.ts` (reine Parsing-Funktion `parseFleetWSMessage`, kein `WebSocket`-Bezug — wirft
nie, kaputtes JSON/fehlendes `type`/`data` kollabiert zu einem expliziten `'unknown'`-Tag statt
den Union-Typ auf `string` aufzuweiten) plus `fleet-ws-client.ts` (`FleetWSClient`, an
`ws-client.ts`s Form angelehnt, aber JSON statt Protobuf). Reconnect-Backoff lebt bewusst
**innerhalb** des Clients (anders als bei `ws-client.ts`, wo `useSession.ts` das wegen der sich
ändernden `sessionId` extern übernimmt) — der Fleet-WS hat nur einen stabilen Verbindungsparameter
(Token), daher ist selbstständiges Reconnect hier einfacher. Bestehende `FE_WS_CONNECTED`/
`FE_WS_RECONNECT`-Logkonstanten wiederverwendet, keine neuen nötig.

**DASH-05 — Merge-Logik + Hook ✅**
`fleet-merge.ts`: `mergeVehicleStatus` überschreibt nur im Event tatsächlich vorhandene Felder
(**nicht-destruktives Overlay**, Grill-Me-Entscheidung) — ein `vehicle_status`-Event, das z. B.
kurzzeitig kein GPS liefert (Go `omitempty`), blendet den zuletzt bekannten Wert nicht auf `—`
aus. Kein Treffer für die `vehicle_id` lässt die Liste unverändert (dokumentierte Lücke, keine
erfundene Teilzeile). `useFleetOverview.ts`: REST-Snapshot zuerst, dann WS-Deltas; **Resync bei
jedem Reconnect nach dem ersten** (`Hub.Broadcast` im Backend hat kein Backlog/Replay — ohne
Resync bliebe das Dashboard nach einem kurzen WS-Aussetzer dauerhaft veraltet); `error` wird
explizit gesetzt statt wie `useVehicles.ts` still den alten Stand zu behalten (Fleet Overview ist
Primärinhalt, keine Randspalte); `acknowledgeAlert` aktualisiert optimistisch lokal und wirft bei
REST-Fehlern weiter (sichtbares Feedback für die bewusste Nutzeraktion).

**DASH-06 — Extraktion + View-Switching ✅**
`useActiveSessions.ts` aus `App.tsx`s bisher inline liegendem `GET /api/sessions`-Polling
extrahiert (reiner Refactor, kein Verhaltensunterschied für `AppContent` — der
Session-Restore-nach-Reload-Effekt nutzt jetzt `hasPolled` als State statt als Ref, was ihn sogar
korrekter macht: er reagiert jetzt auch wirklich auf den Zeitpunkt, an dem `hasPolled` kippt,
statt sich indirekt auf einen gleichzeitigen `activeSessions`-Update-Tick zu verlassen). `App.tsx`
route jetzt dreistufig ohne Router (kein Router nötig für 2 Post-Login-Views): kein Token →
`LoginPanel`; Token ohne `sessionId` → `FleetOverview` (neue Landing-View); `sessionId` gesetzt →
unverändertes Cockpit (`AppContent`).

**DASH-07 — Komponenten ✅**
`FleetOverview.tsx` (Container) verknüpft `fleet-service`-Daten mit `control-server`s
`activeSessions` clientseitig (ADR-029: "Frontend führt Services clientseitig zusammen") — hier,
nicht im Backend, wird bestimmt, ob ein Fahrzeug einen aktiven Operator hat. `FleetVehicleList.tsx`
(Autonomie-Status-Punkt, gestrichelt/grau für Fahrzeuge ohne jegliche Statusdaten — deckt
`vehicle-001` ab, das nie eine `vehicle_type`/Status-Meldung bekommen hat), `FleetVehicleDetail.tsx`
(ADR-028-Gating: aktiver Operator → **kein** Teleoperate-Button, stattdessen Badge + Beobachten-
Button, der denselben `session.startSession`-Fluss nutzt wie `ConnectionPanel`s bestehendes
`onJoinSession`; kein aktiver Operator → Teleoperate-Button, außer bei OBSERVER-Rolle), `FleetAlertsPanel.tsx`
(Severity-farbig, Bestätigen-Status pro Zeile statt global — ein hängender Acknowledge-Request
für Alert A darf den Button für Alert B nicht sperren).

**DASH-08 — Tests + Verifikation ✅**
45 neue Tests (reine Funktionen `fleet-merge.test.ts`/`fleet-ws-events.test.ts` — Grenzwerte wie
kaputtes JSON, fehlendes `type`-Feld, kein Fahrzeug-Treffer beim Merge, Idempotenz bei
Alert-Upsert; Hook-Tests `useActiveSessions.test.ts`/`useFleetOverview.test.ts` mit gemocktem
`FleetWSClient` — Reconnect-Resync, REST-Fehlerpfad, optimistisches Acknowledge inkl.
Fehlerfall; Komponententests inkl. des `vehicle-001`-Falls (alle Felder `undefined`, darf nicht
crashen/`NaN` anzeigen) und der wichtigsten Einzel-Assertion dieser Aufgabe — Teleoperate-Button
ist **abwesend**, nicht nur disabled, sobald ein aktiver Operator existiert; `App.test.tsx`, neu,
als Regressionswächter für die DASH-06-Routing-Entscheidung). Alle 167 Tests des gesamten
Frontends (nicht nur die neuen) grün, `tsc --noEmit`/`eslint`/`vite build` sauber.

Gegen den echten laufenden Dev-Stack verifiziert (Container neu gebaut, nicht nur kompiliert):
`GET /fleet/vehicles`/`/fleet/alerts` über `http://localhost:3000/fleet/...` (durch nginx, mit
echtem JWT) liefert echte Daten inkl. `vehicle-001` (nur `id`/`display_name`, alles andere fehlt
tatsächlich) und der simulierten `lastenzug-01`/`lastenrad-01`; `POST
/fleet/alerts/{id}/acknowledge` durch nginx bestätigt (204); ein echter WebSocket-Client (Node,
kein Mock) gegen `ws://localhost:3000/fleet/ws?token=...` empfing binnen Sekunden mehrere echte
`vehicle_status`-Events aus der laufenden `vehicle-mock`-Simulation — bestätigt sowohl die
No-Rewrite-Entscheidung (DASH-01) als auch, dass die reale Backend-JSON-Form zu den TS-Interfaces
passt. Testdaten aus früheren Ad-hoc-Verifikationsläufen (`mqtt-test-*`) aus der Dev-DB bereinigt.

**Bewusst nicht vollständig verifiziert:** ein echter interaktiver Browser-Klick-Test (Playwright)
war in dieser Sandbox-Umgebung nicht möglich — kein zur installierten Playwright-Version
passender Chromium-Build ladbar (`ERROR: Playwright does not support chromium on
ubuntu26.04-x64`), kein Workaround gefunden. Das ist eine echte, offene Lücke, keine stillschweigend
übersprungene: das eigentliche Rendering/State-Management ist über die 167 Komponenten-/Hook-Tests
abgedeckt (inklusive exakt der Szenarien, die ein Klick-Test auch prüfen würde — Teleoperate-Button-
Gating, `vehicle-001`-Rendering), und die komplette Netzwerk-/Datenebene (nginx-Routing, REST, WS,
reale JSON-Formen) wurde end-to-end gegen die echten Container verifiziert — nur der letzte Schritt
"echter Browser rendert das ohne Konsolenfehler" fehlt. Empfehlung für eine Folge-Session mit
funktionierendem Playwright-Setup (oder manuell durch den Nutzer im eigenen Browser).

**Update 2026-07-15 — Lücke weiterhin offen, aber Ursache jetzt anders diagnostiziert:** Für einen
neuen Anlauf wurden zwei Browser-MCP-Server in `.mcp.json` eingetragen (`firefox-devtools`,
`chrome-devtools`, siehe Repo-Root). Ergebnis dieses Versuchs: Die MCP-Tools kamen in der
Session nicht als nutzbare Tools an (leere Trefferliste bei jeder `ToolSearch`-Abfrage nach
Browser-/DevTools-Funktionen). Ursache manuell isoliert, keine Vermutung: `~/.claude.json` zeigt
für dieses Projektverzeichnis `hasTrustDialogAccepted: false` und `enabledMcpjsonServers: []` —
Claude Code hat die in `.mcp.json` deklarierten Server nie geladen, weil das projektbezogene
MCP-Trust-Gate (normalerweise ein interaktiver Zustimmungs-Dialog beim Start einer Terminal-
Session) in dieser Session nie durchlaufen wurde. Das ist unabhängig von den Browsern selbst:
`npx -y chrome-devtools-mcp@latest --executablePath=/snap/bin/chromium --headless` manuell in der
Shell gestartet läuft an, gibt sein Start-Banner aus und beendet sich nur, weil kein Client an
stdin hängt — kein Absturz, kein fehlender Chromium-Build. `/usr/bin/firefox --version` läuft
ebenfalls ohne Fehler durch. D.h. beide Browser-Binaries und der Chrome-DevTools-MCP-Server sind
grundsätzlich startfähig; blockiert ist einzig die Trust-Freigabe der projektweiten `.mcp.json`
durch den Nutzer. Diese Freigabe kann nur interaktiv (Trust-Dialog beim Start von `claude` in
diesem Verzeichnis, oder manuelle Aufnahme der Servernamen in
`enabledMcpjsonServers` durch den Nutzer selbst) erteilt werden — wurde in dieser Session bewusst
nicht selbst umgangen, da das ein Sicherheits-/Zustimmungsschritt ist. Dev-Stack lief zum
Testzeitpunkt bereits vollständig (`docker ps`: alle `avoc-*`-Container inkl. beider
`vehicle-mock`-Instanzen seit Stunden up), stand also bereit — der eigentliche
Browser-Klick-Test (Login, Live-Update via WS, `vehicle-001`-Rendering, Teleoperate-Button-Gating
über zwei parallele Operator-Sessions) konnte dadurch wieder nicht durchgeführt werden. Nächster
Schritt: Nutzer startet einmal `claude` interaktiv in diesem Repo und bestätigt den MCP-Trust-
Dialog für `firefox-devtools`/`chrome-devtools` (oder gibt ausdrücklich frei, dies programmatisch
in `~/.claude.json` einzutragen) — danach ist der eigentliche Test in einer Folge-Session
voraussichtlich ohne weitere Hindernisse möglich.

**Update 2026-07-15 (Fortsetzung) — Trust-Dialog bestätigt, Lücke trotzdem noch offen, jetzt aber
präziser eingegrenzt:** Nutzer hat den Workspace-Trust-Dialog für dieses Projekt inzwischen
interaktiv bestätigt — `~/.claude.json` zeigt für den Projektpfad jetzt `hasTrustDialogAccepted:
true` (vorher `false`). Erneuter Test in einer neuen Subagent-Session direkt danach: `ToolSearch`
nach Browser-/DevTools-Funktionen ("browser screenshot navigate", danach zusätzlich "firefox
chrome devtools mcp page click evaluate") liefert weiterhin ausschließlich das eingebaute
`WebFetch` zurück — kein einziges `firefox-devtools`- oder `chrome-devtools`-Tool taucht als
Deferred Tool auf. Zweite Prüfung von `~/.claude.json` direkt danach: `enabledMcpjsonServers` UND
`disabledMcpjsonServers` sind beide weiterhin leere Arrays (`[]`) — keine der beiden in `.mcp.json`
deklarierten Server-IDs ist dort eingetragen, weder positiv noch negativ. Das belegt: Workspace-
Trust (`hasTrustDialogAccepted`) und die Freigabe einzelner projektbezogener `.mcp.json`-Server
(`enabledMcpjsonServers`) sind zwei getrennte Gates in Claude Code — die Bestätigung des einen löst
das andere nicht automatisch mit aus. Zusätzlich geprüft, ob sich das Gate von dieser Session aus
selbst schließen ließe: in der Bash-Umgebung dieses Subagents ist kein `claude`-Binary im PATH
(`command not found: claude`) — der Slash-Befehl `/mcp`, über den die interaktive
Server-Zustimmung normalerweise abläuft, ist ausschließlich an die interaktive TUI-Session
gebunden und von einer Hintergrund-/Subagent-Session aus nicht aufrufbar. Es gibt also aktuell
keinen Weg, diese Freigabe aus einer solchen Session heraus zu erteilen. Wie in der Aufgabenstellung
vorgegeben, wurde `~/.claude.json` NICHT selbst verändert, um die Freigabe zu erzwingen. Schritte
2–4 (Browser starten, Dev-Stack-Check, eigentlicher Login-/Live-Update-/Teleoperate-Gating-Test)
wurden entsprechend nicht durchgeführt, da sie zwingend funktionierende Browser-MCP-Tools
voraussetzen. Damit ist das ursprünglich vermutete Henne-Ei-Problem bestätigt und präzisiert: Der
fehlende letzte Verifikationsschritt ("echter Browser rendert Login → Fleet Overview → Live-Update
→ Teleoperate-Gating ohne Konsolenfehler") lässt sich mit dem aktuellen Session-Modell (Subagent /
Hintergrund-Job) grundsätzlich nicht schließen — das ist eine Umgebungsgrenze, kein im Rahmen
dieser Aufgabe lösbares Problem. Um weiterzukommen, muss der Nutzer selbst einmal interaktiv
`claude` in diesem Repo-Verzeichnis starten und dort — falls der Zustimmungs-Dialog für
`firefox-devtools`/`chrome-devtools` erscheint — bestätigen, oder die beiden Servernamen manuell in
`enabledMcpjsonServers` in `~/.claude.json` eintragen. Bis dahin bleibt der Browser-Klick-Test
offen; alle bisher per curl/rohem WebSocket-Client verifizierten Backend-Ebenen und die 167
Komponenten-/Hook-Tests sind davon unberührt und weiterhin gültig.

**Bewusst nicht in diesem Sprint:** Karten-/Zonen-Visualisierung (`GET /fleet/zones`/`/fleet/stations`
existieren bereits, aber keine SVG-Rendering-Komponente), Task-Management-UI (`GET/POST
/fleet/tasks` ebenfalls schon vorhanden), Dark-Mode-Umschaltung (UI bleibt fest dunkel wie
bisher), System-Steuerung/-Override-Panel, Performance-Monitoring jenseits der vier gezeigten
Telemetriewerte (Batterie/Geschwindigkeit/Autonomie-Modus/Zone). Alles Folge-Tasks für einen
späteren Sprint, sobald konkret gebraucht.
