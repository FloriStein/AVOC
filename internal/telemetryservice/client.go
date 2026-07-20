// Package telemetryservice implements the MQTT Telemetry Bridge (BE-05, ADR-003/008/016).
// It subscribes to vehicle telemetry topics on Mosquitto and holds the latest
// TelemetryEvent per vehicle for the control server to query.
package telemetryservice

import (
	"sync"
	"time"

	telemetryv1 "avoc/gen/go/telemetry/v1"
	"avoc/pkg/logger"

	"google.golang.org/protobuf/proto"
)

var svcLog = logger.New("telemetry-service")

const (
	topicVehicleTelemetry = "vehicle/+/telemetry"
	reconnectDelay        = 5 * time.Second
)

// Client manages the MQTT connection and caches the latest TelemetryEvent per vehicle.
type Client struct {
	broker     string
	username   string
	password   string
	caCertPath string
	client     MQTTConnection
	mu         sync.RWMutex
	latest     map[string]*telemetryv1.TelemetryEvent
}

func NewClient(broker, username, password, caCertPath string) *Client {
	return &Client{
		broker:     broker,
		username:   username,
		password:   password,
		caCertPath: caCertPath,
		latest:     make(map[string]*telemetryv1.TelemetryEvent),
	}
}

// Connect establishes the MQTT connection with automatic reconnect.
func (c *Client) Connect() error {
	conn, err := NewPahoConnection(
		PahoConnectionConfig{
			Broker:     c.broker,
			Username:   c.username,
			Password:   c.password,
			ClientID:   "avoc-telemetry-service",
			CACertPath: c.caCertPath,
		},
		func() {
			svcLog.Info("MQTT connected", "broker", c.broker)
			c.subscribe()
		},
		func(err error) {
			svcLog.Warn("MQTT connection lost", "error", err)
		},
	)
	if err != nil {
		return err
	}
	c.client = conn
	return c.client.Connect()
}

func (c *Client) subscribe() {
	if err := c.client.Subscribe(topicVehicleTelemetry, 1, c.handleMessage); err != nil {
		svcLog.Error("MQTT subscribe error", "error", err)
		return
	}
	svcLog.Info("MQTT subscribed", "topic", topicVehicleTelemetry)
}

func (c *Client) handleMessage(topic string, payload []byte) {
	event := &telemetryv1.TelemetryEvent{}
	if err := proto.Unmarshal(payload, event); err != nil {
		svcLog.Warn("MQTT parse error", "topic", topic, "error", err)
		return
	}

	vehicleID := ""
	if event.Header != nil {
		vehicleID = event.Header.VehicleId
	}
	if vehicleID == "" {
		return
	}

	c.mu.Lock()
	c.latest[vehicleID] = event
	c.mu.Unlock()

	svcLog.Debug("telemetry received",
		"vehicle_id", vehicleID,
		"speed_kmh", event.SpeedKmh,
		"battery_pct", event.BatteryPct)
}

// GetLatest returns the most recent TelemetryEvent for the given vehicle.
func (c *Client) GetLatest(vehicleID string) (*telemetryv1.TelemetryEvent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.latest[vehicleID]
	return e, ok
}

// Disconnect closes the MQTT connection cleanly.
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}
