package server

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

func Start() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	registerRoutes(r)

	slog.Info("Starting server", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
