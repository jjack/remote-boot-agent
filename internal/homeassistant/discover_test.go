package homeassistant

import (
	"context"
	"net"
	"testing"

	"github.com/grandcat/zeroconf"
)

func TestDiscover_Timeout(t *testing.T) {
	// Without a zeroconf server, this will timeout and return an empty string.
	url, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
	if url != "" {
		t.Logf("Found HA at %s", url)
	}
}

func TestExtractURL(t *testing.T) {
	tests := []struct {
		name     string
		entry    *zeroconf.ServiceEntry
		expected string
	}{
		{
			name: "internal_url present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=http://ha.local:8123", "base_url=http://base.local"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://ha.local:8123",
		},
		{
			name: "base_url present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"base_url=http://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://base.local:8123",
		},
		{
			name: "empty txt records fallback to ip",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=", "base_url="},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://192.168.1.100:8123",
		},
		{
			name: "no txt records fallback to ip",
			entry: &zeroconf.ServiceEntry{
				AddrIPv4: []net.IP{net.ParseIP("10.0.0.5")},
				Port:     8123,
			},
			expected: "http://10.0.0.5:8123",
		},
		{
			name:     "no useful info",
			entry:    &zeroconf.ServiceEntry{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := extractURL(tt.entry)
			if url != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, url)
			}
		})
	}
}
