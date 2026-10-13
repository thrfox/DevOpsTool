package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONOptionalRepair(t *testing.T) {
	input := `{name:'中文',items:[1,2,], enabled:True, missing:None, /* 备注 */ big:900719925474099312345,}`
	if _, _, err := FormatWithRepair("JSON 格式化", input, false); err == nil {
		t.Fatal("关闭纠错时应保持严格校验")
	}
	out, notes, err := FormatWithRepair("JSON 格式化", input, true)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(out)) || len(notes) < 4 || !strings.Contains(out, "900719925474099312345") {
		t.Fatalf("纠错结果不完整或数值改变：%s，%v", out, notes)
	}
	valid := `{"message":"a,] b // /* ' &","number":1.2300}`
	out, notes, err = FormatWithRepair("JSON 压缩", valid, true)
	if err != nil || out != valid || len(notes) != 0 {
		t.Fatalf("有效 JSON 不应被纠错改写：%s %v", out, err)
	}
	for _, broken := range []string{`{"value":"unfinished}`, `{"a":1 "b":2}`, `{"a":`} {
		if _, _, err := FormatWithRepair("JSON 格式化", broken, true); err == nil {
			t.Fatalf("不能猜测缺失结构：%s", broken)
		}
	}
	out, _, err = FormatWithRepair("JSON 格式化", "```json\n"+`{value:'a\'b',}`+"\n```", true)
	if err != nil || !strings.Contains(out, "a'b") {
		t.Fatalf("单引号转义或代码围栏修复失败：%s %v", out, err)
	}
}

func TestXMLAndSQLRepair(t *testing.T) {
	input := `<root attr="a&b"><![CDATA[x&y]]><!-- z&w --><item>c&d</item></root>`
	out, notes, err := FormatWithRepair("XML 格式化", input, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !strings.Contains(out, "a&amp;b") || !strings.Contains(out, "<![CDATA[x&y]]>") || !strings.Contains(out, "<!-- z&w -->") {
		t.Fatalf("XML 修复不应改变 CDATA 或注释：%s", out)
	}
	if _, _, err := FormatWithRepair("XML 格式化", `<root><x></root>`, true); err == nil {
		t.Fatal("应拒绝猜测 XML 标签结构")
	}
	out, notes, err = FormatWithRepair("SQL 格式化", "select （a，b） from table1 where title='中文，标点；'； -- 注释，\nselect 1", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !strings.Contains(out, "'中文，标点；'") || !strings.Contains(out, "-- 注释，") {
		t.Fatal(out)
	}
}
