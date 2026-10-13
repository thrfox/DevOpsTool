package ui

import (
	"context"
	"fmt"
	"io"

	"devopstool/internal/buildinfo"
	"devopstool/internal/config"
	"devopstool/internal/network"
	"devopstool/internal/tools"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Desktop struct {
	App                  fyne.App
	Window               fyne.Window
	Store                *config.Store
	startupError         error
	selectPage           func(int)
	listConfigAdapters   func(context.Context) ([]network.Adapter, error)
	readAdapterSettings  func(context.Context, network.Adapter) (network.CurrentSettings, error)
	listPhysicalAdapters func(context.Context) ([]network.Adapter, error)
	imageClipboardReader func() ([]byte, error)
	imagePasteActions    map[int]func()
	activeImagePaste     func()
	pendingTasks         int
}

func New(a fyne.App) *Desktop {
	w := a.NewWindow("DevOpsTool · 开发运维工具箱")
	a.SetIcon(appIcon())
	w.SetIcon(appIcon())
	d := &Desktop{App: a, Window: w, listConfigAdapters: network.ConfigAdapters, readAdapterSettings: network.CurrentAdapterSettings, listPhysicalAdapters: network.PhysicalAdapters, imageClipboardReader: readClipboardImage, imagePasteActions: make(map[int]func())}
	d.Store, d.startupError = config.NewStore()
	w.Resize(fyne.NewSize(1180, 800))
	w.CenterOnScreen()
	d.build()
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("帮助", fyne.NewMenuItem("关于 / 许可证", d.about))))
	w.Canvas().AddShortcut(&fyne.ShortcutPaste{}, func(fyne.Shortcut) {
		if d.activeImagePaste != nil {
			d.activeImagePaste()
		}
	})
	return d
}

func (d *Desktop) ShowAndRun() {
	if d.startupError != nil {
		dialog.ShowError(d.startupError, d.Window)
	}
	d.Window.ShowAndRun()
}

func (d *Desktop) build() {
	title := widget.NewLabelWithStyle("开发运维工具箱", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("本地工具 · v" + buildinfo.Version)
	pages := []struct {
		name  string
		icon  fyne.Resource
		build func() fyne.CanvasObject
	}{
		{"网络配置", moduleIcon("network"), d.networkConfig},
		{"图片 / Base64", moduleIcon("image"), d.base64Page},
		{"网络调试", moduleIcon("ping"), d.networkPage},
		{"二维码生成", moduleIcon("qr"), d.qrPage},
		{"格式化 / 测试", moduleIcon("format"), d.textPage},
		{"图片压缩", moduleIcon("compress"), d.compressPage},
	}
	body := container.NewStack()
	heading := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	logo := canvas.NewImageFromResource(appIcon())
	logo.FillMode = canvas.ImageFillContain
	brand := container.NewHBox(container.NewGridWrap(fyne.NewSize(42, 42), logo), container.NewVBox(title, subtitle))
	navigation := container.NewVBox(brand, widget.NewSeparator())
	buttons := make([]*widget.Button, len(pages))
	var cache = make(map[int]fyne.CanvasObject)
	selectPage := func(index int) {
		for i, b := range buttons {
			if i == index {
				b.Importance = widget.HighImportance
			} else {
				b.Importance = widget.MediumImportance
			}
			b.Refresh()
		}
		heading.SetText(pages[index].name)
		if cache[index] == nil {
			cache[index] = pages[index].build()
		}
		d.activeImagePaste = d.imagePasteActions[index]
		body.Objects = []fyne.CanvasObject{cache[index]}
		body.Refresh()
	}
	d.selectPage = selectPage
	for i, p := range pages {
		index := i
		buttons[i] = widget.NewButtonWithIcon(p.name, p.icon, func() { selectPage(index) })
		navigation.Add(buttons[i])
	}
	left := container.NewBorder(navigation, widget.NewLabel("Go + Fyne\nWindows / macOS / Linux"), nil, nil, layout.NewSpacer())
	leftBox := container.NewGridWrap(fyne.NewSize(220, 720), left)
	content := container.NewBorder(container.NewVBox(heading, widget.NewSeparator()), nil, nil, nil, body)
	d.Window.SetContent(container.NewPadded(container.NewBorder(nil, nil, leftBox, nil, content)))
	selectPage(4)
}

func note(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}
func editor(placeholder string) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.SetPlaceHolder(placeholder)
	e.Wrapping = fyne.TextWrapWord
	return e
}
func primary(label string, fn func()) *widget.Button {
	b := widget.NewButton(label, fn)
	b.Importance = widget.HighImportance
	return b
}

// 后台任务只处理快照数据，控件更新统一回到 Fyne 主线程。
func (d *Desktop) perform(job func() (func(), error)) {
	d.pendingTasks++
	progress := dialog.NewCustomWithoutButtons("正在处理", container.NewVBox(widget.NewProgressBarInfinite(), note("请稍候…")), d.Window)
	progress.Show()
	go func() {
		var done func()
		var err error
		func() {
			defer func() {
				if value := recover(); value != nil {
					err = fmt.Errorf("处理失败：%v", value)
				}
			}()
			done, err = job()
		}()
		fyne.Do(func() {
			defer func() { d.pendingTasks-- }()
			progress.Hide()
			if err != nil {
				dialog.ShowError(err, d.Window)
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}

func (d *Desktop) openImage(callback func(loadedImage)) {
	chooser := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		if r == nil {
			return
		}
		name := r.URI().Name()
		d.perform(func() (func(), error) {
			defer r.Close()
			img, err := imagePayload(r, name)
			if err != nil {
				return nil, err
			}
			return func() { callback(img) }, nil
		})
	}, d.Window)
	chooser.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	chooser.Resize(fyne.NewSize(880, 600))
	chooser.Show()
}

func (d *Desktop) saveBytes(name string, data []byte) {
	if len(data) == 0 {
		dialog.ShowInformation("没有结果", "请先生成或处理内容", d.Window)
		return
	}
	chooser := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		if w == nil {
			return
		}
		d.perform(func() (func(), error) {
			_, err := w.Write(data)
			closeErr := w.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			return func() { dialog.ShowInformation("保存成功", "文件已保存", d.Window) }, nil
		})
	}, d.Window)
	chooser.SetFileName(name)
	chooser.Resize(fyne.NewSize(880, 600))
	chooser.Show()
}

func (d *Desktop) textActions(e *widget.Entry, name string) fyne.CanvasObject {
	return container.NewHBox(widget.NewButtonWithIcon("复制", theme.ContentCopyIcon(), func() { d.Window.Clipboard().SetContent(e.Text) }), widget.NewButton("导出", func() { d.saveBytes(name, []byte(e.Text)) }), widget.NewButton("清空", func() { e.SetText("") }))
}

func (d *Desktop) openText(e *widget.Entry) {
	dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, d.Window)
			return
		}
		if r == nil {
			return
		}
		d.perform(func() (func(), error) {
			defer r.Close()
			b, err := io.ReadAll(io.LimitReader(r, tools.MaxTextBytes+1))
			if err != nil {
				return nil, err
			}
			if len(b) > tools.MaxTextBytes {
				return nil, fmt.Errorf("文本超过 4 MiB 限制")
			}
			return func() { e.SetText(string(b)) }, nil
		})
	}, d.Window)
}

func fileSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	}
	return fmt.Sprintf("%.2f MiB", float64(n)/(1024*1024))
}

func (d *Desktop) ShowPage(index int) bool {
	if index < 0 || index > 5 {
		return false
	}
	d.selectPage(index)
	return true
}

// 只读刷新在后台进行，不用模态弹窗阻塞网卡切换和表单编辑。
func (d *Desktop) background(job func() (func(), error), failed func(error)) {
	d.pendingTasks++
	go func() {
		var done func()
		var err error
		func() {
			defer func() {
				if value := recover(); value != nil {
					err = fmt.Errorf("后台刷新失败：%v", value)
				}
			}()
			done, err = job()
		}()
		fyne.Do(func() {
			defer func() { d.pendingTasks-- }()
			if err != nil {
				if failed != nil {
					failed(err)
				}
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}
func (d *Desktop) HasPendingTasks() bool { return d.pendingTasks > 0 }
