package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/beck-8/subs-check/config"
)

// NotifyRequest defines the notification request body.
type NotifyRequest struct {
	URLs  string `json:"urls"`  // Notification target URL, such as mailto:// or discord://.
	Body  string `json:"body"`  // Notification body.
	Title string `json:"title"` // Notification title (optional).
}

// Notify sends a notification.
func Notify(request NotifyRequest) error {
	// Build request body.
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to build request body: %w", err)
	}

	// Send request.
	resp, err := http.Post(config.GlobalConfig.AppriseApiServer, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification failed, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

func SendNotify(length int) {
	if config.GlobalConfig.AppriseApiServer == "" {
		return
	} else if len(config.GlobalConfig.RecipientUrl) == 0 {
		slog.Error("No notification targets configured")
		return
	}

	for _, url := range config.GlobalConfig.RecipientUrl {
		request := NotifyRequest{
			URLs: url,
			Body: fmt.Sprintf("✅ Usable nodes: %d\n🕒 %s",
				length,
				GetCurrentTime()),
			Title: config.GlobalConfig.NotifyTitle,
		}
		var err error
		for i := 0; i < config.GlobalConfig.SubUrlsReTry; i++ {
			err = Notify(request)
			if err == nil {
				slog.Info(fmt.Sprintf("%s notification sent successfully", strings.SplitN(url, "://", 2)[0]))
				break
			}
		}
		if err != nil {
			slog.Error(fmt.Sprintf("%s notification failed: %v", strings.SplitN(url, "://", 2)[0], err))
		}
	}
}

func GetCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
