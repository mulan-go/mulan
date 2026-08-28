package core

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestBound(t *testing.T) {
	fontSize := 32
	fontFace, err := GetFontFace(nil, fontSize, 72)
	if err != nil {
		t.Error(err)
	}
	var (
		w, h, sumW, sumH int
		l                = 6
	)
	drawer := &font.Drawer{
		Face: fontFace,
		Dot:  fixed.P(0, int(fontSize)),
	}
	for range 100 {
		db, _ := drawer.BoundString(RandomString(l))
		_, _, w, h = drawRect(db)
		sumW += w
		sumH += h
	}
	avgH := float64(sumH) / float64(100)
	avgW := float64(sumW) / float64(100)
	fmt.Printf("sumW: %d sumH: %d avgW: %f avgH: %f w/h %f w/l: %f",
		sumW, sumH,
		avgW, avgH,
		avgW/avgH,
		avgW/float64(l))
}

// 字体，字体大小，字符串长度 ~ 图片尺寸
// 字符串区域检测
func TestCheck(t *testing.T) {
	var (
		fontSize = 32.0
		content  = RandomString(8)
	)
	obj, _ := sfnt.Parse(fontData)
	fontFace, _ := opentype.NewFace(obj, &opentype.FaceOptions{
		Size: fontSize, DPI: 72, Hinting: font.HintingFull,
	})
	m := fontFace.Metrics()
	// 上伸线(Ascent Line): 一条与字体最高点对齐的虚构线。
	// 下伸线(Descent Line): 一条与字体最低点对齐的虚构线。
	// 高度(Height): 上伸线到下伸线的距离。
	fmt.Printf("m.Ascent %+v\n", int(m.Ascent>>6))
	fmt.Printf("m.Descent %+v\n", int(m.Descent>>6))
	fmt.Printf("m.Height %+v\n", int(m.Height>>6))
	// 文本画布
	drawer := &font.Drawer{
		Src:  image.Black,
		Face: fontFace,
		Dot:  fixed.P(0, int(fontSize)),
	}
	db, _ := drawer.BoundString(content)
	var _, _, width, height = drawRect(db)
	fmt.Printf("width: %d height: %d\n", width, height)
	// 绘制文本
	img := newCaptcha(width, height).fillBkg(color.White)
	fmt.Println("图片大小: ", img.Bounds().Max)
	drawer.Dst = img
	drawer.DrawString(content)
	// 输出文件
	f, err := os.OpenFile(filepath.Join(t.TempDir(), "test.png"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		t.Fail()
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		t.Fail()
	}
}

func TestDrawArcLine(t *testing.T) {
	img := newCaptcha(100, 40).fillBkg(color.White)
	img.drawArcLine(20, 20, 5, 8, 23, 43, randColorAlpha())
}

func TestDraw(t *testing.T) {
	fontSize := 32
	fontFace := DefaultFontFace(fontSize)
	img := newCaptcha(120, 50).
		fillBkg(color.White).
		drawText("98-77=?", fontSize, fontFace, color.Black)
	f, err := os.OpenFile(filepath.Join(t.TempDir(), "test.png"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		t.Fail()
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		t.Fail()
	}
}

func BenchmarkDrawText(b *testing.B) {
	fontSize := 32
	fontFace := DefaultFontFace(fontSize)
	img := newCaptcha(120, 50).fillBkg(color.White)
	for b.Loop() {
		img.drawText("78-35", fontSize, fontFace, color.Black)
	}
}
