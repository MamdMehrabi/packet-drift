package xray

import (
	"net"
	"sort"
)

func LANAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	addresses := make([]string, 0)
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		interfaceAddresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, interfaceAddress := range interfaceAddresses {
			var ip net.IP
			switch value := interfaceAddress.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			ip = ip.To4()
			if ip == nil || !ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			addresses = append(addresses, ip.String())
		}
	}
	sort.Strings(addresses)
	return uniqueStrings(addresses)
}

func IsLANAddress(address string) bool {
	host, _, err := privateLANAddress(address)
	if err != nil {
		return false
	}
	for _, candidate := range LANAddresses() {
		if host == candidate {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	unique := values[:1]
	for _, value := range values[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}
