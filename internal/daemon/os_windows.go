//go:build windows

package daemon

import (
	"os/exec"
)

func getShutdownCommand() *exec.Cmd {
	return exec.Command("shutdown", "/s", "/t", "0")
}
