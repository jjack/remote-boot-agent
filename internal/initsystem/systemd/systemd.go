package systemd

import (
	"os"

	"github.com/jjack/remote-boot-agent/internal/initsystem"
)

const SYSTEMD_PATH = "/run/systemd/system"

type SystemdPlugin struct {
	// configuration fields
}

func init() {
	initsystem.Register("systemd", &SystemdPlugin{})
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
