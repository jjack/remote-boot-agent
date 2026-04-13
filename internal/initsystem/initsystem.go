package initsystem

// InitSystem defines the interface for init system plugins.
type InitSystem interface {
	Name() string
	// Detect returns true if this init system is active on the host
	Detect() bool
	// Add more methods interacting with init systems...
}

// Registry manages the available initsystem plugins
type Registry struct {
	plugins map[string]InitSystem
}

// NewRegistry creates a new init system registry with the provided plugins
func NewRegistry(plugins ...InitSystem) *Registry {
	r := &Registry{plugins: make(map[string]InitSystem)}
	for _, p := range plugins {
		r.plugins[p.Name()] = p
	}
	return r
}

// Get returns a registered init system plugin by name.
func (r *Registry) Get(name string) (InitSystem, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// Detect iterates over all registered plugins and returns the name of the first one that detects itself.
func (r *Registry) Detect() string {
	for name, plugin := range r.plugins {
		if plugin.Detect() {
			return name
		}
	}
	return ""
}
