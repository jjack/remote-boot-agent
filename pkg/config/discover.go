package config

import (
	"net"
	"os"
)

// discoverNetworkInfo tries to find the hostname and a valid MAC address
func discoverNetworkInfo() (string, string) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	var macAddr string
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Skip loopback and down interfaces
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			mac := iface.HardwareAddr.String()
			if mac != "" { // We found one
				macAddr = mac
				break
			}
		}
	}

	return hostname, macAddr
}
