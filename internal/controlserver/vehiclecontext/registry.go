// Package vehiclecontext isolates safety-critical state per vehicle (ADR-026).
// Before this package existed, the State Machine and all three Watchdogs
// (Deadman, ACKTimeoutWatcher, VehicleACKWatchdog) were single process-wide
// singletons — two operators on two different vehicles would silently
// overwrite each other's safety monitoring. Registry fixes this by lazily
// creating and permanently retaining one VehicleContext per vehicle ID.
package vehiclecontext

import (
	"sync"
	"time"

	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/telemetrycheck"
	"avoc/pkg/audit"
)

// VehicleContext bundles the safety-critical state for exactly ONE vehicle.
// AuthWatchdog/TelemetryWatchdog are nil unless the Registry was given a
// UserChecker/telemetrycheck.Checker respectively — callers must nil-check
// before Start()/Stop(), same as any optional dependency (mirrors
// auditWriter's nil-safe handling in each watchdog). In production both are
// always configured (cmd/control-server/main.go) — the nil-checks exist so
// unrelated tests that construct a Registry without a TELEMETRY_SERVICE_URL/
// UserChecker don't need to care about either watchdog.
type VehicleContext struct {
	SM                 *statemachine.Machine
	Deadman            *csafety.DeadmanWatchdog
	ACKTimeoutWatcher  *csafety.ACKTimeoutWatcher
	VehicleACKWatchdog *csafety.VehicleACKWatchdog
	AuthWatchdog       *csafety.AuthWatchdog
	TelemetryWatchdog  *telemetrycheck.TelemetryWatchdog
}

// Registry lazily creates and permanently retains one VehicleContext per
// vehicle ID. Per ADR-026: small, known fleet — no GC, no eviction.
type Registry struct {
	mu                 sync.Mutex
	contexts           map[string]*VehicleContext
	deadmanTimeout     time.Duration
	ackTimeout         time.Duration
	vehicleACKTimeout  time.Duration
	authInterval       time.Duration
	authThreshold      int
	telemetryInterval  time.Duration
	telemetryThreshold int
	publisher          csafety.Publisher
	auditWriter        audit.SafetyAuditWriter
	userChecker        csafety.UserChecker
	telemetryChecker   *telemetrycheck.Checker
}

func NewRegistry(deadmanTimeout, ackTimeout, vehicleACKTimeout time.Duration, publisher csafety.Publisher) *Registry {
	return &Registry{
		contexts:           make(map[string]*VehicleContext),
		deadmanTimeout:     deadmanTimeout,
		ackTimeout:         ackTimeout,
		vehicleACKTimeout:  vehicleACKTimeout,
		authInterval:       csafety.DefaultAuthCheckInterval,
		authThreshold:      csafety.DefaultAuthFailThreshold,
		telemetryInterval:  telemetrycheck.DefaultTelemetryCheckInterval,
		telemetryThreshold: telemetrycheck.DefaultTelemetryFailThreshold,
		publisher:          publisher,
	}
}

// WithAuditWriter sets the audit writer applied to every VehicleContext's
// watchdogs (ADR-018). Call before the first Get().
func (r *Registry) WithAuditWriter(aw audit.SafetyAuditWriter) *Registry {
	r.auditWriter = aw
	return r
}

// WithUserChecker enables AuthWatchdog on every VehicleContext (DRIFT-K1). Call
// before the first Get() — without it, VehicleContext.AuthWatchdog stays nil.
func (r *Registry) WithUserChecker(checker csafety.UserChecker) *Registry {
	r.userChecker = checker
	return r
}

// WithAuthWatchdogTiming overrides the default 5s×2 poll/threshold (tests only —
// production always uses csafety.DefaultAuthCheckInterval/DefaultAuthFailThreshold).
func (r *Registry) WithAuthWatchdogTiming(interval time.Duration, threshold int) *Registry {
	r.authInterval = interval
	r.authThreshold = threshold
	return r
}

// WithTelemetryChecker enables TelemetryWatchdog on every VehicleContext (DRIFT-K3-TELEMETRY,
// Sprint 50). Call before the first Get() — without it, VehicleContext.TelemetryWatchdog stays
// nil, same nil-safety idiom as WithUserChecker/AuthWatchdog above.
func (r *Registry) WithTelemetryChecker(checker *telemetrycheck.Checker) *Registry {
	r.telemetryChecker = checker
	return r
}

// WithTelemetryWatchdogTiming overrides the default 2s×2 poll/threshold (tests only — production
// always uses telemetrycheck.DefaultTelemetryCheckInterval/DefaultTelemetryFailThreshold).
func (r *Registry) WithTelemetryWatchdogTiming(interval time.Duration, threshold int) *Registry {
	r.telemetryInterval = interval
	r.telemetryThreshold = threshold
	return r
}

// Get returns the VehicleContext for vehicleID, creating it on first access.
// Safe for concurrent use — exactly one VehicleContext is ever created per ID.
func (r *Registry) Get(vehicleID string) *VehicleContext {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ctx, ok := r.contexts[vehicleID]; ok {
		return ctx
	}
	sm := statemachine.New()
	ctx := &VehicleContext{
		SM:                 sm,
		Deadman:            csafety.NewDeadmanWatchdog(r.deadmanTimeout, sm, r.publisher).WithAuditWriter(r.auditWriter),
		ACKTimeoutWatcher:  csafety.NewACKTimeoutWatcher(r.ackTimeout, sm, r.publisher).WithAuditWriter(r.auditWriter),
		VehicleACKWatchdog: csafety.NewVehicleACKWatchdog(r.vehicleACKTimeout, sm, r.publisher).WithAuditWriter(r.auditWriter),
	}
	if r.userChecker != nil {
		ctx.AuthWatchdog = csafety.NewAuthWatchdog(r.authInterval, r.authThreshold, sm, r.publisher, r.userChecker).WithAuditWriter(r.auditWriter)
	}
	if r.telemetryChecker != nil {
		ctx.TelemetryWatchdog = telemetrycheck.NewTelemetryWatchdog(r.telemetryInterval, r.telemetryThreshold, sm, r.telemetryChecker)
	}
	r.contexts[vehicleID] = ctx
	return ctx
}

// StateMachine returns just the State Machine for vehicleID, creating the
// VehicleContext on first access if needed. Satisfies safety.VehicleStates
// (used by SafetyBusWatchdog, ADR-026) without that package importing this one.
func (r *Registry) StateMachine(vehicleID string) *statemachine.Machine {
	return r.Get(vehicleID).SM
}
