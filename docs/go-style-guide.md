# Go Coding Style Guide

Verbindlich für allen Go-Code in diesem Projekt (`cmd/`, `internal/`, `pkg/`). Ergänzt CLAUDE.MD
Abschnitt 12 ("Code-Qualität") um konkrete, Go-spezifische Schwellenwerte — bei Konflikt gilt
CLAUDE.MD Abschnitt 15 ("Refactoring-Regeln") als härtere Leitplanke: **Verhalten ändern ist bei
jeder Anwendung dieser Regeln auf bestehenden Code verboten**, auch wenn eine Regel hier eine
Umstrukturierung nahelegt.

Rollout-Status und Task-Aufschlüsselung: `tasks/backlog.md`, EPIC "Go Coding Style Guide Rollout".
Bestandsaufnahme, die diesem Dokument zugrunde liegt: siehe Rollout-Task GOSTYLE-00 in
`tasks/backlog.md` bzw. die zugehörige Sitzung vom 2026-07-17.

---

These rules are designed to guide the generation of Go code that is simple, readable, and
maintainable, adhering to Go's idiomatic style and the principles of pragmatic engineering.

## 1. The Principle of Least Abstraction
Your primary goal is clarity, not cleverness. Start with the simplest possible solution.
- **Rule 1.1: Default to a Single Function** - Solve the problem within a single function first.
  Do not create helper functions, new types, or new packages prematurely.
- **Rule 1.2: Justify Every Abstraction** - Before creating a new function, struct, or package,
  you must justify its existence based on the rules below (e.g., function length, parameter
  count, or the Rule of Three). If there's no strong reason to abstract, don't.

## 2. Function Design and Granularity
Functions are the fundamental building blocks. They must be clear and focused.
- **Rule 2.1: Functions Do One Thing** - Every function should have a single, clear
  responsibility. If you cannot describe what a function does in one simple sentence, it's doing
  too much.
- **Rule 2.2: Strict Function Length Limit** - A function should rarely exceed 50 lines. If a
  function grows longer, immediately decompose it into smaller, private helper functions. Keep
  these helpers in the same file to maintain locality.
- **Rule 2.3: Strict Parameter Limit** - A function must not have more than four parameters. If
  you need more, group related parameters into a struct. If a function needs to operate on shared
  state, make it a method on a struct that holds that state.
- **Rule 2.4: Return Values** - Return one or two values directly. If you need to return three or
  more related values, use a named struct. Avoid returning a map or a bare tuple of many values.

## 3. Duplication vs. Abstraction
Avoid hasty abstractions. Duplication is often better than the wrong abstraction.
- **Rule 3.1: The Rule of Three** - Do not refactor duplicated code on its first or second
  appearance. Only on the third instance consider a shared abstraction.
- **Rule 3.2: Verify True Duplication** - Confirm duplicated code represents the same core logic
  before refactoring. Coincidentally similar but logically unrelated code must stay separate.

## 4. Package and Interface Philosophy
Follow Go's idiomatic approach to packages and interfaces.
- **Rule 4.1: Packages Have a Singular Purpose** - No generic "utility," "common," or "helpers"
  packages. Keep related types/functions together in a cohesive package.
- **Rule 4.2: Interfaces are Defined by the Consumer** - The function that uses a dependency
  defines a small interface describing only the behavior it requires, not the producer.
- **Rule 4.3: Keep Interfaces Small** - Ideally one method. More than three methods is a red flag.

---

## Projektspezifische Anmerkungen

**Rule 2.2 in sicherheitskritischem Code (`control-server`):** Reihenfolge-Semantik, die aktuell
nur in Kommentaren dokumentiert ist (z. B. Bootstrap in `cmd/control-server/main.go`), muss bei
der Zerlegung 1:1 erhalten bleiben. Extrahierte Helper dürfen die Aufrufreihenfolge nicht implizit
verändern. Siehe `docs/adr/031-hexagonal-architecture-migration.md` für die Risikoeinschätzung
dieses Service.

**Rule 4.2/4.3 und ADR-031:** Interface-Segregation auf den bestehenden Produktions-Interfaces
(`UserStore`, `VehicleStore`, `AuditWriter`, `SessionRecorder`, `FleetGateway`, `safety.Publisher`)
ist abgeschlossen (GOSTYLE-IF-01..06, Sprint 34/35, `tasks/backlog.md` EPIC "Go Coding Style Guide
Rollout" Phase 2) — koordiniert mit der in ADR-031 beschlossenen Hexagonal-Migration, nicht
unabhängig davon umgesetzt. `SessionRecorder` wurde als tote Abstraktion entfernt; die übrigen
fünf Interfaces sind jetzt konsumentenseitig schlank geschnitten (Bootstrap-/Lifecycle-Methoden wie
`SeedAdmin`/`SeedDefault`/`Close` liegen am konkreten Typ). Für **neuen** Code gelten Rule 4.2/4.3
ohnehin uneingeschränkt.

**Bootstrap-/Lifecycle-Methoden:** Methoden, die nur einmalig beim Start (`SeedAdmin`,
`SeedDefault`) oder beim Shutdown (`Close`) aufgerufen werden, gehören nicht in ein Interface, das
für den laufenden Betrieb konsumiert wird — sie dürfen am konkreten Typ bleiben, auch wenn dadurch
zwei verschiedene "Zugriffspfade" auf denselben Wert existieren (Bootstrap-Code vs. Handler).

**Verhältnis zu Abschnitt 17 (Teststandard):** Eine Funktionszerlegung nach Rule 2.2 ändert nichts
an der Testpflicht aus CLAUDE.MD Abschnitt 17 — extrahierte Helper brauchen keine eigenen Tests,
wenn sie über den bestehenden Test der ursprünglichen Funktion bereits abgedeckt sind (Verhalten,
nicht Struktur, ist das Testobjekt).

**HTTP-Server-Timeouts (SEC-CI-02, Sprint 59):** Kein Service darf `http.ListenAndServe(addr, mux)`
direkt aufrufen (gosec `G114`, seit Sprint 59 ein Required-Check, s. README.md "Required Status
Checks"). Stattdessen explizit `http.Server{Addr:, Handler:, ReadHeaderTimeout:, ReadTimeout:,
WriteTimeout:}` konstruieren und `.ListenAndServe()` darauf aufrufen — Schutz gegen Slowloris-
artige Angriffe (unbegrenzt langsame Clients halten sonst Handler-Goroutinen offen). Einheitliche
Werte in allen sechs Services (`cmd/*/main.go`): `ReadHeaderTimeout: 10 * time.Second`,
`ReadTimeout: 10 * time.Second`, `WriteTimeout: 30 * time.Second`. Diese Timeouts wirken nur auf
die HTTP-Request-Phase — WebSocket-Verbindungen (`control-server`, `fleet-service`,
`vehicle-connection`) übernehmen die Connection per `Hijack()` (intern in `gorilla/websocket`) und
verwalten ihre eigenen Read-Deadlines danach selbst (`conn.SetReadDeadline` im PongHandler), daher
keine Konflikte mit lang laufenden Sessions.
