package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validProfile() Profile {
	return Profile{Name: "办公室", Note: "中文备注", Index: 2, IP: "192.168.1.10", Prefix: 24, Gateway: "192.168.1.1", DNS: []string{"1.1.1.1"}}
}
func TestStoreLifecycle(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "config", "profiles.json")}
	p, err := s.Save(validProfile())
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" {
		t.Fatal("缺少配置标识")
	}
	p.Note = "更新备注"
	if _, err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	items, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Note != "更新备注" {
		t.Fatal("更新配置应覆盖原条目")
	}
	if err := s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.List()
	if err != nil || len(items) != 0 {
		t.Fatal("删除配置失败")
	}
}

func TestCorruptedFileIsPreserved(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "profiles.json")}
	input := []byte("broken json")
	if err := os.WriteFile(s.Path, input, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(validProfile()); err == nil {
		t.Fatal("不应覆盖损坏文件")
	}
	b, _ := os.ReadFile(s.Path)
	if string(b) != string(input) {
		t.Fatal("原配置被覆盖")
	}
}

func TestValidateNetwork(t *testing.T) {
	p := validProfile()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.Gateway = "10.0.0.1"
	if p.Validate() == nil {
		t.Fatal("应拒绝子网外网关")
	}
	p = validProfile()
	p.IP = "999.1.1.1"
	if p.Validate() == nil {
		t.Fatal("应拒绝无效地址")
	}
	p = validProfile()
	p.DHCP = true
	p.IP = ""
	p.Gateway = ""
	if err := p.Validate(); err != nil {
		t.Fatal("DHCP 不需要静态地址")
	}
	p.DNS = []string{"bad-dns"}
	if p.Validate() == nil {
		t.Fatal("应拒绝无效 DNS")
	}
}
