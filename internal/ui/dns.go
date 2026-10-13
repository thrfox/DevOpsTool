package ui

import (
	"devopstool/internal/network"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"strings"
)

type dnsChooser struct {
	Select    *widget.Select
	Addresses *widget.Entry
	box       fyne.CanvasObject
	optional  bool
}

func newDNSChooser(allowEmpty ...bool) *dnsChooser {
	c := &dnsChooser{Addresses: widget.NewEntry(), optional: len(allowEmpty) > 0 && allowEmpty[0]}
	c.Addresses.SetPlaceHolder("上游 IPv4 DNS，多个地址用逗号分隔")
	options := []string{}
	if c.optional {
		options = append(options, "留空 / 不指定 DNS")
	}
	for _, p := range network.DNSPresets {
		options = append(options, p.Name)
	}
	options = append(options, "自定义 DNS")
	c.Select = widget.NewSelect(options, func(value string) {
		if value == "留空 / 不指定 DNS" {
			c.Addresses.SetText("")
			c.Addresses.Disable()
			return
		}
		for _, p := range network.DNSPresets {
			if value == p.Name {
				c.Addresses.SetText(strings.Join(p.Servers, ", "))
				c.Addresses.Disable()
				return
			}
		}
		c.Addresses.Enable()
	})
	c.box = container.NewVBox(c.Select, c.Addresses)
	if c.optional {
		c.Select.SetSelected("留空 / 不指定 DNS")
	} else {
		c.Select.SetSelected("阿里 DNS")
	}
	return c
}
func (c *dnsChooser) SetServers(servers []string) {
	if c.optional && len(servers) == 0 {
		if c.Select.Selected == "留空 / 不指定 DNS" && c.Addresses.Text == "" {
			return
		}
		c.Select.SetSelected("留空 / 不指定 DNS")
		c.Addresses.SetText("")
		return
	}
	for _, p := range network.DNSPresets {
		if strings.Join(servers, ",") == strings.Join(p.Servers, ",") {
			if c.Select.Selected == p.Name && c.Addresses.Text == strings.Join(servers, ", ") {
				return
			}
			c.Select.SetSelected(p.Name)
			c.Addresses.SetText(strings.Join(servers, ", "))
			return
		}
	}
	value := strings.Join(servers, ", ")
	if c.Select.Selected != "自定义 DNS" {
		c.Select.SetSelected("自定义 DNS")
	}
	if c.Addresses.Text != value {
		c.Addresses.SetText(value)
	}
}
func (c *dnsChooser) Servers() ([]string, error) {
	if c.optional && strings.TrimSpace(c.Addresses.Text) == "" {
		return nil, nil
	}
	return network.ParseDNSServers(c.Addresses.Text)
}
