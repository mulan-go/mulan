package equation

import (
	"bytes"
	"math"
	"math/rand"
	"strconv"

	"github.com/mulan-go/mulan/captcha/core"
)

const (
	equationMethodAdd = iota // x + y = z
	equationMethodSub        // x - y = z
	equationMethodMul        // x * y = z
)

var _ core.ICaptcha = (*Equation)(nil)

type Equation struct {
	*core.Captcha
}

func NewEquation(bit int, opts ...core.Option) *Equation {
	if bit <= 0 {
		panic("bit must be greater than 0")
	}
	c := core.DefaultCaptcha().
		Options(
			append(
				opts,
				core.WithGenerator(equationGenerator(bit)),
			)...,
		).Init()
	return &Equation{Captcha: c}
}

func equationGenerator(bit int) core.Generator {
	minNum := int64(math.Pow10(bit - 1))
	int63n := core.BitNumInt63n(bit)
	return func() (content, result string) {
		var resultNum int64
		buf := new(bytes.Buffer)
		switch rand.Intn(3) {
		case equationMethodAdd: // left + right = result
			var left, right int64
			resultNum = int63n()
			for resultNum < minNum*2 {
				resultNum = int63n()
			}
			for left < minNum || right < minNum {
				left = rand.Int63n(resultNum-minNum) + minNum
				right = resultNum - left
			}
			buf.WriteString(strconv.FormatInt(left, 10))
			buf.WriteByte('+')
			buf.WriteString(strconv.FormatInt(resultNum-left, 10))
		case equationMethodSub: // left - right = result
			var left, right int64
			for left < minNum*2 {
				left = int63n()
			}
			for resultNum < minNum || right < minNum {
				resultNum = rand.Int63n(left-minNum) + minNum
				right = left - resultNum
			}
			buf.WriteString(strconv.FormatInt(left, 10))
			buf.WriteByte('-')
			buf.WriteString(strconv.FormatInt(right, 10))
		case equationMethodMul: // left * right = result
			left, right := int63n(), int63n()
			resultNum = left * right
			buf.WriteString(strconv.FormatInt(left, 10))
			buf.WriteByte('*')
			buf.WriteString(strconv.FormatInt(right, 10))
		}
		return buf.String(), strconv.FormatInt(resultNum, 10)
	}
}
