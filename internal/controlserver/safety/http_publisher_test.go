package safety

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avoc/internal/safetyservice"

	"github.com/stretchr/testify/assert"
)

// captureLog redirects the standard `log` package output (used by HTTPPublisher, not the
// structured pkg/logger) for the duration of the test and restores it afterwards.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return &buf
}

// runWithDeadline fails the test if fn does not return within d — used to prove the swallow
// path returns instead of blocking the caller.
func runWithDeadline(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("call did not return within %s — caller would be blocked", d)
	}
}

func TestTriggerEmergencyStop_Success_SendsExpectedBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPPublisher(server.URL)
	runWithDeadline(t, time.Second, func() {
		p.TriggerEmergencyStop("session-1", "vehicle-1", "operator EMERGENCY_STOP command")
	})

	assert.Equal(t, "/safety/emergency-stop", gotPath)
	assert.Equal(t, "session-1", gotBody["session_id"])
	assert.Equal(t, "vehicle-1", gotBody["vehicle_id"])
	assert.Equal(t, "operator EMERGENCY_STOP command", gotBody["reason"])
}

// TestTriggerEmergencyStop_NetworkError_SwallowedAndLogged verifies the documented swallow
// behaviour: the safety-service being unreachable is logged but must not panic, error, or block
// the caller (the caller — command.Engine.handleEmergencyStop — has already committed the
// SAFE_MODE transition and must not wait on this call, ADR-002).
func TestTriggerEmergencyStop_NetworkError_SwallowedAndLogged(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close() // closed immediately — URL now refuses connections

	p := NewHTTPPublisher(unreachableURL)
	runWithDeadline(t, time.Second, func() {
		p.TriggerEmergencyStop("session-1", "vehicle-1", "operator EMERGENCY_STOP command")
	})

	assert.Contains(t, buf.String(), "failed to reach safety service")
	assert.Contains(t, buf.String(), "/safety/emergency-stop")
}

// TestTriggerEmergencyStop_Timeout_SwallowedAndLogged covers a safety-service that hangs. The
// production client has no configured Timeout (NewHTTPPublisher), so this test overrides the
// unexported client field directly (same package) purely to make a hang observable within test
// time — it does not change production behaviour.
func TestTriggerEmergencyStop_Timeout_SwallowedAndLogged(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPPublisher(server.URL)
	p.client = &http.Client{Timeout: 20 * time.Millisecond}

	runWithDeadline(t, time.Second, func() {
		p.TriggerEmergencyStop("session-1", "vehicle-1", "reason")
	})

	assert.Contains(t, buf.String(), "failed to reach safety service")
}

// TestTriggerEmergencyStop_Non2xxStatus_NotTreatedAsError documents the current, real behaviour
// of post(): only a transport-level error (client.Post returning err) is logged — a non-2xx HTTP
// status from safety-service is NOT inspected at all and produces no log line. This is existing
// production behaviour (ADR-002 swallow path), not something this test suite changes.
func TestTriggerEmergencyStop_Non2xxStatus_NotTreatedAsError(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := NewHTTPPublisher(server.URL)
	runWithDeadline(t, time.Second, func() {
		p.TriggerEmergencyStop("session-1", "vehicle-1", "reason")
	})

	assert.Empty(t, buf.String(), "a non-2xx response is currently not logged at all — status code is never inspected")
}

func TestPublishEvent_Success_SendsExpectedBody(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPPublisher(server.URL)
	runWithDeadline(t, time.Second, func() {
		p.PublishEvent(safetyservice.SafetyEvent{
			SessionID: "session-1",
			VehicleID: "vehicle-1",
			Type:      safetyservice.EventEmergencyStop,
			Reason:    "test",
			Timestamp: time.Now(),
		})
	})

	assert.Equal(t, "/safety/event", gotPath)
}

func TestPublishEvent_NetworkError_SwallowedAndLogged(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	p := NewHTTPPublisher(unreachableURL)
	runWithDeadline(t, time.Second, func() {
		p.PublishEvent(safetyservice.SafetyEvent{SessionID: "session-1", VehicleID: "vehicle-1"})
	})

	assert.Contains(t, buf.String(), "failed to reach safety service")
	assert.Contains(t, buf.String(), "/safety/event")
}
