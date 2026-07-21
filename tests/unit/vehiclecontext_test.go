// VehicleContextRegistry Tests (ADR-026, Sprint 17 MV-01).
//
// This is the core fix for the Sprint-17 safety defect: before ADR-026, the
// State Machine and both Deadman-/ACK-Watchdogs were process-wide singletons.
// Two operators controlling two different vehicles would silently overwrite
// each other's safety monitoring. These tests encode that property directly —
// most importantly the *_Isolation tests, which are a literal reproduction of
// the original incident and must stay red until the Registry isolates state
// per vehicle.
//
// Written before the implementation exists (TDD) — see Sprint 17 in
// tasks/current-sprint.md. `vehiclecontext.Registry` does not exist yet;
// this file is expected to fail to compile until MV-01 is implemented.
package unit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"avoc/internal/controlserver/statemachine"
	vc "avoc/internal/controlserver/vehiclecontext"
	"avoc/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Short timeouts so isolation tests that wait for a real watchdog fire stay fast.
// main.go wires the Registry with the csafety.Default* constants instead.
const (
	testRegistryDeadmanTimeout    = 80 * time.Millisecond
	testRegistryACKTimeout        = 80 * time.Millisecond
	testRegistryVehicleACKTimeout = 80 * time.Millisecond
)

func newRegistry() (*vc.Registry, *mocks.MockSafetyPublisher) {
	pub := &mocks.MockSafetyPublisher{}
	return vc.NewRegistry(testRegistryDeadmanTimeout, testRegistryACKTimeout, testRegistryVehicleACKTimeout, pub), pub
}

// connectAll drives a slice of VehicleContexts to CONNECTED so SAFE_MODE transitions are valid.
func connectAll(t *testing.T, ctxs ...*vc.VehicleContext) {
	t.Helper()
	for _, c := range ctxs {
		c.SM.TransitionSystem(statemachine.StateConnecting)
		c.SM.TransitionSystem(statemachine.StateAuthenticated)
		require.True(t, c.SM.TransitionToConnected())
	}
}

// ── Lazy creation & identity ──────────────────────────────────────────────────

// 1. First Get() for an unknown vehicle ID lazily creates a fully-wired VehicleContext.
func TestRegistry_Get_CreatesFreshContext(t *testing.T) {
	r, _ := newRegistry()
	ctx := r.Get("vehicle-001")
	require.NotNil(t, ctx)
	require.NotNil(t, ctx.SM)
	require.NotNil(t, ctx.Deadman)
	require.NotNil(t, ctx.ACKTimeoutWatcher)
	require.NotNil(t, ctx.VehicleACKWatchdog)

	sys, _, _, _ := ctx.SM.Get()
	assert.Equal(t, statemachine.StateIdle, sys, "fresh VehicleContext must start at IDLE")
}

// DRIFT-K1 (2026-07-16): AuthWatchdog wiring is opt-in via WithUserChecker — these
// two cases pin down both branches, since main.go's actual wiring (WithUserChecker
// + Get()'s `if r.userChecker != nil`) previously had no direct test coverage of its
// own (only AuthWatchdog itself, constructed directly, was unit-tested).

func TestRegistry_Get_WithoutUserChecker_AuthWatchdogStaysNil(t *testing.T) {
	r, _ := newRegistry()
	ctx := r.Get("vehicle-001")
	assert.Nil(t, ctx.AuthWatchdog, "no WithUserChecker call → AuthWatchdog must stay nil (callers nil-check before use)")
}

func TestRegistry_Get_WithUserChecker_AuthWatchdogIsWired(t *testing.T) {
	pub := &mocks.MockSafetyPublisher{}
	r := vc.NewRegistry(testRegistryDeadmanTimeout, testRegistryACKTimeout, testRegistryVehicleACKTimeout, pub).
		WithUserChecker(&fakeUserChecker{active: true})
	ctx := r.Get("vehicle-001")
	require.NotNil(t, ctx.AuthWatchdog, "WithUserChecker must cause Get() to wire an AuthWatchdog")
}

// 2. Repeated Get() with the same ID returns the IDENTICAL instance — no re-creation,
// no lost state between calls.
func TestRegistry_Get_IsIdempotent(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-001")
	assert.Same(t, ctx1, ctx2, "Get() must return the same VehicleContext for the same vehicle ID")
}

// 3. Different vehicle IDs get structurally distinct VehicleContexts (and distinct
// SM/Deadman/ACKWatchdog instances inside them — no accidental sharing).
func TestRegistry_Get_DifferentIDs_AreDistinct(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	assert.NotSame(t, ctx1, ctx2)
	assert.NotSame(t, ctx1.SM, ctx2.SM)
	assert.NotSame(t, ctx1.Deadman, ctx2.Deadman)
	assert.NotSame(t, ctx1.ACKTimeoutWatcher, ctx2.ACKTimeoutWatcher)
	assert.NotSame(t, ctx1.VehicleACKWatchdog, ctx2.VehicleACKWatchdog)
}

// 4. Mutations via the returned pointer persist across subsequent Get() calls —
// guards against an accidental by-value copy bug inside the registry.
func TestRegistry_MutationsPersist_AcrossGetCalls(t *testing.T) {
	r, _ := newRegistry()
	ctx := r.Get("vehicle-001")
	ctx.SM.TransitionSystem(statemachine.StateConnecting)

	ctxAgain := r.Get("vehicle-001")
	sys, _, _, _ := ctxAgain.SM.Get()
	assert.Equal(t, statemachine.StateConnecting, sys)
}

// 5. Empty string is treated like any other map key — deliberate, documented
// behavior. The registry does not validate vehicle IDs; callers validate at the
// HTTP/WS boundary.
func TestRegistry_Get_EmptyVehicleID_DoesNotPanic(t *testing.T) {
	r, _ := newRegistry()
	require.NotPanics(t, func() {
		ctx := r.Get("")
		assert.NotNil(t, ctx)
	})
}

// ── Safety isolation — the actual bug this sprint fixes ───────────────────────

// 6. THE CORE FIX: transitioning one vehicle's state machine to SAFE_MODE must
// never affect another vehicle's state machine.
func TestRegistry_StateIsolation_SafeModeDoesNotLeak(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	connectAll(t, ctx1, ctx2)

	ctx1.SM.TransitionSystem(statemachine.StateSafeMode)

	sys1, _, _, _ := ctx1.SM.Get()
	sys2, _, _, _ := ctx2.SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys1, "vehicle-001 must be in SAFE_MODE")
	assert.Equal(t, statemachine.StateConnected, sys2, "vehicle-002 must be unaffected — still CONNECTED")
}

// 7. Deadman-Watchdog isolation: Operator A releases the dead-man switch on
// vehicle-001 (timeout fires) while Operator B's vehicle-002 is untouched.
// Reproduces the original incident from the Grill-Me session 1:1.
func TestRegistry_DeadmanWatchdog_Isolation(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	connectAll(t, ctx1, ctx2)

	ctx1.Deadman.Start("sess-1", "vehicle-001")
	ctx1.Deadman.Reset() // arms the watchdog (simulates first DEADMAN_HOLD)
	// ctx2's Deadman is deliberately never started — Operator B is driving fine.

	require.True(t, waitForSafeMode(t, ctx1.SM, 500*time.Millisecond),
		"vehicle-001 deadman must fire SAFE_MODE on its own timeout")

	sys2, _, _, _ := ctx2.SM.Get()
	assert.Equal(t, statemachine.StateConnected, sys2,
		"vehicle-002 must remain CONNECTED — unaffected by vehicle-001's deadman timeout")

	ctx1.Deadman.Stop()
}

// 8. ACKTimeoutWatcher isolation: this is the SERVER's own control-loop-budget
// watchdog (did the control-server ACK the operator's command in time?) — distinct
// from VehicleACKWatchdog (did the vehicle ACK a forwarded command?). A slow ACK
// on vehicle-001 must not affect vehicle-002.
func TestRegistry_ACKTimeoutWatcher_Isolation(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	connectAll(t, ctx1, ctx2)

	ctx1.ACKTimeoutWatcher.CommandReceived("sess-1", "vehicle-001")
	// ctx2's ACKTimeoutWatcher never receives a command.

	require.True(t, waitForSafeMode(t, ctx1.SM, 500*time.Millisecond),
		"vehicle-001 must SAFE_MODE after the control-loop ACK budget is exceeded")

	sys2, _, _, _ := ctx2.SM.Get()
	assert.Equal(t, statemachine.StateConnected, sys2,
		"vehicle-002 must remain CONNECTED — unaffected by vehicle-001's ACK timeout")
}

// 9. VehicleACKWatchdog isolation: vehicle-001 stops ACKing commands (timeout
// fires) while vehicle-002's ACK watchdog was never started and stays unaffected.
func TestRegistry_VehicleACKWatchdog_Isolation(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	connectAll(t, ctx1, ctx2)

	ctx1.VehicleACKWatchdog.Start("sess-1", "vehicle-001")
	ctx1.VehicleACKWatchdog.CommandForwarded()
	// ctx2's ACK watchdog is deliberately never started.

	require.True(t, waitForSafeMode(t, ctx1.SM, 500*time.Millisecond),
		"vehicle-001 must SAFE_MODE after a missing ACK")

	sys2, _, _, _ := ctx2.SM.Get()
	assert.Equal(t, statemachine.StateConnected, sys2,
		"vehicle-002 must remain CONNECTED — unaffected by vehicle-001's missing ACK")

	ctx1.VehicleACKWatchdog.Stop()
}

// 10. Stopping one vehicle's watchdog must not stop another vehicle's watchdog —
// the symmetric case to #7/#8: Operator A ends their session cleanly while
// Operator B is still actively driving and must keep full safety coverage.
func TestRegistry_Stop_DoesNotAffectOtherVehicle(t *testing.T) {
	r, _ := newRegistry()
	ctx1 := r.Get("vehicle-001")
	ctx2 := r.Get("vehicle-002")
	connectAll(t, ctx1, ctx2)

	ctx1.Deadman.Start("sess-1", "vehicle-001")
	ctx1.Deadman.Reset()
	ctx2.Deadman.Start("sess-2", "vehicle-002")
	ctx2.Deadman.Reset()
	ctx1.Deadman.Stop() // Operator A ends session 1 cleanly.

	// vehicle-002's deadman must still be live and fire on its own timeout.
	require.True(t, waitForSafeMode(t, ctx2.SM, 500*time.Millisecond),
		"vehicle-002 deadman must still fire — it was never stopped")

	sys1, _, _, _ := ctx1.SM.Get()
	assert.Equal(t, statemachine.StateConnected, sys1,
		"vehicle-001 must remain CONNECTED — its watchdog was deliberately stopped, no SAFE_MODE")
}

// ── Concurrency ────────────────────────────────────────────────────────────────

// 11. Concurrent Get() calls for the SAME new vehicle ID must create EXACTLY ONE
// VehicleContext — the thundering-herd / double-checked-locking race. This is
// what happens in production when an operator's browser polls GET /vehicles
// while their session/start request races a parallel WS auto-connect.
// Run with -race.
func TestRegistry_ConcurrentGet_SameNewID_CreatesExactlyOneInstance(t *testing.T) {
	r, _ := newRegistry()
	const n = 100
	results := make([]*vc.VehicleContext, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = r.Get("vehicle-race")
		}(i)
	}
	wg.Wait()

	first := results[0]
	require.NotNil(t, first)
	for i, ctx := range results {
		assert.Same(t, first, ctx, "goroutine %d got a different VehicleContext instance", i)
	}
}

// 12. Concurrent Get() for many DIFFERENT new vehicle IDs must not race or
// corrupt the registry — each ID ends up with exactly one distinct context.
// Run with -race.
func TestRegistry_ConcurrentGet_DifferentIDs_NoRaceNoCorruption(t *testing.T) {
	r, _ := newRegistry()
	const n = 100
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("vehicle-%d", i)
	}

	results := make([]*vc.VehicleContext, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = r.Get(ids[i])
		}(i)
	}
	wg.Wait()

	seen := make(map[*vc.VehicleContext]bool, n)
	for i, ctx := range results {
		require.NotNil(t, ctx, "vehicle %s got nil context", ids[i])
		assert.False(t, seen[ctx], "duplicate VehicleContext pointer for distinct vehicle IDs")
		seen[ctx] = true
	}
	assert.Len(t, seen, n)
}

// 13. Mixed concurrent load: some goroutines repeatedly Get() an already-existing
// ID while others create brand-new IDs at the same time. No race, no panic,
// no cross-contamination between the warm ID and the freshly created ones.
// Run with -race.
func TestRegistry_ConcurrentGet_MixedReadAndCreate_NoRace(t *testing.T) {
	r, _ := newRegistry()
	existing := r.Get("vehicle-warm") // pre-created before the concurrent load starts

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := r.Get("vehicle-warm")
			assert.Same(t, existing, ctx)
		}()
	}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := r.Get(fmt.Sprintf("vehicle-new-%d", i))
			assert.NotSame(t, existing, ctx)
		}(i)
	}
	wg.Wait()
}
