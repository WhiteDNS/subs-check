package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/utils"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// initConfigPath initializes the config file path.
func (app *App) initConfigPath() error {
	if app.configPath == "" {
		execPath := utils.GetExecutablePath()
		configDir := filepath.Join(execPath, "config")

		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		app.configPath = filepath.Join(configDir, "config.yaml")
	}
	return nil
}

// loadConfig loads the config file.
func (app *App) loadConfig() error {
	yamlFile, err := os.ReadFile(app.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return app.createDefaultConfig()
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(yamlFile, config.GlobalConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	slog.Info("Config file loaded")
	return nil
}

// createDefaultConfig creates the default config file.
func (app *App) createDefaultConfig() error {
	slog.Info("Config file does not exist; creating default config file")

	if err := os.WriteFile(app.configPath, []byte(config.DefaultConfigTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write default config file: %w", err)
	}

	slog.Info("Default config file created")
	slog.Info(fmt.Sprintf("Please edit the config file: %s", app.configPath))
	os.Exit(0)
	return nil
}

// initConfigWatcher initializes config file watching.
func (app *App) initConfigWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	app.watcher = watcher

	// Debounce timer. Editors such as VS Code may create a temp file before
	// overwriting, which produces two write events.
	var debounceTimer *time.Timer
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if absPath, _ := filepath.Abs(app.configPath); event.Name != absPath {
					continue
				}
				// Support changes made from outside the container.
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Reset the timer if it already exists.
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					// Create a new timer with a 100ms delay.
					debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
						slog.Info("Config file changed; reloading")
						oldCronExpr := config.GlobalConfig.CronExpression
						oldInterval := app.interval

						if err := app.loadConfig(); err != nil {
							slog.Error(fmt.Sprintf("Failed to reload config file: %v", err))
							return
						}

						// Check whether the cron expression or check interval changed.
						if oldCronExpr != config.GlobalConfig.CronExpression ||
							oldInterval != config.GlobalConfig.CheckInterval {

							app.interval = func() int {
								if config.GlobalConfig.CheckInterval <= 0 {
									return 1
								}
								return config.GlobalConfig.CheckInterval
							}()
							slog.Warn("Check settings changed; reconfiguring timer")

							// Reconfigure the timer through setTimer.
							app.setTimer()
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error(fmt.Sprintf("Config file watcher error: %v", err))
			}
		}
	}()

	// Start watching the config file directory.
	if err := watcher.Add(filepath.Dir(app.configPath)); err != nil {
		return fmt.Errorf("failed to add config file watcher: %w", err)
	}

	slog.Info("Config file watcher started")
	return nil
}
