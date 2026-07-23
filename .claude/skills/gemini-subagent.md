# Gemini Sub-Agent & MCP Execution Skill

This skill guides Claude Code on how and when to offload tasks to the **Gemini MCP Server** (`gemini-mcp`) and orchestrate asynchronous Gemini Sub-Agents with real-time progress tracking.

---

## 🎯 When to Use Gemini MCP & Sub-Agents

1. **Asynchronous Sub-Agent Delegation**:
   - Long-running operations (e.g. multi-file code reviews, deep security audits, test suite generation).
   - Offloading background tasks to Gemini while maintaining progress transparency.
2. **Large Context Processing (1M+ Tokens)**:
   - Offloading massive log files, deep git traces, or repository documentation (`gemini_analyze_large_context`).
3. **Architectural Second Opinion**:
   - Trade-off matrices, pros/cons critiques using Gemini Pro (`gemini_brainstorm`).

---

## 🤖 Sub-Agent Workflow & Instructions

When executing a task via Gemini Sub-Agent, follow this exact workflow:

### Step 1: Start Sub-Agent Task

Call the MCP tool `gemini_subagent_start`:

```json
{
  "task_name": "Multi-File Security Audit",
  "subagent_role": "Senior Security Auditor",
  "target_tool": "gemini_code_review_refactor",
  "input_params": {
    "source_code": "<code_content>",
    "language": "typescript"
  },
  "model_override": "gemini-3.1-pro-preview"
}
```

- Save the returned `task_id` (e.g. `subagent-mrx6ptu1-89pex`).

### Step 2: Progress Tracking & Status Polling

- Call `gemini_subagent_status` with `task_id`.
- Report the status, progress (`0-100%`), and current step to the user.
- If status is `running`, poll `gemini_subagent_status` until `completed` or `failed`.

### Step 3: Present Final Result

- When status is `completed`, render the final output cleanly.
- If status is `failed`, report the logged step-history and error details.

---

## 🛠️ Direct Tool Quick Reference

For immediate synchronous tasks:

- `gemini_simple_code_generator`: Utility functions, regex, SQL.
- `gemini_unit_test_generator`: Vitest/Jest/PyTest suites with edge cases.
- `gemini_quick_bug_fixer`: Stacktrace minimal bug fixes.
- `gemini_analyze_large_context`: 1M+ token context analysis.
- `gemini_brainstorm`: Architectural design critiques (`gemini-3.1-pro-preview`).
- `gemini_code_review_refactor`: OWASP & concurrency code audits (`gemini-3.1-pro-preview`).

_For full details, see [`GEMINI_MCP_GUIDE.md`](../../GEMINI_MCP_GUIDE.md)._
