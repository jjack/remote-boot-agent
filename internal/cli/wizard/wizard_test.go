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

type mockSystemResolver struct {
	discoverHomeAssistantFunc func(ctx context.Context) ([]homeassistant.ServiceInstance, error)
	detectSystemHostnameFunc  func() (string, error)
	getWOLInterfacesFunc      func() ([]net.Interface, error)
	getIPInfoFunc             func(inf net.Interface) ([]string, map[string]string)
	getFQDNFunc               func(hostname string) string
	saveConfigFunc            func(cfg *config.Config, path string) error
	discoverGrubConfigFunc    func(ctx context.Context) (string, error)
}

func (m *mockSystemResolver) DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
	if m.discoverHomeAssistantFunc != nil {
		return m.discoverHomeAssistantFunc(ctx)
	}
	return []homeassistant.ServiceInstance{{Name: "Home", URLs: []string{"http://hass.local:8123"}}}, nil
}

func (m *mockSystemResolver) DiscoverGrubConfig(ctx context.Context) (string, error) {
	if m.discoverGrubConfigFunc != nil {
		return m.discoverGrubConfigFunc(ctx)
	}
	return "/boot/grub/grub.cfg", nil
}

func (m *mockSystemResolver) DetectSystemHostname() (string, error) {
	if m.detectSystemHostnameFunc != nil {
		return m.detectSystemHostnameFunc()
	}
	return "detected-host", nil
}

func (m *mockSystemResolver) GetWOLInterfaces() ([]net.Interface, error) {
	if m.getWOLInterfacesFunc != nil {
		return m.getWOLInterfacesFunc()
	}
	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
}

func (m *mockSystemResolver) GetIPInfo(inf net.Interface) ([]string, map[string]string) {
	if m.getIPInfoFunc != nil {
		return m.getIPInfoFunc(inf)
	}
	return []string{"192.168.1.100"}, map[string]string{"192.168.1.100": "192.168.1.255"}
}

func (m *mockSystemResolver) GetFQDN(hostname string) string {
	if m.getFQDNFunc != nil {
		return m.getFQDNFunc(hostname)
	}
	return "detected-host.local"
}

func (m *mockSystemResolver) SaveConfig(cfg *config.Config, path string) error {
	if m.saveConfigFunc != nil {
		return m.saveConfigFunc(cfg, path)
	}
	return nil
}

type mockSurveyDeps struct {
	resolver     *mockSystemResolver
	services     []string
	installed    bool
	installedErr error
}

func (m *mockSurveyDeps) GetSystemResolver() SystemResolver { return m.resolver }
func (m *mockSurveyDeps) IsInstalled(ctx context.Context) (bool, error) {
	return m.installed, m.installedErr
}
func (m *mockSurveyDeps) GetSupportedServices(ctx context.Context) []string { return m.services }

func setupSurveyDeps(t *testing.T) *mockSurveyDeps {
	return &mockSurveyDeps{
		resolver: &mockSystemResolver{},
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
	deps.resolver.detectSystemHostnameFunc = func() (string, error) {
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
	deps.resolver.getWOLInterfacesFunc = func() ([]net.Interface, error) {
		return nil, errors.New("interfaces error")
	}

	_, err := generateConfigInteractive(ctx, deps, false, 0, false)
	if err == nil || !strings.Contains(err.Error(), "interfaces error") {
		t.Errorf("expected interfaces error, got %v", err)
	}
}
