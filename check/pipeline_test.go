package check

import (
	"fmt"
	"testing"

	"github.com/beck-8/subs-check/config"
)

// makePipelineProxies returns n stub proxies; the numeric index is
// encoded in the "name" field so the collector's output order can
// be verified end-to-end.
func makePipelineProxies(n int) []map[string]any {
	proxies := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		proxies[i] = map[string]any{
			"name":   fmt.Sprintf("node-%d", i),
			"server": fmt.Sprintf("s%d.test", i),
			"port":   443,
			"type":   "trojan",
		}
	}
	return proxies
}

// idxFromProxyName parses the numeric suffix from a test proxy's "name".
func idxFromProxyName(t *testing.T, r Result) int {
	t.Helper()
	name, _ := r.Proxy["name"].(string)
	var idx int
	if _, err := fmt.Sscanf(name, "node-%d", &idx); err != nil {
		t.Fatalf("cannot parse idx from %q: %v", name, err)
	}
	return idx
}

func assertAllProxyNamesPresent(t *testing.T, results []Result, n int) {
	t.Helper()
	seen := make(map[int]bool, n)
	for _, r := range results {
		idx := idxFromProxyName(t, r)
		if idx < 0 || idx >= n {
			t.Fatalf("result idx out of range: %d", idx)
		}
		if seen[idx] {
			t.Fatalf("duplicate result idx: %d", idx)
		}
		seen[idx] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d unique results, got %d", n, len(seen))
	}
}

// TestPipeline_CollectsAllResults pushes 200 stub proxies through the full
// pipeline (dispatch → alive → media+filter → speed → collect) under
// SUB_CHECK_SKIP so every node passes every stage.
func TestPipeline_CollectsAllResults(t *testing.T) {
	t.Setenv("SUB_CHECK_SKIP", "1")
	const n = 200
	withConfig(t, config.Config{
		Concurrent:      20,
		MediaConcurrent: 10,
		SpeedConcurrent: 10,
		SpeedTestUrl:    "http://example.invalid/dl",
		MinSpeed:        0,
		Timeout:         1000,
		PrintProgress:   false,
	}, func() {
		pc := &ProxyChecker{results: make([]Result, 0)}
		results, err := pc.run(makePipelineProxies(n))
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if len(results) != n {
			t.Fatalf("expected %d results, got %d", n, len(results))
		}
		assertAllProxyNamesPresent(t, results, n)
	})
}

// TestPipeline_HonorsSuccessLimit verifies that cancel halts the
// dispatcher once SuccessLimit items have been gathered. Cancellation
// policy is asymmetric:
//   - middle stages use a ctx-aware select on send, so queued items and
//     select-race losers are dropped to avoid wasted downstream work
//   - the speed→collector send is unconditional, so items already
//     classified as passing the speed test never get thrown away
//
// Overshoot bound reflects that asymmetry: up to cap(collectIn) items
// may have been queued for the collector when cancel fired, and each
// speed worker in flight may send one more item (unconditional). That
// gives a ceiling around limit + 2*speed-concurrent, loose enough to
// stay robust against Go's random select scheduling.
func TestPipeline_HonorsSuccessLimit(t *testing.T) {
	t.Setenv("SUB_CHECK_SKIP", "1")
	const (
		input  = 2000
		limit  = 10
		speedC = 10
	)
	withConfig(t, config.Config{
		Concurrent:      50,
		MediaConcurrent: 20,
		SpeedConcurrent: speedC,
		SpeedTestUrl:    "http://example.invalid/dl",
		SuccessLimit:    limit,
		MinSpeed:        0,
		Timeout:         1000,
		PrintProgress:   false,
	}, func() {
		pc := &ProxyChecker{results: make([]Result, 0)}
		results, err := pc.run(makePipelineProxies(input))
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if len(results) < limit {
			t.Fatalf("expected at least %d results, got %d", limit, len(results))
		}
		if max := limit + 2*speedC; len(results) > max {
			t.Fatalf("expected at most %d results (overshoot window), got %d", max, len(results))
		}
		if alive := int(Progress.Load()); alive >= input {
			t.Fatalf("cancellation did not stop dispatch: aliveDone=%d (input=%d)", alive, input)
		}
	})
}

// TestPipeline_NoSpeedStage verifies that when SpeedTestUrl is empty the
// collector receives items directly from the media stage (speed workers
// never start).
func TestPipeline_NoSpeedStage(t *testing.T) {
	t.Setenv("SUB_CHECK_SKIP", "1")
	const n = 100
	withConfig(t, config.Config{
		Concurrent:      20,
		MediaConcurrent: 10,
		SpeedTestUrl:    "", // speed stage skipped
		Timeout:         1000,
		PrintProgress:   false,
	}, func() {
		pc := &ProxyChecker{results: make([]Result, 0)}
		results, err := pc.run(makePipelineProxies(n))
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if len(results) != n {
			t.Fatalf("expected %d results, got %d", n, len(results))
		}
		assertAllProxyNamesPresent(t, results, n)
	})
}

func TestCheck_HonorsMaxProbesAcrossBatches(t *testing.T) {
	t.Setenv("SUB_CHECK_SKIP", "1")
	oldHistory := config.GlobalProxies
	defer func() { config.GlobalProxies = oldHistory }()

	const (
		input = 12000
		limit = 7000
	)
	history := makePipelineProxies(input)
	for _, proxy := range history {
		proxy["sub_url"] = "history"
	}
	config.GlobalProxies = history

	withConfig(t, config.Config{
		Concurrent:       100,
		MediaConcurrent:  100,
		MaxProbesPerRun:  limit,
		Timeout:          1000,
		PrintProgress:    false,
		SubUrls:          nil,
		SubUrlsRemote:    nil,
		RenameNode:       false,
		NodeNameTemplate: "",
	}, func() {
		results, err := Check()
		if err != nil {
			t.Fatalf("Check returned error: %v", err)
		}
		if len(results) != limit {
			t.Fatalf("expected %d results, got %d", limit, len(results))
		}
		if len(config.GlobalProxies) != 0 {
			t.Fatalf("expected GlobalProxies to be cleared, got %d", len(config.GlobalProxies))
		}
		for _, result := range results {
			if _, ok := result.Proxy["sub_url"]; ok {
				t.Fatal("sub_url leaked into saved result")
			}
		}
	})
}
