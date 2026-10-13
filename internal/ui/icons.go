package ui

import (
	_ "embed"
	"fmt"
	"fyne.io/fyne/v2"
)

//go:embed assets/app.svg
var applicationSVG []byte

//go:embed assets/app.png
var applicationPNG []byte

func appIcon() fyne.Resource { return fyne.NewStaticResource("devopstool.png", applicationPNG) }

// 原创统一图标：圆角线条搭配蓝青色点缀，在深色界面与小尺寸下保持清晰。
func moduleIcon(name string) fyne.Resource {
	shapes := map[string]string{
		"network":   `<rect x="3" y="3" width="8" height="6" rx="2"/><rect x="13" y="15" width="8" height="6" rx="2"/><path d="M7 9v8h6M17 15V7h-6"/><circle cx="7" cy="17" r="2" fill="#31d9b8"/>`,
		"image":     `<rect x="3" y="3" width="18" height="18" rx="4"/><circle cx="8" cy="8" r="1.5" fill="#31d9b8"/><path d="m3 17 5-5 4 4 4-7 5 7"/>`,
		"ping":      `<path d="M2 12h4l3-7 5 14 3-7h5"/><circle cx="20" cy="5" r="2" fill="#31d9b8" stroke="none"/>`,
		"terminal":  `<rect x="2" y="4" width="20" height="16" rx="4"/><path d="m6 9 4 3-4 3m8 1h4"/>`,
		"dns":       `<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18M5 6h14M5 18h14"/>`,
		"route":     `<path d="M6 19V8a3 3 0 0 1 3-3h9m-3-3 3 3-3 3M6 14h8a3 3 0 0 1 3 3v3"/><circle cx="6" cy="19" r="2" fill="#31d9b8"/><circle cx="17" cy="20" r="2" fill="#31d9b8"/>`,
		"qr":        `<rect x="3" y="3" width="6" height="6" rx="1"/><rect x="15" y="3" width="6" height="6" rx="1"/><rect x="3" y="15" width="6" height="6" rx="1"/><path d="M15 15h3v6m0-6h3m0 3v3h-3M12 3v3m0 6h3M3 12h3m6 6v3"/>`,
		"format":    `<path d="M7 3H5v7l-2 2 2 2v7h2M17 3h2v7l2 2-2 2v7h-2M10 7h4m-4 5h4m-4 5h4"/><circle cx="12" cy="12" r="1" fill="#31d9b8"/>`,
		"compress":  `<rect x="3" y="3" width="18" height="18" rx="4"/><path d="m6 6 4 4m-4 0h4V6m8 12-4-4m0 4v-4h4"/>`,
		"clipboard": `<rect x="5" y="5" width="14" height="17" rx="3"/><rect x="8" y="2" width="8" height="5" rx="2" fill="#182434"/><path d="M9 13h6m-6 4h4"/>`,
	}
	body, ok := shapes[name]
	if !ok {
		body = shapes["terminal"]
	}
	return fyne.NewStaticResource(name+".svg", []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><g fill="none" stroke="#8bd8eb" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">%s</g></svg>`, body)))
}
