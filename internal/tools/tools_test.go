package tools

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	img.Set(10, 10, color.NRGBA{R: 255, A: 128})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestBase64RoundTrip(t *testing.T) {
	b := testPNG(t)
	for _, prefix := range []bool{false, true} {
		encoded, err := ImageToBase64(b, prefix)
		if err != nil {
			t.Fatal(err)
		}
		decoded, format, err := Base64ToImage(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if format != "png" || !bytes.Equal(decoded, b) {
			t.Fatal("图片往返转换改变了原始文件")
		}
		if prefix && !strings.HasPrefix(encoded, "data:image/png;base64,") {
			t.Fatal("前缀不正确")
		}
	}
	if _, _, err := Base64ToImage("data:image/png,broken"); err == nil {
		t.Fatal("应拒绝无效前缀")
	}
	if _, err := ImageToBase64([]byte("not image"), true); err == nil {
		t.Fatal("应拒绝非图片")
	}
}

func TestCompression(t *testing.T) {
	input := testPNG(t)
	out, err := CompressImage(input, "png", 80, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, _, _ := DecodeImage(input)
	b, _, _ := DecodeImage(out)
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				t.Fatal("PNG 无损优化改变像素")
			}
		}
	}
	if len(out) > len(input) {
		t.Fatal("PNG 无损优化不应增加大小")
	}
	resized, err := CompressImage(input, "jpeg", 75, 20)
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := DecodeImage(resized)
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" || img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Fatal("缩放比例或格式不正确")
	}
	r, g, bv, _ := img.At(0, 0).RGBA()
	if r < 60000 || g < 60000 || bv < 60000 {
		t.Fatal("JPEG 透明背景应合成白色")
	}
	if _, err := CompressImage(input, "jpeg", 0, 0); err == nil {
		t.Fatal("应拒绝无效质量")
	}
}

func TestQRCode(t *testing.T) {
	for level := 0; level < 4; level++ {
		b, err := QRCode("中文测试：https://example.com", 256, level)
		if err != nil {
			t.Fatal(err)
		}
		img, format, err := DecodeImage(b)
		if err != nil {
			t.Fatal(err)
		}
		if format != "png" || img.Bounds().Dx() != 256 {
			t.Fatal("二维码尺寸或格式不正确")
		}
	}
	if _, err := QRCode("", 256, 1); err == nil {
		t.Fatal("应拒绝空二维码")
	}
}

func TestJSONPreservesNumbers(t *testing.T) {
	s := `{"large":900719925474099312345,"value":1.2300}`
	pretty, err := FormatJSON(s, false)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := FormatJSON(pretty, true)
	if err != nil {
		t.Fatal(err)
	}
	if compact != s {
		t.Fatalf("格式化改变了数值：%s", compact)
	}
	if _, err := FormatJSON(`{"a":}`, false); err == nil {
		t.Fatal("应拒绝无效 JSON")
	}
}

func TestExpressions(t *testing.T) {
	result, err := TestJSONPath(`{"items":[{"price":2},{"price":5}]}`, `$.items[?(@.price > 3)].price`)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	if err := json.Unmarshal([]byte(result), &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 5 {
		t.Fatalf("JSONPath 过滤错误：%s", result)
	}
	result, err = TestXPath(`<root><item>甲</item><item>乙</item></root>`, `//item`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "匹配 2 个节点") || !strings.Contains(result, "乙") {
		t.Fatal(result)
	}
	if _, err := TestXPath(`<root/>`, `//*[`); err == nil {
		t.Fatal("应拒绝无效 XPath")
	}
	result, err = TestRegex("张三 123 李四 456", `(?P<number>\d+)`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "number：123") || !strings.Contains(result, "number：456") {
		t.Fatal(result)
	}
	if _, err := TestRegex("test", `(?<=a)b`); err == nil {
		t.Fatal("应拒绝不受支持的后行断言")
	}
}

func TestXMLAndSQL(t *testing.T) {
	result, err := FormatXML(`<root><item a="b">中文</item></root>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "\n  <item") {
		t.Fatal("XML 未缩进")
	}
	if _, err := FormatXML(`<root><item></root>`); err == nil {
		t.Fatal("应拒绝无效 XML")
	}
	sql := "select 'from where', `select` from users -- where comment\nwhere id=1 and name='a''b'; select $$from where$$;"
	result, err = FormatSQL(sql)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range []string{"'from where'", "`select`", "-- where comment", "'a''b'", "$$from where$$"} {
		if !strings.Contains(result, literal) {
			t.Fatalf("SQL 改写了字面量：%s", result)
		}
	}
	if !strings.Contains(result, "\nFROM") {
		t.Fatal("SQL 未分段")
	}
	if _, err := FormatSQL("select 'unclosed"); err == nil {
		t.Fatal("应拒绝未闭合字符串")
	}
	result, err = FormatSQL("select doc->>'name', doc#>>'{a,b}', value::text, N'中文', E'escape' from items where id=:id and other=$1 and user_id=@user_id")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"->>", "#>>", "::", ":id", "$1", "@user_id", "N'中文'", "E'escape'"} {
		if !strings.Contains(result, token) {
			t.Fatalf("SQL 运算符或参数被改写：%s", result)
		}
	}
}
