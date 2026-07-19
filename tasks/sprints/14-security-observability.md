# Sprint 14 — Security & Observability

Ziel: REST-Endpoints JWT-geschützt. Control- und Video-Kanal werden separat mit Status + Latenz angezeigt. Frontend signalisiert wenn das Backend nicht erreichbar ist.

Datum: 2026-06-13 | Abgeschlossen (AUTH-01/UI-01/ROB-01): 2026-06-13 | **Status: Auth/UI/ROB fertig ✅ · OBS-01 offen 🔲 (nachgeholt in Sprint 31, siehe `tasks/sprints/31-ap2-vervollstaendigung.md`)**
Vorgänger: Sprint 13 ✅ (Dev-Stack Stabilisierung & Log-Korrelation)

### Nachtrag (Bugfix vor Sprint-Start)
- **E-Stop Race Condition**: `WSClient.disconnect()` setzt `ws.onclose = null` vor `ws.close()` — verhindert unbeabsichtigten Reconnect bei absichtlichem Disconnect (Emergency Stop, Session End)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUTH-01 | JWT-Pflicht auf REST-Endpoints im control-server | M | ✅ |
| UI-01 | Dual-Channel Status: Control + Video separat mit Latenz in ConnectionPanel | M | ✅ |
| ROB-01 | Backend-nicht-erreichbar-Zustand im Frontend (Banner + Zustandsschutz) | S | ✅ |
| OBS-01 | Vehicle "zuletzt gesehen" Heartbeat-Timestamp (Bonus, wenn Zeit bleibt) | S | 🔲 (Sprint 31 nachgeholt) |

---

## Scope-Details

### AUTH-01 — JWT-Pflicht REST-Endpoints ✅
- `requireJWT(secret []byte)` Middleware in `cmd/control-server/main.go` (curried `http.HandlerFunc`-Wrapper)
- **Geschützt (9 Endpoints):** `POST /session/start`, `POST /session/end`, `POST /handover/request`, `POST /handover/confirm`, `POST /handover/cancel`, `POST /media/event`, `POST /emergency-stop`, `GET /audit/events`, `GET /recording/`
- **Bewusst offen:** `GET /state`, `GET /health`, `GET /vehicles`, `GET /ice-config`, `GET /vehicle/ack/latest/{id}`, `POST /log` (Fire-and-forget Logger, muss auch vor Login feuern können)
- Frontend: `Authorization: Bearer <token>` in `api-client.ts` für `startSession`, `endSession`, `emergencyStop`, `reportMediaState`
- `SafetyPanel.tsx` erhält `token: string | null` Prop aus `App.tsx`

### UI-01 — Dual-Channel Status ✅
- `useWebRTC.ts`: `RTCPeerConnection.getStats()` alle 1s, `candidate-pair` mit `r.nominated === true` → `currentRoundTripTime × 1000` → `videoLatencyMs`
- `VideoPanel.tsx`: `onVideoLatency?: (ms: number | null) => void` Callback-Prop
- `ConnectionPanel`: zwei Zeilen — **Control** (WS-ACK-RTT) + **Video** (ICE-RTT); `— ms` solange kein Stream aktiv
- `App.tsx`: `useState<number | null>(null)` für `videoLatency`, weitergegeben über VideoPanel-Callback

### ROB-01 — Backend nicht erreichbar ✅
- `useSystemState.ts`: `failCount` Ref + `UNREACHABLE_THRESHOLD = 3` (1,5s) → `unreachable: boolean` im Return
- `App.tsx`: rotes Banner bei `state.unreachable`; `ControlPanel` disabled wenn `isUnreachable`
- Verhindert dass Operator glaubt zu steuern während Backend tot ist

### OBS-01 — Vehicle Heartbeat (Bonus) 🔲
- MQTT-Telemetry kommt schon alle ~100ms — letzter Timestamp reicht
- `AckBadge` → "Fahrzeug aktiv vor 2s" auch ohne aktive Steuerbefehle
- Nicht in diesem Sprint umgesetzt (Bonus, keine Zeit) — nachgeholt in Sprint 31

---

## Neue/geänderte Dateien (AUTH-01)

- `cmd/control-server/main.go` — `requireJWT(secret []byte)` Middleware + 9 geschützte Endpoints
- `frontend/src/lib/api-client.ts` — `token`-Parameter in `startSession`, `endSession`, `emergencyStop`, `reportMediaState`
- `frontend/src/hooks/useSession.ts` — Token zu `startSession`/`endSession` durchgereicht; `resumingRef` entfernt
- `frontend/src/hooks/useWebRTC.ts` — `token` zu `reportMediaState` durchgereicht
- `frontend/src/components/SafetyPanel.tsx` — `token: string | null` Prop für `emergencyStop`
- `frontend/src/components/SafetyPanel.test.tsx` — `token={null}` in allen Render-Aufrufen
- `frontend/src/App.tsx` — `token={session.token}` an `SafetyPanel`
- `frontend/src/lib/ws-client.ts` — `disconnect()` setzt `ws.onclose = null` vor Close (Race-Condition-Fix)

## Verification (AUTH-01) — E2E PASS (2026-06-13)

**Surface:** EC2 `18.196.24.10:443`, 13 Container up, curl via SSH.

**Geschützte Endpoints (9 Endpoints ohne Token → 401):**
```
POST /session/start → 401   POST /session/end    → 401
POST /emergency-stop → 401  POST /handover/req   → 401
POST /media/event   → 401   POST /vehicles       → 401
DELETE /vehicles/x  → 401   GET /audit/events    → 401
GET /recording/x    → 401
```

**Offene Endpoints (kein Token nötig → 200):**
```
GET /state → 200   GET /health → 200   GET /vehicles → 200
GET /ice-config → 200   POST /log → 202
```

**Edge Cases fehlerhafte Token (alle → 401):**
- `"Token <jwt>"` (falsches Scheme) → 401
- `"Bearer "` (leerer Wert) → 401
- Tampered JWT payload → 401
- JWT mit anderem Secret signiert → 401
- Leerer / fehlender Authorization-Header → 401

**Business-Logik nach Auth:**
- session/start mit gültigem Token bei SAFE_MODE → Auth pass, Body: `"system must be in AUTHENTICATED state"` ✅
- POST /vehicles Duplikat mit Token → 409 ✅
- POST /vehicles malformed JSON mit Token → 400 ✅
- DELETE /vehicles nach DELETE → 404 ✅
- media/event unbekannter State → 400 ✅

**E2E Session-Lifecycle mit JWT:**
```
Login → JWT erhalten
POST /session/start (Token) → 200, session_id=01KV1EA0KTVXXP0V7SRXZXZ7PT
State → CONNECTED / ACTIVE_OPERATOR
POST /emergency-stop (Token) → 202
State → SAFE_MODE / ACTIVE_OPERATOR
POST /session/end (Token) → 204
State → SAFE_MODE / NO_OPERATOR  (wartet auf Frontend-Resume — korrekt)
```

**WSClient-Fix (Doppel-Reconnect-Race):** Kein WS-Client auf EC2 verfügbar → direkt nicht observierbar. Backend-Seiteneffekte: E-Stop → SAFE_MODE korrekt, System bleibt in SAFE_MODE bis Operator-WS-Reconnect (Frontend-Resume). Strukturell korrekt durch `ws.onclose = null` in `disconnect()`. Go Build ✅ · TypeScript ✅ · 41/41 Frontend-Tests ✅.

**⚠️ Finding (pre-existing):** `GET /audit/events?session_id=<unbekannt>` gibt `null` statt `[]` zurück — `json.Encode(nil)` auf nil-Slice. Frontend ruft diesen Endpoint nicht auf; bei späterer Audit-UI defensiv behandeln.

---

**Redaktionshinweis (MD-Konsolidierung, 2026-07-19):** Diese Datei führt zwei zuvor parallel
geführte Versionen dieses Sprints zusammen. `tasks/done.md` enthielt unter dem Titel
"Sprint 14 (Partial)" nur AUTH-01, dafür mit ausführlicher E2E-Verifikation.
`tasks/current-sprint.md` enthielt unter "Sprint 14 (Archiv)" den vollständigen Sprint (alle 4
Tasks inkl. UI-01/ROB-01/OBS-01), aber ohne die tiefe AUTH-01-Verifikation. Beide Versionen waren
komplementär — zusammengeführt auf Basis der vollständigeren `current-sprint.md`-Fassung, ergänzt
um `done.md`s AUTH-01-Verifikationsdetails. Die Endpoint-Zahl "9 Endpoints" spiegelt den
Sprint-14-Stand wider, nicht den heutigen (siehe `docs/architecture.md` für den aktuellen Stand:
13 Endpoints).
