package core

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/image/font"
)

var _ ICaptcha = (*Captcha)(nil)
var _ ICache = (*Captcha)(nil)

type (
	Generator func() (string, string)
	Captcha   struct {
		bucket    cmap.ConcurrentMap[string, *Data]
		bg        color.Color
		front     color.Color
		fontFace  font.Face
		Generator Generator
		fontSize  int
		height    int
		expire    int64
		line      int
		point     int
		rotate    int
		distort   int
		width     int
		cleanupAt atomic.Int64
	}
)

func (c *Captcha) Options(opts ...Option) *Captcha {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Captcha) Draw() (string, *Data, image.Image) {
	now := time.Now().Unix()
	c.cleanupExpired(now)
	// Generate
	text, result := c.Generator()
	// Draw Image
	img := newCaptcha(c.width, c.height).
		fillBkg(c.bg).                                   // 背景
		drawText(text, c.fontSize, c.fontFace, c.front). // 内容
		rotate(c.rotate).                                // 旋转
		distort(c.distort).                              // 扭曲
		drawPoints(c.point).                             // 干扰点
		drawLines(c.line)                                // 干扰线
	// Cache
	data := &Data{
		Content: text,
		Result:  result,
		Expire:  c.expire + now,
	}
	id := uuid.New().String()
	c.bucket.Set(id, data)
	return id, data, img
}

// Init 初始化验证码
func (c *Captcha) Init() *Captcha {
	c.cleanupExpired(time.Now().Unix())
	return c
}

func (c *Captcha) cleanupExpired(now int64) {
	previous := c.cleanupAt.Load()
	if previous != 0 && now-previous < int64(time.Minute/time.Second) {
		return
	}
	if !c.cleanupAt.CompareAndSwap(previous, now) {
		return
	}
	c.bucket.IterCb(func(key string, value *Data) {
		if value.Expire <= now {
			c.bucket.Remove(key)
		}
	})
}

func (c *Captcha) Get(id string) (*Data, bool) { return c.bucket.Get(id) }
func (c *Captcha) Remove(id string)            { c.bucket.Remove(id) }

func (c *Captcha) Check(id, code string) (bool, error) {
	if data, ok := c.Get(id); ok {
		if data.Expire <= time.Now().Unix() {
			c.bucket.Remove(id)
			return false, ErrCodeExpired
		}
		if data.Result == code {
			c.bucket.Remove(id)
			return true, nil
		}
	}
	return false, ErrCodeInvalid
}

// Create 创建验证码，返回图片对象image.Image
func (c *Captcha) Create() (id, result string, img image.Image) {
	id, cd, img := c.Draw()
	if r := recover(); r != nil {
		fmt.Fprintln(os.Stderr, r)
		return c.Create()
	}
	return id, cd.Result, img
}

// CreateBytes 创建验证码，返回图片数据[]byte
func (c *Captcha) CreateBytes() (id, result string, buf []byte, err error) {
	id, result, img := c.Create()
	_buf := new(bytes.Buffer)
	err = png.Encode(_buf, img)
	if err != nil {
		return "", "", nil, err
	}
	return id, result, _buf.Bytes(), nil
}

// CreateB64 创建验证码，返回图片base64
func (c *Captcha) CreateB64() (id, result, data string, err error) {
	id, result, buf, err := c.CreateBytes()
	if err != nil {
		return "", "", "", err
	}
	data = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf)
	return id, result, data, nil
}
