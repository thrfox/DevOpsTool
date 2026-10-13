package main

import (
	"bytes"
	"encoding/binary"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// 从同一 SVG 源生成窗口图标与 Windows 多尺寸图标，保持发布资源一致。
func main() {
	root := "internal/ui/assets"
	svg, err := os.ReadFile(filepath.Join(root, "app.svg"))
	if err != nil {
		panic(err)
	}
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	var images [][]byte
	for _, size := range sizes {
		icon, err := oksvg.ReadIconStream(bytes.NewReader(svg))
		if err != nil {
			panic(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		icon.SetTarget(0, 0, float64(size), float64(size))
		scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
		icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			panic(err)
		}
		images = append(images, b.Bytes())
		if size == 256 {
			if err := os.WriteFile(filepath.Join(root, "app.png"), b.Bytes(), 0644); err != nil {
				panic(err)
			}
		}
	}
	var ico bytes.Buffer
	for _, v := range []uint16{0, 1, uint16(len(sizes))} {
		binary.Write(&ico, binary.LittleEndian, v)
	}
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		dimension := byte(size)
		ico.Write([]byte{dimension, dimension, 0, 0})
		binary.Write(&ico, binary.LittleEndian, uint16(1))
		binary.Write(&ico, binary.LittleEndian, uint16(32))
		binary.Write(&ico, binary.LittleEndian, uint32(len(images[i])))
		binary.Write(&ico, binary.LittleEndian, uint32(offset))
		offset += len(images[i])
	}
	for _, data := range images {
		ico.Write(data)
	}
	if err := os.WriteFile(filepath.Join(root, "app.ico"), ico.Bytes(), 0644); err != nil {
		panic(err)
	}
}
