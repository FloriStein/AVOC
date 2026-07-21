> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Sprint 54 — Testabdeckungs-Gesamtaudit 2026-07-21, Teil 4 (E2E-Flow-Ausbau)

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
| E2ETEST-01 | Login-Fehlerfall: falsches Passwort → "Ungültige Zugangsdaten" sichtbar, kein Übergang zu FleetOverview. | S | 🔲 Backlog | — |
| E2ETEST-02 | Session-Konflikt real gegen Backend (ADR-028): zweiter Browser-Context/Operator sieht "Beobachten" statt "Teleoperate" auf demselben Fahrzeug, nicht nur UI-Mock. | M | 🔲 Backlog | — |
| E2ETEST-03 | Emergency-Stop echter Klick-Flow: State-Transition zu SAFE_MODE sichtbar, Button danach disabled. | S | 🔲 Backlog | — |
| E2ETEST-04 | Logout-Flow über `FleetOverview`s Abmelden-Button (zurück zu LoginPanel, Session serverseitig beendet — kein aktiver `ACTIVE_OPERATOR`-Session-Guard im Weg). | S | 🔲 Backlog | — |
| E2ETEST-05 | Verifikation: `npm run test:e2e` 2× lokal gegen echten Docker-Stack (non-blocking CI-Job, aber Flakiness-Ausschluss laut CLAUDE.MD §17 trotzdem sinnvoll), Doku-Update. | S | 🔲 Backlog | E2ETEST-01..04 |

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

Vorgänger: Sprint 53 ✅ (Testabdeckungs-Gesamtaudit 2026-07-21, Teil 3 — Frontend
Session-/Safety-kritische Hooks), siehe
[tasks/sprints/53-frontend-hooks.md](sprints/53-frontend-hooks.md).
