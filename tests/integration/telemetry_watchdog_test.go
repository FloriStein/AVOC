// TelemetryWatchdog — Integration Test (DRIFT-K3-TELEMETRY Teil 2, Sprint 50).
//
// Exercises the real control-server + telemetry-service + mosquitto stack — the pieces a unit
// test with a fake checker (tests/unit/watchdog_test.go) can't cover: the actual HTTP round trip
// against telemetry-service, and telemetry-service's own MQTT subscription. This test publishes
// directly to mosquitto (like a real vehicle would) instead of relying on vehicle-mock's
// continuous publish loop, so the "telemetry never arrives" / "telemetry arrives once" timing is
// fully test-controlled — mirrors connectVehicle's approach of standing in for the vehicle side
// with a minimal, purpose-built client rather than depending on the shared vehicle-mock container.
package integration_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"

	commonv1 "avoc/gen/go/common/v1"
	telemetryv1 "avoc/gen/go/telemetry/v1"
	"avoc/pkg/mqtttls"
	"avoc/pkg/ulid"

	"google.golang.org/protobuf/proto"
)

const (
	testMQTTBroker   = "localhost:18883"
	testMQTTUsername = "avoc"
	testMQTTPassword = "mqtt_test_secret"
)

// connectTestMQTT connects a throwaway MQTT client directly to the test stack's mosquitto —
// same broker/credentials/TLS setup as fleet-service and vehicle-mock (tests/docker-compose.test.yml).
func connectTestMQTT(t *testing.T) mqtt.Client {
	t.Helper()
	tlsConfig, err := mqtttls.LoadClientConfig(filepath.Join("..", "mosquitto-certs", "ca.pem"))
	require.NoError(t, err)

	opts := mqtt.NewClientOptions().
		AddBroker("tls://" + testMQTTBroker).
		SetTLSConfig(tlsConfig).
		SetClientID("avoc-integration-test-" + ulid.Generate()).
		SetUsername(testMQTTUsername).
		SetPassword(testMQTTPassword)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	require.NoError(t, token.Error(), "test MQTT client connect")
	return client
}

// publishTestTelemetry publishes a single, freshly-timestamped TelemetryEvent for vehicleID —
// same topic/encoding as cmd/vehicle-mock's publishTelemetry.
func publishTestTelemetry(t *testing.T, client mqtt.Client, vehicleID string) {
	t.Helper()
	event := &telemetryv1.TelemetryEvent{
		Header: &commonv1.CorrelationHeader{
			EventId:   ulid.Generate(),
			VehicleId: vehicleID,
			Timestamp: time.Now().UnixMilli(),
		},
		Status: "INTEGRATION_TEST",
	}
	data, err := proto.Marshal(event)
	require.NoError(t, err)

	token := client.Publish(fmt.Sprintf("vehicle/%s/telemetry", vehicleID), 1, false, data)
	token.Wait()
	require.NoError(t, token.Error(), "test telemetry publish")
}

func TestIntegration_TelemetryLoss_TriggersDegrade_ThenRecovers(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"username": "admin", "password": "admin_test_secret"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	const vehicleID = "vehicle-telemetry"
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

	// No telemetry is ever published for this vehicle → TelemetryWatchdog must reach DEGRADED
	// within its 2s×2 budget (ADR-009 Update 2026-07-21 Grill-Me) — generous margin for CI.
	require.Eventually(t, func() bool {
		state := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
		return state["system"] == "DEGRADED"
	}, 10*time.Second, 200*time.Millisecond, "missing telemetry must trigger DEGRADED")

	// A fresh telemetry event for this vehicle must be picked up by the next poll and recover
	// the state back to CONNECTED (DEGRADED never triggers/blocks SAFE_MODE, Invariant 1).
	mqttClient := connectTestMQTT(t)
	defer mqttClient.Disconnect(250)
	publishTestTelemetry(t, mqttClient, vehicleID)

	require.Eventually(t, func() bool {
		state := getJSON(t, controlURL+fmt.Sprintf("/vehicles/%s/state", vehicleID))
		return state["system"] == "CONNECTED"
	}, 10*time.Second, 200*time.Millisecond, "fresh telemetry must recover DEGRADED→CONNECTED")
}
