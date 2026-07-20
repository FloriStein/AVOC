package integration_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"

	"avoc/internal/fleetgateway"
)

// TestIntegration_FleetSimulation_PublishesRealMQTTMessages verifies FLEET-04 end-to-end across
// the real process boundary: vehicle-mock's fleet simulation (a separate container in this test
// stack, see docker-compose.test.yml) publishes to the real Mosquitto broker, and this test
// subscribes as an independent MQTT client — the same verification done manually during FLEET-04
// (docker run + mosquitto_sub), now automated and repeatable.
func TestIntegration_FleetSimulation_PublishesRealMQTTMessages(t *testing.T) {
	client := connectTestMQTTClient(t, "integration-test-fleet-sub")
	defer client.Disconnect(250)

	var mu sync.Mutex
	statusByVehicle := map[string][]fleetgateway.VehicleStatusEvent{}

	subToken := client.Subscribe(fleetgateway.StatusTopicWildcard, 1, func(_ mqtt.Client, msg mqtt.Message) {
		var evt fleetgateway.VehicleStatusEvent
		if err := json.Unmarshal(msg.Payload(), &evt); err != nil {
			t.Logf("failed to parse status message on %s: %v (payload: %s)", msg.Topic(), err, msg.Payload())
			return
		}
		mu.Lock()
		statusByVehicle[evt.VehicleID] = append(statusByVehicle[evt.VehicleID], evt)
		mu.Unlock()
	})
	require.True(t, subToken.WaitTimeout(5*time.Second), "MQTT subscribe timed out")
	require.NoError(t, subToken.Error(), "MQTT subscribe failed")

	wantVehicles := []string{"test-lastenzug-01", "test-lastenrad-01"}
	deadline := time.After(20 * time.Second)
	for {
		mu.Lock()
		ready := true
		for _, id := range wantVehicles {
			if len(statusByVehicle[id]) < 2 {
				ready = false
			}
		}
		snapshot := make(map[string]int, len(statusByVehicle))
		for id, events := range statusByVehicle {
			snapshot[id] = len(events)
		}
		mu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for >=2 status messages per vehicle, got %v", snapshot)
		case <-time.After(500 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, id := range wantVehicles {
		events := statusByVehicle[id]
		require.GreaterOrEqual(t, len(events), 2, "expected multiple status updates for %s", id)

		first := events[0]
		require.Equal(t, id, first.VehicleID)
		require.Equal(t, "autonomous", first.AutonomyMode, "fleet vehicles simulate autonomous operation (ADR-028)")
		require.NotNil(t, first.BatteryPct)
		require.GreaterOrEqual(t, *first.BatteryPct, 0.0)
		require.LessOrEqual(t, *first.BatteryPct, 100.0)
		require.NotNil(t, first.PositionLat)
		require.NotNil(t, first.PositionLon)
		require.False(t, first.Timestamp.IsZero())

		// Battery should change across ticks (draining while moving, per FLEET-04) — not a
		// static/frozen mock value.
		last := events[len(events)-1]
		require.NotEqual(t, *first.BatteryPct, *last.BatteryPct,
			"expected battery to change across ticks for %s, got constant %.2f", id, *first.BatteryPct)
	}
}
