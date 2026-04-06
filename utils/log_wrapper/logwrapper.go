package logwrapper

import (
	"context"
	"log/slog"
	"os"
)

const logFile = "store.log"

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

func Init() {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, opts)

	// Only write to store.log when APP_ENV=local
	if os.Getenv("APP_ENV") != "local" {
		slog.SetDefault(slog.New(jsonHandler))
		slog.Info("Logger initialized (stdout only)", "app_env", os.Getenv("APP_ENV"))
		return
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.SetDefault(slog.New(jsonHandler))
		slog.Error("Failed to open log file, falling back to stdout only", "file", logFile, "error", err)
		return
	}

	textHandler := slog.NewTextHandler(file, opts)

	slog.SetDefault(slog.New(&multiHandler{
		handlers: []slog.Handler{jsonHandler, textHandler},
	}))

	slog.Info("Logger initialized", "log_file", logFile, "app_env", "local")
}
