// Package telemetrycheck gives the Control Server a poll-based view into telemetry-service's
// per-vehicle telemetry freshness (ADR-009 Update 2026-07-20/21, DRIFT-K3-TELEMETRY Teil 2).
//
// telemetry-service itself stays fully unaware of sessions/vehicles (no new coupling in the
// other direction) — this package only reads the existing GET /telemetry/latest/{vehicleID}
// endpoint (BE-05), same "poll an existing health-ish endpoint" pattern as SafetyBusWatchdog
// against safety-service, rather than AuthWatchdog's direct-DB-read pattern (authcheck) — there is
// no shared database between control-server and telemetry-service to read instead.
package telemetrycheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Checker polls telemetry-service for the freshness of a vehicle's latest telemetry event.
type Checker struct {
	baseURL string
	client  *http.Client
}

// NewChecker builds a Checker against telemetry-service's base URL (e.g. TELEMETRY_SERVICE_URL).
// 3s HTTP timeout, same value as SafetyBusWatchdog's client (ADR-009 Update 2026-07-21 Grill-Me).
func NewChecker(baseURL string) *Checker {
	return &Checker{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 3 * time.Second},
	}
}

// HasFreshTelemetry reports whether telemetry-service has a telemetry event for vehicleID no
// older than maxAge. Three outcomes collapse into "not fresh" (false, nil error) because
// TelemetryWatchdog treats them identically — a single consecutive-failure counter, no special
// casing per cause (ADR-009 Update 2026-07-21 Grill-Me):
//   - HTTP 404 ("never received", or not received since telemetry-service's own process start) —
//     not an error, same idiom as authcheck.IsActiveOperator's sql.ErrNoRows handling.
//   - HTTP 200 with a timestamp older than maxAge — telemetry-service's cache is keyed only by
//     vehicleID and persists across sessions (internal/telemetryservice/client.go), so a stale
//     value from a previous session must not be mistaken for a live one in the current session.
//   - Network error / timeout / unexpected status other than 404 — returned as an error, but the
//     caller (TelemetryWatchdog) counts it the same way as the two cases above.
func (c *Checker) HasFreshTelemetry(ctx context.Context, vehicleID string, maxAge time.Duration) (bool, error) {
	reqURL := c.baseURL + "/telemetry/latest/" + url.PathEscape(vehicleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("telemetrycheck: build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("telemetrycheck: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("telemetrycheck: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("telemetrycheck: decode response: %w", err)
	}

	age := time.Duration(time.Now().UnixMilli()-body.Timestamp) * time.Millisecond
	return age <= maxAge, nil
}
