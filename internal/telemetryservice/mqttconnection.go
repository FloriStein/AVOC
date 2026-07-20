package telemetryservice

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"avoc/pkg/mqtttls"
)

// MQTTConnection is the narrow MQTT port for telemetry-service (ADR-031, HEXTELE-01) — lets
// Client depend on a project-owned, 4-method interface instead of importing
// paho.mqtt.golang's 14-method mqtt.Client directly (GOSTYLE Rule 2.2, Interface Segregation).
// The domain side only ever sees error/[]byte — mqtt.Token/mqtt.Message stay inside the adapter.
type MQTTConnection interface {
	Connect() error
	Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error
	Disconnect(quiesceMs uint)
	IsConnected() bool
}

// PahoConnectionConfig bundles PahoConnection's construction parameters (GOSTYLE Rule 2.3).
type PahoConnectionConfig struct {
	Broker     string
	Username   string
	Password   string
	ClientID   string
	CACertPath string
}

// PahoConnection is the production MQTTConnection, backed by paho.mqtt.golang.
type PahoConnection struct {
	client mqtt.Client
}

var _ MQTTConnection = (*PahoConnection)(nil)

// NewPahoConnection builds a paho MQTT client with automatic reconnect over TLS (MQTTS-03,
// Sprint 40 — verified against CACertPath, no InsecureSkipVerify). onConnect fires after every
// successful (re)connect, including reconnects after a drop — paho does not persist subscriptions
// across a reconnect, so callers use onConnect to (re-)subscribe.
func NewPahoConnection(cfg PahoConnectionConfig, onConnect func(), onConnectionLost func(error)) (*PahoConnection, error) {
	tlsConfig, err := mqtttls.LoadClientConfig(cfg.CACertPath)
	if err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions().
		AddBroker("tls://" + cfg.Broker).
		SetTLSConfig(tlsConfig).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetConnectRetryInterval(reconnectDelay).
		SetOnConnectHandler(func(_ mqtt.Client) {
			onConnect()
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			onConnectionLost(err)
		})

	return &PahoConnection{client: mqtt.NewClient(opts)}, nil
}

func (p *PahoConnection) Connect() error {
	token := p.client.Connect()
	token.Wait()
	return token.Error()
}

func (p *PahoConnection) Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error {
	token := p.client.Subscribe(topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
	})
	token.Wait()
	return token.Error()
}

func (p *PahoConnection) Disconnect(quiesceMs uint) {
	p.client.Disconnect(quiesceMs)
}

func (p *PahoConnection) IsConnected() bool {
	return p.client.IsConnected()
}
