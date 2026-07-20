package authcheck

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// usersSchema mirrors internal/authservice's table definition minimally — this package only
// reads is_active by username (ADR-031 prep: authcheck has no FleetStore-style port of its own,
// it's a thin read against a table another service owns, see docs/adr/035-*).
const usersSchema = `
CREATE TABLE IF NOT EXISTS users (
    id           SERIAL PRIMARY KEY,
    username     TEXT NOT NULL UNIQUE,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE
);`

// newTestChecker requires DATABASE_URL (skipped otherwise, matching the project's Postgres
// integration-test convention, e.g. internal/fleetservice/store_test.go).
func newTestChecker(t *testing.T) (*Checker, *sql.DB) {
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

	if _, err := db.Exec(usersSchema); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	return NewChecker(db), db
}

func insertUser(t *testing.T, db *sql.DB, username string, isActive bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (username, is_active) VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET is_active = EXCLUDED.is_active`, username, isActive)
	if err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE username = $1`, username) })
}

func TestIsActiveOperator_ActiveUser_ReturnsTrue(t *testing.T) {
	c, db := newTestChecker(t)
	insertUser(t, db, "authcheck-test-active", true)

	active, err := c.IsActiveOperator(context.Background(), "authcheck-test-active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Fatal("expected active=true")
	}
}

func TestIsActiveOperator_DeactivatedUser_ReturnsFalse(t *testing.T) {
	c, db := newTestChecker(t)
	insertUser(t, db, "authcheck-test-deactivated", false)

	active, err := c.IsActiveOperator(context.Background(), "authcheck-test-deactivated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Fatal("expected active=false for a deactivated account")
	}
}

// TestIsActiveOperator_DeletedUser_ReturnsFalseNotError covers the DRIFT-K1 account-existence
// check (a hard DELETE, cmd/auth-service DeleteUser, is the only revocation path today —
// checker.go's doc comment) — a username with no row must report false, not an error, so
// AuthWatchdog treats "deleted" the same as "deactivated" rather than as a query failure.
func TestIsActiveOperator_DeletedUser_ReturnsFalseNotError(t *testing.T) {
	c, _ := newTestChecker(t)

	active, err := c.IsActiveOperator(context.Background(), "authcheck-test-never-existed")
	if err != nil {
		t.Fatalf("expected no error for a nonexistent user, got %v", err)
	}
	if active {
		t.Fatal("expected active=false for a nonexistent user")
	}
}
