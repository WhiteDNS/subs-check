package check

import (
	"testing"

	"github.com/beck-8/subs-check/check/platform"
	"github.com/beck-8/subs-check/config"
	proxyutils "github.com/beck-8/subs-check/proxy"
)

// withConfig temporarily replaces config.GlobalConfig and restores it after the test.
// GlobalConfig is a *Config pointer, so assigning through the pointer keeps code
// holding the same pointer working as expected.
func withConfig(t *testing.T, cfg config.Config, fn func()) {
	t.Helper()
	old := *config.GlobalConfig
	*config.GlobalConfig = cfg
	defer func() { *config.GlobalConfig = old }()
	fn()
}

func TestRenderName_RenameOff_NoTags(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{"openai", "netflix"},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "🇭🇰Hong Kong 01"},
		}
		got := RenderName(r, false)
		want := "🇭🇰Hong Kong 01"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_RenameOff_PreservesOriginalWithPipes(t *testing.T) {
	// Preserve original provider names that already contain |.
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "🇺🇸United States 01-0.1x | carrier recommended"},
		}
		got := RenderName(r, false)
		want := "🇺🇸United States 01-0.1x | carrier recommended"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_RenameOff_WithMediaTags(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{"openai", "netflix", "disney"},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "🇭🇰Hong Kong 01"},
			Openai:  &platform.OpenAIResult{Full: true, Region: "HK"},
			Netflix: &platform.NetflixResult{Full: true, Region: "HK"},
			Disney:  true,
		}
		got := RenderName(r, false)
		want := "🇭🇰Hong Kong 01|GPT⁺-HK|NF-HK|D+"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_PlatformsOrderMatters(t *testing.T) {
	// Tag order strictly follows config.Platforms.
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{"netflix", "openai"}, // Opposite order from the previous test.
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "n"},
			Openai:  &platform.OpenAIResult{Full: true, Region: "HK"},
			Netflix: &platform.NetflixResult{Full: true, Region: "HK"},
		}
		got := RenderName(r, false)
		want := "n|NF-HK|GPT⁺-HK"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_IncludeSpeedTrue(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode:   false,
		SpeedTestUrl: "https://example.com/file",
		Platforms:    []string{},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "n"},
			Speed: 5120, // 5.0 MB/s
		}
		got := RenderName(r, true)
		want := "n|5.0MB/s"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_IncludeSpeedFalse_NoSpeedTag(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode:   false,
		SpeedTestUrl: "https://example.com/file",
		Platforms:    []string{},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "n"},
			Speed: 5120,
		}
		got := RenderName(r, false) // The filter stage passes false.
		want := "n"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_SpeedTagFormat_KB(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode:   false,
		SpeedTestUrl: "https://example.com/file",
		Platforms:    []string{},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "n"},
			Speed: 512, // < 1024, display KB/s.
		}
		got := RenderName(r, true)
		want := "n|512KB/s"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_SpeedZero_NoSpeedTag(t *testing.T) {
	// Speed=0 means untested (ForceClose scenario), so no tag is added even when includeSpeed=true.
	withConfig(t, config.Config{
		RenameNode:   false,
		SpeedTestUrl: "https://example.com/file",
		Platforms:    []string{},
	}, func() {
		r := Result{
			Proxy: map[string]any{"name": "n"},
			Speed: 0,
		}
		got := RenderName(r, true)
		want := "n"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_SpeedBeforeMediaTags(t *testing.T) {
	// Lock tag order: base | speed | media-tags | sub_tag.
	withConfig(t, config.Config{
		RenameNode:   false,
		SpeedTestUrl: "https://example.com/file",
		Platforms:    []string{"openai", "netflix"},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "n", "sub_tag": "tag"},
			Speed:   5120, // 5.0MB/s
			Openai:  &platform.OpenAIResult{Full: true, Region: "HK"},
			Netflix: &platform.NetflixResult{Full: true, Region: "HK"},
		}
		got := RenderName(r, true)
		want := "n|5.0MB/s|GPT⁺-HK|NF-HK|tag"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_SubTagAppendedLast(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{"disney"},
	}, func() {
		r := Result{
			Proxy:  map[string]any{"name": "n", "sub_tag": "my-sub"},
			Disney: true,
		}
		got := RenderName(r, false)
		want := "n|D+|my-sub"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_IPRiskTag(t *testing.T) {
	withConfig(t, config.Config{
		RenameNode: false,
		Platforms:  []string{"iprisk"},
	}, func() {
		r := Result{
			Proxy:  map[string]any{"name": "n"},
			IPRisk: "5%",
		}
		got := RenderName(r, false)
		want := "n|5%"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_RenameOnWithCountry(t *testing.T) {
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		RenameNode: true,
		NodePrefix: "PREFIX-",
		Platforms:  []string{},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "original"},
			Country: "HK",
		}
		got := RenderName(r, false)
		if got == "original" {
			t.Errorf("RenderName() should not use original name when RenameNode=true, got %q", got)
		}
		if len(got) < len("PREFIX-") || got[:len("PREFIX-")] != "PREFIX-" {
			t.Errorf("RenderName() should start with prefix, got %q", got)
		}
		if !stringContains(got, "HK") {
			t.Errorf("RenderName() should contain country code HK, got %q", got)
		}
	})
}

func TestRenderName_RenameOnButEmptyCountry_UsesOtherFallback(t *testing.T) {
	// When renaming is enabled but Country is empty because Phase 2 lookup
	// failed, use the ❓Other fallback instead of preserving the original name.
	// Otherwise upstream names polluted with |speed|media suffixes leak through
	// and get appended again.
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		RenameNode: true,
		NodePrefix: "PREFIX-",
		Platforms:  []string{},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "🇹🇼original-name|745KB/s|YT-TW"},
			Country: "",
		}
		got := RenderName(r, false)
		if got == "🇹🇼original-name|745KB/s|YT-TW" {
			t.Errorf("RenderName() should not preserve polluted original name when RenameNode=true, got %q", got)
		}
		if len(got) < len("PREFIX-") || got[:len("PREFIX-")] != "PREFIX-" {
			t.Errorf("RenderName() should start with prefix, got %q", got)
		}
		if !stringContains(got, "Other") {
			t.Errorf("RenderName() should fall back to Other when Country is empty, got %q", got)
		}
	})
}

func TestRenderName_NodeNameTemplateWithSpeed(t *testing.T) {
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		NodeNameTemplate: "{CountryFlag} | @WhiteDNS | {Speed}",
		SpeedTestUrl:     "https://example.com/file",
		Platforms:        []string{},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "original"},
			Country: "US",
			Speed:   2048, // 2.0 MB/s
		}
		got := RenderName(r, true)
		want := "🇺🇸 | @WhiteDNS | 2.0MB/s"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_NodeNameTemplateKeepsTagsPlaceholderExplicit(t *testing.T) {
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		NodeNameTemplate: "{CountryFlag} | {Name} | {Tags}",
		SpeedTestUrl:     "https://example.com/file",
		Platforms:        []string{"netflix"},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "original", "sub_tag": "source"},
			Country: "JP",
			Speed:   1024,
			Netflix: &platform.NetflixResult{Full: true, Region: "JP"},
		}
		got := RenderName(r, true)
		want := "🇯🇵 | original | 1.0MB/s|NF-JP|source"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_NodeNameTemplateFlagAndShortID(t *testing.T) {
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		NodeNameTemplate: "{Flag}|@WhiteDNS|{ShortID}",
		Platforms:        []string{},
	}, func() {
		first := Result{
			Proxy:   map[string]any{"name": "first"},
			Country: "DE",
		}
		second := Result{
			Proxy:   map[string]any{"name": "second"},
			Country: "DE",
		}
		if got, want := RenderName(first, true), "🇩🇪|@WhiteDNS|DE1"; got != want {
			t.Errorf("first RenderName() = %q, want %q", got, want)
		}
		if got, want := RenderName(second, true), "🇩🇪|@WhiteDNS|DE2"; got != want {
			t.Errorf("second RenderName() = %q, want %q", got, want)
		}
	})
}

func TestRenderName_NodeNameTemplateActualNameAlias(t *testing.T) {
	proxyutils.ResetRenameCounter()
	withConfig(t, config.Config{
		NodeNameTemplate: "{CountryFlag}|{ActualName}|@WhiteDNS|{ShortID}",
		Platforms:        []string{},
	}, func() {
		r := Result{
			Proxy:   map[string]any{"name": "provider-node-1"},
			Country: "US",
		}
		got := RenderName(r, true)
		want := "🇺🇸|provider-node-1|@WhiteDNS|US1"
		if got != want {
			t.Errorf("RenderName() = %q, want %q", got, want)
		}
	})
}

// Helper.
func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
