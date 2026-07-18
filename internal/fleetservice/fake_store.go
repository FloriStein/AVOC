package fleetservice

import (
	"fmt"
	"sync"
	"time"

	"avoc/pkg/ulid"
)

// FakeFleetStore is an in-memory FleetStore (ADR-031, HEX-03) — lets Handler-level HTTP tests
// run without a live Postgres connection. Mirrors PostgresFleetStore's externally observable
// behavior (uniqueness/FK/transition validation, sentinel errors, non-nil empty slices,
// ordering) closely enough for that purpose; it does not replicate genuine SQL constraint
// enforcement (CHECK/FK/PK at the DB level) — store_test.go/integration_test.go/
// edgecases_test.go/lifecycle_test.go continue to cover that against a real Postgres container,
// which is what actually exercises it (CLAUDE.MD Abschnitt 17).
type FakeFleetStore struct {
	mu sync.Mutex

	vehicles     map[string]*fakeVehicle
	vehicleOrder []string

	zones     map[string]Zone
	zoneOrder []string

	stations     map[string]Station
	stationOrder []string

	tasks             map[string]Task
	taskOrder         []string
	taskStatusHistory map[string][]TaskStatusHistoryEntry

	vehicleStatus   map[string]VehicleStatus
	positionHistory map[string][]VehiclePositionHistoryPoint

	alerts     map[string]Alert
	alertOrder []string
}

type fakeVehicle struct {
	id          string
	displayName string
	vehicleType *string
}

func NewFakeFleetStore() *FakeFleetStore {
	return &FakeFleetStore{
		vehicles:          make(map[string]*fakeVehicle),
		zones:             make(map[string]Zone),
		stations:          make(map[string]Station),
		tasks:             make(map[string]Task),
		taskStatusHistory: make(map[string][]TaskStatusHistoryEntry),
		vehicleStatus:     make(map[string]VehicleStatus),
		positionHistory:   make(map[string][]VehiclePositionHistoryPoint),
		alerts:            make(map[string]Alert),
	}
}

var _ FleetStore = (*FakeFleetStore)(nil)

// EnsureVehicleExists mirrors the Postgres version's ON CONFLICT DO NOTHING — a pre-existing
// (e.g. admin-set) display_name is never clobbered.
func (f *FakeFleetStore) EnsureVehicleExists(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vehicles[id]; ok {
		return nil
	}
	f.vehicles[id] = &fakeVehicle{id: id, displayName: id}
	f.vehicleOrder = append(f.vehicleOrder, id)
	return nil
}

func (f *FakeFleetStore) SetVehicleType(vehicleID, vehicleType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.vehicles[vehicleID]
	if !ok {
		return fmt.Errorf("fleetservice: vehicle %s not found", vehicleID)
	}
	v.vehicleType = &vehicleType
	return nil
}

func (f *FakeFleetStore) AddZone(z Zone) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.zones[z.ID]; ok {
		return fmt.Errorf("fleetservice: add zone: duplicate id %s", z.ID)
	}
	z.CreatedAt = time.Now()
	f.zones[z.ID] = z
	f.zoneOrder = append(f.zoneOrder, z.ID)
	return nil
}

func (f *FakeFleetStore) ListZones() ([]Zone, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	zones := []Zone{}
	for _, id := range f.zoneOrder {
		zones = append(zones, f.zones[id])
	}
	return zones, nil
}

func (f *FakeFleetStore) AddStation(st Station) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.zones[st.ZoneID]; !ok {
		return fmt.Errorf("fleetservice: add station: unknown zone_id %s", st.ZoneID)
	}
	if _, ok := f.stations[st.ID]; ok {
		return fmt.Errorf("fleetservice: add station: duplicate id %s", st.ID)
	}
	st.CreatedAt = time.Now()
	f.stations[st.ID] = st
	f.stationOrder = append(f.stationOrder, st.ID)
	return nil
}

func (f *FakeFleetStore) ListStations() ([]Station, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stations := []Station{}
	for _, id := range f.stationOrder {
		stations = append(stations, f.stations[id])
	}
	return stations, nil
}

func (f *FakeFleetStore) CreateTask(t Task) (Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vehicles[t.VehicleID]; !ok {
		return Task{}, fmt.Errorf("fleetservice: create task: unknown vehicle_id %s", t.VehicleID)
	}
	if _, ok := f.stations[t.FromStationID]; !ok {
		return Task{}, fmt.Errorf("fleetservice: create task: unknown from_station_id %s", t.FromStationID)
	}
	if _, ok := f.stations[t.ToStationID]; !ok {
		return Task{}, fmt.Errorf("fleetservice: create task: unknown to_station_id %s", t.ToStationID)
	}

	t.ID = ulid.Generate()
	if t.Status == "" {
		t.Status = "pending"
	}
	t.CreatedAt = time.Now()
	f.tasks[t.ID] = t
	f.taskOrder = append(f.taskOrder, t.ID)

	t.AllowedTransitions = allowedTaskTransitions(t.Status)
	return t, nil
}

// ListTasks mirrors the Postgres query's ORDER BY created_at DESC (newest first).
func (f *FakeFleetStore) ListTasks() ([]Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tasks := []Task{}
	for i := len(f.taskOrder) - 1; i >= 0; i-- {
		t := f.tasks[f.taskOrder[i]]
		t.AllowedTransitions = allowedTaskTransitions(t.Status)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// UpdateTaskStatus reuses the package-level taskTransitionSources/allowedTaskTransitions/
// ErrTaskNotFound/ErrInvalidTransition (store.go) — same transition matrix, same sentinel
// errors, so Handler behaves identically regardless of which FleetStore backs it (ADR-030).
func (f *FakeFleetStore) UpdateTaskStatus(id, newStatus, changedBy string) (Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sources, ok := taskTransitionSources[newStatus]
	if !ok {
		return Task{}, ErrInvalidTransition
	}
	t, ok := f.tasks[id]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	allowed := false
	for _, source := range sources {
		if source == t.Status {
			allowed = true
			break
		}
	}
	if !allowed {
		return Task{}, ErrInvalidTransition
	}

	previousStatus := t.Status
	t.Status = newStatus
	t.StatusChangedBy = &changedBy
	if newStatus == "completed" {
		now := time.Now()
		t.CompletedAt = &now
	}
	f.tasks[id] = t

	if t.Status == "completed" || t.Status == "cancelled" {
		if vs, ok := f.vehicleStatus[t.VehicleID]; ok && vs.CurrentTaskID != nil && *vs.CurrentTaskID == t.ID {
			vs.CurrentTaskID = nil
			f.vehicleStatus[t.VehicleID] = vs
		}
	}
	f.taskStatusHistory[t.ID] = append(f.taskStatusHistory[t.ID], TaskStatusHistoryEntry{
		ID: ulid.Generate(), TaskID: t.ID, FromStatus: &previousStatus, ToStatus: t.Status,
		ChangedBy: changedBy, ChangedAt: time.Now(),
	})

	t.AllowedTransitions = allowedTaskTransitions(t.Status)
	return t, nil
}

func (f *FakeFleetStore) GetTaskStatusHistory(taskID string) ([]TaskStatusHistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tasks[taskID]; !ok {
		return nil, ErrTaskNotFound
	}
	entries := append([]TaskStatusHistoryEntry{}, f.taskStatusHistory[taskID]...)
	return entries, nil
}

func (f *FakeFleetStore) UpsertVehicleStatus(vs VehicleStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	vs.UpdatedAt = time.Now()
	f.vehicleStatus[vs.VehicleID] = vs
	return nil
}

// RecordPositionHistory mirrors ADR-033's throttling — a no-op without a position, or if the
// vehicle's last recorded sample is younger than positionHistoryMinInterval (store.go constant,
// reused here so both FleetStore implementations throttle identically).
func (f *FakeFleetStore) RecordPositionHistory(vehicleID string, lat, lon *float64) error {
	if lat == nil || lon == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	points := f.positionHistory[vehicleID]
	if len(points) > 0 && time.Since(points[len(points)-1].RecordedAt) < positionHistoryMinInterval {
		return nil
	}
	f.positionHistory[vehicleID] = append(points, VehiclePositionHistoryPoint{
		VehicleID: vehicleID, PositionLat: *lat, PositionLon: *lon, RecordedAt: time.Now(),
	})
	return nil
}

func (f *FakeFleetStore) GetVehiclePositionHistory(vehicleID string) ([]VehiclePositionHistoryPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vehicles[vehicleID]; !ok {
		return nil, ErrVehicleNotFound
	}
	points := append([]VehiclePositionHistoryPoint{}, f.positionHistory[vehicleID]...)
	return points, nil
}

func (f *FakeFleetStore) PruneVehiclePositionHistory() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().Add(-positionHistoryRetention)
	for id, points := range f.positionHistory {
		kept := []VehiclePositionHistoryPoint{}
		for _, p := range points {
			if p.RecordedAt.After(cutoff) {
				kept = append(kept, p)
			}
		}
		f.positionHistory[id] = kept
	}
	return nil
}

func (f *FakeFleetStore) ListVehicleStatus() ([]VehicleStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	statuses := []VehicleStatus{}
	for _, id := range f.vehicleOrder {
		if vs, ok := f.vehicleStatus[id]; ok {
			statuses = append(statuses, vs)
		}
	}
	return statuses, nil
}

// ListVehiclesWithStatus mirrors the Postgres LEFT JOIN — every registered vehicle appears, with
// all status fields nil if it has never reported a vehicle_status row yet.
func (f *FakeFleetStore) ListVehiclesWithStatus() ([]FleetVehicle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	vehicles := []FleetVehicle{}
	for _, id := range f.vehicleOrder {
		v := f.vehicles[id]
		fv := FleetVehicle{ID: v.id, DisplayName: v.displayName, VehicleType: v.vehicleType}
		if vs, ok := f.vehicleStatus[id]; ok {
			autonomy := vs.AutonomyMode
			updatedAt := vs.UpdatedAt
			fv.BatteryPct = vs.BatteryPct
			fv.Speed = vs.Speed
			fv.PositionLat = vs.PositionLat
			fv.PositionLon = vs.PositionLon
			fv.PositionZoneID = vs.PositionZoneID
			fv.PositionX = vs.PositionX
			fv.PositionY = vs.PositionY
			fv.AutonomyMode = &autonomy
			fv.CurrentTaskID = vs.CurrentTaskID
			fv.StatusUpdatedAt = &updatedAt
		}
		vehicles = append(vehicles, fv)
	}
	return vehicles, nil
}

func (f *FakeFleetStore) CreateAlert(a Alert) (Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.ID = ulid.Generate()
	a.CreatedAt = time.Now()
	f.alerts[a.ID] = a
	f.alertOrder = append(f.alertOrder, a.ID)
	return a, nil
}

// ListAlerts mirrors the Postgres query's ORDER BY created_at DESC (newest first).
func (f *FakeFleetStore) ListAlerts() ([]Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	alerts := []Alert{}
	for i := len(f.alertOrder) - 1; i >= 0; i-- {
		alerts = append(alerts, f.alerts[f.alertOrder[i]])
	}
	return alerts, nil
}

func (f *FakeFleetStore) AcknowledgeAlert(id, acknowledgedBy string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.alerts[id]
	if !ok {
		return fmt.Errorf("fleetservice: alert %s not found", id)
	}
	now := time.Now()
	a.AcknowledgedAt = &now
	a.AcknowledgedBy = &acknowledgedBy
	f.alerts[id] = a
	return nil
}
