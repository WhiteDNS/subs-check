package utils

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// SetupSignalHandler wires process signals to a cancel callback.
// SIGHUP (HUB) aborts the currently running check phase via onCancel;
// the program itself keeps running. SIGINT/SIGTERM are not handled
// here to avoid propagating to child processes.
func SetupSignalHandler(onCancel func()) {
	slog.Debug("Setting up signal handlers")

	hubSigChan := make(chan os.Signal, 1)
	signal.Notify(hubSigChan, syscall.SIGHUP)

	go func() {
		for sig := range hubSigChan {
			slog.Debug(fmt.Sprintf("Received HUB signal: %s", sig))
			onCancel()
			slog.Debug("HUB mode: requested cancellation of the current task; process continues")
		}
	}()
}
