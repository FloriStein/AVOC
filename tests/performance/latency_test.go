// Package performance measures ACK-Roundtrip latency for the Control Loop (ADR-006/010).
// Run against the test stack: make test-latency
// CI Build-Fail when p99 > 100ms (ADR-010: <100ms hard requirement).
package performance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	testAuthURL    = "http://localhost:18081"
	testControlURL = "http://localhost:18080"
	testJWTSecret  = "test-secret-integration"
	latencyBudget  = 100 * time.Millisecond // ADR-010 hard requirement
)

// loginOperator authenticates against the seeded admin account (ADMIN_PASSWORD=admin_test_secret
// in docker-compose.test.yml) — there is no "accept any" auth anymore since ADR-024.
func loginOperator(t testing.TB, username string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": "admin_test_secret"})
	resp, err := http.Post(testAuthURL+"/auth/operator/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("auth login failed: %v", err)
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	token, _ := m["token"].(string)
	return token
}

// startSession creates a session for vehicleID and returns the session_id from
// the /session/start response — required as a WS query parameter (ADR-025,
// see authenticateWS in internal/controlserver/transport/websocket.go).
func startSession(t testing.TB, vehicleID, operatorID, token string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"vehicle_id":    vehicleID,
		"operator_id":   operatorID,
		"operator_role": "ACTIVE_OPERATOR",
	})
	req, _ := http.NewRequest(http.MethodPost, testControlURL+"/session/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("session/start failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session/start returned %d", resp.StatusCode)
	}
	var m struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("session/start response decode failed: %v", err)
	}
	if m.SessionID == "" {
		t.Fatalf("session/start response missing session_id")
	}
	return m.SessionID
}

// endSession releases the vehicle lock via /session/end. Without this, the Go
// benchmark harness's calibration re-invocations of BenchmarkControlACKRoundtrip
// (increasing b.N until -benchtime elapses) each call startSession again for the
// same vehicle — since the previous session was never ended, StartSession sees
// the vehicle still locked and hands out an OBSERVER session instead of
// ACTIVE_OPERATOR. OBSERVER commands are never ACKed, so conn.ReadMessage()
// blocks forever on the next calibration run.
func endSession(t testing.TB, sessionID, token string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"session_id": sessionID})
	req, _ := http.NewRequest(http.MethodPost, testControlURL+"/session/end", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("session/end failed (non-fatal, cleanup only): %v", err)
		return
	}
	resp.Body.Close()
}

// BenchmarkControlACKRoundtrip measures the WebSocket ACK roundtrip for a
// Protobuf DEADMAN_HOLD command (field 2 = type 6, minimal valid message).
// CI Build-Fail: p99 must stay < 100ms (ADR-010).
func BenchmarkControlACKRoundtrip(b *testing.B) {
	token := loginOperator(b, "admin")
	if token == "" {
		b.Skip("auth service not available — start test stack with: make test-integration")
	}

	sessionID := startSession(b, "vehicle-int-mock", "admin", token)
	defer endSession(b, sessionID, token)

	wsURL := fmt.Sprintf("ws://localhost:18080/ws?token=%s&session_id=%s", token, sessionID)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Skipf("WebSocket not available (start test stack): %v", err)
	}
	defer conn.Close()

	time.Sleep(300 * time.Millisecond)

	// Minimal Protobuf ControlCommand: field 2 (type=DEADMAN_HOLD=6) as varint
	cmdDeadmanHold := []byte{0x10, 0x06}

	latencies := make([]time.Duration, 0, b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		if err := conn.WriteMessage(websocket.BinaryMessage, cmdDeadmanHold); err != nil {
			b.Fatalf("write error: %v", err)
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			b.Fatalf("read ACK error: %v", err)
		}
		latencies = append(latencies, time.Since(t0))
	}

	b.StopTimer()

	// Compute percentiles
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[int(math.Min(float64(len(latencies)*99/100), float64(len(latencies)-1)))]

	b.ReportMetric(float64(p50.Milliseconds()), "p50_ms")
	b.ReportMetric(float64(p95.Milliseconds()), "p95_ms")
	b.ReportMetric(float64(p99.Milliseconds()), "p99_ms")

	// CI Build-Fail guard — ADR-010: <100ms is non-negotiable
	if p99 > latencyBudget {
		b.Fatalf("LATENCY BUDGET EXCEEDED: p99=%v > %v (ADR-010)", p99, latencyBudget)
	}

	b.Logf("ACK Roundtrip — p50=%v p95=%v p99=%v (budget=%v) ✅",
		p50.Round(time.Millisecond),
		p95.Round(time.Millisecond),
		p99.Round(time.Millisecond),
		latencyBudget)
}

// TestLatencyBudget_DocumentedRequirement verifies the budget constant matches ADR-010.
func TestLatencyBudget_DocumentedRequirement(t *testing.T) {
	if latencyBudget != 100*time.Millisecond {
		t.Fatalf("latency budget must be 100ms per ADR-010, got %v", latencyBudget)
	}
}
