package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/cli/wizard"
	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
	"github.com/spf13/cobra"

	"github.com/yarlson/tap"
)

type mockInstallInitSystem struct {
	installErr     error
	startErr       error
	permissionErr  error
	isInstalledVal bool
	isInstalledErr error
	configPath     string
}

func (m *mockInstallInitSystem) Name() string                      { return "mock-init" }
func (m *mockInstallInitSystem) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallInitSystem) IsInstalled(ctx context.Context) (bool, error) {
	return m.isInstalledVal, m.isInstalledErr
}

func (m *mockInstallInitSystem) CheckPermissions(ctx context.Context) error {
	return m.permissionErr
}

func (m *mockInstallInitSystem) Install(ctx context.Context, configPath string) error {
	m.configPath = configPath
	return m.installErr
}
func (m *mockInstallInitSystem) Preview(ctx context.Context, configPath string) (string, error) {
	return "[Unit]\nDescription=Mock Service", nil
}
func (m *mockInstallInitSystem) Uninstall(ctx context.Context) error { return nil }
func (m *mockInstallInitSystem) Start(ctx context.Context) error     { return m.startErr }
func (m *mockInstallInitSystem) Stop(ctx context.Context) error      { return nil }
func (m *mockInstallInitSystem) Configure(ctx context.Context, cfg *config.Config) error {
	return nil
}

func TestSetupCmd_Execute(t *testing.T) {
	oldRunGenerateSurvey := wizard.RunGenerateSurvey
	defer func() {
		wizard.RunGenerateSurvey = oldRunGenerateSurvey
	}()

	oldOsMkdirAll := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error { return nil }
	defer func() { osMkdirAll = oldOsMkdirAll }()

	tests := []struct {
		name        string
		setup       func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem)
		args        []string
		wantErr     string
		wantInstall bool
		wantOut     []string
	}{
		{
			name: "Success - Full Installation",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
				}))
				t.Cleanup(ts.Close)

				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{
						HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "fake"},
					}, nil
				}
			},
			wantInstall: true,
			wantOut: []string{
				"Proceeding with installation...",
				"Setup complete!",
			},
		},
		{
			name: "Success - Dry Run from CLI",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{}, nil
				}
			},
			args:        []string{"--dry-run", "--config", "dummy.yaml"},
			wantInstall: false,
			wantOut: []string{
				"Dry run completed. Configuration shown above was not saved.",
			},
		},
		{
			name: "Error - ensureSupport Fails (InitSystem)",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				deps.Registry = servicemanager.NewRegistry() // Empty registry causes init system error
			},
			wantErr:     "no supported service manager detected",
			wantInstall: false,
		},
		{
			name: "Error - Generate Survey Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return nil, errors.New("survey failed")
				}
			},
			wantErr:     "survey failed",
			wantInstall: false,
		},
		{
			name: "Error - Permission Denied before Wizard",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				initMock.permissionErr = errors.New("permission denied: root required")
			},
			wantErr:     "permission denied: root required",
			wantInstall: false,
		},
		{
			name: "Error - MkdirAll Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{}, nil
				}
				osMkdirAll = func(path string, perm os.FileMode) error { return errors.New("mkdirall failed") }
				t.Cleanup(func() { osMkdirAll = func(path string, perm os.FileMode) error { return nil } })
			},
			wantErr:     "failed to create config directory: mkdirall failed",
			wantInstall: false,
		},
		{
			name: "Error - Save Config Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{}, nil
				}
				oldSave := config.Save
				config.Save = func(cfg *config.Config, path string) error {
					return errors.New("save config failed")
				}
				t.Cleanup(func() { config.Save = oldSave })
			},
			wantErr:     "save config failed",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub} // will fail since not mocked correctly
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{
						Daemon: config.DaemonConfig{ReportBootOptions: true},
					}, nil
				}
			},
			wantErr:     "failed to install grub",
			wantInstall: false,
		},
		{
			name: "Success Install, Push Succeeds",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				// Mock successful grub setup
				oldExecLookPath := grub.ExecLookPath
				oldExecCommand := grub.ExecCommand
				oldHassPath := grub.HassGrubStationPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubStationPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubStationPath = oldHassPath
				})

				// Mock successful GetBootOptions and a working HA endpoint
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
				}))
				t.Cleanup(ts.Close)

				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte("menuentry 'OS' {}"), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}

				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{
						HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "fake"},
					}, nil
				}
			},
			wantInstall: true,
			wantOut: []string{
				"Installation completed successfully.",
				"Pushing initial boot options to Home Assistant...",
				"Successfully pushed initial state to Home Assistant.",
			},
		},
		{
			name: "Success Install, Push Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				// Mock successful grub setup
				oldExecLookPath := grub.ExecLookPath
				oldExecCommand := grub.ExecCommand
				oldHassPath := grub.HassGrubStationPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubStationPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubStationPath = oldHassPath
				})

				// Make GetBootOptions fail to trigger error in PushBootOptions
				deps.Grub = &grub.Grub{ConfigPath: "/non/existent/path"}

				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return &config.Config{
						HomeAssistant: config.HomeAssistantConfig{URL: "http://fake", WebhookID: "fake"},
					}, nil
				}
			},
			wantErr:     "request to home assistant failed",
			wantInstall: true,
			wantOut: []string{
				"Installation completed successfully.",
				"Pushing initial boot options to Home Assistant...",
			},
		},
		{
			name: "Setup Aborted on Overwrite No",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				wizard.RunGenerateSurvey = func(ctx context.Context, deps wizard.SurveyDeps, isReinstall bool, currentPort int, isDryRun bool) (*config.Config, error) {
					return nil, wizard.ErrAborted
				}
			},
			wantInstall: false,
			wantOut:     []string{"Setup aborted."},
		},
		{
			name: "Success - Apply Only",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem) {
				deps.Config = &config.Config{
					HomeAssistant: config.HomeAssistantConfig{URL: "http://hass.local:8123", WebhookID: "fake"},
				}
			},
			args:        []string{"--apply", "--config", "dummy.yaml"},
			wantInstall: true,
			wantOut:     []string{"Installation completed successfully."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prevent mock bleed across test iterations
			origRunGenerateSurvey := wizard.RunGenerateSurvey
			defer func() {
				wizard.RunGenerateSurvey = origRunGenerateSurvey
			}()

			initMock := &mockInstallInitSystem{}
			initReg := servicemanager.NewRegistry()
			initReg.Register("mock-init", func() servicemanager.Manager { return initMock })

			deps := &CommandDeps{
				Config:   &config.Config{},
				Grub:     &grub.Grub{},
				Registry: initReg,
			}

			tt.setup(t, deps, initMock)

			cmd := NewSetupCmd(deps)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			// Capture tap output into our buffer as well
			tapOut := tap.NewMockWritable()
			tap.SetTermIO(nil, tapOut)
			defer tap.SetTermIO(nil, nil)

			cmd.Flags().String("config", "dummy.yaml", "")

			finalArgs := tt.args
			if len(finalArgs) == 0 {
				finalArgs = []string{"--config", "dummy.yaml"}
			}
			cmd.SetArgs(finalArgs)

			err := cmd.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if tt.wantInstall {
				if initMock.configPath == "" {
					t.Errorf("expected install to occur, but it didn't")
				}
			} else {
				if initMock.configPath != "" {
					t.Errorf("expected install to NOT occur, but it did")
				}
			}

			if len(tt.wantOut) > 0 {
				outStr := out.String() + strings.Join(tapOut.Buffer, "")
				for _, w := range tt.wantOut {
					if !strings.Contains(outStr, w) {
						t.Errorf("expected output to contain %q, got %q", w, outStr)
					}
				}
			}
		})
	}
}

func TestEnsureSupport(t *testing.T) {
	t.Run("InitSystem Not Supported", func(t *testing.T) {
		deps := &CommandDeps{}
		initReg := servicemanager.NewRegistry()
		deps.Registry = initReg

		_, err := ensureSupport(context.Background(), deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no supported service manager detected") {
			t.Errorf("expected init system not supported error, got %v", err)
		}
	})
}

func TestEnsureSupport_GenericErrors(t *testing.T) {
	t.Run("Grub Generic Error", func(t *testing.T) {
		deps := &CommandDeps{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := ensureSupport(ctx, deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("InitSystem Generic Error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		initReg := servicemanager.NewRegistry()
		initReg.Register("systemd", func() servicemanager.Manager { return &mockSurveyService{} })

		deps := &CommandDeps{
			Grub:     &grub.Grub{ConfigPath: t.TempDir() + "/grub.cfg"},
			Registry: initReg,
		}
		cancel()

		_, err := ensureSupport(ctx, deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestCommandDeps_SurveyDeps(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	initReg.Register("systemd", func() servicemanager.Manager { return &mockInstallInitSystem{} })
	deps := &CommandDeps{
		Registry: initReg,
	}

	sd := deps.SurveyDeps()
	if sd.DiscoverHomeAssistant == nil {
		t.Fatal("expected DiscoverHomeAssistant to be set")
	}
}

type mockSurveyService struct{}

func (m *mockSurveyService) Name() string                                         { return "systemd" }
func (m *mockSurveyService) IsActive(ctx context.Context) bool                    { return true }
func (m *mockSurveyService) IsInstalled(ctx context.Context) (bool, error)        { return false, nil }
func (m *mockSurveyService) CheckPermissions(ctx context.Context) error           { return nil }
func (m *mockSurveyService) Install(ctx context.Context, configPath string) error { return nil }
func (m *mockSurveyService) Preview(ctx context.Context, configPath string) (string, error) {
	return "[Unit]\nDescription=Mock Service", nil
}
func (m *mockSurveyService) Uninstall(ctx context.Context) error                     { return nil }
func (m *mockSurveyService) Start(ctx context.Context) error                         { return nil }
func (m *mockSurveyService) Stop(ctx context.Context) error                          { return nil }
func (m *mockSurveyService) Configure(ctx context.Context, cfg *config.Config) error { return nil }

func TestIsInstalled(t *testing.T) {
	initMock := &mockInstallInitSystem{isInstalledVal: true}
	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager { return initMock })

	deps := &CommandDeps{
		Registry: initReg,
	}

	installed, _ := IsInstalled(context.Background(), deps)
	if !installed {
		t.Error("expected installed to be true")
	}
}

func TestIsInstalled_NoSupport(t *testing.T) {
	deps := &CommandDeps{
		Registry: servicemanager.NewRegistry(), // Empty registry
	}

	installed, err := IsInstalled(context.Background(), deps)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if installed {
		t.Error("expected installed to be false")
	}
}

func TestPerformInstall_NonRoot(t *testing.T) {
	cfg := &config.Config{}
	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager {
		return &mockInstallInitSystem{permissionErr: errors.New("need root")}
	})

	deps := &CommandDeps{
		Config:   cfg,
		Registry: initReg,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := performInstall(cmd, deps, "config.yaml", "")
	if err == nil || !strings.Contains(err.Error(), "need root") {
		t.Errorf("expected permission error, got %v", err)
	}
}

func TestPerformInstall_NoManager(t *testing.T) {
	deps := &CommandDeps{
		Registry: servicemanager.NewRegistry(), // Empty registry
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := performInstall(cmd, deps, "config.yaml", "")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPerformInstall_NoReportBootOptions(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: false},
	}

	initMock := &mockInstallInitSystem{}
	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager { return initMock })

	deps := &CommandDeps{
		Config:   cfg,
		Registry: initReg,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := performInstall(cmd, deps, "config.yaml", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPerformInstall_WithToken(t *testing.T) {
	// Mock HA server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	cfg := &config.Config{
		HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "fake"},
		Daemon:        config.DaemonConfig{ReportBootOptions: true},
	}

	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager { return &mockInstallInitSystem{} })

	// Mock successful grub setup
	oldExecLookPath := grub.ExecLookPath
	oldExecCommand := grub.ExecCommand
	oldHassPath := grub.HassGrubStationPath
	grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
	grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/true")
	}
	grub.HassGrubStationPath = t.TempDir() + "/99_ha_grub_os_reporter"
	defer func() {
		grub.ExecLookPath = oldExecLookPath
		grub.ExecCommand = oldExecCommand
		grub.HassGrubStationPath = oldHassPath
	}()

	tempGrub := t.TempDir() + "/grub.cfg"
	_ = os.WriteFile(tempGrub, []byte("menuentry 'OS' {}"), 0o644)

	deps := &CommandDeps{
		Config:   cfg,
		Grub:     &grub.Grub{ConfigPath: tempGrub},
		Registry: initReg,
	}

	// Suppress tap output
	tap.SetTermIO(nil, tap.NewMockWritable())
	defer tap.SetTermIO(nil, nil)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := performInstall(cmd, deps, "config.yaml", "secret-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsInstalled_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager {
		return &mockInstallInitSystem{isInstalledErr: errors.New("fail")}
	})

	deps := &CommandDeps{
		Registry: initReg,
		Grub:     &grub.Grub{ConfigPath: t.TempDir() + "/grub.cfg"},
	}

	_, err := IsInstalled(context.Background(), deps)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
