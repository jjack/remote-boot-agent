package cli

import (
	"strconv"

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
	broadcastAddress := ""
	wolPortStr := ""

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

			huh.NewInput().
				Title("Wake-on-LAN Port").
				Description("The UDP port used to send the WOL magic packet.").
				Value(&wolPortStr).
				Validate(func(v string) error {
					return config.ValidateBroadcastPort(v)
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

	broadcastAddrs, _ := system.GetBroadcastAddresses(macAddress)
	if len(broadcastAddrs) > 1 {
		var bcastOpts []huh.Option[string]
		for _, b := range broadcastAddrs {
			bcastOpts = append(bcastOpts, huh.NewOption(b, b))
		}
		err = huh.NewSelect[string]().
			Title("Select WOL Subnet").
			Description("Multiple subnets detected. Choose the one to use for WOL.").
			Options(bcastOpts...).
			Value(&broadcastAddress).
			Run()
		if err != nil {
			return nil, err
		}
	} else if len(broadcastAddrs) == 1 {
		broadcastAddress = broadcastAddrs[0]
	}

	err = huh.NewInput().
		Title("WOL Broadcast Address").
		Description("Press enter to accept the discovered address or enter a custom broadcast address.").
		Value(&broadcastAddress).
		Run()
	if err != nil {
		return nil, err
	}

	wolPort, _ := strconv.Atoi(wolPortStr)

	cfg = &config.Config{
		Host: config.HostConfig{
			MACAddress:       macAddress,
			Hostname:         finalHostname,
			BroadcastAddress: broadcastAddress,
			BroadcastPort:    wolPort,
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
