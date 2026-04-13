package bootloader

import "github.com/jjack/remote-boot-agent/internal/config"

// BootOptions represents the parsed boot configuration.
type BootOptions struct {
	AvailableOSes []string
	Parameters  map[string]string
	// Add other relevant fields
}

// Bootloader defines the interface for all bootloader plugins.
type Bootloader interface {
	// Name returns the name of the bootloader plugin.
	Name() string
	// Detect returns true if this bootloader is detected as the active/available one on the system.
	Detect() bool
	// Parse parses the bootloader configuration and returns the options.
	Parse(cfg *config.Config) (*BootOptions, error)
	// Add other necessary methods (e.g., SetNextBoot)
}

// Registry to keep track of available plugins
var plugins = make(map[string]Bootloader)

// Register makes a bootloader plugin available by the provided name.
func Register(name string, plugin Bootloader) {
	if plugin == nil {
		panic("bootloader: Register plugin is nil")
	}
	if _, dup := plugins[name]; dup {
		panic("bootloader: Register called twice for plugin " + name)
	}
	plugins[name] = plugin
}

// Get returns a registered bootloader plugin by name.
func Get(name string) (Bootloader, bool) {
	p, ok := plugins[name]
	return p, ok
}

// Detect iterates over all registered plugins and returns the name of the first one that detects itself.
func Detect() string {
	for name, plugin := range plugins {
		if plugin.Detect() {
			return name
		}
	}
	return ""
}
