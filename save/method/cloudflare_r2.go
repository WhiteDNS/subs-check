package method

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"log/slog"

	"github.com/beck-8/subs-check/config"
)

const (
	maxRetries    = 3
	retryInterval = 2 * time.Second
)

// KVPayload defines the data structure uploaded to R2.
type KVPayload struct {
	Filename string `json:"filename"`
	Value    string `json:"value"`
}

// R2Uploader handles R2 storage uploads.
type R2Uploader struct {
	client    *http.Client
	workerURL string
	token     string
}

// NewR2Uploader creates a new R2 uploader.
func NewR2Uploader() *R2Uploader {
	return &R2Uploader{
		client:    &http.Client{Timeout: 30 * time.Second},
		workerURL: config.GlobalConfig.WorkerURL,
		token:     config.GlobalConfig.WorkerToken,
	}
}

// UploadToR2Storage is the entry point for uploading data to R2 storage.
func UploadToR2Storage(yamlData []byte, filename string) error {
	uploader := NewR2Uploader()
	return uploader.Upload(yamlData, filename)
}

// ValiR2Config validates R2 configuration.
func ValiR2Config() error {
	if config.GlobalConfig.WorkerURL == "" {
		return fmt.Errorf("worker url is not configured")
	}
	if config.GlobalConfig.WorkerToken == "" {
		return fmt.Errorf("worker token is not configured")
	}
	return nil
}

// Upload performs the upload.
func (r *R2Uploader) Upload(yamlData []byte, filename string) error {
	// Validate input.
	if err := r.validateInput(yamlData, filename); err != nil {
		return err
	}

	// Prepare request data.
	payload := KVPayload{
		Filename: filename,
		Value:    string(yamlData),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	// Upload with retries.
	return r.uploadWithRetry(jsonData, filename)
}

// validateInput validates input parameters.
func (r *R2Uploader) validateInput(yamlData []byte, filename string) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("yaml data is empty")
	}
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if r.workerURL == "" || r.token == "" {
		return fmt.Errorf("Worker configuration is incomplete")
	}
	return nil
}

// uploadWithRetry uploads with retries.
func (r *R2Uploader) uploadWithRetry(jsonData []byte, filename string) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := r.doUpload(jsonData); err != nil {
			lastErr = err
			slog.Error(fmt.Sprintf("R2 upload failed (attempt %d/%d) %v", attempt+1, maxRetries, err))
			time.Sleep(retryInterval)
			continue
		}
		slog.Info("R2 upload succeeded", "filename", filename)
		return nil
	}

	return fmt.Errorf("upload failed after %d retries: %w", maxRetries, lastErr)
}

// doUpload performs a single upload.
func (r *R2Uploader) doUpload(jsonData []byte) error {
	// Create request.
	req, err := r.createRequest(jsonData)
	if err != nil {
		return err
	}

	// Send request.
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response.
	return r.checkResponse(resp)
}

// createRequest creates an HTTP request.
func (r *R2Uploader) createRequest(jsonData []byte) (*http.Request, error) {
	url := fmt.Sprintf("%s/storage?token=%s", r.workerURL, r.token)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// checkResponse checks the response.
func (r *R2Uploader) checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response (status code: %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("upload failed (status code: %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
