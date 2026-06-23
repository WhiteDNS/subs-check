package proxies

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beck-8/subs-check/config"
)

func TestForEachProxyBatchStreamsBatchesAndHonorsConcurrency(t *testing.T) {
	oldConfig := *config.GlobalConfig
	oldFetch := fetchSubData
	defer func() {
		*config.GlobalConfig = oldConfig
		fetchSubData = oldFetch
	}()

	*config.GlobalConfig = config.Config{
		OutputDir:         t.TempDir(),
		SubUrls:           []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"},
		SubUrlsConcurrent: 2,
	}

	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	fetchSubData = func(rawURL string) ([]byte, error) {
		current := inFlight.Add(1)
		for {
			max := maxInFlight.Load()
			if current <= max || maxInFlight.CompareAndSwap(max, current) {
				break
			}
		}
		defer inFlight.Add(-1)

		time.Sleep(50 * time.Millisecond)
		name := rawURL[strings.LastIndex(rawURL, "/")+1:]
		return []byte(fmt.Sprintf(`proxies:
  - name: %[1]s-1
    type: ss
    server: %[1]s1.example.com
    port: 443
    cipher: aes-128-gcm
    password: test
  - name: %[1]s-2
    type: ss
    server: %[1]s2.example.com
    port: 443
    cipher: aes-128-gcm
    password: test
  - name: %[1]s-3
    type: ss
    server: %[1]s3.example.com
    port: 443
    cipher: aes-128-gcm
    password: test
`, name)), nil
	}

	total := 0
	batches := 0
	err := ForEachProxyBatch(4, func(batch []map[string]any) error {
		if len(batch) > 4 {
			t.Fatalf("batch exceeded limit: %d", len(batch))
		}
		total += len(batch)
		batches++
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachProxyBatch returned error: %v", err)
	}
	if total != 9 {
		t.Fatalf("expected 9 proxies, got %d", total)
	}
	if batches < 3 {
		t.Fatalf("expected at least 3 batches, got %d", batches)
	}
	if maxInFlight.Load() < 2 {
		t.Fatalf("expected concurrent fetches, max in flight was %d", maxInFlight.Load())
	}
}

func TestFailedSubFetchIsIgnoredForThreeDays(t *testing.T) {
	oldConfig := *config.GlobalConfig
	oldFetch := fetchSubData
	defer func() {
		*config.GlobalConfig = oldConfig
		fetchSubData = oldFetch
	}()

	url := "https://example.com/sub.yaml"
	*config.GlobalConfig = config.Config{
		OutputDir:         t.TempDir(),
		SubUrls:           []string{url},
		SubUrlsConcurrent: 1,
	}

	var calls atomic.Int32
	fetchSubData = func(string) ([]byte, error) {
		calls.Add(1)
		return nil, errors.New("fetch failed")
	}

	proxies, err := GetProxies()
	if err != nil {
		t.Fatalf("GetProxies returned error: %v", err)
	}
	if len(proxies) != 0 {
		t.Fatalf("expected no proxies, got %d", len(proxies))
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one fetch attempt, got %d", calls.Load())
	}

	now := time.Now()
	ignores := loadFailedSubIgnores(now)
	if !ignores.ignored(url, now) {
		t.Fatal("expected failed URL to be ignored")
	}

	_, err = GetProxies()
	if err != nil {
		t.Fatalf("GetProxies returned error on ignored URL: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected ignored URL to skip fetch, got %d fetches", calls.Load())
	}

	later := now.Add(failedSubIgnoreDuration + time.Hour)
	expired := loadFailedSubIgnores(later)
	if expired.ignored(url, later) {
		t.Fatal("expected failed URL ignore to expire")
	}
}
