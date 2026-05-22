package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
)

func TestNewServeCmd_Execute(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0, ReportBootOptions: false},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	// Create a context that is immediately cancelled so Run returns quickly
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_DriftDetected(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0, ReportBootOptions: true},
		HomeAssistant: config.HomeAssistantConfig{
			URL: "http://ha.local:8123",
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}

	// Point HassGrubStationPath to a non-existent file to trigger drift (CheckDrift returns true if file missing)
	deps.Grub.HassGrubStationPath = "/tmp/non-existent-grubstation-script"

	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_DriftError(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0, ReportBootOptions: true},
		HomeAssistant: config.HomeAssistantConfig{
			URL: "invalid-url", // This will cause CheckDrift to return an error
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_ExplicitGrubConfig(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0, ReportBootOptions: true},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: 10,
			URL:             "http://grub.local",
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_MinimalConfig(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0},
	}
	// Minimal config with no optional sections
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_WithWOL(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0},
		WakeOnLan: &config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    9,
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServeCmd_NoGrubConfig(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0, ReportBootOptions: true},
		Grub:   nil,
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

type mockManager struct {
	servicemanager.Manager
	name   string
	active bool
}

func (m *mockManager) Name() string                      { return m.name }
func (m *mockManager) IsActive(ctx context.Context) bool { return m.active }
func (m *mockManager) Configure(ctx context.Context, cfg *config.Config) error {
	return nil
}

func TestNewServeCmd_WithServiceManager(t *testing.T) {
	reg := servicemanager.NewRegistry()
	reg.Register("mock-mgr", func() servicemanager.Manager {
		return &mockManager{name: "mock-mgr", active: true}
	})

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 0},
	}
	deps := &CommandDeps{Config: cfg, Grub: grub.NewGrub(), Registry: reg}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cmd.ExecuteContext(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}
