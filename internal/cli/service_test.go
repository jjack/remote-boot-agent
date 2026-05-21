package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/host"
	"github.com/jjack/grubstation/internal/servicemanager"
)

type mockServiceManager struct {
	name         string
	activeCalls  int
	active       bool
	installed    bool
	installErr   error
	uninstallErr error
	startErr     error
	stopErr      error
}

func (m *mockServiceManager) Name() string { return m.name }
func (m *mockServiceManager) IsActive(ctx context.Context) bool {
	res := m.active
	if m.activeCalls == 0 {
		res = true // Force true for Detect
	}
	m.activeCalls++
	return res
}
func (m *mockServiceManager) IsInstalled(ctx context.Context) (bool, error) {
	return m.installed, nil
}
func (m *mockServiceManager) CheckPermissions(ctx context.Context) error { return nil }
func (m *mockServiceManager) Install(ctx context.Context, configPath string) error {
	return m.installErr
}
func (m *mockServiceManager) Preview(ctx context.Context, configPath string) (string, error) {
	return "preview", nil
}
func (m *mockServiceManager) Uninstall(ctx context.Context) error { return m.uninstallErr }
func (m *mockServiceManager) Start(ctx context.Context) error     { return m.startErr }
func (m *mockServiceManager) Stop(ctx context.Context) error      { return m.stopErr }
func (m *mockServiceManager) Configure(ctx context.Context, cfg *config.Config) error {
	return nil
}

func TestServiceInstallCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: initReg,
	}

	cmd := NewServiceInstallCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Installing service: mock-svc") {
		t.Errorf("expected installing message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Installation completed successfully.") {
		t.Errorf("expected success message, got %q", out.String())
	}
}

func TestServiceRemoveCmd(t *testing.T) {
	oldExecLookPath := grub.ExecLookPath
	oldExecCommand := grub.ExecCommand
	oldHassPath := grub.HassGrubStationPath
	t.Cleanup(func() {
		grub.ExecLookPath = oldExecLookPath
		grub.ExecCommand = oldExecCommand
		grub.HassGrubStationPath = oldHassPath
	})

	grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
	grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/true")
	}
	grub.HassGrubStationPath = filepath.Join(t.TempDir(), "99_grubstation")

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{ReportBootOptions: true}},
		Grub:     &grub.Grub{ConfigPath: filepath.Join(t.TempDir(), "grub.cfg")},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Removing service: mock-svc") {
		t.Errorf("expected removing message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Removal completed successfully.") {
		t.Errorf("expected success message, got %q", out.String())
	}
}

func TestServiceStartCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStartCmd(deps)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceStopCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStopCmd(deps)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceStatusCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ALIVE"))
		}
	}))
	defer ts.Close()

	// Extract port from ts.URL
	var port int
	_, _ = fmt.Sscanf(ts.URL, "http://127.0.0.1:%d", &port)

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: port}},
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Service mock-svc is active") {
		t.Errorf("expected active message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Daemon status: ALIVE") {
		t.Errorf("expected status message, got %q", out.String())
	}
}

func TestServiceStatusCmd_Inactive(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: false}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: 0}}, // Port 0 will fail
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Service mock-svc is inactive") {
		t.Errorf("expected inactive message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Daemon status check failed") {
		t.Errorf("expected failed status check message, got %q", out.String())
	}
}

func TestServiceStatusCmd_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	var port int
	_, _ = fmt.Sscanf(ts.URL, "http://127.0.0.1:%d", &port)

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: port}},
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Daemon status check returned non-OK status: 404") {
		t.Errorf("expected 404 status check message, got %q", out.String())
	}
}

func TestServiceCmd(t *testing.T) {
	deps := &CommandDeps{}
	cmd := NewServiceCmd(deps)
	if cmd.Use != "service" {
		t.Errorf("expected Use 'service', got %q", cmd.Use)
	}
}

func TestServiceRemoveCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true, uninstallErr: errors.New("uninstall failed")}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "uninstall failed") {
		t.Fatalf("expected uninstall error, got %v", err)
	}
}

func TestServiceRemoveCmd_GrubError(t *testing.T) {
	oldExecLookPath := grub.ExecLookPath
	oldExecCommand := grub.ExecCommand
	oldHassPath := grub.HassGrubStationPath
	t.Cleanup(func() {
		grub.ExecLookPath = oldExecLookPath
		grub.ExecCommand = oldExecCommand
		grub.HassGrubStationPath = oldHassPath
	})

	grub.ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/bin/false", nil
		}
		return "", errors.New("not found")
	}
	grub.ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	grub.HassGrubStationPath = filepath.Join(t.TempDir(), "99_grubstation")

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{ReportBootOptions: true}},
		Grub:     &grub.Grub{ConfigPath: "/invalid/path"},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to uninstall grub") {
		t.Fatalf("expected grub uninstall error, got %v", err)
	}
}

func TestServiceRemoveCmd_Unregister(t *testing.T) {
	var receivedPayload map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "api/webhook/test-webhook") {
			_ = json.NewDecoder(r.Body).Decode(&receivedPayload)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}
	}))
	defer ts.Close()

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config: &config.Config{
			Host: config.HostConfig{
				Address:    "1.2.3.4",
				MACAddress: "AA:BB:CC:DD:EE:FF",
			},
			HomeAssistant: config.HomeAssistantConfig{
				URL:       ts.URL,
				WebhookID: "test-webhook",
			},
		},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Unregistering from Home Assistant...") {
		t.Errorf("expected unregistering message, got %q", out.String())
	}

	if receivedPayload == nil {
		t.Fatal("expected to receive payload at Home Assistant mock")
	}

	if receivedPayload["action"] != "unregister_host" {
		t.Errorf("expected action 'unregister_host', got %v", receivedPayload["action"])
	}
	if receivedPayload["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected mac 'AA:BB:CC:DD:EE:FF', got %v", receivedPayload["mac"])
	}
	if receivedPayload["address"] != "1.2.3.4" {
		t.Errorf("expected address '1.2.3.4', got %v", receivedPayload["address"])
	}
}

func TestServiceRemoveCmd_UnregisterFallback(t *testing.T) {
	var receivedPayload map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	hwAddr, _ := net.ParseMAC("00:11:22:33:44:55")

	// Mock host package
	oldNetInterfaces := host.NetInterfaces
	oldGetAddrs := host.GetAddrs
	oldOsStat := host.OsStat
	host.NetInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "eth0", HardwareAddr: hwAddr, Flags: net.FlagUp}}, nil
	}
	host.GetAddrs = func(iface net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}}, nil
	}
	host.OsStat = func(name string) (os.FileInfo, error) {
		return nil, nil // mock device file exists
	}
	t.Cleanup(func() {
		host.NetInterfaces = oldNetInterfaces
		host.GetAddrs = oldGetAddrs
		host.OsStat = oldOsStat
	})

	deps := &CommandDeps{
		Config: &config.Config{
			HomeAssistant: config.HomeAssistantConfig{
				URL:       ts.URL,
				WebhookID: "test-webhook",
			},
		},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPayload["mac"] != "00:11:22:33:44:55" {
		t.Errorf("expected detected mac '00:11:22:33:44:55', got %v", receivedPayload["mac"])
	}
	if receivedPayload["address"] != "192.168.1.10" {
		t.Errorf("expected detected address '192.168.1.10', got %v", receivedPayload["address"])
	}
}

func TestServiceInstallCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true, installErr: errors.New("install failed")}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: initReg,
	}

	cmd := NewServiceInstallCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "install failed") {
		t.Fatalf("expected install error, got %v", err)
	}
}

func TestServiceInstallCmd_NoManager(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceInstallCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error due to no manager, got nil")
	}
}

func TestServiceStartCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	// No services registered, Detect will fail
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStartCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestServiceStopCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStopCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestServiceStatusCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStatusCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}
