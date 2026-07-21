package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/mediamtx"
	"avoc/internal/recording"
	"avoc/internal/vehicleconnection"
	"avoc/pkg/audit"
	"avoc/tests/unit/mocks"
)

const testSecret = "control-server-test-secret-32ch"

// emergencyStopCall is what HTTPPublisher.TriggerEmergencyStop posts to /safety/emergency-stop.
type emergencyStopCall struct {
	SessionID string `json:"session_id"`
	VehicleID string `json:"vehicle_id"`
	Reason    string `json:"reason"`
}

// safetyEventRecorder is a local httptest.Server handler standing in for safety-service — it
// records every call HTTPPublisher makes (POST /safety/emergency-stop, POST /safety/event)
// instead of exercising the real Bus (already covered by tests/unit/safety_bus_integration_test.go
// and internal/safetyservice/bus_test.go — this fixture only needs to prove main.go's handlers
// called TriggerEmergencyStop/PublishEvent with the right arguments).
type safetyEventRecorder struct {
	mu             sync.Mutex
	emergencyStops []emergencyStopCall
	events         []json.RawMessage
}

func newSafetyEventRecorder() *safetyEventRecorder {
	return &safetyEventRecorder{}
}

func (r *safetyEventRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	switch req.URL.Path {
	case "/safety/emergency-stop":
		var call emergencyStopCall
		_ = json.Unmarshal(body, &call)
		r.emergencyStops = append(r.emergencyStops, call)
	case "/safety/event":
		r.events = append(r.events, json.RawMessage(body))
	}
	r.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
}

func (r *safetyEventRecorder) EmergencyStops() []emergencyStopCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]emergencyStopCall, len(r.emergencyStops))
	copy(out, r.emergencyStops)
	return out
}

// newTestControlServer builds a real *controlServer from hand-constructed dependencies — no
// Postgres connection needed (vehicleStore/authcheck aren't exercised by the handlers covered in
// this sprint, see tasks/current-sprint.md Sprint 46 scope). safetyPub points at a local
// httptest.Server (rec) instead of a real safety-service, mirroring
// tests/unit/safety_bus_integration_test.go's established pattern — no production code changes
// needed since HTTPPublisher already takes an arbitrary baseURL.
func newTestControlServer(t *testing.T) (*controlServer, *safetyEventRecorder) {
	t.Helper()

	rec := newSafetyEventRecorder()
	safetyServer := httptest.NewServer(rec)
	t.Cleanup(safetyServer.Close)

	safetyPub := csafety.NewHTTPPublisher(safetyServer.URL)
	sessionMgr := session.NewManager(&mocks.MockSFUPublisher{})
	auditWriter := audit.NewNoopWriter()

	vehicleContexts := vehiclecontext.NewRegistry(
		csafety.DefaultDeadmanTimeout, csafety.DefaultACKTimeout, csafety.DefaultVehicleACKTimeout, safetyPub,
	).WithAuditWriter(auditWriter)

	handoverMgr := session.NewHandoverManager(vehicleContexts, sessionMgr, "")
	recorder := recording.NewMemoryRecorder()
	vehicleRegistry := vehicleconnection.NewRegistry()
	vehicleAckStore := vehicleconnection.NewAckStore()

	cmdEngine := command.NewEngine(vehicleContexts, safetyPub, sessionMgr).
		WithAuditWriter(auditWriter).
		WithVehicleForwarder(vehicleRegistry)

	s := &controlServer{
		cfg:             serverConfig{secret: testSecret},
		auditWriter:     auditWriter,
		safetyPub:       safetyPub,
		sessionMgr:      sessionMgr,
		vehicleContexts: vehicleContexts,
		handoverMgr:     handoverMgr,
		recorder:        recorder,
		vehicleRegistry: vehicleRegistry,
		vehicleAckStore: vehicleAckStore,
		cmdEngine:       cmdEngine,
		mtxClient:       mediamtx.NewClient(""),
	}
	s.buildHandlers()
	return s, rec
}

func TestHealth_ReturnsOK(t *testing.T) {
	s, _ := newTestControlServer(t)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ─── handleSessionStart / advanceVehicleToActiveOperator ───────────────────────

func sessionStartRequest(vehicleID, operatorID, role string) *http.Request {
	body, _ := json.Marshal(map[string]string{"vehicle_id": vehicleID, "operator_id": operatorID})
	r := httptest.NewRequest(http.MethodPost, "/session/start", bytes.NewReader(body))
	if role != "" {
		r = r.WithContext(context.WithValue(r.Context(), roleKey, role))
	}
	return r
}

func TestSessionStart_VehicleNotConnected_Returns409(t *testing.T) {
	s, _ := newTestControlServer(t)
	rr := httptest.NewRecorder()

	s.handleSessionStart(rr, sessionStartRequest("v1", "op1", "ACTIVE_OPERATOR"))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSessionStart_ObserverOnFreeVehicle_Returns403(t *testing.T) {
	s, _ := newTestControlServer(t)
	s.vehicleRegistry.RegisterForTest("v1", nil)
	rr := httptest.NewRecorder()

	s.handleSessionStart(rr, sessionStartRequest("v1", "op1", "OBSERVER"))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestSessionStart_ActiveOperator_AdvancesStateMachineToConnected(t *testing.T) {
	s, _ := newTestControlServer(t)
	s.vehicleRegistry.RegisterForTest("v1", nil)
	rr := httptest.NewRecorder()

	s.handleSessionStart(rr, sessionStartRequest("v1", "op1", "ACTIVE_OPERATOR"))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		SessionID string `json:"session_id"`
		Role      string `json:"role"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != "ACTIVE_OPERATOR" {
		t.Fatalf("expected ACTIVE_OPERATOR role, got %q", resp.Role)
	}

	vc := s.vehicleContexts.Get("v1")
	sys, _, _, op := vc.SM.Get()
	if sys != statemachine.StateConnected {
		t.Fatalf("expected SYSTEM CONNECTED, got %s", sys)
	}
	if op != statemachine.OpActive {
		t.Fatalf("expected OPERATOR ACTIVE, got %s", op)
	}
}

func TestSessionStart_ObserverJoinsLockedVehicle_DoesNotReadvanceStateMachine(t *testing.T) {
	s, _ := newTestControlServer(t)
	s.vehicleRegistry.RegisterForTest("v1", nil)

	// First request claims ACTIVE_OPERATOR and advances the state machine to CONNECTED.
	rr1 := httptest.NewRecorder()
	s.handleSessionStart(rr1, sessionStartRequest("v1", "op1", "ACTIVE_OPERATOR"))
	if rr1.Code != http.StatusOK {
		t.Fatalf("setup: expected 200, got %d", rr1.Code)
	}

	// Second operator joins the now-locked vehicle as OBSERVER.
	rr2 := httptest.NewRecorder()
	s.handleSessionStart(rr2, sessionStartRequest("v1", "op2", "OBSERVER"))
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var resp struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != "OBSERVER" {
		t.Fatalf("expected OBSERVER role, got %q", resp.Role)
	}

	vc := s.vehicleContexts.Get("v1")
	sys, _, _, _ := vc.SM.Get()
	if sys != statemachine.StateConnected {
		t.Fatalf("expected SYSTEM to remain CONNECTED, got %s", sys)
	}
}

// ─── handleSessionEnd ────────────────────────────────────────────────────────

func sessionEndRequest(sessionID string) *http.Request {
	var body []byte
	if sessionID != "" {
		body, _ = json.Marshal(map[string]string{"session_id": sessionID})
	}
	return httptest.NewRequest(http.MethodPost, "/session/end", bytes.NewReader(body))
}

// startActiveOperatorSession is a small test-only helper duplicating handleSessionStart's
// happy path, so handleSessionEnd tests don't have to route through the HTTP handler just to
// reach a starting state — this file's own handleSessionStart tests already cover that path.
func startActiveOperatorSession(t *testing.T, s *controlServer, vehicleID, operatorID string) string {
	t.Helper()
	s.vehicleRegistry.RegisterForTest(vehicleID, nil)
	rr := httptest.NewRecorder()
	s.handleSessionStart(rr, sessionStartRequest(vehicleID, operatorID, "ACTIVE_OPERATOR"))
	if rr.Code != http.StatusOK {
		t.Fatalf("setup: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("setup: decode response: %v", err)
	}
	return resp.SessionID
}

func TestSessionEnd_ByID_ActiveOperator_ResetsVehicleToIdle(t *testing.T) {
	s, _ := newTestControlServer(t)
	sessionID := startActiveOperatorSession(t, s, "v1", "op1")

	rr := httptest.NewRecorder()
	s.handleSessionEnd(rr, sessionEndRequest(sessionID))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	vc := s.vehicleContexts.Get("v1")
	sys, _, _, _ := vc.SM.Get()
	if sys != statemachine.StateIdle {
		t.Fatalf("expected SYSTEM IDLE, got %s", sys)
	}
	if _, ok := s.sessionMgr.GetSession(sessionID); ok {
		t.Fatal("expected session to be released")
	}
}

func TestSessionEnd_ByID_Observer_DoesNotResetVehicle(t *testing.T) {
	s, _ := newTestControlServer(t)
	_ = startActiveOperatorSession(t, s, "v1", "op1")

	rr1 := httptest.NewRecorder()
	s.handleSessionStart(rr1, sessionStartRequest("v1", "op2", "OBSERVER"))
	var resp struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(rr1.Body.Bytes(), &resp)

	rr2 := httptest.NewRecorder()
	s.handleSessionEnd(rr2, sessionEndRequest(resp.SessionID))

	if rr2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr2.Code)
	}
	vc := s.vehicleContexts.Get("v1")
	sys, _, _, _ := vc.SM.Get()
	if sys != statemachine.StateConnected {
		t.Fatalf("expected SYSTEM to remain CONNECTED (ACTIVE_OPERATOR session still live), got %s", sys)
	}
	if _, ok := s.sessionMgr.GetSession(resp.SessionID); ok {
		t.Fatal("expected observer session to be released")
	}
}

func TestSessionEnd_UnknownSessionID_NoOpReturns204(t *testing.T) {
	s, _ := newTestControlServer(t)
	rr := httptest.NewRecorder()

	s.handleSessionEnd(rr, sessionEndRequest("does-not-exist"))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestSessionEnd_LegacyPath_NoSessionID_ResetsAllActiveVehicles(t *testing.T) {
	s, _ := newTestControlServer(t)
	startActiveOperatorSession(t, s, "v1", "op1")
	startActiveOperatorSession(t, s, "v2", "op2")

	rr := httptest.NewRecorder()
	s.handleSessionEnd(rr, sessionEndRequest(""))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	for _, vehicleID := range []string{"v1", "v2"} {
		vc := s.vehicleContexts.Get(vehicleID)
		sys, _, _, _ := vc.SM.Get()
		if sys != statemachine.StateIdle {
			t.Fatalf("expected %s SYSTEM IDLE, got %s", vehicleID, sys)
		}
	}
	if len(s.sessionMgr.ActiveVehicleIDs()) != 0 {
		t.Fatalf("expected no active vehicles left, got %v", s.sessionMgr.ActiveVehicleIDs())
	}
}

// ─── handleEmergencyStop ────────────────────────────────────────────────────

func emergencyStopRequest(vehicleID string) *http.Request {
	var body []byte
	if vehicleID != "" {
		body, _ = json.Marshal(map[string]string{"vehicle_id": vehicleID})
	}
	return httptest.NewRequest(http.MethodPost, "/emergency-stop", bytes.NewReader(body))
}

// waitForEmergencyStops polls rec until it has recorded want calls or the timeout expires —
// handleEmergencyStop's HTTPPublisher call is synchronous, but the recorder's HTTP round trip to
// the local httptest.Server is not instantaneous relative to the test goroutine.
func waitForEmergencyStops(t *testing.T, rec *safetyEventRecorder, want int, timeout time.Duration) []emergencyStopCall {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		calls := rec.EmergencyStops()
		if len(calls) >= want {
			return calls
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d emergency-stop call(s), got %d", want, len(calls))
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestEmergencyStop_ScopedToOneVehicle_TriggersSafeModeAndCallsPublisher(t *testing.T) {
	s, rec := newTestControlServer(t)
	sessionID := startActiveOperatorSession(t, s, "v1", "op1")

	rr := httptest.NewRecorder()
	s.handleEmergencyStop(rr, emergencyStopRequest("v1"))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	vc := s.vehicleContexts.Get("v1")
	sys, _, _, _ := vc.SM.Get()
	if sys != statemachine.StateSafeMode {
		t.Fatalf("expected SYSTEM SAFE_MODE, got %s", sys)
	}
	calls := waitForEmergencyStops(t, rec, 1, time.Second)
	if calls[0].SessionID != sessionID || calls[0].VehicleID != "v1" {
		t.Fatalf("unexpected emergency-stop call: %+v (want session %s, vehicle v1)", calls[0], sessionID)
	}
}

func TestEmergencyStop_FleetWide_NoVehicleID_StopsEveryActiveVehicle(t *testing.T) {
	s, rec := newTestControlServer(t)
	startActiveOperatorSession(t, s, "v1", "op1")
	startActiveOperatorSession(t, s, "v2", "op2")

	rr := httptest.NewRecorder()
	s.handleEmergencyStop(rr, emergencyStopRequest(""))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	for _, vehicleID := range []string{"v1", "v2"} {
		vc := s.vehicleContexts.Get(vehicleID)
		sys, _, _, _ := vc.SM.Get()
		if sys != statemachine.StateSafeMode {
			t.Fatalf("expected %s SYSTEM SAFE_MODE, got %s", vehicleID, sys)
		}
	}
	calls := waitForEmergencyStops(t, rec, 2, time.Second)
	gotVehicles := map[string]bool{calls[0].VehicleID: true, calls[1].VehicleID: true}
	if !gotVehicles["v1"] || !gotVehicles["v2"] {
		t.Fatalf("expected emergency-stop calls for both v1 and v2, got %+v", calls)
	}
}

// TestEmergencyStop_VehicleWithoutActiveSession_TransitionSilentlyRejected documents a real
// finding from this sprint's coverage build-out (ADR-035): a vehicle with no active session sits
// in SYSTEM=IDLE, and validSystemTransitions (internal/controlserver/statemachine/state.go) does
// NOT allow IDLE→SAFE_MODE directly (only IDLE→CONNECTING) — so TransitionSystem(StateSafeMode)
// is silently rejected (logged as "invalid state transition rejected", state stays IDLE).
// handleEmergencyStop still calls TriggerEmergencyStop (safety-service is notified) and still
// returns 202, but the vehicle's OWN state machine never actually reaches SAFE_MODE. This is
// pre-existing production behavior, not something introduced by this sprint — flagged in the new
// ADR as a candidate follow-up, no production code changed here (out of scope, see sprint scope).
func TestEmergencyStop_VehicleWithoutActiveSession_TransitionSilentlyRejected(t *testing.T) {
	s, rec := newTestControlServer(t)
	// No session started for v3 at all — vehicleContexts.Get creates a fresh IDLE context lazily.

	rr := httptest.NewRecorder()
	s.handleEmergencyStop(rr, emergencyStopRequest("v3"))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	vc := s.vehicleContexts.Get("v3")
	sys, _, _, _ := vc.SM.Get()
	if sys != statemachine.StateIdle {
		t.Fatalf("expected SYSTEM to stay IDLE (IDLE→SAFE_MODE is not a valid transition), got %s", sys)
	}
	// sess.ID == "" for a vehicle with no session — TriggerEmergencyStop is still called
	// (unconditionally, per handleEmergencyStop), but with an empty session_id.
	calls := waitForEmergencyStops(t, rec, 1, time.Second)
	if calls[0].SessionID != "" || calls[0].VehicleID != "v3" {
		t.Fatalf("unexpected emergency-stop call: %+v", calls[0])
	}
}

func mintTestToken(t *testing.T, secret, subject, role string, ttl time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": subject,
		"exp": time.Now().Add(ttl).Unix(),
	}
	if role != "" {
		claims["role"] = role
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return s
}

func serveWithMiddleware(mw func(http.HandlerFunc) http.HandlerFunc, authHeader string) *httptest.ResponseRecorder {
	var gotSubject, gotRole string
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		gotSubject, _ = r.Context().Value(claimsKey).(string)
		gotRole, _ = r.Context().Value(roleKey).(string)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/protected", nil)
	if authHeader != "" {
		r.Header.Set("Authorization", authHeader)
	}
	rr := httptest.NewRecorder()
	handler(rr, r)
	// Stash results on the recorder's header for the caller to inspect without a second return
	// value — keeps this helper's signature simple for the common case (status-code-only checks).
	rr.Header().Set("X-Test-Subject", gotSubject)
	rr.Header().Set("X-Test-Role", gotRole)
	return rr
}

func TestRequireJWT_MissingHeader_Returns401(t *testing.T) {
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireJWT_MalformedHeader_Returns401(t *testing.T) {
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "NotBearer sometoken")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireJWT_MalformedToken_Returns401(t *testing.T) {
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "Bearer not-a-real-jwt")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireJWT_WrongSecret_Returns401(t *testing.T) {
	token := mintTestToken(t, "a-different-secret-entirely!!!!", "op1", "ACTIVE_OPERATOR", time.Hour)
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "Bearer "+token)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireJWT_ExpiredToken_Returns401(t *testing.T) {
	token := mintTestToken(t, testSecret, "op1", "ACTIVE_OPERATOR", -time.Hour)
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "Bearer "+token)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireJWT_ValidToken_PassesSubjectAndRoleThrough(t *testing.T) {
	token := mintTestToken(t, testSecret, "op1", "OBSERVER", time.Hour)
	rr := serveWithMiddleware(requireJWT([]byte(testSecret)), "Bearer "+token)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-Test-Subject"); got != "op1" {
		t.Fatalf("expected subject op1, got %q", got)
	}
	if got := rr.Header().Get("X-Test-Role"); got != "OBSERVER" {
		t.Fatalf("expected role OBSERVER, got %q", got)
	}
}
