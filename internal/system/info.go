package system

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	osHostname    = os.Hostname
	netInterfaces = net.Interfaces
	getAddrs      = func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	}
)

var (
	ErrListInterfaces       = errors.New("failed to list network interfaces")
	ErrNoSuitableInterfaces = errors.New("no suitable interfaces found")
	ErrDetectHostname       = errors.New("failed to detect hostname")
)

type InterfaceInfo struct {
	Name        string
	MAC         string
	Label       string
	IPs         []string
	IPBroadcast map[string]string
}

// GetIPAddrs returns all addresses for a given interface as strings.
func GetIPAddrs(iface net.Interface) []string {
	addrs, err := getAddrs(iface)
	if err != nil {
		return nil
	}
	var ipAddrs []string
	for _, addr := range addrs {
		ipAddrs = append(ipAddrs, addr.String())
	}
	return ipAddrs
}

// isWOLCapableInterface checks if the given network interface is suitable for WOL (has a MAC address, is up, is not loopback, and is not virtual).
func isWOLCapableInterface(inf net.Interface) bool {
	if len(inf.HardwareAddr) == 0 {
		return false
	}

	if inf.Flags&net.FlagUp == 0 {
		return false
	}

	if inf.Flags&net.FlagLoopback != 0 {
		return false
	}

	virtualInterfaces := []string{"veth", "docker", "br-", "virbr", "vmnet", "vboxnet"}
	for _, prefix := range virtualInterfaces {
		if strings.HasPrefix(inf.Name, prefix) {
			return false
		}
	}

	path := fmt.Sprintf("/sys/class/net/%s/device", inf.Name)
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// GetInterfaceOptions returns a slice of label/value pairs for use in selection UIs.
func GetInterfaceOptions() ([]InterfaceInfo, error) {
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListInterfaces, err)
	}

	var options []InterfaceInfo
	for _, inf := range interfaces {
		if !isWOLCapableInterface(inf) {
			continue
		}

		macStr := inf.HardwareAddr.String()

		var rawIPs []string
		var ipList []string
		ipBroadcast := make(map[string]string)

		addrs, err := getAddrs(inf)
		if err == nil {
			for _, addr := range addrs {
				rawIPs = append(rawIPs, addr.String())
				if ipnet, ok := addr.(*net.IPNet); ok {
					ipStr := ipnet.IP.String()
					ipList = append(ipList, ipStr)

					ip := ipnet.IP.To4()
					if ip != nil {
						mask := ipnet.Mask
						if len(mask) == net.IPv6len {
							mask = mask[12:]
						}
						if len(mask) == net.IPv4len {
							broadcast := net.IPv4(
								ip[0]|^mask[0],
								ip[1]|^mask[1],
								ip[2]|^mask[2],
								ip[3]|^mask[3],
							).String()
							ipBroadcast[ipStr] = broadcast
						}
					}
				}
			}
		}

		ips := strings.Join(rawIPs, ", ")
		label := fmt.Sprintf("%s (%s) [%s]", inf.Name, macStr, ips)
		options = append(options, InterfaceInfo{
			Name:        inf.Name,
			MAC:         macStr,
			Label:       label,
			IPs:         ipList,
			IPBroadcast: ipBroadcast,
		})
	}

	if len(options) == 0 {
		return nil, ErrNoSuitableInterfaces
	}

	return options, nil
}

func DetectHostname() (string, error) {
	hostname, err := osHostname()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDetectHostname, err)
	}
	return hostname, nil
}
