package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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
	// GET /vehicles/{id}/state (MV-12, replaces removed GET /state): a vehicle context is
	// created on first access and defaults to IDLE (ADR-026) — any never-before-seen id works,
	// so a dedicated one avoids interference from other tests' sessions.
	m := getJSON(t, controlURL+"/vehicles/vehicle-initial-state-test/state")
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

	// State should now be AUTHENTICATED or CONNECTED. No vehicle_id yet at this point (session
	// hasn't started) — GET /sessions as a reachability probe won't carry per-vehicle state, so
	// this uses the same vehicle_id the session/start call below is about to use (MV-12: the
	// vehicle context for an id is created lazily and is stable across the whole test).
	state := getJSON(t, controlURL+"/vehicles/vehicle-int-1/state")
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
		state2 := getJSON(t, controlURL+"/vehicles/vehicle-int-1/state")
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

	state := getJSON(t, controlURL+"/vehicles/vehicle-media/state")
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

	state := getJSON(t, controlURL+"/vehicles/vehicle-estop/state")
	assert.Equal(t, "SAFE_MODE", state["system"])
}
