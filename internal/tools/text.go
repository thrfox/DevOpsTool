package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/beevik/etree"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

const MaxTextBytes = 4 << 20

func checkText(text string) error {
	if len(text) > MaxTextBytes {
		return fmt.Errorf("文本超过 4 MiB 限制")
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("请输入内容")
	}
	return nil
}

func FormatJSON(text string, compact bool) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	var out bytes.Buffer
	var err error
	if compact {
		err = json.Compact(&out, []byte(text))
	} else {
		err = json.Indent(&out, []byte(text), "", "  ")
	}
	if err != nil {
		return "", fmt.Errorf("JSON 格式错误：%w", err)
	}
	return out.String(), nil
}

func FormatXML(text string) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	doc := etree.NewDocument()
	doc.ReadSettings.PreserveCData = true
	if err := doc.ReadFromString(text); err != nil {
		return "", fmt.Errorf("XML 格式错误：%w", err)
	}
	if doc.Root() == nil {
		return "", fmt.Errorf("XML 缺少根元素")
	}
	doc.IndentWithSettings(&etree.IndentSettings{Spaces: 2, PreserveLeafWhitespace: true})
	return doc.WriteToString()
}

func TestJSONPath(text, expr string) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	if !json.Valid([]byte(text)) {
		return "", fmt.Errorf("输入不是有效 JSON")
	}
	data, err := oj.ParseString(text)
	if err != nil {
		return "", err
	}
	path, err := jp.ParseString(expr)
	if err != nil {
		return "", fmt.Errorf("JSONPath 错误：%w", err)
	}
	results := path.Get(data)
	if results == nil {
		results = []any{}
	}
	b, err := json.MarshalIndent(results, "", "  ")
	return string(b), err
}

func TestXPath(text, expr string) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	doc, err := xmlquery.Parse(strings.NewReader(text))
	if err != nil {
		return "", fmt.Errorf("XML 格式错误：%w", err)
	}
	nodes, err := xmlquery.QueryAll(doc, expr)
	if err != nil {
		return "", fmt.Errorf("XPath 错误：%w", err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "匹配 %d 个节点\n", len(nodes))
	for i, node := range nodes {
		if i >= 1000 {
			out.WriteString("\n仅显示前 1000 个节点")
			break
		}
		fmt.Fprintf(&out, "\n[%d] %s\n", i+1, node.OutputXML(true))
	}
	return out.String(), nil
}

func TestRegex(text, expr string) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	pattern, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("正则表达式错误：%w", err)
	}
	indices := pattern.FindAllStringSubmatchIndex(text, 1001)
	var out strings.Builder
	fmt.Fprintf(&out, "匹配 %d 项（位置为 UTF-8 字节偏移）\n", min(len(indices), 1000))
	for i, match := range indices {
		if i >= 1000 {
			out.WriteString("\n仅显示前 1000 项")
			break
		}
		fmt.Fprintf(&out, "\n[%d] [%d, %d) %s\n", i+1, match[0], match[1], text[match[0]:match[1]])
		for j := 2; j < len(match); j += 2 {
			value := "（未匹配）"
			if match[j] >= 0 {
				value = text[match[j]:match[j+1]]
			}
			name := pattern.SubexpNames()[j/2]
			fmt.Fprintf(&out, "  分组 %d %s：%s\n", j/2, name, value)
		}
	}
	return out.String(), nil
}
