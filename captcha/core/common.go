package core

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"

	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

var (
	ErrCodeExpired = fmt.Errorf("验证码已过期")
	ErrCodeInvalid = fmt.Errorf("验证码无效")
)

type ICache interface {
	Get(id string) (*Data, bool)
	Remove(id string)
	Check(id, code string) (bool, error)
}

type ICaptcha interface {
	// 验证码缓存
	Get(id string) (*Data, bool)
	Remove(id string)
	Check(id, code string) (bool, error)
	// Draw 绘制验证码
	Draw() (string, *Data, image.Image)
	// Create 创建验证码
	Create() (string, string, image.Image)
	CreateBytes() (string, string, []byte, error)
	CreateB64() (string, string, string, error)
}

type Data struct {
	Content string
	Result  string
	Expire  int64
}

func DefaultFontFace(size int) font.Face {
	fontFace, _ := GetFontFace(fontData, size, 72)
	return fontFace
}

func DefaultCaptcha() *Captcha {
	fontSize := 32
	return &Captcha{
		bucket:   cmap.New[*Data](),
		width:    128,
		height:   52,
		bg:       color.White,
		front:    color.Black,
		fontSize: fontSize,
		fontFace: DefaultFontFace(fontSize),
		expire:   300,
		line:     6,
		point:    100,
	}
}

func GetFontFace(buf []byte, fontSize int, dpi int) (font.Face, error) {
	if buf == nil {
		buf = fontData
	}
	obj, err := sfnt.Parse(buf)
	if err != nil {
		return nil, err
	}
	fontFace, err := opentype.NewFace(
		obj,
		&opentype.FaceOptions{
			Size:    float64(fontSize),
			DPI:     float64(dpi),
			Hinting: font.HintingNone,
		},
	)
	if err != nil {
		return nil, err
	}
	return fontFace, nil
}

func BitNumInt63n(bit int) func() int64 {
	x := int64(math.Pow10(bit - 1))
	y := int64(math.Pow10(bit)) - x
	return func() int64 {
		return rand.Int63n(y) + x
	}
}
