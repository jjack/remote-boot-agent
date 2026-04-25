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
