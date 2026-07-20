# Sprint 6 — Testing & Quality Gates

Abgeschlossen: 2026-06-04

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | ✅ `tests/docker-compose.test.yml` (control-server, auth-service, safety-service, mosquitto auf Ports 18080–18082); 9 Go Integration Tests (Health, Auth, Session-Lifecycle, Invariante 1, Emergency Stop); `make test-integration` |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | ✅ vitest + @testing-library/react + @playwright/test installiert; `vitest.config.ts` + `setup.ts`; **31/31 Tests grün** (ConnectionPanel 10, SafeModeOverlay 4, SafetyPanel 6, ControlPanel 6, VideoPanel 5); `playwright.config.ts` + `tests/e2e/dashboard.spec.ts` |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | ✅ `tests/performance/latency_test.go` (Go Benchmark, p50=0ms, p95=0ms, p99=0ms @ localhost, Build-Fail bei >100ms); `tests/performance/latency.js` (k6, p99=244µs, 100% checks passed, 5 VU / 10s); `make test-latency` + `make test-k6` |
| DC-04 | Local Dev Environment — README finalisieren | S | ✅ README: alle Makefile-Befehle inkl. test-integration/latency/k6; Troubleshooting (6 Szenarien); Contributor Guide (5 Abschnitte: ADR, Go-Service, Proto, Frontend-Component, Safety) |

### Fixes während Implementierung

1. **`@testing-library/dom`** fehlte als Peer-Dependency → `npm install --save-dev @testing-library/dom` ergänzt.
2. **VideoPanel.test.tsx**: `vi.mocked(require(...))` Pattern funktioniert nicht in ESM-Vitest → auf `mockReturnValueOnce` über captured Mock-Funktion umgestellt.
3. **k6 Inline-Script via stdin**: `k6 run -` erwartet einen Default-Export — Script als Datei mounten statt per heredoc.
4. **docker-compose.test.yml**: `control-server` braucht `depends_on` mit `condition: service_healthy` → `healthcheck` für auth-service und safety-service ergänzt.

### Testprotokoll Integration (2026-06-04)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | Go Build (alle Packages inkl. integration + performance) | `OK` | ✅ |
| T02 | Safety Regression (19/19) | Alle grün | ✅ 19/19 |
| T03 | Vitest Component Tests (5 Files) | 31/31 Tests grün | ✅ 31/31 |
| T04 | Vitest verbose — alle 31 Tests einzeln | Jeder Test ✓ | ✅ alle ✓ |
| T05 | Integration Test Stack startet | 3 Services Built + Started + Healthy | ✅ |
| T06 | Health Checks Test-Stack | HTTP 200 auf :18080/:18081/:18082 | ✅ alle |
| T07 | Go Integration Tests (9 Tests) | 9/9 PASS | ✅ 9/9 in 0.833s |
| T08 | Invariante 1 via Integration Test | MEDIA_FAILED → DEGRADED, kein SAFE_MODE | ✅ |
| T09 | Go Benchmark ACK-Roundtrip | p50=0ms p95=0ms p99=0ms, < 100ms Budget | ✅ p99=0ms (Localhost) |
| T10 | k6 Load Test (5 VU, 10s) | p(99)<100ms threshold ✓, 100% checks | ✅ p99=244µs |
| T11 | Makefile targets (12 Targets) | alle vorhanden | ✅ 12/12 |
| T12 | Playwright config + E2E test | beide Dateien vorhanden | ✅ |
| T13 | README Troubleshooting (6 Sections) | alle Sections vorhanden | ✅ 6/6 |
| T14 | README Contributor Guide (5 Sections) | alle Sections vorhanden | ✅ 5/5 |
| T15 | Test-Stack Teardown | Container + Network removed | ✅ |
| T16 | Neue Test-Dateien (14 Dateien) | alle vorhanden | ✅ 14/14 |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| Vitest Component Tests | 31/31 ✅ | — |
| Go Integration Tests | 9/9 ✅ | — |
| Safety Tests | 19/19 ✅ | 19/19 |
| Go Benchmark p99 (localhost) | 0ms | < 100ms ✅ |
| k6 p99 (localhost, 5 VU) | 244 µs | < 100ms ✅ |
| k6 checks_succeeded | 100% | > 99% ✅ |

### Neue Dateien

- `tests/docker-compose.test.yml` — minimaler Integrations-Test-Stack
- `tests/mosquitto-test.conf` — MQTT-Config für Tests
- `tests/integration/setup_test.go` — Test-Setup (Ports, JWT-Secret)
- `tests/integration/services_test.go` — 9 Integration Tests
- `tests/integration/ws_helper_test.go` — WebSocket-Dial-Helper
- `tests/performance/latency_test.go` — Go Benchmark ACK-Roundtrip
- `tests/performance/latency.js` — k6 Load Test Script
- `tests/e2e/dashboard.spec.ts` — Playwright E2E Baseline (5 Tests)
- `frontend/vitest.config.ts` — Vitest + jsdom + @/ Alias
- `frontend/src/test/setup.ts` — @testing-library/jest-dom Setup
- `frontend/src/components/*.test.tsx` — 5 Component-Test-Files (31 Tests)
- `frontend/playwright.config.ts` — Playwright Config (Chromium + WebRTC-Flags)

### Geänderte Dateien

- `Makefile` — `test-integration`, `test-latency`, `test-k6` Targets ergänzt/korrigiert
- `frontend/package.json` — test/test:watch/test:coverage/test:e2e Scripts + Packages
- `README.md` — vollständige Entwicklungs-Befehle, Troubleshooting, Contributor Guide
