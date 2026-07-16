package safety

import (
	"context"
	"sync"
	"time"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"
	"avoc/pkg/audit"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
)

const (
	// DefaultAuthCheckInterval/DefaultAuthFailThreshold mirror SafetyBusWatchdog's
	// 5s×2(=10s) trade-off (2026-07-16 Grill-Me: same risk posture as the existing,
	// already-accepted watchdog for a comparable failure class — a transient DB
	// blip must not drop an operator mid-maneuver, but a genuinely revoked/deleted
	// account should not keep control much longer than that either).
	DefaultAuthCheckInterval  = 5 * time.Second
	DefaultAuthFailThreshold = 2
)

// UserChecker reports whether the named operator account still exists and is
// active. Satisfied by authcheck.Checker — defined here (not imported) to avoid
// a dependency from safety (imported by vehiclecontext) back into authcheck,
// same reasoning as VehicleStates/SessionSource in bus_watchdog.go (ADR-026).
type UserChecker interface {
	IsActiveOperator(ctx context.Context, username string) (bool, error)
}

// AuthWatchdog periodically verifies that the operator account behind the
// current session still exists and is active (DRIFT-K1, ADR-009 "Auth
// Invalidation"). Unlike SafetyBusWatchdog (one shared instance, fleet-wide
// fanout — the safety bus is shared infrastructure), AuthWatchdog is
// per-VehicleContext and tied to one session's operator, matching
// Deadman/VehicleACKWatchdog's Start/Stop lifecycle (ADR-026 per-vehicle
// isolation — revoking one operator's account must not affect any other
// vehicle's session).
type AuthWatchdog struct {
	mu          sync.Mutex
	interval    time.Duration
	threshold   int
	sm          *statemachine.Machine
	publisher   Publisher
	auditWriter audit.AuditWriter
	checker     UserChecker
	cancel      context.CancelFunc
	sessionID   string
	vehicleID   string
	username    string
}

func NewAuthWatchdog(interval time.Duration, threshold int, sm *statemachine.Machine, publisher Publisher, checker UserChecker) *AuthWatchdog {
	return &AuthWatchdog{
		interval:  interval,
		threshold: threshold,
		sm:        sm,
		publisher: publisher,
		checker:   checker,
	}
}

// WithAuditWriter sets the audit writer for guaranteed safety-event persistence (ADR-018).
func (w *AuthWatchdog) WithAuditWriter(aw audit.AuditWriter) *AuthWatchdog {
	w.auditWriter = aw
	return w
}

// Start begins polling for the given session's operator account. Call on
// session/start (and on WS-reconnect recovery, mirroring Deadman.Start).
func (w *AuthWatchdog) Start(sessionID, vehicleID, username string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.sessionID = sessionID
	w.vehicleID = vehicleID
	w.username = username
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.run(ctx, sessionID, vehicleID, username)
	svcLog.Info("auth watchdog started", "session_id", sessionID, "vehicle_id", vehicleID, "username", username)
}

// Stop cancels the polling goroutine. Call on session/end or SAFE_MODE entry.
func (w *AuthWatchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	svcLog.Info("auth watchdog stopped", "session_id", w.sessionID)
}

func (w *AuthWatchdog) run(ctx context.Context, sessionID, vehicleID, username string) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			active, err := w.checker.IsActiveOperator(ctx, username)
			if err == nil && active {
				if failures > 0 {
					svcLog.Info("auth watchdog recovered", "session_id", sessionID, "after_failures", failures)
				}
				failures = 0
				continue
			}
			failures++
			svcLog.Warn("auth watchdog check failed",
				"session_id", sessionID, "username", username,
				"consecutive_failures", failures, "threshold", w.threshold, "error", err)
			if failures >= w.threshold {
				w.fire(sessionID, vehicleID, username)
				return // account is gone — no point in continuing to poll this session
			}
		}
	}
}

func (w *AuthWatchdog) fire(sessionID, vehicleID, username string) {
	svcLog.Event(logger.EventAuthWatchdogTriggered,
		"operator account no longer active — CRITICAL → SAFE_MODE",
		"session_id", sessionID, "vehicle_id", vehicleID, "username", username)

	if w.auditWriter != nil {
		sys, ctrl, _, _ := w.sm.Get()
		if err := w.auditWriter.WriteSync(audit.SafetyAuditEvent{
			EventID:     ulid.Generate(),
			SessionID:   sessionID,
			VehicleID:   vehicleID,
			EventType:   logger.EventAuthWatchdogTriggered,
			Reason:      "operator account revoked or deleted (admin action)",
			SystemState: string(sys),
			CtrlState:   string(ctrl),
			Timestamp:   time.Now(),
		}); err != nil {
			svcLog.Error("audit write failed — proceeding to SAFE_MODE", "error", err)
		}
	}

	// Routes through TransitionOperator, not TransitionSystem directly (DRIFT-K2's
	// hardened path) — a revoked/deleted operator account IS the "no active
	// operator" condition from the state machine's perspective, so this reuses
	// the same guarded transition as the WS-disconnect handler instead of a
	// second, parallel way into SAFE_MODE.
	w.sm.TransitionOperator(statemachine.OpNoOperator)
	w.publisher.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sessionID,
		VehicleID: vehicleID,
		Type:      safetyservice.EventAuthInvalid,
		Reason:    "operator account revoked or deleted (admin action)",
		Timestamp: time.Now(),
	})
}
