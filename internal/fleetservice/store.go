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

// taskStatusHistoryTable is the ADR-032 audit table — one row per PATCH-driven status
// transition, additive to `tasks.status_changed_by` (ADR-030, unchanged, still the fast "last
// changer" lookup). `from_status` is nullable: always populated going forward (UpdateTaskStatus
// reads the true prior status atomically before overwriting it), but left NULL by the one-time
// backfill below for pre-existing `cancelled` tasks, where the prior status (`pending` or
// `in_progress`) can no longer be determined from `tasks` alone.
// `ON DELETE CASCADE`: there is no production task-deletion endpoint (tasks only transition
// status), but history rows have no reason to outlive their task if it's ever removed (test
// cleanup, or future admin tooling) — a plain RESTRICT here would otherwise block deleting a task
// that has any recorded transition.
const taskStatusHistoryTable = `
CREATE TABLE IF NOT EXISTS task_status_history (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status   TEXT NOT NULL,
    changed_by  TEXT NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_task_status_history_task_id ON task_status_history(task_id);
`

// vehiclePositionHistoryTable is the ADR-033 "gefahrene Route" table — one row per recorded
// position sample per vehicle, throttled on write (see RecordPositionHistory) rather than one row
// per telemetry update. `ON DELETE CASCADE` mirrors task_status_history's rationale (ADR-032):
// no production vehicle-deletion endpoint exists, but history rows have no reason to outlive
// their vehicle.
const vehiclePositionHistoryTable = `
CREATE TABLE IF NOT EXISTS vehicle_position_history (
    id           TEXT PRIMARY KEY,
    vehicle_id   TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    position_lat DOUBLE PRECISION NOT NULL,
    position_lon DOUBLE PRECISION NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vehicle_position_history_vehicle_id_recorded_at
    ON vehicle_position_history(vehicle_id, recorded_at);
`

// positionHistoryMinInterval throttles RecordPositionHistory writes (ADR-033) — vehicle-mock's
// fleet simulation reports status every 3s (fleetSimulationTick), which would otherwise write
// ~1,200 rows/hour/vehicle for a map polyline that doesn't need that resolution.
const positionHistoryMinInterval = 10 * time.Second

// positionHistoryRetention bounds table growth (ADR-033) — a deliberately coarse first guess
// (demo/pilot scale, no contractual retention requirement known), applied both at startup and on
// a periodic ticker (cmd/fleet-service/main.go) so a long-running process doesn't need a restart
// to shed old rows.
const positionHistoryRetention = 30 * 24 * time.Hour

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

// TaskStatusHistoryEntry is one recorded status transition for a Task (ADR-032), returned in
// chronological order (oldest first) by GetTaskStatusHistory / GET /fleet/tasks/{id}/history.
type TaskStatusHistoryEntry struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	// FromStatus is nil only for backfilled `cancelled` rows predating ADR-032, where the true
	// prior status (pending or in_progress) can no longer be determined — see
	// backfillTaskStatusHistory. Always set for transitions recorded going forward.
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ChangedBy  string    `json:"changed_by"`
	ChangedAt  time.Time `json:"changed_at"`
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

// VehiclePositionHistoryPoint is one recorded position sample for a vehicle (ADR-033), returned
// in chronological order (oldest first) by GetVehiclePositionHistory /
// GET /fleet/vehicles/{id}/history — the "gefahrene Route" polyline source.
type VehiclePositionHistoryPoint struct {
	VehicleID   string    `json:"vehicle_id"`
	PositionLat float64   `json:"position_lat"`
	PositionLon float64   `json:"position_lon"`
	RecordedAt  time.Time `json:"recorded_at"`
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
	if _, err := db.Exec(taskStatusHistoryTable); err != nil {
		return nil, fmt.Errorf("fleetservice: create task_status_history table: %w", err)
	}
	if err := backfillTaskStatusHistory(db); err != nil {
		return nil, fmt.Errorf("fleetservice: backfill task_status_history: %w", err)
	}
	if _, err := db.Exec(vehiclePositionHistoryTable); err != nil {
		return nil, fmt.Errorf("fleetservice: create vehicle_position_history table: %w", err)
	}
	if err := pruneVehiclePositionHistory(db); err != nil {
		return nil, fmt.Errorf("fleetservice: prune vehicle_position_history: %w", err)
	}
	return &PostgresFleetStore{db: db}, nil
}

// legacyTransition is one pre-ADR-032 task needing a backfilled task_status_history row.
type legacyTransition struct {
	taskID      string
	status      string
	changedBy   string
	completedAt *time.Time
	createdAt   time.Time
}

// backfillTaskStatusHistory (ADR-032) runs once per startup, populating task_status_history for
// tasks that transitioned before this table existed and have no history row yet (NOT EXISTS
// guard — idempotent across repeated restarts, same pattern as taskuiDemoSeedCleanup above).
func backfillTaskStatusHistory(db *sql.DB) error {
	legacy, err := queryLegacyTransitions(db)
	if err != nil {
		return err
	}
	for _, lt := range legacy {
		if err := insertBackfilledHistoryRow(db, lt); err != nil {
			return fmt.Errorf("insert backfilled history for task %s: %w", lt.taskID, err)
		}
	}
	return nil
}

func queryLegacyTransitions(db *sql.DB) ([]legacyTransition, error) {
	rows, err := db.Query(`
		SELECT t.id, t.status, t.status_changed_by, t.completed_at, t.created_at
		FROM tasks t
		WHERE t.status != 'pending' AND t.status_changed_by IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM task_status_history h WHERE h.task_id = t.id)`)
	if err != nil {
		return nil, fmt.Errorf("query tasks needing backfill: %w", err)
	}
	defer rows.Close()

	var legacy []legacyTransition
	for rows.Next() {
		var lt legacyTransition
		if err := rows.Scan(&lt.taskID, &lt.status, &lt.changedBy, &lt.completedAt, &lt.createdAt); err != nil {
			return nil, fmt.Errorf("scan task for backfill: %w", err)
		}
		legacy = append(legacy, lt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks for backfill: %w", err)
	}
	return legacy, nil
}

// insertBackfilledHistoryRow approximates changed_at (completed_at when available, else
// created_at — the exact original transition timestamp for in_progress/cancelled tasks was never
// recorded) and sets from_status only where ADR-030's transition table makes it unambiguous
// (in_progress came only from pending, completed only from in_progress) — cancelled is left NULL
// since it could have come from either.
func insertBackfilledHistoryRow(db *sql.DB, lt legacyTransition) error {
	var fromStatus *string
	switch lt.status {
	case "in_progress":
		s := "pending"
		fromStatus = &s
	case "completed":
		s := "in_progress"
		fromStatus = &s
	}
	changedAt := lt.createdAt
	if lt.completedAt != nil {
		changedAt = *lt.completedAt
	}
	_, err := db.Exec(
		`INSERT INTO task_status_history (id, task_id, from_status, to_status, changed_by, changed_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		ulid.Generate(), lt.taskID, fromStatus, lt.status, lt.changedBy, changedAt,
	)
	return err
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

	// ADR-032: `previous` is a CTE evaluated once against the pre-UPDATE snapshot (Postgres MVCC
	// semantics within a single statement) — reading the true prior status this way stays inside
	// the same atomic statement as the UPDATE itself, no separate read-then-write step added.
	var t Task
	var previousStatus string
	err := s.db.QueryRow(
		`WITH previous AS (SELECT status FROM tasks WHERE id = $3)
		 UPDATE tasks
		 SET status = $1,
		     completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
		     status_changed_by = $2
		 FROM previous
		 WHERE tasks.id = $3 AND tasks.status = ANY($4)
		 RETURNING tasks.id, tasks.vehicle_id, tasks.from_station_id, tasks.to_station_id, tasks.status,
		           tasks.priority, tasks.created_at, tasks.completed_at, tasks.status_changed_by, previous.status`,
		newStatus, changedBy, id, pq.Array(sources),
	).Scan(&t.ID, &t.VehicleID, &t.FromStationID, &t.ToStationID, &t.Status, &t.Priority, &t.CreatedAt, &t.CompletedAt, &t.StatusChangedBy, &previousStatus)

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

	if err := s.recordTaskStatusTransition(t, previousStatus, changedBy); err != nil {
		return Task{}, err
	}

	t.AllowedTransitions = allowedTaskTransitions(t.Status)
	return t, nil
}

// recordTaskStatusTransition clears a stale vehicle_status.current_task_id on a terminal
// transition (ADR-030 bugfix) and appends the ADR-032 audit row — both side effects of a
// successful UpdateTaskStatus, split out to keep that function within Rule 2.2's length limit.
func (s *PostgresFleetStore) recordTaskStatusTransition(t Task, previousStatus, changedBy string) error {
	if t.Status == "completed" || t.Status == "cancelled" {
		if _, err := s.db.Exec(
			`UPDATE vehicle_status SET current_task_id = NULL WHERE vehicle_id = $1 AND current_task_id = $2`,
			t.VehicleID, t.ID,
		); err != nil {
			return fmt.Errorf("fleetservice: clear current_task_id: %w", err)
		}
	}
	if _, err := s.db.Exec(
		`INSERT INTO task_status_history (id, task_id, from_status, to_status, changed_by) VALUES ($1, $2, $3, $4, $5)`,
		ulid.Generate(), t.ID, previousStatus, t.Status, changedBy,
	); err != nil {
		return fmt.Errorf("fleetservice: record task status history: %w", err)
	}
	return nil
}

// GetTaskStatusHistory returns a task's recorded status transitions in chronological order
// (ADR-032). Distinguishes "task does not exist" (ErrTaskNotFound, 404) from "task exists but has
// no transitions yet" (empty, non-nil slice, still pending) — an empty array on a non-existent
// task would otherwise be indistinguishable from the latter.
func (s *PostgresFleetStore) GetTaskStatusHistory(taskID string) ([]TaskStatusHistoryEntry, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`, taskID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("fleetservice: check task exists: %w", err)
	}
	if !exists {
		return nil, ErrTaskNotFound
	}

	rows, err := s.db.Query(
		`SELECT id, task_id, from_status, to_status, changed_by, changed_at
		 FROM task_status_history WHERE task_id = $1 ORDER BY changed_at ASC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list task status history: %w", err)
	}
	defer rows.Close()

	entries := []TaskStatusHistoryEntry{}
	for rows.Next() {
		var e TaskStatusHistoryEntry
		if err := rows.Scan(&e.ID, &e.TaskID, &e.FromStatus, &e.ToStatus, &e.ChangedBy, &e.ChangedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
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

// ErrVehicleNotFound lets Handler.GetVehiclePositionHistory distinguish "no such vehicle" (404)
// from "vehicle exists but has no recorded position samples yet" (empty, non-nil slice) — same
// distinction ErrTaskNotFound draws for GetTaskStatusHistory (ADR-032).
var ErrVehicleNotFound = errors.New("fleetservice: vehicle not found")

// RecordPositionHistory appends a throttled position sample (ADR-033) — a no-op if the vehicle's
// last recorded sample is younger than positionHistoryMinInterval, or if lat/lon is unknown. The
// conditional INSERT is a single atomic statement, not a separate read-then-write.
func (s *PostgresFleetStore) RecordPositionHistory(vehicleID string, lat, lon *float64) error {
	if lat == nil || lon == nil {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO vehicle_position_history (id, vehicle_id, position_lat, position_lon)
		 SELECT $1, $2, $3, $4
		 WHERE NOT EXISTS (
		   SELECT 1 FROM vehicle_position_history
		   WHERE vehicle_id = $2 AND recorded_at > NOW() - $5::INTERVAL
		 )`,
		ulid.Generate(), vehicleID, *lat, *lon, fmt.Sprintf("%d seconds", int(positionHistoryMinInterval.Seconds())),
	)
	if err != nil {
		return fmt.Errorf("fleetservice: record position history: %w", err)
	}
	return nil
}

// GetVehiclePositionHistory returns a vehicle's recorded position samples in chronological order
// (ADR-033), 404 (ErrVehicleNotFound) if the vehicle itself doesn't exist — distinguished from
// "exists but never reported a position yet" (empty, non-nil slice).
func (s *PostgresFleetStore) GetVehiclePositionHistory(vehicleID string) ([]VehiclePositionHistoryPoint, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM vehicles WHERE id = $1)`, vehicleID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("fleetservice: check vehicle exists: %w", err)
	}
	if !exists {
		return nil, ErrVehicleNotFound
	}

	rows, err := s.db.Query(
		`SELECT vehicle_id, position_lat, position_lon, recorded_at
		 FROM vehicle_position_history WHERE vehicle_id = $1 ORDER BY recorded_at ASC`,
		vehicleID,
	)
	if err != nil {
		return nil, fmt.Errorf("fleetservice: list vehicle position history: %w", err)
	}
	defer rows.Close()

	points := []VehiclePositionHistoryPoint{}
	for rows.Next() {
		var p VehiclePositionHistoryPoint
		if err := rows.Scan(&p.VehicleID, &p.PositionLat, &p.PositionLon, &p.RecordedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// pruneVehiclePositionHistory deletes samples older than positionHistoryRetention (ADR-033).
// Called once at startup (NewPostgresFleetStore) and on a periodic ticker
// (cmd/fleet-service/main.go, via PruneVehiclePositionHistory below) so a long-running process
// doesn't rely on a restart to shed old rows.
func pruneVehiclePositionHistory(db *sql.DB) error {
	_, err := db.Exec(
		`DELETE FROM vehicle_position_history WHERE recorded_at < NOW() - $1::INTERVAL`,
		fmt.Sprintf("%d seconds", int(positionHistoryRetention.Seconds())),
	)
	return err
}

// PruneVehiclePositionHistory exposes pruneVehiclePositionHistory to cmd/fleet-service/main.go's
// periodic retention ticker.
func (s *PostgresFleetStore) PruneVehiclePositionHistory() error {
	return pruneVehiclePositionHistory(s.db)
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
