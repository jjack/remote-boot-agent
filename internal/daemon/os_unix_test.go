//go:build !windows

package daemon

import (
	"testing"
)

func TestGetShutdownCommand(t *testing.T) {
	cmd := getShutdownCommand()
	if cmd.Path == "" {
		t.Error("expected non-empty command path")
	}
	if cmd.Args[0] != "poweroff" {
		t.Errorf("expected poweroff command, got %s", cmd.Args[0])
	}
}
