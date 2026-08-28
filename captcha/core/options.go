package core

import (
	"image/color"
	"os"

	"go.uber.org/zap"
	"golang.org/x/image/font"
)

type Option func(b *Captcha)

func WithGenerator(generator Generator) Option {
	return func(b *Captcha) { b.Generator = generator }
}

func WithSize(width, hight int) Option {
	return func(b *Captcha) { b.height, b.width = hight, width }
}

func WithBackground(color color.Color) Option {
	return func(b *Captcha) { b.bg = color }
}

func WithFront(color color.Color) Option {
	return func(b *Captcha) { b.front = color }
}

func WithExpire(t int64) Option {
	return func(b *Captcha) { b.expire = t }
}

func WithLine(n int) Option {
	return func(b *Captcha) { b.line = n }
}

func WithPoint(n int) Option {
	return func(b *Captcha) { b.point = n }
}

func WithFontSize(size int) Option {
	return func(b *Captcha) { b.fontSize = size }
}

func WithFont(fontFace font.Face, size int) Option {
	return func(b *Captcha) { b.fontFace, b.fontSize = fontFace, size }
}

func WithFontName(fontName string, fontSize int, dpi int) Option {
	return func(b *Captcha) {
		buf, err := os.ReadFile(fontName)
		if err != nil {
			zap.L().Warn("Failed to read font", zap.Error(err))
		} else {
			WithFontByte(buf, fontSize, dpi)(b)
		}
	}
}

func WithFontByte(buf []byte, fontSize int, dpi int) Option {
	return func(b *Captcha) {
		fontFace, err := GetFontFace(buf, fontSize, dpi)
		if err != nil {
			zap.L().Warn("Failed to get fontface", zap.Error(err))
			return
		}
		WithFont(fontFace, fontSize)(b)
	}
}
