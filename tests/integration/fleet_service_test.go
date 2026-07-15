package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginAdmin is defined in multivehicle_test.go — reused here against fleet-service since both
// services share the same JWT_SECRET (ADR-004).

func getJSONListAuth(t *testing.T, url, token string) []any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "GET %s", url)
	var list []any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	return list
}

// TestIntegration_FleetService_RequiresAuth verifies /fleet/* is not the historically-open
// GET /vehicles control-server has (ADR-014-era) — Fleet data is operator-facing and gated.
func TestIntegration_FleetService_RequiresAuth(t *testing.T) {
	resp, err := http.Get(fleetURL + "/fleet/vehicles")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)
}

// TestIntegration_FleetService_ConsumesRealMQTTStatus_AcrossProcessBoundary is the true end-to-end
// proof for FLEET-05: vehicle-mock (a separate container, FLEET-04) publishes fleet vehicle status
// over the real Mosquitto broker, and fleet-service (also a separate container) must consume it via
// MQTTGateway, auto-register the vehicle identity (EnsureVehicleExists — a real bug found via
// manual E2E verification against the dev stack, fixed in FLEET-05), and expose it through the REST
// API. internal/fleetservice's own tests cover the store logic in isolation; this is the only test
// that exercises the full real chain: vehicle-mock -> Mosquitto -> fleet-service -> Postgres -> REST.
func TestIntegration_FleetService_ConsumesRealMQTTStatus_AcrossProcessBoundary(t *testing.T) {
	token := loginAdmin(t)

	wantVehicles := map[string]bool{"test-lastenzug-01": false, "test-lastenrad-01": false}
	deadline := time.After(20 * time.Second)
	for {
		vehicles := getJSONListAuth(t, fleetURL+"/fleet/vehicles", token)
		for _, v := range vehicles {
			vm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			id, _ := vm["id"].(string)
			if _, want := wantVehicles[id]; want && vm["battery_pct"] != nil {
				wantVehicles[id] = true
			}
		}
		allSeen := true
		for _, seen := range wantVehicles {
			if !seen {
				allSeen = false
			}
		}
		if allSeen {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for fleet-service to expose live status for %v via REST, got %v", wantVehicles, vehicles)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// TestIntegration_FleetService_ZoneStationTaskCRUD exercises the full CRUD chain (AP2/AP3 first
// slice) against the real HTTP API, not just internal/fleetservice's direct store calls.
func TestIntegration_FleetService_ZoneStationTaskCRUD(t *testing.T) {
	token := loginAdmin(t)

	zoneResp := postJSONAuth(t, fleetURL+"/fleet/zones", token, map[string]string{
		"id": "itg-rest-zone", "name": "Integration REST Zone", "environment": "indoor",
	})
	defer zoneResp.Body.Close()
	require.Equal(t, 201, zoneResp.StatusCode)

	stationAResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-rest-station-a", "zone_id": "itg-rest-zone", "name": "A",
	})
	defer stationAResp.Body.Close()
	require.Equal(t, 201, stationAResp.StatusCode)

	stationBResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-rest-station-b", "zone_id": "itg-rest-zone", "name": "B",
	})
	defer stationBResp.Body.Close()
	require.Equal(t, 201, stationBResp.StatusCode)

	// test-lastenzug-01 only exists in `vehicles` once fleet-service's MQTT auto-register has
	// processed at least one of vehicle-mock's status events (FLEET-05, EnsureVehicleExists) —
	// independent of TestIntegration_FleetService_ConsumesRealMQTTStatus_AcrossProcessBoundary's
	// own wait, since Go doesn't guarantee cross-test ordering across files.
	var taskResp *http.Response
	deadline := time.After(20 * time.Second)
	for {
		taskResp = postJSONAuth(t, fleetURL+"/fleet/tasks", token, map[string]any{
			"vehicle_id": "test-lastenzug-01", "from_station_id": "itg-rest-station-a",
			"to_station_id": "itg-rest-station-b", "priority": 1,
		})
		if taskResp.StatusCode == 201 {
			break
		}
		taskResp.Body.Close()
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for test-lastenzug-01 to be auto-registered via MQTT status")
		case <-time.After(500 * time.Millisecond):
		}
	}
	defer taskResp.Body.Close()
	require.Equal(t, 201, taskResp.StatusCode)

	zones := getJSONListAuth(t, fleetURL+"/fleet/zones", token)
	foundZone := false
	for _, z := range zones {
		if zm, ok := z.(map[string]any); ok && zm["id"] == "itg-rest-zone" {
			foundZone = true
		}
	}
	assert.True(t, foundZone, "created zone must be listable via REST")

	tasks := getJSONListAuth(t, fleetURL+"/fleet/tasks", token)
	foundTask := false
	for _, task := range tasks {
		if tm, ok := task.(map[string]any); ok && tm["vehicle_id"] == "test-lastenzug-01" && tm["from_station_id"] == "itg-rest-station-a" {
			foundTask = true
		}
	}
	assert.True(t, foundTask, "created task must be listable via REST")
}

// TestIntegration_FleetService_AcknowledgeAlert_UnknownID_Returns404 verifies the acknowledge
// endpoint is wired end-to-end (auth + routing + store 404 path) even though alert creation itself
// is gateway-driven (vehicle-initiated), not part of the public REST surface.
func TestIntegration_FleetService_AcknowledgeAlert_UnknownID_Returns404(t *testing.T) {
	token := loginAdmin(t)

	resp := postJSONAuth(t, fleetURL+"/fleet/alerts/does-not-exist/acknowledge", token, map[string]string{
		"acknowledged_by": "operator-1",
	})
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}
