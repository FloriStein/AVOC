> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Kein aktiver Sprint

Sprint 51 (Testabdeckungs-Gesamtaudit 2026-07-21, Teil 1 — Safety-kritische Backend-Testlücken) ist
abgeschlossen — Volltext in
[tasks/sprints/51-testabdeckung-safety-backend.md](sprints/51-testabdeckung-safety-backend.md).

Mögliche nächste Sprints (Nummer jeweils erst bei Kickoff vergeben, siehe [tasks/backlog.md](backlog.md)):
- **Testabdeckungs-Gesamtaudit 2026-07-21, Teil 2** — Fehlende Integrationstests zwischen Services
  (`webrtc-sfu`/`internal/mediamtx` und `telemetry-service` fehlen im Docker-Teststack).
- **Testabdeckungs-Gesamtaudit 2026-07-21, Teil 3** — Frontend Session-/Safety-kritische Hooks
  (`useSession.ts`, `SafetyPanel.test.tsx`, `useControls.ts`, `ws-client.ts`).
- Teil 4 (E2E-Flow-Ausbau) und Teil 5 (CI-Härtung) desselben EPICs.
- oder ein anderer Backlog-Punkt nach Nutzerfreigabe.
