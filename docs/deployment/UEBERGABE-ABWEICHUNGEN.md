# Übergabe-Abweichungen von Architekturentscheidungen

Dieses Dokument sammelt Stellen, an denen die tatsächliche Implementierung bewusst von einer
getroffenen Architekturentscheidung (ADR) abweicht — mit Begründung und Revert-Pfad, falls die
ursprüngliche Entscheidung später doch umgesetzt werden soll. Referenziert aus mehreren Stellen im
Code (`infrastructure/compose/docker-compose.prod.yml`, `scripts/deploy.sh`) und in der
Sprint-Historie (`tasks/sprints/`); als eigene Datei angelegt in Sprint 49 (LOCALVM-09), nachdem
sie zuvor referenziert, aber nie erstellt wurde (bekannte Lücke seit Sprint 48).

---

## Abweichung 1 — Kein Docker Hub für den EC2-Produktivpfad (ADR-019)

**Betroffene Entscheidung:** [ADR-019](../adr/019-deployment-strategy.md) — Docker Hub (private
Repositories) als Registry für den EC2-Deploy.

**Tatsächliche Umsetzung:** `scripts/deploy.sh` und `infrastructure/compose/docker-compose.prod.yml`
nutzen **keine** Registry. Images werden auf dem Dev-Rechner gebaut, per `docker save` als Tar
verpackt, direkt auf die Instanz übertragen (S3+SSM oder scp) und dort per `docker load` in den
lokalen Docker-Daemon der Instanz eingespielt. Entsprechend referenzieren die avoc-*-Images in
`docker-compose.prod.yml` kein Registry-Präfix (nur `avoc-x:${VERSION}`), und es gibt kein
`docker login`/`docker compose pull` in `deploy.sh`.

**Begründung:** Zum Zeitpunkt der Umsetzung war der lokale Save/Load-Weg der pragmatischere
nächste Schritt (kein Docker-Hub-Account/-Setup als Voraussetzung, kein Rate-Limit-Risiko beim
Pull). Sicherheits-/Betriebsanforderungen aus ADR-019 (kein Quellcode auf der Instanz, ein
einzelner Deployment-Befehl) bleiben dabei vollständig erfüllt — nur der Registry-Baustein
selbst wurde durch einen lokalen Transfer ersetzt.

**Revert-Pfad (falls Docker Hub später doch gebraucht wird, z. B. Multi-Instanz-Rollout):**
1. `make push` (baut + pusht alle Images nach `docker.io/$(DOCKER_USERNAME)`).
2. In `docker-compose.prod.yml` bei allen avoc-*-Images das Registry-Präfix ergänzen
   (`${REGISTRY}/avoc-x:${VERSION}` statt `avoc-x:${VERSION}`).
3. In `scripts/deploy.sh` den Save/Load-Block durch `docker login` + `docker compose pull`
   ersetzen (siehe `docs/deployment/hetzner-setup.md` Schritt 7 — dort ist der reguläre
   Docker-Hub-Weg für den späteren echten Hetzner-Server unverändert dokumentiert).

**Status:** Bewusst offen gelassen, kein aktueller Handlungsbedarf — beide Cloud-Zielumgebungen
(EC2 und die für Hetzner geplante, siehe `hetzner-setup.md`) funktionieren mit dem Save/Load-Weg
zuverlässig.

---

## Abweichung 2 — Lokale Ansible-VM (Hetzner-Nachbildung): derselbe Save/Load-Weg, jetzt auch dort verifiziert

**Kontext:** Das EPIC "Lokale Ansible-VM als Hetzner-Nachbildung" (Sprint 48/49,
`tasks/backlog.md`) übernimmt bewusst dasselbe Muster wie Abweichung 1 — `ansible/deploy.yml`
ermittelt die benötigten avoc-*-Images aus `docker-compose.hetzner.yml`, verpackt sie per
`docker save`, überträgt sie auf die VM und lädt sie dort per `docker load`
(`scripts/deploy-hetzner.sh` läuft mit `SKIP_REGISTRY_PULL=true`, kein Docker-Hub-Login/-Pull).
Kein neuer Doku-Fund — konsistente Fortführung von Abweichung 1, jetzt real gegen eine laufende
VM verifiziert (Sprint 49, LOCALVM-08b/c).

**Wichtige Voraussetzung, real erst in Sprint 49 sichtbar geworden:** Der Save/Load-Weg
überträgt exakt die lokal vorhandenen Docker-Images — ist ein Image lokal veraltet (z. B. vor
einem relevanten Code-Merge gebaut), wird genau dieser veraltete Stand auf die Zielmaschine
übertragen, ohne Warnung außer dem knappen "Image fehlt lokal"-Hinweis (der nur das Fehlen prüft,
nicht die Aktualität). Vor jedem realen Deploy über diesen Weg müssen die avoc-*-Images daher
frisch aus dem aktuellen Stand gebaut sein (`make build-prod` + Retag ohne Registry-Präfix, siehe
`ansible/deploy.yml`-Kopfkommentar).

---

## Nicht hier dokumentiert

Kleinere, rein lokale Abweichungen (z. B. Dateiberechtigungen für Bind-Mounts) werden direkt als
Kommentar im jeweiligen Ansible-Task/Skript dokumentiert (siehe z. B.
`ansible/roles/bootstrap/tasks/main.yml`, `ansible/deploy.yml`) statt hier gesammelt — diese Datei
ist für Abweichungen von **explizit getroffenen ADR-Entscheidungen** reserviert, nicht für jeden
im Betrieb gefundenen Bug.
