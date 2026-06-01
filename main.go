package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/beck-8/subs-check/app"
)

func main() {
	application := app.New(fmt.Sprintf("%s-%s", Version, CurrentCommit))
	slog.Info(fmt.Sprintf("Current version: %s-%s", Version, CurrentCommit))

	if err := application.Initialize(); err != nil {
		slog.Error(fmt.Sprintf("Initialization failed: %v", err))
		os.Exit(1)
	}

	application.Run()
}
