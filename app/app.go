package app

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/beck-8/subs-check/app/monitor"
	"github.com/beck-8/subs-check/assets"
	"github.com/beck-8/subs-check/check"
	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/save"
	"github.com/beck-8/subs-check/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
)

// App manages application state.
type App struct {
	configPath string
	interval   int
	watcher    *fsnotify.Watcher
	checkChan  chan struct{} // Channel used to trigger checks.
	checking   atomic.Bool   // Check state flag.
	ticker     *time.Ticker
	done       chan struct{} // Signal used to stop the ticker goroutine.
	cron       *cron.Cron    // Crontab scheduler.
	version    string
}

// New creates a new application instance.
func New(version string) *App {
	configPath := flag.String("f", "", "config file path")
	flag.Parse()

	return &App{
		configPath: *configPath,
		checkChan:  make(chan struct{}),
		done:       make(chan struct{}),
		version:    version,
	}
}

// Initialize initializes the application.
func (app *App) Initialize() error {
	// Initialize config file path.
	if err := app.initConfigPath(); err != nil {
		return fmt.Errorf("failed to initialize config file path: %w", err)
	}

	// Load config file.
	if err := app.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Initialize DNS resolver before any proxy connection because it affects mihomo's global resolver.
	if err := initResolver(); err != nil {
		return fmt.Errorf("failed to initialize DNS: %w", err)
	}

	// Initialize config file watcher.
	if err := app.initConfigWatcher(); err != nil {
		return fmt.Errorf("failed to initialize config file watcher: %w", err)
	}

	// Read the proxy from config and set proxy environment variables.
	if config.GlobalConfig.Proxy != "" {
		os.Setenv("HTTP_PROXY", config.GlobalConfig.Proxy)
		os.Setenv("HTTPS_PROXY", config.GlobalConfig.Proxy)
	}

	app.interval = func() int {
		if config.GlobalConfig.CheckInterval <= 0 {
			return 1
		}
		return config.GlobalConfig.CheckInterval
	}()

	if config.GlobalConfig.ListenPort != "" {
		if err := app.initHttpServer(); err != nil {
			return fmt.Errorf("failed to initialize HTTP server: %w", err)
		}
	}

	if config.GlobalConfig.SubStorePort != "" {
		if runtime.GOOS == "linux" && runtime.GOARCH == "386" {
			slog.Warn("node does not support 32-bit Linux; sub-store service will not start")
		}
		go assets.RunSubStoreService()
		// Wait briefly so logs are emitted in the expected order.
		time.Sleep(500 * time.Millisecond)
	}

	// Start memory monitoring.
	monitor.StartMemoryMonitor()

	// Set up signal handlers.
	utils.SetupSignalHandler(check.RequestCancel)
	return nil
}

// Run runs the application main loop.
func (app *App) Run() {
	defer func() {
		app.watcher.Close()
		if app.ticker != nil {
			app.ticker.Stop()
		}
		if app.cron != nil {
			app.cron.Stop()
		}
	}()

	// Set the initial timer mode.
	app.setTimer()

	// Run the first check immediately only when the cron expression is empty.
	if config.GlobalConfig.CronExpression != "" {
		slog.Warn("Cron expression is configured; the first check will not run immediately")
	} else {
		app.triggerCheck()
	}

	// Handle manual triggers in the main loop.
	for range app.checkChan {
		go app.triggerCheck()
	}
}

// setTimer configures the timer from config.
func (app *App) setTimer() {
	// Stop the existing timer.
	if app.ticker != nil {
		// Send the stop signal first to avoid panics after nil assignment.
		close(app.done)                // Send stop signal.
		app.done = make(chan struct{}) // Create a new channel.
		app.ticker.Stop()
		app.ticker = nil
	}

	// Stop the existing cron scheduler.
	if app.cron != nil {
		app.cron.Stop()
		app.cron = nil
	}

	// Check whether a cron expression is configured.
	if config.GlobalConfig.CronExpression != "" {
		slog.Info(fmt.Sprintf("Using cron expression: %s", config.GlobalConfig.CronExpression))
		app.cron = cron.New()
		_, err := app.cron.AddFunc(config.GlobalConfig.CronExpression, func() {
			app.triggerCheck()
		})
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to parse cron expression '%s': %v; falling back to check interval",
				config.GlobalConfig.CronExpression, err))
			// Use interval mode.
			app.useIntervalTimer()
		} else {
			app.cron.Start()
		}
	} else {
		// Use interval mode.
		app.useIntervalTimer()
	}
}

// useIntervalTimer runs in interval mode.
func (app *App) useIntervalTimer() {
	// Initialize timer.
	app.ticker = time.NewTicker(time.Duration(app.interval) * time.Minute)
	done := app.done
	// Start a goroutine to listen for timer events.
	go func() {
		for {
			select {
			case <-app.ticker.C:
				app.triggerCheck()
			case <-done:
				return // Stop signal received; exit goroutine.
			}
		}
	}()
}

// TriggerCheck exposes check triggering to callers.
func (app *App) TriggerCheck() {
	select {
	case app.checkChan <- struct{}{}:
		slog.Info("Manual check triggered")
	default:
		slog.Warn("A check is already running; ignoring this trigger")
	}
}

// triggerCheck runs the internal check flow.
func (app *App) triggerCheck() {
	// Return immediately if a check is already running.
	if !app.checking.CompareAndSwap(false, true) {
		slog.Warn("A check is already running; skipping this check")
		return
	}
	defer app.checking.Store(false)

	if err := app.checkProxies(); err != nil {
		slog.Error(fmt.Sprintf("Proxy check failed: %v", err))
		os.Exit(1)
	}

	// Show the next check time after a check completes.
	if app.ticker != nil {
		// Interval mode.
		app.ticker.Reset(time.Duration(app.interval) * time.Minute)
		nextCheck := time.Now().Add(time.Duration(app.interval) * time.Minute)
		slog.Info(fmt.Sprintf("Next check time: %s", nextCheck.Format("2006-01-02 15:04:05")))
	} else if app.cron != nil {
		// Cron mode.
		entries := app.cron.Entries()
		if len(entries) > 0 {
			nextTime := entries[0].Next
			slog.Info(fmt.Sprintf("Next check time: %s", nextTime.Format("2006-01-02 15:04:05")))
		}
	}
	debug.FreeOSMemory()
}

// checkProxies runs proxy checks.
func (app *App) checkProxies() error {
	slog.Info("Preparing proxy checks", "show-progress", config.GlobalConfig.PrintProgress)

	// Load historical usable nodes into the check queue.
	if config.GlobalConfig.KeepDays > 0 {
		if hp := save.LoadHistoryProxies(); len(hp) > 0 {
			config.GlobalProxies = append(config.GlobalProxies, hp...)
		}
	}

	results, err := check.Check()
	if err != nil {
		return fmt.Errorf("proxy check failed: %w", err)
	}
	slog.Info("Check completed")
	save.SaveConfig(results)
	utils.SendNotify(len(results))
	utils.UpdateSubs()

	// Execute callback script.
	utils.ExecuteCallback(len(results))

	return nil
}
