package platform

import "testing"

func TestClassifyDNSLeak_PassesMatchingASN(t *testing.T) {
	got, err := classifyDNSLeak([]dnsLeakBlock{
		{Type: "ip", IP: "203.0.113.1", ASN: "AS64500"},
		{Type: "dns", IP: "203.0.113.53", ASN: "AS64500"},
		{Type: "dns", IP: "203.0.113.54", ASN: "AS64500"},
	})
	if err != nil {
		t.Fatalf("classifyDNSLeak() error = %v", err)
	}
	if !got.NoLeak {
		t.Fatalf("classifyDNSLeak() NoLeak = false, want true")
	}
	if got.ExitIP != "203.0.113.1" || got.ExitASN != "AS64500" || len(got.Resolvers) != 2 {
		t.Fatalf("classifyDNSLeak() = %#v", got)
	}
}

func TestClassifyDNSLeak_FailsDifferentASN(t *testing.T) {
	got, err := classifyDNSLeak([]dnsLeakBlock{
		{Type: "ip", IP: "203.0.113.1", ASN: "AS64500"},
		{Type: "dns", IP: "198.51.100.53", ASN: "AS64501"},
	})
	if err == nil {
		t.Fatalf("classifyDNSLeak() error = nil, want leak error")
	}
	if got.NoLeak {
		t.Fatalf("classifyDNSLeak() NoLeak = true, want false")
	}
}

func TestClassifyDNSLeak_FailsMissingDNS(t *testing.T) {
	_, err := classifyDNSLeak([]dnsLeakBlock{
		{Type: "ip", IP: "203.0.113.1", ASN: "AS64500"},
	})
	if err == nil {
		t.Fatalf("classifyDNSLeak() error = nil, want inconclusive error")
	}
}

func TestClassifyDNSLeak_FailsMissingASN(t *testing.T) {
	_, err := classifyDNSLeak([]dnsLeakBlock{
		{Type: "ip", IP: "203.0.113.1", ASN: "AS64500"},
		{Type: "dns", IP: "203.0.113.53"},
	})
	if err == nil {
		t.Fatalf("classifyDNSLeak() error = nil, want inconclusive error")
	}
}
