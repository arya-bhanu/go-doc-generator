package database

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func Connect() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		os.Exit(1)
	}

	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("Database ping failed", "error", err)
		os.Exit(1)
	}

	DB = pool
	slog.Info("Connected to Supabase (PostgreSQL) successfully")
}

func Close() {
	if DB != nil {
		DB.Close()
		slog.Info("Database connection closed")
	}
}
