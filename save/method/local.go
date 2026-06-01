package method

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/utils"
)

const (
	outputDirName = "output"
	fileMode      = 0644
	dirMode       = 0755
)

// LocalSaver handles local file saves.
type LocalSaver struct {
	BasePath   string
	OutputPath string
}

// NewLocalSaver creates a new local saver.
func NewLocalSaver() (*LocalSaver, error) {
	basePath := utils.GetExecutablePath()
	if basePath == "" {
		return nil, fmt.Errorf("failed to get executable path")
	}

	var outputPath string
	if config.GlobalConfig.OutputDir != "" {
		outputPath = config.GlobalConfig.OutputDir
	} else {
		outputPath = filepath.Join(basePath, outputDirName)
	}

	return &LocalSaver{
		BasePath:   basePath,
		OutputPath: outputPath,
	}, nil
}

// SaveToLocal saves config to a local file.
func SaveToLocal(yamlData []byte, filename string) error {
	saver, err := NewLocalSaver()
	if err != nil {
		return fmt.Errorf("failed to create local saver: %w", err)
	}

	return saver.Save(yamlData, filename)
}

// Save performs the save operation.
func (ls *LocalSaver) Save(yamlData []byte, filename string) error {
	// Ensure the output directory exists.
	if err := ls.ensureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate input parameters.
	if err := ls.validateInput(yamlData, filename); err != nil {
		return err
	}

	// Build the file path and save.
	filepath := filepath.Join(ls.OutputPath, filename)

	if err := os.WriteFile(filepath, yamlData, fileMode); err != nil {
		return fmt.Errorf("failed to write file [%s]: %w", filename, err)
	}
	slog.Info("Saved local file", "filepath", filepath)

	return nil
}

// ensureOutputDir ensures the output directory exists.
func (ls *LocalSaver) ensureOutputDir() error {
	if _, err := os.Stat(ls.OutputPath); os.IsNotExist(err) {
		if err := os.MkdirAll(ls.OutputPath, dirMode); err != nil {
			return fmt.Errorf("failed to create directory [%s]: %w", ls.OutputPath, err)
		}
	}
	return nil
}

// validateInput validates input parameters.
func (ls *LocalSaver) validateInput(yamlData []byte, filename string) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("yaml data is empty")
	}

	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check whether the filename contains illegal characters.
	if filepath.Base(filename) != filename {
		return fmt.Errorf("filename contains illegal characters: %s", filename)
	}

	return nil
}
