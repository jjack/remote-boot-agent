package homeassistant

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/brutella/dnssd"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		entry    dnssd.BrowseEntry
		expected []string
	}{
		{
			name: "internal_url and ip present",
			entry: dnssd.BrowseEntry{
				Name: "Home",
				IPs:  []net.IP{net.ParseIP("192.168.1.100")},
				Port: 8123,
				Text: map[string]string{
					"internal_url": "http://ha.local:8123",
					"base_url":     "http://base.local",
				},
			},
			expected: []string{"http://ha.local:8123", "http://base.local", "http://192.168.1.100:8123"},
		},
		{
			name: "only ip present",
			entry: dnssd.BrowseEntry{
				Name: "Home",
				IPs:  []net.IP{net.ParseIP("192.168.1.100")},
				Port: 8123,
			},
			expected: []string{"http://192.168.1.100:8123"},
		},
		{
			name: "no useful info",
			entry: dnssd.BrowseEntry{
				Name: "Home",
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := extractURLs(tt.entry)
			if len(urls) != len(tt.expected) {
				t.Fatalf("expected %d urls, got %d: %v", len(tt.expected), len(urls), urls)
			}
			for i := range urls {
				if urls[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %s, got %s", i, tt.expected[i], urls[i])
				}
			}
		})
	}
}

func TestDiscover_Timeout(t *testing.T) {
	// Set a very short timeout for the test
	oldTimeout := discoveryTimeout
	discoveryTimeout = 10 * time.Millisecond
	defer func() { discoveryTimeout = oldTimeout }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := Discover(ctx)
	// We expect either nil (timeout reached) or context.DeadlineExceeded if it actually times out
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("expected no error or deadline exceeded, got %v", err)
	}
}
