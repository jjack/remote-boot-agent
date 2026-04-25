package cli

import (
	"charm.land/huh/v2"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/system"
)

func GenerateConfigForm(
	hostname string,
	hassURL string,
	interfaceOptions []system.InterfaceInfo,
	bootloaderOptions []string,
	defaultBootloader string,
	defaultBootloaderPath string,
	initSystemOptions []string,
	defaultInitSystem string,
) (cfg *config.Config, err error) {
	macAddress := ""
	finalHassURL := hassURL
	webhookID := ""
	finalHostname := hostname
	blName := defaultBootloader
	blPath := defaultBootloaderPath
	initSysName := defaultInitSystem

	var ifaceOpts []huh.Option[string]
	for _, opt := range interfaceOptions {
		ifaceOpts = append(ifaceOpts, huh.NewOption(opt.Label, opt.Value))
	}

	var blOpts []huh.Option[string]
	for _, opt := range bootloaderOptions {
		blOpts = append(blOpts, huh.NewOption(opt, opt))
	}

	var initSysOpts []huh.Option[string]
	for _, opt := range initSystemOptions {
		initSysOpts = append(initSysOpts, huh.NewOption(opt, opt))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Hostname").
				Description("This is how Home Assistant will refer to your machine.\nPress enter to accept or enter a custom hostname").
				Placeholder("my-computer").
				Value(&finalHostname).
				Validate(func(v string) error {
					return config.ValidateHostname(v)
				}),

			huh.NewSelect[string]().
				Title("WOL Interface").
				Options(ifaceOpts...).
				Value(&macAddress).
				Validate(func(v string) error {
					return config.ValidateMACAddress(v)
				}),
		).Title("Host Configuration"),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Bootloader").
				Options(blOpts...).
				Value(&blName),
			huh.NewInput().
				Title("Bootloader Config Path").
				Description("Press enter to accept or enter a custom bootloader path.").
				Value(&blPath),
			huh.NewSelect[string]().
				Title("Init System").
				Options(initSysOpts...).
				Value(&initSysName),
		).Title("System Configuration"),
		huh.NewGroup(
			huh.NewInput().
				Title("Home Assistant URL").
				Description("Press enter to accept (if found) or enter a custom Home Assistant URL").
				Placeholder(hassURL).
				Value(&finalHassURL).
				Validate(func(v string) error {
					return config.ValidateURL(v)
				}),
			huh.NewInput().
				Title("Home Assistant Webhook ID").
				Placeholder("").
				Value(&webhookID).
				Validate(func(v string) error {
					return config.ValidateWebhookID(v)
				}),
		).Title("Home Assistant Configuration"),
	)

	err = form.Run()
	if err != nil {
		return nil, err
	}

	cfg = &config.Config{
		Host: config.HostConfig{
			MACAddress: macAddress,
			Hostname:   finalHostname,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       finalHassURL,
			WebhookID: webhookID,
		},
		Bootloader: config.BootloaderConfig{
			Name:       blName,
			ConfigPath: blPath,
		},
		InitSystem: config.InitSystemConfig{
			Name: initSysName,
		},
	}
	return cfg, nil
}
