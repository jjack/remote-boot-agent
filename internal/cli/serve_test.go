package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/daemon"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
)

type mockServeRunner struct {
	called *bool
	runErr error
}

func (m *mockServeRunner) Run(ctx context.Context) error {
	if m.called != nil {
		*m.called = true
	}
	return m.runErr
}

func TestNewServeCmd_RunEInvokesDaemon(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		if cfg.Port != 1234 {
			t.Fatalf("expected listen port 1234, got %d", cfg.Port)
		}
		if !cfg.ReportBootOptions {
			t.Fatalf("expected ReportBootOptions to be true")
		}
		return &mockServeRunner{called: &called, runErr: errors.New("serve run failed")}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 1234, ReportBootOptions: true},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/tmp/grub.cfg"}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "serve run failed") {
		t.Fatalf("expected serve run failure, got %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}

func TestNewServeCmd_NoReportBootOptions(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		if cfg.ReportBootOptions {
			t.Fatalf("expected ReportBootOptions to be false")
		}
		if regHandler == nil {
			t.Fatalf("expected regHandler to be non-nil")
		}
		if updateHandler == nil {
			t.Fatalf("expected updateHandler to be non-nil")
		}
		return &mockServeRunner{called: &called}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 1234, ReportBootOptions: false},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}

func TestNewServeCmd_DriftDetected(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()
	oldHassPath := grub.HassGrubStationPath
	defer func() { grub.HassGrubStationPath = oldHassPath }()

	// Point HassGrubStationPath to a non-existent file to trigger drift (CheckDrift returns true if file missing)
	grub.HassGrubStationPath = "/tmp/non-existent-grubstation-script"

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		return &mockServeRunner{called: &called}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
		HomeAssistant: config.HomeAssistantConfig{
			URL: "http://ha.local:8123",
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}

func TestNewServeCmd_DriftError(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		return &mockServeRunner{called: &called}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
		HomeAssistant: config.HomeAssistantConfig{
			URL: "invalid-url", // This will cause CheckDrift to return an error
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}

func TestNewServeCmd_ExplicitGrubConfig(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		return &mockServeRunner{called: &called}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: 10,
			URL:             "http://grub.local",
		},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}

type mockManager struct {
	servicemanager.Manager
	name   string
	active bool
}

func (m *mockManager) Name() string                      { return m.name }
func (m *mockManager) IsActive(ctx context.Context) bool { return m.active }

func TestNewServeCmd_WithServiceManager(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		if meta.ServiceManager != "mock-mgr" {
			t.Fatalf("expected service manager mock-mgr, got %s", meta.ServiceManager)
		}
		return &mockServeRunner{called: &called}
	}

	reg := servicemanager.NewRegistry()
	reg.Register("mock-mgr", func() servicemanager.Manager {
		return &mockManager{name: "mock-mgr", active: true}
	})

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 1234},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: reg}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}
