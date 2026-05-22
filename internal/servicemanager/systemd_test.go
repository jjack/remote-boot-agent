//go:build linux

package servicemanager

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSystemd(t *testing.T) {
	s := NewSystemd().(*Systemd)

	t.Run("Basic", func(t *testing.T) {
		if s.Name() != "systemd" {
			t.Errorf("expected systemd, got %s", s.Name())
		}

		oldDir := systemdDir
		defer func() { systemdDir = oldDir }()
		systemdDir = t.TempDir()
		if !s.IsActive(context.Background()) {
			t.Error("expected active when systemd directory exists")
		}
	})

	t.Run("Install_Success", func(t *testing.T) {
		s.OsExecutable = func() (string, error) { return "/app", nil }
		s.OsWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}

		if err := s.Install(context.Background(), "config.yaml"); err != nil {
			t.Errorf("Install failed: %v", err)
		}
	})

	t.Run("Install_Errors", func(t *testing.T) {
		s.OsExecutable = func() (string, error) { return "", errors.New("exe fail") }
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on executable fail")
		}

		s.OsExecutable = func() (string, error) { return "/app", nil }
		s.OsWriteFile = func(name string, data []byte, perm os.FileMode) error { return errors.New("write fail") }
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on write fail")
		}

		s.OsWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("false") // CombinedOutput returns error on non-zero exit
		}
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on command execution fail")
		}
	})

	t.Run("Uninstall", func(t *testing.T) {
		s.ServicePath = t.TempDir() + "/svc"
		_ = os.WriteFile(s.ServicePath, []byte(""), 0o644)

		s.OsRemove = os.Remove
		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}

		if err := s.Uninstall(context.Background()); err != nil {
			t.Errorf("Uninstall failed: %v", err)
		}

		// Remove fail
		s.OsRemove = func(name string) error { return errors.New("remove fail") }
		if err := s.Uninstall(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to remove systemd service file") {
			t.Errorf("expected remove error, got %v", err)
		}
		s.OsRemove = os.Remove

		// Reload fail
		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			if len(arg) > 0 && arg[0] == "daemon-reload" {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if err := s.Uninstall(context.Background()); err == nil {
			t.Error("expected error on reload failure")
		}
	})

	t.Run("StartStop", func(t *testing.T) {
		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}
		_ = s.Start(context.Background())
		_ = s.Stop(context.Background())

		s.ExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("false")
		}
		if err := s.Start(context.Background()); err == nil {
			t.Error("expected error on start fail")
		}
		if err := s.Stop(context.Background()); err == nil {
			t.Error("expected error on stop fail")
		}
	})

	t.Run("RegisterDefaultServices", func(t *testing.T) {
		reg := NewRegistry()
		RegisterDefaultServices(reg)
		if reg.Get("systemd") == nil {
			t.Error("systemd was not registered via RegisterDefaultServices")
		}
	})
}

func TestSystemd_IsInstalled(t *testing.T) {
	s := NewSystemd().(*Systemd)

	t.Run("Installed", func(t *testing.T) {
		tmp := t.TempDir() + "/svc"
		_ = os.WriteFile(tmp, []byte(""), 0o644)
		s.ServicePath = tmp
		installed, err := s.IsInstalled(context.Background())
		if err != nil || !installed {
			t.Errorf("expected installed=true, got %v, %v", installed, err)
		}
	})

	t.Run("NotInstalled", func(t *testing.T) {
		s.ServicePath = "/non/existent/path"
		installed, err := s.IsInstalled(context.Background())
		if err != nil || installed {
			t.Errorf("expected installed=false, got %v, %v", installed, err)
		}
	})
}

func TestSystemd_CheckPermissions(t *testing.T) {
	s := NewSystemd().(*Systemd)

	t.Run("Root", func(t *testing.T) {
		s.OsGetuid = func() int { return 0 }
		if err := s.CheckPermissions(context.Background()); err != nil {
			t.Errorf("expected no error for root, got %v", err)
		}
	})

	t.Run("NonRoot", func(t *testing.T) {
		s.OsGetuid = func() int { return 1000 }
		if err := s.CheckPermissions(context.Background()); err == nil {
			t.Error("expected error for non-root, got nil")
		}
	})
}

func TestSystemd_Install_AbsError(t *testing.T) {
	// Break os.Getwd()
	originalWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWD) }()

	temp := t.TempDir()
	_ = os.Chdir(temp)
	_ = os.RemoveAll(temp)

	s := NewSystemd().(*Systemd)
	err := s.Install(context.Background(), "cfg.yaml")
	if err == nil {
		t.Error("expected error from filepath.Abs, got nil")
	}
}
