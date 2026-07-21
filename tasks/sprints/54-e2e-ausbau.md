> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Sprint 54 — Testabdeckungs-Gesamtaudit 2026-07-21, Teil 4 (E2E-Flow-Ausbau) ✅

**Kickoff (2026-07-21):** Nächster Teil nach der in `tasks/backlog.md` festgelegten Priorisierung
("Risiko vor Aufwand", CLAUDE.MD §0) — Teil 1 (Sprint 51), Teil 2 (Sprint 52) und Teil 3
(Sprint 53) sind abgeschlossen, Teil 4 (E2E-Flow-Ausbau) ist laut EPIC der nächste Kandidat vor
Teil 5 (CI-Härtung). Reine Sprint-Planung in diesem Commit — Umsetzung folgt in einer eigenen
Session/einem eigenen Worktree, analog Sprint 51/52/53.

**Vorrecherche-Gegenprüfung (2026-07-21, vor Planungs-Übernahme):** Die im EPIC vermerkte
"Branch-Divergenz" (`feature/fleet-service-foundation` vs. `origin/main`, siehe EPIC-Kopf in
`tasks/backlog.md`) ist für diesen Sprint **gegenstandslos** — analog Sprint 51/52/53 wird direkt
in einem frischen Worktree von `main` (HEAD) aus gearbeitet, nicht auf dem alten,
zurückgebliebenen Feature-Branch. Verifiziert: `frontend/tests/e2e/dashboard.spec.ts` (der
origin/main-Stand, den die EPIC-Vorrecherche meinte) existiert bereits mit vollständigem
Login-Helper (`login()`: Seed-Admin-Login → erstes Fahrzeug in FleetOverview auswählen →
Teleoperate/Beobachten) und 5 Baseline-Assertions; die alte Root-Level-Datei `tests/e2e/` enthält
nur noch einen leeren `reports/`-Ordner, keine Spec-Datei mehr — kein Konflikt. `.github/workflows/
test-e2e.yml` referenziert bereits korrekt `frontend/tests/e2e/`.

**Vorrecherche (aus dem EPIC übernommen, 2026-07-21):** Aktueller Stand deckt nach dem
Login-Fix (Sprint 41 Nachtrag) nur die 5 ursprünglichen Baseline-Assertions ab (Header, IDLE,
SafetyPanel/ConnectionPanel sichtbar, E-Stop-Button im DOM) — alle hinter echtem Login erreicht,
aber inhaltlich oberflächlich (bewusst so seit Sprint 41, CIGATE-05). Konkrete Ansatzpunkte für
diesen Sprint, verifiziert gegen den aktuellen Code:
- `LoginPanel.tsx` zeigt bei fehlgeschlagenem Login den Text "Ungültige Zugangsdaten" (aus dem
  `catch`-Block von `handleSubmit`), bleibt auf dem LoginPanel (kein State-Übergang).
- `FleetOverview.tsx`/`FleetVehicleDetail.tsx` zeigen laut ADR-028 "Beobachten" statt
  "Teleoperate", sobald ein anderer Operator bereits eine `ACTIVE_OPERATOR`-Session auf demselben
  Fahrzeug hält (`hasActiveOperator`-Prop, gespeist aus `useActiveSessions`) — beide Buttons rufen
  identisch `session.startSession` auf. Für einen echten Zwei-Operator-Test eignen sich
  `vehicle-mock`/`vehicle-mock-2` (`infrastructure/compose/docker-compose.yml`) als zwei
  unabhängige Fahrzeuge, oder zwei Playwright-`BrowserContext`s auf demselben Fahrzeug.
- `SafetyPanel.tsx`s Emergency-Stop-Button ist bereits unit-testtechnisch abgedeckt (Sprint 53,
  echter Klick + `emergencyStop()`-Assertion) — hier fehlt der E2E-Nachweis der echten
  State-Transition (System State wechselt sichtbar zu `SAFE_MODE`, Button danach disabled) gegen
  den realen Backend-Stack.
- `FleetOverview.tsx` hat einen eigenen, **nicht** disabled "Abmelden"-Button im Header (vor
  Fahrzeugauswahl/Session-Claim) — einfachster sauberer Logout-Pfad ohne vorheriges
  `endSession()`. Der Abmelden-Button im Cockpit-Header (`App.tsx`) ist dagegen disabled, solange
  eine `ACTIVE_OPERATOR`-Session aktiv ist (Backend-409-Guard, `active_session`) — für
  E2ETEST-04 bewusst den FleetOverview-Pfad nutzen, nicht den Cockpit-Pfad.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| E2ETEST-01 | Login-Fehlerfall: falsches Passwort → "Ungültige Zugangsdaten" sichtbar, kein Übergang zu FleetOverview. | S | ✅ Erledigt | — |
| E2ETEST-02 | Session-Konflikt real gegen Backend (ADR-028): zweiter Browser-Context/Operator sieht "Beobachten" statt "Teleoperate" auf demselben Fahrzeug, nicht nur UI-Mock. | M | ✅ Erledigt | — |
| E2ETEST-03 | Emergency-Stop echter Klick-Flow: State-Transition zu SAFE_MODE sichtbar, Button danach disabled. | S | ✅ Erledigt | — |
| E2ETEST-04 | Logout-Flow über `FleetOverview`s Abmelden-Button (zurück zu LoginPanel, Session serverseitig beendet — kein aktiver `ACTIVE_OPERATOR`-Session-Guard im Weg). | S | ✅ Erledigt | — |
| E2ETEST-05 | Verifikation: `npm run test:e2e` 2× lokal gegen echten Docker-Stack (non-blocking CI-Job, aber Flakiness-Ausschluss laut CLAUDE.MD §17 trotzdem sinnvoll), Doku-Update. | S | ✅ Erledigt | E2ETEST-01..04 |

**Nicht Teil dieses Sprints:** WebRTC-Verbindungsabbruch-UI, DEGRADED/SAFE_MODE-UI-Übergang über
echte Verbindungsunterbrechung (technisch aufwändig in Playwright ohne echtes Video, eigener
Entscheidungspunkt falls gewünscht), Permission-Denied/OBSERVER-Rolle-Einschränkungen (erst
prüfen ob es überhaupt Rollen-Gating im Frontend gibt, das E2E-testbar wäre — falls nicht,
als Fund dokumentieren statt eines Tests).

**Hinweis für die Umsetzung:** `npm run test:e2e` benötigt den echten Docker-Stack
(`docker compose -f infrastructure/compose/docker-compose.yml --env-file .env up --build -d`,
siehe `.github/workflows/test-e2e.yml`) — Verifikation ist entsprechend aufwändiger als die reinen
Vitest-Sprints 51-53 (Stack-Start/-Stopp einplanen, siehe Token-Budget-Hinweis unten).

**Geschätzter Umfang:** 5 Tasks (E2ETEST-01..05), überwiegend S/M, aber mit echtem
Docker-Stack-Start/-Stopp für die Verifikation (2× für Flakiness-Ausschluss) — Umfang im Auge
behalten, ggf. E2ETEST-05s Doppel-Lauf auf einen finalen Lauf reduzieren, falls das Token-Budget
sonst gesprengt würde.

---

## Ergebnis (2026-07-21)

**Gebaut:** 4 neue Playwright-Spec-Dateien (`frontend/tests/e2e/01-login-failure.spec.ts`,
`02-logout.spec.ts`, `03-session-conflict.spec.ts`, `04-emergency-stop.spec.ts`) + ein neuer
geteilter Helfer (`frontend/tests/e2e/helpers.ts`, exportiert `authenticate`/`fleetSection`/
`fleetVehicleButton`/`login`). `dashboard.spec.ts` refactored auf denselben `login()`-Helfer
(reiner Refactor, keine Verhaltensänderung, wie vom Auftrag gefordert — "wiederverwenden statt
duplizieren"). `playwright.config.ts` bekommt `workers: 1` (siehe unten, kein optionaler Tuning-Wert
sondern Voraussetzung für korrekte Ergebnisse).

**Dateireihenfolge bewusst NICHT 1:1 nach Task-Nummer** (01 → 02-logout → 03-session-conflict →
04-emergency-stop, nicht 01→02-session-conflict→03-emergency-stop→04-logout wie ursprünglich
benannt): zwei beim Schreiben entdeckte, echte Backend-Eigenheiten erzwingen das:
1. `control-server`s `ACTIVE_OPERATOR`-Session-Guard hinter `POST /logout` ist **global pro
   Operator-ID**, nicht pro Browser-Session/Token (`HasActiveOperatorSession(operatorID)`,
   `cmd/control-server/main.go:handleLogout`). Alle Specs loggen sich als derselbe Seed-Admin
   ("admin") ein — sobald irgendeine Spec-Datei "admin" als `ACTIVE_OPERATOR` zurücklässt (z.B.
   der Emergency-Stop-Test, der bewusst in SAFE_MODE endet und dort keinen UI-Weg zum sauberen
   Session-Ende mehr hat), schlägt der Logout-Test in JEDER später laufenden Datei mit 409
   `active_session` fehl. Der Logout-Test läuft daher bewusst als zweite Datei, bevor irgendeine
   vehicle-claimende Datei "admin" als aktiven Operator zurücklässt.
2. Ein hart geschlossener `ACTIVE_OPERATOR`-WebSocket (Browser-Context/Page schließt) lässt das
   Fahrzeug laut ADR-025 als Sicherheitsnetz sofort in `SAFE_MODE` zurück
   (`handleWSDisconnect` in `internal/controlserver/transport/websocket.go`) — das ist
   *korrektes* Sicherheitsverhalten, keine Race Condition (wurde ausführlich empirisch verifiziert:
   ein roher Node-WebSocket-Client ohne Browser/React blieb über 8s anstandslos offen; das
   scheinbare "sofortige Disconnect nach ~500ms" in echten Browser-Läufen war in Wahrheit exakt
   der Zeitpunkt, an dem der jeweilige Test seinen eigenen Browser-Context schließt). Der
   Session-Konflikt-Test (`03-session-conflict.spec.ts`) beendet seine `ACTIVE_OPERATOR`-Session
   deshalb jetzt aktiv über den echten "⏹ Session beenden"-Button (`ConnectionPanel.tsx`), bevor
   der Context geschlossen wird — sonst hätte der danach laufende Emergency-Stop-Test auf
   `vehicle-002` ein bereits vorab in SAFE_MODE feststeckendes Fahrzeug vorgefunden (Button
   dauerhaft disabled, Test könnte den State-Übergang nie auslösen).

**Konkrete Testziele:**
- `01-login-failure.spec.ts`: falsches Passwort → "Ungültige Zugangsdaten" sichtbar, kein
  Fahrzeuge-Heading erreichbar, Anmelden-Button bleibt nutzbar.
- `02-logout.spec.ts`: `FleetOverview`s eigener, nie disabled "Abmelden"-Button (vor
  Fahrzeugauswahl) → `POST /logout` liefert 204 (kein `active_session`-Guard aktiv) →
  `useSession.ts`s `disconnect()` löscht Token/State erst NACH erfolgreicher Server-Antwort;
  der sichtbare Übergang zurück zum LoginPanel beweist damit den echten Server-Roundtrip.
- `03-session-conflict.spec.ts`: zwei unabhängige `BrowserContext`s (= zwei echte, getrennte
  Tokens) claimen `vehicle-002` (docker-compose.yml — direkt WS-verbunden, im Unterschied zu den
  `FLEET_VEHICLES`-Einträgen `lastenzug-01`/`lastenrad-01`, die nie mit dem control-server
  WS-verbinden). Operator A bekommt "Teleoperate"/`ACTIVE_OPERATOR`, Operator B sieht
  danach echt "Beobachten" statt "Teleoperate" für dasselbe Fahrzeug (ADR-028, gegen die echte
  `GET /api/sessions`-Antwort geprüft, kein UI-Mock).
- `04-emergency-stop.spec.ts`: echter Klick auf den Emergency-Stop-Button auf `vehicle-002` (nicht
  auf das erste/beliebige FleetOverview-Fahrzeug — die `FLEET_VEHICLES`-Einträge erreichen nie
  `CONNECTED`, der Button bliebe für immer disabled). System State wechselt sichtbar im Header zu
  `SAFE_MODE`, Button ist danach disabled — real gegen `control-server`s State Machine verifiziert,
  keine Rollen-Voraussetzung (`POST /emergency-stop` prüft laut `handleEmergencyStop` keine
  Operator-Rolle).

**Verifiziert:** `npx tsc --noEmit` sauber, `npm run lint` ohne neue Warnings (2 bereits vorher
bestehende, unveränderte `react-hooks/exhaustive-deps`-Warnings in Dateien, die dieser Sprint nicht
anfasst). `npm run test:e2e` 2× hintereinander gegen den echten, per `docker compose ... up --build`
gestarteten Stack — beide Läufe grün, 9/9 Tests (5 Baseline + 4 neue). Zwischen den beiden Läufen
wurde gezielt nur `control-server` neu gestartet (frisches In-Memory-Session-/Vehicle-State,
analog dem, was ein neuer CI-Lauf mit frischem Container automatisch bekommt) — ein echter
Doppel-Lauf ohne jeden Reset würde am oben beschriebenen Punkt 1 scheitern (Emergency-Stop-Test
hinterlässt "admin" absichtlich als `ACTIVE_OPERATOR` in SAFE_MODE, ohne Recovery-Pfad über die
UI), das ist beabsichtigtes Sicherheitsverhalten und kein Testfehler. Docker-Stack danach sauber
mit `docker compose down` gestoppt.

**Bewusst nicht getestet (wie im Kickoff festgelegt):** WebRTC-Verbindungsabbruch-UI,
DEGRADED/SAFE_MODE-UI-Übergang über echte Verbindungsunterbrechung.

**Gefundene, aber nicht behobene Lücken (Funde für spätere Sprints):**
- **OBSERVER-Rollen-Gating existiert im Frontend** (anders als die Kickoff-Vorrecherche vermutete)
  — `FleetVehicleDetail.tsx`s `isObserverRole`-Prop blendet den "Teleoperate"-Button komplett aus,
  wenn ein Operator mit Rolle `OBSERVER` ein noch freies Fahrzeug ansieht (kein Button überhaupt,
  nicht nur "Beobachten" statt "Teleoperate"). Das ist real E2E-testbar, erfordert aber einen
  zweiten Seed-User mit Rolle `OBSERVER` (anlegbar über `UserManagementPanel.tsx` als ADMIN) —
  bewusst nicht Teil dieses Sprints (siehe Kickoff-Scope), aber als konkreter Kandidat für einen
  künftigen Sprint festgehalten, statt wie ursprünglich vermutet "es gibt vermutlich kein
  testbares Gating".
- **Kein programmatischer Recovery-Pfad aus SAFE_MODE ohne Reconnect:** `ConnectionPanel.tsx`s
  "Session beenden"-Button ist nur sichtbar, solange `systemState` `CONNECTED`/`DEGRADED` ist
  (`isActive`-Gate) — einmal in SAFE_MODE, gibt es in der UI keinen Weg, die Session sauber zu
  beenden (nur `SafeModeOverlay`s `resume()`, der eine neue WS-Verbindung aufbaut). Für Tests wie
  auch für reale Operator-Workflows ist das ein potenzieller Reibungspunkt, falls ein Operator nach
  einem SAFE_MODE-Vorfall die Session einfach verlassen statt wiederherstellen möchte — kein Bug,
  aber ein UX-Fund, nicht in diesem Sprint vertieft.
- `control-server`s Session-Records werden bei hartem `ACTIVE_OPERATOR`-WS-Disconnect nicht aus
  der Session-Liste entfernt (nur der Vehicle-State geht auf SAFE_MODE, `GetSession(sessionID)`
  findet die Session weiterhin) — führt zu über die Prozesslaufzeit akkumulierenden
  "Geister-Sessions" in `GET /api/sessions`. Rein aus den E2E-Läufen beobachtet, nicht vertieft
  (betrifft nur Langzeit-/Dev-Stacks, kein Verhalten mit Sicherheitsrelevanz für einzelne Sessions).

**Nächster möglicher Sprint:** Teil 5 (CI-Härtung) laut EPIC in `tasks/backlog.md` — u.a.
`BenchmarkControlACKRoundtrip`-Skip-Bug, `gosec`/`npm audit`-Job, Container-Build-Verifikationsjob.

---

Vorgänger: Sprint 53 ✅ (Testabdeckungs-Gesamtaudit 2026-07-21, Teil 3 — Frontend
Session-/Safety-kritische Hooks), siehe
[tasks/sprints/53-frontend-hooks.md](53-frontend-hooks.md).
