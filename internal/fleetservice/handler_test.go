package fleetservice_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"

	"avoc/internal/fleetgateway"
	"avoc/internal/fleetservice"
)

const testSecret = "test-secret-32-chars-for-testing!"

// ─── Stub Dispatcher ──────────────────────────────────────────────────────────

type stubDispatcher struct {
	lastTask   fleetgateway.TaskAssignment
	dispatched bool
	err        error
}

func (d *stubDispatcher) DispatchTask(task fleetgateway.TaskAssignment) error {
	d.lastTask = task
	d.dispatched = true
	return d.err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mintToken(t *testing.T, secret string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "test-operator",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return s
}

// requirePostgresStore opens a real Postgres connection (skipped without DATABASE_URL, matching
// the project's integration-test convention — see store_test.go) and seeds a base vehicle so
// task/status CRUD tests have a valid FK target.
func requirePostgresStore(t *testing.T) *fleetservice.PostgresFleetStore {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vehicles (
		id TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		t.Fatalf("create base vehicles table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('handler-test-vehicle', 'Handler Test Vehicle')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	store, err := fleetservice.NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(`DELETE FROM alerts WHERE vehicle_id = 'handler-test-vehicle'`)
		db.Exec(`DELETE FROM vehicle_position_history WHERE vehicle_id = 'handler-test-vehicle'`)
		db.Exec(`DELETE FROM vehicle_status WHERE vehicle_id = 'handler-test-vehicle'`)
		db.Exec(`DELETE FROM tasks WHERE vehicle_id = 'handler-test-vehicle'`)
		db.Exec(`DELETE FROM stations WHERE zone_id = 'handler-test-zone'`)
		db.Exec(`DELETE FROM zones WHERE id = 'handler-test-zone'`)
		db.Exec(`DELETE FROM vehicles WHERE id = 'handler-test-vehicle'`)
	})

	return store
}

func doRequest(handler http.HandlerFunc, method, path, token string, body any) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler(rr, r)
	return rr
}

// ─── Health (no DB required) ───────────────────────────────────────────────────

func TestHealth_ReturnsOKWithoutAuth(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.Health, http.MethodGet, "/health", "", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ─── RequireAuth middleware (no DB required) ───────────────────────────────────

func TestRequireAuth_NoToken_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	protected := h.RequireAuth(h.Health)

	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", "", nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_InvalidToken_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	protected := h.RequireAuth(h.Health)

	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", "not.a.jwt", nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_WrongSecret_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	protected := h.RequireAuth(h.Health)

	token := mintToken(t, "a-completely-different-secret!!")
	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", token, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_ValidToken_PassesThrough(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	protected := h.RequireAuth(h.Health)

	token := mintToken(t, testSecret)
	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", token, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ─── Zones (Postgres required) ─────────────────────────────────────────────────

func TestCreateZone_MissingFields_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", map[string]string{"name": "Missing ID"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateZone_InvalidEnvironment_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", map[string]string{
		"id": "handler-test-zone", "name": "Bad Env", "environment": "underwater",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateZone_Valid_Returns201AndListable(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", map[string]string{
		"id": "handler-test-zone", "name": "Test Zone", "environment": "indoor",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(h.ListZones, http.MethodGet, "/fleet/zones", "", nil)
	var zones []fleetservice.Zone
	if err := json.NewDecoder(rr.Body).Decode(&zones); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, z := range zones {
		if z.ID == "handler-test-zone" {
			found = true
		}
	}
	if !found {
		t.Fatal("created zone not present in ListZones")
	}
}

func TestCreateZone_MalformedJSON_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	req := httptest.NewRequest(http.MethodPost, "/fleet/zones", bytes.NewBufferString("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateZone(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestCreateZone_DuplicateID_Returns500 confirms the handler surfaces a store-level failure
// (here: the PK uniqueness violation edgecases_test.go already proves at the store layer, see
// TestEdgeCases_PrimaryKeyDuplicates) as a 500 through the HTTP layer instead of panicking or
// silently succeeding twice.
func TestCreateZone_DuplicateID_Returns500(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	body := map[string]string{"id": "handler-test-zone", "name": "Test Zone", "environment": "indoor"}
	first := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first create to return 201, got %d: %s", first.Code, first.Body.String())
	}

	second := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", body)
	if second.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on duplicate id, got %d: %s", second.Code, second.Body.String())
	}
}

// ─── Stations (Postgres required) ──────────────────────────────────────────────

func TestCreateStation_MissingFields_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateStation, http.MethodPost, "/fleet/stations", "", map[string]string{"name": "Missing ID and zone"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateStation_MalformedJSON_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	req := httptest.NewRequest(http.MethodPost, "/fleet/stations", bytes.NewBufferString("not json at all"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateStation(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestCreateStation_UnknownZoneID_Returns500 mirrors TestCreateZone_DuplicateID_Returns500 for
// the FK-violation case (edgecases_test.go's TestEdgeCases_ForeignKeyViolations proves this at
// the store layer; this proves the HTTP handler surfaces it correctly too).
func TestCreateStation_UnknownZoneID_Returns500(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateStation, http.MethodPost, "/fleet/stations", "", map[string]string{
		"id": "handler-test-station-orphan", "zone_id": "does-not-exist", "name": "Orphan",
	})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown zone_id, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateStation_Valid_Returns201AndListable(t *testing.T) {
	store := requirePostgresStore(t)
	if err := store.AddZone(fleetservice.Zone{ID: "handler-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateStation, http.MethodPost, "/fleet/stations", "", map[string]string{
		"id": "handler-test-station-a", "zone_id": "handler-test-zone", "name": "A",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(h.ListStations, http.MethodGet, "/fleet/stations", "", nil)
	var stations []fleetservice.Station
	if err := json.NewDecoder(rr.Body).Decode(&stations); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, st := range stations {
		if st.ID == "handler-test-station-a" {
			found = true
		}
	}
	if !found {
		t.Fatal("created station not present in ListStations")
	}
}

// ─── Tasks (Postgres required) — dispatch wiring ───────────────────────────────

func TestCreateTask_MissingFields_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateTask, http.MethodPost, "/fleet/tasks", "", map[string]string{"vehicle_id": "handler-test-vehicle"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateTask_Valid_PersistsAndDispatchesToGateway(t *testing.T) {
	store := requirePostgresStore(t)
	if err := store.AddZone(fleetservice.Zone{ID: "handler-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-a", ZoneID: "handler-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-b", ZoneID: "handler-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}

	dispatcher := &stubDispatcher{}
	h := fleetservice.NewHandler(testSecret, store, dispatcher, fleetservice.NewHub())

	rr := doRequest(h.CreateTask, http.MethodPost, "/fleet/tasks", "", map[string]any{
		"vehicle_id": "handler-test-vehicle", "from_station_id": "handler-test-station-a",
		"to_station_id": "handler-test-station-b", "priority": 7,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var created fleetservice.Task
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Status != "pending" || created.ID == "" {
		t.Fatalf("unexpected created task: %+v", created)
	}

	if !dispatcher.dispatched {
		t.Fatal("expected CreateTask to call gw.DispatchTask")
	}
	if dispatcher.lastTask.VehicleID != "handler-test-vehicle" || dispatcher.lastTask.TaskID != created.ID {
		t.Fatalf("unexpected dispatched task: %+v", dispatcher.lastTask)
	}
}

func TestCreateTask_DispatchFails_TaskStillPersistedAndReturns201(t *testing.T) {
	store := requirePostgresStore(t)
	if err := store.AddZone(fleetservice.Zone{ID: "handler-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-a", ZoneID: "handler-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-b", ZoneID: "handler-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}

	dispatcher := &stubDispatcher{err: errors.New("mqtt: broker unreachable")}
	h := fleetservice.NewHandler(testSecret, store, dispatcher, fleetservice.NewHub())

	rr := doRequest(h.CreateTask, http.MethodPost, "/fleet/tasks", "", map[string]any{
		"vehicle_id": "handler-test-vehicle", "from_station_id": "handler-test-station-a",
		"to_station_id": "handler-test-station-b",
	})

	// Dispatch error must not fail the request — the task is already persisted (ADR-027: fire-and-forget).
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 even when dispatch fails, got %d: %s", rr.Code, rr.Body.String())
	}
	if !dispatcher.dispatched {
		t.Fatal("expected DispatchTask to still be called")
	}

	tasks, err := store.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	found := false
	for _, task := range tasks {
		if task.VehicleID == "handler-test-vehicle" {
			found = true
		}
	}
	if !found {
		t.Fatal("task not persisted despite dispatch failure")
	}
}

// TestCreateTask_UnknownVehicleID_Returns500 mirrors TestCreateStation_UnknownZoneID_Returns500
// for CreateTask's vehicle_id FK — edgecases_test.go's TestEdgeCases_ForeignKeyViolations only
// proved this at the store layer, never through the HTTP handler.
func TestCreateTask_UnknownVehicleID_Returns500(t *testing.T) {
	store := requirePostgresStore(t)
	if err := store.AddZone(fleetservice.Zone{ID: "handler-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-a", ZoneID: "handler-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-b", ZoneID: "handler-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateTask, http.MethodPost, "/fleet/tasks", "", map[string]any{
		"vehicle_id": "does-not-exist", "from_station_id": "handler-test-station-a", "to_station_id": "handler-test-station-b",
	})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown vehicle_id, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestCreateTask_UnknownStationID_Returns500 mirrors the above for from_station_id/to_station_id
// — same store-level proof already exists (TestEdgeCases_ForeignKeyViolations), never through HTTP.
func TestCreateTask_UnknownStationID_Returns500(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.CreateTask, http.MethodPost, "/fleet/tasks", "", map[string]any{
		"vehicle_id": "handler-test-vehicle", "from_station_id": "does-not-exist", "to_station_id": "also-missing",
	})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown station_id, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateTask_MalformedJSON_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	req := httptest.NewRequest(http.MethodPost, "/fleet/tasks", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateTask(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestListTasks_IncludesCreatedTask exercises Handler.ListTasks directly — previously only
// store.ListTasks() was exercised (via TestCreateTask_DispatchFails...), never the HTTP handler
// itself.
func TestListTasks_IncludesCreatedTask(t *testing.T) {
	store := requirePostgresStore(t)
	if err := store.AddZone(fleetservice.Zone{ID: "handler-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-a", ZoneID: "handler-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "handler-test-station-b", ZoneID: "handler-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	created, err := store.CreateTask(fleetservice.Task{
		VehicleID: "handler-test-vehicle", FromStationID: "handler-test-station-a", ToStationID: "handler-test-station-b",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.ListTasks, http.MethodGet, "/fleet/tasks", "", nil)
	var tasks []fleetservice.Task
	if err := json.NewDecoder(rr.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, task := range tasks {
		if task.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("created task not present in ListTasks")
	}
}

// ─── UpdateTaskStatus (ADR-030, Postgres required) ─────────────────────────────

// taskStatusFixtureCounter guarantees fixture ID uniqueness across multiple
// newTaskStatusTestFixture calls within the *same* test (t.Name() alone isn't unique enough —
// e.g. TestUpdateTaskStatus_TerminalStates_RejectAnyFurtherTransition needs two independent
// fixtures, one per terminal state, both under one t.Name()).
var taskStatusFixtureCounter atomic.Int64

// newTaskStatusTestFixture creates a fresh zone/2 stations/task for one test's exclusive use —
// unlike other Task tests in this file, UpdateTaskStatus tests mutate the task's status
// repeatedly, so each fixture needs its own rows rather than sharing "handler-test-*" IDs with
// concurrently-run sibling tests (Go runs tests in a package sequentially by default, but a
// shared task row would still make failures in one test contaminate another's starting state).
func newTaskStatusTestFixture(t *testing.T, store *fleetservice.PostgresFleetStore) fleetservice.Task {
	t.Helper()
	suffix := t.Name() + "-" + strconv.FormatInt(taskStatusFixtureCounter.Add(1), 10)
	zoneID := "handler-test-zone-status-" + suffix
	stationAID := "handler-test-station-a-status-" + suffix
	stationBID := "handler-test-station-b-status-" + suffix
	if err := store.AddZone(fleetservice.Zone{ID: zoneID, Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: stationAID, ZoneID: zoneID, Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: stationBID, ZoneID: zoneID, Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	task, err := store.CreateTask(fleetservice.Task{
		VehicleID: "handler-test-vehicle", FromStationID: stationAID, ToStationID: stationBID,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Own short-lived connection for cleanup only — PostgresFleetStore's db field is unexported
	// (this file is package fleetservice_test), and this fixture's dynamically-named rows aren't
	// covered by requirePostgresStore's own fixed-ID cleanup. Registered before
	// requirePostgresStore's t.Cleanup runs (LIFO — this runs first) and must independently unblock
	// the FK chain (vehicle_status.current_task_id -> tasks.id -> stations.id -> zones.id, all
	// RESTRICT) for this fixture's own rows rather than relying on run order.
	t.Cleanup(func() {
		db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
		if err != nil {
			return
		}
		defer db.Close()
		db.Exec(`UPDATE vehicle_status SET current_task_id = NULL WHERE vehicle_id = 'handler-test-vehicle' AND current_task_id = $1`, task.ID)
		db.Exec(`DELETE FROM tasks WHERE id = $1`, task.ID)
		db.Exec(`DELETE FROM stations WHERE zone_id = $1`, zoneID)
		db.Exec(`DELETE FROM zones WHERE id = $1`, zoneID)
	})

	return task
}

func newTaskStatusMux(h *fleetservice.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /fleet/tasks/{id}/status", h.UpdateTaskStatus)
	return mux
}

func patchTaskStatus(mux *http.ServeMux, taskID string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/fleet/tasks/"+taskID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestUpdateTaskStatus_NotFound_Returns404(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := patchTaskStatus(newTaskStatusMux(h), "does-not-exist", map[string]string{"status": "in_progress", "changed_by": "operator-1"})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateTaskStatus_MalformedJSON_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	mux := newTaskStatusMux(h)

	req := httptest.NewRequest(http.MethodPatch, "/fleet/tasks/whatever/status", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTaskStatus_MissingFields_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	mux := newTaskStatusMux(h)

	rr := patchTaskStatus(mux, "whatever", map[string]string{"status": "in_progress"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing changed_by: expected 400, got %d", rr.Code)
	}

	rr = patchTaskStatus(mux, "whatever", map[string]string{"changed_by": "operator-1"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing status: expected 400, got %d", rr.Code)
	}
}

func TestUpdateTaskStatus_UnknownTargetStatus_Returns409(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)

	// "pending" is a valid enum value in the DB CHECK constraint but never a legal PATCH target
	// (ADR-030 — it's exclusively the CreateTask default), and "bogus" isn't a status at all.
	// Both must be rejected as invalid transitions, not crash on an unrecognized map key.
	for _, target := range []string{"pending", "bogus"} {
		rr := patchTaskStatus(newTaskStatusMux(h), task.ID, map[string]string{"status": target, "changed_by": "operator-1"})
		if rr.Code != http.StatusConflict {
			t.Fatalf("target %q: expected 409, got %d: %s", target, rr.Code, rr.Body.String())
		}
	}
}

func TestUpdateTaskStatus_PendingToInProgress_Valid(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)

	rr := patchTaskStatus(newTaskStatusMux(h), task.ID, map[string]string{"status": "in_progress", "changed_by": "operator-1"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var updated fleetservice.Task
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("expected status in_progress, got %q", updated.Status)
	}
	if updated.StatusChangedBy == nil || *updated.StatusChangedBy != "operator-1" {
		t.Fatalf("expected status_changed_by=operator-1, got %v", updated.StatusChangedBy)
	}
	if updated.CompletedAt != nil {
		t.Fatalf("expected completed_at to stay nil for in_progress, got %v", updated.CompletedAt)
	}
}

func TestUpdateTaskStatus_InProgressToCompleted_SetsCompletedAt(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)
	mux := newTaskStatusMux(h)

	if rr := patchTaskStatus(mux, task.ID, map[string]string{"status": "in_progress", "changed_by": "operator-1"}); rr.Code != http.StatusOK {
		t.Fatalf("setup transition to in_progress failed: %d: %s", rr.Code, rr.Body.String())
	}

	rr := patchTaskStatus(mux, task.ID, map[string]string{"status": "completed", "changed_by": "operator-2"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var updated fleetservice.Task
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Status != "completed" || updated.CompletedAt == nil {
		t.Fatalf("expected completed with a completed_at timestamp, got %+v", updated)
	}
	if updated.StatusChangedBy == nil || *updated.StatusChangedBy != "operator-2" {
		t.Fatalf("expected status_changed_by to reflect the latest transition (operator-2), got %v", updated.StatusChangedBy)
	}
}

func TestUpdateTaskStatus_PendingToCancelled_Valid(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)

	rr := patchTaskStatus(newTaskStatusMux(h), task.ID, map[string]string{"status": "cancelled", "changed_by": "operator-1"})
	if rr.Code != http.StatusOK {
		t.Fatalf("pending->cancelled must be allowed (cancelling a never-started task), got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateTaskStatus_TerminalStates_RejectAnyFurtherTransition proves both terminal states
// (completed, cancelled) reject every subsequent transition attempt, not just the "obvious"
// reverse one — the full point of ADR-030's allowed-source-set model over a naive current!=target check.
func TestUpdateTaskStatus_TerminalStates_RejectAnyFurtherTransition(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	mux := newTaskStatusMux(h)

	completedTask := newTaskStatusTestFixture(t, store)
	for _, step := range []string{"in_progress", "completed"} {
		if rr := patchTaskStatus(mux, completedTask.ID, map[string]string{"status": step, "changed_by": "operator-1"}); rr.Code != http.StatusOK {
			t.Fatalf("setup step %q failed: %d: %s", step, rr.Code, rr.Body.String())
		}
	}
	for _, target := range []string{"in_progress", "completed", "cancelled"} {
		rr := patchTaskStatus(mux, completedTask.ID, map[string]string{"status": target, "changed_by": "operator-1"})
		if rr.Code != http.StatusConflict {
			t.Fatalf("completed->%s: expected 409, got %d", target, rr.Code)
		}
	}

	cancelledTask := newTaskStatusTestFixture(t, store)
	if rr := patchTaskStatus(mux, cancelledTask.ID, map[string]string{"status": "cancelled", "changed_by": "operator-1"}); rr.Code != http.StatusOK {
		t.Fatalf("setup cancel failed: %d: %s", rr.Code, rr.Body.String())
	}
	for _, target := range []string{"in_progress", "completed", "cancelled"} {
		rr := patchTaskStatus(mux, cancelledTask.ID, map[string]string{"status": target, "changed_by": "operator-1"})
		if rr.Code != http.StatusConflict {
			t.Fatalf("cancelled->%s: expected 409, got %d", target, rr.Code)
		}
	}
}

// TestUpdateTaskStatus_Idempotency_RepeatingSameTransition_SecondCallReturns409 documents the
// deliberate behaviour (contrast with AcknowledgeAlert, which allows repeat calls to silently
// overwrite): a status transition is a one-way state change, not a settable value, so replaying
// the same PATCH after it already succeeded is itself an invalid transition — the task has already
// left its previous source status — and must be rejected, not silently treated as a no-op success.
func TestUpdateTaskStatus_Idempotency_RepeatingSameTransition_SecondCallReturns409(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)
	mux := newTaskStatusMux(h)

	body := map[string]string{"status": "in_progress", "changed_by": "operator-1"}
	if rr := patchTaskStatus(mux, task.ID, body); rr.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := patchTaskStatus(mux, task.ID, body); rr.Code != http.StatusConflict {
		t.Fatalf("second identical call: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateTaskStatus_TerminalTransition_ClearsVehicleCurrentTaskID proves the ADR-030 bugfix:
// before this, nothing ever cleared vehicle_status.current_task_id when its referenced task
// finished, leaving the Fleet Overview showing a "current task" that had actually ended.
func TestUpdateTaskStatus_TerminalTransition_ClearsVehicleCurrentTaskID(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)

	if err := store.UpsertVehicleStatus(fleetservice.VehicleStatus{
		VehicleID: "handler-test-vehicle", AutonomyMode: "autonomous", CurrentTaskID: &task.ID,
	}); err != nil {
		t.Fatalf("UpsertVehicleStatus: %v", err)
	}

	rr := patchTaskStatus(newTaskStatusMux(h), task.ID, map[string]string{"status": "cancelled", "changed_by": "operator-1"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	statuses, err := store.ListVehicleStatus()
	if err != nil {
		t.Fatalf("ListVehicleStatus: %v", err)
	}
	for _, s := range statuses {
		if s.VehicleID == "handler-test-vehicle" {
			if s.CurrentTaskID != nil {
				t.Fatalf("expected current_task_id cleared after task cancellation, got %v", *s.CurrentTaskID)
			}
			return
		}
	}
	t.Fatal("handler-test-vehicle status not found")
}

// ─── GetTaskStatusHistory (ADR-032, Postgres required) ─────────────────────────

func newTaskHistoryMux(h *fleetservice.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleet/tasks/{id}/history", h.GetTaskStatusHistory)
	return mux
}

func TestGetTaskStatusHistory_NotFound_Returns404(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	req := httptest.NewRequest(http.MethodGet, "/fleet/tasks/does-not-exist/history", nil)
	rr := httptest.NewRecorder()
	newTaskHistoryMux(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetTaskStatusHistory_ReturnsRecordedTransitions drives the endpoint end-to-end: create a
// task, PATCH its status via the real UpdateTaskStatus handler, then verify the history endpoint
// reflects that transition — proves the HTTP layer wiring, not just the store method in isolation
// (store_test.go already covers the store method's own edge cases in more detail).
func TestGetTaskStatusHistory_ReturnsRecordedTransitions(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	task := newTaskStatusTestFixture(t, store)

	patchRR := patchTaskStatus(newTaskStatusMux(h), task.ID, map[string]string{"status": "in_progress", "changed_by": "operator-7"})
	if patchRR.Code != http.StatusOK {
		t.Fatalf("expected PATCH to succeed, got %d: %s", patchRR.Code, patchRR.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/fleet/tasks/"+task.ID+"/history", nil)
	rr := httptest.NewRecorder()
	newTaskHistoryMux(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var entries []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d: %s", len(entries), rr.Body.String())
	}
	if entries[0]["to_status"] != "in_progress" || entries[0]["changed_by"] != "operator-7" {
		t.Fatalf("unexpected history entry: %+v", entries[0])
	}
	if entries[0]["from_status"] != "pending" {
		t.Fatalf("expected from_status=pending, got %v", entries[0]["from_status"])
	}
}

// ─── GetVehiclePositionHistory (ADR-033, Postgres required) ────────────────────

func newVehicleHistoryMux(h *fleetservice.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleet/vehicles/{id}/history", h.GetVehiclePositionHistory)
	return mux
}

func TestGetVehiclePositionHistory_NotFound_Returns404(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	req := httptest.NewRequest(http.MethodGet, "/fleet/vehicles/does-not-exist/history", nil)
	rr := httptest.NewRecorder()
	newVehicleHistoryMux(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetVehiclePositionHistory_ReturnsRecordedSamples drives the endpoint end-to-end: record a
// position sample via the real store method, then verify the history endpoint reflects it —
// proves the HTTP layer wiring (store_test.go's positionhistory_test.go already covers the store
// method's throttling/retention edge cases in more detail).
func TestGetVehiclePositionHistory_ReturnsRecordedSamples(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())
	lat, lon := 52.13, 11.64
	if err := store.RecordPositionHistory("handler-test-vehicle", &lat, &lon); err != nil {
		t.Fatalf("RecordPositionHistory: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/fleet/vehicles/handler-test-vehicle/history", nil)
	rr := httptest.NewRecorder()
	newVehicleHistoryMux(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var points []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &points); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d: %s", len(points), rr.Body.String())
	}
	if points[0]["position_lat"] != lat || points[0]["position_lon"] != lon {
		t.Fatalf("unexpected point: %+v", points[0])
	}
}

// ─── Alerts (Postgres required) ────────────────────────────────────────────────

func TestAcknowledgeAlert_NotFound_Returns404(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	body, _ := json.Marshal(map[string]string{"acknowledged_by": "operator-1"})
	req := httptest.NewRequest(http.MethodPost, "/fleet/alerts/does-not-exist/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestAcknowledgeAlert_MissingAcknowledgedBy_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	req := httptest.NewRequest(http.MethodPost, "/fleet/alerts/whatever/acknowledge", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAcknowledgeAlert_Valid_Returns204(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	alert, err := store.CreateAlert(fleetservice.Alert{VehicleID: "handler-test-vehicle", Severity: "warning", Message: "Low battery"})
	if err != nil {
		t.Fatalf("CreateAlert: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	body, _ := json.Marshal(map[string]string{"acknowledged_by": "operator-1"})
	req := httptest.NewRequest(http.MethodPost, "/fleet/alerts/"+alert.ID+"/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestAcknowledgeAlert_AlreadyAcknowledged_SecondCallSucceedsAndOverwrites documents the current,
// deliberate behaviour: store.AcknowledgeAlert's UPDATE matches the row by id regardless of
// whether acknowledged_at is already set, so a second acknowledgement is not rejected — it
// overwrites acknowledged_by/acknowledged_at with the latest caller's values. Not a bug to fix
// here (no idempotency-key/first-writer-wins requirement exists yet), but previously unverified
// behaviour that a future change could silently alter.
func TestAcknowledgeAlert_AlreadyAcknowledged_SecondCallSucceedsAndOverwrites(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	alert, err := store.CreateAlert(fleetservice.Alert{VehicleID: "handler-test-vehicle", Severity: "warning", Message: "Low battery"})
	if err != nil {
		t.Fatalf("CreateAlert: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	ackAs := func(operator string) int {
		body, _ := json.Marshal(map[string]string{"acknowledged_by": operator})
		req := httptest.NewRequest(http.MethodPost, "/fleet/alerts/"+alert.ID+"/acknowledge", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr.Code
	}

	if code := ackAs("operator-1"); code != http.StatusNoContent {
		t.Fatalf("expected 204 on first acknowledge, got %d", code)
	}
	if code := ackAs("operator-2"); code != http.StatusNoContent {
		t.Fatalf("expected 204 on second acknowledge (current behaviour: not rejected), got %d", code)
	}

	alerts, err := store.ListAlerts()
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	found := false
	for _, a := range alerts {
		if a.ID == alert.ID {
			found = true
			if a.AcknowledgedBy == nil || *a.AcknowledgedBy != "operator-2" {
				t.Fatalf("expected second acknowledge to overwrite acknowledged_by with operator-2, got %v", a.AcknowledgedBy)
			}
		}
	}
	if !found {
		t.Fatal("alert not found after double acknowledge")
	}
}

func TestAcknowledgeAlert_MalformedJSON_Returns400(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{}, fleetservice.NewHub())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	req := httptest.NewRequest(http.MethodPost, "/fleet/alerts/whatever/acknowledge", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestListAlerts_IncludesCreatedAlert exercises Handler.ListAlerts directly — until now nothing
// called it via HTTP; alerts were only ever read back through store.ListAlerts() in tests.
func TestListAlerts_IncludesCreatedAlert(t *testing.T) {
	store := requirePostgresStore(t)
	created, err := store.CreateAlert(fleetservice.Alert{VehicleID: "handler-test-vehicle", Severity: "critical", Message: "Obstacle detected"})
	if err != nil {
		t.Fatalf("CreateAlert: %v", err)
	}
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.ListAlerts, http.MethodGet, "/fleet/alerts", "", nil)
	var alerts []fleetservice.Alert
	if err := json.NewDecoder(rr.Body).Decode(&alerts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, a := range alerts {
		if a.ID == created.ID {
			found = true
			if a.Severity != "critical" || a.Message != "Obstacle detected" {
				t.Fatalf("unexpected alert fields: %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("created alert not present in ListAlerts")
	}
}

// ─── Vehicles (Postgres required) ──────────────────────────────────────────────

func TestListVehicles_IncludesSeededVehicleWithNilStatus(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{}, fleetservice.NewHub())

	rr := doRequest(h.ListVehicles, http.MethodGet, "/fleet/vehicles", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var vehicles []fleetservice.FleetVehicle
	if err := json.NewDecoder(rr.Body).Decode(&vehicles); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, v := range vehicles {
		if v.ID == "handler-test-vehicle" {
			found = true
			if v.BatteryPct != nil {
				t.Fatalf("expected nil BatteryPct for vehicle with no status row, got %v", *v.BatteryPct)
			}
		}
	}
	if !found {
		t.Fatal("seeded vehicle not present in ListVehicles")
	}
}
