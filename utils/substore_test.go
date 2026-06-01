package utils

import (
	"testing"
	"time"
)

func TestFormatTimePlaceholders(t *testing.T) {
	// Baseline time: 2023-01-31 12:00:00, useful for month/year boundary tests.
	base := time.Date(2023, 1, 31, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   string
		want string
	}{
		// No placeholder.
		{"no placeholder", "https://example.com/sub", "https://example.com/sub"},

		// Basic single fields.
		{"Y", "x/{Y}/y", "x/2023/y"},
		{"m", "x/{m}/y", "x/01/y"},
		{"d", "x/{d}/y", "x/31/y"},

		// Combined dates.
		{"Ymd", "x/{Ymd}/y", "x/20230131/y"},
		{"Y_m_d", "x/{Y_m_d}/y", "x/2023_01_31/y"},
		{"Y-m-d", "x/{Y-m-d}/y", "x/2023-01-31/y"},

		// Offset combined dates.
		{"Ymd+1 month boundary", "x/{Ymd+1}/y", "x/20230201/y"},
		{"Ymd-1", "x/{Ymd-1}/y", "x/20230130/y"},
		{"Y_m_d+7", "x/{Y_m_d+7}/y", "x/2023_02_07/y"},
		{"Y-m-d-7", "x/{Y-m-d-7}/y", "x/2023-01-24/y"},
		{"Y-m-d-30", "x/{Y-m-d-30}/y", "x/2023-01-01/y"},

		// Offset single fields. The offset unit is days, and the field is taken after applying it.
		{"d+1 month boundary", "x/{d+1}/y", "x/01/y"},
		{"m+1 month boundary", "x/{m+1}/y", "x/02/y"},
		{"Y+0 equivalent to no offset", "x/{Y}/y", "x/2023/y"},

		// Multiple placeholders.
		{"multi", "{Y-m-d}_{Ymd+1}", "2023-01-31_20230201"},

		// Placeholder-like strings that are not placeholders should stay unchanged.
		{"bogus", "x/{foo}/y", "x/{foo}/y"},
		{"partial braces", "x/{Y/y", "x/{Y/y"},
		{"empty offset sign", "x/{Y+}/y", "x/{Y+}/y"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTimePlaceholders(tc.in, base)
			if got != tc.want {
				t.Errorf("formatTimePlaceholders(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatTimePlaceholders_YearBoundary(t *testing.T) {
	// Year-end: {Y+1 day} crosses into the next year.
	base := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	cases := map[string]string{
		"{Y+1}":     "2024",
		"{m+1}":     "01",
		"{d+1}":     "01",
		"{Ymd+1}":   "20240101",
		"{Y-m-d+1}": "2024-01-01",
		"{Y-1}":     "2023",
		"{Ymd-365}": "20221231",
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			got := formatTimePlaceholders(in, base)
			if got != want {
				t.Errorf("formatTimePlaceholders(%q) = %q, want %q", in, got, want)
			}
		})
	}
}
