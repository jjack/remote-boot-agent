//go:build windows

package daemon

import (
	"os/exec"
)

func shutdownSystem() error {
	return exec.Command("shutdown", "/s", "/t", "0").Run()
}
