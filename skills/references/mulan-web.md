# Mulan Web

`mulan-web` 是基于 `gin` 的 HTTP 服务模块。  
它实现了 `mulan` 的 `Servicer` 接口，可以直接挂到应用启动器里统一管理。

适合做：

- HTTP API 服务
- 带健康检查和版本信息的微服务
- 统一接入请求日志、RequestID，并按需开启 Metrics、Pprof

## 安装

```bash
go get github.com/mulan-go/mulan/web
```

## 导入

```go
import "github.com/mulan-go/mulan/web"
```

## 核心能力

- 基于 `gin` 构建 HTTP 服务
- 实现 `Serve / Shutdown / Close`
- 自动注册：
  - `/health`
  - `/version`
- 可选开启 `/metrics`
- 仅回环监听地址可选开启 `/pprof`
- 自动接入：
  - Recovery
  - RequestID
  - Zap 请求日志
- 支持统一路由前缀
- 可直接接入 `mulan.App`

---

## 配置

### `Config`

```go
type Config struct {
	Prefix   string `json:"prefix" yaml:"prefix" default:"/"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Host     string `json:"host" yaml:"host" default:"127.0.0.1"`
	Port     int    `json:"port" yaml:"port"`
	Debug    bool   `json:"debug" yaml:"debug"`
	Pprof    bool   `json:"pprof" yaml:"pprof"`
	Metrics  bool   `json:"metrics" yaml:"metrics"`
}
```

### 字段说明

| 字段 | 说明 |
| --- | --- |
| `Prefix` | API 路由前缀，默认 `/` |
| `Endpoint` | 对外访问地址，为空时自动使用 `Host:Port` |
| `Host` | 监听地址 |
| `Port` | 监听端口 |
| `Debug` | 是否开启 Gin Debug 模式 |
| `Pprof` | 是否注册 Pprof；仅 `localhost` 或回环 IP 生效 |
| `Metrics` | 是否注册 `/metrics`；可在任意监听地址生效 |

### 配置方法

```go
func FlagSet(defaultPort int) *pflag.FlagSet
func (c *Config) FlagSet(defaultPort int) *pflag.FlagSet
func (c *Config) Addr() string
func (c *Config) GetEndpoint() string
func (c *Config) WithMetrics(v bool) *Config
```

### 示例

```go
cfg := &web.Config{
	Host:   "0.0.0.0",
	Port:   8080,
	Prefix: "/api/v1",
	Debug:  true,
	Pprof:  false,
	Metrics: true,
}

addr := cfg.Addr()           // 0.0.0.0:8080
endpoint := cfg.GetEndpoint()
_ = addr
_ = endpoint
```

---

## 应用信息

### `Info`

```go
type Info struct {
	Name    string
	Version string
	Commit  string
	BuildAt string
}
```

通常用于 `/version` 和 `/health` 等内置接口。

---

## 服务对象

### `Service`

`Service` 实现了 `mulan/service.Servicer`。

```go
type Service struct {
	// 私有字段
}
```

### 创建服务

```go
func New(conf *Config, info *Info, fn func(gin.IRouter)) *Service
```

参数说明：

- `conf`：HTTP 配置
- `info`：服务信息
- `fn`：路由注册函数

### 生命周期方法

```go
func (s *Service) Build() *Service
func (s *Service) Serve() error
func (s *Service) Shutdown(ctx context.Context) error
func (s *Service) Close() error
```

### 获取底层实例

```go
func (s *Service) Server() *http.Server
func (s *Service) Routes() []gin.RouteInfo
func (s *Service) Engine() *gin.Engine
```

---

## 内置中间件与路由

`Build()` 后会自动挂载这些能力：

### 中间件

- `RecoveryWithZap`
- `requestid.New()`
- `ginzap.GinzapWithConfig(...)`

### 固定接口

- `GET /version`
- `GET /health`

### 可选接口

当 `Metrics = true` 时：

- `GET /metrics`

当 `Pprof = true` 且 `Host` 为 `localhost`、`127.0.0.0/8` 或 `::1` 时：

- `/pprof`

非回环地址启用 `Pprof` 时不会注册路由，并记录警告。`Metrics` 不受该限制。

### 请求日志默认跳过路径

默认会跳过这些路径的请求日志：

- `/health`
- `/version`
- `/metrics`
- `/pprof`

同时会跳过这些方法：

- `OPTIONS`
- `HEAD`
- `TRACE`

---

## 辅助方法

### RequestID

```go
func RequestID(c *gin.Context) string
func ZapRequestID(c *gin.Context) zap.Field
```

### 示例

```go
logger := zap.L().With(web.ZapRequestID(c))
logger.Info("request received")
```

---

## 内置 Handler

### 版本信息

```go
func VersionHandler(info *Info) gin.HandlerFunc
```

返回字符串格式：

```text
{Name} {Version} {Commit}
```

### 错误码列表

```go
func ErrCodeHandler(c *gin.Context)
```

会返回 `rsp.Errors`。

---

## 快速开始

### 最小示例

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/mulan-go/mulan/rsp"
	"github.com/mulan-go/mulan/web"
)

func main() {
	cfg := &web.Config{
		Host:   "0.0.0.0",
		Port:   8080,
		Prefix: "/api",
		Debug:  true,
		Metrics: true,
	}

	info := &web.Info{
		Name:    "demo",
		Version: "0.1.0",
		Commit:  "dev",
	}

	srv := web.New(cfg, info, func(r gin.IRouter) {
		r.GET("/ping", func(c *gin.Context) {
			c.JSON(200, rsp.OK())
		})

		r.GET("/user", func(c *gin.Context) {
			c.JSON(200, rsp.S(gin.H{
				"name": "mulan",
			}))
		})
	})

	if err := srv.Serve(); err != nil {
		panic(err)
	}
}
```

`Serve()` 会阻塞，并直接返回 `ListenAndServe` 的监听错误；正常关闭产生的 `http.ErrServerClosed` 会转换为 `nil`。

---

## 与 `mulan` 集成

`mulan-web` 实现了服务接口，所以可以直接交给 `mulan.App` 托管。

```go
package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/mulan-go/mulan/rsp"
	mulan "github.com/mulan-go/mulan/app"
	"github.com/mulan-go/mulan/web"
)

type Config struct {
	Web web.Config `json:"web" yaml:"web"`
}

func main() {
	conf := &Config{}

	meta := &mulan.Meta{
		Name:    "demo",
		Version: "1.0.0",
		Commit:  "dev",
	}

	info := &web.Info{
		Name:    meta.Name,
		Version: meta.Version,
		Commit:  meta.Commit,
		BuildAt: meta.BuildAt,
	}

	webSrv := web.New(&conf.Web, info, func(r gin.IRouter) {
		r.GET("/ping", func(c *gin.Context) {
			c.JSON(200, rsp.OK())
		})
	})

	app := mulan.New(meta, conf).
		AddService(webSrv)

	if err := app.Execute(context.Background()); err != nil {
		panic(err)
	}
}
```

---

## 路由注册方式

传入 `New(...)` 的回调函数接收的是 `gin.IRouter`。

框架会先按 `Prefix` 创建分组，再调用你的路由注册函数。

例如：

```go
cfg := &web.Config{
	Prefix: "/api/v1",
}
```

你的路由这样写：

```go
srv := web.New(cfg, info, func(r gin.IRouter) {
	r.GET("/ping", handler)
})
```

最终访问路径就是：

```text
/api/v1/ping
```

---

## 超时与安全默认值

内部创建 `http.Server` 时带了默认超时：

- `ReadTimeout = 30s`
- `WriteTimeout = 30s`
- `IdleTimeout = 120s`
- `ReadHeaderTimeout = 10s`
- `MaxHeaderBytes = 1MB`

这有助于减少慢连接带来的风险。

---

## 常见用法

### 1. 获取底层 Gin 引擎

```go
engine := srv.Engine()
_ = engine
```

### 2. 获取注册路由

```go
routes := srv.Routes()
_ = routes
```

### 3. 优雅关闭

```go
ctx := context.Background()
if err := srv.Shutdown(ctx); err != nil {
	panic(err)
}
```

### 4. 强制关闭

```go
if err := srv.Close(); err != nil {
	panic(err)
}
```

---

## 建议

- 业务接口统一使用 `rsp` 返回响应
- 日志里带上 `requestid`
- API 路由统一放到 `Prefix` 下
- 只在回环监听器开启 `Pprof`
- 按监控采集边界显式开启 `Metrics`
- 生产环境关闭 `Debug`

---

## 搭配使用

常见组合：

- `mulan`：应用启动与生命周期管理
- `mulan-web`：HTTP 服务
- `mulan-req`：请求绑定与校验
- `mulan-rsp`：统一响应格式
- `mulan-rdb`：Redis
- `mulan-dbx`：数据库
- `mulan-auth`：认证鉴权
