package telemetrycheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHasFreshTelemetry_FreshTimestamp_ReturnsTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"vehicle_id":"v1","timestamp":` + fmt.Sprint(time.Now().UnixMilli()) + `}`))
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	fresh, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Fatal("expected fresh telemetry to report true")
	}
}

func TestHasFreshTelemetry_StaleTimestamp_ReturnsFalse(t *testing.T) {
	staleMs := time.Now().Add(-10 * time.Second).UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"vehicle_id":"v1","timestamp":` + fmt.Sprint(staleMs) + `}`))
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	fresh, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Fatal("expected stale telemetry (cross-session leftover) to report false, not an error")
	}
}

func TestHasFreshTelemetry_NeverReceived404_ReturnsFalseNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no telemetry for vehicle", http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	fresh, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err != nil {
		t.Fatalf("404 must not be treated as an error (analog authcheck sql.ErrNoRows), got: %v", err)
	}
	if fresh {
		t.Fatal("expected 404 (never received) to report false")
	}
}

func TestHasFreshTelemetry_UnexpectedStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	fresh, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err == nil {
		t.Fatal("expected an error for unexpected non-404 status")
	}
	if fresh {
		t.Fatal("expected false alongside the error")
	}
}

func TestHasFreshTelemetry_MalformedJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	_, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err == nil {
		t.Fatal("expected a decode error for malformed JSON")
	}
}

func TestHasFreshTelemetry_UnreachableServer_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed before use — connection refused

	c := NewChecker(srv.URL)
	fresh, err := c.HasFreshTelemetry(context.Background(), "v1", 4*time.Second)
	if err == nil {
		t.Fatal("expected a network error for an unreachable telemetry-service")
	}
	if fresh {
		t.Fatal("expected false alongside the error")
	}
}

func TestHasFreshTelemetry_VehicleIDIsPathEscaped(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		http.Error(w, "no telemetry for vehicle", http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	_, _ = c.HasFreshTelemetry(context.Background(), "vehicle/weird id", 4*time.Second)
	if gotPath != "/telemetry/latest/vehicle%2Fweird%20id" {
		t.Fatalf("expected path-escaped vehicle_id on the wire, got %q", gotPath)
	}
}
