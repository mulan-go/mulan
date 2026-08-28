package core

import (
	crand "crypto/rand"
	"encoding/hex"
	"image/color"
	"math"
	"math/rand"
	"strconv"

	"golang.org/x/image/math/fixed"
)

const (
	letterBytes   = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxMask = 1<<6 - 1 // All 1-bits, as many as 6
	letterIdxMax  = 63 / 6   // # of letter indices fitting in 63 bits
)

func RandomCode() string {
	return strconv.Itoa(rand.Intn(900000) + 100000)
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= 6
		remain--
	}
	return b
}
func RandomSpecialBytes(n int, special string) []byte {
	b := make([]byte, n)
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(special) {
			b[i] = special[idx]
			i--
		}
		cache >>= 6
		remain--
	}
	return b
}

func RandomBytesHex(n int) string {
	return hex.EncodeToString(RandomBytes(n))
}

func RandomString(n int) string {
	return string(RandomBytes(n))
}

func abs(x int) int {
	return int(math.Abs(float64(x)))
}

func drawRect(fr fixed.Rectangle26_6) (x, y, width, height int) {
	return 0, 0, int(fr.Max.X+fr.Min.X) >> 6, int(fr.Max.Y+fr.Min.Y)>>6 + 1
}

func randColor() color.Color {
	buf := make([]byte, 4)
	crand.Read(buf)
	return color.RGBA{buf[0], buf[1], buf[2], 0xff}
}

func randColorAlpha() color.Color {
	buf := make([]byte, 4)
	crand.Read(buf)
	return color.RGBA{buf[0], buf[1], buf[2], buf[3]}
}
