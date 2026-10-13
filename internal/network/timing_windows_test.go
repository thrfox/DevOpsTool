package network

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeReadTimingAndCompatibility(t *testing.T) {
	if os.Getenv("DEVOPSTOOL_NETWORK_SMOKE") != "1" {
		t.Skip("按需执行只读性能对比")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	oldList, err := runPS(ctx, listAdaptersPS, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldListMS := float64(time.Since(start).Microseconds()) / 1000
	var previous []Adapter
	if err := json.Unmarshal(oldList, &previous); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	adapters, err := ConfigAdapters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	newListMS := float64(time.Since(start).Microseconds()) / 1000
	var chosen Adapter
	for _, a := range adapters {
		if !a.Virtual && len(a.Addresses) > 0 {
			chosen = a
			break
		}
	}
	if chosen.Index == 0 {
		t.Skip("没有可比较的物理 IPv4 网卡")
	}
	start = time.Now()
	oldData, err := runPS(ctx, snapshotPS+`Assert-Adapter $p.Index $p.ID; Get-Snapshot $p.Index | ConvertTo-Json -Depth 8 -Compress`, chosen)
	if err != nil {
		t.Fatal(err)
	}
	oldReadMS := float64(time.Since(start).Microseconds()) / 1000
	old, err := settingsFromSnapshot(oldData, chosen)
	if err != nil {
		t.Fatal(err)
	}
	var total time.Duration
	minimum := time.Hour
	maximum := time.Duration(0)
	for i := 0; i < 20; i++ {
		start = time.Now()
		got, err := CurrentAdapterSettings(ctx, chosen)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if got.Profile.DHCP != old.Profile.DHCP || got.Profile.Prefix != old.Profile.Prefix || got.Profile.IP != old.Profile.IP || joinServers(got.Profile.DNS) != joinServers(old.Profile.DNS) || got.Profile.Gateway != old.Profile.Gateway {
			t.Fatalf("原生查询与原只读查询不一致：%+v vs %+v", got, old)
		}
		total += elapsed
		minimum = min(minimum, elapsed)
		maximum = max(maximum, elapsed)
	}
	report := map[string]any{"old_list_ms": oldListMS, "native_list_ms": newListMS, "old_current_read_ms": oldReadMS, "native_current_read_average_ms": float64(total.Microseconds()) / 20000, "native_current_read_min_ms": float64(minimum.Microseconds()) / 1000, "native_current_read_max_ms": float64(maximum.Microseconds()) / 1000, "samples": 20, "native_adapters": len(adapters), "previous_adapters": len(previous), "network_modified": false}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Log(string(b))
	if err := os.MkdirAll(filepath.Join("..", "..", "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("..", "..", "dist", "network-timing.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
}
