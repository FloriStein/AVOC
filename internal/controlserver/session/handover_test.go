package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/safetyservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopSafetyPublisher satisfies vehiclecontext.NewRegistry's Publisher param without pulling in
// the safety package's mocks (handover tests never trigger a safety event).
type noopSafetyPublisherForHandover struct{}

// --- issueHandoverToken (unexported, direct calls) ---

func TestIssueHandoverToken_NoAuthURLConfigured_ReturnsNil(t *testing.T) {
	h := &HandoverManager{authURL: "", httpClient: &http.Client{}}
	err := h.issueHandoverToken("operator-2")
	require.NoError(t, err)
}

func TestIssueHandoverToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/handover/token", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	h := &HandoverManager{authURL: server.URL, httpClient: &http.Client{}}
	err := h.issueHandoverToken("operator-2")
	require.NoError(t, err)
}

func TestIssueHandoverToken_Non2xxStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	h := &HandoverManager{authURL: server.URL, httpClient: &http.Client{}}
	err := h.issueHandoverToken("operator-2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestIssueHandoverToken_NetworkError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	h := &HandoverManager{authURL: unreachableURL, httpClient: &http.Client{}}
	err := h.issueHandoverToken("operator-2")

	require.Error(t, err)
}

func TestIssueHandoverToken_Timeout_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	h := &HandoverManager{authURL: server.URL, httpClient: &http.Client{Timeout: 20 * time.Millisecond}}
	err := h.issueHandoverToken("operator-2")

	require.Error(t, err)
}

// --- ConfirmHandover: full wiring around issueHandoverToken's error paths ---

func newTestHandoverSetup(t *testing.T, authURL string) (*HandoverManager, *vehiclecontext.Registry, *Manager) {
	t.Helper()
	registry := vehiclecontext.NewRegistry(time.Minute, time.Minute, time.Minute, noopSafetyPublisherForHandover{})
	sessions := NewManager(stubSFUPublisher{})
	hm := NewHandoverManager(registry, sessions, authURL)
	return hm, registry, sessions
}

func (noopSafetyPublisherForHandover) PublishEvent(_ safetyservice.SafetyEvent) {}

func TestConfirmHandover_AuthServiceNon2xx_AbortsHandover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hm, registry, sessions := newTestHandoverSetup(t, server.URL)
	vc := registry.Get("vehicle-001")
	vc.SM.TransitionOperator(statemachine.OpActive)
	fromSession := sessions.CreateSession("vehicle-001", "operator-1", "ACTIVE_OPERATOR")

	require.NoError(t, hm.RequestHandover("vehicle-001", "operator-1", "operator-2"))
	err := hm.ConfirmHandover("vehicle-001", "operator-2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "handover token issuance failed")

	// operator must NOT have changed — the handover was aborted before UpdateOperator.
	stored, ok := sessions.GetSession(fromSession.ID)
	require.True(t, ok)
	assert.Equal(t, "operator-1", stored.OperatorID)
}

func TestConfirmHandover_AuthServiceTimeout_AbortsHandover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	hm, registry, sessions := newTestHandoverSetup(t, server.URL)
	hm.httpClient = &http.Client{Timeout: 20 * time.Millisecond}
	vc := registry.Get("vehicle-001")
	vc.SM.TransitionOperator(statemachine.OpActive)
	sessions.CreateSession("vehicle-001", "operator-1", "ACTIVE_OPERATOR")

	require.NoError(t, hm.RequestHandover("vehicle-001", "operator-1", "operator-2"))
	err := hm.ConfirmHandover("vehicle-001", "operator-2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "handover token issuance failed")
}

func TestConfirmHandover_AuthServiceSuccess_CompletesHandover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hm, registry, sessions := newTestHandoverSetup(t, server.URL)
	vc := registry.Get("vehicle-001")
	vc.SM.TransitionOperator(statemachine.OpActive)
	fromSession := sessions.CreateSession("vehicle-001", "operator-1", "ACTIVE_OPERATOR")

	require.NoError(t, hm.RequestHandover("vehicle-001", "operator-1", "operator-2"))
	require.NoError(t, hm.ConfirmHandover("vehicle-001", "operator-2"))

	stored, ok := sessions.GetSession(fromSession.ID)
	require.True(t, ok)
	assert.Equal(t, "operator-2", stored.OperatorID)
	_, _, _, opState := vc.SM.Get()
	assert.Equal(t, statemachine.OpActive, opState)
}

func TestConfirmHandover_NoAuthURLConfigured_CompletesHandover(t *testing.T) {
	hm, registry, sessions := newTestHandoverSetup(t, "")
	vc := registry.Get("vehicle-001")
	vc.SM.TransitionOperator(statemachine.OpActive)
	sessions.CreateSession("vehicle-001", "operator-1", "ACTIVE_OPERATOR")

	require.NoError(t, hm.RequestHandover("vehicle-001", "operator-1", "operator-2"))
	assert.NoError(t, hm.ConfirmHandover("vehicle-001", "operator-2"))
}
