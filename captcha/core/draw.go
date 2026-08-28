package core

import (
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"math/rand"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

//go:embed captcha.ttf
var fontData []byte

var gdCosT = []int{1024, 1023, 1023, 1022, 1021, 1020, 1018, 1016, 1014, 1011, 1008, 1005, 1001, 997, 993, 989, 984, 979, 973, 968, 962, 955, 949, 942, 935, 928, 920, 912, 904, 895, 886, 877, 868, 858, 848, 838, 828, 817, 806, 795, 784, 772, 760, 748, 736, 724, 711, 698, 685, 671, 658, 644, 630, 616, 601, 587, 572, 557, 542, 527, 512, 496, 480, 464, 448, 432, 416, 400, 383, 366, 350, 333, 316, 299, 282, 265, 247, 230, 212, 195, 177, 160, 142, 124, 107, 89, 71, 53, 35, 17, 0, -17, -35, -53, -71, -89, -107, -124, -142, -160, -177, -195, -212, -230, -247, -265, -282, -299, -316, -333, -350, -366, -383, -400, -416, -432, -448, -464, -480, -496, -512, -527, -542, -557, -572, -587, -601, -616, -630, -644, -658, -671, -685, -698, -711, -724, -736, -748, -760, -772, -784, -795, -806, -817, -828, -838, -848, -858, -868, -877, -886, -895, -904, -912, -920, -928, -935, -942, -949, -955, -962, -968, -973, -979, -984, -989, -993, -997, -1001, -1005, -1008, -1011, -1014, -1016, -1018, -1020, -1021, -1022, -1023, -1023, -1024, -1023, -1023, -1022, -1021, -1020, -1018, -1016, -1014, -1011, -1008, -1005, -1001, -997, -993, -989, -984, -979, -973, -968, -962, -955, -949, -942, -935, -928, -920, -912, -904, -895, -886, -877, -868, -858, -848, -838, -828, -817, -806, -795, -784, -772, -760, -748, -736, -724, -711, -698, -685, -671, -658, -644, -630, -616, -601, -587, -572, -557, -542, -527, -512, -496, -480, -464, -448, -432, -416, -400, -383, -366, -350, -333, -316, -299, -282, -265, -247, -230, -212, -195, -177, -160, -142, -124, -107, -89, -71, -53, -35, -17, 0, 17, 35, 53, 71, 89, 107, 124, 142, 160, 177, 195, 212, 230, 247, 265, 282, 299, 316, 333, 350, 366, 383, 400, 416, 432, 448, 464, 480, 496, 512, 527, 542, 557, 572, 587, 601, 616, 630, 644, 658, 671, 685, 698, 711, 724, 736, 748, 760, 772, 784, 795, 806, 817, 828, 838, 848, 858, 868, 877, 886, 895, 904, 912, 920, 928, 935, 942, 949, 955, 962, 968, 973, 979, 984, 989, 993, 997, 1001, 1005, 1008, 1011, 1014, 1016, 1018, 1020, 1021, 1022, 1023, 1023}

var gdSinT = []int{0, 17, 35, 53, 71, 89, 107, 124, 142, 160, 177, 195, 212, 230, 247, 265, 282, 299, 316, 333, 350, 366, 383, 400, 416, 432, 448, 464, 480, 496, 512, 527, 542, 557, 572, 587, 601, 616, 630, 644, 658, 671, 685, 698, 711, 724, 736, 748, 760, 772, 784, 795, 806, 817, 828, 838, 848, 858, 868, 877, 886, 895, 904, 912, 920, 928, 935, 942, 949, 955, 962, 968, 973, 979, 984, 989, 993, 997, 1001, 1005, 1008, 1011, 1014, 1016, 1018, 1020, 1021, 1022, 1023, 1023, 1024, 1023, 1023, 1022, 1021, 1020, 1018, 1016, 1014, 1011, 1008, 1005, 1001, 997, 993, 989, 984, 979, 973, 968, 962, 955, 949, 942, 935, 928, 920, 912, 904, 895, 886, 877, 868, 858, 848, 838, 828, 817, 806, 795, 784, 772, 760, 748, 736, 724, 711, 698, 685, 671, 658, 644, 630, 616, 601, 587, 572, 557, 542, 527, 512, 496, 480, 464, 448, 432, 416, 400, 383, 366, 350, 333, 316, 299, 282, 265, 247, 230, 212, 195, 177, 160, 142, 124, 107, 89, 71, 53, 35, 17, 0, -17, -35, -53, -71, -89, -107, -124, -142, -160, -177, -195, -212, -230, -247, -265, -282, -299, -316, -333, -350, -366, -383, -400, -416, -432, -448, -464, -480, -496, -512, -527, -542, -557, -572, -587, -601, -616, -630, -644, -658, -671, -685, -698, -711, -724, -736, -748, -760, -772, -784, -795, -806, -817, -828, -838, -848, -858, -868, -877, -886, -895, -904, -912, -920, -928, -935, -942, -949, -955, -962, -968, -973, -979, -984, -989, -993, -997, -1001, -1005, -1008, -1011, -1014, -1016, -1018, -1020, -1021, -1022, -1023, -1023, -1024, -1023, -1023, -1022, -1021, -1020, -1018, -1016, -1014, -1011, -1008, -1005, -1001, -997, -993, -989, -984, -979, -973, -968, -962, -955, -949, -942, -935, -928, -920, -912, -904, -895, -886, -877, -868, -858, -848, -838, -828, -817, -806, -795, -784, -772, -760, -748, -736, -724, -711, -698, -685, -671, -658, -644, -630, -616, -601, -587, -572, -557, -542, -527, -512, -496, -480, -464, -448, -432, -416, -400, -383, -366, -350, -333, -316, -299, -282, -265, -247, -230, -212, -195, -177, -160, -142, -124, -107, -89, -71, -53, -35, -17}

type captcha struct {
	*image.RGBA
	width, height int
}

// newCaptcha 创建一个新的图片
func newCaptcha(w, h int) *captcha {
	return &captcha{RGBA: image.NewRGBA(image.Rect(0, 0, w, h)), width: w, height: h}
}

// fillBkg 填充背景色
func (img *captcha) fillBkg(c color.Color) *captcha {
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Over)
	return img
}

// TODO: rotate 旋转
func (img *captcha) rotate(_ /* deg */ int) *captcha {
	return img
}

// TODO: distort 扭曲
func (img *captcha) distort(_ /* deg */ int) *captcha {
	return img
}

// drawText 绘制文本
func (img *captcha) drawText(text string, fontSize int, fontFace font.Face, front color.Color) *captcha {
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(front),
		Face: fontFace,
		Dot:  fixed.P(0, fontSize),
	}
	// 字符串居中
	m := fontFace.Metrics()
	b, _ := drawer.BoundString(text)
	// 单个字符宽度
	width := b.Max.X / fixed.I(len(text))
	// 字符串水平居中
	drawer.Dot.X = (fixed.I(img.width) - b.Max.X) / 2
	// 字符串垂直居中
	imgH := fixed.I(img.height)
	intervalY := int32(imgH - m.Height)
	baseY := imgH/2 + m.Height/4 + imgH - m.Height
	// 绘制每个字符
	for _, c := range text {
		drawer.Dot.Y = baseY - fixed.Int26_6(rand.Int31n(intervalY*2))
		drawer.DrawString(string(c))
		drawer.Dot.X += width
	}
	return img
}

// drawLines 绘制干扰线
func (img *captcha) drawLines(n int) *captcha {
	lx := img.width / 4
	for i := 0; i <= n; i++ {
		x := rand.Intn(lx)
		rx := rand.Intn(lx) + lx*3
		y := rand.Intn(img.height)
		ry := rand.Intn(img.height)
		c := randColorAlpha()
		if i%3 == 0 {
			// 直线
			img.drawLine(x, y, rx, ry, c)
		} else {
			// 弧线
			img.drawArcLine(x, y, rand.Intn(img.width)+50, rand.Intn(img.height), rand.Intn(360), rand.Intn(360), c)
		}
	}
	return img
}

// drawLine 画直线 x1,y1 起点 x2,y2终点
// Bresenham算法(https://zh.wikipedia.org/zh-cn/布雷森漢姆直線演算法#最佳化)
func (img *captcha) drawLine(x0, y0, x1, y1 int, c color.Color) *captcha {
	steep := abs(y1-y0) > abs(x1-x0)
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}
	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}
	dx := x1 - x0
	dy := abs(y1 - y0)
	err := dx / 2
	y := y0
	var ystep int
	if y0 < y1 {
		ystep = 1
	} else {
		ystep = -1
	}
	for x := x0; x <= x1; x++ {
		if steep {
			img.Set(y, x, c)
		} else {
			img.Set(x, y, c)
		}
		err -= dy
		if err < 0 {
			y += ystep
			err += dx
		}
	}
	return img
}

// drawPoints 绘制干扰点
func (img *captcha) drawPoints(n int) *captcha {
	for i := 0; i <= n; i++ {
		img.Set(rand.Intn(img.width)+1, rand.Intn(img.height)+1, randColor())
	}
	return img
}

// drawArcLine 绘制弧线
// imagearc from php-gd
func (img *captcha) drawArcLine(centerX, centerY, width, height, startAngle, endAngle int, c color.Color) *captcha {
	var lx, ly, endx, endy int
	if (startAngle % 360) == (endAngle % 360) {
		startAngle = 0
		endAngle = 360
	} else {
		startAngle = startAngle % 360
		endAngle = endAngle % 360
		for endAngle < startAngle {
			endAngle += 360
		}
		if startAngle == endAngle {
			startAngle = 0
			endAngle = 360
		}
	}
	for i := startAngle; i <= endAngle; i++ {
		endx = gdCosT[i%360]*width/2048 + centerX
		endy = gdSinT[i%360]*height/2048 + centerY
		if i != startAngle {
			img.drawLine(lx, ly, endx, endy, c)
		}
		lx, ly = endx, endy
	}
	return img
}
