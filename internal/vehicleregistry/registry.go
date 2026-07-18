package vehicleregistry

import (
	"errors"
	"time"
)

// ErrNotFound is returned by Delete when no vehicle with the given ID exists.
var ErrNotFound = errors.New("vehicleregistry: not found")

// Vehicle represents a pre-configured vehicle in the fleet.
type Vehicle struct {
	ID          string
	DisplayName string
	Description string
	CreatedAt   time.Time
	Online      bool // live — not persisted, from ConnectionChecker
}

// ConnectionChecker reports whether a vehicle currently has an active WebSocket connection.
// vehicleconnection.Registry satisfies this interface.
type ConnectionChecker interface {
	Connected(vehicleID string) bool
}

// VehicleStore manages the persisted fleet configuration. SeedDefault is deliberately not part
// of this interface (GOSTYLE-IF-05) — it's a one-shot bootstrap step called on the concrete
// *PostgresVehicleStore in cmd/control-server/main.go's newVehicleStore, never through this
// interface once the store is running.
type VehicleStore interface {
	List() ([]Vehicle, error)
	Add(id, displayName, description string) error
	Delete(id string) error
	Exists(id string) (bool, error)
}
