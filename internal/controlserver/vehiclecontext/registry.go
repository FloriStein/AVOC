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
	"avoc/pkg/audit"
)

// VehicleContext bundles the safety-critical state for exactly ONE vehicle.
type VehicleContext struct {
	SM                 *statemachine.Machine
	Deadman            *csafety.DeadmanWatchdog
	ACKTimeoutWatcher  *csafety.ACKTimeoutWatcher
	VehicleACKWatchdog *csafety.VehicleACKWatchdog
}

// Registry lazily creates and permanently retains one VehicleContext per
// vehicle ID. Per ADR-026: small, known fleet — no GC, no eviction.
type Registry struct {
	mu                sync.Mutex
	contexts          map[string]*VehicleContext
	deadmanTimeout    time.Duration
	ackTimeout        time.Duration
	vehicleACKTimeout time.Duration
	publisher         csafety.Publisher
	auditWriter       audit.SafetyAuditWriter
}

func NewRegistry(deadmanTimeout, ackTimeout, vehicleACKTimeout time.Duration, publisher csafety.Publisher) *Registry {
	return &Registry{
		contexts:          make(map[string]*VehicleContext),
		deadmanTimeout:    deadmanTimeout,
		ackTimeout:        ackTimeout,
		vehicleACKTimeout: vehicleACKTimeout,
		publisher:         publisher,
	}
}

// WithAuditWriter sets the audit writer applied to every VehicleContext's
// watchdogs (ADR-018). Call before the first Get().
func (r *Registry) WithAuditWriter(aw audit.SafetyAuditWriter) *Registry {
	r.auditWriter = aw
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
	r.contexts[vehicleID] = ctx
	return ctx
}

// StateMachine returns just the State Machine for vehicleID, creating the
// VehicleContext on first access if needed. Satisfies safety.VehicleStates
// (used by SafetyBusWatchdog, ADR-026) without that package importing this one.
func (r *Registry) StateMachine(vehicleID string) *statemachine.Machine {
	return r.Get(vehicleID).SM
}
