// Package integration tests all services in the Docker test stack (TEST-03, ADR-006).
// Run via: make test-integration (starts/stops docker-compose.test.yml automatically).
// Requires the test stack to be running: ports 18080 (control), 18081 (auth), 18082 (safety),
// 18085 (fleet), 18883 (mosquitto, TLS since Sprint 40).
package integration_test

import (
	"os"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"avoc/pkg/mqtttls"
)

const (
	controlURL = "http://localhost:18080"
	authURL    = "http://localhost:18081"
	safetyURL  = "http://localhost:18082"
	fleetURL   = "http://localhost:18085"
	jwtSecret  = "test-secret-integration"
)

// Matches the fixed test credentials/CA in tests/docker-compose.test.yml / tests/mosquitto-*
// (MQTTAUTH-01: the test broker rejects anonymous connections; MQTTS-01/02: TLS since Sprint 40).
const (
	mqttTestBroker     = "tls://localhost:18883"
	mqttTestUsername   = "avoc"
	mqttTestPassword   = "mqtt_test_secret"
	mqttTestCACertPath = "../mosquitto-certs/ca.pem"
)

// connectTestMQTTClient builds and connects an MQTT client against the Docker test stack's
// Mosquitto broker — shared by every test in this package that acts as an independent MQTT
// publisher/subscriber alongside the real services (GOSTYLE Rule 3.1: the third call site of this
// exact connect-and-assert shape, see fleet_simulation_test.go / fleet_service_test.go).
func connectTestMQTTClient(t *testing.T, clientID string) mqtt.Client {
	t.Helper()
	tlsConfig, err := mqtttls.LoadClientConfig(mqttTestCACertPath)
	if err != nil {
		t.Fatalf("LoadClientConfig: %v", err)
	}
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(mqttTestBroker).
		SetTLSConfig(tlsConfig).
		SetClientID(clientID).
		SetUsername(mqttTestUsername).
		SetPassword(mqttTestPassword).
		SetConnectTimeout(5 * time.Second))
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("MQTT connect timed out")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("MQTT connect failed: %v", err)
	}
	return client
}

// TestMain allows suite-level setup/teardown if needed.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
