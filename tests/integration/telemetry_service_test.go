// telemetry-service — Direct Integration Test (INTTEST-02, Sprint 52).
//
// telemetry_watchdog_test.go (Sprint 50) already exercises telemetry-service indirectly, through
// control-server's DegradeWatchdog polling it — that proves the watchdog's HTTP client works, not
// that telemetry-service's own GET /telemetry/latest/{vehicleID} endpoint is correct across a real
// process boundary. This test talks to telemetry-service directly, mirroring what a Prozessgrenze-
// crossing consumer (or a future dashboard) would do: publish to MQTT like a real vehicle, then
// read the ingested event back over telemetry-service's own HTTP API.
package integration_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const telemetryURL = "http://localhost:18083"

func TestIntegration_MQTTPublish_ReachesTelemetryServiceLatestEndpoint(t *testing.T) {
	client := connectTestMQTTClient(t, "integration-test-telemetry-latest")
	defer client.Disconnect(250)

	const vehicleID = "vehicle-telemetry-latest"
	publishTestTelemetry(t, client, vehicleID)

	require.Eventually(t, func() bool {
		resp, err := http.Get(telemetryURL + "/telemetry/latest/" + vehicleID)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 100*time.Millisecond, "telemetry-service must ingest the MQTT publish and expose it via GET /telemetry/latest")

	body := getJSON(t, telemetryURL+"/telemetry/latest/"+vehicleID)
	assert.Equal(t, vehicleID, body["vehicle_id"])
	assert.Equal(t, "INTEGRATION_TEST", body["status"])
}

func TestIntegration_TelemetryLatest_UnknownVehicle_404(t *testing.T) {
	resp, err := http.Get(telemetryURL + "/telemetry/latest/vehicle-never-published")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
