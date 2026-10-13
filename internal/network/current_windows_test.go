package network

import (
	"testing"
)

func TestDHCPCurrentValuesAreNotLost(t *testing.T) {
	a := Adapter{Index: 3, Name: "以太网", ID: "adapter-a"}
	data := []byte(`{"DHCP":true,"DNSAuto":true,"DNS":["192.168.10.1"],"CurrentAddresses":[{"IP":"192.168.10.20","Prefix":24,"Origin":"Dhcp"}],"Addresses":[],"Gateways":[{"NextHop":"192.168.10.1"}]}`)
	s, err := settingsFromSnapshot(data, a)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Profile.DHCP || s.Profile.IP != "192.168.10.20" || len(s.Profile.DNS) != 1 || !s.DNSAuto || s.Profile.Gateway != "192.168.10.1" {
		t.Fatalf("DHCP 地址或 DNS 未完整回填：%+v", s)
	}
	if adapterBackupFile("{ABC}") != adapterBackupFile("abc") {
		t.Fatal("同一网卡的备份标识应一致")
	}
	if adapterBackupFile("abc") == adapterBackupFile("def") {
		t.Fatal("不同网卡不能共用最近备份")
	}
}
