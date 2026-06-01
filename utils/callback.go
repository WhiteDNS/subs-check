package utils

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/beck-8/subs-check/config"
)

// ExecuteCallback runs the callback script.
func ExecuteCallback(successCount int) {
	callbackScript := config.GlobalConfig.CallbackScript
	if callbackScript == "" {
		return
	}

	slog.Info(fmt.Sprintf("Executing callback script: %s", callbackScript))

	// Check whether the script file exists.
	if _, err := os.Stat(callbackScript); os.IsNotExist(err) {
		slog.Error(fmt.Sprintf("Callback script does not exist: %s", callbackScript))
		return
	}

	// On non-Windows systems, check and set execute permissions.
	if runtime.GOOS != "windows" {
		err := os.Chmod(callbackScript, 0755) // rwxr-xr-x permissions.
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to set script execute permissions: %v", err))
		}

		// Check whether the script has a shebang.
		content, err := os.ReadFile(callbackScript)
		if err == nil && len(content) > 0 {
			hasShebang := false
			if len(content) >= 2 && content[0] == '#' && content[1] == '!' {
				hasShebang = true
			}

			if !hasShebang {
				slog.Warn("Script is missing a shebang line; add one at the beginning, such as #!/bin/bash, #!/bin/sh, or #!/usr/bin/env bash")
			}
		}
	}

	// Choose an execution method based on the operating system.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows.
		if strings.HasSuffix(strings.ToLower(callbackScript), ".bat") ||
			strings.HasSuffix(strings.ToLower(callbackScript), ".cmd") {
			// Use the full path and handle paths with spaces correctly.
			absPath, err := filepath.Abs(callbackScript)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to get absolute script path: %v", err))
				return
			}
			cmd = exec.Command("cmd", "/C", absPath)
		} else if strings.HasSuffix(strings.ToLower(callbackScript), ".ps1") {
			// PowerShell script.
			absPath, err := filepath.Abs(callbackScript)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to get absolute script path: %v", err))
				return
			}
			// Use -ExecutionPolicy Bypass to bypass execution policy restrictions.
			cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", absPath)
		} else {
			cmd = exec.Command(callbackScript)
		}
		// Set the working directory to the script directory.
		cmd.Dir = filepath.Dir(callbackScript)
	} else {
		// Unix/Linux/macOS.
		cmd = exec.Command(callbackScript)
	}

	// Set environment variables and pass the successful node count.
	cmd.Env = append(os.Environ(), fmt.Sprintf("SUCCESS_COUNT=%d", successCount))

	// Execute command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error(fmt.Sprintf("Callback script failed: %v, output: %s", err, string(output)))
		return
	}
	slog.Info("Callback script completed successfully")
}
