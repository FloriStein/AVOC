package telemetryservice

import "sync"

// FakeMQTTConnection is an in-memory MQTTConnection (ADR-031, HEXTELE-03) — lets Client-level
// tests exercise Connect()/subscribe()/Disconnect() wiring without a live MQTT broker. Delivering
// a message to a subscribed handler is done via Deliver, simulating what paho would do when a
// broker publishes to a subscribed topic.
type FakeMQTTConnection struct {
	mu sync.Mutex

	ConnectErr   error
	SubscribeErr error

	connectCalls    int
	connected       bool
	disconnectCalls int
	lastQuiesceMs   uint

	subscribedTopic string
	subscribedQoS   byte
	handler         func(topic string, payload []byte)
}

var _ MQTTConnection = (*FakeMQTTConnection)(nil)

func newFakeMQTTConnection() *FakeMQTTConnection {
	return &FakeMQTTConnection{}
}

func (f *FakeMQTTConnection) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectCalls++
	if f.ConnectErr != nil {
		return f.ConnectErr
	}
	f.connected = true
	return nil
}

func (f *FakeMQTTConnection) Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SubscribeErr != nil {
		return f.SubscribeErr
	}
	f.subscribedTopic = topic
	f.subscribedQoS = qos
	f.handler = handler
	return nil
}

func (f *FakeMQTTConnection) Disconnect(quiesceMs uint) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnectCalls++
	f.lastQuiesceMs = quiesceMs
	f.connected = false
}

func (f *FakeMQTTConnection) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

// Deliver simulates the broker publishing payload on topic to whatever handler Subscribe
// registered. Panics if no handler was registered, since delivering to an unsubscribed fake is
// a test bug, not a runtime condition Client needs to handle.
func (f *FakeMQTTConnection) Deliver(topic string, payload []byte) {
	f.mu.Lock()
	handler := f.handler
	f.mu.Unlock()
	if handler == nil {
		panic("FakeMQTTConnection.Deliver: no handler registered — call Subscribe first")
	}
	handler(topic, payload)
}

func (f *FakeMQTTConnection) setConnected(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = v
}
