package network

import (
	"context"
	"devopstool/internal/config"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestEmptyDNSCommandsAndDifferencePlan(t *testing.T) {
	for _, dhcp := range []bool{true, false} {
		p := config.Profile{Index: 12, DHCP: dhcp, IP: "192.0.2.10", Prefix: 24}
		commands := profileCommands(p, SettingsChanges{Address: true, DNS: true})
		if len(commands) != 2 {
			t.Fatal(commands)
		}
		dns := strings.Join(commands[1].Args, " ")
		if dhcp && !strings.Contains(dns, "source=dhcp") {
			t.Fatal("DHCP 留空应自动获取 DNS")
		}
		if !dhcp && !strings.Contains(dns, "address=none") {
			t.Fatal("静态 IP 留空应清除 DNS，不应恢复为 DHCP DNS")
		}
	}
	p := config.Profile{Index: 12, IP: "192.0.2.10", Prefix: 24}
	current := CurrentSettings{Profile: p, Addresses: []IPv4Address{{IP: p.IP, Prefix: 24}}, DNSAuto: false}
	changes := ProfileChanges(p, current)
	if changes.Address || changes.DNS {
		t.Fatal("相同配置不应重复写入")
	}
	p.DNS = []string{"223.5.5.5", "223.6.6.6"}
	changes = ProfileChanges(p, current)
	if changes.Address || !changes.DNS {
		t.Fatal("只改 DNS 时不应重新设置 IP")
	}
	if got := profileCommands(p, changes); len(got) != 2 || strings.Contains(strings.Join(got[0].Args, " "), "set address") {
		t.Fatal(got)
	}
	if unsafe.Sizeof(forwardRow{}) != 104 {
		t.Fatal("原生路由结构布局错误")
	}
}

func TestBackupPrecedesCommandsAndRollbackSurvivesCancellation(t *testing.T) {
	dir := t.TempDir()
	p := config.Profile{Index: 2, InterfaceID: "adapter-a", IP: "192.0.2.10", Prefix: 24}
	before := nativeSnapshot{Index: 2, ID: p.InterfaceID, Name: "测试网卡", Addresses: []IPv4Address{{IP: "192.0.2.5", Prefix: 24}}, DNSMode: "none"}
	ctx, cancel := context.WithCancel(context.Background())
	restored := false
	run := func(ctx context.Context, c netCommand) error {
		if _, err := os.Stat(filepath.Join(dir, adapterBackupFile(p.InterfaceID))); err != nil {
			t.Fatal("写入网络前必须已有备份")
		}
		cancel()
		return context.Canceled
	}
	restore := func(ctx context.Context, s nativeSnapshot) error {
		if ctx.Err() != nil {
			t.Fatal("恢复不能使用已取消的上下文")
		}
		if s.ID != before.ID || s.Addresses[0].IP != "192.0.2.5" {
			t.Fatal("恢复快照错误")
		}
		restored = true
		return nil
	}
	path, err := applyCommandsWithBackup(ctx, p, before, profileCommands(p, SettingsChanges{Address: true, DNS: true}), dir, run, restore)
	if !restored || path == "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("失败或取消后未恢复：%s %v", path, err)
	}
}
