//go:build linux

package host

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func isPhysicalInterface(inf net.Interface) bool {
	virtualInterfaces := []string{"veth", "docker", "br-", "virbr", "vmnet", "vboxnet"}
	for _, prefix := range virtualInterfaces {
		if strings.HasPrefix(inf.Name, prefix) {
			return false
		}
	}

	path := fmt.Sprintf("/sys/class/net/%s/device", inf.Name)
	_, err := osStat(path)
	return !os.IsNotExist(err)
}

func Platform() string {
	return "linux"
}

// GetFQDN attempts to resolve the Fully Qualified Domain Name for a given hostname.
func GetFQDN(hostname string, _ *net.Interface) string {
	if cname, err := netLookupCNAME(hostname); err == nil && cname != "" {
		return strings.TrimSuffix(cname, ".")
	}
	return hostname
}
