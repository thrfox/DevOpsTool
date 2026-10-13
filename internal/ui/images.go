package ui

import (
	"bytes"
	"fmt"
	"strconv"

	"devopstool/internal/tools"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func preview() *canvas.Image {
	p := canvas.NewImageFromResource(nil)
	p.FillMode = canvas.ImageFillContain
	p.SetMinSize(fyne.NewSize(320, 300))
	return p
}
func setPreview(p *canvas.Image, b []byte) {
	img, _, err := tools.DecodeImage(b)
	if err == nil {
		p.Image = img
		p.Refresh()
	}
}

func (d *Desktop) base64Page() fyne.CanvasObject {
	text := editor("粘贴图片 Base64，支持 data:image/...;base64, 前缀")
	var imagePreview *imagePasteArea
	info := note("尚未选择图片，支持 PNG / JPEG")
	withPrefix := widget.NewCheck("添加 Data URI 前缀（自动匹配 PNG/JPEG）", nil)
	withPrefix.SetChecked(true)
	var imageBytes []byte
	var imageFormat string
	loaded := func(img loadedImage) {
		imageBytes = img.Data
		imageFormat = img.Format
		imagePreview.SetImage(img.Image)
		info.SetText(img.Name + " · " + fileSize(len(img.Data)))
	}
	paste := func() { d.pasteImage(loaded) }
	d.imagePasteActions[1] = paste
	imagePreview = newImagePasteArea(paste)
	selectImage := widget.NewButton("选择图片", func() { d.openImage(loaded) })
	pasteButton := widget.NewButtonWithIcon("粘贴图片", moduleIcon("clipboard"), paste)
	encode := primary("图片 → Base64", func() {
		b, prefix := append([]byte(nil), imageBytes...), withPrefix.Checked
		d.perform(func() (func(), error) {
			result, err := tools.ImageToBase64(b, prefix)
			if err != nil {
				return nil, err
			}
			return func() { text.SetText(result) }, nil
		})
	})
	decode := primary("Base64 → 图片", func() {
		value := text.Text
		d.perform(func() (func(), error) {
			b, format, err := tools.Base64ToImage(value)
			if err != nil {
				return nil, err
			}
			img, _, err := tools.DecodeImage(b)
			if err != nil {
				return nil, err
			}
			return func() {
				imageBytes = b
				imageFormat = format
				imagePreview.SetImage(img)
				info.SetText("已解码 · " + format + " · " + fileSize(len(b)))
			}, nil
		})
	})
	export := widget.NewButton("导出图片", func() {
		ext := imageFormat
		if ext == "jpeg" {
			ext = "jpg"
		}
		d.saveBytes("image."+ext, imageBytes)
	})
	left := container.NewBorder(container.NewVBox(info, withPrefix), container.NewVBox(container.NewHBox(selectImage, pasteButton), container.NewHBox(encode, export)), nil, nil, imagePreview)
	right := container.NewBorder(note("Base64 内容"), container.NewHBox(decode, d.textActions(text, "image-base64.txt")), nil, nil, text)
	return container.NewHSplit(left, right)
}

func (d *Desktop) qrPage() fyne.CanvasObject {
	text := editor("输入文本或网址")
	text.SetText("https://example.com")
	size := widget.NewSelect([]string{"256", "512", "1024", "2048"}, nil)
	size.SetSelected("512")
	level := widget.NewSelect([]string{"L · 7%", "M · 15%", "Q · 25%", "H · 30%"}, nil)
	level.SetSelectedIndex(1)
	p := preview()
	info := note("二维码在本地生成，支持中文文本")
	var data []byte
	run := primary("生成二维码", func() {
		value := text.Text
		n, _ := strconv.Atoi(size.Selected)
		l := level.SelectedIndex()
		d.perform(func() (func(), error) {
			b, err := tools.QRCode(value, n, l)
			if err != nil {
				return nil, err
			}
			return func() {
				data = b
				setPreview(p, b)
				info.SetText(fmt.Sprintf("%d × %d · PNG · %s", n, n, fileSize(len(b))))
			}, nil
		})
	})
	left := container.NewBorder(container.NewVBox(note("二维码内容"), container.NewHBox(widget.NewLabel("尺寸"), size, widget.NewLabel("纠错等级"), level)), container.NewHBox(run, widget.NewButton("导出 PNG", func() { d.saveBytes("qrcode.png", data) })), nil, nil, text)
	right := container.NewBorder(info, nil, nil, nil, p)
	return container.NewHSplit(left, right)
}

func (d *Desktop) compressPage() fyne.CanvasObject {
	var originalPreview *imagePasteArea
	resultPreview := preview()
	originalInfo, resultInfo := note("原图：未选择"), note("结果：尚未压缩")
	format := widget.NewSelect([]string{"PNG 无损优化", "JPEG"}, nil)
	format.SetSelected("PNG 无损优化")
	quality := widget.NewSlider(1, 100)
	quality.Step = 1
	quality.SetValue(80)
	qualityInfo := widget.NewLabel("JPEG 质量：80")
	quality.OnChanged = func(value float64) { qualityInfo.SetText(fmt.Sprintf("JPEG 质量：%.0f", value)) }
	width := widget.NewEntry()
	width.SetText("0")
	var original, result []byte
	var ext = "png"
	loaded := func(img loadedImage) {
		original = img.Data
		result = nil
		originalPreview.SetImage(img.Image)
		resultPreview.Image = nil
		resultPreview.Resource = nil
		resultPreview.Refresh()
		originalInfo.SetText(img.Name + " · " + fileSize(len(img.Data)))
		resultInfo.SetText("结果：尚未压缩")
	}
	paste := func() { d.pasteImage(loaded) }
	d.imagePasteActions[5] = paste
	originalPreview = newImagePasteArea(paste)
	load := widget.NewButton("选择图片", func() { d.openImage(loaded) })
	pasteButton := widget.NewButtonWithIcon("粘贴图片", moduleIcon("clipboard"), paste)
	run := primary("开始压缩", func() {
		maxWidth, err := strconv.Atoi(width.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("最大宽度必须为整数"), d.Window)
			return
		}
		b := append([]byte(nil), original...)
		q := int(quality.Value)
		f := "png"
		if format.Selected == "JPEG" {
			f = "jpeg"
		}
		d.perform(func() (func(), error) {
			out, err := tools.CompressImage(b, f, q, maxWidth)
			if err != nil {
				return nil, err
			}
			return func() {
				result = out
				ext = f
				if ext == "jpeg" {
					ext = "jpg"
				}
				setPreview(resultPreview, out)
				reduction := 100 * (1 - float64(len(out))/float64(len(b)))
				label := fmt.Sprintf("结果：%s · 减少 %.1f%%", fileSize(len(out)), reduction)
				if bytes.Equal(b, out) {
					label += "（原图已较小，保留原文件）"
				}
				resultInfo.SetText(label)
			}, nil
		})
	})
	options := container.NewGridWithColumns(2, container.NewVBox(note("输出格式"), format, qualityInfo, quality), container.NewVBox(note("最大宽度（0 = 原尺寸，缩放将改变像素）"), width, note("JPEG 输出会将透明背景合成为白色。PNG 不缩放时保留像素，但重编码可能移除元数据。")))
	images := container.NewHSplit(container.NewBorder(originalInfo, nil, nil, nil, originalPreview), container.NewBorder(resultInfo, nil, nil, nil, resultPreview))
	return container.NewBorder(options, container.NewHBox(load, pasteButton, run, widget.NewButton("导出结果", func() { d.saveBytes("compressed."+ext, result) })), nil, nil, images)
}
