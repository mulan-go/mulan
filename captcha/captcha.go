package captcha

import (
	"image"
	"sync"

	"github.com/mulan-go/mulan/captcha/core"
	"github.com/mulan-go/mulan/captcha/equation"
)

var (
	std   core.ICaptcha = equation.NewEquation(2)
	stdMu sync.RWMutex
)

func SetGlobal(m core.ICaptcha) {
	stdMu.Lock()
	std = m
	stdMu.Unlock()
}

func global() core.ICaptcha {
	stdMu.RLock()
	defer stdMu.RUnlock()
	return std
}

// Create 创建验证码，返回图片对象image.Image
func Create() (id, result string, img image.Image) {
	return global().Create()
}

// CreateBytes 创建验证码，返回图片数据[]byte
func CreateBytes() (id, result string, buf []byte, err error) {
	return global().CreateBytes()
}

// CreateB64 创建验证码，返回图片base64
func CreateB64() (id, result, data string, err error) {
	return global().CreateB64()
}

func Check(id, code string) (bool, error) {
	return global().Check(id, code)
}
