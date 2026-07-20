// safety_bus_integration_test.go closes the gap documented in safety_test.go's header comment
// (SAFETYBUS-01, Sprint 42): the Safety Test Suite exercises the trigger logic (state machine →
// Publisher) against MockSafetyPublisher, but never the real internal/safetyservice.Bus that
// cmd/safety-service/main.go wires up in production via HTTPPublisher → POST /safety/event.
//
// newSafetyMux (cmd/safety-service/main.go) lives in package main and is not importable from
// tests/unit (package unit_test). Rather than exporting production code for testability — out of
// scope for this sprint, see tasks/backlog.md — this file duplicates newSafetyMux's two-line
// POST /safety/event handler in a local httptest.Server and wires a real HTTPPublisher against a
// real safetyservice.Bus, verified through bus.GetSafetyState().
package unit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBusIntegrationTestServer wires a real safetyservice.Bus behind a local httptest.Server that
// duplicates the POST /safety/event handler from cmd/safety-service/main.go's newSafetyMux, and
// returns an HTTPPublisher pointed at it plus the raw server URL (for tests that need to bypass
// HTTPPublisher, e.g. to POST a malformed body).
func newBusIntegrationTestServer(t *testing.T) (bus *safetyservice.Bus, pub *csafety.HTTPPublisher, serverURL string) {
	t.Helper()
	bus = safetyservice.NewBus()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /safety/event", func(w http.ResponseWriter, r *http.Request) {
		var event safetyservice.SafetyEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		bus.PublishSafetyEvent(event)
		w.WriteHeader(http.StatusAccepted)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return bus, csafety.NewHTTPPublisher(server.URL), server.URL
}

// connectedStateMachine builds a Machine already in CONNECTED (IDLE → CONNECTING → AUTHENTICATED →
// CONNECTED), the same sequence connectSession in safety_test.go uses — a bare statemachine.New()
// would reject the watchdog's SAFE_MODE transition from IDLE and log a misleading warning.
func connectedStateMachine(t *testing.T) *statemachine.Machine {
	t.Helper()
	sm := statemachine.New()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	require.True(t, sm.TransitionToConnected(), "TransitionToConnected must succeed from AUTHENTICATED")
	return sm
}

// --- ADR-006-CRITICAL Trigger 2: Dead-man Switch Timeout, against the real Bus ---

func TestSafetyBusIntegration_DeadmanTimeout_ReachesRealBusOverHTTP(t *testing.T) {
	bus, pub, _ := newBusIntegrationTestServer(t)
	sm := connectedStateMachine(t)

	watchdog := csafety.NewDeadmanWatchdog(50*time.Millisecond, sm, pub)
	watchdog.Start("session-1", "vehicle-1")
	watchdog.Reset() // arm the watchdog — countdown starts now

	// fire() runs on the timer's own goroutine and posts over real HTTP, so the real Bus's state
	// update is asynchronous relative to this test goroutine — poll instead of asserting immediately.
	require.Eventually(t, func() bool {
		return bus.GetSafetyState().SafeMode
	}, 2*time.Second, 10*time.Millisecond, "dead-man timeout must reach the real safetyservice.Bus over HTTP")

	state := bus.GetSafetyState()
	assert.Equal(t, safetyservice.EventDeadmanTimeout, state.LastEvent)
}

// --- ADR-006-CRITICAL Trigger 3: Command ACK Timeout, against the real Bus ---

func TestSafetyBusIntegration_ACKTimeout_ReachesRealBusOverHTTP(t *testing.T) {
	bus, pub, _ := newBusIntegrationTestServer(t)
	sm := connectedStateMachine(t)

	watcher := csafety.NewACKTimeoutWatcher(50*time.Millisecond, sm, pub)
	watcher.CommandReceived("session-1", "vehicle-1")
	// Do NOT call CommandACKed() — let the timer fire.

	require.Eventually(t, func() bool {
		return bus.GetSafetyState().SafeMode
	}, 2*time.Second, 10*time.Millisecond, "ACK timeout must reach the real safetyservice.Bus over HTTP")

	state := bus.GetSafetyState()
	assert.Equal(t, safetyservice.EventACKTimeout, state.LastEvent)
}

// --- Error path: malformed request body must not corrupt Bus state ---

func TestSafetyBusIntegration_MalformedEventBody_Returns400AndBusUnaffected(t *testing.T) {
	bus, _, serverURL := newBusIntegrationTestServer(t)
	before := bus.GetSafetyState()

	resp, err := http.Post(serverURL+"/safety/event", "application/json", strings.NewReader("not-json"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, before, bus.GetSafetyState(), "malformed body must not change Bus state")
}
