package session

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLog redirects the standard `log` package output (used by HTTPSFUPublisher) for the
// duration of the test and restores it afterwards.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return &buf
}

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

func TestNewHTTPSFUPublisher_ReturnsUsablePublisher(t *testing.T) {
	p := NewHTTPSFUPublisher("http://sfu.internal")
	var _ SFUPublisher = p // must satisfy the interface
	assert.NotNil(t, p)
}

func TestPublishSessionEvent_Success_SendsExpectedBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPSFUPublisher(server.URL)
	runWithDeadline(t, time.Second, func() {
		p.PublishSessionEvent("SESSION_CREATED", "session-1", "operator-1")
	})

	assert.Equal(t, "/session/event", gotPath)
	assert.Equal(t, "SESSION_CREATED", gotBody["type"])
	assert.Equal(t, "session-1", gotBody["session_id"])
	assert.Equal(t, "operator-1", gotBody["operator_id"])
}

// TestPublishSessionEvent_NetworkError_SwallowedAndLogged verifies the SFU being unreachable is
// logged but does not panic or block the caller — session lifecycle events to the SFU are
// best-effort (ADR-015, Dumb Media Router).
func TestPublishSessionEvent_NetworkError_SwallowedAndLogged(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	p := NewHTTPSFUPublisher(unreachableURL)
	runWithDeadline(t, time.Second, func() {
		p.PublishSessionEvent("SESSION_CREATED", "session-1", "operator-1")
	})

	assert.Contains(t, buf.String(), "SFU event push failed")
	assert.Contains(t, buf.String(), "SESSION_CREATED")
}

// TestPublishSessionEvent_Timeout_SwallowedAndLogged covers an SFU that hangs. The production
// client has no configured Timeout — this test overrides the unexported field directly (same
// package) purely to make a hang observable within test time, without changing production code.
func TestPublishSessionEvent_Timeout_SwallowedAndLogged(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPSFUPublisher(server.URL)
	p.client = &http.Client{Timeout: 20 * time.Millisecond}

	runWithDeadline(t, time.Second, func() {
		p.PublishSessionEvent("SESSION_CREATED", "session-1", "operator-1")
	})

	assert.Contains(t, buf.String(), "SFU event push failed")
}

// TestPublishSessionEvent_Non2xxStatus_NotTreatedAsError documents the current, real behaviour:
// only a transport-level error is logged — a non-2xx HTTP status from the SFU is never inspected.
func TestPublishSessionEvent_Non2xxStatus_NotTreatedAsError(t *testing.T) {
	buf := captureLog(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := NewHTTPSFUPublisher(server.URL)
	runWithDeadline(t, time.Second, func() {
		p.PublishSessionEvent("SESSION_CREATED", "session-1", "operator-1")
	})

	assert.Empty(t, buf.String(), "a non-2xx response is currently not logged at all — status code is never inspected")
}

func TestPublishSessionEvent_EmptyFields_NoPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewHTTPSFUPublisher(server.URL)
	require.NotPanics(t, func() { p.PublishSessionEvent("", "", "") })
}
