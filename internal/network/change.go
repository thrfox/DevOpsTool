package network

import (
	"devopstool/internal/config"
	"slices"
)

type SettingsChanges struct {
	Address bool
	DNS     bool
}

func ProfileChanges(p config.Profile, current CurrentSettings) SettingsChanges {
	changes := SettingsChanges{Address: p.DHCP != current.Profile.DHCP}
	if !p.DHCP {
		changes.Address = changes.Address || len(current.Addresses) != 1 || current.Profile.IP != p.IP || current.Profile.Prefix != p.Prefix
		if p.Gateway == "" {
			changes.Address = changes.Address || len(current.Gateways) > 0
		} else {
			changes.Address = changes.Address || len(current.Gateways) != 1 || current.Gateways[0] != p.Gateway
		}
	}
	if len(p.DNS) == 0 {
		if p.DHCP {
			changes.DNS = !current.DNSAuto
		} else {
			changes.DNS = current.DNSAuto || len(current.Profile.DNS) > 0
		}
	} else {
		changes.DNS = current.DNSAuto || !slices.Equal(p.DNS, current.Profile.DNS)
	}
	return changes
}
