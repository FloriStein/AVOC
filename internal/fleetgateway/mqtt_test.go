package fleetgateway

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Compile-time check: MQTTGateway must satisfy the FleetGateway interface.
var _ FleetGateway = (*MQTTGateway)(nil)

func mqttTestBroker(t *testing.T) string {
	t.Helper()
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		t.Skip("MQTT_BROKER not set — skipping Mosquitto integration test")
	}
	return "tcp://" + broker
}

func TestMQTTGateway_ReceivesStatusPublishedByExternalClient(t *testing.T) {
	broker := mqttTestBroker(t)

	gw, err := NewMQTTGateway(broker)
	if err != nil {
		t.Fatalf("NewMQTTGateway: %v", err)
	}
	t.Cleanup(gw.Close)

	var mu sync.Mutex
	var received VehicleStatusEvent
	got := make(chan struct{})
	gw.SubscribeVehicleStatus(func(e VehicleStatusEvent) {
		mu.Lock()
		received = e
		mu.Unlock()
		close(got)
	})

	pub := connectPublisher(t, broker)
	defer pub.Disconnect(250)

	battery := 42.5
	event := VehicleStatusEvent{VehicleID: "mqtt-test-v1", BatteryPct: &battery, AutonomyMode: "autonomous", Timestamp: time.Now()}
	publishJSON(t, pub, StatusTopic("mqtt-test-v1"), event)

	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for status event")
	}

	mu.Lock()
	defer mu.Unlock()
	if received.VehicleID != "mqtt-test-v1" || received.BatteryPct == nil || *received.BatteryPct != 42.5 {
		t.Fatalf("unexpected event: %+v", received)
	}
}

func TestMQTTGateway_ReceivesAlertPublishedByExternalClient(t *testing.T) {
	broker := mqttTestBroker(t)

	gw, err := NewMQTTGateway(broker)
	if err != nil {
		t.Fatalf("NewMQTTGateway: %v", err)
	}
	t.Cleanup(gw.Close)

	var mu sync.Mutex
	var received VehicleAlertEvent
	got := make(chan struct{})
	gw.SubscribeVehicleAlerts(func(e VehicleAlertEvent) {
		mu.Lock()
		received = e
		mu.Unlock()
		close(got)
	})

	pub := connectPublisher(t, broker)
	defer pub.Disconnect(250)

	event := VehicleAlertEvent{VehicleID: "mqtt-test-v2", Severity: "critical", Message: "Hindernis erkannt", Timestamp: time.Now()}
	publishJSON(t, pub, AlertTopic("mqtt-test-v2"), event)

	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for alert event")
	}

	mu.Lock()
	defer mu.Unlock()
	if received.VehicleID != "mqtt-test-v2" || received.Severity != "critical" || received.Message != "Hindernis erkannt" {
		t.Fatalf("unexpected event: %+v", received)
	}
}

func TestMQTTGateway_MalformedPayload_DoesNotCrashSubscriber(t *testing.T) {
	broker := mqttTestBroker(t)

	gw, err := NewMQTTGateway(broker)
	if err != nil {
		t.Fatalf("NewMQTTGateway: %v", err)
	}
	t.Cleanup(gw.Close)

	var callCount int
	var mu sync.Mutex
	gw.SubscribeVehicleStatus(func(VehicleStatusEvent) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	pub := connectPublisher(t, broker)
	defer pub.Disconnect(250)

	// Malformed JSON on the status topic — must be silently dropped, not panic the process.
	token := pub.Publish(StatusTopic("mqtt-test-malformed"), 1, false, []byte("not json{{{"))
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Then a valid message on a different vehicle — proves the subscriber loop survived the
	// malformed payload and keeps working.
	battery := 10.0
	valid := VehicleStatusEvent{VehicleID: "mqtt-test-recovers", BatteryPct: &battery, AutonomyMode: "autonomous", Timestamp: time.Now()}
	got := make(chan struct{})
	gw.SubscribeVehicleStatus(func(e VehicleStatusEvent) {
		if e.VehicleID == "mqtt-test-recovers" {
			close(got)
		}
	})
	publishJSON(t, pub, StatusTopic("mqtt-test-recovers"), valid)

	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for valid event after malformed payload — subscriber may have crashed")
	}
}

func TestMQTTGateway_DispatchTask_PublishesToTaskTopic(t *testing.T) {
	broker := mqttTestBroker(t)

	gw, err := NewMQTTGateway(broker)
	if err != nil {
		t.Fatalf("NewMQTTGateway: %v", err)
	}
	t.Cleanup(gw.Close)

	sub := connectPublisher(t, broker) // reused as a plain subscriber client
	defer sub.Disconnect(250)

	got := make(chan []byte, 1)
	token := sub.Subscribe(TaskTopic("mqtt-test-dispatch"), 1, func(_ mqtt.Client, msg mqtt.Message) {
		got <- msg.Payload()
	})
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("subscribe timed out")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	err = gw.DispatchTask(TaskAssignment{TaskID: "t1", VehicleID: "mqtt-test-dispatch", FromStationID: "a", ToStationID: "b", Priority: 3})
	if err != nil {
		t.Fatalf("DispatchTask: %v", err)
	}

	select {
	case payload := <-got:
		if len(payload) == 0 {
			t.Fatal("expected non-empty task payload")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for dispatched task message")
	}
}

func connectPublisher(t *testing.T, broker string) mqtt.Client {
	t.Helper()
	client := mqtt.NewClient(mqtt.NewClientOptions().AddBroker(broker).SetClientID("fleetgateway-test-pub-" + t.Name()))
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("publisher connect timed out")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("publisher connect failed: %v", err)
	}
	return client
}

func publishJSON(t *testing.T, client mqtt.Client, topic string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	token := client.Publish(topic, 1, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("publish to %s failed: %v", topic, err)
	}
}
