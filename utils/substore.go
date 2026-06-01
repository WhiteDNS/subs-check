package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beck-8/subs-check/config"
)

// ============================================================================
// sub-store data model overview. The backend is schemaless and stores JSON; the fields are listed below.
//
// Two top-level objects:
//   - subscription  /api/sub/:name   -- a source for a group of nodes. We use a
//     local subscription named "sub" (source=local) to hold checked nodes in
//     the content field as YAML. See the sub struct.
//   - file          /api/file/:name  -- an output file generated from a source.
//     We use a file named "mihomo" (type=mihomoProfile, sourceName="sub") for
//     the mihomo override. See the file struct.
//
// Both objects have a process pipeline (array), where each item is an Operator:
//   { type, args, disabled, customName, id }
//   - type:          operator type, such as "Quick Setting Operator", "Script Operator", or "Sort Operator"
//   - args:          parameters. The type depends on the operator and can be an object, string, or array, so any is used here
//   - disabled:      whether the operator is disabled
//   - customName/id: frontend metadata. The sub-store backend reads only
//     type/args/disabled, and customName is only a frontend label, so we use it
//     as a marker for "this is a subs-check operator" (see overwriteOpMarker).
//
// API notes:
//   - Create with POST /api/subs and /api/files.
//   - Update with PATCH /api/sub/:name and /api/file/:name. PATCH is a shallow
//     merge ({...old, ...body}), so we only send fields we own and preserve user
//     changes elsewhere (see updateSub / updatefile).
//   - Checks only inspect the returned status field (see statusResult). Missing
//     resources return HTTP 500 from both endpoints because sub-store maps its
//     ResourceNotFoundError 404 into error.details and the real status falls
//     back to 500: /api/sub returns 500+HTML, /api/wholeFile returns
//     500+JSON(status=failed). Check status code before parsing.
//   - /api/wholeFile returns only the raw stored object and does no conversion,
//     so status!=success only means "file missing", not conversion failed. The
//     endpoint that actually converts and may fail is /api/file/:name (produce,
//     500 on failure, 404 when missing).
//
// Note: fields below with omitempty that we do not write (displayName/ua/...)
// are kept only as field documentation.
// References: ../sub-store/backend/src/restful/{subscriptions,file}.js
//        ../sub-store/backend/src/core/proxy-utils/processors/index.js
// ============================================================================

// sub maps to fields in sub-store /api/sub.
// Reference: ../sub-store/backend/src/restful/subscriptions.js
// We only write required fields. Other fields have omitempty and are kept only
// as documentation, so they will not be serialized.
type sub struct {
	Name                  string     `json:"name"`
	DisplayName           string     `json:"displayName,omitempty"`
	Source                string     `json:"source"` // local | remote
	URL                   string     `json:"url,omitempty"`
	Content               string     `json:"content,omitempty"`
	UA                    string     `json:"ua,omitempty"`
	MergeSources          string     `json:"mergeSources,omitempty"`
	IgnoreFailedRemoteSub bool       `json:"ignoreFailedRemoteSub,omitempty"`
	Proxy                 string     `json:"proxy,omitempty"`
	Process               []Operator `json:"process,omitempty"`
	Remark                string     `json:"remark,omitempty"`
	Tag                   []string   `json:"tag,omitempty"`
	SubscriptionTags      []string   `json:"subscriptionTags,omitempty"`
}

// file maps to fields in sub-store /api/file.
// Reference: ../sub-store/backend/src/restful/file.js
type file struct {
	Name                   string     `json:"name"`
	DisplayName            string     `json:"displayName,omitempty"`
	Source                 string     `json:"source"`               // local | remote
	SourceType             string     `json:"sourceType,omitempty"` // subscription | collection
	SourceName             string     `json:"sourceName,omitempty"`
	Type                   string     `json:"type"` // mihomoProfile | ...
	URL                    string     `json:"url,omitempty"`
	Content                string     `json:"content,omitempty"`
	UA                     string     `json:"ua,omitempty"`
	MergeSources           string     `json:"mergeSources,omitempty"`
	IgnoreFailedRemoteFile bool       `json:"ignoreFailedRemoteFile,omitempty"`
	Proxy                  string     `json:"proxy,omitempty"`
	Process                []Operator `json:"process,omitempty"`
	Remark                 string     `json:"remark,omitempty"`
}

// Operator is a process item. In sub-store, args type varies by operator:
// Script/Quick Setting Operator uses an object, Sort Operator uses the string
// "asc", Regex operators use strings/arrays, and so on, so Args uses any.
// Reference: ../sub-store/backend/src/core/proxy-utils/processors/index.js
type Operator struct {
	Type       string `json:"type"`
	Args       any    `json:"args,omitempty"`
	Disabled   bool   `json:"disabled,omitempty"`
	CustomName string `json:"customName,omitempty"` // Marker for our override operator.
}

// scriptArgs is the args payload for a Script Operator.
type scriptArgs struct {
	Content string `json:"content"`
	Mode    string `json:"mode"`
}

// statusResult is only used to check whether an API response succeeded. Do not
// parse data strongly because Operator args types vary and strict parsing can fail.
type statusResult struct {
	Status string `json:"status"`
}

const (
	SubName    = "sub"
	MihomoName = "mihomo"
	// Marker for our override operator, written to the process item's customName.
	// sub-store reads only type/args/disabled during processing; customName is a
	// frontend label and can safely be used as a marker.
	overwriteOpMarker       = "subs-check reserved, do not edit"
	legacyOverwriteOpMarker = "subs-check\u4e13\u7528,\u52ff\u52a8"
)

// Tracks whether the user changed the override subscription URL at runtime.
var mihomoOverwriteUrl string

// BaseURL is the configured base URL.
var BaseURL string

func UpdateSubStore(yamlData []byte) {
	// Wait for node startup during tests/debugging.
	if os.Getenv("SUB_CHECK_SKIP") != "" && config.GlobalConfig.SubStorePort != "" {
		time.Sleep(time.Second * 1)
	}
	// Normalize the user input format.
	config.GlobalConfig.SubStorePort = formatPort(config.GlobalConfig.SubStorePort)
	// Set base URL.
	BaseURL = fmt.Sprintf("http://127.0.0.1%s", config.GlobalConfig.SubStorePort)
	if config.GlobalConfig.SubStorePath != "" {
		BaseURL = fmt.Sprintf("%s%s", BaseURL, config.GlobalConfig.SubStorePath)
	}

	if err := checkSub(); err != nil {
		slog.Debug(fmt.Sprintf("Sub config check failed: %v; creating...", err))
		if err := createSub(yamlData); err != nil {
			slog.Error(fmt.Sprintf("Failed to create sub config: %v", err))
			return
		}
	}
	if config.GlobalConfig.MihomoOverwriteUrl == "" {
		slog.Error("mihomo override subscription URL is not set")
		return
	}
	if err := checkfile(); err != nil {
		slog.Debug(fmt.Sprintf("mihomo config check failed: %v; creating...", err))
		if err := createfile(); err != nil {
			slog.Error(fmt.Sprintf("Failed to create mihomo config: %v", err))
			return
		}
		mihomoOverwriteUrl = config.GlobalConfig.MihomoOverwriteUrl
	}
	if err := updateSub(yamlData); err != nil {
		slog.Error(fmt.Sprintf("Failed to update sub config: %v", err))
		return
	}
	if config.GlobalConfig.MihomoOverwriteUrl != mihomoOverwriteUrl {
		if err := updatefile(); err != nil {
			slog.Error(fmt.Sprintf("Failed to update mihomo config: %v", err))
			return
		}
		mihomoOverwriteUrl = config.GlobalConfig.MihomoOverwriteUrl
	}
	slog.Info("substore update completed")
}
func checkSub() error {
	resp, err := http.Get(fmt.Sprintf("%s/api/sub/%s", BaseURL, SubName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// sub-store returns 500 + HTML for a missing sub instead of clean JSON. Check
	// the status code first to avoid feeding HTML into json.Unmarshal and logging
	// misleading "invalid character '<'" errors.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sub does not exist or sub-store is not ready (HTTP %d)", resp.StatusCode)
	}
	var result statusResult
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse sub response: %w", err)
	}
	if result.Status != "success" {
		return fmt.Errorf("failed to get sub config")
	}
	return nil
}
func createSub(data []byte) error {
	// sub-store upload default limit is 1MB.
	sub := sub{
		Content: string(data),
		Name:    "sub",
		Remark:  overwriteOpMarker,
		Source:  "local",
		Process: []Operator{
			{Type: "Quick Setting Operator"},
		},
	}
	json, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("%s/api/subs", BaseURL), "application/json", bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create sub config, status code: %d", resp.StatusCode)
	}
	return nil
}

func updateSub(data []byte) error {
	// PATCH is a shallow merge ({...old, ...body}); send only content to refresh
	// nodes and preserve user changes to other sub fields such as process,
	// remark, and tag.
	payload, err := json.Marshal(map[string]string{"content": string(data)})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/api/sub/%s", BaseURL, SubName),
		bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update sub config, status code: %d", resp.StatusCode)
	}
	return nil
}

func checkfile() error {
	resp, err := http.Get(fmt.Sprintf("%s/api/wholeFile/%s", BaseURL, MihomoName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mihomo file does not exist or sub-store is not ready (HTTP %d)", resp.StatusCode)
	}
	var result statusResult
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse mihomo file response: %w", err)
	}
	if result.Status != "success" {
		return fmt.Errorf("failed to get mihomo config")
	}
	return nil
}
func createfile() error {
	file := file{
		Name: MihomoName,
		Process: []Operator{
			{
				Type: "Script Operator",
				Args: scriptArgs{
					Content: WarpUrl(config.GlobalConfig.MihomoOverwriteUrl),
					Mode:    "link",
				},
				CustomName: overwriteOpMarker,
			},
		},
		Remark:     overwriteOpMarker,
		Source:     "local",
		SourceName: "sub",
		SourceType: "subscription",
		Type:       "mihomoProfile",
	}
	json, err := json.Marshal(file)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("%s/api/files", BaseURL), "application/json", bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create mihomo config, status code: %d", resp.StatusCode)
	}
	return nil
}

func updatefile() error {
	// Fetch the entire existing file and modify only our Script Operator's
	// override URL. Then PATCH only process through a shallow merge, preserving
	// other user-added operators and file fields.
	resp, err := http.Get(fmt.Sprintf("%s/api/wholeFile/%s", BaseURL, MihomoName))
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	var result struct {
		Data struct {
			Process []map[string]any `json:"process"`
		} `json:"data"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if result.Status != "success" {
		return fmt.Errorf("failed to get mihomo config")
	}

	newContent := WarpUrl(config.GlobalConfig.MihomoOverwriteUrl)
	process := result.Data.Process
	found := false
	changed := false

	// 1. Prefer finding our operator by marker to avoid confusing it with
	// user-added Script Operators.
	for _, op := range process {
		customName := op["customName"]
		if customName != overwriteOpMarker && customName != legacyOverwriteOpMarker {
			continue
		}
		if customName != overwriteOpMarker {
			changed = true
		}
		op["customName"] = overwriteOpMarker
		found = true
		if a, ok := op["args"].(map[string]any); ok {
			if c, _ := a["content"].(string); c != newContent {
				a["content"] = newContent
				changed = true
			}
		}
		break
	}
	// 2. Support legacy data created before markers existed: take the first
	// mode=link Script Operator, update content, and add the marker so future
	// runs can match it exactly.
	if !found {
		for _, op := range process {
			if op["type"] != "Script Operator" {
				continue
			}
			if a, ok := op["args"].(map[string]any); ok && a["mode"] == "link" {
				a["content"] = newContent
				op["customName"] = overwriteOpMarker
				found = true
				changed = true
				break
			}
		}
	}
	// 3. If none exists because the user deleted it, insert the marked operator first.
	if !found {
		process = append([]map[string]any{{
			"type":       "Script Operator",
			"args":       map[string]any{"content": newContent, "mode": "link"},
			"customName": overwriteOpMarker,
		}}, process...)
		changed = true
	}

	// If content did not change, do not PATCH or log. This avoids reporting
	// updates when the user did not change anything.
	if !changed {
		return nil
	}

	payload, err := json.Marshal(map[string]any{"process": process})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/api/file/%s", BaseURL, MihomoName),
		bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update mihomo config, status code: %d", resp.StatusCode)
	}
	slog.Debug("mihomo override subscription URL updated")
	return nil
}

// If the user listens on a LAN IP, later requests may fail; keep only the port.
func formatPort(port string) string {
	if strings.Contains(port, ":") {
		parts := strings.Split(port, ":")
		return ":" + parts[len(parts)-1]
	}
	return ":" + port
}

func WarpUrl(url string) string {
	url = formatTimePlaceholders(url, time.Now())

	// Use the GitHub proxy when the URL starts with https://raw.githubusercontent.com.
	if strings.HasPrefix(url, "https://raw.githubusercontent.com") {
		return config.GlobalConfig.GithubProxy + url
	}
	return url
}

// Dynamic time placeholders.
// Links can contain time placeholders that are replaced with the current date/time:
// - `{Y}` - four-digit year (2023)
// - `{m}` - two-digit month (01-12)
// - `{d}` - two-digit day (01-31)
// - `{Ymd}` - combined date (20230131)
// - `{Y_m_d}` - underscore-separated date (2023_01_31)
// - `{Y-m-d}` - hyphen-separated date (2023-01-31)
//
// All placeholders support an optional day-offset suffix `±N`:
// - `{Ymd+1}` - tomorrow's combined date
// - `{Y-m-d-7}` - hyphen-separated date 7 days ago
// - `{Y+1}` - the year of tomorrow's date, usually unchanged unless crossing year-end
// Offsets are always calculated in days, avoiding ambiguous month/year rollovers.
var timePlaceholderRe = regexp.MustCompile(`\{(Ymd|Y_m_d|Y-m-d|Y|m|d)([+-]\d+)?\}`)

var timePlaceholderLayouts = map[string]string{
	"Y":     "2006",
	"m":     "01",
	"d":     "02",
	"Ymd":   "20060102",
	"Y_m_d": "2006_01_02",
	"Y-m-d": "2006-01-02",
}

func formatTimePlaceholders(url string, t time.Time) string {
	return timePlaceholderRe.ReplaceAllStringFunc(url, func(s string) string {
		m := timePlaceholderRe.FindStringSubmatch(s)
		offset := 0
		if m[2] != "" {
			offset, _ = strconv.Atoi(m[2])
		}
		return t.AddDate(0, 0, offset).Format(timePlaceholderLayouts[m[1]])
	})
}
