# Mulan Auth - 认证扩展

`mulan-auth` 提供四类认证能力：

- `apikey`：简单 API Key 鉴权
- `session`：会话认证，支持内存、文件、Redis 存储
- `oauth2`：Authorization Code + PKCE、state 校验、UserInfo、Session 映射
- `oidc`：Discovery、ID Token/JWKS 校验、nonce、UserInfo、Session 映射

适合用在：

- 内部服务接口鉴权
- 管理后台登录态
- 基于 Cookie / Header 的登录认证
- 需要角色判断的接口保护
- 第三方 OAuth2 登录
- 标准 OpenID Connect 单点登录

---

## 导入

### API Key

```go
import "github.com/mulan-go/mulan/auth/apikey"
```

### Session

```go
import "github.com/mulan-go/mulan/auth/session"
```

### OAuth2 / OIDC

```go
import "github.com/mulan-go/mulan/auth/oauth2"
import "github.com/mulan-go/mulan/auth/oidc"
```

---

## 目录

1. `apikey` - API Key 鉴权
2. `session` - 会话认证
3. `oauth2` - 通用 OAuth2 授权码登录
4. `oidc` - 带 ID Token 验证的 OIDC 登录

---

## apikey

`apikey` 适合简单场景。  
它会从请求头、Cookie、`Authorization: Bearer xxx` 中取值并校验。

### 配置

```go
type Config struct {
	Name   string   `json:"name" yaml:"name"`
	Value  string   `json:"value" yaml:"value"`
	Values []string `json:"values" yaml:"values"`
}
```

### 字段说明

| 字段     | 说明                    |
| -------- | ----------------------- |
| `Name`   | Key 名称，默认 `apikey` |
| `Value`  | 单个 Key                |
| `Values` | 多个可用 Key            |

### API

```go
func FlagSet() *pflag.FlagSet
func (c *Config) FlagSet() *pflag.FlagSet
func Mw(cfg *Config) gin.HandlerFunc
```

### 行为说明

中间件会按下面顺序取值：

1. Header：`{Name}`
2. Cookie：`{Name}`
3. Header：`Authorization: Bearer xxx`

如果命中任意一个合法 Key，则放行；否则返回 `401`。

如果 `Value` 和 `Values` 都为空，中间件会直接放行；不要把空配置当成已启用鉴权。

### 最小示例

```go
r.Use(apikey.Mw(&apikey.Config{
	Name:   "X-Api-Key",
	Values: []string{"key1", "key2"},
}))
```

### 配置参数

`FlagSet()` 会生成：

```text
apikey.name
apikey.value
apikey.values
```

---

## session

`session` 提供完整的会话认证能力。

支持：

- 自动创建 / 读取会话
- 从 Header / Cookie 提取 token
- 可配置为只接受 Header / Bearer token
- 把用户信息写入当前请求上下文
- 登录态校验
- 角色校验
- 多种存储驱动

---

## Session 配置

```go
type Config struct {
	Name       string     `json:"name" yaml:"name"`
	TTL        int        `json:"ttl" yaml:"ttl"`
	Driver     string     `json:"driver" yaml:"driver"`
	RDB        rdb.Config `json:"rdb" yaml:"rdb"`
	Dir        string     `json:"dir" yaml:"dir"`
	HeaderOnly bool       `json:"header_only" yaml:"header_only"`
}
```

`TTL` 和各 Store 的 `maxAge` 单位均为秒。`HeaderOnly = true` 时：

- 仍接受 `X-Token`、`X-Api-Key`、配置名、`X-{配置名}` 和 `Authorization: Bearer ...`
- 不读取 Cookie
- 返回 `X-Token`
- 不写 `Set-Cookie`

命令行开关是 `--session.header-only`；Viper 配置键通常使用 `session.header_only`，组合配置时需要注册别名。

---

## 核心常量

```go
const (
	DefaultKey = "github.com/mulan-go/mulan/auth"
	TokenKey   = "github.com/mulan-go/mulan/auth/session"

	CtxKeyID      = "id"
	CtxKeyAccount = "account"
	CtxKeyState   = "state"
	CtxKeyRoles   = "roles"
	CtxKeyIsAdmin = "is_admin"
	RoleAdmin     = "admin"
)
```

常见用途：

- `session.Default(c)` 取当前 Session
- 从 `gin.Context` 读取用户 ID、账号、角色
- 判断管理员角色

---

## Session 对象

### 主要方法

```go
func Default(c *gin.Context) *Session
func NewSession(ctx context.Context, store Store, data Data, maxAge ...int) *Session
```

### `Session` 常用操作

```go
sess.Token() string

sess.SetMaxAge(v int)
sess.SetSecure(v bool)
sess.SetHttpOnly(v bool)

sess.ID() uint64
sess.Account() string
sess.State() uint16
sess.Roles() []string
sess.HasRole(role string) bool

sess.Get(key string) any
sess.Set(key string, val any)
sess.SetID(val uint64)
sess.SetAccount(val string)
sess.SetState(val uint16)
sess.SetRoles(roles []string)
sess.SetValues(key string, val any)

sess.Save(lifetime ...time.Duration) error
sess.Delete(key string) error
sess.Clear() error
```

### 说明

- `Save(...)`：保存当前会话
- `Clear()`：清空旧会话并生成新 token
- `Delete(key)`：删除指定扩展字段
- `SetID / SetAccount / SetRoles`：设置用户身份信息
- `Token()`：拿当前 token
- 请求结束后中间件总会把 token 写到 `X-Token`
- 非 Header-only 模式还会写 Cookie：`{name}`

---

## Store 接口

会话存储由 `Store` 抽象出来。

```go
type Store interface {
	Clear(ctx context.Context, token string) error
	Get(ctx context.Context, token string) (Data, error)
	Save(ctx context.Context, data Data, lifetime ...time.Duration) error
}
```

---

## Data 接口

会话数据对象需要实现 `Data`。

```go
type Data interface {
	Token() string
	SetToken(string) Data

	ID() uint64
	SetID(uint64) Data

	Account() string
	SetAccount(string) Data

	State() uint16
	SetState(uint16) Data

	Roles() []string
	SetRoles([]string) Data

	Items() DataMap
	Get(key string) any
	Set(key string, value any) Data
	SetValues(key string, value any) Data
	Delete(key string) Data
	Clear() Data
	New() string
}
```

通常默认实现就够用，不需要自己手写。

---

## 中间件初始化

### 方式一：直接创建中间件

```go
func Mw(name string, store Store, data ...Data) gin.HandlerFunc
```

适合你自己决定存储实现。该入口保持 Cookie 兼容，不读取 `Config.HeaderOnly`；纯 Header 模式必须使用 `Init(&Config{HeaderOnly: true})`。

### 方式二：按配置初始化

```go
func Init(cfg *Config) (gin.HandlerFunc, error)
```

它会根据 `cfg.Driver` 自动创建对应 Store。

已支持：

- `mem` / 默认：内存
- `fs`：文件系统
- `rdb`：Redis

---

## 登录态与角色校验

常用中间件与工具函数：

```go
session.AuthMW() gin.HandlerFunc
session.RoleMW(roles ...string) gin.HandlerFunc

session.HasRole(c *gin.Context, role string) bool
session.HasRoles(c *gin.Context, roles ...string) bool
```

用途：

- `AuthMW()`：要求已登录
- `RoleMW("admin")`：要求具备指定角色
- `HasRole(...)`：代码里做角色判断

---

## 存储实现

### 1. 内存存储

适合：

- 单机开发
- 本地调试
- 无状态要求不高的服务

常见创建方式：

```go
store := session.NewMemStore()
```

或带 TTL：

```go
store := session.NewMemStore(24 * 60 * 60)
```

---

### 2. 文件存储

适合：

- 本地开发
- 简单持久化场景

```go
store, err := session.NewFsStore("./data/session", 24*60*60)
if err != nil {
	panic(err)
}
```

---

### 3. Redis 存储

适合：

- 多实例部署
- 分布式会话
- 共享登录态

```go
import (
	"github.com/mulan-go/mulan/auth/session"
	"github.com/mulan-go/mulan/rdb"
)

client, err := rdb.New(&rdb.Config{
	Host: "127.0.0.1",
	Port: 6379,
	DB:   0,
})
if err != nil {
	panic(err)
}

store, err := session.NewRedisStore(client, 24*60*60)
if err != nil {
	panic(err)
}
```

---

## 推荐用法

### 1. 初始化 Session 中间件

```go
store := session.NewMemStore()

r.Use(session.Mw("token", store))
```

或使用配置方式：

```go
mw, err := session.Init(&session.Config{
	Name:       "token",
	Driver:     "rdb",
	TTL:        24 * 60 * 60,
	HeaderOnly: true,
	RDB: rdb.Config{
		Host: "127.0.0.1",
		Port: 6379,
		DB:   0,
	},
})
if err != nil {
	panic(err)
}

r.Use(mw)
```

---

### 2. 登录时写入会话

```go
func handleLogin(c *gin.Context) {
	sess := session.Default(c)

	sess.SetID(user.ID)
	sess.SetAccount(user.Account)
	sess.SetState(user.State)
	sess.SetRoles([]string{"user"})

	if err := sess.Save(24 * time.Hour); err != nil {
		c.JSON(500, rsp.C(err))
		return
	}

	c.JSON(200, rsp.S(gin.H{
		"token": sess.Token(),
	}))
}
```

---

### 3. 退出登录

```go
func handleLogout(c *gin.Context) {
	sess := session.Default(c)
	if err := sess.Clear(); err != nil {
		c.JSON(500, rsp.C(err))
		return
	}
	c.JSON(200, rsp.OK())
}
```

---

### 4. 读取当前登录用户

```go
func handleProfile(c *gin.Context) {
	sess := session.Default(c)

	c.JSON(200, rsp.S(gin.H{
		"id":      sess.ID(),
		"account": sess.Account(),
		"state":   sess.State(),
		"roles":   sess.Roles(),
	}))
}
```

---

### 5. 保护路由

```go
auth := r.Group("/api")
auth.Use(session.AuthMW())
{
	auth.GET("/profile", handleProfile)
	auth.POST("/logout", handleLogout)
}
```

管理员路由：

```go
admin := r.Group("/admin")
admin.Use(session.AuthMW(), session.RoleMW("admin"))
{
	admin.GET("/users", handleUserList)
}
```

---

## 完整示例

```go
package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mulan-go/mulan/auth/session"
	"github.com/mulan-go/mulan/rsp"
)

func main() {
	r := gin.Default()

	store := session.NewMemStore(24 * 60 * 60)
	r.Use(session.Mw("token", store))

	r.POST("/login", func(c *gin.Context) {
		sess := session.Default(c)
		sess.SetID(1001)
		sess.SetAccount("admin")
		sess.SetRoles([]string{"admin"})

		if err := sess.Save(24 * time.Hour); err != nil {
			c.JSON(500, rsp.C(err))
			return
		}

		c.JSON(200, rsp.S(gin.H{
			"token": sess.Token(),
		}))
	})

	auth := r.Group("/api")
	auth.Use(session.AuthMW())
	{
		auth.GET("/profile", func(c *gin.Context) {
			sess := session.Default(c)
			c.JSON(200, rsp.S(gin.H{
				"id":      sess.ID(),
				"account": sess.Account(),
				"roles":   sess.Roles(),
			}))
		})
	}

	admin := r.Group("/admin")
	admin.Use(session.AuthMW(), session.RoleMW("admin"))
	{
		admin.GET("/dashboard", func(c *gin.Context) {
			c.JSON(200, rsp.OK())
		})
	}

	_ = r.Run(":8080")
}
```

---

## OAuth2

```go
type Config struct {
	ClientID               string
	ClientSecret           string
	RedirectURL            string
	AuthURL                string
	TokenURL               string
	UserInfoURL            string
	Scopes                 []string
	AuthStyle              xoauth2.AuthStyle
	CookieName             string
	CookieSecret           string
	CookieSecure           bool
	StateTTL               int
	SuccessURL             string
	AllowInsecureEndpoints bool
}
```

最小授权码登录：

```go
client, err := oauth2.New(&oauth2.Config{
	ClientID:     "client-id",
	ClientSecret: "client-secret",
	RedirectURL:  "https://app.example.com/auth/oauth2/callback",
	AuthURL:      "https://provider.example.com/oauth/authorize",
	TokenURL:     "https://provider.example.com/oauth/token",
	UserInfoURL:  "https://provider.example.com/oauth/userinfo",
	Scopes:       []string{"profile", "email"},
	CookieSecret: "至少 32 字节的独立随机密钥........",
	SuccessURL:   "/",
})
if err != nil {
	panic(err)
}

r.Use(session.Mw("token", session.NewMemStore()))
r.GET("/auth/oauth2/login", client.LoginHandler())
r.GET("/auth/oauth2/callback", client.SessionCallback())
```

内置行为：

- Authorization Code + PKCE S256
- HMAC 签名、带过期时间的 state Cookie
- 常量时间 state 比较
- Provider 错误与 Token Exchange 错误归类
- UserInfo 响应限制为 1 MiB
- `return_to` 只接受站内绝对路径，外部 URL 回退到 `SuccessURL`
- HTTPS 回调自动把 state Cookie 标记为 `Secure`

默认 Session 映射从 UserInfo 读取：

- subject：`sub`、`id`
- account：`preferred_username`、`login`、`username`、`email`、`name`
- roles：`roles`、`groups`

Provider claims 不符合该约定时传入自定义 `oauth2.SessionMapper`：

```go
r.GET("/auth/oauth2/callback", client.SessionCallback(
	func(c *gin.Context, identity *oauth2.Identity, sess *session.Session) error {
		sess.SetAccount(identity.UserInfo["email"].(string))
		return nil
	},
))
```

只需要 Token、不创建 Session 时，使用 `Exchange`、`Authenticate` 或 `CallbackHandler`。

## OIDC

```go
type Config struct {
	IssuerURL           string
	ClientID            string
	ClientSecret        string
	RedirectURL         string
	Scopes              []string
	CookieName          string
	CookieSecret        string
	CookieSecure        bool
	StateTTL            int
	SuccessURL          string
	FetchUserInfo       bool
	AllowInsecureIssuer bool
}
```

最小接入：

```go
authenticator, err := oidc.New(context.Background(), &oidc.Config{
	IssuerURL:     "https://accounts.example.com",
	ClientID:      "client-id",
	ClientSecret:  "client-secret",
	RedirectURL:   "https://app.example.com/auth/oidc/callback",
	CookieSecret:  "至少 32 字节的独立随机密钥........",
	Scopes:        []string{"openid", "profile", "email"},
	SuccessURL:    "/",
	FetchUserInfo: true,
})
if err != nil {
	panic(err)
}

r.Use(session.Mw("token", session.NewMemStore()))
r.GET("/auth/oidc/login", authenticator.LoginHandler())
r.GET("/auth/oidc/callback", authenticator.SessionCallback())
```

OIDC 在 OAuth2 流程上增加：

- Provider Discovery
- ID Token 签名、issuer、audience、有效期验证
- nonce 验证
- 可选 UserInfo 获取
- UserInfo `sub` 与 ID Token `sub` 一致性校验

默认 Session 映射优先读取已验证的 ID Token claims，并补充 UserInfo 中未重复的 claims。非标准 claims 使用自定义 `oidc.SessionMapper`。

登录地址支持站内回跳：

```text
/auth/oidc/login?return_to=/settings
```

## OAuth2 / OIDC 安全约束

- `CookieSecret` 至少 32 字节，且不得复用业务密钥
- Provider endpoint 默认必须使用 HTTPS；HTTP 只允许 loopback
- `AllowInsecureEndpoints`、`AllowInsecureIssuer` 只用于受控本地开发
- `SessionCallback` 必须挂在 Session 中间件之后
- Header-only Session 不影响 OAuth2/OIDC 的一次性 state Cookie；两者用途不同
- 登录成功时 `SessionCallback` 会先轮换 Session，再写入身份

## 最佳实践

- 简单服务间鉴权优先用 `apikey`
- 用户登录态优先用 `session`
- 通用第三方 Provider 用 `oauth2`
- 支持 Discovery 和 ID Token 的 Provider 优先用 `oidc`
- 单机开发可用内存存储
- 生产环境优先 Redis 存储
- Handler 层只做会话读取和返回
- 业务层不要依赖 `gin.Context`
- 角色名统一管理，避免到处写死
- 登出时调用 `sess.Clear()`
- 浏览器 Cookie Session 需要开启：
  - `sess.SetSecure(true)`
  - `sess.SetHttpOnly(true)`
- 纯 API 使用 `HeaderOnly: true`

---

## 常见组合

`mulan-auth` 常和这些模块一起用：

- `mulan-web`：HTTP 服务
- `mulan-req`：请求绑定
- `mulan-rsp`：统一响应
- `mulan-rdb`：Redis 客户端
- `mulan-passwd`：密码加密与验证
- `oauth2` / `oidc`：第三方登录与单点登录

例如登录流程通常是：

1. `req.Bind(...)` 绑定登录参数
2. `passwd.Verify(...)` 校验密码
3. `session.Default(c)` 获取会话
4. `sess.SetID(...) / SetRoles(...)`
5. `sess.Save(...)`
6. `rsp.S(...)` 返回结果

---

## 小结

`mulan-auth` 的能力边界：

- `apikey`：轻量鉴权
- `session`：完整登录态管理
- `oauth2`：通用授权码登录
- `oidc`：标准身份验证与 claims 映射

如果你在做 Mulan 生态下的 HTTP 服务：

- 外部或内部接口简单鉴权，用 `apikey`
- 用户登录、权限控制、角色校验，用 `session`
- 企业身份源优先用 `oidc`，非 OIDC Provider 再用 `oauth2`
