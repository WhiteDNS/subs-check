package method

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"log/slog"

	"github.com/beck-8/subs-check/config"
)

var (
	webdavMaxRetries = 3
	webdavRetryDelay = 2 * time.Second
)

// WebDAVUploader handles WebDAV uploads.
type WebDAVUploader struct {
	client   *http.Client
	baseURL  string
	username string
	password string
}

// NewWebDAVUploader creates a new WebDAV uploader.
func NewWebDAVUploader() *WebDAVUploader {
	return &WebDAVUploader{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  config.GlobalConfig.WebDAVURL,
		username: config.GlobalConfig.WebDAVUsername,
		password: config.GlobalConfig.WebDAVPassword,
	}
}

// UploadToWebDAV is the entry point for uploading data to WebDAV.
func UploadToWebDAV(yamlData []byte, filename string) error {
	uploader := NewWebDAVUploader()
	return uploader.Upload(yamlData, filename)
}

// ValiWebDAVConfig validates WebDAV configuration.
func ValiWebDAVConfig() error {
	if config.GlobalConfig.WebDAVURL == "" {
		return fmt.Errorf("webdav URL is not configured")
	}
	if config.GlobalConfig.WebDAVUsername == "" {
		return fmt.Errorf("webdav username is not configured")
	}
	if config.GlobalConfig.WebDAVPassword == "" {
		return fmt.Errorf("webdav password is not configured")
	}
	return nil
}

// Upload performs the upload.
func (w *WebDAVUploader) Upload(yamlData []byte, filename string) error {
	if err := w.validateInput(yamlData, filename); err != nil {
		return err
	}

	return w.uploadWithRetry(yamlData, filename)
}

// validateInput validates input parameters.
func (w *WebDAVUploader) validateInput(yamlData []byte, filename string) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("yaml data is empty")
	}
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if w.baseURL == "" {
		return fmt.Errorf("webdav URL is not configured")
	}
	return nil
}

// uploadWithRetry uploads with retries.
func (w *WebDAVUploader) uploadWithRetry(yamlData []byte, filename string) error {
	var lastErr error

	for attempt := 0; attempt < webdavMaxRetries; attempt++ {
		if err := w.doUpload(yamlData, filename); err != nil {
			lastErr = err
			slog.Error(fmt.Sprintf("webdav upload failed (attempt %d/%d) %v", attempt+1, webdavMaxRetries, err))
			time.Sleep(webdavRetryDelay)
			continue
		}
		slog.Info("webdav upload succeeded", "filename", filename)
		return nil
	}

	return fmt.Errorf("webdav upload failed after %d retries: %w", webdavMaxRetries, lastErr)
}

// doUpload performs a single upload.
func (w *WebDAVUploader) doUpload(yamlData []byte, filename string) error {
	req, err := w.createRequest(yamlData, filename)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return w.checkResponse(resp)
}

// createRequest creates an HTTP request.
func (w *WebDAVUploader) createRequest(yamlData []byte, filename string) (*http.Request, error) {
	baseURL := w.baseURL
	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}

	url := baseURL + filename

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(yamlData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(w.username, w.password)
	req.Header.Set("Content-Type", "application/x-yaml")
	return req, nil
}

// checkResponse checks the response.
func (w *WebDAVUploader) checkResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response (status code: %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("upload failed (status code: %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
