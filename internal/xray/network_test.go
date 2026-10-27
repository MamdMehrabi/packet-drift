package xray

import (
	"net"
	"sort"
	"testing"
)

func TestLANAddressesArePrivateIPv4AndSorted(t *testing.T) {
	addresses := LANAddresses()
	if !sort.StringsAreSorted(addresses) {
		t.Fatalf("LAN addresses are not sorted: %v", addresses)
	}
	for index, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			t.Errorf("LAN address %q is not an allowed private IPv4 address", address)
		}
		if index > 0 && addresses[index-1] == address {
			t.Errorf("LAN address %q is duplicated", address)
		}
		if !IsLANAddress(net.JoinHostPort(address, "10808")) {
			t.Errorf("IsLANAddress(%q) = false for an enumerated interface address", address)
		}
	}
}
