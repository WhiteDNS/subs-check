package platform

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const dnsLeakAPIDomain = "bash.ws"

type dnsLeakBlock struct {
	IP   dnsLeakString `json:"ip"`
	ASN  dnsLeakString `json:"asn"`
	Type string        `json:"type"`
}

type DNSLeakResolver struct {
	IP  string
	ASN string
}

type DNSLeakResult struct {
	ExitIP    string
	ExitASN   string
	Resolvers []DNSLeakResolver
	NoLeak    bool
}

func CheckDNSLeak(httpClient *http.Client) (*DNSLeakResult, error) {
	testID, err := getDNSLeakTestID(httpClient)
	if err != nil {
		return nil, err
	}

	triggerDNSLeakProbe(httpClient, testID)

	var blocks []dnsLeakBlock
	if err := getDNSLeakJSON(httpClient, fmt.Sprintf("https://%s/dnsleak/test/%s?json", dnsLeakAPIDomain, testID), &blocks); err != nil {
		return nil, err
	}
	return classifyDNSLeak(blocks)
}

func getDNSLeakTestID(httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/id", dnsLeakAPIDomain), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dns leak id status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	testID := strings.TrimSpace(string(body))
	if testID == "" {
		return "", fmt.Errorf("dns leak id is empty")
	}
	return testID, nil
}

func triggerDNSLeakProbe(httpClient *http.Client, testID string) {
	var wg sync.WaitGroup
	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, err := http.NewRequest("GET", fmt.Sprintf("https://%d.%s.%s", i, testID, dnsLeakAPIDomain), nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
			resp, err := httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()
}

func getDNSLeakJSON(httpClient *http.Client, url string, v any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "subs-check (https://github.com/beck-8/subs-check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dns leak result status: %d", resp.StatusCode)
	}
	return readJSONPooled(resp.Body, v)
}

func classifyDNSLeak(blocks []dnsLeakBlock) (*DNSLeakResult, error) {
	result := &DNSLeakResult{}
	for _, block := range blocks {
		switch block.Type {
		case "ip":
			if result.ExitIP == "" {
				result.ExitIP = strings.TrimSpace(string(block.IP))
				result.ExitASN = strings.TrimSpace(string(block.ASN))
			}
		case "dns":
			result.Resolvers = append(result.Resolvers, DNSLeakResolver{
				IP:  strings.TrimSpace(string(block.IP)),
				ASN: strings.TrimSpace(string(block.ASN)),
			})
		}
	}

	if result.ExitIP == "" || result.ExitASN == "" {
		return result, fmt.Errorf("dns leak test inconclusive: missing exit ip/asn")
	}
	if len(result.Resolvers) == 0 {
		return result, fmt.Errorf("dns leak test inconclusive: missing dns resolvers")
	}
	for _, resolver := range result.Resolvers {
		if resolver.IP == "" || resolver.ASN == "" {
			return result, fmt.Errorf("dns leak test inconclusive: missing resolver ip/asn")
		}
		if resolver.ASN != result.ExitASN {
			return result, fmt.Errorf("dns leak detected: exit asn %s, resolver asn %s", result.ExitASN, resolver.ASN)
		}
	}

	result.NoLeak = true
	return result, nil
}

type dnsLeakString string

func (s *dnsLeakString) UnmarshalJSON(data []byte) error {
	if string(data) == "false" || string(data) == "null" {
		*s = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = dnsLeakString(value)
	return nil
}
