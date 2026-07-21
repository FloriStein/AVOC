package statemachine

// Multi-Cause-DEGRADED (ADR-009 Update 2026-07-20, DRIFT-K3-TELEMETRY, Sprint 47/50). White-box
// (package statemachine, not statemachine_test) — some tests below still call the private
// enterDegraded/exitDegraded helpers directly (pre-dating TransitionTelemetry, Sprint 47) to
// exercise the multi-cause set logic without a watchdog detour; newer tests use the real exported
// TransitionTelemetry (Sprint 50) now that it exists.

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

// TestTransitionTelemetry_SingleCause exercises TelemetryWatchdog's entry point in isolation
// (Sprint 50) — mirrors TestTransitionMedia_SingleCause_RegressionUnchanged's shape for Media.
func TestTransitionTelemetry_SingleCause(t *testing.T) {
	m := New()
	m.System = StateConnected
	m.Control = ControlActive

	m.TransitionTelemetry(false)
	if m.System != StateDegraded {
		t.Fatalf("telemetry loss from CONNECTED must enter DEGRADED, got %s", m.System)
	}
	if m.Control != ControlActive {
		t.Fatalf("control must remain ACTIVE during DEGRADED (ADR-011), got %s", m.Control)
	}

	m.TransitionTelemetry(true)
	if m.System != StateConnected {
		t.Fatalf("telemetry recovery from DEGRADED must return to CONNECTED, got %s", m.System)
	}
}

// TestTransitionTelemetry_And_TransitionMedia_AreIndependentCauses is the Sprint-50 counterpart
// to TestMultiCause_BothReasonsMustClearBeforeRecovery — same scenario, but both causes now go
// through their real exported entry points (TransitionMedia/TransitionTelemetry) instead of the
// private enterDegraded/exitDegraded helpers, now that TelemetryWatchdog is a real caller.
func TestTransitionTelemetry_And_TransitionMedia_AreIndependentCauses(t *testing.T) {
	m := New()
	m.System = StateConnected

	m.TransitionMedia(MediaFailed)
	m.TransitionTelemetry(false)
	if m.System != StateDegraded {
		t.Fatalf("expected DEGRADED with both causes active, got %s", m.System)
	}
	m.mu.RLock()
	n := len(m.degradedReasons)
	m.mu.RUnlock()
	if n != 2 {
		t.Fatalf("expected 2 active degraded reasons, got %d", n)
	}

	// Telemetry recovers — Media reason is still active, must stay DEGRADED.
	m.TransitionTelemetry(true)
	if m.System != StateDegraded {
		t.Fatalf("expected still DEGRADED while media remains failed, got %s", m.System)
	}

	// Media recovers too — set is now empty, must return to CONNECTED.
	m.TransitionMedia(MediaConnected)
	if m.System != StateConnected {
		t.Fatalf("expected CONNECTED once both causes cleared, got %s", m.System)
	}
}

// TestSecondCause_ArrivingAfterFirstCauseAlreadyDegraded is the regression test for the
// 2026-07-21 guard bugfix: a second, independent DEGRADED cause arriving while SYSTEM is already
// DEGRADED from a first cause must still be added to degradedReasons — previously the outer guard
// in TransitionMedia/TransitionTelemetry required System == StateConnected, so enterDegraded was
// never even called for the second cause, and the first cause's later recovery incorrectly
// cleared DEGRADED while the second cause was still active.
func TestSecondCause_ArrivingAfterFirstCauseAlreadyDegraded(t *testing.T) {
	m := New()
	m.System = StateConnected

	// Media fails first, System CONNECTED → DEGRADED.
	m.TransitionMedia(MediaFailed)
	if m.System != StateDegraded {
		t.Fatalf("expected DEGRADED after media failure, got %s", m.System)
	}

	// Telemetry fails *while already DEGRADED* — must still register as a second cause.
	m.TransitionTelemetry(false)
	m.mu.RLock()
	n := len(m.degradedReasons)
	_, hasTelemetry := m.degradedReasons[DegradedReasonTelemetry]
	m.mu.RUnlock()
	if n != 2 || !hasTelemetry {
		t.Fatalf("expected telemetry to be registered as a second active cause, got %d reasons (%v)", n, m.degradedReasons)
	}

	// Media recovers — Telemetry cause must keep SYSTEM in DEGRADED.
	m.TransitionMedia(MediaConnected)
	if m.System != StateDegraded {
		t.Fatalf("expected still DEGRADED — telemetry cause was silently dropped (pre-fix bug), got %s", m.System)
	}

	// Telemetry recovers too — now CONNECTED.
	m.TransitionTelemetry(true)
	if m.System != StateConnected {
		t.Fatalf("expected CONNECTED once both causes cleared, got %s", m.System)
	}
}

// TestTransitionTelemetry_NoOpDuringSafeMode confirms the SAFE_MODE guard documented in the
// ADR-009 2026-07-21 update: TelemetryWatchdog keeps polling/calling TransitionTelemetry during
// SAFE_MODE (Start/Stop is session-scoped, not SYSTEM-STATE-scoped), but the guard (enter only
// from CONNECTED, exit only from DEGRADED) makes those calls no-ops — Invariant 1 (telemetry never
// triggers or lifts SAFE_MODE) holds without any SAFE_MODE-specific code in TransitionTelemetry.
func TestTransitionTelemetry_NoOpDuringSafeMode(t *testing.T) {
	m := New()
	m.System = StateConnected
	m.TransitionSystem(StateSafeMode)

	m.TransitionTelemetry(false)
	if m.System != StateSafeMode {
		t.Fatalf("telemetry loss during SAFE_MODE must not change SYSTEM STATE, got %s", m.System)
	}

	m.TransitionTelemetry(true)
	if m.System != StateSafeMode {
		t.Fatalf("telemetry recovery during SAFE_MODE must not change SYSTEM STATE, got %s", m.System)
	}
}
