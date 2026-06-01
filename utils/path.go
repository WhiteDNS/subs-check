package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"log/slog"
)

func GetExecutablePath() string {
	ex, err := os.Executable()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get program path: %v", err))
		return "."
	}
	return filepath.Dir(ex)
}
