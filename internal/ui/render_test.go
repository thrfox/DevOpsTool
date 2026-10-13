package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
)

func TestDesktopRendering(t *testing.T) {
	d := fixtureDesktop(t)
	d.Window.Resize(fyne.NewSize(1180, 800))
	if d.Window.Content().MinSize().Width > 1180 || d.Window.Content().MinSize().Height > 800 {
		t.Fatalf("默认窗口无法容纳界面：%v", d.Window.Content().MinSize())
	}
	if os.Getenv("DEVOPSTOOL_RENDER") != "1" {
		return
	}
	dir := filepath.Join("..", "..", "dist", "screenshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, page := range []struct {
		index int
		name  string
	}{{4, "desktop"}, {0, "network-config"}, {2, "network-debug"}, {1, "base64"}, {3, "qrcode"}, {5, "compression"}} {
		d.selectPage(page.index)
		waitUI(t, func() bool { return d.pendingTasks == 0 })
		if size := d.Window.Content().MinSize(); size.Width > 1180 || size.Height > 800 {
			t.Errorf("页面 %s 无法容纳默认窗口：%v", page.name, size)
		}
		f, err := os.Create(filepath.Join(dir, page.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, d.Window.Canvas().Capture()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}
