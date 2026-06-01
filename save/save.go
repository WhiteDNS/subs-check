package save

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/beck-8/subs-check/check"
	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/save/method"
	"github.com/beck-8/subs-check/utils"
	"gopkg.in/yaml.v3"
)

// SaveFunc defines the save method function signature.
type SaveFunc func(data []byte, filename string) error

// SaveConfig saves check results locally and optionally to remote storage.
//
// Order matters:
//  1. Serialize results to history first while proxy["name"] is still original,
//     so history stays clean and keep-days does not accumulate tags on reload.
//  2. Mutate each result.Proxy["name"] in place to the final display name from
//     check.RenderName: base + media tags + speed tag + sub_tag.
//  3. Serialize the mutated results to all.yaml, mihomo.yaml, and base64.txt,
//     then write them locally, remotely, and to SubStore.
//
// Implicit contract: after SaveConfig returns, results are considered consumed.
// Callers should not read results[i].Proxy["name"], because it is now the display
// name rather than the original name.
func SaveConfig(results []check.Result) {
	// Zero nodes is a common valid result, such as all timeouts or all nodes
	// being filtered out. Downstream serialization would fail, so short-circuit
	// here and log once as a warning.
	if len(results) == 0 {
		slog.Warn("No nodes to save in this run; skipping save")
		return
	}

	// 1. Write history first while proxy["name"] still has the original value.
	if config.GlobalConfig.KeepDays > 0 {
		historyYamlData, err := marshalProxies(results)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to serialize history snapshot: %v", err))
		} else {
			SaveHistory(historyYamlData)
		}
	}

	// 2. Mutate each proxy name in place to the final display name.
	for i := range results {
		if results[i].Proxy == nil {
			continue
		}
		results[i].Proxy["name"] = check.RenderName(results[i], true)
	}

	// 3. Serialize mutated results for all.yaml, remote storage, and SubStore.
	allYamlData, err := marshalProxies(results)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to serialize proxy data: %v", err))
		return
	}

	// Save all.yaml locally.
	if err := method.SaveToLocal(allYamlData, "all.yaml"); err != nil {
		slog.Error(fmt.Sprintf("Failed to save all.yaml locally: %v", err))
	}

	// Update SubStore and fetch derived files (mihomo.yaml / base64.txt).
	var mihomoData, base64Data []byte
	if config.GlobalConfig.SubStorePort != "" {
		utils.UpdateSubStore(allYamlData)
		mihomoData = fetchSubStoreData(
			fmt.Sprintf("%s/api/file/%s", utils.BaseURL, utils.MihomoName),
			"mihomo.yaml",
		)
		base64Data = fetchSubStoreData(
			fmt.Sprintf("%s/download/%s?target=V2Ray", utils.BaseURL, utils.SubName),
			"base64.txt",
		)
	}

	// Save derived files locally.
	saveIfNotEmpty(method.SaveToLocal, mihomoData, "mihomo.yaml")
	saveIfNotEmpty(method.SaveToLocal, base64Data, "base64.txt")

	// Save all files remotely if a remote save method is configured.
	if config.GlobalConfig.SaveMethod == "local" {
		return
	}
	remoteSaver, err := newRemoteSaver()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize remote save method (%s): %v", config.GlobalConfig.SaveMethod, err))
		return
	}
	saveIfNotEmpty(remoteSaver, allYamlData, "all.yaml")
	saveIfNotEmpty(remoteSaver, mihomoData, "mihomo.yaml")
	saveIfNotEmpty(remoteSaver, base64Data, "base64.txt")
}

// marshalProxies extracts proxies from check results and serializes them as YAML.
func marshalProxies(results []check.Result) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(results))
	for _, result := range results {
		proxies = append(proxies, result.Proxy)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("no usable proxy nodes")
	}
	return yaml.Marshal(map[string]any{"proxies": proxies})
}

// fetchSubStoreData fetches data from the SubStore API.
func fetchSubStoreData(url, name string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to request %s: %v", name, err))
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to read %s: %v", name, err))
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Failed to get %s, status code: %d, error: %s", name, resp.StatusCode, body))
		return nil
	}
	return body
}

// saveIfNotEmpty saves data when it is not empty.
func saveIfNotEmpty(saver SaveFunc, data []byte, filename string) {
	if len(data) == 0 {
		return
	}
	if err := saver(data, filename); err != nil {
		slog.Error(fmt.Sprintf("Failed to save %s to %s: %v", filename, config.GlobalConfig.SaveMethod, err))
	}
}

// newRemoteSaver creates a remote save method from config.
func newRemoteSaver() (SaveFunc, error) {
	switch config.GlobalConfig.SaveMethod {
	case "r2":
		if err := method.ValiR2Config(); err != nil {
			return nil, fmt.Errorf("R2 configuration is incomplete: %w", err)
		}
		return method.UploadToR2Storage, nil
	case "gist":
		if err := method.ValiGistConfig(); err != nil {
			return nil, fmt.Errorf("Gist configuration is incomplete: %w", err)
		}
		return method.UploadToGist, nil
	case "webdav":
		if err := method.ValiWebDAVConfig(); err != nil {
			return nil, fmt.Errorf("WebDAV configuration is incomplete: %w", err)
		}
		return method.UploadToWebDAV, nil
	case "s3":
		if err := method.ValiS3Config(); err != nil {
			return nil, fmt.Errorf("S3 configuration is incomplete: %w", err)
		}
		return method.UploadToS3, nil
	default:
		return nil, fmt.Errorf("unknown save method: %s", config.GlobalConfig.SaveMethod)
	}
}
