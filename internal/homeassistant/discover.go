package homeassistant

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brutella/dnssd"
)

const homeAssistantService = "_home-assistant._tcp.local."

var (
	discoveryTimeout = 5 * time.Second
)

type ServiceInstance struct {
	Name string
	URLs []string
}

func isSupportedURL(url string) bool {
	return url != "" && (strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://"))
}

func Discover(ctx context.Context) ([]ServiceInstance, error) {
	var instances []ServiceInstance
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	add := func(e dnssd.BrowseEntry) {
		mu.Lock()
		defer mu.Unlock()

		urls := extractURLs(e)
		if len(urls) > 0 {
			instances = append(instances, ServiceInstance{
				Name: e.Name,
				URLs: urls,
			})
		}
	}

	// dnssd.LookupType blocks until context is cancelled
	err := dnssd.LookupType(ctx, homeAssistantService, add, func(e dnssd.BrowseEntry) {})
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, fmt.Errorf("dnssd lookup failed: %w", err)
	}

	return instances, nil
}

func extractURLs(e dnssd.BrowseEntry) []string {
	var urls []string
	seen := make(map[string]bool)

	addURL := func(url string) {
		if url != "" && !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}

	// Try TXT records first
	if url, ok := e.Text["internal_url"]; ok && isSupportedURL(url) {
		addURL(url)
	}
	if url, ok := e.Text["base_url"]; ok && isSupportedURL(url) {
		addURL(url)
	}

	// Fallback to IP:Port
	for _, ip := range e.IPs {
		// Prefer IPv4 for compatibility
		if ip.To4() != nil {
			addURL(fmt.Sprintf("http://%s:%d", ip.String(), e.Port))
		}
	}

	return urls
}
