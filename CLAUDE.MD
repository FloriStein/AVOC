# CLAUDE.md

## 0. Executive Contract (Kurzregelwerk)

Dein Ziel ist nicht Code zu schreiben, sondern:

- Probleme korrekt zu verstehen
- Architektur sauber zu entwerfen
- Entscheidungen nachvollziehbar zu dokumentieren
- langfristig wartbare Systeme zu bauen

Du darfst niemals implementieren ohne:

- Anforderungen verstanden
- Architekturkontext geprüft
- Risiken bewertet

Wenn Informationen fehlen:
→ immer Grill-Me-Session starten (außer bei Fast-Track)

---

Wenn mehrere Regeln gleichzeitig zutreffen:

1. Sicherheit schlägt alles
2. Anforderungen schlagen Architektur
3. ADRs schlagen Implementierungsfreiheit
4. Architektur schlägt Code-Optimierung
5. Klarheit schlägt Performance-Optimierung
6. Explizite Anweisung schlägt implizite Regel

Bei Unsicherheit immer:
→ nachfragen statt interpretieren

---

## 1. Arbeitsphasen (verpflichtend)

Jede Aufgabe durchläuft standardmäßig:

1. Verstehen
2. Grill-Me-Session (falls nötig)
3. Anforderungen strukturieren
4. Architektur entwerfen
5. ADRs erstellen / prüfen
6. Tasks ableiten
7. Implementierung
8. Tests
9. Dokumentation

Keine Phase darf übersprungen werden ohne Begründung (oder durch Fast-Track-Regel).

### 1.1 Task-Klassifizierung & Fast-Track

Aufgaben werden vorab nach Komplexität bewertet, um unnötigen Overhead zu vermeiden:

- **Typ S (Micro-Task / <30 Min):** Reine Kosmetik, Typo-Fixes, eindeutige Bugfixes ohne Architektureinfluss.
  → _Fast-Track:_ Phasen 2–6 werden übersprungen. Direkt von Verstehen zu Implementierung & Test.
- **Typ M (Standard-Task / 30–180 Min):** Reguläre Feature-Entwicklung, Refactoring, Standard-Komponenten.
  → _Normal-Track:_ Alle 9 Phasen verpflichtend. Grill-Me-Session nur bei Unklarheit.
- **Typ L (Architektur-Task):** Betrifft Kernsysteme, neue Technologien, bricht bestehende Regeln oder verändert Datenstrukturen grundlegend.
  → _Deep-Track:_ Erfordert zwingend eine Grill-Me-Session und ein neues ADR vor der Umsetzung.

---

## 2. Projektwissen (Lazy-Loading System)

Um das Kontextfenster (Tokens) zu schonen und Informationsüberlastung der KI zu vermeiden, gilt folgendes Ladeprinzip:

**Core-Load (Zu Beginn JEDER Session einlesen):**

1. `tasks/current-sprint.md` (Aktueller Arbeitsstand)
2. `CONTEXT.md` (Nur Glossar & aktuelle offene Fragen/Risiken)

**Lazy-Load (Bedarfsbasiert vor Phase 4/5 nachladen):**

- `docs/requirements.md` & `docs/vision.md` → Nur bei Typ M/L Tasks zur detaillierten Anforderungsprüfung.
- `docs/architecture.md` & `docs/adr/*` & `DECISIONS.md` → Nur, wenn der Task die betroffene Komponente oder die Infrastruktur berührt. `DECISIONS.md` ist die Live-Übersicht für ADR-Kernentscheidungen und offene Folge-Entscheidungen.
- `docs/go-style-guide.md` → wird automatisch über `.claude/rules/go-style.md` geladen, sobald eine Go-Datei unter `cmd/`, `internal/` oder `pkg/` gelesen wird — kein manuelles Nachladen nötig. Verbindlich (§12), nicht optional; hier dokumentiert für Menschen, die diese Datei lesen.
- `tasks/backlog.md` & `tasks/done.md` → Nur bei Sprint-Planung oder wenn frühere Sprints als Referenz gebraucht werden. `done.md` ist nur ein Index (Titel + Highlights + Link) — für Details eines konkreten abgeschlossenen Sprints gezielt die zugehörige `tasks/sprints/NN-slug.md` nachladen, nicht den ganzen Ordner.

_Regel:_ Die KI muss im Chat explizit deklarieren, wenn sie ein Dokument nachlädt (z. B. _„[Lazy-Load] Lade ADR-004 für DB-Design...“_).

**Verfügbare MCP-Server (2026-07-23, siehe README.md "Claude Code MCP-Server" für Details/Setup):**
`.mcp.json` stellt `chrome-devtools`, `firefox-devtools`, `playwright`, `github`, `postgres`,
`grafana` sowie den **`gemini-mcp`** Server (Gemini Modelle & Sub-Agenten) bereit — sofern die
Session danach neu gestartet wurde und (bei `github`/`postgres`/`grafana`) die nötigen Env-Vars
gesetzt sind. Für `gemini-mcp` siehe [`.claude/skills/gemini-subagent.md`](.claude/skills/gemini-subagent.md)
(Sub-Agenten-Workflow) bzw. [`GEMINI_MCP_GUIDE.md`](GEMINI_MCP_GUIDE.md) (volle Werkzeug-Referenz).
Bevor auf einen Workaround ausgewichen wird (curl gegen die GitHub-API ohne Auth, Selektoren blind
aus React-Quellcode ableiten, Ad-hoc-DB-Skripte), prüfen ob das passende MCP-Tool verfügbar ist
und es stattdessen nutzen — Beispiel aus der Praxis: ohne `github`-MCP endeten Actions-Log-/
Artefakt-Abrufe wiederholt an 403/401, PRs mussten vom Nutzer manuell geöffnet werden. Ist ein
MCP-Tool nicht verfügbar (z. B. fehlendes Token), das kurz benennen statt stillschweigend auf den
alten Workaround zurückzufallen.

---

## 3. Regel-Hierarchie (wichtig für Konflikte)

Dokumentenspezifische Ausprägung der Prioritäten aus §0 (dort: allgemeine Prinzipien; hier:
konkret welches Dokument bei einem Widerspruch gewinnt). Wenn Konflikte auftreten, gilt:

1. vision.md
2. requirements.md
3. aktive ADRs
4. architecture.md
5. CONTEXT.md
6. Code

Bei Konflikten:

- explizit benennen
- Ursache erklären
- Lösungsvorschlag machen
- niemals stillschweigend überschreiben

---

## 4. Entscheidungsregel (Anti-Halluzination)

Wenn Informationen fehlen:
→ nicht raten
→ immer fragen

Wenn mehrere Optionen existieren:
→ Alternativen zeigen
→ Vor- / Nachteile erklären
→ klare Empfehlung geben

---

## 5. Grill-Me-Session (Pflicht bei Unklarheit & Typ L)

Bei neuen oder unklaren Anforderungen:

- Annahmen hinterfragen
- Risiken identifizieren
- Zielkonflikte erklären
- max. 5 Fragen gleichzeitig stellen
- Technologieentscheidungen challengen

Ziel: Problem verstehen, nicht sofort lösen.

---

## 6. Architekturentscheidungen (ADRs)

ADRs sind Pflicht bei (Typ L):

- Cloud / Infrastruktur
- Datenbanken
- API-Design
- Authentifizierung
- Eventing / Messaging
- Frontend-Frameworks
- Deployment
- Security-Modelle

Regeln:

- niemals überschreiben
- neue Erkenntnisse → neuer ADR
- immer Konsequenzen dokumentieren
- ADR-Index sofort im selben Task nachziehen: `DECISIONS.md` + `docs/adr/README.md` (§19)

---

## 7. Dokumentensystem

### vision.md (Warum?)

- Ziel, Vision, Business Value
- Zielgruppen
- Erfolgskriterien
- Nicht-Ziele

### requirements.md (Was?)

- funktionale & nichtfunktionale Anforderungen
- User Stories
- Akzeptanzkriterien
  → Erste Sammelstelle für neue Anforderungen

### architecture.md (Wie?)

- Systemdesign
- Komponenten
- Schnittstellen
- Infrastruktur
  → Muss konsistent mit ADRs sein

---

## 8. CONTEXT.md (Domänenwissen)

Enthält:

- Glossar
- Domänenmodell
- Annahmen
- Risiken
- offene Fragen
- Architekturprinzipien

Regel: Begriffe müssen konsistent gepflegt werden.

---

## 9. DECISIONS.md (Live-Übersicht)

Enthält:

- aktuelle Entscheidungen
- Referenzen auf ADRs
- offene Folge-Entscheidungen (einzige Quelle dafür — nicht in `docs/adr/README.md` duplizieren)

Regel: Nach jedem ADR aktualisieren; erledigte Folge-Entscheidungen im selben Task abhaken (§19).

---

## 10. Task-System (operatives System)

### backlog.md

- alle neuen / offenen Aufgaben
- inkl. Tech Debt & Research

### current-sprint.md

- **nur der aktuell aktive Sprint** (max. 3–10 Tasks), klar umsetzbar, klein geschnitten
- bei Sprint-Abschluss wandert der Inhalt unverändert nach `tasks/sprints/NN-slug.md`, diese Datei
  wird danach für den nächsten Sprint geleert

### tasks/sprints/NN-slug.md

- Volltext-Archiv aller abgeschlossenen Sprints, ein File pro Sprint (Namensschema analog zu
  `docs/adr/NNN-slug.md`)
- wird nie mehr verändert, sobald angelegt (historisches Protokoll, wie ADRs)

### done.md

- kompakter chronologischer Index aller abgeschlossenen Sprints — Titel + 2–4 Ergebnis-Highlights
  + Link auf `tasks/sprints/NN-slug.md`, kein Volltext

### Task-Regeln

- jeder Task muss im System sichtbar sein
- keine direkte Implementierung ohne Task
- Tasks müssen 30–180 Minuten groß sein (Typ M)
- große Tasks → zerlegen in kleinere Einheiten (Typ M/S)

Lifecycle: backlog → current-sprint.md → tasks/sprints/NN-slug.md (Volltext) + done.md (Index-Eintrag)

---

## 11. Definition of Done (Abgestuft)

Eine Aufgabe ist nur erledigt, wenn die Kriterien ihrer Komplexitätsklasse erfüllt sind:

| Kriterium                                         | Typ S |      Typ M       | Typ L |
| :------------------------------------------------ | :---: | :--------------: | :---: |
| Code erfüllt Anforderungen & Tests bestehen       |   X   |        X         |   X   |
| Teststandard erfüllt (Abschnitt 17)               |   –   |        X         |   X   |
| `done.md` aktualisiert & Sprint-Task geschlossen  |   X   |        X         |   X   |
| `requirements.md` / `architecture.md` nachgezogen |   –   |        X         |   X   |
| `CONTEXT.md` / `DECISIONS.md` aktualisiert        |   –   | (nur bei Bedarf) |   X   |
| ADR erstellt und im System verlinkt               |   –   |        –         |   X   |

Bei Go-Code-Änderungen (`cmd/`, `internal/`, `pkg/`) zusätzlich, unabhängig von der Typklasse:
`docs/go-style-guide.md` eingehalten — zählt als Teil von "Code erfüllt Anforderungen".

---

## 12. Code-Qualität

Bevorzuge:

- KISS (Keep It Simple, Stupid)
- klare Strukturen
- geringe Komplexität
- Wartbarkeit
- explizite Abhängigkeiten

Vermeide:

- Overengineering
- versteckte Logik
- unnötige Abstraktion
- Magic Values

Für Go-Code (`cmd/`, `internal/`, `pkg/`) gilt zusätzlich verbindlich `docs/go-style-guide.md` —
konkrete, messbare Schwellenwerte zu Funktionslänge, Parameterzahl, Duplikat-Schwelle und
Interface-Design, die diese Prinzipien für Go konkretisieren. Wird pfadgebunden automatisch
geladen, siehe §2.

---

## 13. Security & Risiko

Immer prüfen:

- Authentifizierung
- Autorisierung
- Secrets Handling
- Input Validation
- Logging sensibler Daten

Risiken aktiv benennen.

---

## 14. Kostenbewusstsein

Bei Cloud-Systemen immer berücksichtigen:

- Betriebskosten
- Skalierungskosten
- Vendor Lock-in
- Wartungsaufwand

---

## 15. Refactoring-Regeln

Erlaubt:

- Lesbarkeit verbessern
- Struktur verbessern
- Komplexität reduzieren

Verboten:

- Verhalten ändern
- Anforderungen verletzen
- ADRs ignorieren

---

## 16. Kommunikationsstil

Ergänzt §4 (Entscheidungsregel) um den Ton der Kommunikation selbst:

- Entscheidungen und Empfehlungen immer begründen, nicht nur behaupten
- Unsicherheiten und Annahmen explizit markieren, nie stillschweigend voraussetzen (§4: nicht raten, nachfragen)

---

## 17. Teststandard (Sicherheit ohne Codereview)

Grundsatz: Code muss auch für jemanden, der die Implementierung nicht liest, allein durch Tests
und deren Dokumentation als sicher nachvollziehbar sein. Ergänzt Phase 8 ("Tests") und Abschnitt
11 ("Definition of Done"). Konkretisiert für dieses Projekt zusätzlich ADR-006 (Testing Strategy).

Gilt verpflichtend für Typ M und Typ L (siehe 1.1). Bei Typ S (reine Kosmetik/Typo-Fixes ohne
Architektureinfluss) entfällt die volle Checkliste — Fast-Track bleibt in Kraft.

**Pflicht-Fallgruppen pro Logikeinheit** (nicht nur Happy Path):

- Grenzwerte: 0, negative Zahlen, leere Strings/Listen, `nil`/`null`, Maximalwerte
- Fehlerhafte Eingaben: falsches Format, falscher Typ, zu lang, unerwartetes Encoding
- Fehlerpfade: DB-/Netzwerk-/Auth-Fehler explizit testen, nicht nur den Erfolgsfall
- Wiederholung/Idempotenz: gleiche Aktion zweimal ausführen — was ändert sich (oder nicht)?
- Zustandsübergänge, falls Code Zustand über Zeit hält (Reihenfolge-Abhängigkeit,
  Schwellwert-Flattern, Races zwischen Zuständen)
- Nebenläufigkeit, falls geteilter State existiert (Race-Detector, mehrere gleichzeitige
  Aufrufer)
- Zugriffsgrenzen, falls zutreffend: kein Token, ungültiges Token, falsche Berechtigung

**Integrationstests gegen echte Abhängigkeiten**, nicht nur In-Memory-Mocks (ADR-006 Teil 2) —
Mocks verstecken reale Fehlerklassen (Startreihenfolge, Serialisierung, Netzwerk-Timing). Bei
mehreren zusammenspielenden Services: mindestens ein Test über die echte Prozessgrenze, nicht nur
isolierte Einzelteile.

**Flakiness ausschließen:** sicherheitsrelevante oder nebenläufige Tests mindestens zweimal
hintereinander gegen frischen Zustand laufen lassen. Ein einmaliger Durchlauf gilt nicht als
stabil.

**Ergebnis dokumentieren, nicht behaupten:** am Ende jeder Implementierung eine konkrete Liste,
was getestet wurde (welche Fälle) und was bewusst nicht abgedeckt ist und warum. Keine pauschale
Aussage wie "vollständig getestet".

**Bei Unsicherheit über einen Edge Case:** zusätzlichen Test schreiben oder nachfragen — Lücke
nicht stillschweigend offen lassen.

---

## 18. Ziel des Systems

Ergebnis der in §0 beschriebenen Arbeitsweise: ein Projekt, das langfristig verständlich bleibt,
vollständig dokumentiert und ohne implizites Wissen rekonstruierbar ist — mit nachvollziehbaren
Architekturentscheidungen und wartbar für jeden neuen Entwickler.

---

## 19. MD-Doku-Pflege (Drift-Vermeidung)

Ergänzt Phase 9 ("Dokumentation") und Abschnitt 11 (Definition of Done). Anlass: Sprint-35-
Nacharbeit (2026-07-19) musste einen eigenen Audit-Sprint aufwenden, um über ein Dutzend
Dokumente wieder auf den tatsächlichen Code-/Sprint-Stand zu bringen — Ziel dieses Abschnitts ist,
dass das nicht mehr als separater Sprint nötig ist, sondern laufend nebenbei passiert.

**Offene Fragen im selben Task schließen, nicht erst beim nächsten Audit:** Löst ein Task eine in
`CONTEXT.md` ("Offene Fragen") oder `DECISIONS.md` ("Offene Folge-Entscheidungen") gelistete Frage
— auch nur nebenbei —, wird der Eintrag im selben Task durchgestrichen/auf ✅ gesetzt und mit
Referenz (Sprint/Commit) versehen. Nicht warten, bis jemand explizit "räum die Docs auf" sagt.

**Keine fragilen Zahlen in lebenden Dokumenten:** Testanzahlen, Endpoint-Zahlen u. ä. veralten
schneller, als jemand sie nachzieht (Beispiel: `README.md` nannte vor der Sprint-35-Aufräumung
eine Testanzahl, die seit vielen Sprints falsch war). Wo eine Zahl nicht zentral fürs Verständnis
ist: qualitativ beschreiben oder auf die Quelle verweisen (z. B. „siehe `tests/unit/safety_test.go`")
statt hart kodieren.

**Cross-Referenzen bei Struktur-Änderungen mitziehen:** Wird eine Datei umbenannt, verschoben,
gelöscht oder ihre Rolle geändert (z. B. Interface verschlankt, Service-Verzeichnis umbenannt),
im selben Task per `grep` nach Verweisen darauf im gesamten Repo suchen (nicht nur `docs/`) und
mitziehen — analog zum Vorgehen bei der `Doku.md`-Ablösung und der `tasks/sprints/`-Aufteilung.

**Periodischer Leicht-Audit:** etwa alle 8–10 Sprints oder beim Start eines neuen EPICs/
Meilensteins einen kleinen eigenen Task (Typ S/M) einplanen, der die lebenden Dokumente
(`README.md`, `CONTEXT.md`, `DECISIONS.md`, `docs/architecture.md`, `docs/requirements.md`,
`docs/adr/README.md`) gegen den tatsächlichen Code-/Sprint-Stand gegenprüft — nicht nur die
Dokumente, die der zuletzt bearbeitete Code direkt betrifft. Ergebnis wie jeder Sprint in
`tasks/sprints/NN-slug.md` dokumentieren.
