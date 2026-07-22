> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 57 — GHCR-Pull-Deployment

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "GHCR-Pull-Deployment (Sprint 57)".
Baut auf Sprint 56 (CI-Image-Publishing nach GHCR,
[tasks/sprints/56-ci-image-publishing.md](sprints/56-ci-image-publishing.md)) auf: die
`avoc-*`-Images existieren bereits in GHCR, das VM-Deployment (`ansible/deploy.yml`, lokaler
Build+`docker save`/`load`-Weg) nutzt sie bisher nicht — das war in Sprint 56 bewusst
ausgeklammert ("Nicht Teil dieses Sprints").

**Auftrag (2026-07-22):** Nutzer möchte den VM-Deploy-Workflow auf `commit → push → merge main
→ GitHub baut+pusht Images → VM pullt von GHCR` umstellen, statt die Images für jedes Deployment
nochmal lokal zu bauen.

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- **Fallback bleibt erhalten:** Lokaler Build+save/load-Weg wird nicht entfernt, bleibt über den
  bestehenden `SKIP_REGISTRY_PULL`-Mechanismus als Opt-out erreichbar. Nur der Default dreht sich
  um (GHCR-Pull wird Normalfall für die lokale Test-VM).
- **Versions-Tags werden mit eingeführt:** CI pusht bei echten Git-Tag-Pushes (z. B. `v1.2.3`)
  zusätzlich einen Versions-Tag nach GHCR, nicht nur `:latest`/`:<sha>`.

Datum: 2026-07-22 | Status: ✅ vollständig — alle 5 Tasks umgesetzt und real verifiziert.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| GHCRPULL-01 | **Nutzeraktion:** Least-Privilege GHCR-PAT anlegen — Classic PAT nur mit `read:packages`-Scope (Fine-grained-PATs unterstützen GHCR-Login/Packages-API nachweislich nicht zuverlässig). NICHT den `Administration`-Token aus CIPUB-04 wiederverwenden. | S | ✅ | — |
| GHCRPULL-02 | `.github/workflows/docker-build.yml`: Tag-Push-Trigger (`on.push.tags: ['v*']`), zusätzlicher Versions-Tag `ghcr.io/<owner>/avoc-<service>:<git-tag>` bei echtem Tag-Push. `pull_request` bleibt `push: false`. | M | ✅ | CIPUB-01 (Sprint 56) |
| GHCRPULL-03 | `infrastructure/compose/docker-compose.hetzner.yml`: die 8 `avoc-*`-Image-Referenzen auf `ghcr.io/<owner-lowercase>/avoc-<service>:${VERSION}` umstellen (Drittanbieter-Images unverändert). | S | ✅ | GHCRPULL-02 |
| GHCRPULL-04 | `ansible/roles/secrets` (echten PAT einbinden, NICHT im Repo committen) + `ansible/deploy.yml`/`scripts/deploy-hetzner.sh`: `SKIP_REGISTRY_PULL`-Default umdrehen, `REGISTRY`/`DOCKER_USERNAME`/`DOCKER_PASSWORD` real befüllen. Fallback bleibt über Flag erreichbar. | M | ✅ | GHCRPULL-01, GHCRPULL-03 |
| GHCRPULL-05 | Verifikation real gegen `avoc-local-vm` (libvirt/KVM, analog Sprint 49): Internet-Konnektivität VM→`ghcr.io`, GHCR-Pull-Weg UND Fallback-Weg je einmal real durchspielen, Doku-Updates. | M | ✅ | GHCRPULL-04 |

**Nicht Teil dieses Sprints:** Umbenennung `DOCKER_USERNAME`/`DOCKER_PASSWORD` auf generische
Registry-Namen (kosmetisch, eigener Task falls gewünscht); echter Hetzner-Produktivserver
existiert noch nicht (Verifikation nur gegen lokale Test-VM); automatisches Rollback bei
fehlgeschlagenem Pull; Image-Retention-/Cleanup-Policies in GHCR.

## Ergebnis

**GHCRPULL-02 (✅):** `on.push.tags: ["v*"]` ergänzt. Alle 3 Jobs (`go-services`, `frontend`,
`vehicle-mock`) bekommen einen neuen "Compute image tags"-Step: bei Tag-Push nur
`ghcr.io/<owner>/avoc-<service>:<git-tag>`, bei Push auf `main` weiterhin `:latest` + `:<sha>`
(kein `:latest`-Move durch einen Tag-Push, der ggf. nicht von `main`s aktuellem HEAD gebaut ist).
Login-`if`/`push`-Bedingung erweitert um `github.ref_type == 'tag'`.

**GHCRPULL-03 (✅):** Alle 8 `avoc-*`-Image-Referenzen in `docker-compose.hetzner.yml` von
`avoc-x:${VERSION}` auf `${REGISTRY}/avoc-x:${VERSION}` umgestellt. `${REGISTRY}` kommt aus der
`.env` (bereits vorhandenes Template-Feld) und ist damit der einzige Schalter zwischen
GHCR-Pull-Weg und Fallback. Drittanbieter-Images unverändert. Lokal gegen `docker compose config
--images` verifiziert (REGISTRY=ghcr.io/floristein → alle 8 korrekt präfigiert, Drittanbieter
unverändert).

**GHCRPULL-04 (✅):** `avoc_skip_registry_pull` (Default `false`) neu in
`ansible/group_vars/local_vm.yml`. `ansible/deploy.yml`: kompletter Save/Load-Block
(Images ermitteln → prüfen → `docker save` → übertragen → `docker load` → aufräumen) läuft nur
noch `when: avoc_skip_registry_pull | bool`; `select('match','^avoc-')`-Filter auf
`select('search','avoc-')` korrigiert (Images heißen nach GHCRPULL-03 nicht mehr `avoc-...`,
sondern `<registry>/avoc-...`). `SKIP_REGISTRY_PULL` wird 1:1 aus `avoc_skip_registry_pull`
durchgereicht statt hartkodiert `"true"`. `ansible/roles/secrets/defaults/main.yml`:
`avoc_secrets_registry` default `ghcr.io/floristein` (Override via `GHCR_REGISTRY`-Env-Var);
`avoc_secrets_docker_username`/`avoc_secrets_docker_password` lesen jetzt `GHCR_USERNAME`/
`GHCR_TOKEN` aus der Umgebung (Fallback auf alte Dummy-Werte, falls ungesetzt — echter GHCR-Pull
schlägt dann bewusst beim Login fehl statt still falsche Images zu laden).
`scripts/deploy-hetzner.sh`: `docker login` bekommt jetzt explizit den Registry-Host
(`${REGISTRY%%/*}`) statt implizit Docker Hub anzunehmen. `ansible-playbook deploy.yml
--syntax-check` grün, alle geänderten YAML-Dateien mit `python3 -c "yaml.safe_load(...)"`
geprüft, `bash -n deploy-hetzner.sh` grün.

**GHCRPULL-01 (✅):** Nutzer hat einen Classic-PAT mit `read:packages`-Scope angelegt und lokal
(gitignored `ansible/.ghcr-credentials.sh`, s. `.gitignore`-Ergänzung) als `GHCR_USERNAME`/
`GHCR_TOKEN` bereitgestellt — nie im Chat-Verlauf sichtbar geworden. Real verifiziert:
`docker login ghcr.io` erfolgreich, `docker pull ghcr.io/floristein/avoc-frontend:latest`
erfolgreich (Digest-Pull, nicht nur Login).

**GHCRPULL-05 (✅ — beide Wege real gegen `avoc-local-vm` durchgespielt):**
- VM gestartet (`virsh start avoc-local-vm`), IP per `virsh domifaddr` ermittelt
  (`192.168.122.13`, DHCP-Lease unverändert zu Sprint 49), `ansible/inventory/hosts.ini` lokal
  (nicht committet — Datei bleibt mit `<VM_IP_PLACEHOLDER>` im Repo, analog Sprint-49-Präzedenz)
  aktualisiert, `ansible -m ping` bestätigte Erreichbarkeit.
- `site.yml` erneut gelaufen (idempotent, `.env` neu templated) — `.env` auf der VM bestätigt
  echte Werte: `REGISTRY=ghcr.io/floristein`, `DOCKER_USERNAME=FloriStein`, `DOCKER_PASSWORD`
  korrekte Länge (Token nie ausgegeben).
- **GHCR-Pull-Weg (Default):** `ansible-playbook deploy.yml` — Save/Load-Block korrekt
  übersprungen (`skipping`), `deploy-hetzner.sh` loggt sich bei `ghcr.io` ein ("Login Succeeded"),
  pullt alle 8 `avoc-*`-Images direkt von GHCR, Stack startet vollständig (15/15 Container
  "Up"). Health-Check: `https://localhost:443/` → HTTP 200, `http://localhost:8080/health` →
  HTTP 200.
- **Fallback-Weg:** lokal alle 8 `avoc-*`-Images unter `ghcr.io/floristein/avoc-<service>:latest`
  gezogen (simuliert "lokal gebaut", identischer Tag wie ihn `make build-prod
  REGISTRY=ghcr.io/floristein` erzeugen würde). `ansible-playbook deploy.yml -e
  avoc_skip_registry_pull=true` — `select('search','avoc-')`-Fix bestätigt: alle 8 Images korrekt
  gefunden (keine "Image fehlt lokal"-Warnung), `docker save`→Transfer→`docker load` erfolgreich,
  `deploy-hetzner.sh` nimmt korrekt den `SKIP_REGISTRY_PULL=true`-Zweig (kein Login/Pull), Stack
  erneut vollständig hochgefahren, Health-Checks wieder HTTP 200/200.
- VM bewusst nicht wieder heruntergefahren (läuft weiter, Nutzer kann Stack direkt ansehen).

**Nachtrag:** `docs/deployment/UEBERGABE-ABWEICHUNGEN.md` Abweichung 2 um einen
"Ergänzung Sprint 57"-Absatz erweitert — die dort zuvor als aktueller Stand beschriebene Aussage
("SKIP_REGISTRY_PULL=true, kein Docker-Hub-Login/-Pull") war nach GHCRPULL-04 nicht mehr korrekt,
jetzt real bestätigt korrigiert. `hetzner-setup.md`-Volltext-Umstellung (Docker-Hub-Sprache auf
Registry-agnostisch) bewusst nicht in diesem Sprint gemacht — betrifft nur den (weiterhin nicht
existierenden) echten Hetzner-Server, kein Blocker für die hier verifizierte lokale VM.
