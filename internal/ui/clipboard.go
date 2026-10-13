package ui

import (
	"bytes"
	"context"
	"devopstool/internal/tools"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.design/x/clipboard"
	"image"
	"io"
	"time"
)

type loadedImage struct {
	Data         []byte
	Name, Format string
	Image        image.Image
}

func imagePayload(r io.Reader, name string) (loadedImage, error) {
	b, err := tools.ReadImage(r)
	if err != nil {
		return loadedImage{}, err
	}
	img, format, err := tools.DecodeImage(b)
	return loadedImage{Data: b, Name: name, Format: format, Image: img}, err
}
func readClipboardImage() ([]byte, error) {
	if err := clipboard.Init(); err != nil {
		return nil, fmt.Errorf("当前环境无法读取图片剪贴板：%w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := clipboard.Read(ctx, clipboard.FmtImage)
	if err != nil {
		return nil, fmt.Errorf("剪贴板图片读取失败，请复制截图或图片内容：%w", err)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("剪贴板中没有图片，请先复制截图或图片内容")
	}
	return b, nil
}
func (d *Desktop) pasteImage(callback func(loadedImage)) {
	d.perform(func() (func(), error) {
		b, err := d.imageClipboardReader()
		if err != nil {
			return nil, err
		}
		img, err := imagePayload(bytes.NewReader(b), "剪贴板图片")
		if err != nil {
			return nil, err
		}
		return func() { callback(img) }, nil
	})
}

// 图片区域独立处理粘贴快捷键，文本编辑框继续使用正常的文本粘贴。
type imagePasteArea struct {
	widget.BaseWidget
	Image   *canvas.Image
	hint    *widget.Label
	onPaste func()
}

func newImagePasteArea(paste func()) *imagePasteArea {
	p := &imagePasteArea{Image: preview(), onPaste: paste, hint: note("点击此区域后按 Ctrl+V / ⌘V\n或点击“粘贴图片”\n支持截图与图片内容")}
	p.hint.Alignment = fyne.TextAlignCenter
	p.hint.Wrapping = fyne.TextWrapOff
	p.ExtendBaseWidget(p)
	return p
}
func (p *imagePasteArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(p.Image, container.NewCenter(p.hint)))
}
func (p *imagePasteArea) SetImage(img image.Image) {
	p.Image.Image = img
	p.Image.Resource = nil
	p.Image.Refresh()
	if img == nil {
		p.hint.Show()
	} else {
		p.hint.Hide()
	}
}
func (p *imagePasteArea) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(p); c != nil {
		c.Focus(p)
	}
}
func (p *imagePasteArea) FocusGained()            { p.Refresh() }
func (p *imagePasteArea) FocusLost()              { p.Refresh() }
func (p *imagePasteArea) TypedRune(rune)          {}
func (p *imagePasteArea) TypedKey(*fyne.KeyEvent) {}
func (p *imagePasteArea) TypedShortcut(s fyne.Shortcut) {
	if _, ok := s.(*fyne.ShortcutPaste); ok && p.onPaste != nil {
		p.onPaste()
	}
}
