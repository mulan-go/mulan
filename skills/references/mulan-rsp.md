# Mulan RSP - 统一响应

`mulan-rsp` 用于统一接口成功和失败的返回结构，适合配合 Gin 直接返回 JSON。

## 导入

```go
import "github.com/mulan-go/mulan/rsp"
```

## 作用

- 统一成功响应
- 统一错误响应
- 统一列表响应
- 统一业务错误码
- 避免把内部错误直接暴露给前端

---

## 核心类型

### `AppError`

业务错误对象。

```go
type AppError struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}
```

常用方法：

```go
func (a *AppError) Error() string
func (a *AppError) WithMsg(msg string) *AppError
func (a *AppError) WithError(err error) *AppError
```

说明：

- `WithMsg`：追加自定义说明
- `WithError`：追加原始错误文本
- 适合在参数校验、业务判断失败时返回

---

### `Rsp[T]`

通用响应结构。

```go
type Rsp[T any] struct {
	*AppError `json:",inline"`
	Data      T `json:"data"`
}
```

特点：

- 成功和失败都走统一结构
- `code` 和 `msg` 来自 `AppError`
- `data` 为返回数据

---

### `Items`

列表响应数据结构。

```go
type Items struct {
	Items any   `json:"items"`
	Ext   any   `json:"ext,omitempty"`
	Total int64 `json:"total"`
}
```

适合分页、列表查询。

---

## 成功响应 API

### 返回单个对象

```go
rsp.S(data)
```

示例：

```go
c.JSON(200, rsp.S(user))
```

返回结构大致为：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1,
    "name": "tom"
  }
}
```

---

### 返回空成功

```go
rsp.OK()
```

示例：

```go
c.JSON(200, rsp.OK())
```

适合：

- 创建成功
- 更新成功
- 删除成功
- 不需要返回数据

---

### 返回自定义成功消息

```go
rsp.M("操作成功")
```

示例：

```go
c.JSON(200, rsp.M("保存成功"))
```

---

## 错误响应 API

### 返回业务错误

```go
rsp.E(err)
```

这里的 `err` 要是 `*rsp.AppError`。

示例：

```go
c.JSON(200, rsp.E(rsp.ErrParamInvalid))
```

或：

```go
c.JSON(200, rsp.E(rsp.ErrParamInvalid.WithMsg("name 不能为空")))
```

---

### 返回内部错误

```go
rsp.C(err)
```

特点：

- 会记录原始错误日志
- 不把内部错误内容直接返回给前端
- 前端只会拿到统一的内部错误响应

示例：

```go
if err != nil {
	c.JSON(200, rsp.C(err))
	return
}
```

适合：

- 数据库错误
- 网络错误
- 第三方服务错误
- 系统内部异常

---

### 返回带自定义提示的内部错误

```go
rsp.CWithMsg(err, "系统繁忙，请稍后再试")
```

特点：

- 日志里保留原始错误
- 给用户返回更友好的提示
- 适合不想暴露底层错误细节的场景

示例：

```go
if err != nil {
	c.JSON(200, rsp.CWithMsg(err, "保存失败，请稍后重试"))
	return
}
```

---

## 列表响应 API

### 基础列表

```go
rsp.Item(total, items)
```

示例：

```go
c.JSON(200, rsp.Item(total, list))
```

返回结构大致为：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [],
    "total": 100
  }
}
```

---

### 带扩展字段的列表

```go
rsp.ItemExt(total, items, ext)
```

示例：

```go
c.JSON(200, rsp.ItemExt(total, list, map[string]any{
	"page":     1,
	"pageSize": 20,
}))
```

适合：

- 分页信息
- 统计信息
- 额外聚合字段

---

### 空列表响应

```go
rsp.ItemNone()
```

说明：

- 返回空列表结构
- 默认使用对应错误码
- 适合“未找到列表数据”的统一输出场景

---

## ID 响应类型

库里还提供了一个简单 ID 结构：

```go
type IDRsp[T uint64 | string] struct {
	ID   T      `json:"id,omitempty"`
	UUID string `json:"uuid,omitempty"`
}
```

示例：

```go
c.JSON(200, rsp.S(rsp.IDRsp[uint64]{ID: 1001}))
```

适合：

- 创建资源后返回 ID
- 同时返回数值 ID 和 UUID

---

## 错误码定义

`mulan-rsp` 内部使用 `AppError` 管理错误码，并维护一个全局错误表。

可用方式：

```go
rsp.New(system, biz, code, msg)
```

说明：

- 用于注册新的业务错误
- 如果错误码重复，会直接 panic
- 适合在公共错误码文件中集中定义

建议做法：

```go
var ErrUserNotFound = rsp.New(1, 1, 1, "用户不存在")
```

> 实际 `system`、`biz` 的取值应按项目里的错误码规划统一管理。

---

## 推荐用法

### 1. Handler 层统一返回

```go
func (h *Handler) Detail(c *gin.Context) {
	data, err := h.biz.Detail(c.Request.Context(), 1)
	if err != nil {
		c.JSON(200, rsp.C(err))
		return
	}
	c.JSON(200, rsp.S(data))
}
```

---

### 2. 参数错误走业务错误

```go
if err := req.Bind(c, &_req); err != nil {
	c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
	return
}
```

---

### 3. 列表接口统一格式

```go
list, total, err := h.biz.List(ctx, query)
if err != nil {
	c.JSON(200, rsp.C(err))
	return
}
c.JSON(200, rsp.Item(total, list))
```

---

## 最佳实践

- 成功响应统一用 `rsp.S`、`rsp.OK`
- 列表响应统一用 `rsp.Item`
- 业务可预期错误统一用 `rsp.E`
- 内部异常统一用 `rsp.C`
- 不要把数据库、SQL、堆栈等内部错误直接返回给前端
- 错误码尽量集中定义，不要在各处随意写死

---

## 常见搭配

通常和下面模块一起使用：

- `mulan-req`：请求绑定和校验
- `mulan-web`：HTTP 服务
- `mulan`：应用启动和服务管理

示例：

```go
var _req UserCreateReq
if err := req.Bind(c, &_req); err != nil {
	c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
	return
}

user, err := biz.Create(c.Request.Context(), _req)
if err != nil {
	c.AbortWithStatusJSON(500, rsp.C(err))
	return
}

c.JSON(200, rsp.S(user))
```
