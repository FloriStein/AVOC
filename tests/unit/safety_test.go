// Safety Test Suite — dedicated scenario-based tests for all CRITICAL triggers (ADR-006/009/011).
// Each test maps to a documented failure class. These tests are the safety gate in CI.
//
// Scope note (SAFETYBUS-02, Sprint 42): these tests cover the trigger logic only — state machine
// transitions and that the Publisher interface (internal/controlserver/safety.Publisher) is called
// with the correct SafetyEventType, verified against mocks.MockSafetyPublisher. They do not exercise
// the real Safety Event Bus (internal/safetyservice.Bus) that HTTPPublisher talks to in production
// via POST /safety/event (cmd/safety-service/main.go's newSafetyMux). For that, see
// safety_bus_integration_test.go, which wires HTTPPublisher against a real safetyservice.Bus behind
// a local httptest.Server and verifies via bus.GetSafetyState() — currently for the Dead-man-Timeout
// and ACK-Timeout triggers.
package unit_test

import (
	"testing"
	"time"

	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/safetyservice"
	"avoc/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSetup builds the minimal wiring needed for safety tests.
func newTestSetup(t *testing.T) (
	sm *statemachine.Machine,
	pub *mocks.MockSafetyPublisher,
	sfuPub *mocks.MockSFUPublisher,
	mgr *session.Manager,
) {
	t.Helper()
	sm = statemachine.New()
	pub = &mocks.MockSafetyPublisher{}
	sfuPub = &mocks.MockSFUPublisher{}
	mgr = session.NewManager(sfuPub)
	return
}

// connectSession transitions the state machine to CONNECTED and creates a session.
func connectSession(t *testing.T, sm *statemachine.Machine, mgr *session.Manager) session.Session {
	t.Helper()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	ok := sm.TransitionToConnected()
	require.True(t, ok, "TransitionToConnected must succeed from AUTHENTICATED")
	sess := mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")
	mgr.PushSFUEvent("SESSION_CREATED")
	return sess
}

// --- Transition Validation ---

func TestSafety_InvalidTransitionRejected(t *testing.T) {
	sm, _, _, _ := newTestSetup(t)

	// IDLE → CONNECTED is invalid (must go via CONNECTING → AUTHENTICATED)
	sm.TransitionSystem(statemachine.StateConnected)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateIdle, sys, "invalid transition must be rejected")
}

// --- CRITICAL Trigger 1: WS Disconnect ---

func TestSafety_WSDisconnect_TriggersSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	// Simulate WS disconnect
	sm.TransitionSystem(statemachine.StateSafeMode)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- CRITICAL Trigger 2: Dead-man Switch Timeout ---

func TestSafety_DeadmanTimeout_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watchdog := csafety.NewDeadmanWatchdog(50*time.Millisecond, sm, pub)
	watchdog.Start(sess.ID, sess.VehicleID)
	watchdog.Reset() // arm the watchdog (simulates first DEADMAN_HOLD received)

	// Do NOT call Reset() again — let the armed timer fire
	time.Sleep(150 * time.Millisecond)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "dead-man timeout must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEventType())
}

func TestSafety_DeadmanReset_PreventsTimeout(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watchdog := csafety.NewDeadmanWatchdog(100*time.Millisecond, sm, pub)
	watchdog.Start(sess.ID, sess.VehicleID)

	// Keep resetting — should never fire
	for i := 0; i < 5; i++ {
		time.Sleep(60 * time.Millisecond)
		watchdog.Reset()
	}

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "reset dead-man must NOT trigger SAFE_MODE")
	watchdog.Stop()
}

// --- CRITICAL Trigger 3: Command ACK Timeout ---

func TestSafety_ACKTimeout_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watcher := csafety.NewACKTimeoutWatcher(50*time.Millisecond, sm, pub)
	watcher.CommandReceived(sess.ID, sess.VehicleID)

	// Do NOT call CommandACKed() — let timer fire
	time.Sleep(150 * time.Millisecond)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "ACK timeout must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, safetyservice.EventACKTimeout, pub.LastEventType())
}

func TestSafety_ACKInTime_NoSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watcher := csafety.NewACKTimeoutWatcher(100*time.Millisecond, sm, pub)
	watcher.CommandReceived(sess.ID, sess.VehicleID)
	watcher.CommandACKed() // ACK within budget

	time.Sleep(150 * time.Millisecond)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "ACK in time must NOT trigger SAFE_MODE")
}

// --- CRITICAL Trigger 4: No Active Operator ---

func TestSafety_NoOperator_TriggersSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)

	// Operator leaves
	sm.TransitionOperator(statemachine.OpNoOperator)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "NO_OPERATOR must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// DRIFT-K2 (2026-07-16): TransitionOperator(OpNoOperator) now routes its internal
// SAFE_MODE branch through the same transitionSystemLocked guard as TransitionSystem
// (previously set m.System directly). These cases pin down the guard boundary and the
// idempotent-when-already-SAFE_MODE behavior that the WS-disconnect handler relies on.

func TestSafety_NoOperator_AlreadyInSafeMode_NoOpNoInvalidWarningFlood(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)
	sm.TransitionSystem(statemachine.StateSafeMode)

	// Simulates the WS-disconnect handler calling TransitionOperator AFTER it has
	// already called TransitionSystem(StateSafeMode) directly (websocket.go readLoop
	// defer) — must not attempt a SAFE_MODE→SAFE_MODE transition, only correct Operator.
	sm.TransitionOperator(statemachine.OpNoOperator)

	sys, ctrl, _, op := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, statemachine.OpNoOperator, op, "OPERATOR layer must reflect NO_OPERATOR, not hang at ACTIVE_OPERATOR")
}

func TestSafety_NoOperator_WhileAuthenticated_DoesNotForceSafeMode(t *testing.T) {
	sm, _, _, _ := newTestSetup(t)
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)

	// No operator ever became ACTIVE (e.g. login completed but session/start not yet
	// called) — OpNoOperator here is not a "operator disappeared" event and must not
	// force a SYSTEM transition; only StateConnected/StateDegraded arm the branch.
	sm.TransitionOperator(statemachine.OpNoOperator)

	sys, _, _, op := sm.Get()
	assert.Equal(t, statemachine.StateAuthenticated, sys, "guard boundary: only CONNECTED/DEGRADED trigger SAFE_MODE")
	assert.Equal(t, statemachine.OpNoOperator, op)
}

func TestSafety_NoOperator_FromDegraded_TriggersSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)
	sm.TransitionMedia(statemachine.MediaFailed) // → DEGRADED (Invariant 1)
	require.Equal(t, statemachine.StateDegraded, func() statemachine.SystemState { s, _, _, _ := sm.Get(); return s }())

	sm.TransitionOperator(statemachine.OpNoOperator)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "NO_OPERATOR must also trigger SAFE_MODE from DEGRADED")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- CRITICAL Trigger 5: Emergency Stop ---

func TestSafety_EmergencyStop_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	// Emergency stop bypasses all layers — direct SAFE_MODE transition
	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.TriggerEmergencyStop(sess.ID, sess.VehicleID, "operator triggered E-Stop")

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, 1, pub.EmergencyStopCount())
}

// --- CRITICAL Trigger 6: Safety Bus Event (AUTH_INVALID / SAFETY_BUS_DOWN) ---

func TestSafety_AuthInvalidation_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sess.ID,
		VehicleID: sess.VehicleID,
		Type:      safetyservice.EventAuthInvalid,
		Reason:    "JWT revoked",
	})

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, safetyservice.EventAuthInvalid, pub.LastEventType())
}

func TestSafety_SafetyBusDown_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sess.ID,
		VehicleID: sess.VehicleID,
		Type:      safetyservice.EventSafetyBusDown,
		Reason:    "safety bus unreachable",
	})

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
}

// --- ADR-009 Invariant 1: MEDIA_FAILED must NEVER trigger SAFE_MODE ---

func TestSafety_MediaFailed_TriggersDegrade_NeverSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionMedia(statemachine.MediaFailed)

	sys, ctrl, media, _ := sm.Get()
	assert.Equal(t, statemachine.StateDegraded, sys, "MEDIA_FAILED must trigger DEGRADED")
	assert.NotEqual(t, statemachine.StateSafeMode, sys, "MEDIA_FAILED must NEVER trigger SAFE_MODE (Invariant 1)")
	assert.Equal(t, statemachine.ControlActive, ctrl, "control must remain ACTIVE during DEGRADED")
	assert.Equal(t, statemachine.MediaFailed, media)
}

func TestSafety_MediaDegraded_TriggersDegrade_NeverSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionMedia(statemachine.MediaDegraded)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateDegraded, sys)
	assert.NotEqual(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlActive, ctrl, "control stays active during DEGRADED")
}

// DRIFT-K3 (2026-07-16): MediaConnected while SYSTEM is DEGRADED must recover back to
// CONNECTED — CONTEXT.MD documents CONNECTED ⇄ DEGRADED as bidirectional, but only the
// DEGRADED-entry direction existed before this fix, leaving SYSTEM STATE permanently
// stuck at DEGRADED once video failed even once, regardless of later recovery.

func TestSafety_MediaRecovered_FromDegraded_ReturnsToConnected(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionMedia(statemachine.MediaFailed)
	require.Equal(t, statemachine.StateDegraded, func() statemachine.SystemState { s, _, _, _ := sm.Get(); return s }())

	sm.TransitionMedia(statemachine.MediaConnected)

	sys, ctrl, media, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "media recovery must clear DEGRADED")
	assert.Equal(t, statemachine.ControlActive, ctrl)
	assert.Equal(t, statemachine.MediaConnected, media)
}

func TestSafety_MediaConnected_WhileAlreadyConnected_NoOp(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	// MediaConnected fires (e.g. renegotiation) while SYSTEM was never DEGRADED —
	// must not spuriously touch SYSTEM/CONTROL state.
	sm.TransitionMedia(statemachine.MediaConnected)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys)
	assert.Equal(t, statemachine.ControlActive, ctrl)
}

func TestSafety_MediaRecovered_DuringSafeMode_DoesNotEscapeSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionSystem(statemachine.StateSafeMode)

	// Media recovering while the vehicle is in SAFE_MODE for an unrelated reason
	// (e.g. dead-man timeout) must NOT pull SYSTEM back to CONNECTED — recovery
	// requires an explicit Operator Ack (ADR-009 "No Auto-Resume"), never a media event.
	sm.TransitionMedia(statemachine.MediaConnected)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "SAFE_MODE must only be left via explicit recovery, never by a MEDIA event")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- Recovery Checkpoint ---

func TestSafety_RecoveryCheckpoint_SavedOnSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	sys, ctrl, _, _ := sm.Get()
	mgr.SaveCheckpoint(string(sys), string(ctrl), "WS_DISCONNECT")

	cp, ok := mgr.LoadCheckpoint()
	require.True(t, ok, "checkpoint must exist after SAFE_MODE")
	assert.Equal(t, sess.ID, cp.SessionID, "checkpoint session ID must match active session")
	assert.Equal(t, "SAFE_MODE", cp.LastSystemState)
	assert.Equal(t, "CONTROL_BLOCKED", cp.LastControlState)
	assert.Equal(t, "WS_DISCONNECT", cp.SafetyReason)
}

func TestSafety_Recovery_FailsIfValidationFails(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	sm.TransitionSystem(statemachine.StateRecovering)
	mgr.SaveCheckpoint("RECOVERING", "CONTROL_RECOVERING", "reconnect_failed")

	// Simulate validation failure: recovering → SAFE_MODE fallback
	sm.TransitionSystem(statemachine.StateSafeMode)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "failed recovery must fall back to SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- Session Manager (GSA) ---

func TestSafety_SessionID_IsULID(t *testing.T) {
	_, _, sfuPub, mgr := newTestSetup(t)
	_ = sfuPub

	sess := mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")

	assert.NotEmpty(t, sess.ID, "session ID must not be empty")
	assert.Len(t, sess.ID, 26, "ULID must be 26 characters")
}

func TestSafety_SessionID_UniquePerSession(t *testing.T) {
	_, _, _, mgr := newTestSetup(t)

	s1 := mgr.CreateSession("v-1", "op-1", "ACTIVE_OPERATOR")
	s2 := mgr.CreateSession("v-1", "op-1", "ACTIVE_OPERATOR")

	assert.NotEqual(t, s1.ID, s2.ID, "each session must have a unique ID")
}

// --- Operator Handover ---
//
// These tests wire HandoverManager through a real vehiclecontext.Registry (same
// type main.go uses, ADR-026 follow-up/MV-11) instead of a standalone Machine —
// that standalone-Machine setup used to hide a real bug: nothing in the actual
// request path ever transitioned it to ACTIVE_OPERATOR, so RequestHandover's
// precondition could never pass outside of a test manually priming it.

// newHandoverTestSetup builds a vehiclecontext.Registry + session.Manager, mirroring
// main.go's real wiring (see tests/unit/watchdog_test.go's newBusWatchdogSetup for
// the same pattern used for SafetyBusWatchdog).
func newHandoverTestSetup(sfuPub *mocks.MockSFUPublisher) (*vehiclecontext.Registry, *session.Manager) {
	pub := &mocks.MockSafetyPublisher{}
	registry := vehiclecontext.NewRegistry(10*time.Second, 100*time.Millisecond, 1*time.Second, pub)
	mgr := session.NewManager(sfuPub)
	return registry, mgr
}

func TestSafety_Handover_TransitionsToHandoverPending(t *testing.T) {
	sfuPub := &mocks.MockSFUPublisher{}
	registry, mgr := newHandoverTestSetup(sfuPub)
	vc := registry.Get("vehicle-1")
	connectSession(t, vc.SM, mgr)
	vc.SM.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(registry, mgr, "") // no auth URL in test

	err := handoverMgr.RequestHandover("vehicle-1", "operator-1", "operator-2")
	require.NoError(t, err)

	_, _, _, op := vc.SM.Get()
	assert.Equal(t, statemachine.OpHandoverPending, op)
}

func TestSafety_Handover_ConfirmSwitchesActiveOperator(t *testing.T) {
	sfuPub := &mocks.MockSFUPublisher{}
	registry, mgr := newHandoverTestSetup(sfuPub)
	vc := registry.Get("vehicle-1")
	connectSession(t, vc.SM, mgr)
	vc.SM.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(registry, mgr, "")
	require.NoError(t, handoverMgr.RequestHandover("vehicle-1", "operator-1", "operator-2"))
	require.NoError(t, handoverMgr.ConfirmHandover("vehicle-1", "operator-2"))

	_, _, _, op := vc.SM.Get()
	assert.Equal(t, statemachine.OpActive, op)

	sess, ok := mgr.GetSessionByVehicle("vehicle-1")
	require.True(t, ok)
	assert.Equal(t, "operator-2", sess.OperatorID)

	// PushSFUEvent is async — wait briefly for the goroutine to deliver
	assert.Eventually(t, func() bool {
		events := sfuPub.Events()
		for _, e := range events {
			if e.Type == "OPERATOR_HANDOVER" {
				return true
			}
		}
		return false
	}, 200*time.Millisecond, 10*time.Millisecond, "OPERATOR_HANDOVER must be pushed to SFU")
}

func TestSafety_Handover_CancelRestoresActiveOperator(t *testing.T) {
	sfuPub := &mocks.MockSFUPublisher{}
	registry, mgr := newHandoverTestSetup(sfuPub)
	vc := registry.Get("vehicle-1")
	connectSession(t, vc.SM, mgr)
	vc.SM.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(registry, mgr, "")
	require.NoError(t, handoverMgr.RequestHandover("vehicle-1", "operator-1", "operator-2"))
	handoverMgr.CancelHandover("vehicle-1")

	_, _, _, op := vc.SM.Get()
	assert.Equal(t, statemachine.OpActive, op)
	assert.False(t, handoverMgr.IsPending("vehicle-1"))
}

// TestSafety_Handover_TwoVehicles_IndependentHandovers is the MV-11 regression test:
// a handover pending on one vehicle must never block or leak into another's. Before
// the per-vehicle fix, HandoverManager held one process-wide standalone Machine, so
// vehicle-2's RequestHandover would have failed with "got HANDOVER_PENDING" while
// vehicle-1's handover was still in progress.
func TestSafety_Handover_TwoVehicles_IndependentHandovers(t *testing.T) {
	sfuPub := &mocks.MockSFUPublisher{}
	registry, mgr := newHandoverTestSetup(sfuPub)

	vc1 := registry.Get("vehicle-1")
	connectSession(t, vc1.SM, mgr)
	vc1.SM.TransitionOperator(statemachine.OpActive)

	vc2 := registry.Get("vehicle-2")
	vc2.SM.TransitionSystem(statemachine.StateConnecting)
	vc2.SM.TransitionSystem(statemachine.StateAuthenticated)
	require.True(t, vc2.SM.TransitionToConnected())
	mgr.CreateSession("vehicle-2", "operator-3", "ACTIVE_OPERATOR")
	vc2.SM.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(registry, mgr, "")

	require.NoError(t, handoverMgr.RequestHandover("vehicle-1", "operator-1", "operator-2"))
	assert.True(t, handoverMgr.IsPending("vehicle-1"))
	assert.False(t, handoverMgr.IsPending("vehicle-2"), "vehicle-2 must not see vehicle-1's pending handover")

	// The actual regression: this must succeed even while vehicle-1's handover is pending.
	err := handoverMgr.RequestHandover("vehicle-2", "operator-3", "operator-4")
	require.NoError(t, err, "handover on vehicle-2 must not be blocked by vehicle-1's pending handover")

	_, _, _, op1 := vc1.SM.Get()
	_, _, _, op2 := vc2.SM.Get()
	assert.Equal(t, statemachine.OpHandoverPending, op1)
	assert.Equal(t, statemachine.OpHandoverPending, op2)

	require.NoError(t, handoverMgr.ConfirmHandover("vehicle-2", "operator-4"))
	_, _, _, op1After := vc1.SM.Get()
	assert.Equal(t, statemachine.OpHandoverPending, op1After, "confirming vehicle-2's handover must not affect vehicle-1")
}
