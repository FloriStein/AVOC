package main

import (
	"context"
	"net/http"

	"avoc/internal/authservice"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/env"
	"avoc/pkg/logger"
)

var log = logger.New("auth-service")

func main() {
	port := env.OptionalOr("AUTH_PORT", "8081")
	secret := env.Require("JWT_SECRET", log)
	databaseURL := env.Require("DATABASE_URL", log)
	adminPassword := env.Require("ADMIN_PASSWORD", log)

	db := pkgdb.OpenAndWait(databaseURL, log, "database not reachable after retries — proceeding anyway")
	defer db.Close()

	userStore, err := authservice.NewPostgresUserStore(db)
	if err != nil {
		log.Fatal("failed to initialize user store", "error", err)
	}

	ctx := context.Background()
	if err := userStore.SeedAdmin(ctx, "admin", adminPassword); err != nil {
		log.Fatal("failed to seed admin user", "error", err)
	}
	log.Info("admin user seeded (idempotent)")

	handler := authservice.NewHandler(secret, userStore)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/operator/login", handler.OperatorLogin)
	mux.HandleFunc("POST /auth/vehicle/register", handler.VehicleRegister)
	mux.HandleFunc("POST /auth/token/validate", handler.ValidateToken)
	mux.HandleFunc("POST /auth/token/refresh", handler.RefreshToken)
	mux.HandleFunc("POST /auth/handover/token", handler.HandoverToken)

	// User management — ADMIN only (ADR-024)
	mux.HandleFunc("GET /auth/users",         handler.RequireAdmin(handler.ListUsers))
	mux.HandleFunc("POST /auth/users",        handler.RequireAdmin(handler.CreateUser))
	mux.HandleFunc("DELETE /auth/users/{id}", handler.RequireAdmin(handler.DeleteUser))
	mux.HandleFunc("PATCH /auth/users/{id}",  handler.RequireAdmin(handler.UpdateUserRole))

	mux.HandleFunc("GET /health", handler.Health)

	log.Info("Auth Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Auth Service failed", "error", err)
	}
}
