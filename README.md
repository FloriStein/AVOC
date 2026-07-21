# AVOC — Autonomous Vehicle Operational Control Center

Sicheres, modulares Echtzeit-Teleoperation-System zur Fernsteuerung von Fahrzeugen über das offene Internet (Vehicle ↔ Internet ↔ OCC).

→ Vollständige Projektdokumentation: [docs/](docs/) | ADRs: [docs/adr/](docs/adr/) | Vision: [docs/vision.md](docs/vision.md)

---

## Schnellstart

**Voraussetzungen:** Docker, Docker Compose

```bash
# Umgebungsvariablen einrichten
cp .env.example .env
# JWT_SECRET in .env auf einen sicheren Wert setzen

# Alle Services starten (Build inklusive)
make up
# oder direkt:
docker compose -f infrastructure/compose/docker-compose.yml --env-file .env up --build
```

**Services nach Start:**

| URL | Service |
|-----|---------|
| http://localhost:3000 | Frontend (React Dashboard) |
| http://localhost:8080 | Control Server |
| http://localhost:8081 | Auth Service |
| http://localhost:8082 | Safety Service |
| http://localhost:8083 | Telemetry Service |
| http://localhost:8084 | WebRTC SFU (passiv, Session-Events) |
| http://localhost:8085 | Fleet Service (Zonen/Stationen/Tasks/Alerts — IBATOUR) |
| http://localhost:8889 | MediaMTX WHIP/WHEP (Fahrzeug → Browser) |
| http://localhost:3001 | Grafana (Log-Dashboard) |
| http://localhost:3100 | Loki (Log-Aggregation API) |

---

## Architektur

Zwei orthogonale Hubs, vier Kommunikationskanäle:

```
CONTROL HUB (Rang 1 — Safety Truth)     VIDEO HUB (Rang 2 — Awareness only)
Control Server (Go)                      MediaMTX (WHIP/WHEP Router — ADR-020)
  · 4-Layer State Machine                  · WHIP Ingestion (Fahrzeug-Kamera)
  · Safety Decision Engine                 · WHEP Distribution (Operator Browser)
  · Session Manager (GSA)                  · Auth-Hook → Control Server
  · Failure Detection                      · SAFE_MODE-Kick via Management API
  · Operator Handover
  · MediaMTX Auth + SAFE_MODE-Kontrolle

WebRTC SFU (Pion/Go) — passiv: nur Session-Event-Subscriber, kein Media-Routing
```

**4-Layer State Machine:** SYSTEM STATE (Master) · CONTROL STATE · MEDIA STATE · OPERATOR STATE

**Kanäle:** WebSocket (Control) · MQTT (Telemetry) · Safety Event Bus (Go In-Memory) · WHIP/WHEP via MediaMTX (Video)

→ Details: [docs/architecture.md](docs/architecture.md)

---

## Entwicklung

```bash
# Proto-Code generieren (Go + TypeScript)
make proto-gen          # Go (via Docker)
make proto-gen-ts       # TypeScript (via Docker) — einmalig vor npm run dev erforderlich

# Alle Go-Services bauen
make build

# Tests
make test               # alle Go-Tests
make test-safety        # Safety Test Suite (CI Safety Gate — muss grün bleiben, tests/unit/safety_test.go)
make test-integration   # Integration Tests (startet/stoppt Test-Stack automatisch)
make test-latency       # Go Benchmark ACK-Roundtrip <100ms (ADR-010 Build-Fail)
make test-k6            # k6 Load Test 10 VU / 30s (benötigt Docker)

# Frontend Tests
cd frontend && npm test           # Vitest Component-/Hook-Tests
cd frontend && npm run test:e2e   # Playwright E2E (benötigt laufenden Stack)

# Stack stoppen
make down
```

**Lokales Frontend mit Hot-Reload:**
```bash
# 1. Proto-Dateien generieren (einmalig nach git clone oder proto/-Änderungen)
make proto-gen-ts

# 2. Dependencies installieren
cd frontend && npm install

# 3. Dev-Server starten (benötigt laufenden Backend-Stack via `make up`)
make dev-frontend
# oder direkt: cd frontend && npm run dev
# → http://localhost:5173
```

---

## Claude Code MCP-Server

`.mcp.json` (Repo-Root) konfiguriert 6 MCP-Server für Claude-Code-Sessions/-Agenten in diesem
Projekt. Nach Änderungen an `.mcp.json` braucht es einen Neustart der Claude-Code-Session, damit
die Server geladen werden.

| Server | Zweck | Setup nötig |
|---|---|---|
| `chrome-devtools` / `firefox-devtools` | Browser interaktiv steuern (Navigation, Klicks, Snapshots, Netzwerk/Konsole) | Keins — nutzt System-Chromium (`/snap/bin/chromium`) headless |
| `playwright` | Wie oben, aber mit Playwright-Semantik — passend zu `frontend/tests/e2e/` (gleiche Locator-API wie im echten Testcode) | Keins — nutzt ebenfalls System-Chromium, umgeht so den bekannten `npx playwright install`-Blocker auf nicht unterstützten Host-OS-Versionen |
| `github` | Direkter GitHub-Zugriff (PRs, Issues, Actions-Runs inkl. Logs) — ohne `gh`-CLI/Token stößt man sonst auf 403/401 bei Actions-Logs/Artefakten | `export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...` (Fine-grained Token, Scope `repo`+`workflow` für `FloriStein/AVOC`) |
| `postgres` | Read/Write-Zugriff auf die lokale Dev-DB (`avoc`) für Debugging, ohne Ad-hoc-Skripte | `export POSTGRES_CONNECTION_STRING="postgresql://avoc:avoc_dev_secret@localhost:5432/avoc"` — braucht laufenden Dev-Stack (`postgres`-Service published seit 2026-07-21 Port `5432:5432` an den Host) |
| `grafana` | Grafana/Loki-Abfragen (Logs, Dashboards) gegen den lokalen Dev-Stack | Keins zwingend — Dev-Grafana läuft mit `GF_AUTH_ANONYMOUS_ENABLED=true` (Admin-Rolle) auf Port `3001`; `export GRAFANA_SERVICE_ACCOUNT_TOKEN=""` sicherheitshalber setzen, falls die Platzhalter-Expansion sonst leerläuft |

`github`/`postgres`/`grafana` lesen Secrets ausschließlich über `${VAR}`-Platzhalter aus der
Shell-Umgebung — nie im Klartext in `.mcp.json` (die Datei ist committed).

---

## Troubleshooting

### 502 Bad Gateway nach Container-Rebuild
nginx cached Docker-IPs beim Start. Fix:
```bash
docker exec avoc-frontend-1 nginx -s reload
```
Ursache: `set $upstream` + `rewrite...break` — `set` muss vor `rewrite` stehen (nginx Rewrite-Modul).

### `npm run dev` schlägt fehl: `Cannot find @/gen/control_pb.js`
Proto-Dateien fehlen lokal. Fix (einmalig, aus Repo-Root):
```bash
make proto-gen-ts
```
Hintergrund: `frontend/src/gen/` ist gitignored — wird nur im Docker-Build und via `make proto-gen-ts` generiert.

### `npm run dev` schlägt fehl: `Cannot find @rollup/rollup-linux-x64-gnu`
`node_modules` wurde in Docker (Alpine/musl) als root installiert. Fix:
```bash
# Root-owned node_modules via Docker löschen
docker run --rm \
  -v $(PWD)/frontend:/app -w /app node:22-alpine \
  sh -c 'rm -rf node_modules package-lock.json'
# Neu installieren auf Host-Platform
cd frontend && npm install
```

### Port-Konflikte beim Stack-Start
```bash
lsof -i :3000   # Frontend (nginx)
lsof -i :8080   # Control Server
lsof -i :8081   # Auth Service
lsof -i :8084   # WebRTC SFU
```
Test-Stack läuft auf Ports 18080–18082 (kein Konflikt mit Dev-Stack).

### WHIP/WHEP (`/whip/`, `/whep/`) liefert 502 Bad Gateway
`mediamtx` läuft im Dev-Stack mit `network_mode: host` (ICE-Stabilität) und hat daher
keinen Docker-DNS-Eintrag auf `avoc-net`. `nginx.dev.conf` routet `/whip/` und `/whep/`
seit Sprint 19 über `host.docker.internal` (mit `extra_hosts` im `frontend`-Service) —
bewusst mit statischem `proxy_pass` statt der `resolver`-Variablen-Technik der anderen
Locations, da diese nur Docker-DNS (127.0.0.11) fragt und `host.docker.internal` (ein
reiner `/etc/hosts`-Eintrag) nicht kennt.

### `make up --build` / `docker buildx build` schlägt mit DNS-Fehlern fehl
```
failed to resolve source metadata for docker.io/library/...: dial tcp: lookup registry-1.docker.io ...: i/o timeout
```
Der `buildx`-Builder-Container merkt sich `/etc/resolv.conf` beim Start und aktualisiert
es nicht automatisch bei Netzwerkwechseln (WLAN-Wechsel, VPN an/aus). Nach langer
Laufzeit des Builders kann das auf eine nicht mehr erreichbare DNS-IP zeigen. Fix:
```bash
docker restart buildx_buildkit_<projektname>-builder0
```

### WSL2: Services nicht erreichbar über `localhost`
WSL2 hat eine eigene IP-Adresse. `.env` und `frontend/vite.config.ts` ggf. anpassen:
```bash
# WSL2-IP ermitteln:
hostname -I | awk '{print $1}'
```

### `make test-integration` schlägt fehl: Services nicht erreichbar
Test-Stack braucht ggf. mehr Zeit. Timeout erhöhen oder manuell starten:
```bash
docker compose -f tests/docker-compose.test.yml up --build -d
sleep 10
go test ./tests/integration/... -v -timeout 120s
docker compose -f tests/docker-compose.test.yml down
```

---

## Projektstruktur

```
├── cmd/                    # Go Service Entry Points (control-server, auth-service, safety-service,
│                           #   telemetry-service, webrtc-sfu, fleet-service, vehicle-mock)
├── internal/               # Go Service-interne Pakete
│   ├── authservice/
│   ├── controlserver/
│   │   ├── command/        # Command Engine — Protobuf Parsing, Rate Limiting
│   │   ├── safety/         # Safety Decision Module (DeadmanWatchdog, ACKTimeout)
│   │   ├── session/        # Session Manager (GSA), Handover
│   │   ├── statemachine/   # 4-Layer State Machine
│   │   ├── transport/      # WebSocket Transport Layer
│   │   └── vehiclecontext/ # Registry — pro Fahrzeug eigene State Machine + Watchdogs (ADR-026)
│   ├── safetyservice/      # Safety Event Bus (In-Memory)
│   ├── recording/          # Session Recording (MemoryRecorder, ADR-005)
│   ├── vehicleconnection/  # Vehicle WebSocket Handler
│   ├── vehicleregistry/    # Vehicle Registry (ADR-022/023) — PostgresVehicleStore, VehicleStore Interface
│   ├── fleetservice/       # Fleet REST-Handler, Store, Broadcast-Hub, Alert-Engine (ADR-029/031)
│   └── fleetgateway/       # FleetGateway-Interface + Mock-/MQTT-Implementierung (ADR-027)
├── pkg/                    # Shared Go-Pakete: ulid (ADR-016), logger (ADR-017), audit (ADR-018),
│                           #   db, env
├── proto/                  # .proto Source — Single Source of Truth
├── gen/                    # Generated Code — gitignored
├── frontend/               # React 18 + TypeScript + Vite + Tailwind
├── infrastructure/
│   ├── compose/            # docker-compose.yml + docker-compose.prod.yml
│   ├── docker/             # Dockerfiles, nginx.conf
│   ├── coturn/             # STUN/TURN Konfiguration
│   ├── mediamtx/           # MediaMTX WHIP/WHEP Config (ADR-020)
│   ├── mosquitto/          # MQTT Broker Konfiguration
│   ├── grafana/ loki/ promtail/  # Log-Aggregation & -Visualisierung (ADR-017)
│   └── AWS/                # CDK Stack (EC2, Security Groups)
└── tests/unit/             # Safety Test Suite (siehe tests/unit/safety_test.go)
```

---

## Implementierungsstand & Sprint-Stand

Der jeweils aktuelle Stand wird ausschließlich in den lebenden Task-/Entscheidungs-Dokumenten gepflegt (nicht hier, damit nichts mehr veraltet):

- **Aktiver Sprint:** [tasks/current-sprint.md](tasks/current-sprint.md)
- **Abgeschlossene Sprints (vollständige Historie):** [tasks/done.md](tasks/done.md)
- **Offener Backlog:** [tasks/backlog.md](tasks/backlog.md)

---

## ADR-Übersicht

Alle Architekturentscheidungen sind dokumentiert und unveränderlich. Neue Erkenntnisse führen zu einem neuen ADR.

→ Vollständiger, aktuell gepflegter Index: [docs/adr/README.md](docs/adr/README.md) | Live-Übersicht: [DECISIONS.MD](DECISIONS.MD)

---

## Contributor Guide

### Neues ADR erstellen
1. Kopiere [docs/adr/000-template.md](docs/adr/000-template.md) → `docs/adr/0XX-titel.md`
2. Fülle alle Pflichtfelder aus (Kontext, Optionen, Entscheidung, Konsequenzen)
3. Trage ADR in [DECISIONS.MD](DECISIONS.MD) und [docs/adr/README.md](docs/adr/README.md) ein

### Neuen Go-Service hinzufügen
1. Erstelle `cmd/<service-name>/main.go` mit `/health` Endpoint
2. Nutze `infrastructure/docker/go-service.Dockerfile` (wiederverwendbar via `SERVICE_NAME` ARG)
3. Ergänze Service in `infrastructure/compose/docker-compose.yml` + `tests/docker-compose.test.yml`
4. Füge `pkg/logger.New("<service-name>")` für strukturiertes Logging ein (Phase 7)

### Proto-Schema ändern
Field-based Versioning (ADR-012): **keine Field-IDs ändern**, keine Felder entfernen.
```bash
# 1. proto/*.proto ändern
# 2. Code generieren:
make proto-gen       # Go → gen/go/
make proto-gen-ts    # TypeScript → frontend/src/gen/
# gen/ ist gitignored — nie committen
```

### Neuen Frontend-Component erstellen
1. `frontend/src/components/<Name>.tsx`
2. Hooks in `frontend/src/hooks/use<Name>.ts`
3. Test: `frontend/src/components/<Name>.test.tsx` — Vitest + RTL
4. Mock externe Dependencies: `vi.mock('@/hooks/use...')`

### Safety-kritischen Code ändern
- Safety Tests (19/19) müssen grün bleiben: `make test-safety`
- Änderungen an `detector.go`, `statemachine.go`, `websocket.go` erfordern Test-Update
- SAFE_MODE-Transitionen: erst `AuditWriter.WriteSync()` (Phase 7 — ADR-018), dann Transition

### CI-Pipeline & Branch-Protection (Sprint 41, ADR-006-Bestandsaufnahme)

`.github/workflows/` enthält 5 Dateien, 4 davon Merge-Gates (siehe ADR-006 + CLAUDE.MD §17):

| Datei | Jobs | Blocking? |
|-------|------|-----------|
| `lint.yml` | `golangci-lint` | Nein (`continue-on-error`, Rollout noch nicht abgeschlossen) |
| `test-go.yml` | `unit` (`make test-unit`), `safety` (`make test-safety`), `integration` (`make test-integration`) | **Ja, alle 3** |
| `test-frontend.yml` | `vitest` (`npm run test`) | **Ja** |
| `test-latency.yml` | `go-benchmark` (`make test-latency`), `k6` (`make test-k6`) | Nein — bewusste Abweichung von ADR-006 ("BLOCKING"), siehe Begründung dort und in `tasks/sprints/41-ci-gates-einfuehren.md` (Shared-Runner-Rauschen) |
| `test-e2e.yml` | `playwright` (`npm run test:e2e`) | Nein (ADR-006 WebRTC/E2E Non-Determinism Policy) |

**Required Status Checks — vorbereitet, NICHT aktiviert:** Branch-Protection ist eine geteilte
Repo-Einstellung (betrifft alle künftigen PRs) und wurde in Sprint 41 bewusst nicht scharf
geschaltet — das erfordert explizite Nutzerbestätigung (MB-Regeln zu risikoreichen/schwer
umkehrbaren Aktionen). Bei Aktivierung sind genau diese 4 Checks als "Required" einzutragen
(GitHub → Settings → Branches → Branch protection rule für `main` → "Require status checks to
pass before merging"):

```
Unit Tests            (test-go.yml       / job: unit)
Safety Test Suite     (test-go.yml       / job: safety)
Integration Tests (Docker) (test-go.yml  / job: integration)
Vitest Unit Tests      (test-frontend.yml / job: vitest)
```

`golangci-lint`, `go-benchmark`, `k6` und `playwright` bleiben absichtlich außen vor (non-blocking
per Design, siehe Tabelle oben). Äquivalenter `gh`-Befehl (zur Referenz, nicht ausgeführt):

```bash
gh api repos/:owner/:repo/branches/main/protection \
  --method PUT \
  -f "required_status_checks[strict]=true" \
  -f "required_status_checks[contexts][]=Unit Tests" \
  -f "required_status_checks[contexts][]=Safety Test Suite" \
  -f "required_status_checks[contexts][]=Integration Tests (Docker)" \
  -f "required_status_checks[contexts][]=Vitest Unit Tests" \
  -f "enforce_admins=true"
```
