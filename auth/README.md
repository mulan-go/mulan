# Mulan Ext Auth

Gin 认证组件：

- `apikey`：Header、Cookie、Bearer API Key
- `session`：内存、文件、Redis Session
- `oauth2`：Authorization Code + PKCE、state 校验、UserInfo、Session 映射
- `oidc`：Discovery、ID Token/JWKS 校验、nonce、UserInfo、Session 映射

## OAuth2

```go
import (
	"github.com/gin-gonic/gin"
	"github.com/mulan-go/mulan/auth/oauth2"
	"github.com/mulan-go/mulan/auth/session"
)

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

r := gin.New()
r.Use(session.Mw("token", session.NewMemStore()))
r.GET("/auth/oauth2/login", client.LoginHandler())
r.GET("/auth/oauth2/callback", client.SessionCallback())
```

默认 Session 映射从 UserInfo 读取：

- subject：`sub`、`id`
- account：`preferred_username`、`login`、`username`、`email`、`name`
- roles：`roles`、`groups`

非标准 Provider 可传入自定义 `oauth2.SessionMapper`。只需要 Token 时，使用 `CallbackHandler` 或 `Exchange`。

## OIDC

```go
import (
	"context"

	"github.com/mulan-go/mulan/auth/oidc"
)

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

r.GET("/auth/oidc/login", authenticator.LoginHandler())
r.GET("/auth/oidc/callback", authenticator.SessionCallback())
```

OIDC 回调会验证 issuer、audience、签名、有效期和 nonce。启用 `FetchUserInfo` 后还会校验 UserInfo `sub` 与 ID Token 一致。默认 Session 映射读取 ID Token claims；非标准 claims 可传入自定义 `oidc.SessionMapper`。

登录地址支持站内回跳：

```text
/auth/oidc/login?return_to=/settings
```

外部 URL 会被拒绝，回退到 `SuccessURL`。

## 安全配置

- `CookieSecret` 必须是至少 32 字节的独立随机值。
- HTTPS 回调会自动设置 state Cookie 的 `Secure`；也可强制启用 `CookieSecure`。
- Provider endpoint 默认要求 HTTPS；仅 loopback 地址可直接使用 HTTP。
- `AllowInsecureEndpoints` / `AllowInsecureIssuer` 只用于受控开发环境。
- OAuth2/OIDC 回调必须挂在 `session.Mw` 之后，`SessionCallback` 才能创建登录态。

## Session Store

纯 API 服务应使用 Header-only Session，禁止浏览器 Cookie 作为环境凭据：

```go
middleware, err := session.Init(&session.Config{
	Driver:     "memory",
	HeaderOnly: true,
})
```

该模式仅从 Header/Bearer Token 提取 Session，并且不写入 `Set-Cookie`。

- [x] Memory
- [x] Filesystem
- [x] Redis
