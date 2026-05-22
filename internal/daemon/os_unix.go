//go:build !windows

package daemon

import (
	"os/exec"
)

func shutdownSystem() error {
	return exec.Command("poweroff").Run()
}
