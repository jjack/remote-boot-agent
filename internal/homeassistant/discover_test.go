package homeassistant

import (
	"testing"
)

func TestDiscover_Timeout(t *testing.T) {
	// Without a zeroconf server, this will timeout and return an empty string.
	url, err := Discover()
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
	if url != "" {
		t.Logf("Found HA at %s", url)
	}
}
