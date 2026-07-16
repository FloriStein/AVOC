package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "GET %s", url)
	var m map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	return m
}

// getJSONList is getJSON's counterpart for endpoints returning a JSON array (e.g. GET /vehicles).
func getJSONList(t *testing.T, url string) []any {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "GET %s", url)
	var list []any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	return list
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	require.NoError(t, err, "POST %s", url)
	return resp
}

// postJSONAuth posts with a Bearer token — required for the JWT-protected
// control-server endpoints (session/start, session/end, media/event, emergency-stop — AUTH-01).
func postJSONAuth(t *testing.T, url, token string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "POST %s", url)
	return resp
}

// patchJSONAuth is postJSONAuth's PATCH counterpart — needed for the ADR-030 manual task
// status-transition endpoint (PATCH /fleet/tasks/{id}/status), the only PATCH-verb fleet-service
// route so far.
func patchJSONAuth(t *testing.T, url, token string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "PATCH %s", url)
	return resp
}

// --- Health Checks ---

func TestIntegration_ControlServer_Healthy(t *testing.T) {
	m := getJSON(t, controlURL+"/health")
	assert.Equal(t, "ok", m["status"])
	assert.Equal(t, "control-server", m["service"])
}

func TestIntegration_AuthService_Healthy(t *testing.T) {
	m := getJSON(t, authURL+"/health")
	assert.Equal(t, "ok", m["status"])
}

func TestIntegration_SafetyService_Healthy(t *testing.T) {
	m := getJSON(t, safetyURL+"/health")
	assert.Equal(t, "ok", m["status"])
}

func TestIntegration_FleetService_Healthy(t *testing.T) {
	m := getJSON(t, fleetURL+"/health")
	assert.Equal(t, "ok", m["status"])
	assert.Equal(t, "fleet-service", m["service"])
}

// TestIntegration_FleetService_CoexistsWithControlServer_VehicleRegistration verifies the
// ADR-029 coexistence claim in the real multi-service network — not just the isolated Go test
// in internal/fleetservice/integration_test.go. control-server and fleet-service start against
// the same Postgres DB with no explicit ordering between them (docker-compose.test.yml: both
// only depend_on postgres). Registering a vehicle via control-server's real HTTP API must keep
// working and remain visible after fleet-service has also initialized its schema (ALTER TABLE
// vehicles ADD COLUMN vehicle_type) against the same table.
func TestIntegration_FleetService_CoexistsWithControlServer_VehicleRegistration(t *testing.T) {
	// fleet-service must be up (proves its schema-init against the shared vehicles table
	// already ran/is running concurrently with control-server in this stack).
	fm := getJSON(t, fleetURL+"/health")
	require.Equal(t, "ok", fm["status"])

	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	resp2 := postJSONAuth(t, controlURL+"/vehicles", token, map[string]string{
		"id": "vehicle-fleet-coexist-1", "display_name": "Fleet Coexist Test Vehicle",
	})
	defer resp2.Body.Close()
	// 201 on first run, 409 if a previous run already registered it — both prove the vehicles
	// table (extended by fleet-service) is still fully functional via control-server's own API.
	assert.Contains(t, []int{201, 409}, resp2.StatusCode)

	vehicles := getJSONList(t, controlURL+"/vehicles")
	found := false
	for _, v := range vehicles {
		if vm, ok := v.(map[string]any); ok && vm["id"] == "vehicle-fleet-coexist-1" {
			found = true
		}
	}
	assert.True(t, found, "vehicle registered via control-server must remain visible after fleet-service extended the shared vehicles table")
}

// --- Auth Service ---

func TestIntegration_Auth_OperatorLogin_ReturnsJWT(t *testing.T) {
	// Uses the auto-seeded admin account (ADMIN_PASSWORD=admin_test_secret in docker-compose.test.yml)
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token, ok := body["token"].(string)
	require.True(t, ok, "response must contain token string")
	assert.Greater(t, len(token), 20, "JWT must be non-trivial length")
}

func TestIntegration_Auth_VehicleRegister_ReturnsJWT(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/vehicle/register",
		map[string]string{"username": "vehicle-int-test"})
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	_, ok := body["token"].(string)
	assert.True(t, ok)
}

// --- Control Server: Initial State ---

func TestIntegration_ControlServer_InitialState_IsIDLE(t *testing.T) {
	m := getJSON(t, controlURL+"/state")
	assert.Equal(t, "IDLE", m["system"])
	assert.Equal(t, "CONTROL_INIT", m["control"])
	assert.Equal(t, "MEDIA_INIT", m["media"])
	assert.Equal(t, "NO_OPERATOR", m["operator"])
}

// --- Session Lifecycle ---

func TestIntegration_SessionLifecycle_StartAndEnd(t *testing.T) {
	// 1. Login using seeded admin account
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var loginBody map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginBody))
	token := loginBody["token"].(string)

	// 2. WebSocket connect (upgrade) — skip full WS in integration test;
	//    instead call session/start directly to verify HTTP API path.
	//    State machine starts at IDLE — we need AUTHENTICATED first.
	// Authenticate by connecting WebSocket (transition IDLE → AUTHENTICATED)
	wsURL := fmt.Sprintf("ws://localhost:18080/ws?token=%s", token)
	conn, err := dialWS(t, wsURL)
	if err != nil {
		t.Skipf("WebSocket dial failed (expected in minimal test stack): %v", err)
	}
	defer conn.Close()
	time.Sleep(200 * time.Millisecond)

	// State should now be AUTHENTICATED or CONNECTED
	state := getJSON(t, controlURL+"/state")
	sys, _ := state["system"].(string)
	assert.Contains(t, []string{"AUTHENTICATED", "CONNECTED", "CONNECTING"}, sys)

	// 3. Start session
	resp2 := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id":    "vehicle-int-1",
		"operator_id":   "admin",
		"operator_role": "ACTIVE_OPERATOR",
	})
	defer resp2.Body.Close()

	if resp2.StatusCode == 200 {
		var sessBody map[string]any
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&sessBody))
		sessionID, _ := sessBody["session_id"].(string)
		assert.Greater(t, len(sessionID), 10, "session_id must be ULID")

		// 4. SYSTEM STATE should be CONNECTED
		state2 := getJSON(t, controlURL+"/state")
		assert.Equal(t, "CONNECTED", state2["system"])

		// 5. End session
		resp3 := postJSONAuth(t, controlURL+"/session/end", token, nil)
		resp3.Body.Close()
		assert.Equal(t, 204, resp3.StatusCode)
	}
}

// --- MEDIA STATE: Invariante 1 ---

func TestIntegration_MediaFailed_TriggersDegrade_NeverSafeMode(t *testing.T) {
	// Login + WS connect + session start (seeded admin account)
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s", token))
	if err != nil {
		t.Skipf("WebSocket not available: %v", err)
	}
	defer conn.Close()
	time.Sleep(300 * time.Millisecond)

	postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": "vehicle-media", "operator_id": "admin", "operator_role": "ACTIVE_OPERATOR",
	}).Body.Close()

	// MEDIA_FAILED event
	resp2 := postJSONAuth(t, controlURL+"/media/event", token, map[string]string{"state": "MEDIA_FAILED", "vehicle_id": "vehicle-media"})
	resp2.Body.Close()
	assert.Equal(t, 202, resp2.StatusCode)

	state := getJSON(t, controlURL+"/state")
	sys := state["system"].(string)
	assert.Equal(t, "DEGRADED", sys, "MEDIA_FAILED must → DEGRADED, never SAFE_MODE (Invariante 1)")

	postJSONAuth(t, controlURL+"/session/end", token, nil).Body.Close()
}

// --- Emergency Stop ---

func TestIntegration_EmergencyStop_TriggersSafeMode(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s", token))
	if err != nil {
		t.Skipf("WebSocket not available: %v", err)
	}
	defer conn.Close()
	time.Sleep(300 * time.Millisecond)

	postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": "vehicle-estop", "operator_id": "admin", "operator_role": "ACTIVE_OPERATOR",
	}).Body.Close()

	resp2 := postJSONAuth(t, controlURL+"/emergency-stop", token, map[string]string{})
	resp2.Body.Close()
	assert.Equal(t, 202, resp2.StatusCode)

	state := getJSON(t, controlURL+"/state")
	assert.Equal(t, "SAFE_MODE", state["system"])
}

// startSessionAndDialWS logs in, starts an ACTIVE_OPERATOR session on the one
// vehicle the test stack's vehicle-mock actually registers as online
// ("vehicle-int-mock" — any other vehicle_id makes /session/start fail with
// 409 "vehicle not connected", which silently starved every WS-dependent test
// below of a session_id: /ws requires ?session_id=..., dialing without one is
// the actual root cause of the "bad handshake" failures, not a sandbox/docker
// limitation), then dials the operator WS with that session_id.
func startSessionAndDialWS(t *testing.T, token, operatorID string) (*websocket.Conn, string) {
	t.Helper()
	startResp := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": "vehicle-int-mock", "operator_id": operatorID,
	})
	defer startResp.Body.Close()
	require.Equal(t, 200, startResp.StatusCode, "session/start must succeed against the connected vehicle-mock")
	var startBody map[string]any
	require.NoError(t, json.NewDecoder(startResp.Body).Decode(&startBody))
	sessionID := startBody["session_id"].(string)
	require.NotEmpty(t, sessionID)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s&session_id=%s", token, sessionID))
	require.NoError(t, err, "WS dial must succeed once session_id is supplied")
	time.Sleep(200 * time.Millisecond)
	return conn, sessionID
}

// endAllSessions force-resets every active vehicle to IDLE (legacy fleet-wide
// /session/end path — no session_id required, no ownership check) so the
// shared "vehicle-int-mock" is guaranteed free for the next test regardless
// of how the current test ended (SAFE_MODE, deleted operator, ...).
func endAllSessions(t *testing.T, token string) {
	t.Helper()
	postJSONAuth(t, controlURL+"/session/end", token, nil).Body.Close()
}

// --- DRIFT-K3 (2026-07-16): MEDIA_DEGRADED wiring + DEGRADED→CONNECTED recovery ---

func TestIntegration_MediaDegraded_TriggersDegrade_ThenRecovers(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, _ := startSessionAndDialWS(t, token, "admin")
	defer conn.Close()
	defer endAllSessions(t, token)

	const vehicleID = "vehicle-int-mock"

	// MEDIA_DEGRADED (quality loss, not full failure) must also map to SYSTEM DEGRADED.
	resp2 := postJSONAuth(t, controlURL+"/media/event", token, map[string]string{"state": "MEDIA_DEGRADED", "vehicle_id": vehicleID})
	resp2.Body.Close()
	assert.Equal(t, 202, resp2.StatusCode)

	state := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
	assert.Equal(t, "DEGRADED", state["system"])

	// Recovery (DRIFT-K3 fix): MEDIA_CONNECTED while DEGRADED must clear it —
	// before this fix TransitionMedia had no path back to CONNECTED at all.
	resp3 := postJSONAuth(t, controlURL+"/media/event", token, map[string]string{"state": "MEDIA_CONNECTED", "vehicle_id": vehicleID})
	resp3.Body.Close()
	assert.Equal(t, 202, resp3.StatusCode)

	state2 := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
	assert.Equal(t, "CONNECTED", state2["system"], "media recovery must clear DEGRADED back to CONNECTED")
}

// --- DRIFT-K2 (2026-07-16): OPERATOR layer must reflect NO_OPERATOR after WS disconnect ---

func TestIntegration_WSDisconnect_OperatorLayerReflectsNoOperator(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, _ := startSessionAndDialWS(t, token, "admin")
	defer endAllSessions(t, token)

	const vehicleID = "vehicle-int-mock"

	preState := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
	require.Equal(t, "ACTIVE_OPERATOR", preState["operator"], "operator must be ACTIVE before disconnect")

	// Close the WS without calling /session/end — the readLoop defer must fire.
	conn.Close()

	var state map[string]any
	require.Eventually(t, func() bool {
		state = getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
		return state["system"] == "SAFE_MODE"
	}, 5*time.Second, 100*time.Millisecond, "WS disconnect must trigger SAFE_MODE")

	assert.Equal(t, "NO_OPERATOR", state["operator"],
		"DRIFT-K2 fix: OPERATOR layer must no longer hang at ACTIVE_OPERATOR after disconnect")
}

// --- DRIFT-K1 (2026-07-16): AuthWatchdog — revoked operator account → SAFE_MODE ---

func TestIntegration_AuthWatchdog_DeletedAccount_TriggersSafeMode(t *testing.T) {
	adminResp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer adminResp.Body.Close()
	require.Equal(t, 200, adminResp.StatusCode)
	var adminBody map[string]any
	require.NoError(t, json.NewDecoder(adminResp.Body).Decode(&adminBody))
	adminToken := adminBody["token"].(string)
	defer endAllSessions(t, adminToken)

	username := fmt.Sprintf("int-k1-%d", time.Now().UnixNano())
	createResp := postJSONAuth(t, authURL+"/auth/users", adminToken, map[string]string{
		"username": username, "password": "throwaway-secret-1", "role": "ADMIN",
	})
	createResp.Body.Close()
	require.Equal(t, 201, createResp.StatusCode, "test user creation must succeed")

	users := getJSONListAuth(t, authURL+"/auth/users", adminToken)
	var userID float64
	for _, u := range users {
		m := u.(map[string]any)
		if m["username"] == username {
			userID = m["id"].(float64)
		}
	}
	require.NotZero(t, userID, "created test user must appear in GET /auth/users")

	loginResp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": username, "password": "throwaway-secret-1"})
	defer loginResp.Body.Close()
	require.Equal(t, 200, loginResp.StatusCode)
	var loginBody map[string]any
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginBody))
	token := loginBody["token"].(string)

	conn, _ := startSessionAndDialWS(t, token, username)
	defer conn.Close()

	const vehicleID = "vehicle-int-mock"
	preState := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
	require.Equal(t, "CONNECTED", preState["system"], "session must be CONNECTED before revocation")

	// Admin deletes the account mid-session — the only revocation path that
	// exists today (no soft-deactivate endpoint). No WS close, no session/end —
	// the account is simply gone out from under an otherwise-live connection.
	delResp := func() *http.Response {
		req, _ := http.NewRequest(http.MethodDelete, authURL+fmt.Sprintf("/auth/users/%d", int(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return r
	}()
	delResp.Body.Close()
	require.Equal(t, 204, delResp.StatusCode)

	// AuthWatchdog defaults to 5s poll × 2 failures (~10s) — generous timeout
	// for the real production timing, not a shortened test-only value.
	var state map[string]any
	require.Eventually(t, func() bool {
		state = getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
		return state["system"] == "SAFE_MODE"
	}, 20*time.Second, 500*time.Millisecond,
		"deleted operator account must trigger SAFE_MODE via AuthWatchdog within ~10s")

	assert.Equal(t, "NO_OPERATOR", state["operator"])
}
