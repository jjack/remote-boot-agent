package wizard

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

func setupSurveyDeps(t *testing.T) SurveyDeps {
	return SurveyDeps{
		DiscoverHomeAssistant: func(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
			return []homeassistant.ServiceInstance{{Name: "Home", URLs: []string{"http://hass.local:8123"}}}, nil
		},
		DiscoverGrubConfig: func(ctx context.Context) (string, error) {
			return "/boot/grub/grub.cfg", nil
		},
		DetectSystemHostname: func() (string, error) {
			return "detected-host", nil
		},
		GetWOLInterfaces: func() ([]net.Interface, error) {
			mac, _ := net.ParseMAC("00:11:22:33:44:55")
			return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
		},
		GetIPInfo: func(inf net.Interface) ([]string, map[string]string) {
			return []string{"192.168.1.100"}, map[string]string{"192.168.1.100": "192.168.1.255"}
		},
		GetFQDN: func(hostname string, inf *net.Interface) string {
			return "detected-host.local"
		},
		IsInstalled: func(ctx context.Context) (bool, error) {
			return false, nil
		},
	}
}

func TestPrintConfigSummary(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	tapOut := tap.NewMockWritable()
	tap.SetTermIO(nil, tapOut)
	defer tap.SetTermIO(nil, nil)

	cfg := &config.Config{
		Host: config.HostConfig{
			Address:    "192.168.1.50",
			MACAddress: "00:11:22:33:44:55",
		},
		WakeOnLan: &config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    99,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local:8123",
			WebhookID: strings.Repeat("a", 64),
		},
		Daemon: config.DaemonConfig{
			Port:              8081,
			ReportBootOptions: true,
		},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: 2,
		},
	}

	PrintConfigSummary(cmd, cfg, "/etc/grubstation/config.yaml")

	out := strings.Join(tapOut.Buffer, "")
	if !strings.Contains(out, "/etc/grubstation/config.yaml") {
		t.Errorf("expected config path, got %s", out)
	}
	if !strings.Contains(out, "aaaa...") {
		t.Errorf("expected truncated webhook id, got %s", out)
	}
}

func TestGenerateConfigSurvey_HostnameError(t *testing.T) {
	ctx := context.Background()
	deps := setupSurveyDeps(t)
	deps.DetectSystemHostname = func() (string, error) {
		return "", errors.New("hostname error")
	}

	_, err := generateConfigInteractive(ctx, deps, false, 0, false)
	if err == nil || !strings.Contains(err.Error(), "hostname error") {
		t.Errorf("expected hostname error, got %v", err)
	}
}

func TestGenerateConfigSurvey_InterfacesError(t *testing.T) {
	ctx := context.Background()
	deps := setupSurveyDeps(t)
	deps.GetWOLInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("interfaces error")
	}

	_, err := generateConfigInteractive(ctx, deps, false, 0, false)
	if err == nil || !strings.Contains(err.Error(), "interfaces error") {
		t.Errorf("expected interfaces error, got %v", err)
	}
}
