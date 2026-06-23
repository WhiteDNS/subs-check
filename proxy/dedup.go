package proxies

import (
	"fmt"
)

func DeduplicateProxies(proxies []map[string]any) []map[string]any {
	seenKeys := make(map[string]bool)
	return DeduplicateProxiesBySeen(proxies, seenKeys)
}

func DeduplicateProxiesBySeen(proxies []map[string]any, seenKeys map[string]bool) []map[string]any {
	result := make([]map[string]any, 0, len(proxies))

	for _, proxy := range proxies {
		server, _ := proxy["server"].(string)
		if server == "" {
			continue
		}
		servername, _ := proxy["servername"].(string)

		password, _ := proxy["password"].(string)
		if password == "" {
			password, _ = proxy["uuid"].(string)
		}

		sni, _ := proxy["sni"].(string)

		network, _ := proxy["network"].(string)

		key := fmt.Sprintf("%s:%v:%s:%s:%s:%s", server, proxy["port"], servername, password, sni, network)

		if !seenKeys[key] {
			seenKeys[key] = true
			result = append(result, proxy)
		}
	}

	return result
}
