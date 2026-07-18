// Package env provides helpers for reading process configuration from
// environment variables.
package env

import (
	"os"

	"avoc/pkg/logger"
)

// Require reads key from the environment. If it is unset or empty, it logs
// a fatal error via log and exits the process — the shared shape every
// service previously repeated inline for required configuration like
// DATABASE_URL and JWT_SECRET.
func Require(key string, log *logger.Logger) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal(key + " environment variable is required")
	}
	return v
}

// OptionalOr reads key from the environment, returning fallback if key is
// unset or empty.
func OptionalOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
