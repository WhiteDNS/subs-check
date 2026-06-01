//go:build substore_it

// Starts a real temporary sub-store on an isolated port and data directory, then
// runs end-to-end CRUD tests for substore.go.
// Default go test runs skip this; pass the tag explicitly:
//
//	go test -tags substore_it ./utils/ -run TestSubStore -v
//
// Depends on the embedded node binary for the current platform, the same asset
// used in production.
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/klauspost/compress/zstd"
)

// Do not import the assets package because it creates an assets -> save/method
// -> utils import cycle. Read the .zst assets under ../assets directly instead.
func assetsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "assets")
}

func nodeAssetName() string {
	arch := runtime.GOARCH
	switch arch {
	case "386":
		arch = "i386"
	case "arm":
		arch = "armv7"
	}
	return fmt.Sprintf("node_%s_%s.zst", runtime.GOOS, arch)
}

const overwriteMarker = overwriteOpMarker

// startTempSubStore extracts and starts a sub-store in a temp directory and
// returns its BaseURL. Data is written to an isolated SUB_STORE_DATA_BASE_PATH,
// never to the user's real data.
func startTempSubStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName += ".exe"
	}
	ad := assetsDir()
	nodeSrc := filepath.Join(ad, nodeAssetName())
	if _, err := os.Stat(nodeSrc); err != nil {
		t.Skipf("current platform has no embedded node asset (%s); skipping integration test", nodeSrc)
	}
	nodePath := filepath.Join(dir, nodeName)
	jsPath := filepath.Join(dir, "sub-store.bundle.js")
	decodeAsset(t, nodeSrc, nodePath, 0o755)
	decodeAsset(t, filepath.Join(ad, "sub-store.bundle.js.zst"), jsPath, 0o644)

	port := freePort(t)
	cmd := exec.Command(nodePath, jsPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"SUB_STORE_DATA_BASE_PATH="+dir,
		"SUB_STORE_BACKEND_API_PORT="+port,
		"SUB_STORE_BACKEND_API_HOST=127.0.0.1",
		"SUB_STORE_BODY_JSON_LIMIT=30mb",
	)
	logFile, _ := os.Create(filepath.Join(dir, "sub-store.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sub-store: %v", err)
	}
	t.Logf("temporary sub-store started: pid=%d port=%s working-dir=%s", cmd.Process.Pid, port, dir)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if logFile != nil {
			logFile.Close()
		}
	})

	base := "http://127.0.0.1:" + port
	waitReady(t, base)
	return base
}

func decodeAsset(t *testing.T, srcPath, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read asset %s: %v", srcPath, err)
	}
	dec, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("zstd reader: %v", err)
	}
	defer dec.Close()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		t.Fatalf("open %s: %v", dst, err)
	}
	if _, err := io.Copy(f, dec); err != nil {
		f.Close()
		t.Fatalf("decode %s: %v", dst, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", dst, err)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func waitReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/api/subs")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("sub-store did not become ready before timeout")
}

// patchRaw sends a raw PATCH to simulate a user editing config in the UI.
func patchRaw(t *testing.T, url string, body any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch %s status code %d", url, resp.StatusCode)
	}
}

// getProcess fetches process and content for a sub or file.
func getProcess(t *testing.T, url string) ([]map[string]any, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			Process []map[string]any `json:"process"`
			Content string           `json:"content"`
		} `json:"data"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("failed to parse %s: %v\n%s", url, err, string(body))
	}
	if r.Status != "success" {
		t.Fatalf("get %s status=%s", url, r.Status)
	}
	return r.Data.Process, r.Data.Content
}

func hasOperatorType(process []map[string]any, typ string) bool {
	for _, op := range process {
		if op["type"] == typ {
			return true
		}
	}
	return false
}

func TestSubStoreCRUD(t *testing.T) {
	base := startTempSubStore(t)

	// Functions in substore.go read globals, so set them up for the test.
	BaseURL = base
	config.GlobalConfig.GithubProxy = ""
	config.GlobalConfig.MihomoOverwriteUrl = "http://127.0.0.1:9/strategy-v1.yaml"
	mihomoOverwriteUrl = ""

	// ---- sub: create ----
	if err := checkSub(); err == nil {
		t.Fatal("sub has not been created; checkSub should fail")
	}
	if err := createSub([]byte("proxies:\n  - {name: a}\n")); err != nil {
		t.Fatalf("createSub: %v", err)
	}
	if err := checkSub(); err != nil {
		t.Fatalf("checkSub should succeed after create: %v", err)
	}

	// Simulate a user adding a Sort Operator to sub.
	subURL := base + "/api/sub/" + SubName
	patchRaw(t, subURL, map[string]any{
		"process": []map[string]any{
			{"type": "Quick Setting Operator"},
			{"type": "Sort Operator", "args": "asc", "customName": "my sort"},
		},
	})

	// ---- sub: update content only; the user's Sort Operator must be preserved ----
	if err := updateSub([]byte("proxies:\n  - {name: b}\n  - {name: c}\n")); err != nil {
		t.Fatalf("updateSub: %v", err)
	}
	proc, content := getProcess(t, subURL)
	if !hasOperatorType(proc, "Sort Operator") {
		t.Errorf("updateSub should not overwrite the user's Sort Operator, process=%v", proc)
	}
	if !bytes.Contains([]byte(content), []byte("name: b")) {
		t.Errorf("content was not refreshed after updateSub: %q", content)
	}

	// ---- file: create; our operator should be marked ----
	if err := checkfile(); err == nil {
		t.Fatal("file has not been created; checkfile should fail")
	}
	if err := createfile(); err != nil {
		t.Fatalf("createfile: %v", err)
	}
	if err := checkfile(); err != nil {
		t.Fatalf("checkfile should succeed after create: %v", err)
	}
	fileURL := base + "/api/file/" + MihomoName
	proc, _ = getProcess(t, base+"/api/wholeFile/"+MihomoName)
	if !markerPresent(proc) {
		t.Fatalf("createfile should add the customName marker, process=%v", proc)
	}

	// Simulate a user adding another operator to file.
	ourArgs := map[string]any{"content": WarpUrl(config.GlobalConfig.MihomoOverwriteUrl), "mode": "link"}
	patchRaw(t, fileURL, map[string]any{
		"process": []map[string]any{
			{"type": "Sort Operator", "args": "desc", "customName": "user sort"},
			{"type": "Script Operator", "args": ourArgs, "customName": overwriteMarker},
		},
	})

	// ---- file: update override URL; only our marked operator changes ----
	config.GlobalConfig.MihomoOverwriteUrl = "http://127.0.0.1:9/strategy-v2.yaml"
	if err := updatefile(); err != nil {
		t.Fatalf("updatefile: %v", err)
	}
	proc, _ = getProcess(t, base+"/api/wholeFile/"+MihomoName)
	if !hasOperatorType(proc, "Sort Operator") {
		t.Errorf("updatefile should not delete the user's Sort Operator, process=%v", proc)
	}
	want := WarpUrl("http://127.0.0.1:9/strategy-v2.yaml")
	if got := markerContent(proc); got != want {
		t.Errorf("our operator content should be updated to %q, got %q", want, got)
	}
	if c := userScriptContent(proc); c != "" {
		t.Errorf("user operator was changed unexpectedly; userScript content=%q", c)
	}

	// ---- file: idempotency; updatefile should change nothing when URL is unchanged ----
	before, _ := getProcess(t, base+"/api/wholeFile/"+MihomoName)
	if err := updatefile(); err != nil {
		t.Fatalf("second updatefile: %v", err)
	}
	after, _ := getProcess(t, base+"/api/wholeFile/"+MihomoName)
	if len(after) != len(before) {
		t.Errorf("idempotent update should not change operator count: before=%d after=%d", len(before), len(after))
	}
	if markerContent(after) != want {
		t.Errorf("idempotent update should not change content: %q", markerContent(after))
	}
	if n := countMarker(after); n != 1 {
		t.Errorf("idempotent update should not insert duplicate marked operators, count=%d", n)
	}
}

func countMarker(process []map[string]any) int {
	n := 0
	for _, op := range process {
		if op["customName"] == overwriteMarker {
			n++
		}
	}
	return n
}

// TestSubStoreLegacyFileMigration verifies that old unmarked data is detected
// and marked on first update.
func TestSubStoreLegacyFileMigration(t *testing.T) {
	base := startTempSubStore(t)
	BaseURL = base
	config.GlobalConfig.GithubProxy = ""
	config.GlobalConfig.MihomoOverwriteUrl = "http://127.0.0.1:9/old.yaml"

	// Create sub, which file references as its source.
	if err := createSub([]byte("proxies: []\n")); err != nil {
		t.Fatalf("createSub: %v", err)
	}

	// Directly POST an old-format file:link Script Operator without a customName marker.
	legacy := map[string]any{
		"name": MihomoName,
		"process": []map[string]any{
			{"type": "Script Operator", "args": map[string]any{"content": "http://127.0.0.1:9/old.yaml", "mode": "link"}},
		},
		"source":     "local",
		"sourceName": "sub",
		"sourceType": "subscription",
		"type":       "mihomoProfile",
	}
	b, _ := json.Marshal(legacy)
	resp, err := http.Post(base+"/api/files", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post legacy file: %v", err)
	}
	resp.Body.Close()

	// Update: match the old operator by type+mode, change content, and add marker.
	config.GlobalConfig.MihomoOverwriteUrl = "http://127.0.0.1:9/new.yaml"
	if err := updatefile(); err != nil {
		t.Fatalf("updatefile: %v", err)
	}
	proc, _ := getProcess(t, base+"/api/wholeFile/"+MihomoName)
	if !markerPresent(proc) {
		t.Errorf("legacy data should be marked after update, process=%v", proc)
	}
	if got, want := markerContent(proc), WarpUrl("http://127.0.0.1:9/new.yaml"); got != want {
		t.Errorf("content should be updated to %q, got %q", want, got)
	}
}

func markerPresent(process []map[string]any) bool {
	for _, op := range process {
		if op["customName"] == overwriteMarker {
			return true
		}
	}
	return false
}

func markerContent(process []map[string]any) string {
	for _, op := range process {
		if op["customName"] != overwriteMarker {
			continue
		}
		if a, ok := op["args"].(map[string]any); ok {
			if c, ok := a["content"].(string); ok {
				return c
			}
		}
	}
	return ""
}

// userScriptContent returns the content of an unmarked Script Operator, which tests expect to stay unchanged.
func userScriptContent(process []map[string]any) string {
	for _, op := range process {
		if op["type"] != "Script Operator" || op["customName"] == overwriteMarker {
			continue
		}
		if a, ok := op["args"].(map[string]any); ok {
			if c, ok := a["content"].(string); ok {
				return c
			}
		}
	}
	return ""
}
