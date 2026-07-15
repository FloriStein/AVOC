// Multi-Vehicle State Isolation — Integration Tests (ADR-026, Sprint 17).
//
// These tests exercise the real control-server process (not an isolated Go
// type) — they are what would have caught the original Sprint-17 defect: a
// wiring bug in cmd/control-server/main.go where a global statemachine/watchdog
// singleton was used instead of one instance per vehicle. Unit tests on
// vehiclecontext.Registry (MV-01) prove the building block is correct in
// isolation; these prove main.go actually wires it in everywhere.
//
// Written before MV-03/MV-06 exist — GET /vehicles/{id}/state and the
// vehicle_id-scoped emergency-stop are not implemented yet, so this file is
// expected to fail (404 / unscoped behavior) until that work lands.
package integration_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loginAdmin(t *testing.T) string {
	t.Helper()
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body["token"].(string)
}

// connectVehicle registers vehicleID and opens its /vehicle/ws connection —
// required since POST /session/start rejects vehicles that aren't connected
// (ADR-021). Caller must keep the returned connection open for the test's
// duration (defer conn.Close()).
func connectVehicle(t *testing.T, vehicleID string) *websocket.Conn {
	t.Helper()
	resp := postJSON(t, authURL+"/auth/vehicle/register", map[string]string{"username": vehicleID})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/vehicle/ws?token=%s", token))
	require.NoError(t, err, "vehicle %s WS connect failed", vehicleID)
	// The client-side handshake can complete a moment before the server has
	// finished registering the connection — give it a beat (same pattern as
	// services_test.go's WS-then-session/start flow).
	time.Sleep(200 * time.Millisecond)
	return conn
}

func startVehicleSession(t *testing.T, token, vehicleID string) {
	t.Helper()
	resp := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id":    vehicleID,
		"operator_id":   "admin",
		"operator_role": "ACTIVE_OPERATOR",
	})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "session/start for %s must succeed", vehicleID)
	// Clean up at test end — leaving sessions dangling on the shared live
	// server would pollute sessionMgr.GetCurrentSession() for later tests
	// (e.g. TestIntegration_ControlServer_InitialState_IsIDLE).
	t.Cleanup(func() {
		resp := postJSONAuth(t, controlURL+"/session/end", token, nil)
		resp.Body.Close()
	})
}

func vehicleState(t *testing.T, vehicleID string) map[string]any {
	t.Helper()
	return getJSON(t, controlURL+"/vehicles/"+vehicleID+"/state")
}

// 1. Two vehicles, two independent ACTIVE_OPERATOR sessions (same operator,
// different vehicles — vehicleController is keyed per vehicle, not per operator).
// Both must reach CONNECTED independently.
func TestIntegration_MultiVehicle_IndependentSessions_BothConnected(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-connect-1")
	defer conn1.Close()
	conn2 := connectVehicle(t, "vehicle-mv-connect-2")
	defer conn2.Close()
	startVehicleSession(t, token, "vehicle-mv-connect-1")
	startVehicleSession(t, token, "vehicle-mv-connect-2")

	state1 := vehicleState(t, "vehicle-mv-connect-1")
	state2 := vehicleState(t, "vehicle-mv-connect-2")
	assert.Equal(t, "CONNECTED", state1["system"])
	assert.Equal(t, "CONNECTED", state2["system"])
}

// 2. THE CORE FIX, end-to-end: Emergency Stop scoped to vehicle-mv-1 via
// vehicle_id must SAFE_MODE only that vehicle. vehicle-mv-2 must stay CONNECTED.
// This is the scenario that would have caught the original main.go wiring bug.
func TestIntegration_MultiVehicle_ScopedEmergencyStop_OnlyAffectsTargetVehicle(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-estop-1")
	defer conn1.Close()
	conn2 := connectVehicle(t, "vehicle-mv-estop-2")
	defer conn2.Close()
	startVehicleSession(t, token, "vehicle-mv-estop-1")
	startVehicleSession(t, token, "vehicle-mv-estop-2")

	resp := postJSONAuth(t, controlURL+"/emergency-stop", token, map[string]string{
		"vehicle_id": "vehicle-mv-estop-1",
	})
	resp.Body.Close()
	assert.Equal(t, 202, resp.StatusCode)

	state1 := vehicleState(t, "vehicle-mv-estop-1")
	state2 := vehicleState(t, "vehicle-mv-estop-2")
	assert.Equal(t, "SAFE_MODE", state1["system"], "vehicle-mv-estop-1 must be in SAFE_MODE")
	assert.Equal(t, "CONNECTED", state2["system"], "vehicle-mv-estop-2 must be unaffected")
}

// 3. Emergency Stop WITHOUT vehicle_id is the fleet-wide safety net (ADR-026
// consequence "E-Stop ohne vehicle_id wirkt fleet-weit") — every currently
// active vehicle must go to SAFE_MODE, not just one.
func TestIntegration_MultiVehicle_UnscopedEmergencyStop_AffectsAllVehicles(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-fleetstop-1")
	defer conn1.Close()
	conn2 := connectVehicle(t, "vehicle-mv-fleetstop-2")
	defer conn2.Close()
	startVehicleSession(t, token, "vehicle-mv-fleetstop-1")
	startVehicleSession(t, token, "vehicle-mv-fleetstop-2")

	resp := postJSONAuth(t, controlURL+"/emergency-stop", token, map[string]string{})
	resp.Body.Close()
	assert.Equal(t, 202, resp.StatusCode)

	state1 := vehicleState(t, "vehicle-mv-fleetstop-1")
	state2 := vehicleState(t, "vehicle-mv-fleetstop-2")
	assert.Equal(t, "SAFE_MODE", state1["system"], "fleet-wide E-Stop must hit vehicle-mv-fleetstop-1")
	assert.Equal(t, "SAFE_MODE", state2["system"], "fleet-wide E-Stop must hit vehicle-mv-fleetstop-2")
}

// 4. GET /vehicles/{id}/state for a vehicle that was never connected/started
// must not panic and must not return another vehicle's state — fresh IDLE.
func TestIntegration_MultiVehicle_UnknownVehicleState_IsFreshIdle(t *testing.T) {
	state := vehicleState(t, "vehicle-mv-never-seen")
	assert.Equal(t, "IDLE", state["system"])
	assert.Equal(t, "NO_OPERATOR", state["operator"])
}

// 5. POST /session/end WITHOUT session_id (the legacy fleet-wide path) must reset
// EVERY active vehicle to IDLE, not just one resolved via
// sessionMgr.GetCurrentSession(). Found live during manual two-operator testing:
// the old code reset only one vehicle's Machine before EndSession() wiped every
// session's bookkeeping — any other active vehicle was left stranded in its last
// state (observed: SAFE_MODE) with no session left to reset it, recoverable only
// by restarting the process.
func TestIntegration_MultiVehicle_LegacySessionEnd_ResetsAllActiveVehicles(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-legacyend-1")
	defer conn1.Close()
	conn2 := connectVehicle(t, "vehicle-mv-legacyend-2")
	defer conn2.Close()

	resp1 := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": "vehicle-mv-legacyend-1", "operator_id": "admin", "operator_role": "ACTIVE_OPERATOR",
	})
	resp1.Body.Close()
	resp2 := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": "vehicle-mv-legacyend-2", "operator_id": "admin", "operator_role": "ACTIVE_OPERATOR",
	})
	resp2.Body.Close()

	require.Equal(t, "CONNECTED", vehicleState(t, "vehicle-mv-legacyend-1")["system"])
	require.Equal(t, "CONNECTED", vehicleState(t, "vehicle-mv-legacyend-2")["system"])

	// Legacy path: no session_id in the body.
	endResp := postJSONAuth(t, controlURL+"/session/end", token, nil)
	endResp.Body.Close()
	assert.Equal(t, 204, endResp.StatusCode)

	state1 := vehicleState(t, "vehicle-mv-legacyend-1")
	state2 := vehicleState(t, "vehicle-mv-legacyend-2")
	assert.Equal(t, "IDLE", state1["system"], "vehicle-mv-legacyend-1 must be reset to IDLE")
	assert.Equal(t, "IDLE", state2["system"], "vehicle-mv-legacyend-2 must also be reset to IDLE, not stranded")
}

// 6. POST /media/event scoped to vehicle_id — regression test for a bug where
// this endpoint resolved "the current session" via sessionMgr.GetCurrentSession()
// (an arbitrary ACTIVE_OPERATOR session, not necessarily the reporting vehicle's
// own) instead of the vehicle_id in the request. With two vehicles active, a
// MEDIA_FAILED report for vehicle-1 must degrade only vehicle-1.
func TestIntegration_MultiVehicle_MediaEvent_OnlyAffectsTargetVehicle(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-media-1")
	defer conn1.Close()
	conn2 := connectVehicle(t, "vehicle-mv-media-2")
	defer conn2.Close()
	startVehicleSession(t, token, "vehicle-mv-media-1")
	startVehicleSession(t, token, "vehicle-mv-media-2")

	resp := postJSONAuth(t, controlURL+"/media/event", token, map[string]string{
		"vehicle_id": "vehicle-mv-media-1", "state": "MEDIA_FAILED",
	})
	resp.Body.Close()
	assert.Equal(t, 202, resp.StatusCode)

	state1 := vehicleState(t, "vehicle-mv-media-1")
	state2 := vehicleState(t, "vehicle-mv-media-2")
	assert.Equal(t, "DEGRADED", state1["system"], "vehicle-mv-media-1 must degrade on its own MEDIA_FAILED")
	assert.Equal(t, "CONNECTED", state2["system"], "vehicle-mv-media-2 must be unaffected by vehicle-1's media event")
}

// 7. POST /internal/media/auth (MediaMTX's WHEP-read auth hook) scoped to the
// requested vehicle path — regression test for a bug where the "read" branch
// only checked sessionMgr.GetCurrentSession() ("does ANY active session exist
// anywhere"), which would have granted WHEP video-read access to a vehicle the
// caller has no session for at all, once a second vehicle was active elsewhere.
func TestIntegration_MultiVehicle_WHEPAuth_ScopedToVehicleWithSession(t *testing.T) {
	token := loginAdmin(t)
	conn1 := connectVehicle(t, "vehicle-mv-whep-1")
	defer conn1.Close()
	startVehicleSession(t, token, "vehicle-mv-whep-1")
	// vehicle-mv-whep-2 deliberately has no session at all.

	respOwn := postJSON(t, controlURL+"/internal/media/auth", map[string]string{
		"action": "read", "path": "vehicle-mv-whep-1", "token": token,
	})
	defer respOwn.Body.Close()
	assert.Equal(t, 200, respOwn.StatusCode, "WHEP read must be allowed for a vehicle with an active session")

	respOther := postJSON(t, controlURL+"/internal/media/auth", map[string]string{
		"action": "read", "path": "vehicle-mv-whep-2", "token": token,
	})
	defer respOther.Body.Close()
	assert.Equal(t, 401, respOther.StatusCode, "WHEP read must be rejected for a vehicle with no active session")
}
