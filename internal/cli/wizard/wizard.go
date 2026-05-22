package wizard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/jjack/grubstation/internal/host"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

type SystemState struct {
	Hostname       string
	Interfaces     []net.Interface
	GrubConfigPath string
	IsReinstall    bool
	CurrentPort    int
}

var (
	RunGenerateSurvey func(context.Context, SystemState, bool) (*config.Config, error) = generateConfigInteractive

	ErrAborted = errors.New("setup aborted")
)

func generateConfigInteractive(ctx context.Context, state SystemState, isDryRun bool) (*config.Config, error) {
	// 0. Overwrite Confirmation
	if state.IsReinstall && !isDryRun {
		overwrite := tap.Confirm(ctx, tap.ConfirmOptions{
			Message:      "GrubStation is already configured. Do you want to re-run setup and overwrite the existing configuration?",
			InitialValue: false,
		})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !overwrite {
			return nil, ErrAborted
		}
	}

	// Initial Discovery (hostname and interfaces are already passed in state)
	hostname := state.Hostname
	interfaces := state.Interfaces
	grubConfigPath := state.GrubConfigPath

	// Background: HA Discovery
	type haResult struct {
		instances []homeassistant.ServiceInstance
		err       error
	}
	haChan := make(chan haResult, 1)
	go func() {
		slog.Debug("Starting background Home Assistant discovery")
		instances, err := homeassistant.DiscoverFunc(ctx)
		if err != nil {
			slog.Debug("Background HA discovery failed", "error", err)
		} else {
			slog.Debug("Background HA discovery complete", "count", len(instances))
		}
		haChan <- haResult{instances, err}
	}()

	// Background: Global CNAME resolution (can be slow)
	type globalInfo struct {
		fqdn string
	}
	globalInfoChan := make(chan globalInfo, 1)
	go func() {
		slog.Debug("Starting background global FQDN resolution", "hostname", hostname)
		fqdn := host.GetFQDN(hostname, nil)
		slog.Debug("Background global FQDN resolution complete", "fqdn", fqdn)
		globalInfoChan <- globalInfo{fqdn}
	}()

	// 2. Installation Mode
	mode := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Installation Mode",
		Options: GetModeOptions(grubConfigPath),
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	slog.Debug("Selected installation mode", "mode", mode)

	// 3. Network Interface
	ifaceIdx := tap.Select(ctx, tap.SelectOptions[int]{
		Message: "Available Network Interface",
		Options: BuildIfaceOptions(interfaces, host.GetIPInfo),
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	selectedIface := interfaces[ifaceIdx]
	slog.Debug("Selected network interface", "interface", selectedIface.Name, "mac", selectedIface.HardwareAddr.String())

	// 4. Host Address
	ips, broadcasts := host.GetIPInfo(selectedIface)
	slog.Debug("Interface IP info", "ips", ips, "broadcasts", broadcasts)

	// Local FQDN resolution (fast on Windows, just a local check)
	localFQDN := host.GetFQDN(hostname, &selectedIface)
	slog.Debug("Local FQDN resolution result", "fqdn", localFQDN)

	// Global FQDN resolution (slow on Windows)
	var globalFQDN string
	select {
	case res := <-globalInfoChan:
		globalFQDN = res.fqdn
	default:
		s := tap.NewSpinner(tap.SpinnerOptions{})
		s.Start("Resolving network information...")
		res := <-globalInfoChan
		s.Stop("Network information resolved", 0)
		globalFQDN = res.fqdn
	}
	slog.Debug("Global FQDN resolution result", "fqdn", globalFQDN)

	hostAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Host Address (Used for communication with the daemon)",
		Options: BuildHostOptions(hostname, globalFQDN, localFQDN, ips),
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	slog.Debug("Selected host address", "address", hostAddress)

	reportsBoot, runsDaemon := GetModeFlags(mode)

	// 5. Daemon Port
	var AgentPort int
	if runsDaemon {
		defaultValue := strconv.Itoa(config.DefaultAgentPort)
		if state.CurrentPort > 0 {
			defaultValue = strconv.Itoa(state.CurrentPort)
		}
		portStr := tap.Text(ctx, tap.TextOptions{
			Message:      fmt.Sprintf("Daemon Port (default: %d)", config.DefaultAgentPort),
			DefaultValue: defaultValue,
			InitialValue: defaultValue,
			Validate: func(s string) error {
				portChecker := CheckPortAvailability
				if os.Getenv("GRUBSTATION_SKIP_PORT_CHECK") == "true" {
					portChecker = func(int) error { return nil }
				}
				return ValidatePort(s, state.IsReinstall, state.CurrentPort, portChecker)
			},
		})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		AgentPort, _ = strconv.Atoi(portStr)
		slog.Debug("Selected daemon port", "port", AgentPort)
	}

	// 6. WOL Address
	WolBroadcastAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "WOL Broadcast Address (you may need to choose subnet broadcast for cross-VLAN setups)",
		Options: BuildWolOptions(ips, broadcasts),
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	slog.Debug("Selected WOL broadcast address", "address", WolBroadcastAddress)

	var grubWaitTime int
	if reportsBoot {
		defaultWait := strconv.Itoa(config.DefaultGrubWaitSeconds)
		waitStr := tap.Text(ctx, tap.TextOptions{
			Message:      "GRUB Network Wait (seconds to wait for network before getting next boot option from Home Assistant)",
			DefaultValue: defaultWait,
			InitialValue: defaultWait,
			Validate: func(s string) error {
				return config.ValidateGrubWaitTime(s)
			},
		})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		grubWaitTime, _ = strconv.Atoi(waitStr)
		slog.Debug("Selected GRUB wait time", "seconds", grubWaitTime)
	} else {
		grubConfigPath = ""
	}

	// 8. Home Assistant URL
	var discovered []homeassistant.ServiceInstance
	select {
	case res := <-haChan:
		discovered = res.instances
	default:
		s := tap.NewSpinner(tap.SpinnerOptions{})
		s.Start("Discovering Home Assistant...")
		res := <-haChan
		s.Stop("Home Assistant discovery complete", 0)
		discovered = res.instances
	}

	var haURL string
	var grubURL string

	totalURLs := 0
	for _, inst := range discovered {
		totalURLs += len(inst.URLs)
	}

	if totalURLs == 1 {
		// Single URL fallback
		haURL = discovered[0].URLs[0]
		slog.Debug("Single Home Assistant instance discovered, using it", "url", haURL)
	} else if len(discovered) > 0 {
		// 1. Instance Selection
		var instOpts []tap.SelectOption[int]
		for i, inst := range discovered {
			label := fmt.Sprintf("%s (%s)", inst.Name, strings.Join(inst.URLs, ", "))
			instOpts = append(instOpts, tap.SelectOption[int]{Value: i, Label: label})
		}
		instOpts = append(instOpts, tap.SelectOption[int]{Value: -1, Label: "Other (Enter manually)"})

		instIdx := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Home Assistant instances discovered. Please select one:",
			Options: instOpts,
		})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if instIdx != -1 {
			selectedInst := discovered[instIdx]
			slog.Debug("Selected Home Assistant instance", "name", selectedInst.Name)

			// 2. Agent URL Selection
			var agentOpts []tap.SelectOption[string]
			for _, u := range selectedInst.URLs {
				label := u
				if strings.HasPrefix(u, "https://") {
					label += " (HTTPS is Preferred)"
				}
				agentOpts = append(agentOpts, tap.SelectOption[string]{Value: u, Label: label})
			}

			haURL = tap.Select(ctx, tap.SelectOptions[string]{
				Message: fmt.Sprintf("Select URL for the Agent (%s):", selectedInst.Name),
				Options: agentOpts,
			})
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			slog.Debug("Selected Home Assistant URL", "url", haURL)

			// 3. GRUB URL Selection (only if Agent is HTTPS)
			if strings.HasPrefix(haURL, "https://") {
				var grubOpts []tap.SelectOption[string]
				for _, u := range selectedInst.URLs {
					if !strings.HasPrefix(u, "https://") {
						grubOpts = append(grubOpts, tap.SelectOption[string]{Value: u, Label: u})
					}
				}

				if len(grubOpts) > 0 {
					grubURL = tap.Select(ctx, tap.SelectOptions[string]{
						Message: "Select HTTP URL for GRUB (HTTPS not readily supported in GRUB):",
						Options: grubOpts,
					})
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					slog.Debug("Selected GRUB HTTP URL", "url", grubURL)
				}
			}
		}
	}

	if haURL == "" {
		haURL = tap.Text(ctx, tap.TextOptions{
			Message: "Home Assistant URL",
			Validate: func(s string) error {
				skipCheck := os.Getenv("GRUBSTATION_SKIP_HA_URL_CHECK") == "true"
				return ValidateHAURL(ctx, s, skipCheck, CheckHAConnection)
			},
		})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Debug("Manually entered Home Assistant URL", "url", haURL)
	}

	// 9. HA Webhook ID
	haWebhook := tap.Password(ctx, tap.PasswordOptions{
		Message: "Home Assistant Webhook ID (generated by the integration)",
		Validate: func(s string) error {
			return config.ValidateWebhookID(s)
		},
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	slog.Debug("Home Assistant Webhook ID provided and validated")

	return AssembleConfig(hostAddress, selectedIface.HardwareAddr.String(), WolBroadcastAddress, haURL, haWebhook, AgentPort, reportsBoot, grubWaitTime, grubConfigPath, grubURL), nil
}

// AssembleConfig is a pure function that populates the Config struct.
func AssembleConfig(hostAddress, mac, wolAddress, haURL, haWebhook string, agentPort int, reportsBoot bool, grubWait int, grubPath, grubURL string) *config.Config {
	return &config.Config{
		Host: config.HostConfig{
			Address:    hostAddress,
			MACAddress: mac,
		},
		WakeOnLan: &config.WakeOnLanConfig{
			Address: wolAddress,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       haURL,
			WebhookID: haWebhook,
		},
		Daemon: config.DaemonConfig{
			Port:              agentPort,
			ReportBootOptions: reportsBoot,
		},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: grubWait,
			ConfigPath:      grubPath,
			URL:             grubURL,
		},
	}
}

func PrintConfigSummary(cmd *cobra.Command, cfg *config.Config, cfgPath string) {
	exporter := &config.Exporter{
		Config:     *cfg,
		Mask:       true,
		Exhaustive: false,
	}
	out, err := exporter.ToYAML()
	if err != nil {
		tap.Message(fmt.Sprintf("Error generating summary: %v", err))
		return
	}

	tap.Message(fmt.Sprintf("Configuration saved to %s", cfgPath))
	out = fmt.Sprintf("\n---\n%s", out)
	tap.Box(out, " Configuration Preview ", tap.BoxOptions{
		ContentPadding: 2,
	})
}
