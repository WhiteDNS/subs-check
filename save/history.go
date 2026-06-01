package save

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/save/method"
	"gopkg.in/yaml.v3"
)

const (
	historyDir        = "history"
	historyPrefix     = "all_"
	historyTimeFormat = "2006-01-02_1504"
)

// SaveHistory saves a node snapshot for this check, such as history/all_2026-04-07_1430.yaml.
func SaveHistory(yamlData []byte) {
	dir := getHistoryDir()
	if dir == "" {
		return
	}
	os.MkdirAll(dir, 0755)

	filename := fmt.Sprintf("%s%s.yaml", historyPrefix, time.Now().Format(historyTimeFormat))
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		slog.Error(fmt.Sprintf("Failed to save history snapshot: %v", err))
		return
	}
	slog.Info(fmt.Sprintf("Saved history snapshot: %s", filename))
}

// LoadHistoryProxies loads history nodes from the last N days and removes expired files.
func LoadHistoryProxies() []map[string]any {
	dir := getHistoryDir()
	if dir == "" {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -config.GlobalConfig.KeepDays)
	pattern := filepath.Join(dir, historyPrefix+"*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	var allProxies []map[string]any
	for _, f := range files {
		t, ok := parseTimeFromFilename(filepath.Base(f))
		if !ok {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(f)
			slog.Debug(fmt.Sprintf("Removed expired history file: %s", filepath.Base(f)))
			continue
		}
		proxies := loadProxiesFromYaml(f)
		allProxies = append(allProxies, proxies...)
	}

	return allProxies
}

// parseTimeFromFilename parses time from a filename.
// all_2026-04-07_1430.yaml -> 2026-04-07 14:30
func parseTimeFromFilename(name string) (time.Time, bool) {
	name = strings.TrimPrefix(name, historyPrefix)
	name = strings.TrimSuffix(name, ".yaml")
	t, err := time.ParseInLocation(historyTimeFormat, name, time.Local)
	return t, err == nil
}

func loadProxiesFromYaml(path string) []map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to read history file: %s, %v", filepath.Base(path), err))
		return nil
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		slog.Warn(fmt.Sprintf("Failed to parse history file: %s, %v", filepath.Base(path), err))
		return nil
	}
	return doc.Proxies
}

func getHistoryDir() string {
	saver, err := method.NewLocalSaver()
	if err != nil {
		return ""
	}
	return filepath.Join(saver.OutputPath, historyDir)
}
