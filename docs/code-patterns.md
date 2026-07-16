# Code-Patterns — Zentrale Muster im Quellcode

Dieser Abschnitt zeigt die zentralen Muster und Mechanismen anhand des echten Quellcodes. Ziel: Ein neuer Entwickler versteht, wie die Kernkomponenten zusammenspielen.

Extrahiert aus der früheren `DOKU.MD` (dort war dies Abschnitt 18) — der Rest von `DOKU.MD` war redundant zu `README.md`/`docs/architecture.md`/`CONTEXT.MD` und wurde entfernt.

---

### 1. State Machine — Transitionen und Validierung

**Datei:** [internal/controlserver/statemachine/state.go](../internal/controlserver/statemachine/state.go)

> **Update (2026-07-16):** Seit `ADR-026` existiert `*Machine` nicht mehr als ein einziges
> Prozess-Singleton, sondern eine eigene Instanz pro Fahrzeug. Erzeugung/Zugriff läuft über
> [`vehiclecontext.Registry`](../internal/controlserver/vehiclecontext/registry.go) — siehe
> Abschnitt 2 unten für das konkrete Lookup-Pattern (`registry.Get(vehicleID).SM`). Die Transition-
> Logik der `Machine` selbst (Tabelle, `TransitionSystem`, `TransitionMedia`) ist unverändert und
> unten weiterhin korrekt beschrieben.

Die State Machine ist der Safety-Kern des Systems. Transitionen sind durch eine Tabelle explizit erlaubt oder verboten — ungültige Versuche werden still verworfen.

```go
// Erlaubte Übergänge — jede nicht gelistete Kombination wird abgelehnt.
var validSystemTransitions = map[SystemState][]SystemState{
    StateIdle:          {StateConnecting},
    StateConnecting:    {StateAuthenticated, StateSafeMode},
    StateAuthenticated: {StateConnected, StateSafeMode},
    StateConnected:     {StateDegraded, StateSafeMode},
    StateDegraded:      {StateConnected, StateSafeMode},
    StateSafeMode:      {StateRecovering},
    StateRecovering:    {StateAuthenticated, StateSafeMode},
}

// TransitionSystem setzt den SYSTEM STATE und erzwingt abhängige CONTROL STATE-Regeln.
// Ungültige Transitionen werden geloggt und verworfen — kein Panic, kein Crash.
func (m *Machine) TransitionSystem(next SystemState) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !isValidTransition(m.System, next) {
        log.Printf("[STATE] invalid transition rejected: %s → %s", m.System, next)
        return  // System bleibt im aktuellen Zustand
    }

    m.System = next
    switch next {
    case StateSafeMode:
        m.Control = ControlBlocked  // SAFE_MODE erzwingt CONTROL_BLOCKED
    case StateConnected:
        m.Control = ControlActive
    }
}
```

**Wichtige Invariante für Media:** Video-Fehler dürfen niemals SAFE_MODE auslösen (ADR-009 Invariant 1):

```go
// TransitionMedia: MEDIA_FAILED → SYSTEM DEGRADED (niemals SAFE_MODE)
func (m *Machine) TransitionMedia(next MediaState) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Media = next
    if (next == MediaFailed || next == MediaDegraded) && m.System == StateConnected {
        // Absichtlich nur DEGRADED — SAFE_MODE ist hier formal verboten
        m.System = StateDegraded
    }
}
```

**Neuer Zustand aus einem bestehenden lesen (Thread-safe), heute per Fahrzeug über die Registry:**

```go
vc := vehicleContexts.Get(vehicleID)  // lazy-create, dauerhaft behalten (ADR-026)
sysState, ctrlState, mediaState, opState := vc.SM.Get()
```

---

### 2. Safety Watchdogs — Per-Vehicle über `vehiclecontext.Registry` (ADR-026)

**Dateien:** [internal/controlserver/safety/detector.go](../internal/controlserver/safety/detector.go),
[internal/controlserver/vehiclecontext/registry.go](../internal/controlserver/vehiclecontext/registry.go)

`Registry` bündelt `*statemachine.Machine`, `DeadmanWatchdog`, `ACKTimeoutWatcher` und
`VehicleACKWatchdog` in einem `VehicleContext` pro Fahrzeug (`map[vehicleID]*VehicleContext`,
`sync.Mutex`-geschützt, lazy-create beim ersten `Get()`):

```go
type VehicleContext struct {
    SM                 *statemachine.Machine
    Deadman            *csafety.DeadmanWatchdog
    ACKTimeoutWatcher  *csafety.ACKTimeoutWatcher
    VehicleACKWatchdog *csafety.VehicleACKWatchdog
}

func (r *Registry) Get(vehicleID string) *VehicleContext {
    r.mu.Lock()
    defer r.mu.Unlock()
    if ctx, ok := r.contexts[vehicleID]; ok {
        return ctx  // Instanzen bleiben für die Prozesslaufzeit bestehen — kein GC
    }
    sm := statemachine.New()
    ctx := &VehicleContext{SM: sm, Deadman: csafety.NewDeadmanWatchdog(r.deadmanTimeout, sm, r.publisher), /* ... */}
    r.contexts[vehicleID] = ctx
    return ctx
}
```

Vor `ADR-026` waren `Machine` und alle Watchdogs einzelne Prozess-Singletons — ein zweites
Fahrzeug hätte die Deadman-/ACK-Überwachung des ersten stillschweigend überschrieben. Aufrufer
(Command Engine, WS-Handler) lösen die per-Fahrzeug-Instanz bei jedem Zugriff über
`registry.Get(vehicleID)` auf, statt ein injiziertes `sm`/`deadman`-Feld zu halten.

#### DeadmanWatchdog

Der Watchdog läuft erst an, wenn der Operator den ersten `DEADMAN_HOLD` schickt (*Armed Pattern*). So verhindert er einen False Positive direkt nach dem Verbindungsaufbau.

```go
// Start: registriert die Session, startet den Timer NICHT.
// Der erste Reset()-Call bewaffnet den Watchdog.
func (w *DeadmanWatchdog) Start(sessionID, vehicleID string) { ... }

// Reset: beim ersten Aufruf → bewaffnet den Timer.
// Bei jedem weiteren Aufruf → setzt den Countdown zurück.
// Operator muss alle 1,5s einen DEADMAN_HOLD senden (Frontend-Intervall).
func (w *DeadmanWatchdog) Reset() {
    if !w.armed {
        w.armed = true
        w.timer = time.AfterFunc(w.timeout, w.fire)
        return
    }
    w.timer.Reset(w.timeout)
}

// fire: wird vom Timer aufgerufen wenn kein Reset() kam → CRITICAL
func (w *DeadmanWatchdog) fire() {
    w.sm.TransitionSystem(statemachine.StateSafeMode)
    w.publisher.PublishEvent(safetyservice.SafetyEvent{
        Type:   safetyservice.EventDeadmanTimeout,
        Reason: "dead-man switch timeout — operator released hold",
    })
}
```

#### ACKTimeoutWatcher

Jeder eingehende Command startet einen Timer. Kommt kein ACK innerhalb von 100ms → CRITICAL:

```go
// CommandReceived: startet den Per-Command-ACK-Timer
func (w *ACKTimeoutWatcher) CommandReceived(sessionID, vehicleID string) {
    w.pendingTimer = time.AfterFunc(w.timeout, w.fire)
}

// CommandACKed: bricht den Timer ab (happy path)
func (w *ACKTimeoutWatcher) CommandACKed() {
    if w.pendingTimer != nil {
        w.pendingTimer.Stop()
    }
}
```

Im Transport-Layer wird das Muster so verwendet — der Watcher kommt aus der per-Fahrzeug
`VehicleContext`, nicht aus einem Handler-Feld:

```go
// websocket.go readLoop — Wrappt jeden Command mit ACK-Tracking
vc := h.vehicleContexts.Get(sess.VehicleID)
vc.ACKTimeoutWatcher.CommandReceived(sess.ID, sess.VehicleID)
ackBytes, err = h.engine.Handle(msg, sess)
conn.WriteMessage(websocket.BinaryMessage, ackBytes)
vc.ACKTimeoutWatcher.CommandACKed()  // Timer abbrechen — alles gut
```

---

### 3. Command Engine — Protobuf Routing

**Datei:** [internal/controlserver/command/engine.go](../internal/controlserver/command/engine.go)

Der Command Engine parst eingehende Protobuf-Bytes, routet nach `CommandType` und antwortet mit einem `ControlAck`. State Machine und Watchdogs werden bei jedem Aufruf per Fahrzeug aus der `vehiclecontext.Registry` geholt (ADR-026) — es gibt kein injiziertes `e.sm`/`e.deadman`-Feld mehr, da zwei Fahrzeuge niemals Safety-State teilen dürfen. Zwei sicherheitsrelevante Zweige, die im vereinfachten Beispiel unten fehlen würden, sind bewusst mit aufgeführt: der **Audit-Eintrag vor der SAFE_MODE-Transition** (ADR-018 — Persistenz muss dem State-Wechsel vorausgehen) und der **OBSERVER-Rollenblock** für Steuerbefehle (ADR-025 — nur `ACTIVE_OPERATOR` darf STEER/THROTTLE/BRAKE/SPEED senden, `EMERGENCY_STOP` bleibt für alle Rollen erlaubt).

```go
func (e *Engine) Handle(rawMsg []byte, sess session.Session) ([]byte, error) {
    cmd := &controlv1.ControlCommand{}
    if err := proto.Unmarshal(rawMsg, cmd); err != nil {
        return e.ack(sess, "", false, "invalid protobuf message")
    }

    if !e.limiter.allow() {
        return e.ack(sess, cmd.Header.EventId, false, "rate limited")
    }

    vc := e.vehicleContexts.Get(sess.VehicleID)  // Per-Fahrzeug Lookup (ADR-026)

    switch cmd.Type {
    case controlv1.CommandType_COMMAND_TYPE_DEADMAN_HOLD:
        vc.Deadman.Reset()  // Watchdog zurücksetzen

    case controlv1.CommandType_COMMAND_TYPE_DEADMAN_RELEASE:
        // Bewusst kein Reset — Watchdog läuft ab → SAFE_MODE

    case controlv1.CommandType_COMMAND_TYPE_EMERGENCY_STOP:
        // Audit-Eintrag VOR der State-Transition schreiben (ADR-018-Invariante:
        // SAFE_MODE erst nach garantierter Persistenz). Schreibfehler blockiert
        // die Transition NICHT — Safety geht vor Audit-Vollständigkeit.
        if e.auditWriter != nil {
            sys, ctrl, _, _ := vc.SM.Get()
            if err := e.auditWriter.WriteSync(audit.SafetyAuditEvent{
                EventID: ulid.Generate(), SessionID: sess.ID, VehicleID: sess.VehicleID,
                OperatorID: sess.OperatorID, EventType: logger.EventEmergencyStop,
                Reason: "operator EMERGENCY_STOP command",
                SystemState: string(sys), CtrlState: string(ctrl), Timestamp: time.Now(),
            }); err != nil {
                svcLog.Error("audit write failed — proceeding to SAFE_MODE", "error", err)
            }
        }

        vc.SM.TransitionSystem(statemachine.StateSafeMode)
        e.safetyPub.PublishEvent(safetyservice.SafetyEvent{
            SessionID: sess.ID, VehicleID: sess.VehicleID,
            Type:   safetyservice.EventEmergencyStop,
            Reason: "operator EMERGENCY_STOP command",
        })

    case controlv1.CommandType_COMMAND_TYPE_STEER,
         controlv1.CommandType_COMMAND_TYPE_THROTTLE,
         controlv1.CommandType_COMMAND_TYPE_BRAKE,
         controlv1.CommandType_COMMAND_TYPE_SPEED:
        // OBSERVER darf keine Steuerbefehle senden (ADR-025) — EMERGENCY_STOP
        // oben ist davon bewusst ausgenommen (Safety schlägt Rollenmodell).
        if sess.OperatorRole == "OBSERVER" {
            return e.ack(sess, cmd.Header.EventId, false, "observer role: control commands not allowed")
        }
        // Weiterleitung an Fahrzeug (Vehicle Connection Layer, ADR-021)
        if e.vehicleForwarder != nil {
            if err := e.vehicleForwarder.ForwardCommand(sess.VehicleID, rawMsg); err == nil {
                vc.VehicleACKWatchdog.CommandForwarded()
            }
        }
    }

    return e.ack(sess, cmd.Header.EventId, true, "")
}
```

Der ACK enthält immer den vollständigen `CorrelationHeader` — so kann das Frontend die Antwort der richtigen Session und dem richtigen Event zuordnen:

```go
func (e *Engine) ack(sess session.Session, eventID string, success bool, errMsg string) ([]byte, error) {
    ack := &controlv1.ControlAck{
        Header: &commonv1.CorrelationHeader{
            SessionId:  sess.ID,
            EventId:    eventID,   // Echo des Command-EventIDs
            VehicleId:  sess.VehicleID,
            OperatorId: sess.OperatorID,
            Timestamp:  time.Now().UnixMilli(),
        },
        Success:  success,
        ErrorMsg: errMsg,
    }
    return proto.Marshal(ack)
}
```

Das Rate-Limiting (100 Commands/s) läuft über einen einfachen Token-Bucket ohne externe Dependency:

```go
// tokenBucket: erlaubt maxCommandsPerSecond = 100 Commands/s
func (b *tokenBucket) allow() bool {
    b.tokens += elapsed.Seconds() * b.rate  // Tokens nachfüllen
    if b.tokens < 1.0 { return false }      // Budget erschöpft
    b.tokens--
    return true
}
```

---

### 4. WebSocket Transport — JWT Handshake & SAFE_MODE Handling

**Datei:** [internal/controlserver/transport/websocket.go](../internal/controlserver/transport/websocket.go)

Der JWT-Token wird im WebSocket-Handshake validiert — entweder als `Authorization: Bearer <token>` Header oder als `?token=` Query-Parameter:

```go
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
    tokenStr := extractToken(r)
    claims, err := h.validateJWT(tokenStr)
    if err != nil {
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }

    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()

    // Recovery-Pfad: SAFE_MODE → RECOVERING → AUTHENTICATED
    // Normal-Pfad:   IDLE → CONNECTING → AUTHENTICATED
    if current, _, _, _ := h.sm.Get(); current == statemachine.StateSafeMode {
        h.sm.TransitionSystem(statemachine.StateRecovering)
    } else {
        h.sm.TransitionSystem(statemachine.StateConnecting)
    }
    h.sm.TransitionSystem(statemachine.StateAuthenticated)

    go h.heartbeat(conn)
    h.readLoop(conn, claims)
}
```

Im ReadLoop: Commands in SAFE_MODE werden still verworfen, Disconnect → SAFE_MODE + Recovery Checkpoint:

```go
func (h *WSHandler) readLoop(conn *websocket.Conn, claims *Claims) {
    defer func() {
        h.deadman.Stop()
        sysState, _, _, _ := h.sm.Get()
        if sysState != statemachine.StateSafeMode {
            // WS-Disconnect → CRITICAL → SAFE_MODE (ADR-009/010)
            h.sm.TransitionSystem(statemachine.StateSafeMode)
        }
        // Recovery Checkpoint speichern
        h.sessionMgr.SaveCheckpoint(...)
        h.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
    }()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil { return }  // Verbindung geschlossen → defer greift

        sysState, ctrlState, _, _ := h.sm.Get()
        if sysState == statemachine.StateSafeMode || ctrlState == statemachine.ControlBlocked {
            continue  // Commands in SAFE_MODE still verwerfen (ADR-011)
        }
        // ... Command verarbeiten
    }
}
```

---

### 5. Session Manager (GSA) — Session-Lifecycle

**Datei:** [internal/controlserver/session/manager.go](../internal/controlserver/session/manager.go)

Der Session Manager ist der einzige Ort im System, der Sessions erzeugt. Die Session-ID ist ein ULID und überlebt SAFE_MODE:

```go
// CreateSession: erzeugt eine neue Control Session (1 Vehicle + 1 Operator)
// ULID als Root Anchor — zeitlich sortierbar, URL-safe (ADR-016)
func (m *Manager) CreateSession(vehicleID, operatorID, operatorRole string) Session {
    s := Session{
        ID:           ulid.Generate(),  // pkg/ulid — einziger ULID-Erzeuger
        VehicleID:    vehicleID,
        OperatorID:   operatorID,
        OperatorRole: operatorRole,
        CreatedAt:    time.Now(),
    }
    m.current = &s
    return s
}

// SaveCheckpoint: friert den Session-Zustand bei SAFE_MODE-Eintritt ein.
// Basis für RECOVERING → CONNECTED re-Aktivierung ohne Auto-Resume.
func (m *Manager) SaveCheckpoint(sysState, ctrlState, safetyReason string) {
    m.checkpoint = &RecoveryCheckpoint{
        SessionID:        m.current.ID,
        LastSystemState:  sysState,
        LastControlState: ctrlState,
        SafetyReason:     safetyReason,  // z.B. "WS_DISCONNECT" oder "DEADMAN_TIMEOUT"
        CheckpointTS:     time.Now(),
    }
}

// PushSFUEvent: sendet Session-Events asynchron an den Video Hub (ADR-015).
// Der SFU konsumiert, interpretiert aber niemals — er ist ein Dumb Media Router.
func (m *Manager) PushSFUEvent(eventType string) {
    go m.sfuPublisher.PublishSessionEvent(eventType, s.ID, s.OperatorID)
}
```

---

### 6. Protobuf Schema — ControlCommand & CorrelationHeader

**Datei:** [proto/control.proto](../proto/control.proto)

```protobuf
// Alle Commands tragen einen CorrelationHeader (aus common.proto).
// session_id + event_id + vehicle_id + operator_id + timestamp
// → vollständige Rekonstruierbarkeit über alle Kanäle (ADR-016)
message ControlCommand {
  avoc.common.v1.CorrelationHeader header = 1;
  CommandType                       type  = 2;
  float                             value = 3;  // normalisiert [-1.0, 1.0]
}

// ControlAck ist die synchrone Antwort vom Server (<100ms Ziel).
// Enthält denselben CorrelationHeader → Frontend kann ACK der Command zuordnen.
message ControlAck {
  avoc.common.v1.CorrelationHeader header    = 1;
  bool                              success   = 2;
  string                            error_msg = 3;
}

enum CommandType {
  COMMAND_TYPE_UNSPECIFIED     = 0;
  COMMAND_TYPE_STEER           = 1;  // value: [-1.0 links, 1.0 rechts]
  COMMAND_TYPE_THROTTLE        = 2;  // value: [-1.0 rückwärts, 1.0 vorwärts]
  COMMAND_TYPE_BRAKE           = 3;  // value: [0.0, 1.0]
  COMMAND_TYPE_SPEED           = 4;
  COMMAND_TYPE_EMERGENCY_STOP  = 5;
  COMMAND_TYPE_DEADMAN_HOLD    = 6;
  COMMAND_TYPE_DEADMAN_RELEASE = 7;
}
```

**Versioning-Regel:** Field-IDs (1, 2, 3, ...) dürfen **niemals** geändert werden. Neue Felder werden als optional hinzugefügt. Felder entfernen → neues ADR.

---

### 7. Frontend: WSClient — Verbindung & ACK-Latenz

**Datei:** [frontend/src/lib/ws-client.ts](../frontend/src/lib/ws-client.ts)

Der WSClient misst die Roundtrip-Latenz pro Command automatisch:

```typescript
export class WSClient {
  private pendingAckTs = 0  // Timestamp des gesendeten Commands

  connect(token: string): void {
    // Token als Query-Parameter (WebSocket unterstützt keine Custom-Header im Browser)
    const url = `${proto}://${window.location.host}/ws?token=${token}`
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'  // Protobuf = binär

    this.ws.onmessage = (e) => {
      const latency = this.pendingAckTs > 0 ? Date.now() - this.pendingAckTs : 0
      this.pendingAckTs = 0
      if (latency > 0) this.onAck?.(latency)  // Callback mit gemessener Latenz

      // Protobuf ControlAck parsen — error_msg aus Server surfacen
      const ack = fromBinary(ControlAckSchema, new Uint8Array(e.data))
      if (!ack.success && ack.errorMsg) this.onAckError?.(ack.errorMsg)
    }

    this.ws.onclose = () => this.onClose?.()  // → SAFE_MODE im Frontend
  }

  send(bytes: Uint8Array): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.pendingAckTs = Date.now()  // Startzeitpunkt für Latenz-Messung
    this.ws.send(bytes)
  }
}
```

---

### 8. Frontend: useDeadmanSwitch — Server-Watchdog am Leben erhalten

**Datei:** [frontend/src/hooks/useDeadmanSwitch.ts](../frontend/src/hooks/useDeadmanSwitch.ts)

Der Hook sendet alle 1,5s ein `DEADMAN_HOLD` Protobuf-Command solange der Operator die Leertaste (oder den Button) hält. Der Server-seitige Watchdog läuft auf 10s Timeout — loslassen → Timeout → SAFE_MODE.

```typescript
const DEADMAN_INTERVAL_MS = 1500  // Alle 1,5s senden (Server-Timeout: 10s)

export function useDeadmanSwitch(wsClient, sessionId, vehicleId, operatorId, enabled) {
  const [isActive, setIsActive] = useState(false)

  const sendHold = useCallback(() => {
    const header = create(CorrelationHeaderSchema, {
      sessionId, eventId: generateULID(),
      vehicleId, operatorId,
      timestamp: BigInt(Date.now()),
    })
    const cmd = create(ControlCommandSchema, {
      header,
      type: CommandType.DEADMAN_HOLD,  // protoc-gen-es v2: Kurzname (kein Präfix)
      value: 1.0,
    })
    wsClient.send(toBinary(ControlCommandSchema, cmd))
  }, [wsClient, sessionId, vehicleId, operatorId])

  // Interval läuft nur solange gehalten
  useEffect(() => {
    if (!isActive || !enabled) return
    sendHold()                                      // Sofort beim Aktivieren senden
    const id = setInterval(sendHold, DEADMAN_INTERVAL_MS)
    return () => clearInterval(id)
  }, [isActive, enabled, sendHold])

  // Keyboard: Spacebar
  useEffect(() => {
    const onKeyDown = (e) => { if (e.code === 'Space' && !e.repeat) activate() }
    const onKeyUp   = (e) => { if (e.code === 'Space') deactivate() }
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    return () => { /* cleanup */ }
  }, [activate, deactivate])

  return {
    isActive,
    buttonProps: { onMouseDown: activate, onMouseUp: deactivate, onMouseLeave: deactivate },
  }
}
```

---

### 9. Frontend: useControls — 20 Hz Command Loop mit Prioritäten

**Datei:** [frontend/src/hooks/useControls.ts](../frontend/src/hooks/useControls.ts)

Der Control-Loop läuft mit 20 Hz (alle 50ms). Eingabequellen haben eine feste Priorität: **Gamepad > Joystick > Keyboard**.

```typescript
const INTERVAL_MS = 50  // 20 Hz

useEffect(() => {
  if (!enabled) return
  const id = setInterval(() => {
    const s = speedRef.current  // Speed Multiplier [0.0, 1.0]

    // Priorität 1: Gamepad (wenn verbunden)
    const gp = Array.from(navigator.getGamepads()).find(g => g?.connected)
    if (gp) {
      const steerVal    = Math.abs(gp.axes[0]) > 0.1 ? gp.axes[0] * s : 0  // Deadzone 0.1
      const throttleVal = Math.abs(gp.axes[1]) > 0.1 ? -gp.axes[1] * s : 0
      const brakeVal    = gp.buttons[6]?.value ?? 0  // L2-Trigger
      if (steerVal) sendCmd(CommandType.STEER, steerVal)
      if (throttleVal) sendCmd(CommandType.THROTTLE, throttleVal)
      setActiveMode('gamepad')
      return  // Gamepad hat Vorrang — Keyboard/Joystick werden ignoriert
    }

    // Priorität 2: Virtueller Joystick (Touch/Maus-Drag)
    if (joyActiveRef.current) {
      const { x, y } = joyPosRef.current  // normalisiert [-1, 1]
      sendCmd(CommandType.STEER, x * s)
      sendCmd(CommandType.THROTTLE, y * s)
      setActiveMode('joystick')
      return
    }

    // Priorität 3: Keyboard (WASD oder Pfeiltasten)
    const keys = heldKeys.current  // Set<string> mit gedrückten Key-Codes
    let sv = 0, tv = 0
    if (keys.has('KeyA') || keys.has('ArrowLeft'))  sv -= 1
    if (keys.has('KeyD') || keys.has('ArrowRight')) sv += 1
    if (keys.has('KeyW') || keys.has('ArrowUp'))    tv += 1
    if (keys.has('KeyS') || keys.has('ArrowDown'))  tv -= 1
    if (sv || tv) {
      sendCmd(CommandType.STEER, sv * s)
      sendCmd(CommandType.THROTTLE, tv * s)
      setActiveMode('keyboard')
    }
  }, INTERVAL_MS)
  return () => clearInterval(id)
}, [enabled, sendCmd])
```

Wenn `enabled = false` (z.B. SAFE_MODE), wird der Interval sofort gestoppt und alle gedrückten Keys werden gecleart:

```typescript
useEffect(() => {
  if (!enabled) {
    heldKeys.current.clear()
    joyActiveRef.current = false
    setJoyPos({ x: 0, y: 0 })
  }
}, [enabled])
```

---

### 10. Safety Tests — Muster & Aufbau

**Datei:** [tests/unit/safety_test.go](../tests/unit/safety_test.go)

Alle Safety-Tests folgen demselben Aufbaumuster: State Machine + Mock-Publisher + Session Manager.

```go
// Gemeinsames Setup für alle Safety-Tests
func newTestSetup(t *testing.T) (sm *statemachine.Machine, pub *mocks.MockSafetyPublisher, ...) {
    sm = statemachine.New()
    pub = &mocks.MockSafetyPublisher{}
    sfuPub = &mocks.MockSFUPublisher{}
    mgr = session.NewManager(sfuPub)
    return
}

// Hilfsfunktion: bringt die State Machine in den CONNECTED-Zustand
func connectSession(t *testing.T, sm *statemachine.Machine, mgr *session.Manager) session.Session {
    sm.TransitionSystem(statemachine.StateConnecting)
    sm.TransitionSystem(statemachine.StateAuthenticated)
    ok := sm.TransitionToConnected()
    require.True(t, ok)
    return mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")
}

// Beispiel: Test für Dead-man Timeout → SAFE_MODE
func TestSafety_DeadmanTimeout_TriggersSafeMode(t *testing.T) {
    sm, pub, _, mgr := newTestSetup(t)
    sess := connectSession(t, sm, mgr)

    deadman := csafety.NewDeadmanWatchdog(50*time.Millisecond, sm, pub)
    deadman.Start(sess.ID, sess.VehicleID)
    deadman.Reset()  // Arming: Watchdog startet jetzt den Timer

    time.Sleep(100 * time.Millisecond)  // Timeout abwarten

    sys, ctrl, _, _ := sm.Get()
    assert.Equal(t, statemachine.StateSafeMode, sys)
    assert.Equal(t, statemachine.ControlBlocked, ctrl)
    assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEvent.Type)
}

// Beispiel: Ungültige Transition muss abgelehnt werden
func TestSafety_InvalidTransitionRejected(t *testing.T) {
    sm, _, _, _ := newTestSetup(t)

    sm.TransitionSystem(statemachine.StateConnected)  // IDLE → CONNECTED: ungültig

    sys, _, _, _ := sm.Get()
    assert.Equal(t, statemachine.StateIdle, sys)  // muss IDLE geblieben sein
}
```

**Neuen Safety-Test schreiben:** `newTestSetup` + `connectSession` + gewünschten Trigger auslösen + `sm.Get()` prüfen. Safety-Tests müssen immer grün bleiben: `make test-safety`.
