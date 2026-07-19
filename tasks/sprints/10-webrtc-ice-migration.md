# Sprint 10 — Browser WebRTC ICE Migration

Abgeschlossen: 2026-06-10

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| WEBRTC-01 | CDK Security Group: 3478 TCP/UDP, 8189 UDP, 49152–65535 UDP | S | ✅ Via `aws ec2 authorize-security-group-ingress` (CDK deploy übersprungen wegen Subnet-AZ-Drift) |
| WEBRTC-02 | `mediamtx.yml`: `webrtcIPsFromInterfaces: false`, ICEServers2 entfernt, Port 8189 | S | ✅ Verifiziert via `/v3/config/global/get` |
| WEBRTC-03 | `docker-compose.prod.yml`: coturn `network_mode: host`, relay-ip, external-ip=PUBLIC/PRIVATE | M | ✅ `relay-ip=10.0.33.191`, `external-ip=18.196.24.10/10.0.33.191` |
| WEBRTC-04 | mediamtx UDP-Port 8889 → 8189 | S | ✅ |
| WEBRTC-05 | `deploy.sh`: `TURN_PRIVATE_IP` aus IMDS (IMDSv2 Token-Header) | S | ✅ Amazon Linux 2023 erfordert IMDSv2 |
| WEBRTC-06 | control-server: `GET /ice-config` Endpoint | M | ✅ STUN + TURN UDP + TURN TCP; nginx-Präfix-Regel beachtet |
| WEBRTC-07 | control-server env: `TURN_USER`, `TURN_PASSWORD`, `TURN_EXTERNAL_IP` | S | ✅ |
| WEBRTC-08 | `useWebRTC.ts`: DTLS-Fix (actpass→active), `/api/ice-config` fetch, 5s Gathering-Timeout | M | ✅ |
| WEBRTC-09 | Deploy auf EC2 `18.196.24.10`; alle 12 Container Up | M | ✅ Deployed, E2E Smoke Test offen |

### Neue/geänderte Dateien

- `infrastructure/mediamtx/mediamtx.yml` — Komplett neu (webrtcIPsFromInterfaces, Port 8189, kein ICEServers2)
- `infrastructure/compose/docker-compose.prod.yml` — coturn host mode, mediamtx Ports, control-server env
- `scripts/deploy.sh` — IMDSv2 TURN_PRIVATE_IP, TURN_REALM, TURN_USER, TURN_PASSWORD
- `cmd/control-server/main.go` — `GET /ice-config` Endpoint
- `frontend/src/hooks/useWebRTC.ts` — DTLS-Fix, fetchIceServers(), 5s ICE-Gathering-Timeout
- `infrastructure/AWS/cdk_server-stack.ts` — Port 3478 statt 3479, Port 8189, Relay 49152-65535
- `docs/sprints/sprint-10-webrtc-ice-migration.md` — Sprint-Dokument mit Deployment-Protokoll

### Bugs gefunden & behoben

| Bug | Ursache | Fix |
|-----|---------|-----|
| `TURN_PRIVATE_IP` leer | IMDSv1 auf Amazon Linux 2023 | IMDSv2 Token-Header |
| loki/promtail crash loop | Docker erstellte Verzeichnis statt File-Bind | Container stoppen, `rm -rf`, Config neu hochladen |
| `/api/ice-config` → 404 | Route `/api/ice-config` statt `/ice-config` (nginx strippt Präfix) | Route umbenannt |
| mediamtx startet nicht | Port 9997 durch streaming-mediamtx-1 belegt | streaming-platform gestoppt |
| Grafana Provisioning fehlt | Config-Pfade `~/grafana/provisioning` nicht angelegt | Dirs + Files hochgeladen |

### Testprotokoll (2026-06-10)

| Test | Ergebnis |
|------|---------|
| `curl http://18.196.24.10:3000/api/ice-config` | STUN + TURN UDP + TURN TCP ✅ |
| `POST /whep/vehicle-test/whep` ohne Token | HTTP 401 ✅ |
| Frontend `http://18.196.24.10:3000/` | HTTP 200 ✅ |
| coturn relay-ip / external-ip | 10.0.33.191 / 18.196.24.10/10.0.33.191 ✅ |
| mediamtx webrtcIPsFromInterfaces | false ✅ |
| Port 3478 TCP von extern | OPEN ✅ |
| Port 8889 TCP von extern | OPEN ✅ |
| 31/31 TypeScript Unit-Tests | ✅ |
| Go Unit-Tests | ✅ |
