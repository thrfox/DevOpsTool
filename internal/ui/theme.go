package ui

import (
	_ "embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

//go:embed assets/NotoSansSC.ttf
var chineseFont []byte

type desktopTheme struct{ font fyne.Resource }

func NewTheme() fyne.Theme {
	return &desktopTheme{font: fyne.NewStaticResource("NotoSansSC.ttf", chineseFont)}
}
func (t *desktopTheme) Font(style fyne.TextStyle) fyne.Resource { return t.font }
func (t *desktopTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 42, G: 144, B: 223, A: 255}
	case theme.ColorNameBackground:
		return color.NRGBA{R: 19, G: 25, B: 36, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 228, G: 235, B: 244, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 27, G: 35, B: 49, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 34, G: 45, B: 63, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 119, G: 134, B: 153, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 129, G: 143, B: 162, A: 255}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 27, G: 35, B: 49, A: 255}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}
func (t *desktopTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (t *desktopTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 14
	}
	if name == theme.SizeNamePadding {
		return 8
	}
	return theme.DefaultTheme().Size(name)
}
