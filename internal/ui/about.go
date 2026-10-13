package ui

import (
	"devopstool/internal/buildinfo"
	_ "embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

//go:embed assets/THIRD_PARTY_LICENSES.txt
var thirdPartyLicenses string

func (d *Desktop) about() {
	licenses := widget.NewButton("查看第三方许可证", func() {
		text := editor("")
		text.SetText(thirdPartyLicenses)
		view := dialog.NewCustom("第三方许可证与版权声明", "关闭", container.NewBorder(nil, widget.NewButton("复制", func() { d.Window.Clipboard().SetContent(thirdPartyLicenses) }), nil, nil, text), d.Window)
		view.Resize(fyne.NewSize(900, 650))
		view.Show()
	})
	view := dialog.NewCustom("关于 DevOpsTool", "关闭", container.NewVBox(note("开发运维工具箱 v"+buildinfo.Version+"\nGo + Fyne 跨平台桌面程序\n图片、文本及配置在本地处理。\n网络调试按用户输入的目标发起请求。"), licenses), d.Window)
	view.Show()
}
