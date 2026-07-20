> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 44 — Backup-Strategie Audit Store (ADR-018/023 Folge)

**Freigabe (2026-07-19):** Nutzer wählt dieses Thema für Sprint 44 gegenüber zwei Alternativen
(Migration zu AWS ECR, Session-Recording-Storage-Entscheidung). Schließt den seit ADR-019
offenen Punkt "Audit Store Backup-Strategie". Eingereiht nach Sprint 43.

**Wichtiger Vorrecherche-Befund — Backlog-Text war veraltet:** `tasks/backlog.md` und
`docs/adr/019-deployment-strategy.md` beschrieben den offenen Punkt bisher als "SQLite Volume auf
S3" (Stand ADR-018, Sprint 7). Tatsächlich hat ADR-023 (PostgreSQL-Migration) SQLite bereits
vollständig ersetzt: `audit_events` liegt seither in PostgreSQL (`postgres-data`-Docker-Volume,
`infrastructure/compose/docker-compose.prod.yml:31-46`), das `audit-data`-Volume aus ADR-018
existiert im Compose-Setup nicht mehr. `cmd/control-server/main.go:81-89` (`newAuditWriter`) nutzt
`audit.NewPostgresAuditWriter(db)`, kein SQLite-Pfad mehr im Code. ADR-023 selbst nennt
"Standardisierte Backup-Workflows (`pg_dump`)" bereits als erwarteten Vorteil der Migration — der
Backup-Task war seither nur nie eingeplant. Beide Doku-Stellen oben in diesem Sprint bereits auf
"Postgres-Volume" korrigiert.

**Architektur-Entscheidung (bei der Planung getroffen):**
- **`pg_dump` gegen den laufenden `postgres`-Container statt Datei-Kopie des Docker-Volumes** —
  ein Volume-Snapshot während laufendem Betrieb kann inkonsistent sein (kein atomarer Zustand),
  `pg_dump` liefert einen konsistenten logischen Dump zur Laufzeit, ohne den Service zu stoppen.
  Kein `pg_basebackup`/WAL-Archivierung (Point-in-Time-Recovery) — für dieses Betriebsmodell
  (Single-Instance-Testbetrieb, kein HA-Anspruch) ist ein tägliches logisches Backup ausreichend,
  analog zur bereits akzeptierten "kein Cross-Host-Bedarf"-Argumentation aus Sprint 40.
- **S3-Bucket-Name per SSM statt hartkodiert** — der bestehende `AppBucket` (CDK,
  `infrastructure/AWS/cdk_server-stack.ts:75-79`, bereits `grantReadWrite` für die Instance-Role,
  siehe Kommentar Zeile 138 "für zukünftige Audit-Log-Backups, ADR-018") bekommt einen neuen
  `ssm.StringParameter` unter `/avoc/prod/backup-bucket-name` direkt im selben CDK-Stack (kein
  manueller Zusatzschritt nach `cdk deploy` nötig) — konsistent mit dem bestehenden
  SSM-getriebenen Secret-Verteilungsmuster in `scripts/deploy.sh`.
- **S3-Lifecycle-Regel statt manueller Löschung** — Backups unter Prefix `backups/postgres/`
  verfallen nach 30 Tagen automatisch (`lifecycleRules` im CDK-Stack), damit der ohnehin schon
  `versioned: true`/`autoDeleteObjects: true` konfigurierte Bucket nicht unbegrenzt wächst.
- **Tägliches Cron-Backup auf dem EC2-Host statt Container-internem Scheduler** — einfachste
  Lösung ohne neuen Docker-Compose-Service, analog zur bestehenden "generiere/registriere einmalig
  bei Deploy"-Philosophie in `scripts/deploy.sh` (SSL-Zertifikat, Mosquitto-Passwd).

**Vorrecherche (2026-07-19):**
- `infrastructure/compose/docker-compose.prod.yml:31-46`: `postgres`-Service, `POSTGRES_USER=avoc`,
  `POSTGRES_DB=avoc`, `POSTGRES_PASSWORD=${DB_PASSWORD}` (aus `$APP_DIR/.env`, nicht SSM — siehe
  `scripts/deploy.sh` Kommentar Zeile 61-62), Healthcheck `pg_isready -U avoc -d avoc`.
- `infrastructure/AWS/cdk_server-stack.ts:75-79`: `AppBucket` (`s3.Bucket`, `versioned: true`,
  `removalPolicy: DESTROY`, `autoDeleteObjects: true`), Zeile 139 `bucket.grantReadWrite(instance.role)`
  bereits vorhanden. Zeile 203-205: `CfnOutput BucketName` existiert bereits, aber landet aktuell
  nur in der CDK-Konsolenausgabe, nicht in SSM — Backup-Skript bräuchte sonst den Bucket-Namen
  hartkodiert oder manuell in `.env` gepflegt.
- `scripts/deploy.sh:38-51` (`get`/`get_secure`-Helfer für SSM-Parameter) — Muster für den neuen
  `/avoc/prod/backup-bucket-name`-Parameter wiederverwendbar.
- `scripts/deploy.sh:94-115` (SSL-Zertifikat-Generierung, "einmalig generieren, wiederverwenden")
  als Strukturvorbild für ein neues Skript `scripts/backup-audit-store.sh` (hier aber täglich
  ausgeführt statt einmalig).
- CDK verwendet `aws-cdk-lib/aws-s3` bereits (Zeile 5); `aws-cdk-lib/aws-ssm` (für
  `ssm.StringParameter`) ist als Teil von `aws-cdk-lib` bereits verfügbar, kein neues
  `package.json`-Dependency nötig.
- `docs/adr/023-postgresql-migration.md:71` nennt "Standardisierte Backup-Workflows (`pg_dump`)"
  bereits explizit als erwarteten Vorteil — dieser Sprint löst genau dieses Versprechen ein.

**Nicht Teil dieses Sprints:** Point-in-Time-Recovery (`pg_basebackup`/WAL-Archivierung — kein
HA-Anspruch für Single-Instance-Testbetrieb), automatisierter Restore-Test/-Runbook (eigener
Folge-Task, sobald ein erstes echtes Backup vorliegt), Verschlüsselung des Dumps vor Upload
(Bucket-seitige S3-Default-Encryption gilt bereits, kein zusätzlicher Client-seitiger Schritt in
diesem Scope).

Datum: 2026-07-19 | **Status: ✅ Abgeschlossen**
Vorgänger: Sprint 43 (Parallel-Worktree, separater Merge)
Branch/Worktree: `feature/fleet-service-foundation-auditbackup` (eigener Worktree, Basis
`feature/fleet-service-foundation-sprint35`).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| AUDITBACKUP-01 | CDK-Stack (`infrastructure/AWS/cdk_server-stack.ts`): neuer `ssm.StringParameter` (`/avoc/prod/backup-bucket-name` = `bucket.bucketName`) + S3-Lifecycle-Regel (Prefix `backups/postgres/`, Expiration 30 Tage) auf `AppBucket`. | S | ✅ | — |
| AUDITBACKUP-02 | Neues Skript `scripts/backup-audit-store.sh`: liest Bucket-Namen aus SSM (`get`-Helfer analog `deploy.sh`), `docker compose exec postgres pg_dump -U avoc avoc \| gzip`, Upload via `aws s3 cp` nach `s3://$BUCKET/backups/postgres/$(date +%F)-avoc.sql.gz`. | S | ✅ | AUDITBACKUP-01 |
| AUDITBACKUP-03 | Cron-Registrierung: `scripts/deploy.sh` ergänzt einen idempotenten Crontab-Eintrag (täglich, 03:00 UTC) für `backup-audit-store.sh` — Prüfung auf Doppel-Registrierung analog zum "generiere einmalig"-Muster (`crontab -l \| grep -q ... \|\| ...`). | S | ✅ | AUDITBACKUP-02 |
| AUDITBACKUP-04 | Verifikation: lokaler Trockenlauf von `backup-audit-store.sh` gegen den Dev-`postgres`-Container (Dump + lokale Datei, kein echter S3-Upload). Doku: `docs/adr/019-deployment-strategy.md` Zeile "Audit Store Backup-Strategie" auf ✅, `DECISIONS.MD`, `tasks/backlog.md`-Status-Update. | S | ✅ | AUDITBACKUP-01..03 |

**Nicht Teil dieses Sprints:** siehe "Nicht Teil dieses Sprints" oben (Point-in-Time-Recovery,
automatisierter Restore-Test, client-seitige Verschlüsselung des Dumps).

---

## Ergebnisse

**AUDITBACKUP-01 (CDK-Stack):** `infrastructure/AWS/cdk_server-stack.ts` — neuer Import
`aws-cdk-lib/aws-ssm`, `AppBucket` bekommt `lifecycleRules: [{ prefix: "backups/postgres/",
expiration: cdk.Duration.days(30) }]`, neuer `ssm.StringParameter` "BackupBucketNameParam"
(`parameterName: "/avoc/prod/backup-bucket-name"`, `stringValue: bucket.bucketName`). Es existiert
in diesem Repo kein lauffähiges CDK-App-Scaffold (kein `cdk.json`/`package.json`/`bin/`-Entry für
diese Stack-Datei — `cdk_server-stack.ts` ist eine reine IaC-Definitionsdatei ohne lokal
buildbares CDK-Projekt), daher kein `cdk synth`/`tsc` möglich; Syntax manuell gegen die
CDK-v2-API (`aws-cdk-lib`) geprüft. Kein `cdk deploy` ausgeführt (siehe Auftragsvorgabe — reale
Cloud-Ressource, separater späterer Schritt).

**AUDITBACKUP-02 (Backup-Skript):** neues `scripts/backup-audit-store.sh` (ausführbar,
`chmod +x`), Struktur analog `scripts/deploy.sh` (`get`-SSM-Helfer, `REGION`/`APP_DIR`-Env-Var-
Konvention). Zusätzlich `COMPOSE_FILE`-Env-Var (Default `docker-compose.prod.yml`) ergänzt, um das
Skript unverändert auch gegen die Dev-Compose-Datei testen zu können (siehe AUDITBACKUP-04) — kein
Abweichen vom Backlog-Auftrag, da Produktionsverhalten (Default) identisch bleibt. `bash -n`
syntaktisch geprüft.

**AUDITBACKUP-03 (Cron-Registrierung):** `scripts/deploy.sh` — neuer Schritt "[3/5] Prüfe
Cron-Registrierung für Audit-Store-Backup" (vorhandene Schritte auf `[1/5]`..`[5/5]` umnummeriert).
Idempotenz-Idiom `(crontab -l 2>/dev/null | grep -qF "backup-audit-store.sh") || { (crontab -l
2>/dev/null; echo "$CRON_CMD") | crontab -; }`. Zusätzlich `docs/deployment/ec2-bootstrap.md`
(Verzeichnisstruktur-Baum + `scp`-Befehlsliste) um `backup-audit-store.sh` ergänzt, damit die
Deployment-Anleitung nicht driftet (CLAUDE.MD §19) — das neue Skript muss wie `deploy.sh` selbst
nach `~/app/` kopiert werden, sonst schlägt der registrierte Cronjob beim ersten Lauf fehl.

**AUDITBACKUP-04 (Verifikation):**
- **Lokaler Trockenlauf gegen echten Dev-`postgres`-Container:** `docker compose -f
  infrastructure/compose/docker-compose.yml up -d postgres` (Container `avoc-postgres-1`,
  `healthy`). Das unveränderte `scripts/backup-audit-store.sh` wurde mit `APP_DIR=
  infrastructure/compose COMPOSE_FILE=docker-compose.yml` sowie einem temporären Fake-`aws`-
  Executable in `$PATH` ausgeführt (`aws ssm get-parameter` liefert einen Dummy-Bucket-Namen,
  `aws s3 cp` führt **keinen echten Upload** aus, validiert stattdessen die lokale Datei per
  `gzip -t` und kopiert sie zur Inspektion in ein Scratch-Verzeichnis) — kein echter AWS-Call.
  Ergebnis: Skript lief exitcode 0 durch, erzeugte einen echten `pg_dump`-Dump (8.0K gzip,
  `PostgreSQL database dump`-Header, 20 `CREATE TABLE`/`COPY`-Statements gegen das reale
  `avoc`-Schema, u.a. Tabelle `alerts`), Pfadaufbau `backups/postgres/<Datum>-avoc.sql.gz` korrekt.
  Lokale Dump-Datei wird vom Skript nach erfolgreichem Upload aufgeräumt (`rm -f`), wie in Prod
  vorgesehen. Dev-`postgres`-Container danach gestoppt (Volume/Daten unangetastet).
- **Cron-Idempotenz separat verifiziert (Fehlerfund im ersten Testlauf, siehe unten):** die
  `crontab -l | ... | crontab -`-Pipe-Idiom aus `scripts/deploy.sh` wurde gegen den **echten**
  `crontab`-Befehl dieser Maschine getestet (temporär, Original-Crontab war leer und wurde
  danach exakt wiederhergestellt): (1) vorhandener fremder Cron-Eintrag bleibt nach Registrierung
  erhalten, neuer Eintrag kommt hinzu (2 Zeilen), (2) erneuter Aufruf (Re-Deploy-Simulation)
  erzeugt **keinen** Duplikat-Eintrag (weiterhin 2 Zeilen). Hinweis: ein erster Test mit einer
  **simulierten** `crontab`-Shell-Funktion (liest/schreibt direkt eine Datei statt des echten
  Spool-Mechanismus) zeigte einen Datenverlust des vorhandenen Eintrags — das war ein Artefakt des
  unvollständigen Test-Doubles (naives `cat > Datei` im Downstream einer Pipe race't gegen das
  Upstream-`cat < Datei`, weil beide Pipeline-Glieder parallel starten), **kein Bug im echten
  `crontab`-Kommando**, das erst den kompletten stdin puffert und dann atomar installiert. Gegen
  das echte Kommando verhält sich das Idiom korrekt.
- **Nicht ausgeführt (Auftragsvorgabe):** `cdk deploy`, echter `aws s3 cp`/`aws ssm
  get-parameter` gegen ein reales AWS-Konto.

**Doku:** `docs/adr/019-deployment-strategy.md` (Zeile "Audit Store Backup-Strategie" → ✅),
`DECISIONS.MD` (Zeile "Backup-Strategie Audit Store" → ✅), `tasks/backlog.md` (EPIC-Überschrift +
alle vier Tasks + "Offene Entscheidungen"-Zeile → ✅), `docs/deployment/ec2-bootstrap.md`
(Verzeichnisbaum + `scp`-Liste ergänzt).

**Geänderte/neue Dateien:**
- `infrastructure/AWS/cdk_server-stack.ts` (geändert)
- `scripts/backup-audit-store.sh` (neu)
- `scripts/deploy.sh` (geändert)
- `docs/deployment/ec2-bootstrap.md` (geändert)
- `docs/adr/019-deployment-strategy.md` (geändert)
- `DECISIONS.MD` (geändert)
- `tasks/backlog.md` (geändert)

**Bewusst nicht getestet / offen:** Restore-Pfad (`gunzip | psql`) wurde nicht end-to-end gegen
eine leere Datenbank durchgespielt (kein automatisierter Restore-Test ist Teil dieses Sprints,
siehe "Nicht Teil dieses Sprints"). Reale S3-Anbindung (IAM-Berechtigung, SSM-Parameter in Prod,
Lifecycle-Regel-Wirksamkeit) ist erst nach einem echten `cdk deploy` prüfbar — nicht Teil dieses
Sprints (siehe Auftragsvorgabe, kein `cdk deploy` ohne ausdrückliche Freigabe).
