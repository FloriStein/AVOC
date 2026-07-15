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
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{})

	rr := doRequest(h.Health, http.MethodGet, "/health", "", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ─── RequireAuth middleware (no DB required) ───────────────────────────────────

func TestRequireAuth_NoToken_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{})
	protected := h.RequireAuth(h.Health)

	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", "", nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_InvalidToken_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{})
	protected := h.RequireAuth(h.Health)

	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", "not.a.jwt", nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_WrongSecret_Returns401(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{})
	protected := h.RequireAuth(h.Health)

	token := mintToken(t, "a-completely-different-secret!!")
	rr := doRequest(protected, http.MethodGet, "/fleet/vehicles", token, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_ValidToken_PassesThrough(t *testing.T) {
	h := fleetservice.NewHandler(testSecret, nil, &stubDispatcher{})
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
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

	rr := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", map[string]string{"name": "Missing ID"})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateZone_InvalidEnvironment_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

	rr := doRequest(h.CreateZone, http.MethodPost, "/fleet/zones", "", map[string]string{
		"id": "handler-test-zone", "name": "Bad Env", "environment": "underwater",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateZone_Valid_Returns201AndListable(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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

// ─── Tasks (Postgres required) — dispatch wiring ───────────────────────────────

func TestCreateTask_MissingFields_Returns400(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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
	h := fleetservice.NewHandler(testSecret, store, dispatcher)

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
	h := fleetservice.NewHandler(testSecret, store, dispatcher)

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

// ─── Alerts (Postgres required) ────────────────────────────────────────────────

func TestAcknowledgeAlert_NotFound_Returns404(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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

// ─── Vehicles (Postgres required) ──────────────────────────────────────────────

func TestListVehicles_IncludesSeededVehicleWithNilStatus(t *testing.T) {
	store := requirePostgresStore(t)
	h := fleetservice.NewHandler(testSecret, store, &stubDispatcher{})

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
