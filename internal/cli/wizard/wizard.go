package wizard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

type SystemResolver interface {
	DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error)
	DetectSystemHostname() (string, error)
	GetWOLInterfaces() ([]net.Interface, error)
	GetIPInfo(inf net.Interface) ([]string, map[string]string)
	GetFQDN(hostname string) string
	DiscoverGrubConfig(ctx context.Context) (string, error)
}

type SurveyDeps interface {
	GetSystemResolver() SystemResolver
	IsInstalled(ctx context.Context) (bool, error)
}

var (
	RunGenerateSurvey func(context.Context, SurveyDeps, bool, int) (*config.Config, bool, error) = generateConfigInteractive

	ErrAborted = errors.New("setup aborted")
)

func generateConfigInteractive(ctx context.Context, deps SurveyDeps, isReinstall bool, currentPort int) (*config.Config, bool, error) {
	resolver := deps.GetSystemResolver()

	// 0. Overwrite Confirmation
	installed, err := deps.IsInstalled(ctx)
	if err == nil && installed {
		overwrite := tap.Confirm(ctx, tap.ConfirmOptions{
			Message:      "GrubStation is already installed as a service. Do you want to re-run setup and overwrite the existing configuration?",
			InitialValue: false,
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		if !overwrite {
			return nil, false, ErrAborted
		}
	}

	// 1. Initial Discovery
	hostname, err := resolver.DetectSystemHostname()
	if err != nil {
		return nil, false, err
	}
	interfaces, err := resolver.GetWOLInterfaces()
	if err != nil {
		return nil, false, err
	}

	grubConfigPath, _ := resolver.DiscoverGrubConfig(ctx)

	// Background HA Discovery
	type haResult struct {
		instances []homeassistant.ServiceInstance
		err       error
	}
	haChan := make(chan haResult, 1)
	go func() {
		instances, err := resolver.DiscoverHomeAssistant(ctx)
		haChan <- haResult{instances, err}
	}()

	// 2. Installation Mode
	mode := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Installation Mode",
		Options: GetModeOptions(grubConfigPath),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	reportsBoot, runsDaemon, isDryRun := GetModeFlags(mode)

	// 3. Network Interface
	ifaceIdx := tap.Select(ctx, tap.SelectOptions[int]{
		Message: "Available Network Interface",
		Options: BuildIfaceOptions(interfaces, resolver.GetIPInfo),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	selectedIface := interfaces[ifaceIdx]

	// 4. Host Address
	ips, broadcasts := resolver.GetIPInfo(selectedIface)
	fqdn := resolver.GetFQDN(hostname)
	hostAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Host Address (Used for ping checks and communication with the daemon)",
		Options: BuildHostOptions(hostname, fqdn, ips),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	// 5. Daemon Port
	var AgentPort int
	if runsDaemon {
		defaultValue := strconv.Itoa(config.DefaultAgentPort)
		if currentPort > 0 {
			defaultValue = strconv.Itoa(currentPort)
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
				return ValidatePort(s, isReinstall, currentPort, portChecker)
			},
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		AgentPort, _ = strconv.Atoi(portStr)
	}

	// 6. WOL Address
	WolBroadcastAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "WOL Broadcast Address (you may need to choose subnet broadcast for cross-VLAN setups)",
		Options: BuildWolOptions(ips, broadcasts),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

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
			return nil, false, ctx.Err()
		}
		grubWaitTime, _ = strconv.Atoi(waitStr)
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
			return nil, false, ctx.Err()
		}

		if instIdx != -1 {
			selectedInst := discovered[instIdx]

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
				return nil, false, ctx.Err()
			}

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
						return nil, false, ctx.Err()
					}
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
			return nil, false, ctx.Err()
		}
	}

	// 9. HA Webhook ID
	haWebhook := tap.Password(ctx, tap.PasswordOptions{
		Message: "Home Assistant Webhook ID (generated by the integration)",
		Validate: func(s string) error {
			return config.ValidateWebhookID(s)
		},
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	return AssembleConfig(hostAddress, selectedIface.HardwareAddr.String(), WolBroadcastAddress, haURL, haWebhook, AgentPort, reportsBoot, grubWaitTime, grubConfigPath, grubURL), isDryRun, nil
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
	maskWebhook := true
	out, err := cfg.ToYAML(maskWebhook)
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
