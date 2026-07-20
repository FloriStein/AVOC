package telemetryservice

import (
	"fmt"
	"sync"
	"testing"

	commonv1 "avoc/gen/go/common/v1"
	telemetryv1 "avoc/gen/go/telemetry/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newTestClient() *Client {
	return &Client{
		broker: "test-broker:1883",
		latest: make(map[string]*telemetryv1.TelemetryEvent),
	}
}

func mustMarshalEvent(t *testing.T, event *telemetryv1.TelemetryEvent) []byte {
	t.Helper()
	payload, err := proto.Marshal(event)
	require.NoError(t, err)
	return payload
}

func TestClient_HandleMessage_ValidPayload_StoresLatest(t *testing.T) {
	c := newTestClient()
	event := &telemetryv1.TelemetryEvent{
		Header:     &commonv1.CorrelationHeader{VehicleId: "vehicle-1", SessionId: "session-1"},
		SpeedKmh:   12.5,
		BatteryPct: 88,
	}

	c.handleMessage("vehicle/vehicle-1/telemetry", mustMarshalEvent(t, event))

	got, ok := c.GetLatest("vehicle-1")
	require.True(t, ok)
	assert.Equal(t, 12.5, got.SpeedKmh)
	assert.Equal(t, float64(88), got.BatteryPct)
}

func TestClient_HandleMessage_MissingHeader_Ignored(t *testing.T) {
	c := newTestClient()
	event := &telemetryv1.TelemetryEvent{SpeedKmh: 5}

	c.handleMessage("vehicle//telemetry", mustMarshalEvent(t, event))

	_, ok := c.GetLatest("")
	assert.False(t, ok)
	assert.Empty(t, c.latest)
}

func TestClient_HandleMessage_EmptyVehicleID_Ignored(t *testing.T) {
	c := newTestClient()
	event := &telemetryv1.TelemetryEvent{
		Header: &commonv1.CorrelationHeader{VehicleId: ""},
	}

	c.handleMessage("vehicle//telemetry", mustMarshalEvent(t, event))

	assert.Empty(t, c.latest)
}

func TestClient_HandleMessage_MalformedPayload_Ignored(t *testing.T) {
	c := newTestClient()

	c.handleMessage("vehicle/vehicle-1/telemetry", []byte{0xFF, 0x00, 0xDE, 0xAD})

	_, ok := c.GetLatest("vehicle-1")
	assert.False(t, ok)
}

func TestClient_HandleMessage_EmptyPayload_Ignored(t *testing.T) {
	c := newTestClient()

	c.handleMessage("vehicle/vehicle-1/telemetry", []byte{})

	assert.Empty(t, c.latest)
}

func TestClient_HandleMessage_OverwritesPreviousEvent(t *testing.T) {
	c := newTestClient()
	first := &telemetryv1.TelemetryEvent{Header: &commonv1.CorrelationHeader{VehicleId: "vehicle-1"}, SpeedKmh: 1}
	second := &telemetryv1.TelemetryEvent{Header: &commonv1.CorrelationHeader{VehicleId: "vehicle-1"}, SpeedKmh: 2}

	c.handleMessage("vehicle/vehicle-1/telemetry", mustMarshalEvent(t, first))
	c.handleMessage("vehicle/vehicle-1/telemetry", mustMarshalEvent(t, second))

	got, ok := c.GetLatest("vehicle-1")
	require.True(t, ok)
	assert.Equal(t, float64(2), got.SpeedKmh)
}

func TestClient_GetLatest_UnknownVehicle_NotFound(t *testing.T) {
	c := newTestClient()

	_, ok := c.GetLatest("unknown-vehicle")

	assert.False(t, ok)
}

func TestClient_Subscribe_WiresTopicAndHandlerThroughPort(t *testing.T) {
	c := newTestClient()
	fake := newFakeMQTTConnection()
	c.client = fake

	c.subscribe()

	assert.Equal(t, topicVehicleTelemetry, fake.subscribedTopic)
	assert.Equal(t, byte(1), fake.subscribedQoS)

	event := &telemetryv1.TelemetryEvent{Header: &commonv1.CorrelationHeader{VehicleId: "vehicle-1"}, SpeedKmh: 42}
	fake.Deliver("vehicle/vehicle-1/telemetry", mustMarshalEvent(t, event))

	got, ok := c.GetLatest("vehicle-1")
	require.True(t, ok)
	assert.Equal(t, float64(42), got.SpeedKmh)
}

func TestClient_Subscribe_PortError_LogsAndDoesNotPanic(t *testing.T) {
	c := newTestClient()
	fake := newFakeMQTTConnection()
	fake.SubscribeErr = fmt.Errorf("broker unavailable")
	c.client = fake

	assert.NotPanics(t, func() { c.subscribe() })
	assert.Empty(t, fake.subscribedTopic)
}

func TestClient_Disconnect_WhenConnected_CallsPortWithQuiesce(t *testing.T) {
	c := newTestClient()
	fake := newFakeMQTTConnection()
	fake.setConnected(true)
	c.client = fake

	c.Disconnect()

	assert.Equal(t, 1, fake.disconnectCalls)
	assert.Equal(t, uint(250), fake.lastQuiesceMs)
}

func TestClient_Disconnect_WhenNotConnected_SkipsPort(t *testing.T) {
	c := newTestClient()
	fake := newFakeMQTTConnection()
	c.client = fake

	c.Disconnect()

	assert.Equal(t, 0, fake.disconnectCalls)
}

func TestClient_Disconnect_NilClient_NoPanic(t *testing.T) {
	c := newTestClient()

	assert.NotPanics(t, func() { c.Disconnect() })
}

func TestClient_ConcurrentHandleMessageAndGetLatest_NoRace(t *testing.T) {
	c := newTestClient()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		vehicleID := fmt.Sprintf("vehicle-%d", i%4)
		wg.Add(2)
		go func(vehicleID string, speed float64) {
			defer wg.Done()
			event := &telemetryv1.TelemetryEvent{
				Header:   &commonv1.CorrelationHeader{VehicleId: vehicleID},
				SpeedKmh: speed,
			}
			payload, err := proto.Marshal(event)
			if err != nil {
				return
			}
			c.handleMessage("vehicle/"+vehicleID+"/telemetry", payload)
		}(vehicleID, float64(i))
		go func(vehicleID string) {
			defer wg.Done()
			c.GetLatest(vehicleID)
		}(vehicleID)
	}

	wg.Wait()
}
