package ui

import (
	"bytes"
	"context"
	"devopstool/internal/config"
	"devopstool/internal/network"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func visit(root fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	seen := map[fyne.CanvasObject]bool{}
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		if obj == nil || seen[obj] {
			return
		}
		seen[obj] = true
		fn(obj)
		switch v := obj.(type) {
		case *fyne.Container:
			for _, child := range v.Objects {
				walk(child)
			}
		case *container.Split:
			walk(v.Leading)
			walk(v.Trailing)
		case *container.Scroll:
			walk(v.Content)
		case *container.AppTabs:
			for _, item := range v.Items {
				walk(item.Content)
			}
		case *widget.Form:
			for _, item := range v.Items {
				walk(item.Widget)
			}
		}
	}
	walk(root)
}
func waitUI(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ok := false
		fyne.DoAndWait(func() { ok = check() })
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("界面后台任务未完成")
}
func fixtureDesktop(t *testing.T) *Desktop {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	a.Settings().SetTheme(NewTheme())
	d := New(a)
	d.Store = &config.Store{Path: filepath.Join(t.TempDir(), "profiles.json")}
	d.listConfigAdapters = func(context.Context) ([]network.Adapter, error) {
		return []network.Adapter{{Index: 2, Name: "设备网卡 A", ID: "a", Addresses: []string{"192.168.10.20"}}, {Index: 3, Name: "设备网卡 B", ID: "b", Addresses: []string{"192.168.20.30"}}}, nil
	}
	d.listPhysicalAdapters = d.listConfigAdapters
	d.readAdapterSettings = func(_ context.Context, a network.Adapter) (network.CurrentSettings, error) {
		ip := "192.168.10.20"
		if a.ID == "b" {
			ip = "192.168.20.30"
		}
		p := config.Profile{Name: a.Name + " 配置", Index: a.Index, InterfaceID: a.ID, InterfaceName: a.Name, DHCP: true, IP: ip, Prefix: 24, DNS: []string{"192.168.1.1"}}
		return network.CurrentSettings{Profile: p, Addresses: []network.IPv4Address{{IP: ip, Prefix: 24, Origin: "Dhcp"}}, DNSAuto: true}, nil
	}
	return d
}

func TestAutomaticAdapterReadAndProfileScoping(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 网卡界面")
	}
	d := fixtureDesktop(t)
	for _, p := range []config.Profile{{Name: "A 专用", Index: 2, InterfaceID: "a", DHCP: true}, {Name: "B 专用", Index: 3, InterfaceID: "b", DHCP: true}} {
		if _, err := d.Store.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	d.selectPage(0)
	var adapter, saved *widget.Select
	var current *widget.Label
	var ip *widget.Entry
	waitUI(t, func() bool {
		visit(d.Window.Content(), func(o fyne.CanvasObject) {
			switch v := o.(type) {
			case *widget.Select:
				if len(v.Options) > 0 && strings.Contains(v.Options[0], "网卡 A") {
					adapter = v
				}
				if len(v.Options) == 1 && v.Options[0] == "A 专用" {
					saved = v
				}
			case *widget.Label:
				if strings.Contains(v.Text, "DNS 来源：") {
					current = v
				}
			case *widget.Entry:
				if v.PlaceHolder == "192.168.10.100" {
					ip = v
				}
			}
		})
		return adapter != nil && saved != nil && current != nil && ip != nil && ip.Text == "192.168.10.20"
	})
	if !strings.Contains(current.Text, "192.168.1.1") {
		t.Fatal("自动 DNS 当前值未显示")
	}
	adapter.SetSelected("设备网卡 B · #3")
	waitUI(t, func() bool {
		return ip.Text == "192.168.20.30" && len(saved.Options) == 1 && saved.Options[0] == "B 专用"
	})
}

func TestNetworkToolsHaveSeparateOptions(t *testing.T) {
	d := fixtureDesktop(t)
	d.selectPage(2)
	var tabs *container.AppTabs
	visit(d.Window.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*container.AppTabs); ok {
			tabs = v
		}
	})
	if tabs == nil || len(tabs.Items) != 4 {
		t.Fatal("应提供四个独立网络工具")
	}
	for _, item := range tabs.Items {
		labels := []string{}
		visit(item.Content, func(o fyne.CanvasObject) {
			if v, ok := o.(*widget.Form); ok {
				for _, f := range v.Items {
					labels = append(labels, f.Text)
				}
			}
		})
		fields := strings.Join(labels, ",")
		if item.Text == "Ping" && (strings.Contains(fields, "端口") || strings.Contains(fields, "记录类型")) {
			t.Fatal("Ping 混入其他工具选项")
		}
		if item.Text == "Telnet" && (!strings.Contains(fields, "端口") || strings.Contains(fields, "发送次数")) {
			t.Fatal("Telnet 选项不正确")
		}
		if item.Text == "DNS 查询" && !strings.Contains(fields, "记录类型") {
			t.Fatal("缺少 DNS 类型")
		}
		if item.Text == "Traceroute" && !strings.Contains(fields, "最大跳数") {
			t.Fatal("缺少路由探测选项")
		}
	}
	waitUI(t, func() bool {
		done := false
		visit(d.Window.Content(), func(o fyne.CanvasObject) {
			if v, ok := o.(*widget.Select); ok && v.Selected == "设备网卡 A · #2" {
				done = true
			}
		})
		return done
	})
}

func TestImagePasteButtonAndShortcut(t *testing.T) {
	d := fixtureDesktop(t)
	img := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	img.Set(1, 1, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	d.imageClipboardReader = func() ([]byte, error) { return b.Bytes(), nil }
	for _, page := range []int{1, 5} {
		d.selectPage(page)
		var area *imagePasteArea
		var button *widget.Button
		visit(d.Window.Content(), func(o fyne.CanvasObject) {
			if v, ok := o.(*imagePasteArea); ok {
				area = v
			}
			if v, ok := o.(*widget.Button); ok && v.Text == "粘贴图片" {
				button = v
			}
		})
		if area == nil || button == nil {
			t.Fatal("缺少图片粘贴入口")
		}
		if page == 1 {
			test.Tap(button)
		} else {
			area.TypedShortcut(&fyne.ShortcutPaste{})
		}
		waitUI(t, func() bool { return area.Image.Image != nil })
		if area.Image.Image.Bounds().Dx() != 8 {
			t.Fatal("剪贴板图片未正确加载")
		}
	}
}

func TestCachedSwitchDoesNotWaitOrOverwriteEdits(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 网卡界面")
	}
	d := fixtureDesktop(t)
	d.selectPage(0)
	waitUI(t, func() bool { return d.pendingTasks == 0 })
	var adapter *widget.Select
	var ip *widget.Entry
	visit(d.Window.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Select); ok && len(v.Options) > 0 && strings.Contains(v.Options[0], "网卡 A") {
			adapter = v
		}
		if v, ok := o.(*widget.Entry); ok && v.PlaceHolder == "192.168.10.100" {
			ip = v
		}
	})
	adapter.SetSelected("设备网卡 B · #3")
	waitUI(t, func() bool { return d.pendingTasks == 0 })
	originalRead := d.readAdapterSettings
	gate := make(chan struct{})
	d.readAdapterSettings = func(ctx context.Context, a network.Adapter) (network.CurrentSettings, error) {
		select {
		case <-gate:
			return originalRead(ctx, a)
		case <-ctx.Done():
			return network.CurrentSettings{}, ctx.Err()
		}
	}
	var timings []float64
	for _, label := range []string{"设备网卡 A · #2", "设备网卡 B · #3", "设备网卡 A · #2", "设备网卡 B · #3", "设备网卡 A · #2"} {
		started := time.Now()
		adapter.SetSelected(label)
		timings = append(timings, float64(time.Since(started).Microseconds())/1000)
		expected := "192.168.10.20"
		if strings.Contains(label, "网卡 B") {
			expected = "192.168.20.30"
		}
		if adapter.Selected != label || ip.Text != expected {
			close(gate)
			t.Fatalf("切换未实际完成：选择 %q，当前 %q，IP %q，选项 %v", label, adapter.Selected, ip.Text, adapter.Options)
		}
	}
	if ip.Text != "192.168.10.20" {
		close(gate)
		t.Fatal("缓存回填不应等待后台查询")
	}
	ip.SetText("192.168.10.99")
	close(gate)
	waitUI(t, func() bool { return d.pendingTasks == 0 })
	if ip.Text != "192.168.10.99" {
		t.Fatal("后台刷新覆盖了用户刚输入的地址")
	}
	t.Logf("缓存切换回填耗时（ms）：%v", timings)
}

func TestOptionalDNSIsSeparateFromDiagnosticDNS(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	optional := newDNSChooser(true)
	servers, err := optional.Servers()
	if err != nil || len(servers) != 0 {
		t.Fatal("网卡配置应允许 DNS 留空")
	}
	optional.Select.SetSelected("自定义 DNS")
	optional.Addresses.SetText("")
	if _, err := optional.Servers(); err != nil {
		t.Fatal(err)
	}
	optional.SetServers(nil)
	if optional.Addresses.Text != "" {
		t.Fatal("空 DNS 不能被替换为公共 DNS")
	}
	strict := newDNSChooser()
	strict.Select.SetSelected("自定义 DNS")
	strict.Addresses.SetText("")
	if _, err := strict.Servers(); err == nil {
		t.Fatal("网络诊断仍需明确上游 DNS")
	}
}
