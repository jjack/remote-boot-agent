package host

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

var (
	ErrListInterfaces       = errors.New("failed to list network interfaces")
	ErrNoSuitableInterfaces = errors.New("no suitable interfaces found")
	ErrDetectHostname       = errors.New("failed to detect hostname")
)

// Host handles system information discovery.
type Host struct {
	OsHostname     func() (string, error)
	NetInterfaces  func() ([]net.Interface, error)
	NetLookupCNAME func(name string) (string, error)
	GetAddrs       func(iface net.Interface) ([]net.Addr, error)
	OsStat         func(name string) (os.FileInfo, error)
}

func New() *Host {
	return &Host{
		OsHostname:     os.Hostname,
		NetInterfaces:  net.Interfaces,
		NetLookupCNAME: net.LookupCNAME,
		GetAddrs: func(iface net.Interface) ([]net.Addr, error) {
			return iface.Addrs()
		},
		OsStat: os.Stat,
	}
}

// DetectHostname returns the system hostname.
func (h *Host) DetectHostname() (string, error) {
	hostname, err := h.OsHostname()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDetectHostname, err)
	}
	return hostname, nil
}

// GetWOLInterfaces returns a slice of net.Interface that are capable of Wake-on-LAN.
func (h *Host) GetWOLInterfaces() ([]net.Interface, error) {
	interfaces, err := h.NetInterfaces()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListInterfaces, err)
	}

	var wolIfaces []net.Interface
	for _, inf := range interfaces {
		slog.Debug("Checking to see if interface is suitable for WOL", "name", inf.Name)
		if h.isWOLCapableInterface(inf) {
			slog.Debug("Interface is suitable for WOL", "name", inf.Name)
			wolIfaces = append(wolIfaces, inf)
		}
	}

	if len(wolIfaces) == 0 {
		return nil, ErrNoSuitableInterfaces
	}

	return wolIfaces, nil
}

// GetIPInfo returns a list of IPv4 addresses and a map of those addresses to their computed broadcast address.
func (h *Host) GetIPInfo(inf net.Interface) ([]string, map[string]string) {
	var ips []string
	broadcasts := make(map[string]string)

	addrs, err := h.GetAddrs(inf)
	if err != nil {
		return ips, broadcasts
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ipv4 := ipnet.IP.To4()
			if ipv4 == nil {
				continue // Skip IPv6
			}

			ips = append(ips, ipv4.String())

			if broadcast := getLastIP(ipnet); broadcast != nil {
				broadcasts[ipv4.String()] = broadcast.String()
			}
		}
	}
	return ips, broadcasts
}

func getLastIP(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP.To4()
	if ip == nil {
		return nil
	}

	mask := ipnet.Mask
	if len(mask) == 16 {
		// If it's a 16-byte mask for an IPv4 address, the mask is in the last 4 bytes.
		mask = mask[12:]
	}

	if len(mask) != 4 {
		return nil
	}

	last := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		last[i] = ip[i] | ^mask[i]
	}
	slog.Debug("Computed broadcast address", "broadcast", last.String(), "ip", ip.String(), "mask", mask.String())
	return last
}

// isWOLCapableInterface checks if the given network interface is suitable for WOL (has a MAC address, is up, is not loopback, and is not virtual).
func (h *Host) isWOLCapableInterface(inf net.Interface) bool {
	if len(inf.HardwareAddr) == 0 {
		slog.Debug("Interface has no MAC address (skipping)", "name", inf.Name)
		return false
	}

	if inf.Flags&net.FlagUp == 0 {
		slog.Debug("Interface is not up (skipping)", "name", inf.Name)
		return false
	}

	if inf.Flags&net.FlagLoopback != 0 {
		slog.Debug("Interface is loopback (skipping)", "name", inf.Name)
		return false
	}

	return h.isPhysicalInterface(inf)
}
