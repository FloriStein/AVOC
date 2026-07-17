package db

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"

	"avoc/pkg/logger"
)

// DefaultConnectRetries/DefaultConnectRetryDelay bound how long WaitForReady
// waits for Postgres at process startup (~20s total by default) — enough to
// cover Postgres's own healthcheck window (interval 5s, retries 10 in
// docker-compose.prod.yml) for the case where Docker's `restart:
// unless-stopped` policy restarts this service without honoring
// `depends_on: service_healthy` (only `docker compose up` does).
const (
	DefaultConnectRetries   = 10
	DefaultConnectRetryDelay = 2 * time.Second
)

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Second)
	return db, nil
}

// WaitForReady pings db until it responds or retries are exhausted, sleeping
// delay between attempts. Returns the last ping error if never reachable.
func WaitForReady(db *sql.DB, retries int, delay time.Duration) error {
	var err error
	for i := 0; i < retries; i++ {
		if err = db.Ping(); err == nil {
			return nil
		}
		if i < retries-1 {
			time.Sleep(delay)
		}
	}
	return err
}

// OpenAndWait opens databaseURL and waits up to DefaultConnectRetries ×
// DefaultConnectRetryDelay for Postgres to become reachable. If Open itself
// fails, it logs a fatal error via log and exits the process. If Postgres is
// still unreachable after retries, it logs degradedModeMsg as a warning and
// returns the *sql.DB anyway, so the caller can continue in whatever
// degraded mode it already implements (e.g. Noop stores) — this is the
// shared shape every service previously duplicated to handle Docker's
// `restart: unless-stopped` policy, which, unlike `docker compose up`, does
// not honor `depends_on: service_healthy`: a crash-restarted service can
// otherwise race Postgres's own startup and get stuck needing a manual
// restart (Sprint 18 Bugfix, found after ICE-Fix-Redeploy 2026-07-09).
func OpenAndWait(databaseURL string, log *logger.Logger, degradedModeMsg string) *sql.DB {
	db, err := Open(databaseURL)
	if err != nil {
		log.Fatal("failed to open database", "error", err)
	}
	if err := WaitForReady(db, DefaultConnectRetries, DefaultConnectRetryDelay); err != nil {
		log.Warn(degradedModeMsg, "error", err)
	}
	return db
}
