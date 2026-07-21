> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Kein aktiver Sprint

Sprint 47 (Multi-Cause-DEGRADED-Fundament in der State Machine, DRIFT-K3-TELEMETRY Teil 1) ist
abgeschlossen — Volltext in
[tasks/sprints/47-multi-cause-degraded-fundament.md](sprints/47-multi-cause-degraded-fundament.md).

Mögliche nächste Sprints (Nummer jeweils erst bei Kickoff vergeben, siehe [tasks/backlog.md](backlog.md)):
- **DRIFT-K3-TELEMETRY Teil 2** — der eigentliche `TelemetryWatchdog`: neues Paket
  `internal/controlserver/telemetrycheck`, Verdrahtung in `vehiclecontext.Registry`/
  `cmd/control-server/main.go`, `TELEMETRY_SERVICE_URL`, Schwellwert-Entscheidungen per eigener
  Grill-Me-Session — Architekturskizze bereits in
  [docs/adr/009-failure-model.md](../docs/adr/009-failure-model.md) Update 2026-07-20 dokumentiert.
  (War in Sprint 47 als "Sprint 48" angekündigt — die Nummer ist inzwischen durch die lokale
  Ansible-VM belegt, siehe `DECISIONS.MD`; nächste freie Nummer ist aktuell 51, da 48/49 die
  Ansible-VM und 50 Teil 1 des Testabdeckungs-Gesamtaudits belegen.)
- **Testabdeckungs-Gesamtaudit 2026-07-21, Teil 1** (Sprint 50, bereits vollständig geplant) —
  Safety-kritische Backend-Testlücken (`internal/mediamtx`, `internal/vehicleregistry`, u. a.),
  siehe EPIC in [tasks/backlog.md](backlog.md).
- oder ein anderer Backlog-Punkt nach Nutzerfreigabe.
