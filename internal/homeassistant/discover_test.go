package homeassistant

import (
	"context"
	"net"
	"strings"
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

func TestDiscover_Success(t *testing.T) {
	oldLookupType := lookupType
	defer func() { lookupType = oldLookupType }()

	lookupType = func(ctx context.Context, service string, add dnssd.AddFunc, rm dnssd.RmvFunc) error {
		add(dnssd.BrowseEntry{
			Name: "Home",
			IPs:  []net.IP{net.ParseIP("192.168.1.100")},
			Port: 8123,
		})
		return nil
	}

	instances, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	if instances[0].Name != "Home" {
		t.Errorf("expected name Home, got %s", instances[0].Name)
	}

	if len(instances[0].URLs) != 1 || instances[0].URLs[0] != "http://192.168.1.100:8123" {
		t.Errorf("unexpected URLs: %v", instances[0].URLs)
	}
}

func TestDiscover_Error(t *testing.T) {
	oldLookupType := lookupType
	defer func() { lookupType = oldLookupType }()

	lookupType = func(ctx context.Context, service string, add dnssd.AddFunc, rm dnssd.RmvFunc) error {
		return net.ErrClosed
	}

	_, err := Discover(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dnssd lookup failed") {
		t.Fatalf("expected dnssd lookup failed error, got %v", err)
	}
}
