package telemetrycheck

import (
	"context"
	"sync"
	"time"

	"avoc/internal/controlserver/statemachine"
	"avoc/pkg/logger"
)

var svcLog = logger.New("control-server")

const (
	// DefaultTelemetryCheckInterval/DefaultTelemetryFailThreshold — ADR-009 Update 2026-07-21
	// Grill-Me: 2s poll (faster than AuthWatchdog/SafetyBusWatchdog's 5s — telemetry is the
	// operator's main situational-awareness feed) × 2 consecutive failures = 4s budget until
	// DEGRADED, same budget used as the freshness cutoff (see maxAge in run()).
	DefaultTelemetryCheckInterval = 2 * time.Second
	DefaultTelemetryFailThreshold = 2
)

// TelemetryWatchdog periodically polls telemetry-service for a vehicle's telemetry freshness
// (ADR-009 Update 2026-07-20/21, DRIFT-K3-TELEMETRY Teil 2). Per-VehicleContext, tied to one
// session, matching AuthWatchdog/Deadman/VehicleACKWatchdog's Start/Stop lifecycle (ADR-026
// per-vehicle isolation) — unlike SafetyBusWatchdog, telemetry loss on one vehicle must not
// affect any other vehicle's session.
//
// Unlike AuthWatchdog, firing does not stop the polling loop: a telemetry outage is reversible
// and does not end the session (only a revoked operator account does), so the watchdog keeps
// polling for recovery exactly like SafetyBusWatchdog does after a fleet-wide trigger.
//
// Deliberately does NOT use safety.Publisher or audit.SafetyAuditWriter — DEGRADED-only causes
// are not part of the Safety Event Bus or the audit trail in this codebase (that channel is
// reserved for CRITICAL/SAFE_MODE-triggering events, e.g. AuthWatchdog/Deadman/ACKTimeoutWatcher/
// SafetyBusWatchdog). TransitionMedia — the existing DEGRADED-only cause — never publishes or
// audit-writes either (cmd/control-server/main.go's MediaDegraded/MediaFailed call sites), and
// TransitionTelemetry already does its own structured logging (statemachine/state.go). Following
// that precedent instead of the Publisher/AuditWriter wiring sketched in the original task
// breakdown (tasks/sprints/50-telemetry-watchdog.md) — see its TW-04 note.
// freshnessChecker abstracts Checker.HasFreshTelemetry for watchdog-level tests — the real HTTP
// behavior is already covered by checker_test.go; watchdog tests use a fake to exercise only the
// polling/threshold/firing logic, analog AuthWatchdog's split against fakeUserChecker.
type freshnessChecker interface {
	HasFreshTelemetry(ctx context.Context, vehicleID string, maxAge time.Duration) (bool, error)
}

type TelemetryWatchdog struct {
	mu        sync.Mutex
	interval  time.Duration
	threshold int
	sm        *statemachine.Machine
	checker   freshnessChecker
	cancel    context.CancelFunc
	sessionID string
}

func NewTelemetryWatchdog(interval time.Duration, threshold int, sm *statemachine.Machine, checker freshnessChecker) *TelemetryWatchdog {
	return &TelemetryWatchdog{
		interval:  interval,
		threshold: threshold,
		sm:        sm,
		checker:   checker,
	}
}

// Start begins polling for the given session's vehicle. Call on session/start.
func (w *TelemetryWatchdog) Start(sessionID, vehicleID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.sessionID = sessionID
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.run(ctx, sessionID, vehicleID)
	svcLog.Info("telemetry watchdog started", "session_id", sessionID, "vehicle_id", vehicleID)
}

// Stop cancels the polling goroutine. Call on session/end.
func (w *TelemetryWatchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	svcLog.Info("telemetry watchdog stopped", "session_id", w.sessionID)
}

func (w *TelemetryWatchdog) run(ctx context.Context, sessionID, vehicleID string) {
	maxAge := w.interval * time.Duration(w.threshold)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fresh, err := w.checker.HasFreshTelemetry(ctx, vehicleID, maxAge)
			if err == nil && fresh {
				if failures > 0 {
					svcLog.Info("telemetry watchdog recovered", "session_id", sessionID, "vehicle_id", vehicleID, "after_failures", failures)
				}
				failures = 0
				w.sm.TransitionTelemetry(true)
				continue
			}
			failures++
			svcLog.Warn("telemetry watchdog check failed",
				"session_id", sessionID, "vehicle_id", vehicleID,
				"consecutive_failures", failures, "threshold", w.threshold, "error", err)
			if failures >= w.threshold {
				svcLog.Event(logger.EventTelemetryWatchdogTriggered,
					"telemetry stale or missing — DEGRADED",
					"session_id", sessionID, "vehicle_id", vehicleID, "threshold", w.threshold)
				w.sm.TransitionTelemetry(false)
				failures = 0 // avoid re-triggering every tick while still down (SafetyBusWatchdog pattern)
			}
		}
	}
}
