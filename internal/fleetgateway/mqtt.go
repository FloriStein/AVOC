package fleetgateway

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"avoc/pkg/ulid"
)

// MQTTGateway is the FleetGateway implementation backed by real Mosquitto pub/sub (FLEET-05) —
// the concrete counterpart to vehicle-mock's fleet simulation (FLEET-04). Subscribes to
// StatusTopicWildcard/AlertTopicWildcard, parses JSON payloads, and dispatches to registered
// callbacks exactly like MockGateway does for in-process events — callers (fleet-service) don't
// need to know or care which implementation is behind the FleetGateway interface.
type MQTTGateway struct {
	client mqtt.Client

	mu         sync.RWMutex
	statusSubs []func(VehicleStatusEvent)
	alertSubs  []func(VehicleAlertEvent)
}

// NewMQTTGateway connects to broker (e.g. "tcp://mosquitto:1883") and subscribes to the fleet
// topic wildcards. Returns an error if the initial connection fails — callers should treat this
// the same way pkg/db.WaitForReady is used elsewhere (retry at the call site if needed).
func NewMQTTGateway(broker string) (*MQTTGateway, error) {
	g := &MQTTGateway{}

	// A fixed ClientID would collide with every other MQTTGateway instance connected to the same
	// broker (a second real fleet-service, another test run, ...) — MQTT brokers evict the
	// existing session when a new connection reuses its ClientID, and with SetAutoReconnect on
	// both sides that turns into a reconnect fight that silently drops in-flight messages for
	// whichever side loses the race (found via manual E2E verification against a live
	// fleet-service container, FLEET-05).
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID("fleet-service-gateway-" + ulid.Generate()).
		SetAutoReconnect(true).
		SetConnectRetryInterval(5 * time.Second)
	g.client = mqtt.NewClient(opts)

	token := g.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("fleetgateway: MQTT connect timed out (broker %s)", broker)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("fleetgateway: MQTT connect failed: %w", err)
	}

	if err := g.subscribe(StatusTopicWildcard, g.handleStatusMessage); err != nil {
		return nil, err
	}
	if err := g.subscribe(AlertTopicWildcard, g.handleAlertMessage); err != nil {
		return nil, err
	}

	return g, nil
}

func (g *MQTTGateway) subscribe(topic string, handler mqtt.MessageHandler) error {
	token := g.client.Subscribe(topic, 1, handler)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("fleetgateway: MQTT subscribe to %s timed out", topic)
	}
	return token.Error()
}

func (g *MQTTGateway) handleStatusMessage(_ mqtt.Client, msg mqtt.Message) {
	var e VehicleStatusEvent
	if err := json.Unmarshal(msg.Payload(), &e); err != nil {
		return // malformed payload from an unexpected/future adapter — drop, don't crash the subscriber loop
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, fn := range g.statusSubs {
		fn(e)
	}
}

func (g *MQTTGateway) handleAlertMessage(_ mqtt.Client, msg mqtt.Message) {
	var e VehicleAlertEvent
	if err := json.Unmarshal(msg.Payload(), &e); err != nil {
		return
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, fn := range g.alertSubs {
		fn(e)
	}
}

func (g *MQTTGateway) SubscribeVehicleStatus(fn func(VehicleStatusEvent)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.statusSubs = append(g.statusSubs, fn)
}

func (g *MQTTGateway) SubscribeVehicleAlerts(fn func(VehicleAlertEvent)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.alertSubs = append(g.alertSubs, fn)
}

// DispatchTask publishes the assignment to TaskTopic(task.VehicleID). Fire-and-forget — no
// vehicle-mock consumer acts on it yet (ADR-027, real semantics unknown until the AP1 workshop).
func (g *MQTTGateway) DispatchTask(task TaskAssignment) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("fleetgateway: marshal task: %w", err)
	}
	token := g.client.Publish(TaskTopic(task.VehicleID), 1, false, data)
	token.Wait()
	return token.Error()
}

// Close disconnects the MQTT client. Safe to call once during shutdown.
func (g *MQTTGateway) Close() {
	g.client.Disconnect(250)
}
