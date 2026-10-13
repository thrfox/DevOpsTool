package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"devopstool/internal/config"
)

type DNSPreset struct {
	Name    string
	Servers []string
}

var DNSPresets = []DNSPreset{
	{Name: "阿里 DNS", Servers: []string{"223.5.5.5", "223.6.6.6"}},
	{Name: "腾讯 DNS", Servers: []string{"119.29.29.29"}},
	{Name: "谷歌 DNS", Servers: []string{"8.8.8.8", "8.8.4.4"}},
}

func ParseDNSServers(text string) ([]string, error) {
	var out []string
	for _, value := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == ' ' || r == '\n' }) {
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() {
			return nil, fmt.Errorf("请输入有效的 IPv4 DNS 地址：%s", value)
		}
		out = append(out, ip.String())
	}
	if len(out) == 0 || len(out) > 8 {
		return nil, fmt.Errorf("请指定 1～8 个上游 DNS 地址")
	}
	return out, nil
}

func Prefix(text string) (int, error) {
	text = strings.TrimSpace(text)
	if n, err := strconv.Atoi(text); err == nil {
		if n >= 1 && n <= 32 {
			return n, nil
		}
		return 0, fmt.Errorf("前缀长度必须为 1～32")
	}
	ip := net.ParseIP(text)
	if ip == nil || ip.To4() == nil {
		return 0, fmt.Errorf("请输入前缀长度或子网掩码，例如 24 或 255.255.255.0")
	}
	n, bits := net.IPMask(ip.To4()).Size()
	if bits != 32 || n < 1 {
		return 0, fmt.Errorf("子网掩码必须为连续的二进制 1")
	}
	return n, nil
}

type IPv4Address struct {
	IP     string
	Prefix int
	Origin string
}
type CurrentSettings struct {
	Profile   config.Profile
	Addresses []IPv4Address
	Gateways  []string
	DNSAuto   bool
}

func ProfilesForAdapter(profiles []config.Profile, a Adapter) []config.Profile {
	var out []config.Profile
	for _, p := range profiles {
		if p.InterfaceID != "" && strings.EqualFold(strings.Trim(p.InterfaceID, "{}"), strings.Trim(a.ID, "{}")) {
			out = append(out, p)
		}
	}
	return out
}
