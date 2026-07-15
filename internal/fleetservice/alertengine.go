package fleetservice

import (
	"fmt"
	"sync"
)

// Battery tiers, ordered worst-last so a plain int comparison tells "got worse" from "got
// better" (FLEET-07 — example threshold rule; more rule types can be added later as additional
// tier dimensions without changing AlertEngine's shape).
const (
	batteryTierNormal = iota
	batteryTierWarning
	batteryTierCritical
)

// Thresholds intentionally asymmetric (enter/clear differ) — a hysteresis gap so a vehicle
// sitting exactly at the boundary doesn't flap in and out of alerting on every status tick.
const (
	batteryWarningThreshold  = 20.0 // below this: enter/stay warning
	batteryWarningClear      = 25.0 // at/above this: clear back to normal
	batteryCriticalThreshold = 10.0 // below this: enter/stay critical
	batteryCriticalClear     = 15.0 // at/above this: clear back to warning
)

// batteryTier computes the next tier from batteryPct and the vehicle's current tier — the
// current tier matters because the enter and clear thresholds differ (hysteresis).
func batteryTier(batteryPct float64, current int) int {
	switch current {
	case batteryTierCritical:
		if batteryPct < batteryCriticalClear {
			return batteryTierCritical
		}
		if batteryPct < batteryWarningClear {
			return batteryTierWarning
		}
		return batteryTierNormal
	case batteryTierWarning:
		if batteryPct < batteryCriticalThreshold {
			return batteryTierCritical
		}
		if batteryPct < batteryWarningClear {
			return batteryTierWarning
		}
		return batteryTierNormal
	default:
		if batteryPct < batteryCriticalThreshold {
			return batteryTierCritical
		}
		if batteryPct < batteryWarningThreshold {
			return batteryTierWarning
		}
		return batteryTierNormal
	}
}

// AlertEngine raises threshold-based alerts from a vehicle's own reported status (FLEET-07) —
// deliberately separate from vehicle-initiated alerts (ADR-028: a vehicle detecting its own
// problem and reporting it via FleetGateway's alert channel), which arrive through a completely
// different path (gw.SubscribeVehicleAlerts in cmd/fleet-service/main.go) and never touch this
// type. Both end up as ordinary rows in the same `alerts` table (ADR-028 — one shape, two
// sources), so the Dashboard doesn't need to know which one produced any given alert.
type AlertEngine struct {
	mu   sync.Mutex
	tier map[string]int // vehicleID -> current battery tier
}

func NewAlertEngine() *AlertEngine {
	return &AlertEngine{tier: make(map[string]int)}
}

// Evaluate returns an Alert to raise if status's battery level crosses into a worse tier than
// last observed for this vehicle, or nil if nothing new should be raised — covers both "already
// alerting at this tier" (no repeat alert on every status tick) and "recovering" (no alert for
// getting better). A status with no battery reading (BatteryPct == nil, e.g. a vehicle that
// hasn't reported telemetry yet) never raises an alert.
func (e *AlertEngine) Evaluate(status VehicleStatus) *Alert {
	if status.BatteryPct == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	current := e.tier[status.VehicleID]
	next := batteryTier(*status.BatteryPct, current)
	e.tier[status.VehicleID] = next

	if next <= current {
		return nil
	}

	var severity, message string
	switch next {
	case batteryTierWarning:
		severity = "warning"
		message = fmt.Sprintf("Batterie schwach (%.1f%%)", *status.BatteryPct)
	case batteryTierCritical:
		severity = "critical"
		message = fmt.Sprintf("Batterie kritisch (%.1f%%)", *status.BatteryPct)
	default:
		return nil
	}

	return &Alert{VehicleID: status.VehicleID, Severity: severity, Message: message}
}
