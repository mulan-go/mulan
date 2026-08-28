# Mulan Captcha - 验证码

`mulan-captcha` 提供带内存缓存、过期控制和一次性校验的图片验证码。默认全局实例是两位数算式验证码。

## 安装与导入

```bash
go get github.com/mulan-go/mulan/captcha
```

```go
import (
	"github.com/mulan-go/mulan/captcha"
	"github.com/mulan-go/mulan/captcha/core"
	"github.com/mulan-go/mulan/captcha/equation"
	"github.com/mulan-go/mulan/captcha/random"
)
```

## 核心接口

```go
type ICaptcha interface {
	Get(id string) (*Data, bool)
	Remove(id string)
	Check(id, code string) (bool, error)
	Draw() (string, *Data, image.Image)
	Create() (string, string, image.Image)
	CreateBytes() (string, string, []byte, error)
	CreateB64() (string, string, string, error)
}

type Data struct {
	Content string
	Result  string
	Expire  int64
}
```

`Check` 行为：

- 正确：返回 `true, nil`，并删除该验证码
- 已过期：删除该验证码，返回 `core.ErrCodeExpired`
- ID 不存在或答案错误：返回 `core.ErrCodeInvalid`

## 全局入口

```go
func SetGlobal(m core.ICaptcha)
func Create() (id, result string, img image.Image)
func CreateBytes() (id, result string, buf []byte, err error)
func CreateB64() (id, result, data string, err error)
func Check(id, code string) (bool, error)
```

`SetGlobal` 与全局创建、校验操作可并发使用。全局实例替换由读写锁保护。

默认实例：

```go
equation.NewEquation(2)
```

## 内置实现

### 算式验证码

```go
func equation.NewEquation(bit int, opts ...core.Option) *equation.Equation
```

`bit` 必须大于 0，表示参与运算数字的位数。

```go
captcha.SetGlobal(equation.NewEquation(
	2,
	core.WithExpire(300),
	core.WithSize(128, 52),
))
```

### 随机字符验证码

```go
func random.NewRandom(
	count int,
	letterType random.LetterType,
	opts ...core.Option,
) *random.Random
```

`count <= 0` 时按 6 个字符处理。当前版本应传入非零字符集，例如：

```go
captcha.SetGlobal(random.NewRandom(
	6,
	random.LetterLower|random.LetterUpper,
))
```

不要单独传 `random.LetterDigits`：当前代码中它是零值，不能启用数字字符集。

### 可选项

```go
core.WithGenerator(generator)
core.WithSize(width, height)
core.WithBackground(color)
core.WithFront(color)
core.WithExpire(seconds)
core.WithLine(count)
core.WithPoint(count)
core.WithFontSize(size)
core.WithFont(face, size)
core.WithFontName(name, size, dpi)
core.WithFontByte(buf, size, dpi)
```

默认过期时间为 300 秒。过期项在绘制新验证码时按需清理，清理节流为每分钟最多一次；没有常驻清理 goroutine。

## Gin 接入

生成接口只返回 `id` 和图片。`result` 用于服务端调试，不应发给客户端，也无需另存到 Session 或 Redis；验证码实例已经保存答案。

```go
type CaptchaRsp struct {
	ID    string `json:"id"`
	Image string `json:"image"`
}

func HandleCaptcha(c *gin.Context) {
	id, _, data, err := captcha.CreateB64()
	if err != nil {
		c.JSON(http.StatusInternalServerError, rsp.C(err))
		return
	}
	c.JSON(http.StatusOK, rsp.S(CaptchaRsp{ID: id, Image: data}))
}
```

校验接口：

```go
type VerifyReq struct {
	ID   string `json:"id" binding:"required"`
	Code string `json:"code" binding:"required"`
}

func HandleVerify(c *gin.Context) {
	var input VerifyReq
	if err := req.Bind(c, &input); err != nil {
		c.JSON(http.StatusBadRequest, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}
	ok, err := captcha.Check(input.ID, input.Code)
	if err != nil || !ok {
		c.JSON(http.StatusOK, rsp.E(rsp.CaptchaInvalid.WithMsg("验证码错误或已过期")))
		return
	}
	c.JSON(http.StatusOK, rsp.OK())
}
```

需要限制暴力尝试时，在 Handler 外层增加按 IP、账号或设备维度的限流。错误答案不会立即删除验证码，正确或过期时才删除。

## 实例隔离

不同业务需要独立策略时，直接持有实例，不修改全局对象：

```go
type AuthHandler struct {
	captcha core.ICaptcha
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{captcha: equation.NewEquation(1)}
}
```

这种方式适合测试、不同有效期、不同图片尺寸或不同业务限流策略。

## 使用约束

- 不向客户端返回 `result`
- 验证时使用同一实例生成的 `id`
- 正确校验后无需再次 `Remove`
- 多实例部署时，本地缓存互不共享；需要粘性路由，或在上层改造共享校验存储
- 全局实例允许并发替换，但频繁替换会让旧实例中的未验证 ID 不再可达
