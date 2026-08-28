package random

import (
	"github.com/mulan-go/mulan/captcha/core"
)

type LetterType int

const (
	LetterDigits LetterType = iota << 2
	LetterLower
	LetterUpper

	lowerLetters = "abcdefghijklmnopqrstuvwxyz"
	upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitLetters = "0123456789"
)

var _ core.ICaptcha = (*Random)(nil)

type Random struct {
	*core.Captcha
}

func randomGenerator(count int, lType LetterType) core.Generator {
	letters := ""
	if lType&LetterDigits != 0 {
		letters += digitLetters
	}
	if lType&LetterLower != 0 {
		letters += lowerLetters
	}
	if lType&LetterUpper != 0 {
		letters += upperLetters
	}
	randomStr := func() string {
		return string(core.RandomSpecialBytes(count, letters))
	}
	return func() (content, result string) {
		content = randomStr()
		return content, content
	}
}

func NewRandom(count int, letterType LetterType, opts ...core.Option) *Random {
	if count <= 0 {
		count = 6
	}
	c := core.DefaultCaptcha().
		Options(
			append(
				opts,
				core.WithGenerator(
					randomGenerator(count, letterType),
				),
			)...,
		).Init()
	return &Random{Captcha: c}
}
