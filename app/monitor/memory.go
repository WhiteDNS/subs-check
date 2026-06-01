package monitor

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	human "github.com/docker/go-units"
)

// StartMemoryMonitor starts memory monitoring.
func StartMemoryMonitor() {
	// Keep this restart guard for memory pressure even though the original mihomo
	// memory issue has been resolved.
	if limit := os.Getenv("SUB_CHECK_MEM_LIMIT"); limit != "" {
		memoryLimit, err := human.FromHumanSize(limit)
		if err != nil {
			slog.Error("Invalid memory limit parameter", "error", err)
			return
		}

		if memoryLimit == 0 {
			return
		}

		go func() {
			for {
				time.Sleep(30 * time.Second)
				checkMemory(uint64(memoryLimit))
			}
		}()
	}

	// Add memory usage monitoring.
	if strings.ToLower(os.Getenv("SUB_CHECK_MEM_MONITOR")) != "" {
		go func() {
			var m runtime.MemStats
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				runtime.ReadMemStats(&m)
				slog.Info("Memory usage",
					"Alloc", formatBytes(m.Alloc),
					"TotalAlloc", formatBytes(m.TotalAlloc),
					"Sys", formatBytes(m.Sys),
					"HeapAlloc", formatBytes(m.HeapAlloc),
					"HeapSys", formatBytes(m.HeapSys),
					"HeapInuse", formatBytes(m.HeapInuse),
					"HeapIdle", formatBytes(m.HeapIdle),
					"HeapReleased", formatBytes(m.HeapReleased),
					"HeapObjects", m.HeapObjects,
					"StackInuse", formatBytes(m.StackInuse),
					"StackSys", formatBytes(m.StackSys),
					"MSpanInuse", formatBytes(m.MSpanInuse),
					"MSpanSys", formatBytes(m.MSpanSys),
					"MCacheInuse", formatBytes(m.MCacheInuse),
					"MCacheSys", formatBytes(m.MCacheSys),
					"BuckHashSys", formatBytes(m.BuckHashSys),
					"GCSys", formatBytes(m.GCSys),
					"OtherSys", formatBytes(m.OtherSys),
					"NextGC", formatBytes(m.NextGC),
					"LastGC", time.Unix(0, int64(m.LastGC)).Format("15:04:05"),
					"PauseTotalNs", m.PauseTotalNs,
					"NumGC", m.NumGC,
					"NumForcedGC", m.NumForcedGC,
					"GCCPUFraction", m.GCCPUFraction,
				)
			}
		}()
	}
}

// checkMemory checks memory usage.
func checkMemory(memoryLimit uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentUsage := m.HeapAlloc + m.StackInuse
	if currentUsage > memoryLimit {
		metadata := m.Sys - m.HeapSys - m.StackSys
		heapFrag := m.HeapInuse - m.HeapAlloc
		approxRSS := m.HeapAlloc + m.StackInuse + metadata + heapFrag
		slog.Warn("Memory usage exceeded the limit",
			"rss", human.HumanSize(float64(approxRSS)),
			"metadata", human.HumanSize(float64(metadata)),
			"heapFrag", human.HumanSize(float64(heapFrag)),
			"limit", human.HumanSize(float64(memoryLimit)))

		// Restart this process.
		cmd := getSelfCommand()
		if cmd != nil {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Start() // Let the new process start.
			slog.Warn("Started a new process due to memory pressure; binary users can close this window/terminal to stop it")
		}

		// Exit the current process.
		os.Exit(1)
	}
}

// getSelfCommand returns the current program path and arguments.
func getSelfCommand() *exec.Cmd {
	exePath, err := os.Executable()
	if err != nil {
		slog.Error("Failed to get executable path:", "error", err)
		return nil
	}
	args := os.Args[1:] // Get arguments excluding the program name.
	slog.Warn("🔄 Process is about to restart...", "path", exePath, "args", args)
	return exec.Command(exePath, args...)
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
