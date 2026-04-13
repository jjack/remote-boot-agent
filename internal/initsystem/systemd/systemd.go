package systemd

import (
	"os"
)

const SYSTEMD_PATH = "/run/systemd/system"

type SystemdPlugin struct {
	// configuration fields
}

func New() *SystemdPlugin {
	return &SystemdPlugin{}
}

func (p *SystemdPlugin) Name() string {
	return "systemd"
}

func (p *SystemdPlugin) Detect() bool {
	if _, err := os.Stat(SYSTEMD_PATH); err == nil {
		return true
	}

	return false
}
