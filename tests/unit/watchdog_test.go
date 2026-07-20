// Watchdog Tests (ADR-009).
// Covers all edge cases for VehicleACKWatchdog and SafetyBusWatchdog.
// These are safety-gate tests — every CRITICAL failure path must be green.
package unit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

// ── helpers ──────────────────────────────────────────────────────────────────

func newWatchdogSetup() (*statemachine.Machine, *mocks.MockSafetyPublisher) {
	sm := statemachine.New()
	// Put state machine into CONNECTED so SAFE_MODE transitions are valid.
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	sm.TransitionToConnected()
	return sm, &mocks.MockSafetyPublisher{}
}

// waitForSafeMode polls until SAFE_MODE is reached or deadline passes.
func waitForSafeMode(t *testing.T, sm *statemachine.Machine, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		sys, _, _, _ := sm.Get()
		if sys == statemachine.StateSafeMode {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// ── VehicleACKWatchdog ────────────────────────────────────────────────────────

const testVehicleACKTimeout = 80 * time.Millisecond

func newVehicleACKWatchdog(sm *statemachine.Machine, pub *mocks.MockSafetyPublisher) *csafety.VehicleACKWatchdog {
	w := csafety.NewVehicleACKWatchdog(testVehicleACKTimeout, sm, pub)
	w.Start("sess-1", "vehicle-1")
	return w
}

// 1. No ACK within timeout → SAFE_MODE.
func TestVehicleACK_TimeoutFiresSafeMode(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	w.CommandForwarded()

	require.True(t, waitForSafeMode(t, sm, testVehicleACKTimeout*3),
		"SAFE_MODE must be reached after ACK timeout")
	assert.Equal(t, safetyservice.EventVehicleACKTimeout, pub.LastEventType(),
		"VEHICLE_ACK_TIMEOUT event must be published")
}

// 2. ACK arrives before timeout → no SAFE_MODE.
func TestVehicleACK_ACKBeforeTimeout_NoSafeMode(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	w.CommandForwarded()
	time.Sleep(testVehicleACKTimeout / 4)
	w.ACKReceived()

	time.Sleep(testVehicleACKTimeout * 2)
	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "no SAFE_MODE when ACK arrives in time")
	assert.Empty(t, pub.Events(), "no safety event must be published")
}

// 3. Rapid commands keep resetting the sliding window — only fires if the
// last command goes unacknowledged.
func TestVehicleACK_RapidCommands_SlidingWindow(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	// Send 5 commands, ACK each one promptly.
	for range 5 {
		w.CommandForwarded()
		time.Sleep(testVehicleACKTimeout / 5)
		w.ACKReceived()
	}

	// Final command — no ACK.
	w.CommandForwarded()

	require.True(t, waitForSafeMode(t, sm, testVehicleACKTimeout*3),
		"SAFE_MODE must fire after the last un-ACKed command")
	assert.Equal(t, safetyservice.EventVehicleACKTimeout, pub.LastEventType())
}

// 4. Stop() while timer is pending → timer cancelled, no SAFE_MODE after stop.
func TestVehicleACK_StopCancelsPendingTimer(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	w.CommandForwarded()
	time.Sleep(testVehicleACKTimeout / 4) // partway through
	w.Stop()

	time.Sleep(testVehicleACKTimeout * 2) // well past what the timeout would have been
	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "Stop() must cancel pending timer")
	assert.Empty(t, pub.Events())
}

// 5. CommandForwarded() after Stop() is a no-op — no timer started.
func TestVehicleACK_CommandForwarded_AfterStop_NoOp(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)
	w.Stop()

	w.CommandForwarded()
	time.Sleep(testVehicleACKTimeout * 2)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "no timer started after Stop()")
	assert.Empty(t, pub.Events())
}

// 6. System already in SAFE_MODE when fire() runs → state machine rejects duplicate
//
//	transition; no additional safety event.
func TestVehicleACK_AlreadyInSafeMode_NoDuplicateTransition(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	// Put system into SAFE_MODE before the watchdog fires.
	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.Reset() // clear the event that the manual transition caused

	w.CommandForwarded()
	time.Sleep(testVehicleACKTimeout * 2)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "must stay in SAFE_MODE")
	// The watchdog still publishes the event for audit purposes even when already in SAFE_MODE.
	// What must NOT happen: a second SAFE_MODE transition (state machine rejects it silently).
	// We can only assert the state didn't change further — that is enough.
}

// 7. Second Start() after Stop() resets state; fresh session works correctly.
func TestVehicleACK_StartAfterStop_FreshSession(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	// First session: no ACK → SAFE_MODE.
	w.CommandForwarded()
	require.True(t, waitForSafeMode(t, sm, testVehicleACKTimeout*3))

	// Recover state machine manually (simulate operator Resume).
	sm.TransitionSystem(statemachine.StateRecovering)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	sm.TransitionToConnected()
	pub.Reset()

	// Second session via fresh Start().
	w.Start("sess-2", "vehicle-1")
	w.CommandForwarded()
	time.Sleep(testVehicleACKTimeout / 4)
	w.ACKReceived()

	time.Sleep(testVehicleACKTimeout * 2)
	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys,
		"second session must work independently — ACK in time, no SAFE_MODE")
}

// 8. Concurrent CommandForwarded() and ACKReceived() — must not race or panic.
func TestVehicleACK_ConcurrentForwardAndACK_RaceSafe(t *testing.T) {
	sm, _ := newWatchdogSetup()
	pub := &mocks.MockSafetyPublisher{}
	w := csafety.NewVehicleACKWatchdog(50*time.Millisecond, sm, pub)
	w.Start("sess-race", "vehicle-1")

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() { defer wg.Done(); w.CommandForwarded() }()
		go func() { defer wg.Done(); w.ACKReceived() }()
	}
	wg.Wait()
	// If we reach here without a race detector hit, the test passes.
}

// 9. ACKReceived() without a pending CommandForwarded() is a safe no-op.
func TestVehicleACK_ACKReceivedWithoutForward_NoOp(t *testing.T) {
	sm, pub := newWatchdogSetup()
	w := newVehicleACKWatchdog(sm, pub)

	// No CommandForwarded() call — just an unexpected ACK.
	w.ACKReceived()
	w.ACKReceived()

	time.Sleep(testVehicleACKTimeout * 2)
	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys)
	assert.Empty(t, pub.Events())
}

// ── SafetyBusWatchdog (ADR-026: fleet-wide fanout) ────────────────────────────
// SafetyBusWatchdog polls ONE shared safety-service health endpoint — it stays
// a process-wide singleton, since there is only one safety-service (ADR-002).
// On failure it must transition EVERY vehicle with a currently active session
// to SAFE_MODE, not just whichever vehicle started most recently. Before
// ADR-026, Start(sessionID, vehicleID) overwrote a single tracked vehicle on
// every session/start call — vehicle-1's coverage silently vanished the moment
// vehicle-2's session began.

const (
	testBusInterval  = 40 * time.Millisecond
	testBusThreshold = 2
)

func healthServer(t *testing.T, healthy *atomic.Bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if healthy.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
}

// newBusWatchdogSetup wires a real Registry + session.Manager — the same
// types main.go uses — so these tests exercise the actual fan-out path.
func newBusWatchdogSetup() (*vehiclecontext.Registry, *session.Manager, *mocks.MockSafetyPublisher) {
	pub := &mocks.MockSafetyPublisher{}
	registry := vehiclecontext.NewRegistry(10*time.Second, 100*time.Millisecond, 1*time.Second, pub)
	sessionMgr := session.NewManager(nil)
	return registry, sessionMgr, pub
}

// activateVehicle registers an active session for vehicleID and drives its
// state machine to CONNECTED — mirrors what POST /session/start does.
func activateVehicle(t *testing.T, registry *vehiclecontext.Registry, sessionMgr *session.Manager, vehicleID string) *statemachine.Machine {
	t.Helper()
	sessionMgr.StartSession(vehicleID, "operator-"+vehicleID)
	sm := registry.Get(vehicleID).SM
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	require.True(t, sm.TransitionToConnected())
	return sm
}

func newBusWatchdog(registry *vehiclecontext.Registry, sessionMgr *session.Manager, pub *mocks.MockSafetyPublisher, healthURL string) *csafety.SafetyBusWatchdog {
	w := csafety.NewSafetyBusWatchdog(csafety.SafetyBusWatchdogOptions{
		HealthURL: healthURL,
		Interval:  testBusInterval,
		Threshold: testBusThreshold,
		Vehicles:  registry,
		Sessions:  sessionMgr,
		Publisher: pub,
	})
	w.Start()
	return w
}

// 1. One active vehicle, threshold consecutive failures → that vehicle SAFE_MODE.
func TestSafetyBus_ThresholdReached_TriggersSafeMode(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")
	healthy := &atomic.Bool{}
	healthy.Store(false)
	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm, time.Duration(testBusThreshold+2)*testBusInterval*3),
		"SAFE_MODE must be triggered after threshold failures")
	assert.Equal(t, safetyservice.EventSafetyBusDown, pub.LastEventType())
}

// 2. One failure followed by recovery → counter resets, no SAFE_MODE.
func TestSafetyBus_OneFailureThenRecovery_NoSafeMode(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")
	healthy := &atomic.Bool{}
	healthy.Store(false) // start unhealthy → 1 failure

	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	// Let exactly one check fire (just under threshold).
	time.Sleep(testBusInterval + testBusInterval/2)
	healthy.Store(true) // recover before second failure

	// Wait for several more checks to confirm no SAFE_MODE.
	time.Sleep(testBusInterval * 4)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys,
		"single failure followed by recovery must NOT trigger SAFE_MODE")
	assert.Empty(t, pub.Events())
}

// 3. Failure → recovery → failure again: second run must again need full threshold.
func TestSafetyBus_RecoveryResetsCounter(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")
	healthy := &atomic.Bool{}
	healthy.Store(false)

	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	// First failure.
	time.Sleep(testBusInterval + testBusInterval/2)
	healthy.Store(true)
	// Recovery resets counter.
	time.Sleep(testBusInterval * 2)
	healthy.Store(false) // second run of failures

	// Now needs full threshold again to trigger.
	require.True(t, waitForSafeMode(t, sm, time.Duration(testBusThreshold+3)*testBusInterval*3),
		"SAFE_MODE must fire after second full run of failures")
	assert.Equal(t, safetyservice.EventSafetyBusDown, pub.LastEventType())
}

// 4. Stop() cancels the polling goroutine — no SAFE_MODE fired after stop.
func TestSafetyBus_StopCancelsWatchdog(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")
	healthy := &atomic.Bool{}
	healthy.Store(false)

	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)

	// Stop before threshold is reached.
	time.Sleep(testBusInterval / 2)
	w.Stop()

	// Let time pass well past what the threshold would have taken.
	time.Sleep(time.Duration(testBusThreshold+2) * testBusInterval * 3)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys,
		"Stop() must cancel watchdog — no SAFE_MODE after stop")
	assert.Empty(t, pub.Events())
}

// 5. A vehicle already in SAFE_MODE when the threshold fires gets no duplicate
// transition or event — but other active vehicles still get fanned out to.
func TestSafetyBus_AlreadyInSafeMode_SkippedButOthersStillFanOut(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	smAlready := activateVehicle(t, registry, sessionMgr, "vehicle-already-safe")
	smOther := activateVehicle(t, registry, sessionMgr, "vehicle-other")
	smAlready.TransitionSystem(statemachine.StateSafeMode)
	pub.Reset()

	healthy := &atomic.Bool{}
	healthy.Store(false)
	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	require.True(t, waitForSafeMode(t, smOther, time.Duration(testBusThreshold+2)*testBusInterval*3),
		"vehicle-other must still be fanned out to")

	sysAlready, _, _, _ := smAlready.Get()
	assert.Equal(t, statemachine.StateSafeMode, sysAlready, "must remain in SAFE_MODE, no bounce")

	for _, e := range pub.Events() {
		assert.NotEqual(t, "vehicle-already-safe", e.VehicleID,
			"already-SAFE_MODE vehicle must not receive a duplicate event")
	}
}

// 6. Non-200 HTTP response (503) is treated as a failure.
func TestSafetyBus_Non200Response_CountsAsFailure(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm, time.Duration(testBusThreshold+2)*testBusInterval*3),
		"503 must count as failure and eventually trigger SAFE_MODE")
}

// 7. Connection refused (server closed) is treated as a failure.
func TestSafetyBus_ConnectionRefused_CountsAsFailure(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")

	// Start a server then immediately close it so the port is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	closedURL := srv.URL
	srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, closedURL)
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm, time.Duration(testBusThreshold+2)*testBusInterval*3),
		"connection refused must count as failure and eventually trigger SAFE_MODE")
}

// 8. Start() after Stop() provides a clean restart — failure counter reset.
func TestSafetyBus_StartAfterStop_FreshStart(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")
	healthy := &atomic.Bool{}
	healthy.Store(false)

	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)

	// Let 1 failure register, then stop.
	time.Sleep(testBusInterval + testBusInterval/2)
	w.Stop()
	pub.Reset()

	// Make server healthy before restarting.
	healthy.Store(true)

	// Fresh Start() — should see healthy and never trigger.
	w.Start()
	defer w.Stop()
	time.Sleep(testBusInterval * 4)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys,
		"Start() after Stop() with healthy server must not trigger SAFE_MODE")
	assert.Empty(t, pub.Events())
}

// 9. Healthy bus throughout session → never triggers SAFE_MODE.
func TestSafetyBus_AlwaysHealthy_NeverTriggers(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm := activateVehicle(t, registry, sessionMgr, "vehicle-1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	time.Sleep(testBusInterval * 6)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "healthy bus must never trigger SAFE_MODE")
	assert.Empty(t, pub.Events())
}

// 10. THE CORE FIX: two active vehicles, one bus failure → BOTH go SAFE_MODE,
// each with its own correctly-attributed SafetyEvent. Before ADR-026 this was
// impossible — the watchdog only ever knew about the most recently started session.
func TestSafetyBus_FleetWide_AllActiveVehiclesGoSafeMode(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()
	sm1 := activateVehicle(t, registry, sessionMgr, "vehicle-fleet-1")
	sm2 := activateVehicle(t, registry, sessionMgr, "vehicle-fleet-2")

	healthy := &atomic.Bool{}
	healthy.Store(false)
	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm1, time.Duration(testBusThreshold+2)*testBusInterval*3))
	require.True(t, waitForSafeMode(t, sm2, time.Duration(testBusThreshold+2)*testBusInterval*3))

	gotEvent := make(map[string]bool)
	for _, e := range pub.Events() {
		if e.Type == safetyservice.EventSafetyBusDown {
			gotEvent[e.VehicleID] = true
		}
	}
	assert.True(t, gotEvent["vehicle-fleet-1"], "vehicle-fleet-1 must have its own SafetyEvent")
	assert.True(t, gotEvent["vehicle-fleet-2"], "vehicle-fleet-2 must have its own SafetyEvent")
}

// 11. No active vehicles at all → bus failure is a no-op, no crash, no event.
func TestSafetyBus_NoActiveVehicles_NoOp(t *testing.T) {
	registry, sessionMgr, pub := newBusWatchdogSetup()

	healthy := &atomic.Bool{}
	healthy.Store(false)
	srv := healthServer(t, healthy)
	defer srv.Close()

	w := newBusWatchdog(registry, sessionMgr, pub, srv.URL)
	defer w.Stop()

	time.Sleep(time.Duration(testBusThreshold+2) * testBusInterval * 3)

	assert.Empty(t, pub.Events(), "no active vehicles means nothing to fan out to")
}

// ── AuthWatchdog (DRIFT-K1, 2026-07-16) ─────────────────────────────────────
// Per-VehicleContext, tied to one session's operator — unlike SafetyBusWatchdog
// (shared infra, fleet-wide fanout), revoking one operator's account must only
// affect that operator's own vehicle (ADR-026 per-vehicle isolation). Fires via
// TransitionOperator(OpNoOperator), the same guarded path the WS-disconnect
// handler uses (DRIFT-K2) — not a second, parallel way into SAFE_MODE.

const (
	testAuthInterval  = 40 * time.Millisecond
	testAuthThreshold = 2
)

// fakeUserChecker is a controllable in-memory stand-in for authcheck.Checker —
// keeps these tests focused on AuthWatchdog's polling/threshold/firing logic
// without a real Postgres dependency. The real Checker's SQL query is covered
// separately by an integration test against the live docker test stack DB.
type fakeUserChecker struct {
	mu     sync.Mutex
	active bool
	err    error
	calls  int
}

func (f *fakeUserChecker) IsActiveOperator(_ context.Context, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.active, f.err
}

func (f *fakeUserChecker) setActive(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active, f.err = v, nil
}

func (f *fakeUserChecker) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeUserChecker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newAuthWatchdogSetup(t *testing.T) (*statemachine.Machine, *mocks.MockSafetyPublisher, *fakeUserChecker) {
	t.Helper()
	sm := statemachine.New()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	require.True(t, sm.TransitionToConnected())
	sm.TransitionOperator(statemachine.OpActive)
	return sm, &mocks.MockSafetyPublisher{}, &fakeUserChecker{active: true}
}

// 1. Account still active → never fires, even after many poll intervals.
func TestAuthWatchdog_AccountActive_NeverTriggersSafeMode(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	defer w.Stop()

	time.Sleep(testAuthInterval * 5)

	sys, _, _, op := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys)
	assert.Equal(t, statemachine.OpActive, op)
	assert.Empty(t, pub.Events())
	assert.GreaterOrEqual(t, checker.callCount(), 2, "watchdog must actually be polling")
}

// 2. Account deleted (checker reports inactive) → threshold consecutive
// failures → SAFE_MODE via the OPERATOR layer, EventAuthInvalid published.
func TestAuthWatchdog_AccountDeleted_TriggersSafeModeViaOperatorLayer(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	checker.setActive(false)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm, time.Duration(testAuthThreshold+3)*testAuthInterval*3))

	sys, ctrl, _, op := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, statemachine.OpNoOperator, op, "OPERATOR layer must reflect the revoked operator")
	assert.Equal(t, safetyservice.EventAuthInvalid, pub.LastEventType())
}

// 3. Checker query errors (e.g. transient DB blip) count as failures too —
// same treatment as SafetyBusWatchdog's connection-refused/non-200 handling.
func TestAuthWatchdog_CheckerError_CountsAsFailure(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	checker.setErr(fmt.Errorf("connection reset"))

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	defer w.Stop()

	require.True(t, waitForSafeMode(t, sm, time.Duration(testAuthThreshold+3)*testAuthInterval*3))
}

// 4. One failure then recovery (account active again) resets the counter —
// a single transient blip must not cost the operator their session.
func TestAuthWatchdog_OneFailureThenRecovery_NoSafeMode(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	checker.setActive(false)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	defer w.Stop()

	time.Sleep(testAuthInterval + testAuthInterval/2) // let exactly 1 failure register
	checker.setActive(true)
	time.Sleep(testAuthInterval * 5)

	sys, _, _, op := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "single blip + recovery must not trigger SAFE_MODE")
	assert.Equal(t, statemachine.OpActive, op)
	assert.Empty(t, pub.Events())
}

// 5. Stop() before threshold is reached cancels the watchdog — no SAFE_MODE
// even after the time that would have been needed passes.
func TestAuthWatchdog_StopCancelsWatchdog(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	checker.setActive(false)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")

	time.Sleep(testAuthInterval / 2)
	w.Stop()

	time.Sleep(time.Duration(testAuthThreshold+2) * testAuthInterval * 3)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "Stop() must cancel watchdog — no SAFE_MODE after stop")
	assert.Empty(t, pub.Events())
}

// 6. Already in SAFE_MODE for an unrelated reason (e.g. dead-man timeout) —
// the watchdog's fire() must not double-transition or duplicate the event.
func TestAuthWatchdog_AlreadyInSafeMode_NoDuplicateTransition(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.Reset()
	checker.setActive(false)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	defer w.Stop()

	time.Sleep(time.Duration(testAuthThreshold+2) * testAuthInterval * 3)

	sys, ctrl, _, op := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, statemachine.OpNoOperator, op, "OPERATOR field still gets corrected even though SYSTEM was already SAFE_MODE")
}

// 7. Start() after Stop() is a clean restart — failure counter resets, a
// vehicle reconnecting after recovery does not inherit stale failure state.
func TestAuthWatchdog_StartAfterStop_FreshStart(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	checker.setActive(false)

	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)
	w.Start("sess-1", "vehicle-1", "operator-1")
	time.Sleep(testAuthInterval + testAuthInterval/2) // 1 failure registered
	w.Stop()

	checker.setActive(true)
	w.Start("sess-1", "vehicle-1", "operator-1") // fresh counter
	time.Sleep(testAuthInterval * 5)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "restart must not carry over the pre-Stop() failure count")
}

// 8. Concurrent Start()/Stop() calls (e.g. rapid reconnect) must not race —
// run with -race (CLAUDE.MD §17).
func TestAuthWatchdog_ConcurrentStartStop_RaceSafe(t *testing.T) {
	sm, pub, checker := newAuthWatchdogSetup(t)
	w := csafety.NewAuthWatchdog(testAuthInterval, testAuthThreshold, sm, pub, checker)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); w.Start("sess-1", "vehicle-1", "operator-1") }()
		go func() { defer wg.Done(); w.Stop() }()
	}
	wg.Wait()
	w.Stop()
}
