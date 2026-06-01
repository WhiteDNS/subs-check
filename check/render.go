package check

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/beck-8/subs-check/config"
	proxyutils "github.com/beck-8/subs-check/proxy"
)

// RenderName builds the display name from Result's structured fields.
//
// This is the only node-name generation entry point in the project.
// It does not read/write the proxy map's name field and does not modify Result.
//
// includeSpeed appends a speed tag when true and is only used for the final all.yaml output.
// The filter stage should pass false because speed testing has not run yet.
func RenderName(r Result, includeSpeed bool) string {
	// 1. Base name. A node-name template is a hard override. Otherwise,
	// RenameNode replaces the original name with the built-in country/index
	// format. If Country is empty, Rename falls back to ❓Other_N. This prevents
	// upstream |speed|media suffixes from leaking into the final name and being
	// appended again, which is common when IP lookup fails on free nodes.
	template := strings.TrimSpace(config.GlobalConfig.NodeNameTemplate)
	templateHasSpeed := strings.Contains(template, "{Speed}")
	templateHasTags := strings.Contains(template, "{Tags}")

	var base string
	if template != "" {
		base = renderNameTemplate(template, r, includeSpeed)
	} else if config.GlobalConfig.RenameNode {
		base = config.GlobalConfig.NodePrefix + proxyutils.Rename(r.Country)
	} else if r.Proxy != nil {
		if n, ok := r.Proxy["name"].(string); ok {
			base = strings.TrimSpace(n)
		}
	}

	// 2. Speed tag. Append only when includeSpeed is true and speed is present,
	// before media tags to preserve the old display order.
	var tags []string
	if !templateHasSpeed && !templateHasTags && includeSpeed && config.GlobalConfig.SpeedTestUrl != "" && r.Speed > 0 {
		tags = append(tags, formatSpeedTag(r.Speed))
	}

	// 3. Collect media tags in config.Platforms order.
	if !templateHasTags {
		for _, plat := range config.GlobalConfig.Platforms {
			if tag := mediaTagFor(plat, &r); tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// 4. Append sub_tag last.
	if !templateHasTags && r.Proxy != nil {
		if t, ok := r.Proxy["sub_tag"].(string); ok && t != "" {
			tags = append(tags, t)
		}
	}

	if len(tags) == 0 {
		return base
	}
	return base + "|" + strings.Join(tags, "|")
}

func renderNameTemplate(template string, r Result, includeSpeed bool) string {
	rename := proxyutils.NextRenameParts(r.Country)
	replacer := strings.NewReplacer(
		"{Flag}", rename.CountryFlag,
		"{CountryFlag}", rename.CountryFlag,
		"{Country}", rename.Country,
		"{Index}", strconv.Itoa(rename.Index),
		"{ShortID}", rename.ShortID,
		"{ActualName}", originalName(r),
		"{Name}", originalName(r),
		"{Speed}", speedTag(r, includeSpeed),
		"{Tags}", strings.Join(allTags(r, includeSpeed), "|"),
	)
	return strings.TrimSpace(replacer.Replace(template))
}

func originalName(r Result) string {
	if r.Proxy == nil {
		return ""
	}
	name, _ := r.Proxy["name"].(string)
	return strings.TrimSpace(name)
}

func speedTag(r Result, includeSpeed bool) string {
	if includeSpeed && config.GlobalConfig.SpeedTestUrl != "" && r.Speed > 0 {
		return formatSpeedTag(r.Speed)
	}
	return ""
}

func allTags(r Result, includeSpeed bool) []string {
	var tags []string
	if tag := speedTag(r, includeSpeed); tag != "" {
		tags = append(tags, tag)
	}
	for _, plat := range config.GlobalConfig.Platforms {
		if tag := mediaTagFor(plat, &r); tag != "" {
			tags = append(tags, tag)
		}
	}
	if r.Proxy != nil {
		if t, ok := r.Proxy["sub_tag"].(string); ok && t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// mediaTagFor returns the display tag for one platform, or an empty string when it does not match.
// Add a case here and a corresponding Result field when adding a platform.
func mediaTagFor(plat string, r *Result) string {
	switch plat {
	case "openai":
		if r.Openai != nil {
			if r.Openai.Full {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT⁺-%s", r.Openai.Region)
				}
				return "GPT⁺"
			}
			if r.Openai.Web {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT-%s", r.Openai.Region)
				}
				return "GPT"
			}
		}
	case "netflix":
		if r.Netflix != nil {
			if r.Netflix.Full {
				if r.Netflix.Region != "" {
					return fmt.Sprintf("NF-%s", r.Netflix.Region)
				}
				return "NF"
			}
			if r.Netflix.OriginalsOnly {
				return "NF"
			}
		}
	case "disney":
		if r.Disney {
			return "D+"
		}
	case "gemini":
		if r.Gemini != "" {
			return fmt.Sprintf("GM-%s", r.Gemini)
		}
	case "claude":
		if r.Claude != "" {
			return fmt.Sprintf("CL-%s", r.Claude)
		}
	case "spotify":
		if r.Spotify != "" {
			return fmt.Sprintf("SP-%s", r.Spotify)
		}
	case "iprisk":
		if r.IPRisk != "" {
			return r.IPRisk
		}
	case "youtube":
		if r.Youtube != "" {
			return fmt.Sprintf("YT-%s", r.Youtube)
		}
	case "tiktok":
		if r.TikTok != "" {
			return fmt.Sprintf("TK-%s", r.TikTok)
		}
	}
	return ""
}

// formatSpeedTag formats the speed-test result (KB/s) for display.
//
//	<1024 → "NKB/s"
//	>=1024 → "X.XMB/s"
func formatSpeedTag(speed int) string {
	if speed < 1024 {
		return fmt.Sprintf("%dKB/s", speed)
	}
	return fmt.Sprintf("%.1fMB/s", float64(speed)/1024)
}
