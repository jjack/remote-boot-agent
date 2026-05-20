package homeassistant

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	homeAssistantService = "_home-assistant._tcp"
	searchDomain         = "local"
)

var discoveryTimeout = 5 * time.Second

type ServiceInstance struct {
	Name string
	URLs []string
}

func isSupportedURL(url string) bool {
	return url != "" && (strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://"))
}

func Discover(ctx context.Context) ([]ServiceInstance, error) {
	slog.Debug("Starting Home Assistant discovery via zeroconf", "service", homeAssistantService, "domain", searchDomain)

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var instances []ServiceInstance
	done := make(chan struct{})

	go func() {
		for entry := range entries {
			urls := extractURLs(entry)
			if len(urls) > 0 {
				instances = append(instances, ServiceInstance{
					Name: entry.Instance,
					URLs: urls,
				})
			}
		}
		close(done)
	}()

	browseCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	err = resolver.Browse(browseCtx, homeAssistantService, searchDomain, entries)
	if err != nil {
		return nil, fmt.Errorf("failed to browse: %w", err)
	}

	<-browseCtx.Done()
	<-done

	return instances, nil
}

func extractURLs(e *zeroconf.ServiceEntry) []string {
	var urls []string
	seen := make(map[string]bool)

	addURL := func(url string) {
		if url != "" && !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}

	// Parse TXT records
	txtMap := make(map[string]string)
	for _, txt := range e.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) == 2 {
			txtMap[parts[0]] = parts[1]
		}
	}
	if len(txtMap) > 0 {
		slog.Debug("Parsed TXT records", "instance", e.Instance, "data", txtMap)
	}

	// Try TXT records first
	if url, ok := txtMap["internal_url"]; ok && isSupportedURL(url) {
		slog.Debug("Found internal_url in TXT", "instance", e.Instance, "url", url)
		addURL(url)
	}
	if url, ok := txtMap["base_url"]; ok && isSupportedURL(url) {
		slog.Debug("Found base_url in TXT", "instance", e.Instance, "url", url)
		addURL(url)
	}

	// Fallback to IP:Port
	for _, ip := range e.AddrIPv4 {
		url := fmt.Sprintf("http://%s:%d", ip.String(), e.Port)
		slog.Debug("Falling back to IP:Port URL", "instance", e.Instance, "url", url)
		addURL(url)
	}

	return urls
}
