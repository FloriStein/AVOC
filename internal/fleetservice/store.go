// Package fleetservice manages the Fleet domain (zones, stations, tasks, alerts, live vehicle
// status) introduced by ADR-029. It shares the `vehicles` identity table owned by
// internal/vehicleregistry (control-server) via foreign keys — no duplicate vehicle registry.
package fleetservice

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"avoc/pkg/ulid"
)

// vehicleBaseTable mirrors internal/vehicleregistry's schema exactly (CREATE TABLE IF NOT
// EXISTS is a no-op if control-server already created it). Without this, fleet-service starting
// before control-server would fail: `ALTER TABLE vehicles ...` errors if the table doesn't
// exist yet, and docker-compose only orders both services after Postgres, not after each other
// (found via tests/integration — fleet-service and control-server have no depends_on between
// them, so either start order must work, ADR-029).
const vehicleBaseTable = `
CREATE TABLE IF NOT EXISTS vehicles (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// vehicleTypeColumn extends the `vehicles` table (owned by control-server/vehicleregistry for
// id/display_name) with the fleet-specific vehicle_type column (ADR-029). Using
// ADD COLUMN IF NOT EXISTS keeps this idempotent regardless of which service starts first.
const vehicleTypeColumn = `ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS vehicle_type TEXT;`

// taskStatusChangedByColumn extends the `tasks` table with the operator-attribution column for
// manual status transitions (ADR-030). Same idempotent-ALTER pattern as vehicleTypeColumn — the
// column is also declared directly in `schema`'s CREATE TABLE below for fresh installs, but this
// ALTER is required for the already-running dev DB where `tasks` predates ADR-030.
const taskStatusChangedByColumn = `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS status_changed_by TEXT;`

const schema = `
CREATE TABLE IF NOT EXISTS zones (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    environment  TEXT NOT NULL CHECK (environment IN ('indoor', 'outdoor')),
    svg_geometry TEXT NOT NULL DEFAULT '',
    geo_bounds   JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stations (
    id            TEXT PRIMARY KEY,
    zone_id       TEXT NOT NULL REFERENCES zones(id),
    name          TEXT NOT NULL,
    position_x    DOUBLE PRECISION,
    position_y    DOUBLE PRECISION,
    position_lat  DOUBLE PRECISION,
    position_lon  DOUBLE PRECISION,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id               TEXT PRIMARY KEY,
    vehicle_id       TEXT NOT NULL REFERENCES vehicles(id),
    from_station_id  TEXT NOT NULL REFERENCES stations(id),
    to_station_id    TEXT NOT NULL REFERENCES stations(id),
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'cancelled')),
    priority         INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ,
    status_changed_by TEXT
);

CREATE TABLE IF NOT EXISTS vehicle_status (
    vehicle_id      TEXT PRIMARY KEY REFERENCES vehicles(id),
    battery_pct     REAL,
    speed           REAL,
    position_lat    DOUBLE PRECISION,
    position_lon    DOUBLE PRECISION,
    position_zone_id TEXT REFERENCES zones(id),
    autonomy_mode   TEXT NOT NULL DEFAULT 'autonomous' CHECK (autonomy_mode IN ('autonomous', 'teleoperated', 'manual')),
    current_task_id TEXT REFERENCES tasks(id),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alerts (
    id              TEXT PRIMARY KEY,
    vehicle_id      TEXT NOT NULL REFERENCES vehicles(id),
    severity        TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    message         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by TEXT
);
`

// taskuiDemoSeedCleanup removes the `-taskui`-suffixed placeholder Zone/Stations that Sprint 24
// used to unblock the Task-creation UI before any real zones/stations existed in the dev DB
// (ADR-030). The parallel Sprint 23 (map/zone visualization) session has since provided real
// seed data (`scripts/seed-fleet-demo.sh`, MAP-01, zone `zone-betriebshof-nord`), making the
// `-taskui` placeholder a redundant duplicate (TASKUI-01). Runs on every startup like the
// ALTER-based migrations above; the NOT EXISTS guards make it a no-op both once the rows are
// gone and in the (unexpected) case a real Task still references a `-taskui` station — deleting
// would otherwise fail the tasks.from_station_id/to_station_id FK and break service startup.
const taskuiDemoSeedCleanup = `
DELETE FROM stations
WHERE id LIKE '%-taskui'
  AND NOT EXISTS (
    SELECT 1 FROM tasks
    WHERE tasks.from_station_id = stations.id OR tasks.to_station_id = stations.id
  );

DELETE FROM zones
WHERE id LIKE '%-taskui'
  AND NOT EXISTS (SELECT 1 FROM stations WHERE stations.zone_id = zones.id);
`

// Zone is an admin-managed spatial area (AP3 "Räumliche Zonenzuweisung") — indoor (SVG-based)
// or outdoor (geo-referenced SVG overlay, ADR-029).
type Zone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Environment string    `json:"environment"` // "indoor" | "outdoor"
	SVGGeometry string    `json:"svg_geometry,omitempty"`
	GeoBounds   *string   `json:"geo_bounds,omitempty"` // raw JSON, nil for indoor zones without geo-reference
	CreatedAt   time.Time `json:"created_at"`
}

// Station is a map-anchored point — the endpoint unit for Tasks.
type Station struct {
	ID          string    `json:"id"`
	ZoneID      string    `json:"zone_id"`
	Name        string    `json:"name"`
	PositionX   *float64  `json:"position_x,omitempty"`
	PositionY   *float64  `json:"position_y,omitempty"`
	PositionLat *float64  `json:"position_lat,omitempty"`
	PositionLon *float64  `json:"position_lon,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Task represents a vehicle moving between two Stations (ADR-029 — first-version task model,
// no multi-step job/loading model yet).
type Task struct {
	ID              string     `json:"id"`
	VehicleID       string     `json:"vehicle_id"`
	FromStationID   string     `json:"from_station_id"`
	ToStationID     string     `json:"to_station_id"`
	Status          string     `json:"status"` // "pending" | "in_progress" | "completed" | "cancelled"
	Priority        int        `json:"priority"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	StatusChangedBy *string    `json:"status_changed_by,omitempty"` // ADR-030 — set on manual PATCH transitions, nil until the first one
	// AllowedTransitions lists the target statuses reachable from Status, derived from
	// taskTransitionSources below (TASKUI-02) — lets the frontend render the correct
	// status-change buttons without duplicating the transition matrix itself. Never nil (see
	// allowedTaskTransitions), so it marshals to `[]` rather than `null` for terminal statuses.
	AllowedTransitions []string `json:"allowed_transitions"`
}

// VehicleStatus is the live fleet telemetry for one vehicle — separate from the vehicles
// identity table because it's updated at high frequency (ADR-029).
type VehicleStatus struct {
	VehicleID      string    `json:"vehicle_id"`
	BatteryPct     *float64  `json:"battery_pct,omitempty"`
	Speed          *float64  `json:"speed,omitempty"`
	PositionLat    *float64  `json:"position_lat,omitempty"`
	PositionLon    *float64  `json:"position_lon,omitempty"`
	PositionZoneID *string   `json:"position_zone_id,omitempty"`
	AutonomyMode   string    `json:"autonomy_mode"` // "autonomous" | "teleoperated" | "manual"
	CurrentTaskID  *string   `json:"current_task_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Alert is a fleet-service-generated or vehicle-initiated notification (ADR-028: two sources,
// same table/shape — threshold-based from fleet-service, or forwarded verbatim from a
// vehicle-initiated "needs intervention" event).
type Alert struct {
	ID             string     `json:"id"`
	VehicleID      string     `json:"vehicle_id"`
	Severity       string     `json:"severity"` // "info" | "warning" | "critical"
	Message        string     `json:"message"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string    `json:"acknowledged_by,omitempty"`
}

// PostgresFleetStore persists the Fleet domain in the shared `avoc` Postgres database
// (ADR-023/029) — own connection pool, same DB as control-server/auth-service.
type PostgresFleetStore struct {
	db *sql.DB
}

func NewPostgresFleetStore(db *sql.DB) (*PostgresFleetStore, error) {
	if _, err := db.Exec(vehicleBaseTable); err != nil {
		return nil, fmt.Errorf("fleetservice: create base vehicles table: %w", err)
	}
	if _, err := db.Exec(vehicleTypeColumn); err != nil {
		return nil, fmt.Errorf("fleetservice: extend vehicles table: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("fleetservice: create schema: %w", err)
	}
	// Runs after `schema` — taskStatusChangedByColumn ALTERs a table `schema` just created (or
	// that already existed pre-ADR-030); ordering here mirrors vehicleTypeColumn/vehicleBaseTable
	// above (create-then-extend).
	if _, err := db.Exec(taskStatusChangedByColumn); err != nil {
		return nil, fmt.Errorf("fleetservice: extend tasks table: %w", err)
	}
	if _, err := db.Exec(taskuiDemoSeedCleanup); err != nil {
		return nil, fmt.Errorf("fleetservice: clean up taskui demo seed: %w", err)
	}
	return &PostgresFleetStore{db: db}, nil
}

// EnsureVehicleExists auto-registers a bare vehicle identity row (id + a display_name defaulted
// to id) the first time fleet-service sees a vehicle — mirroring control-server's own
// "Auto-Register bei erstem WS-Connect" pattern (ADR-029), but for vehicles that only ever speak
// MQTT and never establish a WS connection to control-server. ON CONFLICT DO NOTHING: an admin
// (or control-server) may have already set a friendlier display_name, which this must not clobber.
func (s *PostgresFleetStore) EnsureVehicleExists(id string) error {
	_, err := s.db.Exec(
		`INSERT INTO vehicles (id, display_name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`,
		id,
	)
	if err != nil {
		return fmt.Errorf("fleetservice: ensure vehicle exists: %w", err)
	}
	return nil
}

// SetVehicleType sets the fleet-specific vehicle_type column on the shared vehicles table
// (ADR-029 — fleet-service owns this column, control-server owns id/display_name).
func (s *PostgresFleetStore) SetVehicleType(vehicleID, vehicleType string) error {
	res, err := s.db.Exec(`UPDATE vehicles SET vehicle_type = $1 WHERE id = $2`, vehicleType, vehicleID)
	if err != nil {
		return fmt.Errorf("fleetservice: set vehicle type: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("fleetservice: vehicle %s not found", vehicleID)
	}
	return nil
}

func (s *PostgresFleetStore) AddZone(z Zone) error {
	_, err := s.db.Exec(
		`INSERT INTO zones (id, name, environment, svg_geometry, geo_bounds) VALUES ($1, $2, $3, $4, $5)`,
		z.ID, z.Name, z.Environment, z.SVGGeometry, z.GeoBounds,
	)
	if err != nil {
		return fmt.Errorf("fleetservice: add zone: %w", err)
	}
	return nil
}

func (s *PostgresFleetStore) ListZones() ([]Zone, error) {
	rows, err := s.db.Query(`SELECT id, name, environment, svg_geometry, geo_bounds, created_at FROM zones ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list zones: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows — see ListTasks for why (TASKUI-04).
	zones := []Zone{}
	for rows.Next() {
		var z Zone
		if err := rows.Scan(&z.ID, &z.Name, &z.Environment, &z.SVGGeometry, &z.GeoBounds, &z.CreatedAt); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (s *PostgresFleetStore) AddStation(st Station) error {
	_, err := s.db.Exec(
		`INSERT INTO stations (id, zone_id, name, position_x, position_y, position_lat, position_lon)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		st.ID, st.ZoneID, st.Name, st.PositionX, st.PositionY, st.PositionLat, st.PositionLon,
	)
	if err != nil {
		return fmt.Errorf("fleetservice: add station: %w", err)
	}
	return nil
}

func (s *PostgresFleetStore) ListStations() ([]Station, error) {
	rows, err := s.db.Query(`SELECT id, zone_id, name, position_x, position_y, position_lat, position_lon, created_at FROM stations ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list stations: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows — see ListTasks for why (TASKUI-04).
	stations := []Station{}
	for rows.Next() {
		var st Station
		if err := rows.Scan(&st.ID, &st.ZoneID, &st.Name, &st.PositionX, &st.PositionY, &st.PositionLat, &st.PositionLon, &st.CreatedAt); err != nil {
			return nil, err
		}
		stations = append(stations, st)
	}
	return stations, rows.Err()
}

func (s *PostgresFleetStore) CreateTask(t Task) (Task, error) {
	t.ID = ulid.Generate()
	if t.Status == "" {
		t.Status = "pending"
	}
	err := s.db.QueryRow(
		`INSERT INTO tasks (id, vehicle_id, from_station_id, to_station_id, status, priority)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		t.ID, t.VehicleID, t.FromStationID, t.ToStationID, t.Status, t.Priority,
	).Scan(&t.CreatedAt)
	if err != nil {
		return Task{}, fmt.Errorf("fleetservice: create task: %w", err)
	}
	t.AllowedTransitions = allowedTaskTransitions(t.Status)
	return t, nil
}

func (s *PostgresFleetStore) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, vehicle_id, from_station_id, to_station_id, status, priority, created_at, completed_at, status_changed_by FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list tasks: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows: a nil slice marshals to JSON `null`, which crashed
	// FleetTaskPanel.tsx's `tasks.length` check on a fresh/empty task list (found via manual
	// browser verification, TASK-14 follow-up).
	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.VehicleID, &t.FromStationID, &t.ToStationID, &t.Status, &t.Priority, &t.CreatedAt, &t.CompletedAt, &t.StatusChangedBy); err != nil {
			return nil, err
		}
		t.AllowedTransitions = allowedTaskTransitions(t.Status)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ErrTaskNotFound and ErrInvalidTransition let Handler.UpdateTaskStatus distinguish 404 from 409
// (ADR-030) — unlike AcknowledgeAlert, which only ever needs a single "not found" outcome.
var (
	ErrTaskNotFound      = errors.New("fleetservice: task not found")
	ErrInvalidTransition = errors.New("fleetservice: invalid task status transition")
)

// taskTransitionSources maps each allowed target status to the set of statuses a task may
// currently be in for that transition to succeed (ADR-030's state machine). "pending" is
// deliberately absent as a key — it is only ever the CreateTask default, never a PATCH target.
var taskTransitionSources = map[string][]string{
	"in_progress": {"pending"},
	"completed":   {"in_progress"},
	"cancelled":   {"pending", "in_progress"},
}

// taskTransitionOrder fixes the button order the frontend renders for a given current status
// (e.g. "Abschließen" before "Stornieren" for in_progress) — map iteration in
// allowedTaskTransitions would otherwise be non-deterministic. Must list every key of
// taskTransitionSources.
var taskTransitionOrder = []string{"in_progress", "completed", "cancelled"}

// allowedTaskTransitions derives, for a task currently in status, the list of target statuses it
// may transition to — the inverse of taskTransitionSources. This is the single source of truth
// for the transition matrix (TASKUI-02): previously frontend/src/components/FleetTaskPanel.tsx
// hand-maintained a duplicate copy (NEXT_TRANSITIONS) that had to be kept in sync manually.
// Task.AllowedTransitions now carries this over the wire instead. Always returns a non-nil slice
// (possibly empty for terminal statuses), matching the TASKUI-04 nil-slice convention.
func allowedTaskTransitions(status string) []string {
	targets := []string{}
	for _, target := range taskTransitionOrder {
		for _, source := range taskTransitionSources[target] {
			if source == status {
				targets = append(targets, target)
				break
			}
		}
	}
	return targets
}

// UpdateTaskStatus atomically transitions a task to newStatus, but only if its current status is
// an allowed source for that target (ADR-030) — race-safe against concurrent PATCHes on the same
// task via a single conditional UPDATE, not a read-then-write. On 0 rows affected, a follow-up
// SELECT tells "no such task" (ErrTaskNotFound) apart from "task exists but wrong current status"
// (ErrInvalidTransition). On a terminal transition (completed/cancelled), also clears a matching
// vehicle_status.current_task_id so the Fleet Overview doesn't keep showing a finished task as
// the vehicle's "current" one.
func (s *PostgresFleetStore) UpdateTaskStatus(id, newStatus, changedBy string) (Task, error) {
	sources, ok := taskTransitionSources[newStatus]
	if !ok {
		return Task{}, fmt.Errorf("fleetservice: unknown target status %q: %w", newStatus, ErrInvalidTransition)
	}

	var t Task
	err := s.db.QueryRow(
		`UPDATE tasks
		 SET status = $1,
		     completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
		     status_changed_by = $2
		 WHERE id = $3 AND status = ANY($4)
		 RETURNING id, vehicle_id, from_station_id, to_station_id, status, priority, created_at, completed_at, status_changed_by`,
		newStatus, changedBy, id, pq.Array(sources),
	).Scan(&t.ID, &t.VehicleID, &t.FromStationID, &t.ToStationID, &t.Status, &t.Priority, &t.CreatedAt, &t.CompletedAt, &t.StatusChangedBy)

	if err == sql.ErrNoRows {
		var current string
		selErr := s.db.QueryRow(`SELECT status FROM tasks WHERE id = $1`, id).Scan(&current)
		if selErr == sql.ErrNoRows {
			return Task{}, ErrTaskNotFound
		}
		return Task{}, ErrInvalidTransition
	}
	if err != nil {
		return Task{}, fmt.Errorf("fleetservice: update task status: %w", err)
	}

	if newStatus == "completed" || newStatus == "cancelled" {
		if _, err := s.db.Exec(
			`UPDATE vehicle_status SET current_task_id = NULL WHERE vehicle_id = $1 AND current_task_id = $2`,
			t.VehicleID, t.ID,
		); err != nil {
			return Task{}, fmt.Errorf("fleetservice: clear current_task_id: %w", err)
		}
	}

	t.AllowedTransitions = allowedTaskTransitions(t.Status)
	return t, nil
}

// UpsertVehicleStatus writes the latest live telemetry for a vehicle — one row per vehicle,
// overwritten on every update (not a time series; ADR-029 leaves history persistence open,
// see tasks/backlog.md FLEET-02).
func (s *PostgresFleetStore) UpsertVehicleStatus(vs VehicleStatus) error {
	_, err := s.db.Exec(
		`INSERT INTO vehicle_status (vehicle_id, battery_pct, speed, position_lat, position_lon, position_zone_id, autonomy_mode, current_task_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (vehicle_id) DO UPDATE SET
		   battery_pct = EXCLUDED.battery_pct,
		   speed = EXCLUDED.speed,
		   position_lat = EXCLUDED.position_lat,
		   position_lon = EXCLUDED.position_lon,
		   position_zone_id = EXCLUDED.position_zone_id,
		   autonomy_mode = EXCLUDED.autonomy_mode,
		   current_task_id = EXCLUDED.current_task_id,
		   updated_at = NOW()`,
		vs.VehicleID, vs.BatteryPct, vs.Speed, vs.PositionLat, vs.PositionLon, vs.PositionZoneID, vs.AutonomyMode, vs.CurrentTaskID,
	)
	if err != nil {
		return fmt.Errorf("fleetservice: upsert vehicle status: %w", err)
	}
	return nil
}

func (s *PostgresFleetStore) ListVehicleStatus() ([]VehicleStatus, error) {
	rows, err := s.db.Query(`SELECT vehicle_id, battery_pct, speed, position_lat, position_lon, position_zone_id, autonomy_mode, current_task_id, updated_at FROM vehicle_status`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list vehicle status: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows — see ListTasks for why (TASKUI-04).
	statuses := []VehicleStatus{}
	for rows.Next() {
		var vs VehicleStatus
		if err := rows.Scan(&vs.VehicleID, &vs.BatteryPct, &vs.Speed, &vs.PositionLat, &vs.PositionLon, &vs.PositionZoneID, &vs.AutonomyMode, &vs.CurrentTaskID, &vs.UpdatedAt); err != nil {
			return nil, err
		}
		statuses = append(statuses, vs)
	}
	return statuses, rows.Err()
}

// FleetVehicle is the combined view for the Dashboard's Flottenübersicht (AP2) — vehicle
// identity (id/display_name/vehicle_type, owned by control-server/fleet-service respectively,
// ADR-029) joined with live status. A vehicle that has never reported status yet (no
// vehicle_status row) still appears, with all status fields nil — LEFT JOIN, not INNER.
type FleetVehicle struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	VehicleType     *string    `json:"vehicle_type,omitempty"`
	BatteryPct      *float64   `json:"battery_pct,omitempty"`
	Speed           *float64   `json:"speed,omitempty"`
	PositionLat     *float64   `json:"position_lat,omitempty"`
	PositionLon     *float64   `json:"position_lon,omitempty"`
	PositionZoneID  *string    `json:"position_zone_id,omitempty"`
	AutonomyMode    *string    `json:"autonomy_mode,omitempty"`
	CurrentTaskID   *string    `json:"current_task_id,omitempty"`
	StatusUpdatedAt *time.Time `json:"status_updated_at,omitempty"`
}

// ListVehiclesWithStatus is fleet-service's primary read for the Web-Dashboard Flottenübersicht
// (AP2) — joins the shared vehicles table (control-server's `online` is deliberately not
// included here: it's live-computed in control-server's process, not persisted, ADR-022/029;
// the frontend combines both services' data client-side per ADR-029).
func (s *PostgresFleetStore) ListVehiclesWithStatus() ([]FleetVehicle, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.display_name, v.vehicle_type,
		       vs.battery_pct, vs.speed, vs.position_lat, vs.position_lon,
		       vs.position_zone_id, vs.autonomy_mode, vs.current_task_id, vs.updated_at
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		ORDER BY v.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list vehicles with status: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows — see ListTasks for why (TASKUI-04).
	vehicles := []FleetVehicle{}
	for rows.Next() {
		var v FleetVehicle
		if err := rows.Scan(&v.ID, &v.DisplayName, &v.VehicleType,
			&v.BatteryPct, &v.Speed, &v.PositionLat, &v.PositionLon,
			&v.PositionZoneID, &v.AutonomyMode, &v.CurrentTaskID, &v.StatusUpdatedAt); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

// CreateAlert inserts either a threshold-based (fleet-service-generated) or vehicle-initiated
// alert — both share this table (ADR-028).
func (s *PostgresFleetStore) CreateAlert(a Alert) (Alert, error) {
	a.ID = ulid.Generate()
	err := s.db.QueryRow(
		`INSERT INTO alerts (id, vehicle_id, severity, message) VALUES ($1, $2, $3, $4) RETURNING created_at`,
		a.ID, a.VehicleID, a.Severity, a.Message,
	).Scan(&a.CreatedAt)
	if err != nil {
		return Alert{}, fmt.Errorf("fleetservice: create alert: %w", err)
	}
	return a, nil
}

func (s *PostgresFleetStore) ListAlerts() ([]Alert, error) {
	rows, err := s.db.Query(`SELECT id, vehicle_id, severity, message, created_at, acknowledged_at, acknowledged_by FROM alerts ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list alerts: %w", err)
	}
	defer rows.Close()

	// Non-nil even with zero rows — see ListTasks for why (TASKUI-04).
	alerts := []Alert{}
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.VehicleID, &a.Severity, &a.Message, &a.CreatedAt, &a.AcknowledgedAt, &a.AcknowledgedBy); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// AcknowledgeAlert marks an alert as acknowledged by the given operator (AP2 "Alert Acknowledgement Feature").
func (s *PostgresFleetStore) AcknowledgeAlert(id, acknowledgedBy string) error {
	res, err := s.db.Exec(
		`UPDATE alerts SET acknowledged_at = NOW(), acknowledged_by = $1 WHERE id = $2`,
		acknowledgedBy, id,
	)
	if err != nil {
		return fmt.Errorf("fleetservice: acknowledge alert: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("fleetservice: alert %s not found", id)
	}
	return nil
}
