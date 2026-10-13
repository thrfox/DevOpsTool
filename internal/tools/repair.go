package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// 纠错只处理可明确识别的输入错误，原始输入由界面保留，不猜测缺失的数据。
func FormatWithRepair(mode, text string, enabled bool) (string, []string, error) {
	format := func(value string) (string, error) {
		switch mode {
		case "JSON 格式化":
			return FormatJSON(value, false)
		case "JSON 压缩":
			return FormatJSON(value, true)
		case "XML 格式化":
			return FormatXML(value)
		case "SQL 格式化":
			return FormatSQL(value)
		}
		return "", fmt.Errorf("不支持的格式化模式")
	}
	if err := checkText(text); err != nil {
		return "", nil, err
	}
	strict, strictErr := format(text)
	if !enabled {
		return strict, nil, strictErr
	}
	value := strings.TrimPrefix(text, "\ufeff")
	var notes []string
	if value != text {
		notes = append(notes, "移除 UTF-8 BOM")
	}
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "```") && strings.HasSuffix(trimmed, "```") {
		if i := strings.IndexByte(trimmed, '\n'); i >= 0 {
			value = strings.TrimSpace(trimmed[i+1 : len(trimmed)-3])
			notes = append(notes, "移除 Markdown 代码围栏")
		}
	}
	var repaired string
	var changes []string
	var err error
	switch mode {
	case "JSON 格式化", "JSON 压缩":
		repaired, changes, err = repairJSON(value)
	case "XML 格式化":
		repaired, changes, err = repairXML(value)
	case "SQL 格式化":
		repaired, changes, err = repairSQL(value)
	}
	if err != nil {
		return "", nil, fmt.Errorf("无法可靠纠错，原文已保留：%w", err)
	}
	notes = append(notes, changes...)
	result, err := format(repaired)
	if err != nil {
		return "", nil, fmt.Errorf("仍有无法自动修复的错误，原文已保留：%w", err)
	}
	return result, notes, nil
}

type repairToken struct {
	text string
	kind byte
}

func repairJSON(text string) (string, []string, error) {
	if json.Valid([]byte(text)) {
		return text, nil, nil
	}
	r := []rune(text)
	var tokens []repairToken
	changes := map[string]bool{}
	for i := 0; i < len(r); {
		if unicode.IsSpace(r[i]) {
			i++
			continue
		}
		if r[i] == '/' && i+1 < len(r) && (r[i+1] == '/' || r[i+1] == '*') {
			block := r[i+1] == '*'
			i += 2
			if !block {
				for i < len(r) && r[i] != '\n' {
					i++
				}
			} else {
				closed := false
				for i+1 < len(r) {
					if r[i] == '*' && r[i+1] == '/' {
						i += 2
						closed = true
						break
					}
					i++
				}
				if !closed {
					return "", nil, fmt.Errorf("JSON 注释未闭合")
				}
			}
			changes["移除 JSON 注释"] = true
			continue
		}
		if r[i] == '"' || r[i] == '\'' {
			quote := r[i]
			i++
			var out strings.Builder
			out.WriteByte('"')
			closed := false
			for i < len(r) {
				c := r[i]
				i++
				if c == quote {
					closed = true
					break
				}
				if c == '\\' {
					if i >= len(r) {
						break
					}
					next := r[i]
					i++
					if quote == '\'' && next == '\'' {
						out.WriteRune('\'')
					} else {
						out.WriteRune('\\')
						out.WriteRune(next)
					}
					continue
				}
				if c == '"' {
					out.WriteString("\\\"")
				} else if c == '\n' {
					out.WriteString("\\n")
					changes["转义字符串中的换行"] = true
				} else if c == '\r' {
					out.WriteString("\\r")
					changes["转义字符串中的换行"] = true
				} else if c == '\t' {
					out.WriteString("\\t")
					changes["转义字符串中的制表符"] = true
				} else {
					out.WriteRune(c)
				}
			}
			if !closed {
				return "", nil, fmt.Errorf("JSON 字符串未闭合，不补写可能缺失的数据")
			}
			out.WriteByte('"')
			if quote == '\'' {
				changes["将单引号字符串转换为双引号"] = true
			}
			tokens = append(tokens, repairToken{text: out.String(), kind: 's'})
			continue
		}
		if strings.ContainsRune("{}[]:,", r[i]) {
			tokens = append(tokens, repairToken{text: string(r[i]), kind: byte(r[i])})
			i++
			continue
		}
		start := i
		for i < len(r) && !unicode.IsSpace(r[i]) && !strings.ContainsRune("{}[]:,/", r[i]) {
			i++
		}
		if i == start {
			return "", nil, fmt.Errorf("JSON 包含无法识别的符号")
		}
		tokens = append(tokens, repairToken{text: string(r[start:i]), kind: 'w'})
	}
	var out strings.Builder
	for i, t := range tokens {
		if t.kind == ',' && i+1 < len(tokens) && (tokens[i+1].kind == '}' || tokens[i+1].kind == ']') {
			changes["移除末尾多余逗号"] = true
			continue
		}
		if t.kind == 'w' && i+1 < len(tokens) && tokens[i+1].kind == ':' && (i == 0 || tokens[i-1].kind == '{' || tokens[i-1].kind == ',') {
			valid := true
			for j, c := range []rune(t.text) {
				if !(unicode.IsLetter(c) || c == '_' || c == '$' || (j > 0 && unicode.IsDigit(c))) {
					valid = false
				}
			}
			if !valid {
				return "", nil, fmt.Errorf("无法判断对象键名")
			}
			b, _ := json.Marshal(t.text)
			t.text = string(b)
			changes["为未加引号的对象键添加双引号"] = true
		}
		if t.kind == 'w' {
			switch t.text {
			case "True":
				t.text = "true"
				changes["转换 Python 布尔值 / None"] = true
			case "False":
				t.text = "false"
				changes["转换 Python 布尔值 / None"] = true
			case "None":
				t.text = "null"
				changes["转换 Python 布尔值 / None"] = true
			}
		}
		out.WriteString(t.text)
		out.WriteByte(' ')
	}
	value := out.String()
	if !json.Valid([]byte(value)) {
		return "", nil, fmt.Errorf("JSON 结构仍不完整，未猜测缺失的逗号、括号或内容")
	}
	var notes []string
	for _, key := range []string{"移除 JSON 注释", "将单引号字符串转换为双引号", "为未加引号的对象键添加双引号", "移除末尾多余逗号", "转换 Python 布尔值 / None", "转义字符串中的换行", "转义字符串中的制表符"} {
		if changes[key] {
			notes = append(notes, key)
		}
	}
	return value, notes, nil
}

var entityPattern = regexp.MustCompile(`^&(?:[A-Za-z_][A-Za-z0-9_.:-]*|#[0-9]+|#x[0-9a-fA-F]+);`)

func repairXML(text string) (string, []string, error) {
	if _, err := FormatXML(text); err == nil {
		return text, nil, nil
	}
	if strings.Contains(text, "<!DOCTYPE") {
		return "", nil, fmt.Errorf("含 DOCTYPE 的 XML 不自动改写实体")
	}
	var out strings.Builder
	changed := false
	for i := 0; i < len(text); {
		endToken := ""
		if strings.HasPrefix(text[i:], "<!--") {
			endToken = "-->"
		} else if strings.HasPrefix(text[i:], "<![CDATA[") {
			endToken = "]]>"
		} else if strings.HasPrefix(text[i:], "<?") {
			endToken = "?>"
		}
		if endToken != "" {
			j := strings.Index(text[i:], endToken)
			if j < 0 {
				return "", nil, fmt.Errorf("XML 注释或特殊区段未闭合")
			}
			end := i + j + len(endToken)
			out.WriteString(text[i:end])
			i = end
			continue
		}
		if text[i] == '&' && !entityPattern.MatchString(text[i:]) {
			out.WriteString("&amp;")
			changed = true
		} else {
			out.WriteByte(text[i])
		}
		i++
	}
	var notes []string
	if changed {
		notes = append(notes, "转义未编码的 &，保留注释和 CDATA")
	}
	return out.String(), notes, nil
}

func repairSQL(text string) (string, []string, error) {
	tokens, err := sqlTokens(text)
	if err != nil {
		return "", nil, err
	}
	changed := false
	var out strings.Builder
	replacements := map[string]string{"，": ",", "；": ";", "（": "(", "）": ")", "＝": "="}
	for _, t := range tokens {
		if replacement, ok := replacements[t]; ok {
			t = replacement
			changed = true
		}
		out.WriteString(t)
		if strings.HasPrefix(t, "--") || strings.HasPrefix(t, "#") {
			out.WriteByte('\n')
		} else {
			out.WriteByte(' ')
		}
	}
	if !changed {
		return text, nil, nil
	}
	return out.String(), []string{"将 SQL 中的全角标点转换为半角；不改写字面量和注释"}, nil
}
