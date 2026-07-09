package db

import (
	"testing"
	"time"
)

func TestWaitForReady_ReturnsErrorAfterExhaustingRetries(t *testing.T) {
	// Port 1 is a reserved, unlisted TCP port — the connection attempt fails
	// fast (connection refused) without needing a real Postgres instance.
	db, err := Open("postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	const retries = 3
	const delay = 10 * time.Millisecond

	start := time.Now()
	err = WaitForReady(db, retries, delay)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when Postgres is unreachable, got nil")
	}
	if minElapsed := delay * (retries - 1); elapsed < minElapsed {
		t.Errorf("expected WaitForReady to sleep between attempts (>= %v), took %v", minElapsed, elapsed)
	}
}

func TestWaitForReady_SucceedsOnFirstPing(t *testing.T) {
	db, err := Open("postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	db.Close() // closed DB: sql.DB still accepts calls but Ping fails immediately without retry cost

	start := time.Now()
	_ = WaitForReady(db, 1, time.Second)
	elapsed := time.Since(start)

	if elapsed >= time.Second {
		t.Errorf("single retry should not sleep after the last attempt, took %v", elapsed)
	}
}
