// Package transport implements the WebSocket Transport Layer (ADR-010).
// JWT auth in handshake, session_id-based routing, role-aware disconnect (ADR-025),
// Channel Close on CRITICAL events, Heartbeat 30s, DeadmanWatchdog and ACKTimeoutWatcher.
package transport

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	commonv1 "avoc/gen/go/common/v1"
	controlv1 "avoc/gen/go/control/v1"
	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/safetyservice"
	"avoc/pkg/audit"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var svcLog = logger.New("control-server")

const heartbeatInterval = 30 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// WSHandler looks up the per-vehicle State Machine and Watchdogs via the
// Registry on every request/message (ADR-026) — there is no single injected
// sm/deadman/ackWatcher anymore, since two vehicles must never share safety state.
type WSHandler struct {
	jwtSecret       []byte
	vehicleContexts *vehiclecontext.Registry
	sessionMgr      *session.Manager
	engine          *command.Engine
	auditWriter     audit.SafetyAuditWriter
	publisher       csafety.Publisher
}

func NewWSHandler(
	jwtSecret string,
	vehicleContexts *vehiclecontext.Registry,
	sessionMgr *session.Manager,
	engine *command.Engine,
) *WSHandler {
	return &WSHandler{
		jwtSecret:       []byte(jwtSecret),
		vehicleContexts: vehicleContexts,
		sessionMgr:      sessionMgr,
		engine:          engine,
	}
}

// WithAuditWriter sets the audit writer for WS_DISCONNECT persistence (ADR-018).
func (h *WSHandler) WithAuditWriter(aw audit.SafetyAuditWriter) *WSHandler {
	h.auditWriter = aw
	return h
}

// WithPublisher sets the Safety Event Bus publisher (ADR-002), used to publish
// EventNoOperator when the OPERATOR layer transitions to NO_OPERATOR on disconnect
// (2026-07-16, DRIFT-K2). Safe to omit in tests (nil publisher = no-op below).
func (h *WSHandler) WithPublisher(pub csafety.Publisher) *WSHandler {
	h.publisher = pub
	return h
}

// wsAuth bundles the JWT claims + session resolved for one WS upgrade
// request (Rule 2.4 — bundles what would otherwise be 2+ related return values).
type wsAuth struct {
	claims *Claims
	sess   session.Session
}

// authenticateWS validates the JWT + session_id (ADR-004/025) for a WS
// upgrade request. On failure it writes the appropriate HTTP error response
// itself and returns ok=false — callers must return immediately in that case.
func (h *WSHandler) authenticateWS(w http.ResponseWriter, r *http.Request) (wsAuth, bool) {
	tokenStr := extractToken(r)
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return wsAuth{}, false
	}

	claims, err := h.validateJWT(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return wsAuth{}, false
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return wsAuth{}, false
	}

	sess, ok := h.sessionMgr.GetSession(sessionID)
	if !ok {
		svcLog.Event(logger.EventWsSessionInvalid,
			"WS upgrade rejected — session_id not found",
			"session_id", sessionID, "remote", r.RemoteAddr)
		http.Error(w, "session not found", http.StatusNotFound)
		return wsAuth{}, false
	}

	return wsAuth{claims: claims, sess: sess}, true
}

// recoverFromSafeMode resumes an ACTIVE_OPERATOR reconnecting after SAFE_MODE
// (ADR-009/011). POST /session/start already set the machine to CONNECTED for
// a fresh session — only the WS reconnect path (resume after SAFE_MODE) needs
// to transition back.
func (h *WSHandler) recoverFromSafeMode(sess session.Session) {
	vc := h.vehicleContexts.Get(sess.VehicleID)
	current, _, _, _ := vc.SM.Get()
	if current != statemachine.StateSafeMode {
		return
	}
	vc.SM.TransitionSystem(statemachine.StateRecovering)
	vc.SM.TransitionSystem(statemachine.StateAuthenticated)
	vc.SM.TransitionToConnected()
	vc.SM.TransitionOperator(statemachine.OpActive)
	vc.Deadman.Start(sess.ID, sess.VehicleID)
	if vc.AuthWatchdog != nil {
		// Restart polling too (DRIFT-K1) — Stop() is only called on
		// session/end, but a SAFE_MODE recovery does not go through
		// session/start again, so without this the watchdog would never
		// resume checking the reconnected operator's account.
		vc.AuthWatchdog.Start(sess.ID, sess.VehicleID, sess.OperatorID)
	}
	svcLog.Event(logger.EventStateTransition,
		"ACTIVE_OPERATOR reconnected — recovered from SAFE_MODE, deadman restarted",
		"session_id", sess.ID)
}

// ServeWS upgrades the connection and validates the JWT + session_id (ADR-004/025).
// The session_id query parameter is required — it links this WS connection to a
// previously created session (POST /session/start). Role is derived from the session.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	auth, ok := h.authenticateWS(w, r)
	if !ok {
		return
	}
	claims, sess := auth.claims, auth.sess
	isObserver := sess.OperatorRole == "OBSERVER"

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		svcLog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	if !isObserver {
		h.recoverFromSafeMode(sess)
	}

	svcLog.Event(logger.EventWsConnected, "WebSocket connected",
		"subject", claims.Subject, "role", sess.OperatorRole, "session_id", sess.ID)

	go h.heartbeat(conn)
	h.readLoop(conn, claims, sess, isObserver)
}

func (h *WSHandler) heartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

// wsConn bundles the per-connection state a message-loop helper needs
// (Rule 2.3 — keeps processWSMessage/handleWSDisconnect within the 4-param limit).
type wsConn struct {
	conn       *websocket.Conn
	vc         *vehiclecontext.VehicleContext
	claims     *Claims
	sess       session.Session
	isObserver bool
}

func (h *WSHandler) readLoop(conn *websocket.Conn, claims *Claims, sess session.Session, isObserver bool) {
	ws := wsConn{
		conn:       conn,
		vc:         h.vehicleContexts.Get(sess.VehicleID),
		claims:     claims,
		sess:       sess,
		isObserver: isObserver,
	}

	defer h.handleWSDisconnect(ws)

	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(heartbeatInterval * 2))
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if !h.processWSMessage(ws, msg) {
			return
		}
	}
}

// handleWSDisconnect runs the disconnect bookkeeping deferred by readLoop.
// Only ACTIVE_OPERATOR disconnect triggers SAFE_MODE (ADR-025); OBSERVER
// disconnects just release the session.
func (h *WSHandler) handleWSDisconnect(ws wsConn) {
	if ws.isObserver {
		svcLog.Info("OBSERVER WebSocket disconnected — no SAFE_MODE",
			"subject", ws.claims.Subject, "session_id", ws.sess.ID)
		h.sessionMgr.ReleaseSession(ws.sess.ID)
		return
	}

	ws.vc.Deadman.Stop()
	if ws.vc.AuthWatchdog != nil {
		ws.vc.AuthWatchdog.Stop()
	}
	// If the session was already released via POST /session/end, the WS close
	// is intentional — do not trigger SAFE_MODE in that case.
	_, sessionStillActive := h.sessionMgr.GetSession(ws.sess.ID)
	sysState, _, _, _ := ws.vc.SM.Get()
	if sessionStillActive && sysState != statemachine.StateSafeMode {
		svcLog.Event(logger.EventWsDisconnect,
			"ACTIVE_OPERATOR WebSocket disconnected → SAFE_MODE",
			"subject", ws.claims.Subject, "session_id", ws.sess.ID)
		h.auditWSDisconnect(ws)
		ws.vc.SM.TransitionSystem(statemachine.StateSafeMode)

		// OPERATOR layer (2026-07-16, DRIFT-K2): SYSTEM is already SAFE_MODE by
		// this point, so TransitionOperator's own SAFE_MODE branch (guarded on
		// System==Connected/Degraded) is a no-op here — this call only corrects
		// the OPERATOR field, which previously hung at ACTIVE_OPERATOR forever
		// after a disconnect. It also makes TransitionOperator(OpNoOperator) the
		// single canonical "operator gone" trigger that future callers (e.g. the
		// AuthWatchdog, DRIFT-K1) can reuse directly, instead of only being
		// reachable through this WS-specific code path.
		ws.vc.SM.TransitionOperator(statemachine.OpNoOperator)
		if h.publisher != nil {
			h.publisher.PublishEvent(safetyservice.SafetyEvent{
				SessionID: ws.sess.ID,
				VehicleID: ws.sess.VehicleID,
				Type:      safetyservice.EventNoOperator,
				Reason:    "ACTIVE_OPERATOR WebSocket disconnected — no active operator",
				Timestamp: time.Now(),
			})
		}
	}
	if !sessionStillActive {
		svcLog.Event(logger.EventWsDisconnect,
			"ACTIVE_OPERATOR WebSocket closed after session/end — no SAFE_MODE",
			"subject", ws.claims.Subject, "session_id", ws.sess.ID)
		return
	}
	sys, ctrl, _, _ := ws.vc.SM.Get()
	h.sessionMgr.SaveCheckpoint(string(sys), string(ctrl), "WS_DISCONNECT")
	h.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
	svcLog.Event(logger.EventSafeModeEntered,
		"recovery checkpoint saved", "session_id", ws.sess.ID)
}

// auditWSDisconnect persists the ACTIVE_OPERATOR disconnect as a safety audit
// event (ADR-018), if an audit writer is configured. A write failure is
// logged but never blocks the SAFE_MODE transition.
func (h *WSHandler) auditWSDisconnect(ws wsConn) {
	if h.auditWriter == nil {
		return
	}
	sys, ctrl, _, _ := ws.vc.SM.Get()
	if err := h.auditWriter.WriteSync(audit.SafetyAuditEvent{
		EventID:     ulid.Generate(),
		SessionID:   ws.sess.ID,
		VehicleID:   ws.sess.VehicleID,
		OperatorID:  ws.sess.OperatorID,
		EventType:   logger.EventWsDisconnect,
		Reason:      "ACTIVE_OPERATOR WebSocket disconnected",
		SystemState: string(sys),
		CtrlState:   string(ctrl),
		Timestamp:   time.Now(),
	}); err != nil {
		svcLog.Error("audit write failed — proceeding to SAFE_MODE", "error", err)
	}
}

// processWSMessage handles one inbound WS message and writes back the ack.
// Returns false when the connection should be closed (write failure).
func (h *WSHandler) processWSMessage(ws wsConn, msg []byte) bool {
	sysState, ctrlState, _, _ := ws.vc.SM.Get()
	if sysState == statemachine.StateSafeMode || ctrlState == statemachine.ControlBlocked {
		// Commands dropped silently in SAFE_MODE (ADR-011)
		return true
	}

	// Refresh session in case role changed (e.g. after handover).
	currentSess, hasSession := h.sessionMgr.GetSession(ws.sess.ID)

	// Only ACTIVE_OPERATOR commands reset the deadman / ACK watcher.
	if hasSession && !ws.isObserver {
		ws.vc.ACKTimeoutWatcher.CommandReceived(currentSess.ID, currentSess.VehicleID)
	}

	var ackBytes []byte
	if hasSession {
		var err error
		ackBytes, err = h.engine.Handle(msg, currentSess)
		if err != nil {
			ackBytes, _ = proto.Marshal(&controlv1.ControlAck{
				Header:   &commonv1.CorrelationHeader{Timestamp: time.Now().UnixMilli()},
				Success:  false,
				ErrorMsg: "command engine error",
			})
		}
	} else {
		ackBytes, _ = proto.Marshal(&controlv1.ControlAck{
			Header:   &commonv1.CorrelationHeader{Timestamp: time.Now().UnixMilli()},
			Success:  false,
			ErrorMsg: "no active session",
		})
	}

	if err := ws.conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
		ws.vc.ACKTimeoutWatcher.CommandACKed()
		return false
	}
	ws.vc.ACKTimeoutWatcher.CommandACKed()
	return true
}

// validateJWT rejects a token whose header claims a signing method other than the HMAC family
// (SEC-01) — see internal/authservice/tokenissuer.go's JWTTokenIssuer.ParseToken for the full
// rationale (JWT "alg confusion" attack).
func (h *WSHandler) validateJWT(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.jwtSecret, nil
	})
	return claims, err
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
