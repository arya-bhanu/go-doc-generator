package server

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	ctr "github.com/arya-bhanu/go-doc-generator/app/core/documents/controller"
	ops_ctr "github.com/arya-bhanu/go-doc-generator/app/core/users/controller"
)

// cleanup ensures store.log and the temp/ directory exist, then truncates the
// log file and removes every entry inside temp/.  It is called once at server
// startup so that each run begins with a clean slate.
func cleanup() {
	const logFile = "store.log"
	const tempDir = "temp"

	// Ensure store.log exists (create if absent).
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		f, createErr := os.Create(logFile)
		if createErr != nil {
			slog.Warn("cleanup: could not create log file", "file", logFile, "error", createErr)
		} else {
			f.Close()
			slog.Info("cleanup: log file created", "file", logFile)
		}
	}

	// Truncate the log file so it starts empty on every run.
	if err := os.Truncate(logFile, 0); err != nil {
		slog.Warn("cleanup: could not truncate log file", "file", logFile, "error", err)
	} else {
		slog.Info("cleanup: log file truncated", "file", logFile)
	}

	// Ensure temp/ directory exists (create if absent).
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		slog.Warn("cleanup: could not create temp dir", "dir", tempDir, "error", err)
		return
	}

	// Remove every entry inside temp/ (keep the directory itself).
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		slog.Warn("cleanup: could not read temp dir", "dir", tempDir, "error", err)
		return
	}

	for _, entry := range entries {
		path := filepath.Join(tempDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			slog.Warn("cleanup: could not remove temp entry", "path", path, "error", err)
		} else {
			slog.Info("cleanup: removed temp entry", "path", path)
		}
	}
}

func Start(handler *ctr.Handler, opsHandler *ops_ctr.UserOpsHandler) {
	cleanup()

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
