package server

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	ctr "github.com/arya-bhanu/go-doc-generator/app/core/documents/controller"
	ops_ctr "github.com/arya-bhanu/go-doc-generator/app/core/users/controller"
)

func Start(handler *ctr.Handler, opsHandler *ops_ctr.UserOpsHandler) {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	registerRoutes(r, handler, opsHandler)

	slog.Info("Starting server", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
