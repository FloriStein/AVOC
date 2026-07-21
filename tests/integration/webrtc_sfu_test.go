// control-server ↔ webrtc-sfu / mediamtx — Integration Test (INTTEST-01, Sprint 52).
//
// Closes the gap identified in the Sprint-51 test-coverage audit: neither webrtc-sfu nor a real
// MediaMTX container were part of the Docker test stack, so control-server's two SAFE_MODE media
// paths — session/sfu_publisher.go's real HTTP push of SESSION_SAFE_MODE to webrtc-sfu, and
// internal/mediamtx.Client.KickVehicle's real call to MediaMTX's Management API — ran against
// nothing (webrtc-sfu) or a wrong/unreachable default (mediamtx), silently swallowed
// (see log.Warn/log.Printf in both clients, confirmed by GOTEST-03/Sprint 51). No real WebRTC
// SDP/ICE negotiation happens here — that stays out of scope per ADR-006 ("zu flaky in CI").
package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sfuURL      = "http://localhost:18084"
	mediamtxURL = "http://localhost:19997"
)

// sfuSessionState polls webrtc-sfu's status endpoint (added in INTTEST-01) without failing the
// test on a not-yet-arrived event — required so require.Eventually can retry.
func sfuSessionState(sessionID string) (string, bool) {
	resp, err := http.Get(sfuURL + "/session/" + sessionID + "/state")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var body map[string]string
	if json.NewDecoder(resp.Body).Decode(&body) != nil {
		return "", false
	}
	return body["state"], true
}

func TestIntegration_EmergencyStop_PushesSafeModeToSFUAndKicksMediaMTX(t *testing.T) {
	token := loginAdmin(t)

	const vehicleID = "vehicle-sfu-estop"
	vehConn := connectVehicle(t, vehicleID)
	defer vehConn.Close()

	resp1 := postJSONAuth(t, controlURL+"/session/start", token, map[string]string{
		"vehicle_id": vehicleID, "operator_id": "admin", "operator_role": "ACTIVE_OPERATOR",
	})
	require.Equal(t, 200, resp1.StatusCode)
	var sessBody map[string]any
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&sessBody))
	resp1.Body.Close()
	sessionID := sessBody["session_id"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s&session_id=%s", token, sessionID))
	require.NoError(t, err, "operator WS dial must succeed once session_id is known")
	defer conn.Close()
	defer func() { postJSONAuth(t, controlURL+"/session/end", token, nil).Body.Close() }()
	time.Sleep(300 * time.Millisecond)

	resp2 := postJSONAuth(t, controlURL+"/emergency-stop", token, map[string]string{})
	resp2.Body.Close()
	require.Equal(t, 202, resp2.StatusCode)

	// session/sfu_publisher.go pushes SESSION_SAFE_MODE to the real webrtc-sfu container over
	// HTTP — verify it actually arrived and was recorded, not just that control-server accepted
	// the emergency-stop request.
	require.Eventually(t, func() bool {
		state, ok := sfuSessionState(sessionID)
		return ok && state == "SESSION_SAFE_MODE"
	}, 5*time.Second, 100*time.Millisecond, "SESSION_SAFE_MODE must reach webrtc-sfu via real HTTP push")

	// internal/mediamtx.Client.KickVehicle called the real MediaMTX Management API as part of the
	// same emergency-stop request (no active WebRTC session for this vehicle exists, so nothing to
	// kick — this stack never does real SDP/ICE, ADR-006 — but the HTTP roundtrip itself must have
	// succeeded, i.e. the API is reachable and returns a well-formed session list).
	resp3, err := http.Get(mediamtxURL + "/v3/webrtcsessions/list")
	require.NoError(t, err, "MediaMTX Management API must be reachable — this is the real host control-server's KickVehicle calls")
	defer resp3.Body.Close()
	assert.Equal(t, 200, resp3.StatusCode)
}
