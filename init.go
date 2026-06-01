package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/beck-8/subs-check/app"
	"github.com/lmittmann/tint"
	mihomoLog "github.com/metacubex/mihomo/log"
)

var Version = "dev"
var CurrentCommit = "unknown"

var TempLog string

func init() {
	// Set dependency log levels.
	if os.Getenv("MIHOMO_DEBUG") != "" {
		mihomoLog.SetLevel(mihomoLog.DEBUG)
	} else {
		mihomoLog.SetLevel(mihomoLog.SILENT)
	}

	// Read the configured log level.
	logLevel := getLogLevel()

	// Create two separate handlers.
	// 1. Terminal output with color.
	consoleHandler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      logLevel,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// 2. File output without color; writes app.FileLogger ($TMP/subs-check.log) for the web UI.
	fileHandler := tint.NewHandler(app.FileLogger, &tint.Options{
		Level:      logLevel,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    true, // Disable color.
	})

	// Create a custom slog handler that sends records to both handlers.
	handler := &multiHandler{
		console: consoleHandler,
		file:    fileHandler,
	}

	logger := slog.New(handler)

	// Set the global logger.
	slog.SetDefault(logger)

	fmt.Println("==================== WARNING ====================")
	fmt.Println("⚠️  Important notice:")
	fmt.Println("1. This project is fully open source and free. Do not trust any paid versions.")
	fmt.Println("2. This project is for learning and communication only. Do not use it for illegal purposes.")
	fmt.Println("3. Project: https://github.com/beck-8/subs-check")
	fmt.Println("4. Image: ghcr.io/beck-8/subs-check:latest")
	fmt.Println("==================================================")

}

func getLogLevel() slog.Level {
	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL")) // Read the environment variable.
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default INFO level.
	}
}

// multiHandler sends log records to multiple handlers.
type multiHandler struct {
	console slog.Handler
	file    slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.console.Enabled(ctx, level) || h.file.Enabled(ctx, level)
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Clone the record to avoid races.
	r2 := r.Clone()

	// Terminal output with color.
	if err := h.console.Handle(ctx, r); err != nil {
		return err
	}

	// File output without color.
	return h.file.Handle(ctx, r2)
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &multiHandler{
		console: h.console.WithAttrs(attrs),
		file:    h.file.WithAttrs(attrs),
	}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	return &multiHandler{
		console: h.console.WithGroup(name),
		file:    h.file.WithGroup(name),
	}
}
