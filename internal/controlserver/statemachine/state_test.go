package statemachine

// Multi-Cause-DEGRADED (ADR-009 Update 2026-07-20, DRIFT-K3-TELEMETRY, Sprint 47). White-box
// (package statemachine, not statemachine_test) because DegradedReasonTelemetry has no real
// producer yet — Sprint 48's TelemetryWatchdog is the eventual caller of enterDegraded/
// exitDegraded(DegradedReasonTelemetry); until then these tests call the private helpers
// directly to exercise the multi-cause set logic without a watchdog detour.

import "testing"

// TestTransitionMedia_SingleCause_RegressionUnchanged pins the pre-existing, single-cause
// (Media-only) behavior of TransitionMedia bit-identical to before the enterDegraded/
// exitDegraded refactor (SM-03) — pure internal rerouting must not change observable behavior.
func TestTransitionMedia_SingleCause_RegressionUnchanged(t *testing.T) {
	m := New()
	m.System = StateConnected
	m.Control = ControlActive

	m.TransitionMedia(MediaFailed)
	if m.System != StateDegraded {
		t.Fatalf("MediaFailed from CONNECTED must enter DEGRADED, got %s", m.System)
	}
	if m.Control != ControlActive {
		t.Fatalf("control must remain ACTIVE during DEGRADED (ADR-011), got %s", m.Control)
	}

	m.TransitionMedia(MediaConnected)
	if m.System != StateConnected {
		t.Fatalf("MediaConnected from DEGRADED must recover to CONNECTED, got %s", m.System)
	}
}

// TestMultiCause_BothReasonsMustClearBeforeRecovery is the core multi-cause scenario: two
// independent DEGRADED causes active at once, recovery of only one must not leave SYSTEM
// STATE at CONNECTED while the other cause is still unresolved.
func TestMultiCause_BothReasonsMustClearBeforeRecovery(t *testing.T) {
	m := New()
	m.System = StateConnected

	m.TransitionMedia(MediaFailed)
	if m.System != StateDegraded {
		t.Fatalf("expected DEGRADED after media failure, got %s", m.System)
	}

	m.mu.Lock()
	m.enterDegraded(DegradedReasonTelemetry)
	m.mu.Unlock()
	if len(m.degradedReasons) != 2 {
		t.Fatalf("expected 2 active degraded reasons, got %d (%v)", len(m.degradedReasons), m.degradedReasons)
	}

	// Media recovers — Telemetry reason is still active, must stay DEGRADED.
	m.TransitionMedia(MediaConnected)
	if m.System != StateDegraded {
		t.Fatalf("expected still DEGRADED while a second reason remains active, got %s", m.System)
	}

	// Telemetry recovers too — set is now empty, must return to CONNECTED.
	m.mu.Lock()
	m.exitDegraded(DegradedReasonTelemetry)
	m.mu.Unlock()
	if m.System != StateConnected {
		t.Fatalf("expected CONNECTED once all degraded reasons cleared, got %s", m.System)
	}
	if len(m.degradedReasons) != 0 {
		t.Fatalf("expected empty degradedReasons set, got %v", m.degradedReasons)
	}
}

// TestSafeModeEntry_ClearsDegradedReasons verifies SAFE_MODE is a full stop regardless of
// DEGRADED cause: entering SAFE_MODE clears the reason set, and after recovery back to
// CONNECTED a stale/delayed exitDegraded call (e.g. a late watchdog tick) must be a no-op —
// no auto-recovery into DEGRADED for a reason that was never re-added post-recovery.
func TestSafeModeEntry_ClearsDegradedReasons(t *testing.T) {
	m := New()
	m.System = StateConnected

	m.TransitionMedia(MediaFailed)
	m.mu.Lock()
	m.enterDegraded(DegradedReasonTelemetry)
	m.mu.Unlock()
	if len(m.degradedReasons) != 2 {
		t.Fatalf("setup: expected 2 active degraded reasons, got %d", len(m.degradedReasons))
	}

	m.TransitionSystem(StateSafeMode)
	if m.System != StateSafeMode {
		t.Fatalf("expected SAFE_MODE, got %s", m.System)
	}
	if m.Control != ControlBlocked {
		t.Fatalf("expected CONTROL_BLOCKED under SAFE_MODE, got %s", m.Control)
	}
	if len(m.degradedReasons) != 0 {
		t.Fatalf("SAFE_MODE entry must clear all degraded reasons, got %v", m.degradedReasons)
	}

	// Recovery sequence (ADR-009): SAFE_MODE → IDLE → CONNECTING → AUTHENTICATED → CONNECTED.
	m.TransitionSystem(StateIdle)
	m.TransitionSystem(StateConnecting)
	m.TransitionSystem(StateAuthenticated)
	if ok := m.TransitionToConnected(); !ok {
		t.Fatalf("expected AUTHENTICATED→CONNECTED to succeed")
	}
	if m.System != StateConnected {
		t.Fatalf("expected CONNECTED after recovery, got %s", m.System)
	}

	m.mu.Lock()
	m.exitDegraded(DegradedReasonMedia)
	m.mu.Unlock()
	if m.System != StateConnected {
		t.Fatalf("stale exitDegraded after recovery must not affect CONNECTED, got %s", m.System)
	}
	if len(m.degradedReasons) != 0 {
		t.Fatalf("expected degradedReasons to remain empty, got %v", m.degradedReasons)
	}
}
