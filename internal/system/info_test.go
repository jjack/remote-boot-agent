package system

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
)

func TestGetInterfaceOptions(t *testing.T) {
	opts, err := GetInterfaceOptions()
	if err != nil {
		if strings.Contains(err.Error(), "no suitable interfaces found") {
			t.Skip("Skipping interface test: no suitable network interface found in this environment")
		} else {
			t.Fatalf("unexpected error getting interfaces: %v", err)
		}
	}

	if len(opts) == 0 {
		t.Error("expected non-empty interface options list")
	}

	for _, opt := range opts {
		if !strings.Contains(opt.Value, ":") {
			t.Errorf("unrecognized MAC address format: %s", opt.Value)
		}
		if opt.Label == "" {
			t.Error("expected non-empty label")
		}
		if strings.Contains(opt.Label, "[[") || strings.Contains(opt.Label, "]]") {
			t.Errorf("label contains nested brackets, expected clean format: %s", opt.Label)
		}
	}
}

func TestDetectHostname(t *testing.T) {
	hostname, err := DetectHostname()
	if err != nil {
		t.Fatalf("unexpected error detecting hostname: %v", err)
	}

	if hostname == "" {
		t.Error("expected non-empty hostname")
	}

	expected, _ := os.Hostname()
	if hostname != expected {
		t.Errorf("expected hostname %s, got %s", expected, hostname)
	}
}

func TestGetIPAddrs_ValidInterface(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		t.Skip("No interfaces available for testing")
	}
	addrs := GetIPAddrs(interfaces[0])
	if addrs == nil {
		t.Error("expected GetIPAddrs to return a slice, got nil")
	}
}

func TestDetectHostname_Error(t *testing.T) {
	oldOsHostname := osHostname
	defer func() { osHostname = oldOsHostname }()

	osHostname = func() (string, error) {
		return "", errors.New("mock hostname error")
	}

	_, err := DetectHostname()
	if err == nil {
		t.Fatal("expected error detecting hostname, got nil")
	}
	if !strings.Contains(err.Error(), "mock hostname error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetInterfaceOptions_Error(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("mock interfaces error")
	}

	_, err := GetInterfaceOptions()
	if err == nil {
		t.Fatal("expected error getting interface options, got nil")
	}
	if !strings.Contains(err.Error(), "mock interfaces error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetInterfaceOptions_NoSuitable(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		// Return an interface that will be skipped (no MAC)
		return []net.Interface{
			{
				Name:         "lo",
				HardwareAddr: nil,
				Flags:        net.FlagUp | net.FlagLoopback,
			},
		}, nil
	}

	_, err := GetInterfaceOptions()
	if err == nil {
		t.Fatal("expected error for no suitable interfaces, got nil")
	}
	if !strings.Contains(err.Error(), "no suitable interfaces found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetIPAddrs_Error(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		return nil, errors.New("mock addrs error")
	}

	addrs := GetIPAddrs(net.Interface{Name: "eth0"})
	if addrs != nil {
		t.Errorf("expected GetIPAddrs to return nil on error, got %v", addrs)
	}
}

func TestGetBroadcastAddresses(t *testing.T) {
	oldNetInterfaces := netInterfaces
	oldGetAddrs := getAddrs
	defer func() {
		netInterfaces = oldNetInterfaces
		getAddrs = oldGetAddrs
	}()

	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{
				Name:         "eth0",
				HardwareAddr: mac,
				Flags:        net.FlagUp,
			},
		}, nil
	}

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		ip, ipnet, _ := net.ParseCIDR("192.168.1.50/24")
		ipnet.IP = ip
		return []net.Addr{ipnet}, nil
	}

	bcast, err := GetBroadcastAddresses("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bcast) != 1 {
		t.Fatalf("expected 1 broadcast address, got %d", len(bcast))
	}
	if bcast[0] != "192.168.1.255" {
		t.Errorf("expected 192.168.1.255, got %s", bcast[0])
	}
}

func TestGetBroadcastAddresses_Fallback(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{}, nil
	}

	bcast, err := GetBroadcastAddresses("00:11:22:33:44:55")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bcast) != 1 {
		t.Fatalf("expected 1 broadcast address, got %d", len(bcast))
	}
	if bcast[0] != "255.255.255.255" {
		t.Errorf("expected fallback 255.255.255.255, got %s", bcast[0])
	}
}

func TestGetInterfaceOptions_LabelFormat(t *testing.T) {
	oldNetInterfaces := netInterfaces
	oldGetAddrs := getAddrs
	defer func() {
		netInterfaces = oldNetInterfaces
		getAddrs = oldGetAddrs
	}()

	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{
				Name:         "eth0",
				HardwareAddr: mac,
				Flags:        net.FlagUp,
			},
		}, nil
	}

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		ip, ipnet, _ := net.ParseCIDR("192.168.1.50/24")
		ipnet.IP = ip
		return []net.Addr{ipnet}, nil
	}

	opts, err := GetInterfaceOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLabel := "eth0 (aa:bb:cc:dd:ee:ff) [192.168.1.50/24]"
	if len(opts) != 1 || opts[0].Label != expectedLabel {
		t.Errorf("expected label %q, got %q", expectedLabel, opts[0].Label)
	}
}

func TestGetBroadcastAddresses_Error(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("mock interfaces error")
	}

	_, err := GetBroadcastAddresses("aa:bb:cc:dd:ee:ff")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mock interfaces error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetBroadcastAddresses_AddrsError(t *testing.T) {
	oldNetInterfaces := netInterfaces
	oldGetAddrs := getAddrs
	defer func() {
		netInterfaces = oldNetInterfaces
		getAddrs = oldGetAddrs
	}()

	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{
				Name:         "eth0",
				HardwareAddr: mac,
				Flags:        net.FlagUp,
			},
		}, nil
	}

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		return nil, errors.New("mock addrs error")
	}

	bcast, err := GetBroadcastAddresses("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bcast) != 1 {
		t.Fatalf("expected 1 broadcast address, got %d", len(bcast))
	}
	if bcast[0] != "255.255.255.255" {
		t.Errorf("expected fallback 255.255.255.255, got %s", bcast[0])
	}
}
