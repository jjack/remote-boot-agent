//go:build windows

package host

import (
	"errors"
	"net"
	"testing"
)

func TestGetFQDN_Windows(t *testing.T) {
	h := New()
	mockIface := &net.Interface{Index: 1}

	t.Run("with DNS suffix", func(t *testing.T) {
		h.getAdapterDNSSuffix = func(ifIndex uint32) string {
			if ifIndex == 1 {
				return "corp.lan"
			}
			return ""
		}
		fqdn := h.GetFQDN("host", mockIface)
		if fqdn != "host.corp.lan" {
			t.Errorf("expected host.corp.lan, got %s", fqdn)
		}
	})

	t.Run("without DNS suffix, with CNAME", func(t *testing.T) {
		h.getAdapterDNSSuffix = func(ifIndex uint32) string {
			return ""
		}
		h.NetLookupCNAME = func(name string) (string, error) {
			return "host.external.com.", nil
		}
		fqdn := h.GetFQDN("host", mockIface)
		if fqdn != "host.external.com" {
			t.Errorf("expected host.external.com, got %s", fqdn)
		}
	})

	t.Run("fallback to hostname", func(t *testing.T) {
		h.getAdapterDNSSuffix = func(ifIndex uint32) string {
			return ""
		}
		h.NetLookupCNAME = func(name string) (string, error) {
			return "", errors.New("failed")
		}
		fqdn := h.GetFQDN("host", nil)
		if fqdn != "host" {
			t.Errorf("expected host, got %s", fqdn)
		}
	})
}
