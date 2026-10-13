package network

import (
	"devopstool/internal/config"
	"testing"
)

func TestMasksAndFixedUpstream(t *testing.T) {
	for _, text := range []string{"24", "255.255.255.0"} {
		n, err := Prefix(text)
		if err != nil || n != 24 {
			t.Fatalf("掩码解析失败：%s", text)
		}
	}
	for _, text := range []string{"255.0.255.0", "0", "33"} {
		if _, err := Prefix(text); err == nil {
			t.Fatalf("应拒绝无效掩码：%s", text)
		}
	}
	for _, preset := range DNSPresets {
		if _, err := ParseDNSServers(joinServers(preset.Servers)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ParseDNSServers(""); err == nil {
		t.Fatal("不能回退到系统 DNS")
	}
	servers, err := ParseDNSServers("192.0.2.53，192.0.2.54")
	if err != nil || len(servers) != 2 {
		t.Fatal("自定义 DNS 解析失败")
	}
}
func joinServers(values []string) string {
	s := ""
	for i, v := range values {
		if i > 0 {
			s += ","
		}
		s += v
	}
	return s
}

func TestProfilesAreScopedToOneAdapter(t *testing.T) {
	profiles := []config.Profile{{ID: "a", InterfaceID: "{ABC}", Name: "设备 A"}, {ID: "b", InterfaceID: "def", Name: "设备 B"}, {ID: "legacy", Name: "旧配置"}}
	got := ProfilesForAdapter(profiles, Adapter{ID: "abc"})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("其他网卡方案混入：%v", got)
	}
	if len(ProfilesForAdapter(profiles, Adapter{ID: "missing"})) != 0 {
		t.Fatal("不能加载其他适配器方案")
	}
}
