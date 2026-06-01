package check

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/beck-8/subs-check/config"
)

// CompileFilterPatterns compiles the configured filter regex list.
// Invalid patterns are dropped with a warning; returns an empty slice
// when filtering is disabled or all patterns failed to compile.
func CompileFilterPatterns() []*regexp.Regexp {
	if len(config.GlobalConfig.Filter) == 0 {
		return nil
	}
	var patterns []*regexp.Regexp
	for _, pattern := range config.GlobalConfig.Filter {
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn(fmt.Sprintf("Filter regex failed to compile and was skipped: %s, error: %v", pattern, err))
			continue
		}
		patterns = append(patterns, re)
	}
	if len(patterns) == 0 && len(config.GlobalConfig.Filter) > 0 {
		slog.Warn("All filter regexes failed to compile; skipping filtering")
	}
	return patterns
}

// MatchesFilter reports whether r's rendered name (without speed tag)
// matches any pattern. An empty pattern slice counts as "passes".
func MatchesFilter(r Result, patterns []*regexp.Regexp) bool {
	if len(patterns) == 0 {
		return true
	}
	if r.Proxy == nil {
		return false
	}
	name := RenderName(r, false)
	for _, re := range patterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// FilterResults filters nodes using the configured regular expressions.
//
// Only nodes whose rendered display name (without speed tags) matches at least
// one regex are kept. Use RenderName(r, false) instead of r.Proxy["name"] so
// filtering sees the full country + media-tag view while leaving proxy["name"]
// unchanged.
func FilterResults(results []Result) []Result {
	patterns := CompileFilterPatterns()
	if len(patterns) == 0 {
		return results
	}

	slog.Info(fmt.Sprintf("Applying node filters: %d regexes", len(patterns)))

	var filtered []Result
	for _, r := range results {
		if MatchesFilter(r, patterns) {
			filtered = append(filtered, r)
		}
	}

	slog.Info(fmt.Sprintf("Nodes after filtering: %d (before: %d)", len(filtered), len(results)))
	return filtered
}
