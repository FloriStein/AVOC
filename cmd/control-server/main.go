package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"avoc/internal/controlserver/authcheck"
	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/telemetrycheck"
	"avoc/internal/controlserver/transport"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/mediamtx"
	"avoc/internal/recording"
	"avoc/internal/vehicleconnection"
	"avoc/internal/vehicleregistry"
	"avoc/pkg/audit"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/env"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
)

var log = logger.New("control-server")

// feLog logs frontend events received via POST /log (service label: "frontend")
var feLog = logger.New("frontend")

// serverConfig bundles environment-derived configuration read once at startup.
type serverConfig struct {
	port           string
	secret         string
	safetyURL      string
	sfuURL         string
	authURL        string
	telemetryURL   string
	whipStreamKey  string
	mediamtxAPIURL string
	turnExternalIP string
	turnPort       string
	turnUser       string
	turnPassword   string
	databaseURL    string
}

// loadConfig reads all environment-derived configuration. The order of the
// two env.Require calls (secret, then databaseURL) matches the original
// inline main() — env.Require calls log.Fatal on a missing required
// variable, so this order determines which missing variable is reported
// first if both are absent.
func loadConfig() serverConfig {
	cfg := serverConfig{
		port:   env.OptionalOr("CONTROL_PORT", "8080"),
		secret: env.Require("JWT_SECRET", log),
	}
	cfg.safetyURL = env.OptionalOr("SAFETY_SERVICE_URL", "http://safety-service:8082")
	cfg.sfuURL = env.OptionalOr("SFU_SERVICE_URL", "http://webrtc-sfu:8084")
	cfg.authURL = env.OptionalOr("AUTH_SERVICE_URL", "http://auth-service:8081")
	cfg.telemetryURL = env.OptionalOr("TELEMETRY_SERVICE_URL", "http://telemetry-service:8083")
	cfg.whipStreamKey = os.Getenv("WHIP_STREAM_KEY")
	cfg.mediamtxAPIURL = env.OptionalOr("MEDIAMTX_API_URL", "http://mediamtx:9997")
	cfg.turnExternalIP = os.Getenv("TURN_EXTERNAL_IP")
	cfg.turnPort = env.OptionalOr("TURN_PORT", "3478")
	cfg.turnUser = os.Getenv("TURN_USER")
	cfg.turnPassword = os.Getenv("TURN_PASSWORD")
	cfg.databaseURL = env.Require("DATABASE_URL", log)
	return cfg
}

// newAuditWriter opens the Postgres-backed audit writer (LOG-10/11 —
// ADR-018/023), falling back to a no-op writer if unavailable. Returns a
// cleanup func to defer — a no-op when the writer isn't backed by Postgres,
// matching the original conditional defer.
func newAuditWriter(db *sql.DB) (audit.AuditWriter, func()) {
	pgWriter, err := audit.NewPostgresAuditWriter(db)
	if err != nil {
		log.Warn("audit store unavailable — using NoopWriter", "error", err)
		return audit.NewNoopWriter(), func() {}
	}
	log.Info("audit store ready (PostgreSQL)")
	return pgWriter, func() { pgWriter.Close() }
}

// newVehicleStore opens the Postgres-backed vehicle registry (ADR-022/023),
// falling back to a no-op store if unavailable, and seeds the default
// vehicle on first successful open.
func newVehicleStore(db *sql.DB, conn vehicleregistry.ConnectionChecker) vehicleregistry.VehicleStore {
	vs, err := vehicleregistry.NewPostgresVehicleStore(db, conn)
	if err != nil {
		log.Warn("vehicle registry unavailable", "error", err)
		return vehicleregistry.NoopVehicleStore{}
	}
	if seedErr := vs.SeedDefault(); seedErr != nil {
		log.Warn("vehicle registry seed failed", "error", seedErr)
	} else {
		log.Info("vehicle registry ready, vehicle-001 seeded")
	}
	return vs
}

// controlServer bundles every dependency shared across route handlers. It is
// constructed once at startup (newControlServer) in the security-critical
// bootstrap order documented there; route handlers are methods so they can
// only see this already-fully-constructed, verified dependency set.
type controlServer struct {
	cfg             serverConfig
	auditWriter     audit.AuditWriter
	safetyPub       *csafety.HTTPPublisher
	sessionMgr      *session.Manager
	vehicleContexts *vehiclecontext.Registry
	handoverMgr     *session.HandoverManager
	recorder        *recording.MemoryRecorder
	vehicleRegistry *vehicleconnection.Registry
	vehicleAckStore *vehicleconnection.AckStore
	vehicleStore    vehicleregistry.VehicleStore
	cmdEngine       *command.Engine
	wsHandler       *transport.WSHandler
	vehicleHandler  *vehicleconnection.Handler
	mtxClient       *mediamtx.Client
}

// newControlServer wires every core component in the security-critical
// bootstrap order (ADR-026/ADR-002 dependencies below are order-sensitive —
// do not reorder without re-verifying against those ADRs):
//  1. safetyPub / sfuPub / sessionMgr — no dependencies on anything else.
//  2. vehicleContexts (per-vehicle State Machine + Watchdogs, ADR-026) —
//     needs safetyPub + auditWriter.
//  3. handoverMgr / recorder / vehicleRegistry / vehicleAckStore — need
//     vehicleContexts/sessionMgr.
//  4. vehicleStore — needs db + vehicleRegistry (as ConnectionChecker).
//  5. SafetyBusWatchdog — single process-wide instance (ADR-002: only one
//     safety-service), fans failures out to every vehicleContexts entry
//     (ADR-026); started here so it runs for the whole process lifetime,
//     not per session.
//  6. cmdEngine — needs vehicleContexts/safetyPub/sessionMgr/auditWriter/
//     vehicleRegistry (as VehicleForwarder).
//  7. wsHandler / vehicleHandler — need cmdEngine/vehicleStore and
//     everything above.
func newControlServer(cfg serverConfig, db *sql.DB, auditWriter audit.AuditWriter, mtxClient *mediamtx.Client) *controlServer {
	safetyPub := csafety.NewHTTPPublisher(cfg.safetyURL)
	sfuPub := session.NewHTTPSFUPublisher(cfg.sfuURL)
	sessionMgr := session.NewManager(sfuPub)

	// Per-vehicle State Machine + Watchdogs (ADR-026) — replaces the single
	// process-wide sm/deadman/ackWatcher/vehicleACKWatchdog singletons that
	// caused two vehicles to silently share (and overwrite) safety monitoring.
	// AuthWatchdog (DRIFT-K1, 2026-07-16) reads the shared `avoc` DB directly —
	// same pattern as vehicleregistry/audit below — no new HTTP dependency on
	// auth-service. TelemetryWatchdog (DRIFT-K3-TELEMETRY, Sprint 50) polls
	// telemetry-service over HTTP instead — telemetry-service isn't backed by
	// the shared `avoc` DB, unlike auth-service's users table.
	userChecker := authcheck.NewChecker(db)
	telemetryChecker := telemetrycheck.NewChecker(cfg.telemetryURL)
	vehicleContexts := vehiclecontext.NewRegistry(
		csafety.DefaultDeadmanTimeout, csafety.DefaultACKTimeout, csafety.DefaultVehicleACKTimeout, safetyPub,
	).WithAuditWriter(auditWriter).WithUserChecker(userChecker).WithTelemetryChecker(telemetryChecker)

	// HandoverManager resolves each vehicle's own State Machine via the same
	// per-vehicle registry as everything else (ADR-026 follow-up, MV-11) — no
	// longer a disconnected standalone Machine shared across all vehicles.
	handoverMgr := session.NewHandoverManager(vehicleContexts, sessionMgr, cfg.authURL)
	recorder := recording.NewMemoryRecorder()
	vehicleRegistry := vehicleconnection.NewRegistry()
	vehicleAckStore := vehicleconnection.NewAckStore()
	vehicleStore := newVehicleStore(db, vehicleRegistry)

	// SafetyBusWatchdog stays a single process-wide instance — there is only
	// one safety-service (ADR-002) — but fans a failure out to every active
	// vehicle (ADR-026). Starts once for the process lifetime, not per session.
	startSafetyBusWatchdog(cfg, vehicleContexts, sessionMgr, safetyPub)

	cmdEngine := command.NewEngine(vehicleContexts, safetyPub, sessionMgr).
		WithAuditWriter(auditWriter).
		WithVehicleForwarder(vehicleRegistry)

	s := &controlServer{
		cfg:             cfg,
		auditWriter:     auditWriter,
		safetyPub:       safetyPub,
		sessionMgr:      sessionMgr,
		vehicleContexts: vehicleContexts,
		handoverMgr:     handoverMgr,
		recorder:        recorder,
		vehicleRegistry: vehicleRegistry,
		vehicleAckStore: vehicleAckStore,
		vehicleStore:    vehicleStore,
		cmdEngine:       cmdEngine,
		mtxClient:       mtxClient,
	}
	// --- Handlers --- (need s's already-assigned fields above, hence last)
	s.buildHandlers()
	return s
}

// startSafetyBusWatchdog constructs and starts the single process-wide
// SafetyBusWatchdog — there is only one safety-service (ADR-002) — which
// fans a failure out to every active vehicle (ADR-026). Starts once for the
// process lifetime, not per session.
func startSafetyBusWatchdog(cfg serverConfig, vehicleContexts *vehiclecontext.Registry, sessionMgr *session.Manager, safetyPub *csafety.HTTPPublisher) {
	csafety.NewSafetyBusWatchdog(csafety.SafetyBusWatchdogOptions{
		HealthURL: cfg.safetyURL + "/health",
		Interval:  csafety.DefaultBusCheckInterval,
		Threshold: csafety.DefaultBusFailThreshold,
		Vehicles:  vehicleContexts,
		Sessions:  sessionMgr,
		Publisher: safetyPub,
	}).Start()
}

// buildHandlers wires wsHandler/vehicleHandler from s's already-constructed
// dependencies. Must run after every other controlServer field is set.
func (s *controlServer) buildHandlers() {
	s.wsHandler = transport.NewWSHandler(s.cfg.secret, s.vehicleContexts, s.sessionMgr, s.cmdEngine).
		WithAuditWriter(s.auditWriter)
	s.vehicleHandler = vehicleconnection.NewHandler(vehicleconnection.HandlerOptions{
		JWTSecret:       s.cfg.secret,
		VehicleContexts: s.vehicleContexts,
		Publisher:       s.safetyPub,
		Registry:        s.vehicleRegistry,
		AckStore:        s.vehicleAckStore,
	}).WithVehicleAdder(s.vehicleStore)
}

func main() {
	cfg := loadConfig()
	mtxClient := mediamtx.NewClient(cfg.mediamtxAPIURL)

	// --- PostgreSQL (ADR-023) ---
	db := pkgdb.OpenAndWait(cfg.databaseURL, log, "database not reachable after retries — starting in degraded mode")
	defer db.Close()

	// --- Audit Writer (LOG-10/11 — ADR-018/023) ---
	auditWriter, closeAudit := newAuditWriter(db)
	defer closeAudit()

	srv := newControlServer(cfg, db, auditWriter, mtxClient)
	mux := srv.newMux()

	log.Info("Control Server starting", "port", cfg.port)
	if err := http.ListenAndServe(":"+cfg.port, mux); err != nil {
		log.Fatal("Control Server failed", "error", err)
	}
}

// newMux registers every route. Registration order matches the original
// inline main() (no observable effect on routing — net/http's ServeMux
// dispatches by pattern match, not registration order — kept identical
// anyway for diff-review clarity).
func (s *controlServer) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", s.wsHandler.ServeWS)
	mux.HandleFunc("/vehicle/ws", s.vehicleHandler.ServeWS)

	auth := requireJWT([]byte(s.cfg.secret))

	// Session lifecycle (GSA — ADR-015/025)
	mux.HandleFunc("POST /session/start", auth(s.handleSessionStart))
	mux.HandleFunc("POST /session/end", auth(s.handleSessionEnd))
	// Logout gate — rejects logout while an ACTIVE_OPERATOR session is live (ADR-025).
	// Frontend disables the button, but this endpoint enforces the rule server-side.
	// Routed via nginx /api/ → control-server (not /auth/ which goes to auth-service).
	mux.HandleFunc("POST /logout", auth(s.handleLogout))

	// Operator Handover (BE-12), scoped per vehicle (ADR-026 follow-up, MV-11).
	mux.HandleFunc("POST /handover/request", auth(s.handleHandoverRequest))
	mux.HandleFunc("POST /handover/confirm", auth(s.handleHandoverConfirm))
	mux.HandleFunc("POST /handover/cancel", auth(s.handleHandoverCancel))

	// MEDIA STATE update (ADR-009/011 Invariant 1)
	mux.HandleFunc("POST /media/event", auth(s.handleMediaEvent))

	// Emergency Stop proxy (ADR-009/026).
	mux.HandleFunc("POST /emergency-stop", auth(s.handleEmergencyStop))

	// LOG-07: Frontend log ingestion — Browser → POST /log → slog (service="frontend") → Loki
	mux.HandleFunc("POST /log", s.handleLog)

	// LOG-11: Audit events query endpoint
	mux.HandleFunc("GET /audit/events", auth(s.handleAuditEvents))

	// Recording inspection (ADR-005)
	mux.HandleFunc("GET /recording/", auth(s.handleRecording))

	// MediaMTX Auth-Hook (ADR-020) — validiert WHIP publish + WHEP read
	// MediaMTX ruft diesen Endpoint für jeden eingehenden WHIP/WHEP-Request auf.
	mux.HandleFunc("POST /internal/media/auth", s.handleMediaAuth)

	// VEH-07 (ADR-021): Latest VehicleCommandAck per vehicle — polled by frontend
	mux.HandleFunc("GET /vehicle/ack/latest/", s.handleVehicleAckLatest)

	// DEV: Stream key for browser-based WHIP sender (StreamSenderPanel).
	mux.HandleFunc("GET /dev/whip-key", s.handleDevWhipKey)

	// Vehicle Registry (ADR-022)
	mux.HandleFunc("GET /vehicles", s.handleVehiclesList)
	mux.HandleFunc("POST /vehicles", auth(s.handleVehiclesCreate))
	mux.HandleFunc("DELETE /vehicles/{id}", auth(s.handleVehicleDelete))

	// GET /sessions — returns all currently active sessions (visible to all authenticated users).
	mux.HandleFunc("GET /sessions", auth(s.handleSessions))

	// State + Health
	mux.HandleFunc("GET /vehicles/{id}/state", s.handleVehicleState)

	// ICE server config for WebRTC clients (WEBRTC-06 — Sprint 10).
	mux.HandleFunc("GET /ice-config", s.handleICEConfig)

	mux.HandleFunc("GET /health", s.handleHealth)

	return mux
}

// handleSessionStart creates a session for the given vehicle + operator.
// Returns ACTIVE_OPERATOR if the vehicle is free; OBSERVER if already controlled (ADR-025).
// The state machine is only advanced for ACTIVE_OPERATOR sessions.
// WS must be connected AFTER this call using the returned session_id.
func (s *controlServer) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID  string `json:"vehicle_id"`
		OperatorID string `json:"operator_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.VehicleID == "" || req.OperatorID == "" {
		http.Error(w, "vehicle_id and operator_id required", http.StatusBadRequest)
		return
	}
	callerRole, _ := r.Context().Value(roleKey).(string)
	// OBSERVER JWT role may JOIN an already-controlled vehicle (passive watch) but
	// cannot claim ACTIVE_OPERATOR if the vehicle is free.
	if callerRole == "OBSERVER" && !s.sessionMgr.IsVehicleLocked(req.VehicleID) {
		http.Error(w, "observers cannot start a new session", http.StatusForbidden)
		return
	}

	if !s.vehicleRegistry.Connected(req.VehicleID) {
		http.Error(w, "vehicle not connected", http.StatusConflict)
		return
	}

	sess := s.sessionMgr.StartSession(req.VehicleID, req.OperatorID)

	if sess.OperatorRole == "ACTIVE_OPERATOR" {
		s.advanceVehicleToActiveOperator(sess)
	}

	log.Event(logger.EventSessionStarted, "session started",
		"session_id", sess.ID, "vehicle_id", sess.VehicleID,
		"operator_id", sess.OperatorID, "role", sess.OperatorRole)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"session_id": sess.ID,
		"role":       sess.OperatorRole,
		"vehicle_id": sess.VehicleID,
	})
}

// advanceVehicleToActiveOperator advances THIS VEHICLE's state machine:
// IDLE → CONNECTING → AUTHENTICATED → CONNECTED (ADR-026), then starts the
// per-session watchdogs and records the session start.
func (s *controlServer) advanceVehicleToActiveOperator(sess session.Session) {
	vc := s.vehicleContexts.Get(sess.VehicleID)
	vc.SM.TransitionSystem(statemachine.StateConnecting)
	vc.SM.TransitionSystem(statemachine.StateAuthenticated)
	if !vc.SM.TransitionToConnected() {
		// Already CONNECTED (fast re-start without WS disconnect) — acceptable.
		log.Warn("session/start: TransitionToConnected skipped — already CONNECTED",
			"session_id", sess.ID)
	}
	vc.SM.TransitionOperator(statemachine.OpActive)
	vc.Deadman.Start(sess.ID, sess.VehicleID)
	vc.VehicleACKWatchdog.Start(sess.ID, sess.VehicleID)
	if vc.AuthWatchdog != nil {
		vc.AuthWatchdog.Start(sess.ID, sess.VehicleID, sess.OperatorID)
	}
	if vc.TelemetryWatchdog != nil {
		vc.TelemetryWatchdog.Start(sess.ID, sess.VehicleID)
	}
	s.sessionMgr.PushSFUEvent("SESSION_CREATED")
	s.recorder.StartSession(sess.ID, sess.VehicleID, sess.OperatorID)
	sys, ctrl, _, _ := vc.SM.Get()
	s.recorder.RecordStateSnapshot(recording.StateSnapshotParams{
		SessionID:   sess.ID,
		EventID:     ulid.Generate(),
		VehicleID:   sess.VehicleID,
		OperatorID:  sess.OperatorID,
		SystemState: string(sys),
		CtrlState:   string(ctrl),
	})
}

func (s *controlServer) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	// session_id is optional for backward compat — falls back to current ACTIVE_OPERATOR session.
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.SessionID != "" {
		if sess, ok := s.sessionMgr.GetSession(req.SessionID); ok {
			s.recorder.EndSession(sess.ID)
			log.Event(logger.EventSessionEnded, "session ended",
				"session_id", sess.ID, "role", sess.OperatorRole)
			if sess.OperatorRole == "ACTIVE_OPERATOR" {
				vc := s.vehicleContexts.Get(sess.VehicleID)
				vc.Deadman.Stop()
				vc.VehicleACKWatchdog.Stop()
				if vc.AuthWatchdog != nil {
					vc.AuthWatchdog.Stop()
				}
				if vc.TelemetryWatchdog != nil {
					vc.TelemetryWatchdog.Stop()
				}
				s.sessionMgr.PushSFUEvent("SESSION_ENDED")
				s.sessionMgr.ReleaseSession(sess.ID)
				// Reset to IDLE — clears SAFE_MODE if active (e.g. operator logged out mid-session).
				// SAFE_MODE → IDLE is a valid transition (ADR-011 session teardown path).
				vc.SM.TransitionSystem(statemachine.StateIdle)
			} else {
				s.sessionMgr.ReleaseSession(sess.ID)
			}
		}
	} else {
		// Legacy path (no session_id): ends ALL active sessions fleet-wide, so
		// every active vehicle must be reset to IDLE — not just one resolved via
		// sessionMgr.GetCurrentSession() (an arbitrary ACTIVE_OPERATOR session).
		// Previously only that one vehicle's Machine was reset before
		// EndSession() wiped every session's bookkeeping — any other active
		// vehicle was left stranded in its last state (e.g. SAFE_MODE) with no
		// session left to reset it, recoverable only by restarting the process.
		for _, vehicleID := range s.sessionMgr.ActiveVehicleIDs() {
			sess, ok := s.sessionMgr.GetSessionByVehicle(vehicleID)
			if !ok {
				continue
			}
			s.recorder.EndSession(sess.ID)
			log.Event(logger.EventSessionEnded, "session ended", "session_id", sess.ID, "vehicle_id", vehicleID)
			vc := s.vehicleContexts.Get(vehicleID)
			vc.Deadman.Stop()
			vc.VehicleACKWatchdog.Stop()
			if vc.AuthWatchdog != nil {
				vc.AuthWatchdog.Stop()
			}
			if vc.TelemetryWatchdog != nil {
				vc.TelemetryWatchdog.Stop()
			}
			vc.SM.TransitionSystem(statemachine.StateIdle)
		}
		s.sessionMgr.PushSFUEvent("SESSION_ENDED")
		s.sessionMgr.EndSession()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *controlServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(claimsKey).(string)
	if s.sessionMgr.HasActiveOperatorSession(operatorID) {
		http.Error(w, `{"error":"active_session","message":"Session erst beenden"}`, http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *controlServer) handleHandoverRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID      string `json:"vehicle_id"`
		FromOperatorID string `json:"from_operator_id"`
		ToOperatorID   string `json:"to_operator_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.VehicleID == "" {
		http.Error(w, "vehicle_id required", http.StatusBadRequest)
		return
	}
	if err := s.handoverMgr.RequestHandover(req.VehicleID, req.FromOperatorID, req.ToOperatorID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *controlServer) handleHandoverConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID  string `json:"vehicle_id"`
		OperatorID string `json:"operator_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.VehicleID == "" {
		http.Error(w, "vehicle_id required", http.StatusBadRequest)
		return
	}
	if err := s.handoverMgr.ConfirmHandover(req.VehicleID, req.OperatorID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *controlServer) handleHandoverCancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID string `json:"vehicle_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.VehicleID == "" {
		http.Error(w, "vehicle_id required", http.StatusBadRequest)
		return
	}
	s.handoverMgr.CancelHandover(req.VehicleID)
	w.WriteHeader(http.StatusOK)
}

func (s *controlServer) handleMediaEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID string `json:"vehicle_id"`
		State     string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.VehicleID == "" {
		http.Error(w, "vehicle_id required", http.StatusBadRequest)
		return
	}
	// Media events are reported by the operator's own browser for its own
	// vehicle — resolved directly through the per-vehicle registry, not via
	// sessionMgr.GetCurrentSession() (which used to return an arbitrary
	// ACTIVE_OPERATOR session and could misattribute media state between
	// concurrently active vehicles, ADR-026 follow-up).
	vc := s.vehicleContexts.Get(req.VehicleID)
	switch req.State {
	case "MEDIA_NEGOTIATING":
		vc.SM.TransitionMedia(statemachine.MediaNegotiating)
	case "MEDIA_CONNECTED":
		vc.SM.TransitionMedia(statemachine.MediaConnected)
	case "MEDIA_DEGRADED":
		vc.SM.TransitionMedia(statemachine.MediaDegraded)
	case "MEDIA_FAILED":
		vc.SM.TransitionMedia(statemachine.MediaFailed)
	case "MEDIA_INIT":
		vc.SM.TransitionMedia(statemachine.MediaInit)
	default:
		http.Error(w, "unknown media state", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleEmergencyStop proxies an emergency stop (ADR-009/026).
// With vehicle_id in the body: scoped to that one vehicle.
// Without (empty/missing vehicle_id): fleet-wide — every vehicle with an
// active session goes to SAFE_MODE (shared safety net, ADR-026).
func (s *controlServer) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VehicleID string `json:"vehicle_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // optional body — decode errors are not fatal here

	vehicleIDs := []string{req.VehicleID}
	if req.VehicleID == "" {
		vehicleIDs = s.sessionMgr.ActiveVehicleIDs()
	}

	for _, vehicleID := range vehicleIDs {
		sess, _ := s.sessionMgr.GetSessionByVehicle(vehicleID)
		vc := s.vehicleContexts.Get(vehicleID)
		s.safetyPub.TriggerEmergencyStop(sess.ID, vehicleID, "operator emergency stop")
		vc.SM.TransitionSystem(statemachine.StateSafeMode)
		if sess.ID != "" {
			s.sessionMgr.SaveCheckpoint("SAFE_MODE", "CONTROL_BLOCKED", "EMERGENCY_STOP")
			s.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
			s.recorder.RecordSafetyEvent(recording.SafetyEventParams{
				SessionID:  sess.ID,
				EventID:    ulid.Generate(),
				VehicleID:  vehicleID,
				OperatorID: sess.OperatorID,
				EventType:  "EMERGENCY_STOP",
				Reason:     "operator emergency stop",
			})
			// ADR-020: Control Server kicks MediaMTX subscribers directly on SAFE_MODE
			go s.mtxClient.KickVehicle(vehicleID)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *controlServer) handleLog(w http.ResponseWriter, r *http.Request) {
	var entry struct {
		Level      string         `json:"level"`
		EventType  string         `json:"event_type"`
		SessionID  string         `json:"session_id"`
		VehicleID  string         `json:"vehicle_id"`
		OperatorID string         `json:"operator_id"`
		EventID    string         `json:"event_id"`
		Message    string         `json:"msg"`
		Data       map[string]any `json:"data,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if entry.Message == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	feLog.Event(entry.EventType, entry.Message,
		"session_id", entry.SessionID,
		"vehicle_id", entry.VehicleID,
		"operator_id", entry.OperatorID,
		"event_id", entry.EventID,
	)
	w.WriteHeader(http.StatusAccepted)
}

func (s *controlServer) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}
	events, err := s.auditWriter.QueryBySession(sessionID)
	if err != nil {
		log.Error("audit query failed", "error", err, "session_id", sessionID)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (s *controlServer) handleRecording(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/recording/")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}
	entries := s.recorder.GetEntries(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id": sessionID,
		"count":      len(entries),
		"entries":    entries,
	})
}

// handleMediaAuth validates WHIP publish + WHEP read (ADR-020). MediaMTX
// calls this endpoint for every incoming WHIP/WHEP request.
func (s *controlServer) handleMediaAuth(w http.ResponseWriter, r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	log.Info("media auth: raw request", "body", string(bodyBytes))
	var req struct {
		Action string `json:"action"`
		Path   string `json:"path"`
		Token  string `json:"token"` // Bearer Token (WHIP/WHEP via Authorization header)
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "publish":
		// WHIP: Fahrzeug-Client authentifiziert sich mit Stream Key
		if s.cfg.whipStreamKey == "" || req.Token != s.cfg.whipStreamKey {
			log.Warn("media auth: WHIP publish rejected", "path", req.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Info("media auth: WHIP publish allowed", "path", req.Path)
	case "read":
		// WHEP: Operator-Browser authentifiziert sich mit JWT
		// Prüfung: Token nicht leer + aktive Session für GENAU dieses Fahrzeug
		// (req.Path ist die Vehicle-ID, MediaMTX-Pfadregex "~^vehicle-.*").
		// Vorher: sessionMgr.GetCurrentSession() prüfte nur "irgendeine aktive
		// Session existiert", was mit 2 Fahrzeugen Video-Lesezugriff auf ein
		// fremdes Fahrzeug ohne eigene Session erlaubt hätte (ADR-026 follow-up).
		if req.Token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, hasSession := s.sessionMgr.GetSessionByVehicle(req.Path)
		if !hasSession {
			log.Warn("media auth: WHEP read rejected — no active session for vehicle", "path", req.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Info("media auth: WHEP read allowed", "path", req.Path)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *controlServer) handleVehicleAckLatest(w http.ResponseWriter, r *http.Request) {
	vehicleID := strings.TrimPrefix(r.URL.Path, "/vehicle/ack/latest/")
	if vehicleID == "" {
		http.Error(w, "vehicle_id required", http.StatusBadRequest)
		return
	}
	ack, ok := s.vehicleAckStore.Latest(vehicleID)
	if !ok {
		http.Error(w, "no ack for vehicle", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"vehicle_id":        vehicleID,
		"command_event_id":  ack.CommandEventId,
		"received":          ack.Received,
		"received_at_ms":    ack.ReceivedAtMs,
		"vehicle_connected": s.vehicleRegistry.Connected(vehicleID),
	})
}

// handleDevWhipKey returns the stream key for the browser-based WHIP sender
// (StreamSenderPanel). The stream key is a shared secret known to the
// publisher — exposing it here only saves the developer from manually
// copying it from .env.
func (s *controlServer) handleDevWhipKey(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"streamKey": s.cfg.whipStreamKey})
}

// handleVehiclesList includes each vehicle's live SYSTEM STATE (MV-09) alongside its identity —
// lets the Vehicle-Dropdown show a SAFE_MODE badge without a second per-vehicle request.
// s.vehicleContexts.Get is the same lazily-creating-per-ID lookup handleVehicleState already
// uses for one vehicle (ADR-026: small fleet, contexts are cheap and permanently retained, no
// GC — see tasks/backlog.md MV-10). A vehicle with no state yet defaults to IDLE, same as a
// direct GET /vehicles/{id}/state on a never-connected vehicle.
func (s *controlServer) handleVehiclesList(w http.ResponseWriter, _ *http.Request) {
	vehicles, err := s.vehicleStore.List()
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	type vehicleJSON struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Online      bool   `json:"online"`
		SystemState string `json:"system_state"`
	}
	result := make([]vehicleJSON, len(vehicles))
	for i, v := range vehicles {
		sys, _, _, _ := s.vehicleContexts.Get(v.ID).SM.Get()
		result[i] = vehicleJSON{
			ID: v.ID, DisplayName: v.DisplayName, Description: v.Description, Online: v.Online,
			SystemState: string(sys),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *controlServer) handleVehiclesCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.DisplayName == "" {
		http.Error(w, "id and display_name required", http.StatusBadRequest)
		return
	}
	exists, err := s.vehicleStore.Exists(req.ID)
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "vehicle already exists", http.StatusConflict)
		return
	}
	if err := s.vehicleStore.Add(req.ID, req.DisplayName, req.Description); err != nil {
		http.Error(w, "add failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *controlServer) handleVehicleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if s.sessionMgr.IsVehicleLocked(id) {
		http.Error(w, "vehicle is currently in active session", http.StatusConflict)
		return
	}
	if err := s.vehicleStore.Delete(id); err != nil {
		if errors.Is(err, vehicleregistry.ErrNotFound) {
			http.Error(w, "vehicle not found", http.StatusNotFound)
			return
		}
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSessions returns all currently active sessions (visible to all authenticated users).
func (s *controlServer) handleSessions(w http.ResponseWriter, _ *http.Request) {
	sessions := s.sessionMgr.ListSessions()
	type sessionJSON struct {
		SessionID  string `json:"session_id"`
		VehicleID  string `json:"vehicle_id"`
		OperatorID string `json:"operator_id"`
		Role       string `json:"role"`
		CreatedAt  string `json:"created_at"`
	}
	result := make([]sessionJSON, len(sessions))
	for i, sess := range sessions {
		result[i] = sessionJSON{
			SessionID:  sess.ID,
			VehicleID:  sess.VehicleID,
			OperatorID: sess.OperatorID,
			Role:       sess.OperatorRole,
			CreatedAt:  sess.CreatedAt.Format("15:04:05"),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleVehicleState returns the per-vehicle 4-layer state snapshot (ADR-026).
// MV-12: was previously also reachable via the now-removed legacy GET /state
// (a "whichever session is current" global view) — all consumers migrated to this
// per-vehicle endpoint (frontend since MV-07 already; k6 latency.js and
// tests/integration/services_test.go in the same change that removed GET /state).
func (s *controlServer) handleVehicleState(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.PathValue("id")
	if vehicleID == "" {
		http.Error(w, "vehicle id required", http.StatusBadRequest)
		return
	}
	sys, ctrl, media, op := s.vehicleContexts.Get(vehicleID).SM.Get()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"system":   string(sys),
		"control":  string(ctrl),
		"media":    string(media),
		"operator": string(op),
	})
}

// handleICEConfig returns STUN + TURN (UDP + TCP) servers for WebRTC clients
// (WEBRTC-06 — Sprint 10) so the browser can gather its own ICE candidates.
// No auth required — TURN credentials are per-design visible to anyone who
// loads the page (they're transmitted in WebRTC signalling anyway).
func (s *controlServer) handleICEConfig(w http.ResponseWriter, _ *http.Request) {
	type iceServer struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	}
	host := s.cfg.turnExternalIP
	servers := []iceServer{
		{URLs: []string{"stun:" + host + ":" + s.cfg.turnPort}},
		{URLs: []string{"turn:" + host + ":" + s.cfg.turnPort}, Username: s.cfg.turnUser, Credential: s.cfg.turnPassword},
		{URLs: []string{"turn:" + host + ":" + s.cfg.turnPort + "?transport=tcp"}, Username: s.cfg.turnUser, Credential: s.cfg.turnPassword},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"iceServers": servers})
}

func (s *controlServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "control-server"})
}

type contextKey string

const claimsKey contextKey = "jwt_subject"
const roleKey contextKey = "jwt_role"

// requireJWT returns middleware that enforces a valid operator JWT in the Authorization header.
// Stores the JWT subject (operator ID) and role in the request context for downstream handlers.
// Returns 401 when the token is missing, malformed, or signed with a different secret.
func requireJWT(secret []byte) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token, err := jwt.Parse(strings.TrimPrefix(authHeader, "Bearer "),
				func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return secret, nil
				})
			if err != nil || !token.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			subject, _ := token.Claims.GetSubject()
			role, _ := token.Claims.(jwt.MapClaims)["role"].(string)
			ctx := context.WithValue(r.Context(), claimsKey, subject)
			ctx = context.WithValue(ctx, roleKey, role)
			next(w, r.WithContext(ctx))
		}
	}
}
