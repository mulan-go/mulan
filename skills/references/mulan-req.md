# Mulan Req - 请求绑定

```go
import "github.com/mulan-go/mulan/req"
```

`mulan-req` 用于把 Gin 请求里的 **路径参数**、**请求体**、**查询参数** 绑定到同一个结构体，并复用 `binding` 标签做校验。

适合在 Handler 层统一处理入参。

---

## 主要能力

- 统一绑定 `path + body + query`
- 使用 `binding` 标签做参数校验
- 兼容 Gin 的绑定体系
- 支持切片、数组、结构体递归校验
- 提供“严格绑定”和“尽力绑定”两种模式

---

## API

### `req.Bind`

严格绑定。

绑定顺序：

1. Path 参数
2. Body
3. Query
4. 默认值处理
5. 结构体验证

只要某一步出错，就直接返回错误。

```go
func Bind(c *gin.Context, obj any) error
```

### `req.ShouldBind`

尽力绑定。

特点：

- 会尝试从多个来源绑定数据
- 忽略各来源的绑定错误
- 最终只返回默认值处理或结构体验证错误

适合一个请求结构体同时承接多个来源时使用。

```go
func ShouldBind(c *gin.Context, obj any) error
```

---

## 常用写法

## 1. 基础绑定

```go
type UserCreateReq struct {
	Name string `json:"name" form:"name" binding:"required,min=2"`
	Age  int    `json:"age" form:"age" binding:"gte=0,lte=150"`
}

func HandleCreate(c *gin.Context) {
	var _req UserCreateReq
	if err := req.Bind(c, &_req); err != nil {
		c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	c.JSON(200, rsp.OK())
}
```

---

## 2. 同时接收路径参数和查询参数

```go
type UserDetailReq struct {
	ID      uint64 `uri:"id" binding:"required"`
	Verbose bool   `form:"verbose"`
}

func HandleDetail(c *gin.Context) {
	var _req UserDetailReq
	if err := req.Bind(c, &_req); err != nil {
		c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	c.JSON(200, rsp.S(_req))
}
```

路由示例：

```go
r.GET("/users/:id", HandleDetail)
```

请求示例：

```text
GET /users/100?verbose=true
```

---

## 3. 同时接收路径、Body、Query

```go
type UserUpdateReq struct {
	ID    uint64 `uri:"id" binding:"required"`
	Name  string `json:"name" form:"name" binding:"required"`
	Force bool   `form:"force"`
}

func HandleUpdate(c *gin.Context) {
	var _req UserUpdateReq
	if err := req.Bind(c, &_req); err != nil {
		c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	c.JSON(200, rsp.S(_req))
}
```

---

## 4. 使用 `ShouldBind`

当你希望“能绑多少绑多少”，最后只看校验结果时，可以用这个。

```go
type SearchReq struct {
	Keyword string `json:"keyword" form:"keyword" binding:"required"`
	Page    int    `form:"page" binding:"gte=1"`
	Size    int    `form:"size" binding:"gte=1,lte=100"`
}

func HandleSearch(c *gin.Context) {
	var _req SearchReq
	if err := req.ShouldBind(c, &_req); err != nil {
		c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	c.JSON(200, rsp.S(_req))
}
```

---

## 标签说明

`mulan-req` 依赖 Gin 的绑定规则，常见标签如下：

- `json:"field"`：绑定 JSON Body
- `form:"field"`：绑定 Query/Form
- `uri:"field"`：绑定路径参数
- `binding:"required"`：必填
- `binding:"min=1,max=10"`：长度或数值范围
- `binding:"oneof=a b c"`：枚举值

示例：

```go
type LoginReq struct {
	Account  string `json:"account" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
	Type     string `json:"type" binding:"required,oneof=admin user"`
}
```

---

## 绑定规则说明

### `Bind` 的处理顺序

`Bind` 会按下面顺序处理：

1. 路径参数
2. 请求体
3. 查询参数
4. 默认值处理
5. 校验

这意味着一个结构体可以同时承接多个来源的字段。

### `ShouldBind` 的设计意图

`ShouldBind` 更适合下面场景：

- 一个结构体同时接多个来源
- 某个来源可能为空
- 希望忽略中间绑定失败，最终只关心校验结果

---

## 校验行为

库内部使用 `binding` 作为校验标签名。

例如：

```go
type PageReq struct {
	Page int `form:"page" binding:"gte=1"`
	Size int `form:"size" binding:"gte=1,lte=100"`
}
```

对于：

- 结构体
- 指针
- 切片
- 数组

都会执行校验。

---

## 推荐写法

在 Mulan Handler 层建议统一这样写：

```go
func (h *Handler) HandleCreate(c *gin.Context) {
	logger := zap.L().With(zap.String("requestid", requestid.Get(c)))

	var _req UserCreateReq
	if err := req.Bind(c, &_req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	ctx := c.Request.Context()
	if err := h.biz.Create(ctx, logger, _req.To()); err != nil {
		c.AbortWithStatusJSON(500, rsp.E(err))
		return
	}

	c.JSON(200, rsp.OK())
}
```

---

## 最佳实践

- 请求 DTO 命名用 `{Entity}{Action}Req`
- 优先在 Handler 层完成请求绑定
- 校验规则尽量写在结构体标签里
- 返回错误时统一用 `rsp.E(...)`
- 业务逻辑不要直接依赖 `gin.Context`
- 同一个结构体要接收多来源参数时，优先使用 `req.Bind`

---

## 适用场景

适合这些接口：

- RESTful JSON 接口
- 带路径参数的详情/更新接口
- Query + Body 混合查询接口
- 需要统一参数校验的 Gin 项目

---

## 小结

如果你在 Mulan 项目里写 Handler，入参处理可以优先用：

- `req.Bind(c, &_req)`：严格绑定，常用
- `req.ShouldBind(c, &_req)`：尽力绑定，适合多来源容错场景

这样可以让 Handler 层更统一，也更容易维护。
