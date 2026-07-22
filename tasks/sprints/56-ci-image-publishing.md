> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 56 — CI-Image-Publishing nach GHCR

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "CI-Image-Publishing nach GHCR
(Sprint 56)". Baut auf CIHARD-03 (Sprint 55, [tasks/sprints/55-ci-haertung.md](sprints/55-ci-haertung.md))
auf — der bestehende `docker-build.yml`-Job baut bereits alle 8 Targets (6 Go-Services +
Frontend + vehicle-mock), aktuell nur zur Build-Verifikation (`push: false`).

**Auftrag (2026-07-22):** Nutzer möchte die `avoc-*`-Images zusätzlich in GitHub Actions bauen
lassen, statt sie ausschließlich lokal zu bauen (bisherige Voraussetzung für
`ansible/deploy.yml`, siehe EPIC "Lokale Ansible-VM als Hetzner-Nachbildung").

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- Registry: **GHCR** (`ghcr.io`), nicht Docker Hub.
- **Bestehenden Job in `.github/workflows/docker-build.yml` erweitern**, kein neuer separater
  Workflow.

Datum: 2026-07-22 | Status: 🟢 CIPUB-01..03 umgesetzt, CIPUB-04 blockiert (fehlende Berechtigung).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CIPUB-01 | `.github/workflows/docker-build.yml` erweitern: `docker/login-action` gegen `ghcr.io` (`GITHUB_TOKEN`), `permissions: packages: write` auf Job-Ebene, `push: true` nur bei `push`-Event auf `main` (bei `pull_request` weiterhin `push: false`). Tag-Schema festlegen (z. B. `ghcr.io/<owner>/avoc-<service>:latest` + `:<git-sha>`). | M | ✅ | CIHARD-03 (Sprint 55) |
| CIPUB-02 | Kurzer Hinweis-Kommentar in `ansible/deploy.yml`-Kopf und/oder `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`, dass GHCR-Images jetzt existieren, ohne den bestehenden lokalen Build+`docker save`/`load`-Weg in diesem Sprint umzustellen. | S | ✅ | CIPUB-01 |
| CIPUB-03 | Verifikation: `actionlint` gegen die geänderte Workflow-Datei, nach echtem Push auf `main` prüfen, dass die Packages tatsächlich unter GHCR erscheinen (Sichtbarkeit public/private dokumentieren), Doku-/Backlog-Status-Update. | S | ✅ (lokaler Teil) / 🔲 (Post-Merge-Teil, siehe Ergebnis) | CIPUB-01 |
| CIPUB-04 | **CIGATE-06 abschließen** (seit Sprint 41 vorbereitet, nie aktiviert): Branch-Protection/Required-Status-Checks für die 4 blockierenden Jobs real im GitHub-Repo-Setting einschalten — nur nach nochmaliger expliziter Nutzerbestätigung unmittelbar vor der Aktivierung. Auf Nutzerwunsch (2026-07-22) mitaufgenommen, thematisch unabhängig von CIPUB-01..03. | S | 🔲 blockiert | CIGATE-06 (Vorbereitung Sprint 41) |

**Nicht Teil dieses Sprints:** Umstellung von `ansible/deploy.yml` auf GHCR-Pull statt lokalem
Build+Transfer; Docker-Hub als Alternative; Image-Retention-/Cleanup-Policies in GHCR.

## Ergebnis

**CIPUB-01 (✅ umgesetzt):** `.github/workflows/docker-build.yml` erweitert für alle 3 Jobs
(`go-services`-Matrix mit 6 Services, `frontend`, `vehicle-mock`):
- `permissions: {contents: read, packages: write}` auf Job-Ebene ergänzt.
- Neuer Schritt `docker/login-action@v3` gegen `ghcr.io`, Login mit `github.actor` +
  `secrets.GITHUB_TOKEN` (kein neues Secret), nur ausgeführt wenn
  `github.event_name == 'push' && github.ref == 'refs/heads/main'`.
- `push:` auf `docker/build-push-action@v6` von hartem `false` auf denselben bedingten Ausdruck
  umgestellt — bei `pull_request` bleibt es bei `false`, keine Images von unverifizierten
  PR-Branches.
- Tag-Schema: `ghcr.io/<owner>/avoc-<service>:latest` + `ghcr.io/<owner>/avoc-<service>:<sha>`
  (`${{ github.sha }}`, voller SHA — "kurzer SHA reicht" aus dem Auftrag als Hinweis verstanden,
  dass kein zusätzlicher Kurz-SHA-Berechnungsschritt nötig ist, nicht als harte Kürzungspflicht).
  Service-Namen: `control-server`, `auth-service`, `safety-service`, `telemetry-service`,
  `webrtc-sfu`, `fleet-service` (Matrix), `frontend`, `vehicle-mock`.
- **Beim Umsetzen gefundene Eigenheit (nicht im Auftrag genannt):** `github.repository_owner`
  ist für dieses Repo `FloriStein` (gemischte Groß-/Kleinschreibung) — Docker/GHCR verlangt aber
  durchgehend Kleinschreibung in Image-Referenzen, und GitHub-Actions-Ausdrücke kennen kein
  `toLower()`. Ohne Fix wäre der Push bei jedem echten Lauf auf `main` mit `invalid reference
  format` gescheitert. Fix: neuer Schritt "Compute lowercase owner" pro Job
  (`echo "OWNER_LC=${GITHUB_REPOSITORY_OWNER,,}" >> "$GITHUB_ENV"`), Tags referenzieren
  `${{ env.OWNER_LC }}` statt `${{ github.repository_owner }}` direkt.

**CIPUB-02 (✅ umgesetzt):** Beide vorgeschlagenen Stellen ergänzt (nicht nur eine):
- `ansible/deploy.yml`-Kopfkommentar: neuer Absatz nach dem "Kein Docker-Hub-Roundtrip"-Absatz,
  erklärt dass GHCR-Images seit Sprint 56 zusätzlich existieren, der lokale Save/Load-Weg aber
  unverändert der einzige Transferweg auf die VM bleibt.
- `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`: Ergänzung unter "Abweichung 2" (derselbe
  Save/Load-Kontext wie `ansible/deploy.yml`), inkl. Verweis auf den möglichen künftigen
  Revert-Pfad (GHCR-Pull statt lokalem Transfer) — analog zum in Abweichung 1 skizzierten
  Docker-Hub-Pfad, aber bewusst nicht in diesem Sprint umgesetzt.

**CIPUB-03 — lokal verifizierbarer Teil (✅):**
- `actionlint` gegen `.github/workflows/docker-build.yml` UND gegen alle anderen Workflows im
  Verzeichnis: 0 Findings (`actionlint`, Version v1.7.12, `built from source`).
- YAML-Struktur manuell gegengeprüft (3 Jobs, je Login-Schritt korrekt bedingt, Tags korrekt
  referenziert).

**CIPUB-03 — Teil, der erst nach dem Merge verifizierbar ist (🔲, bewusst nicht simuliert):**
Der eigentliche Push nach GHCR passiert erst bei einem echten `push`-Event auf `main` — das kann
aus diesem Feature-Branch/PR heraus nicht ausgelöst oder lokal nachgestellt werden. Offener
Verifikationsschritt nach dem Merge (durch den Nutzer oder in einer Folge-Session):
1. Ersten echten Push auf `main` abwarten (bzw. den Merge-Commit dieses PRs).
2. GitHub-Actions-Lauf von `docker-build.yml` auf `main` grün abwarten, insbesondere die drei
   neuen "Log in to GHCR"/Push-Schritte je Job.
3. Packages unter `https://github.com/FloriStein?tab=packages` prüfen — es sollten 8 neue
   `avoc-*`-Packages erscheinen (6 Go-Services + `avoc-frontend` + `avoc-vehicle-mock`), je mit
   `:latest`- und `:<sha>`-Tag.
4. **Sichtbarkeit explizit prüfen und dokumentieren:** GHCR-Packages sind standardmäßig
   **private**, auch wenn das Repo selbst public ist — das gilt es hier zu bestätigen (nicht
   anzunehmen), da ein versehentlich public gepushtes Image (falls doch anders konfiguriert)
   ein Security-Fund wäre (CLAUDE.MD §13).

**CIPUB-04 (🔲 blockiert, nicht umgesetzt):** Vor jedem Aktivierungsversuch wurde geprüft, ob der
verfügbare GitHub-Zugang (github-MCP-Server) überhaupt die nötige Berechtigung hat.
`mcp__github__get_me` bestätigt einen funktionierenden Zugriff als `FloriStein`, aber der
github-MCP-Server exponiert **kein einziges** Tool für Branch-Protection/Required-Status-Checks
oder generische Repo-Admin-API-Calls (im Unterschied z. B. zum Grafana-MCP-Server, der einen
generischen `grafana_api_request`-Fallback hat) — konsistent mit der aus einer früheren Session
bekannten Einrichtung: das Fine-Grained-Token wurde bewusst nur mit
Contents/Issues/Pull-requests/Actions/Metadata angelegt, ohne `Administration: Read and write`.
Eine `gh`-CLI als Alternative ist auf diesem Rechner nicht installiert (`command not found: gh`).
Da die Berechtigung fehlt, wurde **nicht** versucht, das Token eigenmächtig zu erweitern oder die
Aktivierung zu simulieren — das ist laut Auftrag eine Nutzerentscheidung. CIPUB-04 bleibt offen in
`tasks/backlog.md`; sobald ein Token mit `Administration: Read and write` verfügbar ist, kann der
Schritt (inkl. der davor laut Auftrag ohnehin nötigen erneuten expliziten Bestätigung unmittelbar
vor der Aktivierung) nachgeholt werden.

**Nicht abgedeckt / bewusst offen gelassen:**
- Realer GHCR-Push-Lauf (s. CIPUB-03 oben) — technisch erst nach Merge möglich.
- CIPUB-04 — blockiert durch fehlende Administration-Berechtigung, s. oben.
- Kein neuer Unit-/Integrationstest geschrieben: reine CI-Workflow-Konfiguration ohne
  Anwendungslogik, CLAUDE.MD §17s Pflicht-Fallgruppen-Checkliste (Grenzwerte, Fehlerpfade,
  Nebenläufigkeit etc.) greift hier nicht analog — `actionlint` + der oben dokumentierte manuelle
  Post-Merge-Verifikationsschritt sind die dem Änderungstyp angemessene Prüfung.
