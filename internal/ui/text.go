package ui

import (
	"devopstool/internal/tools"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"strings"
)

func (d *Desktop) textPage() fyne.CanvasObject {
	input := editor("粘贴 JSON、SQL、XML 或待匹配文本")
	output := editor("结果将在这里显示")
	mode := widget.NewSelect([]string{"JSON 格式化", "JSON 压缩", "SQL 格式化", "XML 格式化", "JSONPath 测试", "正则表达式测试", "XPath 测试"}, nil)
	expression := widget.NewEntry()
	expression.SetPlaceHolder("表达式，例如 $.items[*].name、//item/title、(?P<name>\\w+)")
	expression.Disable()
	repair := widget.NewCheck("尝试纠错（保留原文）", nil)
	mode.OnChanged = func(value string) {
		if value == "JSONPath 测试" || value == "正则表达式测试" || value == "XPath 测试" {
			expression.Enable()
			repair.Disable()
		} else {
			expression.Disable()
			repair.Enable()
		}
	}
	mode.SetSelected("JSON 格式化")
	input.SetText("{\"name\":\"开发运维工具箱\",\"items\":[{\"name\":\"网络调试\"},{\"name\":\"图片工具\"}]}")
	status := note("正则使用 Go RE2 语法；XPath 初版支持节点选择；SQL 按词法排版，不进行语法校验。")
	run := primary("执行", func() {
		text, expr, selected, tryRepair := input.Text, expression.Text, mode.Selected, repair.Checked
		d.perform(func() (func(), error) {
			var result string
			var err error
			var changes []string
			switch selected {
			case "JSON 格式化", "JSON 压缩", "SQL 格式化", "XML 格式化":
				result, changes, err = tools.FormatWithRepair(selected, text, tryRepair)
			case "JSONPath 测试":
				result, err = tools.TestJSONPath(text, expr)
			case "正则表达式测试":
				result, err = tools.TestRegex(text, expr)
			case "XPath 测试":
				result, err = tools.TestXPath(text, expr)
			}
			if err != nil {
				return nil, err
			}
			return func() {
				output.SetText(result)
				if len(changes) > 0 {
					status.SetText("已修正：" + strings.Join(changes, "；"))
				} else {
					status.SetText("处理完成；没有改写输入原文。")
				}
			}, nil
		})
	})
	top := container.NewVBox(container.NewBorder(nil, nil, mode, run, expression), repair, status)
	left := container.NewBorder(container.NewHBox(widget.NewLabel("输入"), widget.NewButton("打开文件", func() { d.openText(input) })), d.textActions(input, "input.txt"), nil, nil, input)
	right := container.NewBorder(widget.NewLabel("输出"), d.textActions(output, "result.txt"), nil, nil, output)
	return container.NewBorder(top, nil, nil, nil, container.NewHSplit(left, right))
}
