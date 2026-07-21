package vehicleregistry

import (
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConnectionChecker reports the given vehicles as online — used to verify List() populates
// Vehicle.Online from the injected ConnectionChecker rather than from the database.
type fakeConnectionChecker struct {
	online map[string]bool
}

func (f fakeConnectionChecker) Connected(vehicleID string) bool {
	return f.online[vehicleID]
}

func newMockStore(t *testing.T, conn ConnectionChecker) (*PostgresVehicleStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS vehicles")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := NewPostgresVehicleStore(db, conn)
	require.NoError(t, err)
	return store, mock
}

// --- NewPostgresVehicleStore ---

func TestNewPostgresVehicleStore_SchemaCreationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS vehicles")).
		WillReturnError(errors.New("connection refused"))

	store, err := NewPostgresVehicleStore(db, fakeConnectionChecker{})

	require.Error(t, err)
	assert.Nil(t, store)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- List ---

func TestPostgresVehicleStore_List_Success_PopulatesOnlineFromConnectionChecker(t *testing.T) {
	conn := fakeConnectionChecker{online: map[string]bool{"vehicle-001": true}}
	store, mock := newMockStore(t, conn)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "display_name", "description", "created_at"}).
		AddRow("vehicle-001", "Vehicle 1", "first", now).
		AddRow("vehicle-002", "Vehicle 2", "second", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, display_name, description, created_at FROM vehicles")).
		WillReturnRows(rows)

	vehicles, err := store.List()

	require.NoError(t, err)
	require.Len(t, vehicles, 2)
	assert.True(t, vehicles[0].Online, "vehicle-001 must be reported online by the ConnectionChecker")
	assert.False(t, vehicles[1].Online, "vehicle-002 has no live connection")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresVehicleStore_List_Empty(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	rows := sqlmock.NewRows([]string{"id", "display_name", "description", "created_at"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, display_name, description, created_at FROM vehicles")).
		WillReturnRows(rows)

	vehicles, err := store.List()

	require.NoError(t, err)
	assert.Empty(t, vehicles)
}

func TestPostgresVehicleStore_List_QueryError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, display_name, description, created_at FROM vehicles")).
		WillReturnError(errors.New("connection lost"))

	vehicles, err := store.List()

	require.Error(t, err)
	assert.Nil(t, vehicles)
}

// TestPostgresVehicleStore_List_RowError covers a corrupted row surfacing via rows.Err() after
// iteration stops early — the realistic failure mode for a row-level fault with database/sql
// (Scan itself is hard to force into error given Go's permissive type conversion).
func TestPostgresVehicleStore_List_RowError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	rows := sqlmock.NewRows([]string{"id", "display_name", "description", "created_at"}).
		AddRow("vehicle-001", "Vehicle 1", "first", time.Now()).
		RowError(0, errors.New("corrupted row"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, display_name, description, created_at FROM vehicles")).
		WillReturnRows(rows)

	vehicles, err := store.List()

	require.Error(t, err)
	assert.Nil(t, vehicles)
}

// --- Add ---

func TestPostgresVehicleStore_Add_Success(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3)")).
		WithArgs("vehicle-001", "Vehicle 1", "desc").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.Add("vehicle-001", "Vehicle 1", "desc")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPostgresVehicleStore_Add_DuplicateID covers double registration — the unique constraint on
// the primary key rejects the second Add() for the same vehicle ID.
func TestPostgresVehicleStore_Add_DuplicateID(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles")).
		WithArgs("vehicle-001", "Vehicle 1", "desc").
		WillReturnError(errors.New(`pq: duplicate key value violates unique constraint "vehicles_pkey"`))

	err := store.Add("vehicle-001", "Vehicle 1", "desc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestPostgresVehicleStore_Add_EmptyFields(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	// The store itself does not validate empty strings — that's the HTTP handler's job
	// (cmd/control-server/main.go). At the store level an empty display_name is just another value.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles")).
		WithArgs("vehicle-001", "", "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.Add("vehicle-001", "", "")

	require.NoError(t, err)
}

// --- Delete ---

func TestPostgresVehicleStore_Delete_Success(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-001").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.Delete("vehicle-001")

	require.NoError(t, err)
}

// TestPostgresVehicleStore_Delete_NotFound verifies the ErrNotFound sentinel is returned when no
// row was affected — callers (cmd/control-server/main.go) rely on errors.Is(err, ErrNotFound) to
// map this to HTTP 404.
func TestPostgresVehicleStore_Delete_NotFound(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-unknown").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.Delete("vehicle-unknown")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestPostgresVehicleStore_Delete_ExecError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-001").
		WillReturnError(errors.New("connection lost"))

	err := store.Delete("vehicle-001")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNotFound), "a connection error must not be mistaken for ErrNotFound")
}

// --- Exists ---

func TestPostgresVehicleStore_Exists_True(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := store.Exists("vehicle-001")

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestPostgresVehicleStore_Exists_False(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	exists, err := store.Exists("vehicle-nonexistent")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestPostgresVehicleStore_Exists_QueryError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM vehicles WHERE id = $1")).
		WithArgs("vehicle-001").
		WillReturnError(errors.New("connection lost"))

	exists, err := store.Exists("vehicle-001")

	require.Error(t, err)
	assert.False(t, exists)
}

// --- SeedDefault ---

func TestPostgresVehicleStore_SeedDefault_Success(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING")).
		WithArgs("vehicle-001", "Vehicle 001", "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.SeedDefault()

	require.NoError(t, err)
}

func TestPostgresVehicleStore_SeedDefault_AlreadyExists_NoError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	// ON CONFLICT DO NOTHING — a pre-existing vehicle-001 yields 0 rows affected, not an error.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING")).
		WithArgs("vehicle-001", "Vehicle 001", "").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.SeedDefault()

	require.NoError(t, err)
}

func TestPostgresVehicleStore_SeedDefault_ExecError(t *testing.T) {
	store, mock := newMockStore(t, fakeConnectionChecker{})

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING")).
		WithArgs("vehicle-001", "Vehicle 001", "").
		WillReturnError(errors.New("connection lost"))

	err := store.SeedDefault()

	require.Error(t, err)
}

// --- NoopVehicleStore ---

func TestNoopVehicleStore_AllMethodsAreNoop(t *testing.T) {
	var store VehicleStore = NoopVehicleStore{}

	vehicles, err := store.List()
	require.NoError(t, err)
	assert.Nil(t, vehicles)

	require.NoError(t, store.Add("vehicle-001", "Vehicle 1", "desc"))
	require.NoError(t, store.Delete("vehicle-001"))

	exists, err := store.Exists("vehicle-001")
	require.NoError(t, err)
	assert.False(t, exists)
}
