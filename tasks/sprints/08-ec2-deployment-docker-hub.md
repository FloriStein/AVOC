# Sprint 8 — EC2 Deployment via Docker Hub

Abgeschlossen: 2026-06-04

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| DEPLOY-01 | ADR-019 — Deployment-Strategie | L | ✅ `docs/adr/019-deployment-strategy.md` — Docker Hub private Repos + EC2 Elastic IP + AWS SSM Parameter Store; linux/amd64; 3 Optionen bewertet |
| DEPLOY-02 | Makefile `build-prod` + `push` | M | ✅ `build-prod`: docker buildx --platform linux/amd64 für 5 Go-Services + Frontend; `push`: build-prod + docker push; Guard-Clause bei fehlendem DOCKER_USERNAME; `.docker-username` als gitignorierter lokaler Helper |
| DEPLOY-03 | `docker-compose.prod.yml` | M | ✅ Alle Custom-Services mit `image:` statt `build:`, `restart: unless-stopped` auf allen 11 Services, YAML validiert |
| DEPLOY-04 | `scripts/setup-ssm.sh` + `scripts/deploy.sh` | M | ✅ `setup-ssm.sh`: interaktiv, 9 SSM-Parameter, Input-Validierung (min. Längen), SecureString für Secrets; `deploy.sh`: SSM-Fetch → docker login → pull → up -d; Bash-Syntax valide |
| DEPLOY-05 | coturn EC2-Konfiguration | M | ✅ In `docker-compose.prod.yml`: Command-Override mit `--external-ip=${TURN_EXTERNAL_IP}`, Port-Range 49160-49200 (passend zu CDK Security Group) |
| DEPLOY-06 | Grafana Security | S | ✅ `GF_AUTH_ANONYMOUS_ENABLED: "false"`, `GF_AUTH_DISABLE_LOGIN_FORM: "false"`, Credentials aus SSM |
| DEPLOY-07 | EC2 Bootstrap Guide | M | ✅ `docs/deployment/ec2-bootstrap.md` — 6 Schritte, Troubleshooting (5 Szenarien), Rollback, Update-Prozess |

### Neue Dateien

- `infrastructure/compose/docker-compose.prod.yml` — Production Compose (DEPLOY-03/05/06)
- `scripts/setup-ssm.sh` — SSM Parameter anlegen (DEPLOY-04)
- `scripts/deploy.sh` — EC2 Deployment Script (DEPLOY-04)
- `docs/adr/019-deployment-strategy.md` — ADR-019 (DEPLOY-01)
- `docs/deployment/ec2-bootstrap.md` — Bootstrap Guide (DEPLOY-07)

### Geänderte Dateien

- `Makefile` — `build-prod` + `push` Targets + `DOCKER_USERNAME`/`REGISTRY`/`PLATFORM` Variablen (DEPLOY-02)
- `.gitignore` — `.docker-username` ergänzt
- `tasks/current-sprint.md` — Sprint 8 Plan + DoD
- `tasks/backlog.md` — DEPLOY-Epics + offene Folge-Entscheidungen aktualisiert
- `docs/implementation-plan.md` — Phase 8 ergänzt, ADR-Index auf 19 ADRs
- `DECISIONS.MD` — ADR-019 + Folge-Entscheidungen aktualisiert

### Gelöschte Dateien

- `infrastructure/docker/telemtry.Dockerfile` — leer, Tippfehler im Namen, nicht referenziert
- `infrastructure/docker/video.Dockerfile` — leer, nicht referenziert

### Testprotokoll (2026-06-04)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | `make build-prod` ohne `DOCKER_USERNAME` | Guard-Clause + Fehlermeldung | ✅ ERROR-Meldung + exit 1 |
| T02 | `GO_SERVICES` in Makefile vs. `cmd/` Verzeichnisse | Exakte Übereinstimmung (5/5) | ✅ identisch |
| T03 | Referenzierte Dockerfiles vorhanden | `go-service.Dockerfile`, `frontend.Dockerfile` | ✅ beide vorhanden, keine toten Files mehr |
| T04 | `docker-compose.prod.yml` YAML-Syntax | Keine Fehler | ✅ YAML valide |
| T05 | Kein `build:` in Custom Services | 0 build:-Einträge | ✅ 0 gefunden |
| T06 | coturn `--external-ip` Flag | In Command-Override vorhanden | ✅ `--external-ip=${TURN_EXTERNAL_IP}` |
| T07 | Grafana Anonymous Auth deaktiviert | `GF_AUTH_ANONYMOUS_ENABLED: "false"` | ✅ |
| T08 | `restart: unless-stopped` auf allen Services | 11 Services | ✅ 11/11 |
| T09 | Alle 9 Env-Var-Referenzen in Compose | `${REGISTRY}`, `${JWT_SECRET}`, … | ✅ alle 9 vorhanden |
| T10 | Image-Naming für alle 6 Services | `avoc-<service>` Pattern | ✅ alle 6 vorhanden |
| T11 | SSM-Pfade `setup-ssm.sh` ↔ `deploy.sh` identisch | 9 Pfade, kein Unterschied | ✅ identisch (diff leer) |
| T12 | `deploy.sh` exportiert alle Compose-Env-Vars | 9 `export`-Statements | ✅ alle 9 |
| T13 | `deploy.sh` nutzt `docker compose` (Plugin v2) | Kein `docker-compose` als Befehl | ✅ nur Dateiname enthält Bindestrich, alle Befehle korrekt |
| T14 | Port-Konsistenz Compose ↔ CDK Security Group | 7 Ports übereinstimmend | ✅ 3000, 8080, 1883, 3479, 8084, 10000-10050, 3001 |
| T15 | Bootstrap Guide — 6 Schritte vorhanden | Schritt 1–6 | ✅ alle 6 |
| T16 | Bootstrap Guide — kritische Abschnitte | Troubleshooting, Rollback, SSM, Security Group | ✅ alle vorhanden |
| T17 | `.docker-username` gitignored | In `.gitignore` eingetragen | ✅ |
| T18 | Tote Dockerfiles entfernt | `telemtry.Dockerfile`, `video.Dockerfile` weg | ✅ beide gelöscht |
| T19 | ADR-019 vollständig | 5 Pflichtabschnitte | ✅ Optionen, Entscheidung, Konsequenzen, SSM-Parameterstruktur, Image-Naming |

### Fix während Testphase

**T13 — False Positive**: `grep "docker-compose"` schlug an, weil der Dateiname `docker-compose.prod.yml` im `-f`-Argument vorkommt. Tatsächlicher Befehl ist korrekt `docker compose` (Plugin v2). Test verfeinert auf Erkennung von `docker-compose` als Standalone-Befehl.

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| Tests gesamt | 19/19 ✅ | — |
| Build-Targets neu | 2 (`build-prod`, `push`) | — |
| Services in docker-compose.prod.yml | 11 | — |
| SSM Parameter | 9 | — |
| ADRs gesamt | 19 | — |
| Safety Regression | 19/19 ✅ (kein Go-Code geändert) | 19/19 |
