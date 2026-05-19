package wizard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jjack/grubstation/internal/config"
	"github.com/yarlson/tap"
)

// Mode constants
const (
	ModeDaemonBoth     = "Daemon (Remote shutdown + Report boot options)"
	ModeDaemonShutdown = "Daemon (Remote shutdown only)"
	ModeHookOnly       = "Shutdown hook (Report boot options only)"
	ModeDryRun         = "Dry Run (Preview configuration only)"
)

// GetModeOptions returns the available installation modes based on whether a GRUB config was found.
func GetModeOptions(grubConfigPath string) []tap.SelectOption[string] {
	if grubConfigPath != "" {
		return []tap.SelectOption[string]{
			{Value: ModeDaemonBoth, Label: ModeDaemonBoth},
			{Value: ModeDaemonShutdown, Label: ModeDaemonShutdown},
			{Value: ModeHookOnly, Label: ModeHookOnly},
			{Value: ModeDryRun, Label: ModeDryRun},
		}
	}
	return []tap.SelectOption[string]{
		{Value: ModeDaemonShutdown, Label: ModeDaemonShutdown},
		{Value: ModeDryRun, Label: ModeDryRun},
	}
}

// GetModeFlags converts a selected mode string into boolean flags.
func GetModeFlags(mode string) (reportsBoot, runsDaemon, isDryRun bool) {
	isDryRun = mode == ModeDryRun
	reportsBoot = mode == ModeDaemonBoth || mode == ModeHookOnly || mode == ModeDryRun
	runsDaemon = mode == ModeDaemonBoth || mode == ModeDaemonShutdown || mode == ModeDryRun
	return
}

// BuildIfaceOptions builds the selection options for network interfaces.
func BuildIfaceOptions(interfaces []net.Interface, ipProvider func(net.Interface) ([]string, map[string]string)) []tap.SelectOption[int] {
	var opts []tap.SelectOption[int]
	for i, inf := range interfaces {
		ips, _ := ipProvider(inf)
		desc := fmt.Sprintf("(%s) [%s]", inf.HardwareAddr.String(), strings.Join(ips, ", "))
		opts = append(opts, tap.SelectOption[int]{
			Value: i,
			Label: inf.Name,
			Hint:  desc,
		})
	}
	return opts
}

// BuildHostOptions builds the selection options for the host address.
func BuildHostOptions(hostname, fqdn string, ips []string) []tap.SelectOption[string] {
	var opts []tap.SelectOption[string]
	if fqdn != "" && fqdn != hostname {
		opts = append(opts, tap.SelectOption[string]{Value: fqdn, Label: fqdn, Hint: "FQDN"})
	}
	opts = append(opts, tap.SelectOption[string]{Value: hostname, Label: hostname, Hint: "Hostname"})
	for _, ip := range ips {
		opts = append(opts, tap.SelectOption[string]{Value: ip, Label: ip, Hint: "IP Address [Ensure This is Static!]"})
	}
	return opts
}

// BuildWolOptions builds the selection options for the WOL broadcast address.
func BuildWolOptions(ips []string, ipBroadcasts map[string]string) []tap.SelectOption[string] {
	opts := []tap.SelectOption[string]{
		{Value: config.DefaultWolBroadcastAddress, Label: fmt.Sprintf("%s (Default)", config.DefaultWolBroadcastAddress)},
	}

	seenBroadcasts := make(map[string]bool)
	for _, ip := range ips {
		bc, ok := ipBroadcasts[ip]
		if !ok {
			continue
		}

		if !seenBroadcasts[bc] {
			seenBroadcasts[bc] = true
			opts = append(opts, tap.SelectOption[string]{
				Value: bc,
				Label: fmt.Sprintf("%s (Subnet broadcast for %s)", bc, ip),
			})
		}
	}
	return opts
}

// ValidatePort checks if a port is valid and available.
func ValidatePort(s string, isReinstall bool, currentPort int, portChecker func(int) error) error {
	if err := config.ValidatePort(s); err != nil {
		return err
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if isReinstall && port == currentPort {
		return nil
	}

	return portChecker(port)
}

// CheckPortAvailability is the default implementation of the port checker.
func CheckPortAvailability(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is in use or unavailable: %v", port, err)
	}
	_ = listener.Close()
	return nil
}

// ValidateHAURL checks if a Home Assistant URL is valid and reachable.
func ValidateHAURL(ctx context.Context, s string, skipCheck bool, urlChecker func(context.Context, string) error) error {
	if err := config.ValidateURL(s); err != nil {
		return err
	}

	if skipCheck {
		return nil
	}

	return urlChecker(ctx, s)
}

// CheckHAConnection is the default implementation of the HA URL connection checker.
func CheckHAConnection(ctx context.Context, url string) error {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("invalid request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to HA URL: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}
