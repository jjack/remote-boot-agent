//go:build windows

package host

import (
	"errors"
	"net"
	"testing"
)

func TestGetFQDN_Windows(t *testing.T) {
	oldLookupCNAME := NetLookupCNAME
	oldGetAdapterDNSSuffix := getAdapterDNSSuffix
	defer func() {
		NetLookupCNAME = oldLookupCNAME
		getAdapterDNSSuffix = oldGetAdapterDNSSuffix
	}()

	mockIface := &net.Interface{Index: 1}

	t.Run("with DNS suffix", func(t *testing.T) {
		getAdapterDNSSuffix = func(ifIndex uint32) string {
			if ifIndex == 1 {
				return "corp.lan"
			}
			return ""
		}
		fqdn := GetFQDN("host", mockIface)
		if fqdn != "host.corp.lan" {
			t.Errorf("expected host.corp.lan, got %s", fqdn)
		}
	})

	t.Run("without DNS suffix, with CNAME", func(t *testing.T) {
		getAdapterDNSSuffix = func(ifIndex uint32) string {
			return ""
		}
		NetLookupCNAME = func(name string) (string, error) {
			return "host.external.com.", nil
		}
		fqdn := GetFQDN("host", mockIface)
		if fqdn != "host.external.com" {
			t.Errorf("expected host.external.com, got %s", fqdn)
		}
	})

	t.Run("fallback to hostname", func(t *testing.T) {
		getAdapterDNSSuffix = func(ifIndex uint32) string {
			return ""
		}
		NetLookupCNAME = func(name string) (string, error) {
			return "", errors.New("failed")
		}
		fqdn := GetFQDN("host", nil)
		if fqdn != "host" {
			t.Errorf("expected host, got %s", fqdn)
		}
	})
}
