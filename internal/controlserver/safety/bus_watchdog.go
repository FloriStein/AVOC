package safety

import (
	"context"
	"net/http"
	"sync"
	"time"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"
	"avoc/pkg/logger"
)

const (
	DefaultBusCheckInterval = 5 * time.Second
	DefaultBusFailThreshold = 2 // consecutive failures before SAFE_MODE (= 10s total)
)

// VehicleStates provides per-vehicle State Machine access. Satisfied by
// vehiclecontext.Registry's StateMachine(vehicleID) method — defined here
// (not imported) because vehiclecontext already imports this package for the
// watchdog types, and importing it back would create a cycle (ADR-026).
type VehicleStates interface {
	StateMachine(vehicleID string) *statemachine.Machine
}

// SessionSource lists the vehicles that currently have a live session, so a
// fleet-wide failure can be fanned out to every affected vehicle (ADR-026).
// Satisfied by session.Manager's ActiveVehicleIDs().
type SessionSource interface {
	ActiveVehicleIDs() []string
}

// SafetyBusWatchdog periodically GETs the safety-service /health endpoint.
// It is a single process-wide instance — there is only one safety-service
// (ADR-002) — but on failure it transitions EVERY vehicle with an active
// session to SAFE_MODE, not just one (ADR-026: the safety bus is shared
// infrastructure, so its failure is a fleet-wide event).
type SafetyBusWatchdog struct {
	mu        sync.Mutex
	healthURL string
	interval  time.Duration
	threshold int
	vehicles  VehicleStates
	sessions  SessionSource
	publisher Publisher
	cancel    context.CancelFunc
	client    *http.Client
}

// SafetyBusWatchdogOptions bundles NewSafetyBusWatchdog's inputs (Rule 2.3 — more than 4 params).
type SafetyBusWatchdogOptions struct {
	HealthURL string
	Interval  time.Duration
	Threshold int
	Vehicles  VehicleStates
	Sessions  SessionSource
	Publisher Publisher
}

func NewSafetyBusWatchdog(opts SafetyBusWatchdogOptions) *SafetyBusWatchdog {
	return &SafetyBusWatchdog{
		healthURL: opts.HealthURL,
		interval:  opts.Interval,
		threshold: opts.Threshold,
		vehicles:  opts.Vehicles,
		sessions:  opts.Sessions,
		publisher: opts.Publisher,
		client:    &http.Client{Timeout: 3 * time.Second},
	}
}

// Start begins health polling for the process lifetime. Call once at startup
// (or again after Stop() for a clean restart, e.g. in tests).
func (w *SafetyBusWatchdog) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.run(ctx)
	svcLog.Info("safety bus watchdog started", "url", w.healthURL, "interval", w.interval)
}

// Stop cancels the polling goroutine.
func (w *SafetyBusWatchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	svcLog.Info("safety bus watchdog stopped")
}

func (w *SafetyBusWatchdog) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.ping() {
				if failures > 0 {
					svcLog.Info("safety bus recovered", "after_failures", failures)
				}
				failures = 0
			} else {
				failures++
				svcLog.Warn("safety bus health check failed",
					"consecutive_failures", failures, "threshold", w.threshold)
				if failures >= w.threshold {
					w.triggerSafeModeFleetWide()
					failures = 0 // avoid re-triggering every tick while still down
				}
			}
		}
	}
}

func (w *SafetyBusWatchdog) ping() bool {
	resp, err := w.client.Get(w.healthURL)
	if err != nil {
		return false
	}
	if err := resp.Body.Close(); err != nil {
		svcLog.Warn("failed to close health-check response body", "error", err)
	}
	return resp.StatusCode == http.StatusOK
}

// triggerSafeModeFleetWide transitions every vehicle with a currently active
// session to SAFE_MODE — skips vehicles already there (idempotent, no
// duplicate events).
func (w *SafetyBusWatchdog) triggerSafeModeFleetWide() {
	for _, vehicleID := range w.sessions.ActiveVehicleIDs() {
		sm := w.vehicles.StateMachine(vehicleID)
		sys, _, _, _ := sm.Get()
		if sys == statemachine.StateSafeMode {
			continue
		}
		svcLog.Event(logger.EventSafetyBusDown,
			"safety bus unreachable → SAFE_MODE",
			"vehicle_id", vehicleID, "url", w.healthURL, "threshold", w.threshold)
		sm.TransitionSystem(statemachine.StateSafeMode)
		w.publisher.PublishEvent(safetyservice.SafetyEvent{
			VehicleID: vehicleID,
			Type:      safetyservice.EventSafetyBusDown,
			Reason:    "safety bus unreachable — health check failed",
			Timestamp: time.Now(),
		})
	}
}
