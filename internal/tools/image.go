package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/skip2/go-qrcode"
	xdraw "golang.org/x/image/draw"
)

const MaxImageBytes = 32 << 20
const MaxPixels = 24_000_000

func ReadImage(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxImageBytes {
		return nil, fmt.Errorf("图片超过 32 MiB 限制")
	}
	return b, nil
}

func DecodeImage(data []byte) (image.Image, string, error) {
	if len(data) > MaxImageBytes {
		return nil, "", fmt.Errorf("图片超过 32 MiB 限制")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("只支持有效的 PNG/JPEG 图片：%w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, "", fmt.Errorf("图片超过 2400 万像素限制")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, format, err
}

func ImageToBase64(data []byte, prefix bool) (string, error) {
	_, format, err := DecodeImage(data)
	if err != nil {
		return "", err
	}
	value := base64.StdEncoding.EncodeToString(data)
	if prefix {
		value = "data:image/" + format + ";base64," + value
	}
	return value, nil
}

func Base64ToImage(text string) ([]byte, string, error) {
	if len(text) > MaxImageBytes*2 {
		return nil, "", fmt.Errorf("Base64 内容过大")
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "data:") {
		i := strings.IndexByte(text, ',')
		if i < 0 || !strings.HasSuffix(text[:i], ";base64") {
			return nil, "", fmt.Errorf("Data URI 前缀无效")
		}
		text = text[i+1:]
	}
	text = strings.Join(strings.Fields(text), "")
	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil {
		return nil, "", fmt.Errorf("Base64 无效：%w", err)
	}
	_, format, err := DecodeImage(b)
	return b, format, err
}

func CompressImage(data []byte, format string, quality, maxWidth int) ([]byte, error) {
	img, original, err := DecodeImage(data)
	if err != nil {
		return nil, err
	}
	if quality < 1 || quality > 100 {
		return nil, fmt.Errorf("JPEG 质量必须为 1～100")
	}
	if maxWidth < 0 || maxWidth > 16000 {
		return nil, fmt.Errorf("最大宽度必须为 0～16000，0 表示原尺寸")
	}
	resized := false
	if maxWidth > 0 && img.Bounds().Dx() > maxWidth {
		h := max(1, int(int64(img.Bounds().Dy())*int64(maxWidth)/int64(img.Bounds().Dx())))
		dst := image.NewNRGBA(image.Rect(0, 0, maxWidth, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
		resized = true
	}
	var out bytes.Buffer
	switch format {
	case "png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(&out, img)
		// 无损优化不一定更小；保留原图以免生成更大的同格式文件。
		if err == nil && original == "png" && !resized && out.Len() >= len(data) {
			return append([]byte(nil), data...), nil
		}
	case "jpeg":
		background := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(background, background.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		draw.Draw(background, background.Bounds(), img, img.Bounds().Min, draw.Over)
		err = jpeg.Encode(&out, background, &jpeg.Options{Quality: quality})
	default:
		return nil, fmt.Errorf("输出格式必须为 png 或 jpeg")
	}
	return out.Bytes(), err
}

func QRCode(text string, size, level int) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("请输入二维码内容")
	}
	if len(text) > 4096 {
		return nil, fmt.Errorf("二维码内容过长")
	}
	if size < 128 || size > 2048 {
		return nil, fmt.Errorf("二维码尺寸必须为 128～2048")
	}
	if level < 0 || level > 3 {
		return nil, fmt.Errorf("纠错等级无效")
	}
	code, err := qrcode.New(text, qrcode.RecoveryLevel(level))
	if err != nil {
		return nil, err
	}
	return code.PNG(size)
}
