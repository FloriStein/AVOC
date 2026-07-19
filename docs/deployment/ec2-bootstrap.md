# EC2 Bootstrap Guide — AVOC auf AWS EC2

Diese Anleitung richtet sich an jemanden, der dieses Repository frisch klont und den kompletten
Stack (Frontend, Control Server, Auth/Safety/Telemetry-Services, MediaMTX WHIP/WHEP, coturn,
PostgreSQL, MQTT, Grafana/Loki) auf einer **eigenen** AWS-EC2-Instanz zum Laufen bringen will.

Sie beschreibt den tatsächlich funktionierenden Weg (validiert am 2026-07-09 gegen eine echte
laufende Instanz): Images werden **lokal gebaut und per `docker save`/`scp`/`docker load` auf die
Instanz übertragen** — keine Docker-Hub-Registry nötig. Wer stattdessen über eine Docker-Hub-
Registry deployen möchte (`make push`, siehe [Makefile](../../Makefile)), kann das als Ausgangspunkt
nehmen — `deploy.sh` liest `DOCKER_USERNAME`/`DOCKER_PASSWORD` aktuell **nicht** aus SSM und müsste
dafür um einen `docker compose pull`-Schritt ergänzt werden (siehe "Abweichung" in Schritt 2 unten).

**Architekturhintergrund:** [README.md](../../README.md), [docs/architecture.md](../architecture.md),
[docs/adr/](../adr/). Diese Datei behandelt nur das *Deployment*, keine Architekturentscheidungen.

Voraussetzung: CDK Stack ist deployed (`cdk deploy`), Elastic IP ist bekannt.

---

## Überblick

```
Dev-Rechner                              EC2-Instanz
──────────────────                       ────────────────────────
1. cdk deploy            ──────────▶     EC2 + Security Group + IAM + Elastic IP
2. setup-ssm.(sh|ps1)    ──────────▶     SSM Parameter Store (/avoc/prod/*)
3. Config-Dateien scp'en ──────────▶     ~/app/, ~/loki/, ~/promtail/, ~/grafana/
4. .env anlegen           ──────────▶     ~/app/.env (DB_PASSWORD, ADMIN_PASSWORD)
5. Images lokal bauen,
   docker save + scp     ──────────▶     ~/app/avoc-images.tar.gz
6. deploy.sh ausführen    ──────────▶     Stack läuft (14 Container)
```

---

## Voraussetzungen

**Lokal (Dev-Rechner):**
- Docker Desktop (mit `buildx`) — zum Bauen der Images
- Go 1.23+ — nur falls lokal getestet/gebaut werden soll (`go build ./...`)
- AWS CLI v2, konfiguriert mit einem Account, der EC2/IAM/S3/SSM anlegen darf
- Node.js/CDK CLI (`npm install -g aws-cdk`), falls die Infrastruktur per CDK provisioniert wird
- SSH-Client + ein Key Pair für den Zugriff auf die Instanz

**AWS-Account:**
- Berechtigung, den CDK-Stack zu deployen (VPC, EC2, Security Group, IAM Role, S3 Bucket, SSM Parameter)

---

## Schritt 1 — EC2-Instanz per CDK bereitstellen

Der Stack liegt als reine Construct-Datei unter [infrastructure/AWS/cdk_server-stack.ts](../../infrastructure/AWS/cdk_server-stack.ts)
und muss in eine eigene CDK-App eingebunden werden (kein `cdk.json`/`package.json` im Repo enthalten):

```bash
mkdir avoc-infra && cd avoc-infra
cdk init app --language typescript
# cdk_server-stack.ts nach lib/ kopieren und in bin/*.ts instanziieren:
#   new StreamingStack(app, "StreamingStack");
cdk deploy
```

Der Stack legt an:
- VPC (2 AZs, kein NAT Gateway — Kostenoptimierung)
- Security Group mit den Ports, die AVOC braucht (siehe Tabelle unten)
- `t3.micro` EC2-Instanz, Amazon Linux 2023, mit Docker + Docker Compose Plugin vorinstalliert (UserData)
- Zwei Linux-User auf der Instanz: `ec2-user` (AWS-Standard) und `ec2-admin` (eigens angelegt, gleicher SSH-Key, Docker-Gruppe, passwortloses `sudo`)
- IAM Instance Profile mit Lesezugriff auf `ssm:GetParameter*` unter `/avoc/*` und Read/Write auf den erzeugten S3-Bucket
- Elastic IP, fest mit der Instanz verknüpft

> **Hinweis:** `t3.micro` (1 GB RAM) ist knapp für 14 Container. Für stabilen
> Betrieb `ec2.InstanceSize.SMALL` oder `MEDIUM` verwenden — Achtung, das erzwingt
> ein CloudFormation-Instance-Replacement (Elastic IP bleibt aber gleich).

Nach dem Deploy aus dem CloudFormation-Output notieren:
```
StreamingStack.PublicIP     = 1.2.3.4     ← Elastic IP
StreamingStack.InstanceId   = i-0abc123
StreamingStack.BucketName   = streamingstack-appbucket-xyz
StreamingStack.KeyPairId    = key-0abc...
```

Privaten SSH-Key aus SSM holen (wurde beim `cdk deploy` automatisch erzeugt):
```bash
aws ssm get-parameter --name /ec2/keypair/<KeyPairId> --with-decryption \
  --query Parameter.Value --output text > avoc-ec2-key.pem
chmod 600 avoc-ec2-key.pem
```

**Security-Group-Ports (bereits im CDK-Stack enthalten):**

| Port | Protokoll | Zweck |
|---|---|---|
| 22 | TCP | SSH |
| 3000 | TCP | Frontend (nginx) |
| 8080 | TCP | Control Server (Operator-WS, Vehicle-WS, REST) |
| 1883 | TCP | MQTT Broker (Vehicle-Telemetrie, **ohne Auth** — in Produktion einschränken) |
| 3478 | TCP+UDP | coturn STUN/TURN |
| 49152–65535 | UDP | coturn TURN-Relay-Range |
| 10000–10050 | UDP | WebRTC-SFU RTP (aktuell ungenutzt, Video läuft über MediaMTX) |
| 8889 | TCP | MediaMTX WHIP/WHEP Signaling |
| 8189 | UDP | MediaMTX ICE-Mux |
| 3001 | TCP | Grafana (**in Produktion auf eigene IP einschränken**) |
| 80/443 | TCP | reserviert für Reverse Proxy / HTTPS |

Kein Eintrag für Port 8085 (Fleet Service) — `fleet-service` ist zwar Teil des Dev-Stacks, aber
(Stand jetzt) nicht in `docker-compose.prod.yml` eingetragen und läuft daher auf keiner per diese
Anleitung aufgesetzten Instanz, siehe die Lücke bei "Services & zugehöriger Quellcode" unten.

---

## Schritt 2 — SSM Parameter anlegen (einmalig vom Dev-Rechner)

```bash
AWS_REGION=eu-central-1 bash scripts/setup-ssm.sh
# Windows: .\scripts\setup-ssm.ps1
```

Fragt interaktiv nach allen Werten und legt sie unter `/avoc/prod/*` an:

| SSM-Pfad | Inhalt |
|---|---|
| `/avoc/prod/jwt-secret` | JWT Signing Secret (≥32 Zeichen) |
| `/avoc/prod/whip-stream-key` | MediaMTX WHIP Bearer Token (≥32 Zeichen, ADR-020) |
| `/avoc/prod/turn-external-ip` | Elastic IP (z. B. `1.2.3.4`) |
| `/avoc/prod/turn-realm` | TURN Realm (z. B. `avoc.example.com`) |
| `/avoc/prod/turn-user` | TURN Benutzername |
| `/avoc/prod/turn-password` | TURN Passwort (≥16 Zeichen) |
| `/avoc/prod/grafana-admin-user` | Grafana Login |
| `/avoc/prod/grafana-admin-password` | Grafana Passwort (≥12 Zeichen) |
| `/avoc/prod/docker-username` | Docker Hub Benutzername (siehe Abweichung unten) |
| `/avoc/prod/docker-password` | Docker Hub Access Token (siehe Abweichung unten) |

### Abweichung: keine Docker-Hub-Registry

`setup-ssm.sh` fragt zusätzlich nach `DB_PASSWORD`, `ADMIN_PASSWORD`, `DOCKER_USERNAME` und
`DOCKER_PASSWORD` und legt sie in SSM ab — `scripts/deploy.sh` liest diese vier Parameter aber
**nicht** aus SSM:

- `DB_PASSWORD` und `ADMIN_PASSWORD` müssen stattdessen in einer lokalen `.env`-Datei **auf der
  Instanz** liegen (`docker compose` liest sie automatisch aus dem Arbeitsverzeichnis) — siehe
  Schritt 5.
- `DOCKER_USERNAME`/`DOCKER_PASSWORD` werden nur für den alternativen Docker-Hub-Weg (Bauen →
  Docker Hub → `docker compose pull` auf der Instanz, `make push` im [Makefile](../../Makefile))
  gebraucht. Diese Anleitung nutzt stattdessen **lokal bauen → `docker save` → `scp` →
  `docker load`** (siehe `docker-compose.prod.yml`-Kopfkommentar: "Übergabe-Abweichung von
  ADR-019 — kein Docker Hub"). Das spart eine Docker-Hub-Registrierung; wer lieber über eine
  Registry deployen will, kann `make push` verwenden und `deploy.sh` entsprechend um einen
  `docker compose pull` ergänzen.

Parameter prüfen:
```bash
aws ssm get-parameters-by-path --path /avoc/prod/ --region eu-central-1 \
  --query "Parameters[].Name" --output table
```

---

## Schritt 3 — Docker Compose Plugin auf EC2 (nur bei bestehenden Instanzen)

Das UserData-Script im CDK installiert das Plugin automatisch bei **neuen** Instanzen. Bei
bereits laufenden, älteren Instanzen einmalig manuell prüfen/nachinstallieren (via SSH oder AWS
SSM Session Manager):

```bash
aws ssm start-session --target i-0abc123 --region eu-central-1
# oder: ssh -i "$KEY" ec2-user@${ELASTIC_IP}
```

Im Session-Terminal:
```bash
docker compose version
# → Docker Compose version v2.x.x — falls fehlend:
sudo mkdir -p /usr/local/lib/docker/cli-plugins
sudo curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
```

---

## Schritt 4 — Verzeichnisstruktur + Config-Dateien auf die Instanz kopieren

`deploy.sh` und `docker-compose.prod.yml` erwarten diese Struktur (Pfade relativ
zu `docker-compose.prod.yml`, das in `~/app/` liegt):

```
~/app/
├── docker-compose.prod.yml
├── deploy.sh
├── .env                              # Schritt 5 — DB_PASSWORD, ADMIN_PASSWORD
├── mediamtx/mediamtx.yml
└── mosquitto/mosquitto.conf
~/loki/loki.yml
~/promtail/promtail.yml
~/grafana/provisioning/
├── datasources/loki.yml
└── dashboards/
    ├── dashboards.yml
    └── avoc.json
```

> **Wichtig:** `loki/` und `promtail/` müssen **außerhalb** von `~/app/` liegen
> (nicht `~/app/loki/`), weil `docker-compose.prod.yml` sie per `../loki/loki.yml`
> referenziert. Die Zieldateien müssen **vor** dem ersten `docker compose up`
> existieren — sonst legt Docker beim Bind-Mount ein leeres Verzeichnis statt
> einer Datei an, und der Container crasht mit `read ...: is a directory`.

```bash
ELASTIC_IP=1.2.3.4
KEY=avoc-ec2-key.pem
REMOTE_USER=ec2-user   # oder ec2-admin, siehe Hinweis unten

ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} \
  "mkdir -p ~/app/mediamtx ~/app/mosquitto ~/loki ~/promtail ~/grafana/provisioning/datasources ~/grafana/provisioning/dashboards"

scp -i "$KEY" scripts/deploy.sh                                        ${REMOTE_USER}@${ELASTIC_IP}:~/app/
scp -i "$KEY" infrastructure/compose/docker-compose.prod.yml           ${REMOTE_USER}@${ELASTIC_IP}:~/app/
scp -i "$KEY" infrastructure/mediamtx/mediamtx.yml                     ${REMOTE_USER}@${ELASTIC_IP}:~/app/mediamtx/
scp -i "$KEY" infrastructure/mosquitto/mosquitto.conf                  ${REMOTE_USER}@${ELASTIC_IP}:~/app/mosquitto/
scp -i "$KEY" infrastructure/loki/loki.yml                             ${REMOTE_USER}@${ELASTIC_IP}:~/loki/loki.yml
scp -i "$KEY" infrastructure/promtail/promtail.yml                     ${REMOTE_USER}@${ELASTIC_IP}:~/promtail/promtail.yml
scp -i "$KEY" infrastructure/grafana/provisioning/datasources/loki.yml ${REMOTE_USER}@${ELASTIC_IP}:~/grafana/provisioning/datasources/
scp -i "$KEY" infrastructure/grafana/provisioning/dashboards/dashboards.yml \
              infrastructure/grafana/provisioning/dashboards/avoc.json ${REMOTE_USER}@${ELASTIC_IP}:~/grafana/provisioning/dashboards/
```

> **`ec2-user` vs. `ec2-admin`:** Das UserData-Script im CDK-Stack legt `ec2-admin`
> zusätzlich zum AWS-Standard-User `ec2-user` an und kopiert denselben SSH-Key auf
> beide. UserData läuft aber nur beim **ersten** Boot einer Instanz — bei einer
> bereits laufenden, älteren Instanz existiert `ec2-admin` u.U. nicht. Diese
> Anleitung wurde gegen `ec2-user` mit App-Verzeichnis `/home/ec2-user/app`
> validiert; beide funktionieren identisch, solange `REMOTE_USER` und die Pfade
> konsistent gewählt werden.

---

## Schritt 5 — `.env` auf der Instanz anlegen

```bash
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} "cat > ~/app/.env" <<'EOF'
DB_PASSWORD=<mindestens-16-zeichen>
ADMIN_PASSWORD=<mindestens-12-zeichen>
EOF
```

Diese Datei wird **nicht** eingecheckt (`.gitignore`) und liest `docker compose`
automatisch aus dem Arbeitsverzeichnis von `deploy.sh` (`cd ~/app && docker
compose ... up`).

---

## Schritt 6 — Images lokal bauen und auf die Instanz übertragen

Alle 8 Images für `linux/amd64` bauen (Docker Desktop muss laufen; `GO_SERVICES` hier synchron zu
`Makefile`s `GO_SERVICES`-Variable halten):

```bash
GO_SERVICES="control-server auth-service safety-service telemetry-service webrtc-sfu fleet-service"
for svc in $GO_SERVICES; do
  docker buildx build --platform linux/amd64 --build-arg SERVICE_NAME=$svc \
    -t "avoc-$svc:latest" -f infrastructure/docker/go-service.Dockerfile . --load
done
docker buildx build --platform linux/amd64 -t avoc-vehicle-mock:latest \
  -f infrastructure/docker/vehicle-mock.Dockerfile . --load
docker buildx build --platform linux/amd64 -t avoc-frontend:latest \
  -f infrastructure/docker/frontend.Dockerfile . --load
```

> **Bekannte Lücke:** `fleet-service` wird hier mitgebaut (`make build-prod`/`make push` bauen es
> ebenfalls, `Makefile`s `GO_SERVICES`), ist aber **nicht** in `docker-compose.prod.yml` als
> Service eingetragen — das Fleet-Dashboard-Backend läuft aktuell nicht auf einem per diese
> Anleitung aufgesetzten Produktiv-Server, obwohl das Fleet-Dashboard laut `docs/milestones/
> meilenstein-2-dashboard.md` die primäre Post-Login-Ansicht ist. Nachziehen von
> `docker-compose.prod.yml` (Service-Eintrag analog zum Dev-Compose, Port 8085, eigener
> Postgres-Pool auf `avoc`) ist ein offener Folge-Task.

Als Tar-Archiv verpacken und übertragen:

```bash
docker save -o avoc-images.tar \
  avoc-control-server:latest avoc-auth-service:latest avoc-safety-service:latest \
  avoc-telemetry-service:latest avoc-webrtc-sfu:latest avoc-vehicle-mock:latest avoc-frontend:latest
gzip avoc-images.tar

scp -i "$KEY" avoc-images.tar.gz ${REMOTE_USER}@${ELASTIC_IP}:~/app/avoc-images.tar.gz

ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} \
  "cd ~/app && gunzip -c avoc-images.tar.gz | docker load && rm avoc-images.tar.gz"
```

**Alternative ohne direkten SSH-Zugriff (Windows, S3 + SSM Run Command statt scp):**
```powershell
.\scripts\deploy-images.ps1 -InstanceId i-0abc123 -Bucket streamingstack-appbucket-xyz -RemoteUser ec2-user
```
Baut identisch, lädt aber über den vom CDK-Stack erzeugten S3-Bucket per SSM
Run Command hoch — kein offener Port 22 nötig, dafür IAM-Rechte für
`s3:PutObject` + `ssm:SendCommand` auf dem Dev-Rechner erforderlich.

---

## Schritt 7 — `deploy.sh` ausführen

```bash
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} \
  "cd ~/app && AWS_REGION=eu-central-1 bash deploy.sh"
```

`deploy.sh`:
1. Holt `JWT_SECRET`, `WHIP_STREAM_KEY`, `TURN_EXTERNAL_IP`, `TURN_REALM`, `TURN_USER`,
   `TURN_PASSWORD`, `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD` aus SSM
   (`DB_PASSWORD`/`ADMIN_PASSWORD` kommen aus `.env`, siehe Schritt 5)
2. Ermittelt `TURN_PRIVATE_IP` automatisch über den EC2 Instance Metadata Service
   (IMDSv2 mit Token-Header — IMDSv1 liefert einen leeren Wert)
3. Generiert einmalig ein selbstsigniertes SSL-Zertifikat (`~/app/ssl/`) für
   HTTPS — Browser blockieren `getUserMedia`/WebRTC sonst auf HTTP
4. Prüft, ob alle benötigten Images lokal vorhanden sind
5. `docker compose -f docker-compose.prod.yml up -d`

Fertig, wenn alle Container laufen:
```bash
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} "cd ~/app && docker compose -f docker-compose.prod.yml ps"
```
Erwartet: 14 Container, `postgres` mit Status `(healthy)`, alle anderen `Up`.

---

## Verifikation

| Prüfung | Befehl / URL |
|---|---|
| Frontend erreichbar | `https://<ELASTIC_IP>:443` (Zertifikatswarnung erwartet — selbstsigniert) |
| Control-Server-Health | `curl http://<ELASTIC_IP>:8080/health` |
| Fahrzeug registriert | `curl http://<ELASTIC_IP>:8080/vehicles` → `vehicle-001`, `"online":true` |
| MediaMTX WebRTC-Host-Candidate | `curl http://localhost:9997/v3/config/global/get` (nur auf der Instanz, Port 9997 ist nicht öffentlich) → `"webrtcAdditionalHosts":["<ELASTIC_IP>"]` |
| Grafana | `http://<ELASTIC_IP>:3001`, Login aus SSM (`grafana-admin-user`/`-password`) |
| Container-Logs | `docker logs <container-name>` bzw. via SSH wie oben |

Im Browser: Login → CONNECTION → Fahrzeug aus dem Dropdown wählen → Session
starten → Videostream sollte innerhalb weniger Sekunden verbinden (ICE-Handshake).

---

## Erreichbare Services nach Deploy (Kurzreferenz)

| Service | URL |
|---|---|
| Operator UI (Frontend) | `https://<ELASTIC_IP>:443` |
| Control Server API | `http://<ELASTIC_IP>:8080` |
| MQTT Broker (Vehicle) | `<ELASTIC_IP>:1883` |
| Grafana | `http://<ELASTIC_IP>:3001` |
| STUN/TURN | `<ELASTIC_IP>:3478` |

---

## Services & zugehöriger Quellcode

Sieben Go-Binaries unter `cmd/`, jedes ein eigenes `main.go` plus die
`internal/`-Pakete, die es importiert:

| Service (`cmd/`) | Verwendete `internal/`- und `pkg/`-Pakete | Aufgabe |
|---|---|---|
| **control-server** | `controlserver/{command,session,statemachine,transport,vehiclecontext,safety}`, `mediamtx`, `recording`, `vehicleconnection`, `vehicleregistry`, `pkg/{audit,db,logger,ulid}` | Control Hub: 4-Layer State Machine, Safety Engine, Session-GSA, Handover, MediaMTX-Auth |
| **auth-service** | `internal/authservice`, `pkg/{db,logger}` | JWT-Login, Nutzerverwaltung, bcrypt |
| **safety-service** | `internal/safetyservice` (nur `bus.go`), `pkg/logger` | Safety Event Bus (In-Memory) |
| **telemetry-service** | `internal/telemetryservice` (nur `client.go`), `pkg/logger` | MQTT-Telemetrie-Weiterleitung |
| **webrtc-sfu** | `internal/webrtcsfu` (nur `sfu.go`), `pkg/logger` | Passiver Session-Event-Consumer (Pion) |
| **fleet-service** | `internal/{fleetservice,fleetgateway}`, `pkg/{db,logger}` | Fleet-Domäne: Zonen/Stationen/Tasks/Alerts/Live-Status (ADR-027/028/029/031) — **nicht in `docker-compose.prod.yml`, siehe Lücke in Schritt 6** |
| **vehicle-mock** | `pkg/{logger,ulid}`, `gen/go/{common,control,telemetry,vehicle}/v1` | Fahrzeug-Simulator für Dev/Test |

Geteilte Bausteine, die mehrere Services betreffen können:
- **`pkg/db`** (PostgreSQL, inkl. `WaitForReady`-Retry) → betrifft `control-server` **und** `auth-service`
- **`pkg/audit`**, **`pkg/logger`**, **`pkg/ulid`** → projektweit
- **`gen/go/`** — generierter Protobuf-Code aus `proto/*.proto` (gitignored, wird beim Docker-Build neu erzeugt)
- **`frontend/`** (React/TS/Vite) — komplett getrennt, kein Go, eigenes Image (`frontend.Dockerfile`)

### Was tun, wenn eine Quellcode-Datei geändert wird

**1. Betroffene(n) Service(s) identifizieren** — anhand obiger Tabelle. Ändert
man z. B. `internal/controlserver/safety/detector.go`, ist nur `control-server`
betroffen; ändert man `pkg/logger`, potenziell **alle sechs**.

**2. Lokal verifizieren** (vor jedem Deploy Pflicht):
```bash
go build ./...
go vet ./...
go test ./...              # bzw. gezielt: go test ./tests/unit/... -run Safety
```

**3. Image(s) neu bauen.** Das `go-service.Dockerfile` (`ARG SERVICE_NAME`)
kopiert bei jedem Build das **gesamte Repo** in den Build-Context und
kompiliert `./cmd/${SERVICE_NAME}` neu — das passiert technisch für jedes
Image, unabhängig davon, ob es den geänderten Code überhaupt importiert:
```bash
docker buildx build --platform linux/amd64 --build-arg SERVICE_NAME=<svc> \
  -t "avoc-<svc>:latest" -f infrastructure/docker/go-service.Dockerfile . --load
```
Praktisch macht das nichts — Go kompiliert ein einzelnes Binary in 3–5s, und
wenn ein Service den geänderten Code gar nicht importiert, ist das kompilierte
Binary bit-identisch zu vorher. BuildKit erkennt das über Content-Hashing und
vergibt denselben Image-Digest → `docker load` meldet für diese Services
"already exists", und `docker compose up -d` recreated **nur** die Container,
deren Digest sich tatsächlich geändert hat. Deshalb ist es unproblematisch, im
Zweifel immer alle 7 Images neu zu bauen (siehe Schritt 6), statt selbst
nachzuverfolgen, was betroffen ist.

**4. Auf die Instanz bringen und ausrollen** — Schritt 6 + 7 dieser Anleitung
bzw. verkürzt:
```bash
docker save -o avoc-images.tar avoc-<svc1>:latest avoc-<svc2>:latest ...
gzip avoc-images.tar
scp -i "$KEY" avoc-images.tar.gz ${REMOTE_USER}@${ELASTIC_IP}:~/app/
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} "cd ~/app && gunzip -c avoc-images.tar.gz | docker load && rm avoc-images.tar.gz"
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} "cd ~/app && AWS_REGION=eu-central-1 bash deploy.sh"
```

**Sonderfälle:**
- **`.proto`-Änderung:** vorher `make proto-gen` (Go) bzw. `make proto-gen-ts`
  (Frontend) laufen lassen — `gen/` ist gitignored, wird sonst nicht neu generiert.
- **Reine Config-Änderung ohne Code** (z. B. `mediamtx.yml`,
  `docker-compose.prod.yml`, `mosquitto.conf`): kein Image-Rebuild nötig, nur
  die Datei per `scp` auf die Instanz kopieren und `deploy.sh` erneut
  ausführen — genau der Weg aus dem Troubleshooting-Beispiel weiter unten.
- **Frontend-Änderung:** eigenes Image (`infrastructure/docker/frontend.Dockerfile`),
  unabhängig von den Go-Services, gleicher Build/Save/Scp/Load-Ablauf.

---

## Updates einspielen (Redeploy)

Nach Code-Änderungen — nur betroffene Services neu bauen (siehe oben, Abschnitt
"Was tun, wenn eine Quellcode-Datei geändert wird"). Der Digest-Vergleich beim
`docker load` bzw. `docker compose up -d` sorgt dafür, dass nur Container mit
tatsächlich geändertem Image/geänderter Config neu erstellt werden:

```bash
# 1. Alle 7 Images neu bauen (siehe Schritt 6) — Build-Cache macht das schnell
# 2. docker save + scp + docker load (siehe Schritt 6)
# 3. Bei Config-Änderungen (docker-compose.prod.yml, mediamtx.yml, ...): erneut scp'en
# 4. deploy.sh erneut ausführen (siehe Schritt 7) — recreated nur geänderte Container
```

Reines Konfigurations-Update ohne Image-Rebuild (z. B. `mediamtx.yml`):
```bash
scp -i "$KEY" infrastructure/mediamtx/mediamtx.yml ${REMOTE_USER}@${ELASTIC_IP}:~/app/mediamtx/
ssh -i "$KEY" ${REMOTE_USER}@${ELASTIC_IP} "cd ~/app && AWS_REGION=eu-central-1 bash deploy.sh"
```

**Rollback:** altes Image-Tag erneut laden und mit `VERSION=<alter-tag>` deployen,
falls mit versionierten Tags statt `latest` gearbeitet wird. Ohne separates Tag-Archiv genügt:
```bash
# Auf EC2, altes Image-Tar erneut laden (falls lokal aufbewahrt), dann:
cd ~/app
VERSION=<alter-tag> AWS_REGION=eu-central-1 bash deploy.sh
```

---

## Troubleshooting

**Container startet nicht:**
```bash
docker logs <container-name>
docker compose -f docker-compose.prod.yml ps
```

**Video verbindet nicht ("ICE-Verbindung fehlgeschlagen"):**
Prüfen, ob MediaMTX einen erreichbaren Host-Candidate annonciert:
```bash
curl -s http://localhost:9997/v3/config/global/get | grep webrtcAdditionalHosts
# erwartet: ["<ELASTIC_IP>"], NICHT ["$TURN_EXTERNAL_IP"] oder []
```
MediaMTX interpoliert kein `$VAR` innerhalb der YAML — die IP muss über die
Environment-Variable `MTX_WEBRTCADDITIONALHOSTS` im `mediamtx`-Service in
`docker-compose.prod.yml` gesetzt werden (siehe Kommentar in
[mediamtx.yml](../../infrastructure/mediamtx/mediamtx.yml)). Zusätzlich `docker logs
avoc-mediamtx-1` auf `WAR [WebRTC] cannot resolve additional host` prüfen.

**Fahrzeug fehlt im Dropdown / `GET /vehicles` liefert `[]`:**
`control-server` ist wahrscheinlich per Docker-`restart: unless-stopped`-Policy
neu gestartet, bevor PostgreSQL bereit war — diese Policy respektiert (anders
als `docker compose up`) kein `depends_on: service_healthy`. Seit dem
DB-Retry-Fix (`pkg/db.WaitForReady`, ~20s Backoff) sollte das nicht mehr
dauerhaft passieren; falls doch (z. B. Postgres braucht länger als 20s):
```bash
docker logs avoc-control-server-1 | grep -i "vehicle registry\|audit store"
docker restart avoc-control-server-1
curl http://localhost:8080/vehicles
```

**`TURN_PRIVATE_IP` leer / coturn TURN-Relay funktioniert nicht:**
IMDSv2 erfordert den Token-Header — bei leerem Wert startet coturn ohne
`relay-ip`. Prüfen:
```bash
_TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
curl -sf -H "X-aws-ec2-metadata-token: ${_TOKEN}" http://169.254.169.254/latest/meta-data/local-ipv4
```
Zusätzlich: Security Group muss Port 3478 (TCP+UDP) und UDP 49152–65535 offen haben; `coturn`
läuft mit `network_mode: host` in `docker-compose.prod.yml` (kein Port-Mapping für die Relay-Ports).

**SSM Parameter nicht lesbar:**
```bash
aws sts get-caller-identity   # auf der Instanz — muss die EC2-Instance-Rolle zeigen, nicht den User
```

**Grafana Login schlägt fehl:**
```bash
aws ssm get-parameter --name /avoc/prod/grafana-admin-user --region eu-central-1 \
  --query Parameter.Value --output text
```

**502 Bad Gateway nach Frontend-Neustart:** nginx cached Docker-IPs beim Start.
```bash
docker exec avoc-frontend-1 nginx -s reload
```

**Speicher knapp (`t3.micro`/`t3.small`):**
```bash
free -h
docker stats --no-stream
```
Bei dauerhaft >80 % RAM: Instance-Size im CDK-Stack erhöhen (`InstanceSize.MEDIUM`),
erzwingt Instance-Replacement (Elastic IP bleibt erhalten).

---

## Referenzen

- [docs/deployment/hetzner-setup.md](hetzner-setup.md) — alternative Deployment-Zielumgebung (Hetzner Cloud statt AWS)
- [infrastructure/compose/docker-compose.prod.yml](../../infrastructure/compose/docker-compose.prod.yml) — Quelle der Wahrheit für Services/Ports/Env-Vars
- [scripts/deploy.sh](../../scripts/deploy.sh) — Quelle der Wahrheit dafür, welche Secrets tatsächlich gebraucht werden
- [docs/adr/](../adr/) — Architekturentscheidungen (ADR-014 TURN, ADR-019 Deployment, ADR-020 MediaMTX, ADR-023 PostgreSQL)
