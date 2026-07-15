package fleetservice_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
