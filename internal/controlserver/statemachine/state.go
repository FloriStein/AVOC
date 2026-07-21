// Package statemachine implements the 4-Layer State Machine (ADR-011).
// SYSTEM STATE is the Safety Truth — all other states depend on it.
// MEDIA STATE can never trigger SAFE_MODE (ADR-009 Invariant 1).
package statemachine

import (
	"sync"

	"avoc/pkg/logger"
)

var svcLog = logger.New("control-server")

// SystemState represents the master safety state (ADR-011).
type SystemState string

const (
	StateIdle          SystemState = "IDLE"
	StateConnecting    SystemState = "CONNECTING"
	StateAuthenticated SystemState = "AUTHENTICATED"
	StateConnected     SystemState = "CONNECTED"
	StateDegraded      SystemState = "DEGRADED"
	StateSafeMode      SystemState = "SAFE_MODE"
	StateRecovering    SystemState = "RECOVERING"
)

// ControlState represents the command flow state (ADR-011).
type ControlState string

const (
	ControlInit       ControlState = "CONTROL_INIT"
	ControlActive     ControlState = "CONTROL_ACTIVE"
	ControlBlocked    ControlState = "CONTROL_BLOCKED" // enforced during SAFE_MODE
	ControlLost       ControlState = "CONTROL_LOST"
	ControlRecovering ControlState = "CONTROL_RECOVERING"
)

// MediaState represents WebRTC video health (ADR-011).
// MEDIA_FAILED maps to SYSTEM DEGRADED — never SAFE_MODE (ADR-009 Invariant 1).
type MediaState string

const (
	MediaInit        MediaState = "MEDIA_INIT"
	MediaNegotiating MediaState = "MEDIA_NEGOTIATING"
	MediaConnected   MediaState = "MEDIA_CONNECTED"
	MediaDegraded    MediaState = "MEDIA_DEGRADED"
	MediaFailed      MediaState = "MEDIA_FAILED"
)

// OperatorState represents human governance (ADR-011).
type OperatorState string

const (
	OpNoOperator      OperatorState = "NO_OPERATOR"
	OpAssigned        OperatorState = "OPERATOR_ASSIGNED"
	OpActive          OperatorState = "ACTIVE_OPERATOR"
	OpHandoverPending OperatorState = "HANDOVER_PENDING"
	OpRecovering      OperatorState = "RECOVERING_OPERATOR"
)

// validSystemTransitions defines allowed state transitions.
// SAFE_MODE is reachable from any non-idle state (CRITICAL can occur at any time).
var validSystemTransitions = map[SystemState][]SystemState{
	StateIdle:          {StateConnecting},
	StateConnecting:    {StateAuthenticated, StateSafeMode},
	StateAuthenticated: {StateConnected, StateSafeMode},
	StateConnected:     {StateDegraded, StateSafeMode, StateIdle},
	StateDegraded:      {StateConnected, StateSafeMode, StateIdle},
	StateSafeMode:      {StateRecovering, StateIdle},
	StateRecovering:    {StateAuthenticated, StateSafeMode},
}

// DegradedReason identifies an independent cause of SYSTEM DEGRADED (ADR-009 Update
// 2026-07-20, DRIFT-K3-TELEMETRY). Multiple reasons can be active at once — SYSTEM only
// returns to CONNECTED once all of them have cleared. DegradedReasonTelemetry is defined
// now but only produced starting Sprint 48 (TelemetryWatchdog).
type DegradedReason string

const (
	DegradedReasonMedia     DegradedReason = "media"
	DegradedReasonTelemetry DegradedReason = "telemetry"
)

// Machine holds all 4 orthogonal state machines.
type Machine struct {
	mu       sync.RWMutex
	System   SystemState
	Control  ControlState
	Media    MediaState
	Operator OperatorState

	// degradedReasons is the set of currently active DEGRADED causes (ADR-009 Update
	// 2026-07-20). Guarded by mu, same as the 4 state fields above.
	degradedReasons map[DegradedReason]bool
}

func New() *Machine {
	return &Machine{
		System:          StateIdle,
		Control:         ControlInit,
		Media:           MediaInit,
		Operator:        OpNoOperator,
		degradedReasons: make(map[DegradedReason]bool),
	}
}

func (m *Machine) Get() (SystemState, ControlState, MediaState, OperatorState) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.System, m.Control, m.Media, m.Operator
}

// CanTransitionTo returns true if transitioning from current to next system state is valid.
func (m *Machine) CanTransitionTo(next SystemState) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return isValidTransition(m.System, next)
}

func isValidTransition(current, next SystemState) bool {
	allowed, ok := validSystemTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// TransitionSystem sets the SYSTEM STATE and enforces dependent CONTROL STATE rules.
// Invalid transitions are logged and rejected — system stays in current state.
// SAFE_MODE → CONTROL_BLOCKED (ADR-011).
func (m *Machine) TransitionSystem(next SystemState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitionSystemLocked(next)
}

// transitionSystemLocked performs the validated SYSTEM STATE transition and its
// dependent CONTROL STATE update. Caller must hold m.mu. Shared by TransitionSystem
// and TransitionOperator's NO_OPERATOR→SAFE_MODE branch (2026-07-16, DRIFT-K2) so
// every path into SAFE_MODE goes through the same validSystemTransitions guard —
// previously TransitionOperator set m.System directly, bypassing isValidTransition.
func (m *Machine) transitionSystemLocked(next SystemState) bool {
	if !isValidTransition(m.System, next) {
		svcLog.Warn("invalid state transition rejected", "from", m.System, "to", next)
		return false
	}

	svcLog.Event(logger.EventStateTransition, "system state transition",
		"from", m.System, "to", next)
	m.System = next

	switch next {
	case StateIdle:
		m.Control = ControlInit
		m.Operator = OpNoOperator
	case StateSafeMode:
		m.Control = ControlBlocked
		// SAFE_MODE is a full stop regardless of cause (ADR-009 Update 2026-07-20) — clear
		// all active DEGRADED reasons. Each watchdog/media poll re-checks its own reason
		// independently after recovery and re-enters DEGRADED if it still applies.
		clear(m.degradedReasons)
	case StateConnected:
		m.Control = ControlActive
	case StateAuthenticated:
		if m.Control == ControlBlocked || m.Control == ControlRecovering {
			m.Control = ControlInit
		}
	case StateRecovering:
		m.Control = ControlRecovering
	case StateDegraded:
		// Control remains active during DEGRADED — video loss never blocks control (ADR-011)
	}
	return true
}

// enterDegraded adds reason to the active DEGRADED-reason set and transitions SYSTEM to
// DEGRADED if it is currently CONNECTED (guard unchanged from the pre-multi-cause behavior,
// ADR-009 Update 2026-07-20). Caller must already hold m.mu, analog transitionSystemLocked.
func (m *Machine) enterDegraded(reason DegradedReason) {
	m.degradedReasons[reason] = true
	if m.System == StateConnected {
		m.transitionSystemLocked(StateDegraded)
	}
}

// exitDegraded removes reason from the active DEGRADED-reason set and transitions SYSTEM back
// to CONNECTED only once the set is empty and SYSTEM is still DEGRADED — a still-active reason
// (e.g. Telemetry, Sprint 48) must keep SYSTEM in DEGRADED even though reason itself recovered
// (ADR-009 Update 2026-07-20). Caller must already hold m.mu, analog transitionSystemLocked.
func (m *Machine) exitDegraded(reason DegradedReason) {
	delete(m.degradedReasons, reason)
	if len(m.degradedReasons) == 0 && m.System == StateDegraded {
		m.transitionSystemLocked(StateConnected)
	}
}

// TransitionToConnected atomically moves AUTHENTICATED → CONNECTED and activates control.
// Returns false if the precondition (must be in AUTHENTICATED) is not met.
func (m *Machine) TransitionToConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.System != StateAuthenticated {
		svcLog.Warn("TransitionToConnected rejected", "current_state", m.System)
		return false
	}
	svcLog.Event(logger.EventStateTransition, "system state transition",
		"from", m.System, "to", StateConnected)
	m.System = StateConnected
	m.Control = ControlActive
	return true
}

// TransitionMedia updates media state and maps MEDIA_FAILED/DEGRADED → SYSTEM DEGRADED.
// MEDIA events NEVER trigger SAFE_MODE (ADR-009 Invariant 1).
//
// Recovery (2026-07-16, DRIFT-K3): MediaConnected while System is already DEGRADED
// transitions back to CONNECTED — closes a gap where video recovering after a drop
// left SYSTEM STATE stuck at DEGRADED forever (CONTEXT.MD documents CONNECTED ⇄
// DEGRADED as bidirectional; only the DEGRADED-entry direction was implemented).
//
// Bugfix (2026-07-21, Sprint 50): the entry guard also accepts System == StateDegraded now, not
// just StateConnected. Found while wiring TelemetryWatchdog as the second real DEGRADED cause:
// Media-fails-first-then-Telemetry-fails (or vice versa) silently dropped the second reason,
// because enterDegraded was never even called once System had already left CONNECTED — so a lone
// recovery of the FIRST cause emptied degradedReasons and returned to CONNECTED while the second
// cause was still active. enterDegraded itself already handles "already DEGRADED" correctly (adds
// to the set, skips the redundant transitionSystemLocked); only this outer guard was too narrow.
func (m *Machine) TransitionMedia(next MediaState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Media = next
	switch {
	case (next == MediaFailed || next == MediaDegraded) && (m.System == StateConnected || m.System == StateDegraded):
		svcLog.Event(logger.EventMediaStateChange,
			"media failure → SYSTEM DEGRADED (Invariant 1: never SAFE_MODE)",
			"media_state", next)
		m.enterDegraded(DegradedReasonMedia)
	case next == MediaConnected && m.System == StateDegraded:
		svcLog.Event(logger.EventMediaStateChange,
			"media recovered → SYSTEM DEGRADED→CONNECTED",
			"media_state", next)
		m.exitDegraded(DegradedReasonMedia)
	}
}

// TransitionTelemetry updates the TELEMETRY DEGRADED-reason independent of Media (ADR-009 Update
// 2026-07-21, DRIFT-K3-TELEMETRY Teil 2, TelemetryWatchdog). Unlike TransitionMedia there is no
// separate sub-state to track — the watchdog only reports healthy/unhealthy — so this maps
// directly onto enterDegraded/exitDegraded(DegradedReasonTelemetry) with the same guards
// (enter from CONNECTED or already-DEGRADED, exit only from DEGRADED) as TransitionMedia,
// preserving Invariant 1 (telemetry never triggers or lifts SAFE_MODE). The entry guard accepts
// StateDegraded too, same reasoning as TransitionMedia's 2026-07-21 bugfix above — otherwise a
// Telemetry failure arriving while Media has already caused DEGRADED would never be added to
// degradedReasons, and Media's later recovery would incorrectly clear DEGRADED.
func (m *Machine) TransitionTelemetry(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case !healthy && (m.System == StateConnected || m.System == StateDegraded):
		svcLog.Event(logger.EventTelemetryStateChange,
			"telemetry loss → SYSTEM DEGRADED (Invariant 1: never SAFE_MODE)")
		m.enterDegraded(DegradedReasonTelemetry)
	case healthy && m.System == StateDegraded:
		svcLog.Event(logger.EventTelemetryStateChange,
			"telemetry recovered → SYSTEM DEGRADED→CONNECTED")
		m.exitDegraded(DegradedReasonTelemetry)
	}
}

// TransitionOperator updates operator state and enforces NO_OPERATOR → SAFE_MODE (ADR-011).
// The SAFE_MODE branch routes through transitionSystemLocked (2026-07-16, DRIFT-K2) —
// previously set m.System directly, bypassing the validSystemTransitions guard that
// every other CRITICAL trigger (Deadman, ACKTimeout, SafetyBusWatchdog, ...) goes
// through via TransitionSystem.
func (m *Machine) TransitionOperator(next OperatorState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Operator = next
	if next == OpNoOperator && (m.System == StateConnected || m.System == StateDegraded) {
		svcLog.Event(logger.EventSafeModeEntered,
			"NO_OPERATOR → SAFE_MODE", "trigger", "no_active_operator")
		m.transitionSystemLocked(StateSafeMode)
	}
}
