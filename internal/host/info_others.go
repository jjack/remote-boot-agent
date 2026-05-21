//go:build !linux && !windows

package host

import (
	"net"
	"runtime"
)

func isPhysicalInterface(inf net.Interface) bool {
	return true
}

func Platform() string {
	return runtime.GOOS
}

// GetFQDN attempts to resolve the Fully Qualified Domain Name for a given hostname.
func GetFQDN(hostname string, _ *net.Interface) string {
	return hostname
}
