# Sprint 19 — Lokaler Dev-Stack: Verifikation & Robustheit

Ziel: Sicherstellen, dass das Projekt zuverlässig **lokal** läuft (nicht nur auf der AWS-EC2-Instanz) — reproduzierbar auf einem frischen Checkout, mit dokumentierten Stolpersteinen und geklärter SSL/HTTPS-Frage.

Datum: 2026-07-10 | **Status: Abgeschlossen ✅ (1 Folge-Task an Backlog übergeben)**
Vorgänger: Sprint 18 (pausiert — 3 Tasks offen: AUTH-18-01, OBS-01, MV-11-ADR, siehe unten)

**Vorab-Recherche (2026-07-10):** Ein lokales Compose-Setup existiert bereits (`make up` → `infrastructure/compose/docker-compose.yml`, README-Schnellstart). Verifiziert: mit `.env` aus `.env.example` kopiert und den vorhandenen (7 Tage alten) Images startet der komplette Stack sauber durch — `frontend` (HTTP 200), `control-server /health` (ok), `vehicle-001` online, kein SSL-Fehler.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| LOCAL-01 | Sauberer Full-Rebuild-Test: `docker compose down -v` + `up --build` von Grund auf | S | ✅ |
| LOCAL-02 | `CONTEXT.MD`-Eintrag „Dev-Stack SSL-Fix" verifizieren | S | ✅ |
| LOCAL-03 | Frontend-Hot-Reload-Pfad (`npm run dev`) end-to-end verifizieren | S | ✅ |
| LOCAL-04 | WebRTC/Video-Verbindung lokal real testen (Playwright, Fake-Kamera via WHIP/WHEP) | M | ✅ (Blocker gefixt, Folge-Bug dokumentiert) |
| LOCAL-05 | README/`CONTEXT.MD` um neu gefundene Stolpersteine ergänzen | S | ✅ |

---

## Ergebnisse

**LOCAL-01 — Full-Rebuild ✅**
`docker compose down -v` + alle `avoc-*`-Images gelöscht + `up --build` von Grund auf: alle 7 Images bauen sauber, alle 14 Container gesund. Ein Sandbox-spezifisches Problem dabei gefunden und dokumentiert (README-Troubleshooting): der `buildx`-Builder-Container cacht `/etc/resolv.conf` beim Start und aktualisiert es nicht bei Netzwerkwechseln — nach langer Laufzeit zeigt es ggf. auf eine tote DNS-IP (`failed to resolve source metadata`). Fix: `docker restart buildx_buildkit_<projekt>-builder0`. Echter Zero-Cache-Rebuild (Base-Images neu von Docker Hub) in dieser Sandbox nicht vollständig verifizierbar (Registry-Zugriff über einen anderen Netzpfad als der Buildx-Builder), aber alle projekteigenen Layer wurden nachweislich neu gebaut.

**LOCAL-02 — SSL-Frage ✅ geschlossen**
Mit echtem Chromium verifiziert: `window.isSecureContext === true` und `getUserMedia()` funktionieren über `http://localhost:3000` ohne Zertifikat (Browser-Ausnahme für `localhost`). `nginx.dev.conf` (HTTP-only) deckt das bereits ab. Der `CONTEXT.MD`-Eintrag „Dev-Stack SSL-Fix … offen" war stale und wurde entfernt.

**LOCAL-03 — Hot-Reload-Pfad ✅**
`make proto-gen-ts` → `npm run dev` end-to-end getestet inkl. echtem Login-Flow über den Vite-Proxy (`/auth/operator/login` → 200 + JWT). Neues `make dev-frontend`-Target ergänzt (Makefile + README) — läuft konsistent zu allen anderen Befehlen vom Repo-Root aus, statt dass `npm run dev` root-versehentlich mit `ENOENT` fehlschlägt (realer Vorfall während des Sprints).

**LOCAL-04 — WebRTC/Video ✅ Blocker behoben, ein Folge-Bug dokumentiert**
Playwright-Test (Login → Fahrzeug wählen → Session starten → WHIP-Publish mit Fake-Kamera → WHEP-Empfang) geschrieben und schrittweise durchgetestet:
1. **Gefunden + gefixt:** `/whip/` und `/whep/` in `nginx.dev.conf` zeigten auf `http://mediamtx:8889` — aber `mediamtx` läuft mit `network_mode: host` und hat keinen Docker-DNS-Eintrag auf `avoc-net` → **502 Bad Gateway bei jedem lokalen Video-Versuch über den Docker-Frontend**. Fix: `host.docker.internal` + `extra_hosts` (analog zum bestehenden `control-server`-Pattern) + statisches `proxy_pass` (der `resolver`-Trick der anderen Locations fragt nur Docker-DNS, das kennt `host.docker.internal` nicht).
2. **Gefunden, nicht gefixt (Nutzer-Entscheidung: dokumentieren statt anfassen):** Nach dem Fix erreicht der WHIP-Publish den SDP-Austausch mit MediaMTX, scheitert dort aber an `setRemoteDescription`: *"Offerer must use actpass value for setup attribute"*. Ursache: `useWebRTC.ts`/`useWHIPSender.ts` erzwingen absichtlich `a=setup:active` im Offer (dokumentierter Pion-v1.19.0-Workaround, `docs/webrtc.md`) — aktuelles Chromium lehnt das als Spec-Verstoß ab. Da `useWebRTC.ts` (WHEP/Video-Empfang) auch produktiv auf AWS läuft, potenziell **kein reines Lokal-Problem** — braucht eigene Untersuchung, siehe `CONTEXT.MD` „Offene Fragen" und Backlog-Eintrag.

**LOCAL-05 — Doku ✅**
README: `make dev-frontend`, WHIP/WHEP-502-Troubleshooting, buildx-DNS-Troubleshooting ergänzt. `CONTEXT.MD`: SSL-Eintrag entfernt, neuer Eintrag zum SDP-`actpass`-Fund.
