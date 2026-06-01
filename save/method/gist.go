package method

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/beck-8/subs-check/config"
)

var (
	gistAPIURL     = "https://api.github.com/gists"
	gistMaxRetries = 3
	gistRetryDelay = 2 * time.Second
)

// GistFile represents the Gist file structure.
type GistFile struct {
	Content string `json:"content"`
}

// GistPayload represents the request body for creating/updating a Gist.
type GistPayload struct {
	Description string              `json:"description"`
	Public      bool                `json:"public"`
	Files       map[string]GistFile `json:"files"`
}

// GistUploader handles GitHub Gist uploads.
type GistUploader struct {
	client   *http.Client
	token    string
	id       string
	isPublic bool
}

// NewGistUploader creates a new Gist uploader.
func NewGistUploader() *GistUploader {
	if config.GlobalConfig.GithubAPIMirror != "" {
		gistAPIURL = strings.TrimRight(config.GlobalConfig.GithubAPIMirror, "/") + "/gists"
	}
	return &GistUploader{
		client:   &http.Client{Timeout: 30 * time.Second},
		token:    config.GlobalConfig.GithubToken,
		id:       config.GlobalConfig.GithubGistID,
		isPublic: false,
	}
}

// UploadToGist is the entry point for uploading data to Gist.
func UploadToGist(yamlData []byte, filename string) error {
	uploader := NewGistUploader()
	return uploader.Upload(yamlData, filename)
}

// ValiGistConfig validates Gist configuration.
func ValiGistConfig() error {
	if config.GlobalConfig.GithubToken == "" {
		return fmt.Errorf("github token is not configured")
	}
	if config.GlobalConfig.GithubGistID == "" {
		return fmt.Errorf("gist id is not configured")
	}
	return nil
}

// Upload performs the upload.
func (g *GistUploader) Upload(yamlData []byte, filename string) error {
	if err := g.validateInput(yamlData, filename); err != nil {
		return err
	}

	payload := GistPayload{
		Description: "Configuration",
		Public:      g.isPublic,
		Files: map[string]GistFile{
			filename: {
				Content: string(yamlData),
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	return g.uploadWithRetry(jsonData, filename)
}

// validateInput validates input parameters.
func (g *GistUploader) validateInput(yamlData []byte, filename string) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("yaml data is empty")
	}
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if g.token == "" {
		return fmt.Errorf("github token is not configured")
	}
	return nil
}

// uploadWithRetry uploads with retries.
func (g *GistUploader) uploadWithRetry(jsonData []byte, filename string) error {
	var lastErr error

	for attempt := 0; attempt < gistMaxRetries; attempt++ {
		if err := g.doUpload(jsonData); err != nil {
			lastErr = err
			slog.Error(fmt.Sprintf("gist upload failed (attempt %d/%d) %v", attempt+1, gistMaxRetries, err))
			time.Sleep(gistRetryDelay)
			continue
		}
		slog.Info("gist upload succeeded", "filename", filename)
		return nil
	}

	return fmt.Errorf("gist upload failed after %d retries: %w", gistMaxRetries, lastErr)
}

// doUpload performs a single upload.
func (g *GistUploader) doUpload(jsonData []byte) error {
	req, err := g.createRequest(jsonData)
	if err != nil {
		return err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return g.checkResponse(resp)
}

// createRequest creates an HTTP request.
func (g *GistUploader) createRequest(jsonData []byte) (*http.Request, error) {
	url := gistAPIURL + "/" + g.id
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	return req, nil
}

// checkResponse checks the response.
func (g *GistUploader) checkResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response (status code: %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("upload failed (status code: %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
