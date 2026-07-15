package fleetservice

import (
	"sync"
	"testing"
)

func battery(pct float64) VehicleStatus {
	b := pct
	return VehicleStatus{VehicleID: "v1", BatteryPct: &b}
}

func TestAlertEngine_HighBattery_NoAlert(t *testing.T) {
	e := NewAlertEngine()
	if alert := e.Evaluate(battery(80.0)); alert != nil {
		t.Fatalf("expected no alert, got %+v", alert)
	}
}

func TestAlertEngine_NilBattery_NoAlert(t *testing.T) {
	e := NewAlertEngine()
	if alert := e.Evaluate(VehicleStatus{VehicleID: "v1"}); alert != nil {
		t.Fatalf("expected no alert for nil BatteryPct, got %+v", alert)
	}
}

func TestAlertEngine_DropBelowWarning_RaisesWarningAlert(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(50.0))

	alert := e.Evaluate(battery(15.0))
	if alert == nil {
		t.Fatal("expected a warning alert")
	}
	if alert.Severity != "warning" || alert.VehicleID != "v1" {
		t.Fatalf("unexpected alert: %+v", alert)
	}
}

func TestAlertEngine_StaysAtSameTier_DoesNotRepeatAlert(t *testing.T) {
	e := NewAlertEngine()
	if alert := e.Evaluate(battery(15.0)); alert == nil {
		t.Fatal("expected first warning alert")
	}

	// Multiple subsequent ticks at/around the same tier must not re-alert — this is the whole
	// point of tracking tier state (otherwise every ~2s status tick would spam a fresh alert).
	for _, pct := range []float64{14.0, 16.0, 12.0, 13.5} {
		if alert := e.Evaluate(battery(pct)); alert != nil {
			t.Fatalf("expected no repeat alert while still in warning tier at %.1f%%, got %+v", pct, alert)
		}
	}
}

func TestAlertEngine_DropBelowCritical_RaisesCriticalAlert(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // enters warning first

	alert := e.Evaluate(battery(5.0))
	if alert == nil {
		t.Fatal("expected a critical alert")
	}
	if alert.Severity != "critical" {
		t.Fatalf("expected critical severity, got %+v", alert)
	}
}

func TestAlertEngine_DirectDropToCritical_SkipsWarningRaisesCriticalOnly(t *testing.T) {
	e := NewAlertEngine()
	// A single tick straight from healthy to critical (plausible: infrequent status updates,
	// or a fast-draining lastenrad) must raise exactly the critical alert, not a spurious
	// intermediate warning.
	alert := e.Evaluate(battery(5.0))
	if alert == nil || alert.Severity != "critical" {
		t.Fatalf("expected critical alert on direct drop, got %+v", alert)
	}
}

func TestAlertEngine_RecoveryPastClearThreshold_NoAlertButResetsForNextDrop(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // warning

	// Partial recovery (>= criticalClear but < warningClear) must not fire an alert — recovery
	// is silent by design (FLEET-07 doesn't alert on "getting better").
	if alert := e.Evaluate(battery(22.0)); alert != nil {
		t.Fatalf("expected no alert on partial recovery, got %+v", alert)
	}

	// Full recovery past the warning-clear threshold.
	if alert := e.Evaluate(battery(30.0)); alert != nil {
		t.Fatalf("expected no alert on full recovery, got %+v", alert)
	}

	// Re-dropping below the warning threshold after a full recovery must raise a fresh alert —
	// proves tier state was actually reset to normal, not stuck.
	alert := e.Evaluate(battery(18.0))
	if alert == nil || alert.Severity != "warning" {
		t.Fatalf("expected a fresh warning alert after full recovery and re-drop, got %+v", alert)
	}
}

func TestAlertEngine_HysteresisAtWarningBoundary_DoesNotFlap(t *testing.T) {
	e := NewAlertEngine()
	if alert := e.Evaluate(battery(19.9)); alert == nil {
		t.Fatal("expected warning alert crossing below 20%")
	}

	// Bouncing right around the 20% entry threshold (but below the 25% clear threshold) must
	// not flip back to "normal" and then re-alert — that's exactly what the clear/enter gap
	// (hysteresis) exists to prevent.
	for _, pct := range []float64{20.5, 19.8, 21.0, 19.5} {
		if alert := e.Evaluate(battery(pct)); alert != nil {
			t.Fatalf("expected no flapping alert at %.1f%% (within hysteresis band), got %+v", pct, alert)
		}
	}
}

func TestAlertEngine_CriticalRecoveryToWarning_ThenDropsAgain_RaisesCriticalAgain(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // warning
	e.Evaluate(battery(5.0))  // critical

	// Recovers out of critical but not out of warning — no alert (recovery is silent).
	if alert := e.Evaluate(battery(16.0)); alert != nil {
		t.Fatalf("expected no alert recovering critical->warning, got %+v", alert)
	}

	// Drops back into critical — must alert again, proving the critical-clear threshold (not
	// just "any improvement") is what actually resets the tier.
	alert := e.Evaluate(battery(8.0))
	if alert == nil || alert.Severity != "critical" {
		t.Fatalf("expected a fresh critical alert after re-drop, got %+v", alert)
	}
}

func TestAlertEngine_IndependentPerVehicle(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(VehicleStatus{VehicleID: "v1", BatteryPct: floatPtr(5.0)}) // v1 critical

	// v2 starting fresh at a healthy level must not be affected by v1's tier state.
	if alert := e.Evaluate(VehicleStatus{VehicleID: "v2", BatteryPct: floatPtr(80.0)}); alert != nil {
		t.Fatalf("expected no alert for unrelated healthy vehicle v2, got %+v", alert)
	}

	// v2 dropping into warning must still raise its own alert independently of v1.
	alert := e.Evaluate(VehicleStatus{VehicleID: "v2", BatteryPct: floatPtr(15.0)})
	if alert == nil || alert.VehicleID != "v2" {
		t.Fatalf("expected an independent warning alert for v2, got %+v", alert)
	}
}

// ─── Grenzwerte (CLAUDE.MD Abschnitt 17) ───────────────────────────────────────

func TestAlertEngine_ExactlyAtWarningThreshold_StaysNormal(t *testing.T) {
	// batteryTier uses a strict "<" comparison — exactly 20.0% must NOT enter warning (only
	// values strictly below do). Locks in the boundary semantics explicitly instead of leaving
	// it implicit in the threshold constants.
	e := NewAlertEngine()
	if alert := e.Evaluate(battery(batteryWarningThreshold)); alert != nil {
		t.Fatalf("expected no alert exactly at the warning threshold (%.1f%%), got %+v", batteryWarningThreshold, alert)
	}
}

func TestAlertEngine_JustBelowWarningThreshold_RaisesWarning(t *testing.T) {
	e := NewAlertEngine()
	alert := e.Evaluate(battery(batteryWarningThreshold - 0.1))
	if alert == nil || alert.Severity != "warning" {
		t.Fatalf("expected warning alert just below the threshold, got %+v", alert)
	}
}

func TestAlertEngine_ExactlyAtCriticalThreshold_StaysWarningNotCritical(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // enter warning first

	if alert := e.Evaluate(battery(batteryCriticalThreshold)); alert != nil {
		t.Fatalf("expected no alert exactly at the critical threshold (%.1f%%) — must stay warning, not enter critical, got %+v", batteryCriticalThreshold, alert)
	}
}

func TestAlertEngine_ExactlyAtWarningClear_RecoversToNormal(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // warning
	e.Evaluate(battery(batteryWarningClear))

	// Re-dropping just below the warning threshold must raise a fresh alert only if tier was
	// actually reset to normal by the exact-clear-threshold recovery above.
	alert := e.Evaluate(battery(19.0))
	if alert == nil || alert.Severity != "warning" {
		t.Fatalf("expected tier to have been cleared to normal exactly at %.1f%%, got %+v after re-drop", batteryWarningClear, alert)
	}
}

func TestAlertEngine_ExactlyAtCriticalClear_RecoversToWarningNotNormal(t *testing.T) {
	e := NewAlertEngine()
	e.Evaluate(battery(15.0)) // warning
	e.Evaluate(battery(5.0))  // critical
	e.Evaluate(battery(batteryCriticalClear))

	// If the exact-clear recovery had (incorrectly) jumped all the way to normal instead of
	// warning, this tick at 18% (below warningThreshold, above warningClear) would raise a fresh
	// warning alert. It must not, because the vehicle should already be tracked as "warning".
	if alert := e.Evaluate(battery(18.0)); alert != nil {
		t.Fatalf("expected tier to have cleared to warning (not normal) exactly at %.1f%%, got unexpected re-alert %+v", batteryCriticalClear, alert)
	}
}

func TestAlertEngine_ZeroBattery_RaisesCriticalAlert(t *testing.T) {
	e := NewAlertEngine()
	alert := e.Evaluate(battery(0.0))
	if alert == nil || alert.Severity != "critical" {
		t.Fatalf("expected critical alert at 0%% battery, got %+v", alert)
	}
}

func TestAlertEngine_FullBattery_NoAlert(t *testing.T) {
	e := NewAlertEngine()
	if alert := e.Evaluate(battery(100.0)); alert != nil {
		t.Fatalf("expected no alert at 100%% battery, got %+v", alert)
	}
}

func TestAlertEngine_NegativeBattery_RaisesCriticalWithoutPanicking(t *testing.T) {
	// Physically impossible (a real battery can't report negative charge), but the engine must
	// degrade gracefully rather than panic or misclassify if a malformed/buggy upstream ever
	// sends one — negative is still "worse than critical", so it must alert, not silently no-op.
	e := NewAlertEngine()
	alert := e.Evaluate(battery(-5.0))
	if alert == nil || alert.Severity != "critical" {
		t.Fatalf("expected critical alert for negative battery reading, got %+v", alert)
	}
}

func TestAlertEngine_EmptyVehicleID_StillTracksTierIndependently(t *testing.T) {
	// An empty string is a valid Go map key — must not panic, and must not be silently conflated
	// with any other vehicle's state.
	e := NewAlertEngine()
	alert := e.Evaluate(VehicleStatus{VehicleID: "", BatteryPct: floatPtr(5.0)})
	if alert == nil || alert.VehicleID != "" {
		t.Fatalf("expected a critical alert for the empty-ID vehicle, got %+v", alert)
	}

	// A named vehicle must still be evaluated independently of the empty-ID one.
	other := e.Evaluate(battery(80.0))
	if other != nil {
		t.Fatalf("expected empty-ID vehicle's state not to leak into vehicle %q, got %+v", "v1", other)
	}
}

func TestAlertEngine_ConcurrentEvaluate_NoRace(t *testing.T) {
	e := NewAlertEngine()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vehicleID := "v" + string(rune('a'+i%5))
			for tick := 0; tick < 10; tick++ {
				e.Evaluate(VehicleStatus{VehicleID: vehicleID, BatteryPct: floatPtr(float64(tick))})
			}
		}(i)
	}
	wg.Wait()
}

func floatPtr(f float64) *float64 { return &f }
