package logging

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	Logger = slog.New(handler)
}

func WithComponent(component string) *slog.Logger {
	return Logger.With("component", component)
}
