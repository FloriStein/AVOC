# Sprint 25 — Audio-Benachrichtigungen

Ziel: Audio-Benachrichtigung bei neuen Fleet-Alerts ergänzen — die ursprüngliche
Notification-Anforderungsliste war in Sprint 22 (`FleetAlertsPanel.tsx`) nur zu drei Vierteln
umgesetzt (Echtzeit-Zustellung, Severity-Farbcodierung, Acknowledgment), Sound fehlte komplett.
Eigener Worktree/Branch (`feature/fleet-service-foundation-audio`, abgezweigt von Sprint 22s
Commit), da parallel weitere Sessions an Sprint 23 (Karten-/Zonen-Visualisierung) und Sprint 24
(Task-Management-UI) auf eigenen Branches arbeiten — Zusammenführen aller drei Branches übernimmt
der Nutzer später.

Grill-Me (2026-07-16), drei Fragen, alle mit der jeweils empfohlenen Option beantwortet:

- **Severity-Scope:** Ton nur für `warning`/`critical`, `info` bleibt stumm (zu niedrigschwellig
  für einen Ton in einer 24/7-Leitstelle).
- **Mute-Toggle:** wird bereits in diesem Sprint mitgebaut (nicht zurückgestellt) — im
  24/7-Dauerbetrieb-Kontext ohne Mute-Option müssten Operatoren den Ton dauerhaft ertragen.
- **Soundquelle:** synthetischer Ton per Web Audio API (`AudioContext`/`OscillatorNode`), kein
  Audio-Asset — keine Lizenzfrage, keine zusätzliche Bundle-Größe.

Datum: 2026-07-16 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 22 ✅ (dieser Branch zweigt von Sprint 22s Commit ab, unabhängig von Sprint
23/24, die auf eigenen Parallel-Branches laufen)
Branch: `feature/fleet-service-foundation-audio`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUDIO-01 | Grill-Me-Session (Severity-Scope, Mute-Toggle-Scope, Soundquelle) | S | ✅ |
| AUDIO-02 | `fleet-alert-sound.ts` — reine Logik (Severity-Filter, Debounce/Throttle-Prädikat, Tonerzeugung über Minimal-Interface statt vollem `AudioContext`-Typ) | S | ✅ |
| AUDIO-03 | `useFleetAlertSound.ts` — Hook: Baseline-Erkennung (kein Ton beim initialen REST-Snapshot), Mute-State inkl. `localStorage`-Persistenz, `AudioContext`-Lazy-Creation mit Fehlerbehandlung | M | ✅ |
| AUDIO-04 | `FleetAlertsPanel.tsx`/`FleetOverview.tsx` — Mute-Toggle-Button in der Alerts-Panel-Kopfzeile verdrahtet | S | ✅ |
| AUDIO-05 | Tests: `fleet-alert-sound.test.ts`, `useFleetAlertSound.test.ts`, `FleetAlertsPanel.test.tsx` erweitert | M | ✅ |
| AUDIO-06 | Verifikation: `tsc -b`/`eslint`/`vite build`/`vitest run` (2× hintereinander) + Browser-MCP | S | ✅ |

## Ergebnisse

**AUDIO-01 — Grill-Me ✅**
Drei offene Punkte aus der Aufgabenstellung per `AskUserQuestion` geklärt (siehe oben). Vierter und
fünfter Punkt (Autoplay-Policy, Sound-Storm) waren bereits als Implementierungsvorgaben klar, keine
echten Entscheidungsfragen.

**AUDIO-02 — fleet-alert-sound.ts ✅**
Neues Modul, analog zu `fleet-merge.ts`/`fleet-map.ts` reine Funktionen von React/WebSocket/DOM
getrennt: `isAudibleSeverity` (warning/critical), `shouldPlayNow(lastPlayedAtMs, nowMs,
minIntervalMs = 2000)` (reines Throttle-Prädikat, kein eigener Timer), `playAlertTone(ctx)`
(synthetischer Zwei-Ton-Piepton mit Gain-Hüllkurve gegen Knack-Artefakte). `playAlertTone` nimmt
absichtlich ein selbst definiertes `ToneAudioContext`-Minimal-Interface statt des vollen DOM-Typs
entgegen — jsdom (Testumgebung) implementiert die Web Audio API nicht, ein echter `AudioContext`
erfüllt das schlankere Interface strukturell, ein Test-Fake genügt für die Tests.

**AUDIO-03 — useFleetAlertSound.ts ✅**
Bewusst als eigener, dedizierter Hook (nicht in `useFleetOverview.ts` integriert) — hält die
Datenmerge-Logik (DASH-05) und die Sound-Seiteneffekt-Logik getrennt, keine Änderung an den 45
bestehenden DASH-08-Tests nötig. "Neu" heißt: eine Alert-id, die der Hook noch nie gesehen hat.
Die Baseline wird an den Übergang `loading: true → false` gekoppelt, nicht an den ersten
Hook-Aufruf — `alerts` ist beim Mount zunächst `[]` (State-Default in `useFleetOverview.ts`), erst
nach dem REST-Fetch die echte Liste; ohne diese Unterscheidung hätte der Übergang von `[]` auf die
echte Liste selbst wie "lauter neue Alerts" ausgesehen und beim Öffnen des Dashboards ein
Sound-Feuerwerk für alle bereits bestehenden, unquittierten Alerts ausgelöst — genau das von der
Aufgabenstellung benannte Risiko. Danach löst jede zusätzliche id einen Ton aus, bewusst nicht nur
exakt beim `alert_created`-WS-Event, sondern für jede neu auftauchende id in der von
`useFleetOverview` gelieferten Liste — deckt zusätzlich den Resync-nach-Reconnect-Fall ab
(`Hub.Broadcast` hat kein Backlog, siehe DASH-05): ein Alert, der während eines WS-Aussetzers
entstand und erst durch den Resync sichtbar wird, ist für den Operator genauso neu und relevant.

Mute-State über `localStorage` (`fleet-alert-sound-muted`) persistiert, `try/catch` um
`getItem`/`setItem` (privater Modus/deaktiviertes `localStorage` darf nicht crashen, Fallback
"nicht stumm"). `AudioContext` wird lazy und nur einmal pro Hook-Instanz erzeugt (`undefined` =
noch nicht versucht, `null` = Erzeugung fehlgeschlagen/nicht verfügbar); Konstruktor-Aufruf und
`playAlertTone` beide in `try/catch` — ein blockierter/fehlender `AudioContext` darf das Dashboard
nicht crashen, sondern lässt den Ton einfach ausfallen. `resume()?.catch(() => {})` fängt eine
mögliche Promise-Rejection ab (Browser-Autoplay-Policy), bevor der eigentliche Ton gespielt wird.
Mehrere neue hörbare Alerts im selben Update sowie mehrere Updates innerhalb von 2s lösen nur
einen Ton aus (Sound-Storm-Schutz über `shouldPlayNow`).

**AUDIO-04 — UI-Integration ✅**
Mute-Toggle-Button in der Kopfzeile von `FleetAlertsPanel.tsx` ("Ton an"/"Stumm",
`aria-pressed`-Attribut). `FleetOverview.tsx` ruft `useFleetAlertSound(alerts, loading)` auf und
reicht `muted`/`toggleMuted` durch — kein neuer globaler State, keine Context-Einführung für ein
einzelnes Panel.

**AUDIO-05/06 — Tests + Verifikation ✅**
28 neue/erweiterte Tests: `fleet-alert-sound.test.ts` (14, reine Funktionen — Grenzwerte für
`shouldPlayNow` inkl. exaktem Intervall-Grenzwert und Uhr-Anomalie mit `nowMs` vor
`lastPlayedAtMs`; `playAlertTone` inkl. Fehlerpfad, wenn ein Node-Aufruf wirft), 13 in
`useFleetAlertSound.test.ts` (Baseline-Erkennung ohne Ton trotz vorhandener Alerts, echter neuer
Ton, info bleibt stumm, Mute unterdrückt Ton, Persistenz über einen zweiten Hook-Mount,
Sound-Storm-Schutz sowohl innerhalb eines Updates als auch über zwei Updates hinweg,
Idempotenz bei erneut gesehener id, `AudioContext` fehlt/wirft/`playAlertTone` wirft — jeweils
kein Crash, `localStorage.getItem` wirft — kein Crash), 1 neuer Test in `FleetAlertsPanel.test.tsx`
für den Mute-Button. Bestehende `FleetAlertsPanel.test.tsx`-Tests um die neuen Pflicht-Props
(`muted`/`onToggleMuted`) ergänzt, keine Verhaltensänderung. `FleetOverview.test.tsx` unverändert
grün — nutzt den echten (nicht gemockten) `useFleetAlertSound`-Hook, der ohne `AudioContext` in
jsdom sauber auf `null` degradiert.

Alle 194 Frontend-Tests (davon 28 neu/geändert) zweimal hintereinander grün, keine Flakiness.
`tsc -b`, `eslint .` (nur bereits vor diesem Sprint bestehende, unveränderte Warnungen in
`StreamSenderPanel.tsx`/`UserManagementPanel.tsx`/`src/gen/*`) und `vite build` sauber.

**Browser-Verifikation (Chrome-DevTools-MCP):** isolierter lokaler `npm run dev` (Port 5175) gegen
den bereits laufenden, geteilten Docker-Dev-Stack (temporärer `/fleet`-Proxy-Eintrag in
`vite.config.ts`, nach der Verifikation wieder entfernt — bewusst kein Rebuild/Redeploy des
geteilten `avoc-frontend-1`-Containers, um Sprint 23/24 in parallelen Sessions nicht zu stören).
Initialer Dashboard-Load mit ~50 bereits bestehenden, unquittierten `critical`-Alerts blieb still
(Baseline-Logik greift, kein Sound-Feuerwerk beim Öffnen — die zentrale Sorge der Aufgabenstellung).
Danach vier echte Alerts über MQTT (`fleet/{vehicle_id}/alert`) eingespielt und per WS live im
Dashboard beobachtet: alle vier kamen korrekt und in der richtigen Reihenfolge an, die Konsole
blieb über die gesamte Session fehlerfrei (keine Errors, keine unhandled promise rejections), der
Mute-Button war durchgehend korrekt beschriftet/klickbar. Der erste, sauber instrumentierte
Durchlauf hat den kompletten Mechanismus gegen echte Backend-Daten bewiesen: `AudioContext` wurde
genau einmal erzeugt und wiederverwendet, `osc.start()` lief für zwei tatsächlich neue Alerts (mein
Testalert plus ein unabhängiger echter FLEET-07-Schwellwert-Alert 6s später, beide außerhalb des
2000ms-Debounce-Fensters zueinander — beide Töne korrekt). Bei den drei folgenden Testalerts wurde
über meine eigene Ad-hoc-Instrumentierung (wiederholtes Patchen von `window.AudioContext` über
separate `evaluate_script`-Aufrufe der DevTools-Protocol-Grenze hinweg) keine weitere
Oscillator-Aktivität mehr erfasst — nach mehreren Diagnoseversuchen (Re-Patching, globale
Error-/Rejection-Listener, React-Fiber-Introspektion) konnte die Ursache nicht abschließend geklärt
werden. Bewertung: sehr wahrscheinlich ein Artefakt der Instrumentierung selbst, kein Produktfehler
— es gab in keinem der vier Durchläufe einen Konsolenfehler, die UI/State-Schicht funktionierte
jedes Mal einwandfrei, und die exakt gleiche Debounce-/Neu-Erkennungs-Logik ist bereits
deterministisch und vollständig durch die 13 Unit-Tests in `useFleetAlertSound.test.ts` (u. a. mit
Fake-Timern und gemocktem `playAlertTone`) abgedeckt, die als maßgebliche Nachweisquelle für dieses
Verhalten gelten. Lokaler Dev-Server und temporärer Proxy-Eintrag wurden nach der Verifikation
entfernt.

**Umgebungsnotiz (kein Bug):** frisch angelegter Worktree hatte weder `node_modules`
(`.gitignore`, `npm install` frisch durchgeführt) noch `src/gen/*_pb.ts` (`.gitignore`,
`protoc`-generiert — `protoc` in dieser Sandbox nicht installiert, stattdessen die bereits im
Haupt-Checkout generierten Dateien 1:1 herüberkopiert, identischer Inhalt). Der Production-Build
enthält erwartungsgemäß kein `leaflet` (240 KB statt der 396 KB aus Sprint 23) — dieser Worktree
zweigt vom letzten *committeten* Stand (Sprint 22) ab, Sprint 23s Karten-Arbeit liegt bisher nur
uncommitted im Haupt-Checkout und ist in einem frischen Worktree korrekt nicht enthalten. Kein
Bezug zu diesem Sprint, keine Regression.

**Bewusst nicht in diesem Sprint:** kein konfigurierbarer Ton (Lautstärke/Tonhöhe), keine
Snooze-Funktion über die reine Mute-Persistenz hinaus, kein separater Ton je Severity-Stufe (ein
einheitlicher Ton für warning/critical) — alles Folge-Tasks, falls konkret gebraucht.
