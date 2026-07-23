# 🚀 Gemini MCP Server für Claude Code – Handbuch & Dokumentation

Ein hochentwickelter **Model Context Protocol (MCP)** Server, der **Claude Code** (in VS Code oder CLI) nahtlos mit den mächtigen Gemini-Modellen von Google (**Gemini 3.6 Flash**, **Gemini 3.1 Pro Preview**, **Gemini 2.5 Flash**, **Gemini 2.0 Flash**) verbindet.

---

## 📋 Inhaltsverzeichnis

1. [Übersicht & Architektur](#-übersicht--architektur)
2. [Quickstart & Server-Start](#-quickstart--server-start)
3. [Einbindung in VS Code & Claude Code](#-einbindung-in-vs-code--claude-code)
4. [Übersicht aller MCP-Werkzeuge](#-übersicht-aller-mcp-werkzeuge)
5. [🤖 Sub-Agenten & Progress Tracking](#-sub-agenten--progress-tracking)
6. [Detaillierte Werkzeug-Referenz & Beispiel-Prompts](#-detaillierte-werkzeug-referenz--beispiel-prompts)
7. [Modellauswahl & Fallback-Steuerung](#-modellauswahl--fallback-steuerung)
8. [Fehlerbehebung (Troubleshooting)](#-fehlerbehebung-troubleshooting)

---

## 🏗️ Übersicht & Architektur

$$\text{Claude Code (VS Code / CLI)} \;\xleftrightarrow[\text{HTTP SSE / JSON-RPC}]{\text{Port 3000}}\; \text{Gemini MCP Bridge (Node.js/Express)} \;\xleftrightarrow[\text{HTTPS}]{\text{Google GenAI SDK}}\; \text{Google Gemini API}$$

* **Protokoll-Standard**: MCP Spezifikation `2024-11-05` (Server-Sent Events Stream `/mcp/sse` & JSON-RPC `/api/mcp/jsonrpc`).
* **1M+ Kontextfenster**: Auslagerung von riesigen Logfiles, Repositories und Dokumentationen an Gemini.
* **Auto-Fallback-Kette**: Automatische Modell-Umschaltung bei Rate Limits (`429`) oder Verfügbarkeitsproblemen.
* **Asynchrones Sub-Agent Engine**: Hintergrundverarbeitung mit Echtzeit-Fortschrittsüberwachung (`0-100%`) und Schritt-Protokollen.

---

## 🚀 Quickstart & Server-Start

### 1. Abhängigkeiten installieren
```bash
npm install
```

### 2. API-Key konfigurieren (`.env`)
Trage deinen Google Gemini API-Key in der `.env`-Datei im Projektverzeichnis ein:
```env
GEMINI_API_KEY=dein_gemini_api_key_hier
APP_URL=http://127.0.0.1:3000/
```

### 3. MCP Server starten
```bash
npm run dev
```
Der Server startet auf **`http://localhost:3000`**.

---

## ⚙️ Einbindung in VS Code & Claude Code

### Automatisches Laden via `.mcp.json`
Im Projektverzeichnis ist die `.mcp.json` bereits vorkonfiguriert:

```json
{
  "mcpServers": {
    "gemini-mcp": {
      "type": "sse",
      "url": "http://localhost:3000/mcp/sse",
      "env": {
        "GEMINI_API_KEY": "dein_gemini_api_key_hier"
      }
    }
  }
}
```

### Verbindung in Claude Code testen
1. Öffne das **Claude Code Terminal** in VS Code.
2. Gib folgenden Befehl ein:
   ```text
   /mcp
   ```
3. Du siehst `gemini-mcp (sse)` mit grünem Status **connected**.

---

## 🛠️ Übersicht aller MCP-Werkzeuge

| Werkzeug | Standard-Modell | Beschreibung |
| :--- | :--- | :--- |
| `gemini_subagent_start` | `gemini-3.6-flash` | Startet eine asynchrone Sub-Agenten-Aufgabe mit Live-Fortschritt. |
| `gemini_subagent_status` | `gemini-3.6-flash` | Fragt Status (`running`, `completed`, `failed`), % und Log-Historie ab. |
| `gemini_subagent_list` | `gemini-3.6-flash` | Listet alle aktiven und vergangenen Sub-Agenten Tasks auf. |
| `gemini_subagent_cancel` | `gemini-3.6-flash` | Bricht eine laufende Sub-Agenten-Aufgabe geordnet ab. |
| `gemini_simple_code_generator` | `gemini-3.6-flash` | Schnelle Code-Generierung für Utilities, Helpers, Regex, SQL. |
| `gemini_unit_test_generator` | `gemini-3.6-flash` | Erstellt vollständige Test-Suites (Vitest, Jest, PyTest) inkl. Edge Cases. |
| `gemini_quick_bug_fixer` | `gemini-3.6-flash` | Fehlerdiagnose & präziser Minimal-Fix für Stacktraces. |
| `gemini_docstring_comment_generator` | `gemini-3.6-flash` | Generiert JSDoc, Python Docstrings & Typ-Anmerkungen. |
| `gemini_type_converter_formatter` | `gemini-3.6-flash` | Schema-Konvertierung (JSON ➔ TypeScript, SQL ➔ Pydantic). |
| `gemini_analyze_large_context` | `gemini-3.6-flash` | Durchforstet riesige Kontextmengen (1M+ Tokens), Logs & Dokus. |
| `gemini_brainstorm` | `gemini-3.1-pro-preview` | Architektur-Gegenentwürfe, Pros/Cons-Matrizen & Kritik. |
| `gemini_summarize` | `gemini-3.6-flash` | Komprimiert massive Textdaten auf stichpunktartige Essenz. |
| `gemini_code_review_refactor` | `gemini-3.1-pro-preview` | Tiefgehende Multi-File-Codeaudits (OWASP, Concurrency). |
| `gemini_multimodal_inspect` | `gemini-3.6-flash` | Extrahiert Spezifikationen aus UI-Designs & Diagrammen. |
| `gemini_prompt_optimizer` | `gemini-3.6-flash` | Optimiere Roh-Prompts mit XML-Tags für Sub-Agenten. |
| `gemini_cross_repo_search` | `gemini-3.6-flash` | Semantische Suche & Abhängigkeitsanalyse über Dateien. |
| `gemini_diff_explain` | `gemini-3.6-flash` | Analysiert Git Diffs auf Breaking Changes & Regressionsrisiken. |

---

## 🤖 Sub-Agenten & Progress Tracking

Gemini kann als Hintergrund-Sub-Agent eingesetzt werden, um langlaufende Analysen oder Generierungen asynchron durchzuführen.

### Workflow & Interaktion:
1. **Aufgabe starten**: `gemini_subagent_start` liefert sofort eine `task_id` (z. B. `subagent-mrx6ptu1-89pex`).
2. **Fortschritt prüfen**: Mit `gemini_subagent_status` und der `task_id` wird der aktuelle Fortschritt (`0-100%`), der aktuelle Arbeitsschritt und das Schritt-Protokoll abgefragt.
3. **Endergebnis abholen**: Sobald der Status auf `COMPLETED` wechselt, enthält die Status-Antwort das vollständige Endergebnis.

---

## 📖 Detaillierte Werkzeug-Referenz & Beispiel-Prompts

### 1. `gemini_subagent_start`
* **Zweck**: Startet eine Hintergrund-Aufgabe für Gemini.
* **Parameter**:
  * `task_name` (String, erforderlich): Name der Aufgabe.
  * `subagent_role` (String, erforderlich): Rolle (z. B. *"Senior Security Auditor"*).
  * `target_tool` (String, erforderlich): Ziel-Tool (z. B. `gemini_simple_code_generator`).
  * `input_params` (Object, erforderlich): Eingabeparameter für das Ziel-Tool.
  * `model_override` (String, optional).

### 2. `gemini_subagent_status`
* **Zweck**: Fragt Fortschritt, Status und Zeitstempel-Logs ab.
* **Parameter**:
  * `task_id` (String, erforderlich): Die von `gemini_subagent_start` zurückgegebene ID.

### 3. `gemini_simple_code_generator`
* **Zweck**: Erstellt blitzschnell sauberen Code ohne Smalltalk.
* **Parameter**:
  * `task_description` (String, erforderlich): Beschreibung der Aufgabe.
  * `target_language` (String, erforderlich): Zielsprache (z. B. `typescript`, `python`, `sql`).
  * `model_override` (String, optional): Modellwahl.

### 4. `gemini_unit_test_generator`
* **Zweck**: Erzeugt vollständige Unit-Tests mit Mocks, Grenzwerten und Typ-Prüfungen.
* **Parameter**:
  * `source_code` (String, erforderlich): Der zu testende Quellcode.
  * `test_framework` (String, erforderlich): z. B. `vitest`, `jest`, `pytest`.

### 5. `gemini_quick_bug_fixer`
* **Zweck**: Analysiert Fehlermeldungen/Stacktraces und gibt eine 1-Satz Ursache + Minimal-Fix zurück.
* **Parameter**:
  * `error_message_or_stacktrace` (String, erforderlich).
  * `buggy_code_snippet` (String, erforderlich).

### 6. `gemini_analyze_large_context`
* **Zweck**: Durchsucht bis zu 1M+ Tokens (Logdateien, ganze Dokus) nach bestimmten Mustern.
* **Parameter**:
  * `context_data` (String, erforderlich): Der große Text/Loginhalt.
  * `user_query` (String, erforderlich): Die Suchfrage oder Fehlersuche.

### 7. `gemini_brainstorm`
* **Zweck**: Erstellt architektonische Gegenentwürfe und Vor-/Nachteile-Matrizen.
* **Parameter**:
  * `topic_or_problem` (String, erforderlich): Das Architektur-Thema.
  * `current_proposal` (String, optional): Der aktuelle Entwurf zur Kritik.

### 8. `gemini_code_review_refactor`
* **Zweck**: Führt tiefgehende Code-Audits durch (OWASP, Concurrency Bugs, Speicherlecks).
* **Parameter**:
  * `source_code` (String, erforderlich).
  * `language` (String, optional).

---

## 🎛️ Modellauswahl & Fallback-Steuerung

Du kannst bei jedem Werkzeug-Aufruf ein bestimmtes Modell erzwingen, indem du `model_override` angibst:

* `"gemini-3.6-flash"` *(Standard für schnelle Tasks)*
* `"gemini-3.1-pro-preview"` *(Standard für Architektur & Deep Code Audits)*
* `"gemini-2.5-flash"`
* `"gemini-2.0-flash"`

### Automatisches Resilience-Looping
Sollte ein gewähltes Modell ein Rate-Limit (`429 RESOURCE_EXHAUSTED`) oder einen `404`-Fehler melden, schaltet der Server im Hintergrund automatisch auf das nächste verfügbare Modell um.

---

## 🛠️ Fehlerbehebung (Troubleshooting)

| Problem | Ursache | Lösung |
| :--- | :--- | :--- |
| **Port 3000 belegt (`EADDRINUSE`)** | Ein alter Serverprozess läuft bereits auf Port 3000. | Befehl ausführen: `fuser -k 3000/tcp` und danach `npm run dev`. |
| **Rate Limit 429 Error** | Der API-Key hat das freie Limit im Google AI Studio erreicht. | Warte 30 Sekunden oder trage einen Pay-as-you-go Key in der `.env` ein. |
| **Status `disconnected` in `/mcp`** | Server läuft nicht oder URL stimmt nicht. | Prüfe, ob `npm run dev` läuft und die URL `http://localhost:3000/mcp/sse` lautet. |
