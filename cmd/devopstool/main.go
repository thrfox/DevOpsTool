package main

import (
	"devopstool/internal/ui"
	"flag"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"image/png"
	"os"
	"time"
)

func main() {
	capture := flag.String("self-check-output", "", "内部验证：生成原生界面截图后退出")
	page := flag.Int("self-check-page", 4, "内部验证：选择截图页面")
	flag.Parse()
	a := app.NewWithID("cn.azukin.devopstool")
	a.Settings().SetTheme(ui.NewTheme())
	d := ui.New(a)
	if *capture != "" {
		d.ShowPage(*page)
	}
	var captureError error
	if *capture != "" {
		go func() {
			time.Sleep(2 * time.Second)
			deadline := time.Now().Add(30 * time.Second)
			for {
				done := false
				fyne.DoAndWait(func() {
					if d.HasPendingTasks() && time.Now().Before(deadline) {
						return
					}
					done = true
					f, err := os.Create(*capture)
					if err == nil {
						err = png.Encode(f, d.Window.Canvas().Capture())
						closeErr := f.Close()
						if err == nil {
							err = closeErr
						}
					}
					captureError = err
					a.Quit()
				})
				if done {
					return
				}
				time.Sleep(250 * time.Millisecond)
			}
		}()
	}
	d.ShowAndRun()
	if captureError != nil {
		os.Exit(1)
	}
}
