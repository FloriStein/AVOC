package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"avoc/internal/fleetgateway"
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

// fleetWSURL builds a GET /fleet/ws URL against the real fleet-service container — the token
// travels as a query parameter (not a header) because that's how a real browser WebSocket client
// has to authenticate the handshake (Handler.ServeWS / wsToken, FLEET-06).
func fleetWSURL(token string) string {
	return strings.Replace(fleetURL, "http://", "ws://", 1) + "/fleet/ws?token=" + token
}

// readWSEvent reads one message off conn and decodes it as a fleetservice.WSEvent envelope
// ({"type": ..., "data": ...}) — tests only care about the Type/raw Data here, the same shape
// Handler.ServeWS/Hub.Broadcast produce in production.
func readWSEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) (string, map[string]any) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err, "ReadMessage")
	var evt struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(msg, &evt), "unmarshal WS event: %s", msg)
	return evt.Type, evt.Data
}

// TestIntegration_FleetService_WSBroadcast_RequiresAuth verifies GET /fleet/ws rejects the
// handshake without a valid token — same operator-facing data-gating rationale as
// TestIntegration_FleetService_RequiresAuth, just for the WS endpoint (FLEET-06).
func TestIntegration_FleetService_WSBroadcast_RequiresAuth(t *testing.T) {
	url := strings.Replace(fleetURL, "http://", "ws://", 1) + "/fleet/ws"
	_, err := dialWS(t, url)
	require.Error(t, err, "WS handshake without a token must be rejected")
}

// TestIntegration_FleetService_WSBroadcast_InvalidToken_Rejected covers the edge case
// TestIntegration_FleetService_WSBroadcast_RequiresAuth doesn't: a syntactically present but
// invalid token (garbage, not just absent) must be rejected the same way — ServeWS's
// validateToken call, not just the "did a token arrive at all" branch.
func TestIntegration_FleetService_WSBroadcast_InvalidToken_Rejected(t *testing.T) {
	url := strings.Replace(fleetURL, "http://", "ws://", 1) + "/fleet/ws?token=not.a.valid.jwt"
	_, err := dialWS(t, url)
	require.Error(t, err, "WS handshake with an invalid token must be rejected")
}

// TestIntegration_FleetService_WSBroadcast_DeliversTaskCreated is the real end-to-end proof for
// FLEET-06's REST-triggered broadcasts: connect a Dashboard WS client first, then create a task
// via the normal REST API (as the Dashboard would), and confirm the WS client receives a
// "task_created" event carrying that task — without polling.
func TestIntegration_FleetService_WSBroadcast_DeliversTaskCreated(t *testing.T) {
	token := loginAdmin(t)

	conn, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err, "WS handshake with a valid token must succeed")
	defer conn.Close()

	zoneResp := postJSONAuth(t, fleetURL+"/fleet/zones", token, map[string]string{
		"id": "itg-ws-zone", "name": "WS Broadcast Zone", "environment": "indoor",
	})
	defer zoneResp.Body.Close()
	require.Equal(t, 201, zoneResp.StatusCode)

	stationAResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-ws-station-a", "zone_id": "itg-ws-zone", "name": "A",
	})
	defer stationAResp.Body.Close()
	require.Equal(t, 201, stationAResp.StatusCode)

	stationBResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-ws-station-b", "zone_id": "itg-ws-zone", "name": "B",
	})
	defer stationBResp.Body.Close()
	require.Equal(t, 201, stationBResp.StatusCode)

	// test-lastenrad-01 only exists once vehicle-mock's MQTT status has auto-registered it
	// (FLEET-05, EnsureVehicleExists) — same wait pattern as
	// TestIntegration_FleetService_ZoneStationTaskCRUD, independent test ordering.
	var taskResp *http.Response
	deadline := time.After(20 * time.Second)
	for {
		taskResp = postJSONAuth(t, fleetURL+"/fleet/tasks", token, map[string]any{
			"vehicle_id": "test-lastenrad-01", "from_station_id": "itg-ws-station-a",
			"to_station_id": "itg-ws-station-b", "priority": 2,
		})
		if taskResp.StatusCode == 201 {
			break
		}
		taskResp.Body.Close()
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for test-lastenrad-01 to be auto-registered via MQTT status")
		case <-time.After(500 * time.Millisecond):
		}
	}
	defer taskResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(taskResp.Body).Decode(&created))

	// The WS connection may also receive interleaved vehicle_status broadcasts from the
	// continuously-simulating vehicle-mock — skip those, wait specifically for task_created.
	data := readWSEventOfType(t, conn, "task_created", 10*time.Second)
	assert.Equal(t, created["id"], data["id"])
	assert.Equal(t, "test-lastenrad-01", data["vehicle_id"])
	assert.Equal(t, "itg-ws-station-a", data["from_station_id"])
	assert.Equal(t, "itg-ws-station-b", data["to_station_id"])
}

// TestIntegration_FleetService_WSBroadcast_MultiWorkstationFanout is FLEET-06's core claim
// (ADR-028): several Dashboard clients (= operators at different workstations) connected at the
// same time must all receive the same live update, fed here by vehicle-mock's real, continuous
// MQTT status stream (FLEET-04) — no REST call needed to trigger it.
func TestIntegration_FleetService_WSBroadcast_MultiWorkstationFanout(t *testing.T) {
	token := loginAdmin(t)

	connA, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err)
	defer connA.Close()
	connB, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err)
	defer connB.Close()

	typeA, dataA := readWSEvent(t, connA, 10*time.Second)
	typeB, dataB := readWSEvent(t, connB, 10*time.Second)

	require.Equal(t, "vehicle_status", typeA)
	require.Equal(t, "vehicle_status", typeB)
	assert.Contains(t, []any{"test-lastenzug-01", "test-lastenrad-01"}, dataA["vehicle_id"])
	assert.Contains(t, []any{"test-lastenzug-01", "test-lastenrad-01"}, dataB["vehicle_id"])
}

// readWSEventOfType drains WS messages until it finds one matching wantType or the deadline
// expires — the connection also receives interleaved vehicle_status broadcasts from
// vehicle-mock's continuous simulation, so tests waiting for a specific REST/MQTT-triggered event
// can't just read the very next message.
func readWSEventOfType(t *testing.T, conn *websocket.Conn, wantType string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timed out waiting for a %q WS broadcast", wantType)
		}
		eventType, data := readWSEvent(t, conn, remaining)
		if eventType == wantType {
			return data
		}
	}
}

// TestIntegration_FleetService_WSBroadcast_DeliversAlertCreatedAndAcknowledged is the real
// end-to-end proof for FLEET-06's two MQTT/alert-triggered broadcast paths, which
// TestIntegration_FleetService_WSBroadcast_DeliversTaskCreated does not cover: a vehicle-initiated
// alert arriving over the real Mosquitto broker (gw.SubscribeVehicleAlerts callback in
// cmd/fleet-service/main.go, mirrors TestIntegration_FleetSimulation_PublishesRealMQTTMessages'
// publish pattern) must broadcast "alert_created", and acknowledging it via the real REST API
// must broadcast "alert_acknowledged" — both to an already-connected Dashboard WS client.
func TestIntegration_FleetService_WSBroadcast_DeliversAlertCreatedAndAcknowledged(t *testing.T) {
	token := loginAdmin(t)

	conn, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err)
	defer conn.Close()

	mqttClient := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(mqttTestBroker).
		SetClientID("integration-test-fleet-alert-pub").
		SetUsername(mqttTestUsername).
		SetPassword(mqttTestPassword).
		SetConnectTimeout(5 * time.Second))
	connToken := mqttClient.Connect()
	require.True(t, connToken.WaitTimeout(5*time.Second), "MQTT connect timed out")
	require.NoError(t, connToken.Error(), "MQTT connect failed")
	defer mqttClient.Disconnect(250)

	const vehicleID = "itg-ws-alert-vehicle"
	payload, err := json.Marshal(fleetgateway.VehicleAlertEvent{
		VehicleID: vehicleID, Severity: "critical", Message: "Integration test alert broadcast",
	})
	require.NoError(t, err)
	pubToken := mqttClient.Publish(fleetgateway.AlertTopic(vehicleID), 1, false, payload)
	require.True(t, pubToken.WaitTimeout(5*time.Second), "MQTT publish timed out")
	require.NoError(t, pubToken.Error(), "MQTT publish failed")

	created := readWSEventOfType(t, conn, "alert_created", 15*time.Second)
	assert.Equal(t, vehicleID, created["vehicle_id"])
	assert.Equal(t, "critical", created["severity"])
	assert.Equal(t, "Integration test alert broadcast", created["message"])
	alertID, _ := created["id"].(string)
	require.NotEmpty(t, alertID, "alert_created broadcast must carry the persisted alert's id")

	ackResp := postJSONAuth(t, fleetURL+"/fleet/alerts/"+alertID+"/acknowledge", token, map[string]string{
		"acknowledged_by": "itg-ws-operator",
	})
	defer ackResp.Body.Close()
	require.Equal(t, 204, ackResp.StatusCode)

	acked := readWSEventOfType(t, conn, "alert_acknowledged", 10*time.Second)
	assert.Equal(t, alertID, acked["id"])
	assert.Equal(t, "itg-ws-operator", acked["acknowledged_by"])
	assert.NotEmpty(t, acked["acknowledged_at"])
}

// TestIntegration_FleetService_AlertEngine_LowBatteryTriggersThresholdAlert is the real
// end-to-end proof for FLEET-07: a status event with a critically low battery, published by an
// independent MQTT client (simulating a real vehicle, not going through vehicle-mock's own
// simulation), must make fleet-service raise a threshold-based alert on its own — distinct from
// the vehicle-initiated alert path already covered by
// TestIntegration_FleetService_WSBroadcast_DeliversAlertCreatedAndAcknowledged (which publishes
// directly to the alert topic, bypassing AlertEngine entirely). A second low-battery tick for the
// same vehicle must not raise a second alert (AlertEngine's repeat-suppression, FLEET-07).
func TestIntegration_FleetService_AlertEngine_LowBatteryTriggersThresholdAlert(t *testing.T) {
	token := loginAdmin(t)
	const vehicleID = "itg-alertengine-vehicle"

	conn, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err)
	defer conn.Close()

	mqttClient := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(mqttTestBroker).
		SetClientID("integration-test-fleet-alertengine-pub").
		SetUsername(mqttTestUsername).
		SetPassword(mqttTestPassword).
		SetConnectTimeout(5 * time.Second))
	connToken := mqttClient.Connect()
	require.True(t, connToken.WaitTimeout(5*time.Second), "MQTT connect timed out")
	require.NoError(t, connToken.Error(), "MQTT connect failed")
	defer mqttClient.Disconnect(250)

	publishBattery := func(pct float64) {
		t.Helper()
		battery := pct
		payload, err := json.Marshal(fleetgateway.VehicleStatusEvent{
			VehicleID: vehicleID, BatteryPct: &battery, AutonomyMode: "autonomous", Timestamp: time.Now(),
		})
		require.NoError(t, err)
		pubToken := mqttClient.Publish(fleetgateway.StatusTopic(vehicleID), 1, false, payload)
		require.True(t, pubToken.WaitTimeout(5*time.Second), "MQTT publish timed out")
		require.NoError(t, pubToken.Error(), "MQTT publish failed")
	}

	publishBattery(5.0)

	alert := readWSEventOfType(t, conn, "alert_created", 15*time.Second)
	assert.Equal(t, vehicleID, alert["vehicle_id"])
	assert.Equal(t, "critical", alert["severity"])
	assert.Contains(t, alert["message"], "kritisch")

	alerts := getJSONListAuth(t, fleetURL+"/fleet/alerts", token)
	found := 0
	for _, a := range alerts {
		if am, ok := a.(map[string]any); ok && am["vehicle_id"] == vehicleID {
			found++
		}
	}
	assert.Equal(t, 1, found, "expected exactly one persisted alert for %s", vehicleID)

	// A second tick at (an even lower) still-critical battery must not raise a second alert —
	// AlertEngine only alerts on crossing into a worse tier, not on every status update.
	publishBattery(3.0)
	time.Sleep(1 * time.Second)

	alertsAfter := getJSONListAuth(t, fleetURL+"/fleet/alerts", token)
	foundAfter := 0
	for _, a := range alertsAfter {
		if am, ok := a.(map[string]any); ok && am["vehicle_id"] == vehicleID {
			foundAfter++
		}
	}
	assert.Equal(t, 1, foundAfter, "expected no repeat alert for a second low-battery tick")
}

// TestIntegration_FleetService_UpdateTaskStatus_DeliversBroadcastAndPersists is the ADR-030
// end-to-end proof: create a task via the real REST API, transition it via the real
// PATCH /fleet/tasks/{id}/status endpoint, and confirm both effects a Dashboard client relies on —
// the WS "task_status_changed" broadcast (not polling) and the persisted row via GET /fleet/tasks.
func TestIntegration_FleetService_UpdateTaskStatus_DeliversBroadcastAndPersists(t *testing.T) {
	token := loginAdmin(t)

	conn, err := dialWS(t, fleetWSURL(token))
	require.NoError(t, err, "WS handshake with a valid token must succeed")
	defer conn.Close()

	zoneResp := postJSONAuth(t, fleetURL+"/fleet/zones", token, map[string]string{
		"id": "itg-status-zone", "name": "Status Transition Zone", "environment": "indoor",
	})
	defer zoneResp.Body.Close()
	require.Equal(t, 201, zoneResp.StatusCode)

	stationAResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-status-station-a", "zone_id": "itg-status-zone", "name": "A",
	})
	defer stationAResp.Body.Close()
	require.Equal(t, 201, stationAResp.StatusCode)

	stationBResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-status-station-b", "zone_id": "itg-status-zone", "name": "B",
	})
	defer stationBResp.Body.Close()
	require.Equal(t, 201, stationBResp.StatusCode)

	// test-lastenzug-01 only exists once vehicle-mock's MQTT status has auto-registered it
	// (FLEET-05, EnsureVehicleExists) — same wait pattern as the other REST/WS tests in this file.
	var taskResp *http.Response
	deadline := time.After(20 * time.Second)
	for {
		taskResp = postJSONAuth(t, fleetURL+"/fleet/tasks", token, map[string]any{
			"vehicle_id": "test-lastenzug-01", "from_station_id": "itg-status-station-a",
			"to_station_id": "itg-status-station-b", "priority": 3,
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
	var created map[string]any
	require.NoError(t, json.NewDecoder(taskResp.Body).Decode(&created))
	taskID, _ := created["id"].(string)
	require.NotEmpty(t, taskID)

	patchResp := patchJSONAuth(t, fleetURL+"/fleet/tasks/"+taskID+"/status", token, map[string]string{
		"status": "in_progress", "changed_by": "itg-operator-1",
	})
	defer patchResp.Body.Close()
	require.Equal(t, 200, patchResp.StatusCode)

	// Interleaved vehicle_status broadcasts from vehicle-mock's continuous simulation are expected
	// on this connection — wait specifically for task_status_changed, same pattern as
	// TestIntegration_FleetService_WSBroadcast_DeliversTaskCreated.
	data := readWSEventOfType(t, conn, "task_status_changed", 10*time.Second)
	assert.Equal(t, taskID, data["id"])
	assert.Equal(t, "in_progress", data["status"])
	assert.Equal(t, "itg-operator-1", data["status_changed_by"])

	tasks := getJSONListAuth(t, fleetURL+"/fleet/tasks", token)
	found := false
	for _, task := range tasks {
		if tm, ok := task.(map[string]any); ok && tm["id"] == taskID {
			found = true
			assert.Equal(t, "in_progress", tm["status"])
		}
	}
	assert.True(t, found, "transitioned task must be listable via REST with its new status")
}

// TestIntegration_FleetService_UpdateTaskStatus_InvalidTransition_Returns409 proves ADR-030's
// state machine is enforced through the real HTTP path, not just at the store layer
// (internal/fleetservice's own tests already cover the store/handler in isolation) — a freshly
// created task is still "pending", so jumping straight to "completed" must be rejected.
func TestIntegration_FleetService_UpdateTaskStatus_InvalidTransition_Returns409(t *testing.T) {
	token := loginAdmin(t)

	zoneResp := postJSONAuth(t, fleetURL+"/fleet/zones", token, map[string]string{
		"id": "itg-status-conflict-zone", "name": "Status Conflict Zone", "environment": "indoor",
	})
	defer zoneResp.Body.Close()
	require.Equal(t, 201, zoneResp.StatusCode)

	stationAResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-status-conflict-station-a", "zone_id": "itg-status-conflict-zone", "name": "A",
	})
	defer stationAResp.Body.Close()
	require.Equal(t, 201, stationAResp.StatusCode)

	stationBResp := postJSONAuth(t, fleetURL+"/fleet/stations", token, map[string]string{
		"id": "itg-status-conflict-station-b", "zone_id": "itg-status-conflict-zone", "name": "B",
	})
	defer stationBResp.Body.Close()
	require.Equal(t, 201, stationBResp.StatusCode)

	var taskResp *http.Response
	deadline := time.After(20 * time.Second)
	for {
		taskResp = postJSONAuth(t, fleetURL+"/fleet/tasks", token, map[string]any{
			"vehicle_id": "test-lastenrad-01", "from_station_id": "itg-status-conflict-station-a",
			"to_station_id": "itg-status-conflict-station-b", "priority": 1,
		})
		if taskResp.StatusCode == 201 {
			break
		}
		taskResp.Body.Close()
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for test-lastenrad-01 to be auto-registered via MQTT status")
		case <-time.After(500 * time.Millisecond):
		}
	}
	defer taskResp.Body.Close()
	var created map[string]any
	require.NoError(t, json.NewDecoder(taskResp.Body).Decode(&created))
	taskID, _ := created["id"].(string)
	require.NotEmpty(t, taskID)

	patchResp := patchJSONAuth(t, fleetURL+"/fleet/tasks/"+taskID+"/status", token, map[string]string{
		"status": "completed", "changed_by": "itg-operator-1",
	})
	defer patchResp.Body.Close()
	assert.Equal(t, 409, patchResp.StatusCode, "pending->completed must be rejected (only in_progress->completed is allowed, ADR-030)")
}
