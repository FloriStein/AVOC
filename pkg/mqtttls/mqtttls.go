// Package mqtttls builds the tls.Config the three MQTT connections (telemetry-service,
// fleetgateway, vehicle-mock) need to verify Mosquitto's server certificate against the
// project-owned CA (MQTTS-03, Sprint 40). Shared here per GOSTYLE Rule 3.1 — the same
// "read CA file, build a cert pool" logic would otherwise appear a third time.
package mqtttls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// LoadClientConfig reads the PEM-encoded CA certificate at caCertPath and returns a tls.Config
// that trusts it — no InsecureSkipVerify, so a misconfigured or missing CA fails the connection
// instead of silently falling back to an unverified one (CLAUDE.MD §0).
func LoadClientConfig(caCertPath string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("mqtttls: read CA cert %s: %w", caCertPath, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("mqtttls: no valid certificate found in %s", caCertPath)
	}

	return &tls.Config{RootCAs: pool}, nil
}
