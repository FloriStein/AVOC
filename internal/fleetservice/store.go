// Package fleetservice manages the Fleet domain (zones, stations, tasks, alerts, live vehicle
// status) introduced by ADR-029. It shares the `vehicles` identity table owned by
// internal/vehicleregistry (control-server) via foreign keys — no duplicate vehicle registry.
package fleetservice

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

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
    completed_at     TIMESTAMPTZ
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

// Zone is an admin-managed spatial area (AP3 "Räumliche Zonenzuweisung") — indoor (SVG-based)
// or outdoor (geo-referenced SVG overlay, ADR-029).
type Zone struct {
	ID          string
	Name        string
	Environment string // "indoor" | "outdoor"
	SVGGeometry string
	GeoBounds   *string // raw JSON, nil for indoor zones without geo-reference
	CreatedAt   time.Time
}

// Station is a map-anchored point — the endpoint unit for Tasks.
type Station struct {
	ID          string
	ZoneID      string
	Name        string
	PositionX   *float64
	PositionY   *float64
	PositionLat *float64
	PositionLon *float64
	CreatedAt   time.Time
}

// Task represents a vehicle moving between two Stations (ADR-029 — first-version task model,
// no multi-step job/loading model yet).
type Task struct {
	ID            string
	VehicleID     string
	FromStationID string
	ToStationID   string
	Status        string // "pending" | "in_progress" | "completed" | "cancelled"
	Priority      int
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// VehicleStatus is the live fleet telemetry for one vehicle — separate from the vehicles
// identity table because it's updated at high frequency (ADR-029).
type VehicleStatus struct {
	VehicleID      string
	BatteryPct     *float64
	Speed          *float64
	PositionLat    *float64
	PositionLon    *float64
	PositionZoneID *string
	AutonomyMode   string // "autonomous" | "teleoperated" | "manual"
	CurrentTaskID  *string
	UpdatedAt      time.Time
}

// Alert is a fleet-service-generated or vehicle-initiated notification (ADR-028: two sources,
// same table/shape — threshold-based from fleet-service, or forwarded verbatim from a
// vehicle-initiated "needs intervention" event).
type Alert struct {
	ID             string
	VehicleID      string
	Severity       string // "info" | "warning" | "critical"
	Message        string
	CreatedAt      time.Time
	AcknowledgedAt *time.Time
	AcknowledgedBy *string
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
	return &PostgresFleetStore{db: db}, nil
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

	var zones []Zone
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

	var stations []Station
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
	return t, nil
}

func (s *PostgresFleetStore) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, vehicle_id, from_station_id, to_station_id, status, priority, created_at, completed_at FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.VehicleID, &t.FromStationID, &t.ToStationID, &t.Status, &t.Priority, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
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

	var statuses []VehicleStatus
	for rows.Next() {
		var vs VehicleStatus
		if err := rows.Scan(&vs.VehicleID, &vs.BatteryPct, &vs.Speed, &vs.PositionLat, &vs.PositionLon, &vs.PositionZoneID, &vs.AutonomyMode, &vs.CurrentTaskID, &vs.UpdatedAt); err != nil {
			return nil, err
		}
		statuses = append(statuses, vs)
	}
	return statuses, rows.Err()
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

	var alerts []Alert
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
