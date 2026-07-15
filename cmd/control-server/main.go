package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/transport"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/mediamtx"
	"avoc/internal/recording"
	"avoc/internal/vehicleconnection"
	"avoc/internal/vehicleregistry"
	"avoc/pkg/audit"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
)

var log = logger.New("control-server")

// feLog logs frontend events received via POST /log (service label: "frontend")
var feLog = logger.New("frontend")

func main() {
	port := envOr("CONTROL_PORT", "8080")
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	safetyURL := envOr("SAFETY_SERVICE_URL", "http://safety-service:8082")
	sfuURL := envOr("SFU_SERVICE_URL", "http://webrtc-sfu:8084")
	authURL := envOr("AUTH_SERVICE_URL", "http://auth-service:8081")
	whipStreamKey := os.Getenv("WHIP_STREAM_KEY")
	mediamtxAPIURL := envOr("MEDIAMTX_API_URL", "http://mediamtx:9997")
	turnExternalIP := os.Getenv("TURN_EXTERNAL_IP")
	turnPort := envOr("TURN_PORT", "3478")
	turnUser := os.Getenv("TURN_USER")
	turnPassword := os.Getenv("TURN_PASSWORD")
	mtxClient := mediamtx.NewClient(mediamtxAPIURL)

	// --- PostgreSQL (ADR-023) ---
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	db, err := pkgdb.Open(databaseURL)
	if err != nil {
		log.Fatal("failed to open database", "error", err)
	}
	defer db.Close()

	// Docker's `restart: unless-stopped` policy (unlike `docker compose up`)
	// does not honor `depends_on: service_healthy` — a crash-restarted
	// control-server can otherwise race Postgres's own startup and get stuck
	// in permanent degraded mode (NoopWriter/NoopVehicleStore) until manually
	// restarted (Sprint 18 Bugfix, found after ICE-Fix-Redeploy 2026-07-09).
	if err := pkgdb.WaitForReady(db, pkgdb.DefaultConnectRetries, pkgdb.DefaultConnectRetryDelay); err != nil {
		log.Warn("database not reachable after retries — starting in degraded mode", "error", err)
	}

	// --- Audit Writer (LOG-10/11 — ADR-018/023) ---
	var auditWriter audit.AuditWriter
	pgWriter, err := audit.NewPostgresAuditWriter(db)
	if err != nil {
		log.Warn("audit store unavailable — using NoopWriter", "error", err)
		auditWriter = audit.NewNoopWriter()
	} else {
		auditWriter = pgWriter
		defer pgWriter.Close()
		log.Info("audit store ready (PostgreSQL)")
	}

	// --- Core components ---
	safetyPub := csafety.NewHTTPPublisher(safetyURL)
	sfuPub := session.NewHTTPSFUPublisher(sfuURL)
	sessionMgr := session.NewManager(sfuPub)

	// Per-vehicle State Machine + Watchdogs (ADR-026) — replaces the single
	// process-wide sm/deadman/ackWatcher/vehicleACKWatchdog singletons that
	// caused two vehicles to silently share (and overwrite) safety monitoring.
	vehicleContexts := vehiclecontext.NewRegistry(
		csafety.DefaultDeadmanTimeout, csafety.DefaultACKTimeout, csafety.DefaultVehicleACKTimeout, safetyPub,
	).WithAuditWriter(auditWriter)

	// HandoverManager resolves each vehicle's own State Machine via the same
	// per-vehicle registry as everything else (ADR-026 follow-up, MV-11) — no
	// longer a disconnected standalone Machine shared across all vehicles.
	handoverMgr := session.NewHandoverManager(vehicleContexts, sessionMgr, authURL)
	recorder := recording.NewMemoryRecorder()
	vehicleRegistry := vehicleconnection.NewRegistry()
	vehicleAckStore := vehicleconnection.NewAckStore()

	// --- Vehicle Registry (ADR-022/023) ---
	var vehicleStore vehicleregistry.VehicleStore
	vs, vsErr := vehicleregistry.NewPostgresVehicleStore(db, vehicleRegistry)
	if vsErr != nil {
		log.Warn("vehicle registry unavailable", "error", vsErr)
		vehicleStore = vehicleregistry.NoopVehicleStore{}
	} else {
		vehicleStore = vs
		if seedErr := vs.SeedDefault(); seedErr != nil {
			log.Warn("vehicle registry seed failed", "error", seedErr)
		} else {
			log.Info("vehicle registry ready, vehicle-001 seeded")
		}
	}

	// SafetyBusWatchdog stays a single process-wide instance — there is only
	// one safety-service (ADR-002) — but fans a failure out to every active
	// vehicle (ADR-026). Starts once for the process lifetime, not per session.
	safetyBusWatchdog := csafety.NewSafetyBusWatchdog(
		safetyURL+"/health", csafety.DefaultBusCheckInterval, csafety.DefaultBusFailThreshold,
		vehicleContexts, sessionMgr, safetyPub,
	)
	safetyBusWatchdog.Start()

	cmdEngine := command.NewEngine(vehicleContexts, safetyPub, sessionMgr).
		WithAuditWriter(auditWriter).
		WithVehicleForwarder(vehicleRegistry)

	// --- Handlers ---
	wsHandler := transport.NewWSHandler(secret, vehicleContexts, sessionMgr, cmdEngine).
		WithAuditWriter(auditWriter)
	vehicleHandler := vehicleconnection.NewHandler(secret, vehicleContexts, safetyPub, vehicleRegistry, vehicleAckStore).
		WithVehicleAdder(vehicleStore)

	// --- Routes ---
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", wsHandler.ServeWS)
	mux.HandleFunc("/vehicle/ws", vehicleHandler.ServeWS)

	auth := requireJWT([]byte(secret))

	// Session lifecycle (GSA — ADR-015/025)
	// POST /session/start: creates a session for the given vehicle + operator.
	// Returns ACTIVE_OPERATOR if the vehicle is free; OBSERVER if already controlled (ADR-025).
	// The state machine is only advanced for ACTIVE_OPERATOR sessions.
	// WS must be connected AFTER this call using the returned session_id.
	mux.HandleFunc("POST /session/start", auth(func(w http.ResponseWriter, r *http.Request) {
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
		if callerRole == "OBSERVER" && !sessionMgr.IsVehicleLocked(req.VehicleID) {
			http.Error(w, "observers cannot start a new session", http.StatusForbidden)
			return
		}

		if !vehicleRegistry.Connected(req.VehicleID) {
			http.Error(w, "vehicle not connected", http.StatusConflict)
			return
		}

		sess := sessionMgr.StartSession(req.VehicleID, req.OperatorID)

		if sess.OperatorRole == "ACTIVE_OPERATOR" {
			// Advance THIS VEHICLE's state machine: IDLE → CONNECTING → AUTHENTICATED → CONNECTED (ADR-026).
			vc := vehicleContexts.Get(sess.VehicleID)
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
			sessionMgr.PushSFUEvent("SESSION_CREATED")
			recorder.StartSession(sess.ID, sess.VehicleID, sess.OperatorID)
			sys, ctrl, _, _ := vc.SM.Get()
			recorder.RecordStateSnapshot(sess.ID, ulid.Generate(), sess.VehicleID, sess.OperatorID, string(sys), string(ctrl))
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
	}))

	mux.HandleFunc("POST /session/end", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SessionID string `json:"session_id"`
		}
		// session_id is optional for backward compat — falls back to current ACTIVE_OPERATOR session.
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.SessionID != "" {
			if sess, ok := sessionMgr.GetSession(req.SessionID); ok {
				recorder.EndSession(sess.ID)
				log.Event(logger.EventSessionEnded, "session ended",
					"session_id", sess.ID, "role", sess.OperatorRole)
				if sess.OperatorRole == "ACTIVE_OPERATOR" {
					vc := vehicleContexts.Get(sess.VehicleID)
					vc.Deadman.Stop()
					vc.VehicleACKWatchdog.Stop()
					sessionMgr.PushSFUEvent("SESSION_ENDED")
					sessionMgr.ReleaseSession(sess.ID)
					// Reset to IDLE — clears SAFE_MODE if active (e.g. operator logged out mid-session).
					// SAFE_MODE → IDLE is a valid transition (ADR-011 session teardown path).
					vc.SM.TransitionSystem(statemachine.StateIdle)
				} else {
					sessionMgr.ReleaseSession(sess.ID)
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
			for _, vehicleID := range sessionMgr.ActiveVehicleIDs() {
				sess, ok := sessionMgr.GetSessionByVehicle(vehicleID)
				if !ok {
					continue
				}
				recorder.EndSession(sess.ID)
				log.Event(logger.EventSessionEnded, "session ended", "session_id", sess.ID, "vehicle_id", vehicleID)
				vc := vehicleContexts.Get(vehicleID)
				vc.Deadman.Stop()
				vc.VehicleACKWatchdog.Stop()
				vc.SM.TransitionSystem(statemachine.StateIdle)
			}
			sessionMgr.PushSFUEvent("SESSION_ENDED")
			sessionMgr.EndSession()
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// Logout gate — rejects logout while an ACTIVE_OPERATOR session is live (ADR-025).
	// Frontend disables the button, but this endpoint enforces the rule server-side.
	// Routed via nginx /api/ → control-server (not /auth/ which goes to auth-service).
	mux.HandleFunc("POST /logout", auth(func(w http.ResponseWriter, r *http.Request) {
		operatorID, _ := r.Context().Value(claimsKey).(string)
		if sessionMgr.HasActiveOperatorSession(operatorID) {
			http.Error(w, `{"error":"active_session","message":"Session erst beenden"}`, http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// Operator Handover (BE-12), scoped per vehicle (ADR-026 follow-up, MV-11).
	mux.HandleFunc("POST /handover/request", auth(func(w http.ResponseWriter, r *http.Request) {
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
		if err := handoverMgr.RequestHandover(req.VehicleID, req.FromOperatorID, req.ToOperatorID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	mux.HandleFunc("POST /handover/confirm", auth(func(w http.ResponseWriter, r *http.Request) {
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
		if err := handoverMgr.ConfirmHandover(req.VehicleID, req.OperatorID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("POST /handover/cancel", auth(func(w http.ResponseWriter, r *http.Request) {
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
		handoverMgr.CancelHandover(req.VehicleID)
		w.WriteHeader(http.StatusOK)
	}))

	// MEDIA STATE update (ADR-009/011 Invariant 1)
	mux.HandleFunc("POST /media/event", auth(func(w http.ResponseWriter, r *http.Request) {
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
		vc := vehicleContexts.Get(req.VehicleID)
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
	}))

	// Emergency Stop proxy (ADR-009/026).
	// With vehicle_id in the body: scoped to that one vehicle.
	// Without (empty/missing vehicle_id): fleet-wide — every vehicle with an
	// active session goes to SAFE_MODE (shared safety net, ADR-026).
	mux.HandleFunc("POST /emergency-stop", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			VehicleID string `json:"vehicle_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req) // optional body — decode errors are not fatal here

		vehicleIDs := []string{req.VehicleID}
		if req.VehicleID == "" {
			vehicleIDs = sessionMgr.ActiveVehicleIDs()
		}

		for _, vehicleID := range vehicleIDs {
			sess, _ := sessionMgr.GetSessionByVehicle(vehicleID)
			vc := vehicleContexts.Get(vehicleID)
			safetyPub.TriggerEmergencyStop(sess.ID, vehicleID, "operator emergency stop")
			vc.SM.TransitionSystem(statemachine.StateSafeMode)
			if sess.ID != "" {
				sessionMgr.SaveCheckpoint("SAFE_MODE", "CONTROL_BLOCKED", "EMERGENCY_STOP")
				sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
				recorder.RecordSafetyEvent(sess.ID, ulid.Generate(), vehicleID, sess.OperatorID, "EMERGENCY_STOP", "operator emergency stop")
				// ADR-020: Control Server kicks MediaMTX subscribers directly on SAFE_MODE
				go mtxClient.KickVehicle(vehicleID)
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	// LOG-07: Frontend log ingestion — Browser → POST /log → slog (service="frontend") → Loki
	mux.HandleFunc("POST /log", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// LOG-11: Audit events query endpoint
	mux.HandleFunc("GET /audit/events", auth(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}
		events, err := auditWriter.QueryBySession(sessionID)
		if err != nil {
			log.Error("audit query failed", "error", err, "session_id", sessionID)
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}))

	// Recording inspection (ADR-005)
	mux.HandleFunc("GET /recording/", auth(func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/recording/")
		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}
		entries := recorder.GetEntries(sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"count":      len(entries),
			"entries":    entries,
		})
	}))

	// MediaMTX Auth-Hook (ADR-020) — validiert WHIP publish + WHEP read
	// MediaMTX ruft diesen Endpoint für jeden eingehenden WHIP/WHEP-Request auf.
	mux.HandleFunc("POST /internal/media/auth", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		log.Info("media auth: raw request", "body", string(bodyBytes))
		var req struct {
			Action string `json:"action"`
			Path   string `json:"path"`
			Token  string `json:"token"`  // Bearer Token (WHIP/WHEP via Authorization header)
		}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "publish":
			// WHIP: Fahrzeug-Client authentifiziert sich mit Stream Key
			if whipStreamKey == "" || req.Token != whipStreamKey {
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
			_, hasSession := sessionMgr.GetSessionByVehicle(req.Path)
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
	})

	// VEH-07 (ADR-021): Latest VehicleCommandAck per vehicle — polled by frontend
	mux.HandleFunc("GET /vehicle/ack/latest/", func(w http.ResponseWriter, r *http.Request) {
		vehicleID := strings.TrimPrefix(r.URL.Path, "/vehicle/ack/latest/")
		if vehicleID == "" {
			http.Error(w, "vehicle_id required", http.StatusBadRequest)
			return
		}
		ack, ok := vehicleAckStore.Latest(vehicleID)
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
			"vehicle_connected": vehicleRegistry.Connected(vehicleID),
		})
	})

	// DEV: Stream key for browser-based WHIP sender (StreamSenderPanel).
	// The stream key is a shared secret known to the publisher — exposing it here
	// only saves the developer from manually copying it from .env.
	mux.HandleFunc("GET /dev/whip-key", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"streamKey": whipStreamKey})
	})

	// Vehicle Registry (ADR-022)
	mux.HandleFunc("GET /vehicles", func(w http.ResponseWriter, _ *http.Request) {
		vehicles, err := vehicleStore.List()
		if err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		type vehicleJSON struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
			Online      bool   `json:"online"`
		}
		result := make([]vehicleJSON, len(vehicles))
		for i, v := range vehicles {
			result[i] = vehicleJSON{ID: v.ID, DisplayName: v.DisplayName, Description: v.Description, Online: v.Online}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("POST /vehicles", auth(func(w http.ResponseWriter, r *http.Request) {
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
		exists, err := vehicleStore.Exists(req.ID)
		if err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "vehicle already exists", http.StatusConflict)
			return
		}
		if err := vehicleStore.Add(req.ID, req.DisplayName, req.Description); err != nil {
			http.Error(w, "add failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))

	mux.HandleFunc("DELETE /vehicles/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if sessionMgr.IsVehicleLocked(id) {
			http.Error(w, "vehicle is currently in active session", http.StatusConflict)
			return
		}
		if err := vehicleStore.Delete(id); err != nil {
			if errors.Is(err, vehicleregistry.ErrNotFound) {
				http.Error(w, "vehicle not found", http.StatusNotFound)
				return
			}
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// GET /sessions — returns all currently active sessions (visible to all authenticated users).
	mux.HandleFunc("GET /sessions", auth(func(w http.ResponseWriter, _ *http.Request) {
		sessions := sessionMgr.ListSessions()
		type sessionJSON struct {
			SessionID  string `json:"session_id"`
			VehicleID  string `json:"vehicle_id"`
			OperatorID string `json:"operator_id"`
			Role       string `json:"role"`
			CreatedAt  string `json:"created_at"`
		}
		result := make([]sessionJSON, len(sessions))
		for i, s := range sessions {
			result[i] = sessionJSON{
				SessionID:  s.ID,
				VehicleID:  s.VehicleID,
				OperatorID: s.OperatorID,
				Role:       s.OperatorRole,
				CreatedAt:  s.CreatedAt.Format("15:04:05"),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))

	// State + Health
	// GET /state — legacy global view, kept for backward compat (k6 latency.js,
	// older clients). Resolves through the per-vehicle registry via whichever
	// session is "current" (ADR-026) — no session yet means everything IDLE,
	// matching pre-ADR-026 startup behavior exactly.
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]string{
			"system":   string(statemachine.StateIdle),
			"control":  string(statemachine.ControlInit),
			"media":    string(statemachine.MediaInit),
			"operator": string(statemachine.OpNoOperator),
		}
		if sess, ok := sessionMgr.GetCurrentSession(); ok {
			vc := vehicleContexts.Get(sess.VehicleID)
			sys, ctrl, media, op := vc.SM.Get()
			resp["system"] = string(sys)
			resp["control"] = string(ctrl)
			resp["media"] = string(media)
			resp["operator"] = string(op)
			resp["session_id"] = sess.ID
			resp["vehicle_id"] = sess.VehicleID
			resp["role"] = sess.OperatorRole
			resp["operator_id"] = sess.OperatorID
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /vehicles/{id}/state — per-vehicle 4-layer state snapshot (ADR-026).
	// Replaces GET /state for live polling once a vehicleId is known.
	mux.HandleFunc("GET /vehicles/{id}/state", func(w http.ResponseWriter, r *http.Request) {
		vehicleID := r.PathValue("id")
		if vehicleID == "" {
			http.Error(w, "vehicle id required", http.StatusBadRequest)
			return
		}
		sys, ctrl, media, op := vehicleContexts.Get(vehicleID).SM.Get()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"system":   string(sys),
			"control":  string(ctrl),
			"media":    string(media),
			"operator": string(op),
		})
	})

	// ICE server config for WebRTC clients (WEBRTC-06 — Sprint 10).
	// Returns STUN + TURN (UDP + TCP) servers so the browser can gather its own
	// ICE candidates. No auth required — TURN credentials are per-design visible
	// to anyone who loads the page (they're transmitted in WebRTC signalling anyway).
	mux.HandleFunc("GET /ice-config", func(w http.ResponseWriter, _ *http.Request) {
		type iceServer struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		}
		host := turnExternalIP
		servers := []iceServer{
			{URLs: []string{"stun:" + host + ":" + turnPort}},
			{URLs: []string{"turn:" + host + ":" + turnPort}, Username: turnUser, Credential: turnPassword},
			{URLs: []string{"turn:" + host + ":" + turnPort + "?transport=tcp"}, Username: turnUser, Credential: turnPassword},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"iceServers": servers})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "control-server"})
	})

	log.Info("Control Server starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Control Server failed", "error", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
