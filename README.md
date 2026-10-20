# Mulan 木兰

Mulan 是一组面向 Go 服务开发的轻量组件，覆盖应用启动、配置加载、HTTP 服务、认证、数据库、日志和远程配置。各模块可以独立使用，也可以通过 `app` 统一管理服务生命周期。

本仓库采用 Go 多模块结构，由根目录的 [`go.work`](go.work) 管理本地开发；每个模块都有独立的 `go.mod` 和版本。

## 模块

模块导入路径统一为 `github.com/mulan-go/mulan/<模块名>`。

| 模块 | 能力 | 文档 |
| --- | --- | --- |
| [`app`](app) | 基于 Cobra、Viper 和 Zap 的应用启动、配置加载、CLI 与服务生命周期管理 | [使用指南](app/README.md) |
| [`web`](web) | Gin HTTP 服务、请求日志、Request ID、健康检查与优雅关闭 | [使用指南](skills/references/mulan-web.md) |
| [`auth`](auth) | API Key、Session、OAuth2 和 OIDC 认证组件 | [使用指南](auth/README.md) |
| [`captcha`](captcha) | 算式图片验证码与干扰效果 | [使用指南](skills/references/mulan-captcha.md) |
| [`dbx`](dbx) | 基于 GORM 的数据库接入，支持 MySQL、PostgreSQL 和 SQLite | [使用指南](dbx/README.md) |
| [`heartbeat`](heartbeat) | HTTP / WebSocket 心跳，支持内存和 Redis 存储 | [使用指南](heartbeat/README.md) |
| [`log`](log) | 基于 Zap 的日志，支持控制台、滚动文件和 HTTP 输出 | [使用指南](log/README.md) |
| [`passwd`](passwd) | bcrypt 密码哈希与校验、密码字符组合检查 | [使用指南](skills/references/mulan-passwd.md) |
| [`rdb`](rdb) | Redis 客户端初始化、配置与调试日志 | [使用指南](skills/references/mulan-rdb.md) |
| [`req`](req) | Gin 请求参数绑定、默认值与校验 | [使用指南](skills/references/mulan-req.md) |
| [`rsp`](rsp) | 统一响应结构、业务错误码与列表响应 | [使用指南](skills/references/mulan-rsp.md) |
| [`vault`](vault) | Vault AppRole 登录、KV v2 配置读取与注入、Token 自动续期 | [使用指南](skills/references/mulan-vault.md) |

## 安装

本仓库的开发环境使用 **Go 1.26.5 或更高版本**，具体要求见各模块的 `go.mod`。

在已有 Go 项目中按需安装模块：

```bash
go get github.com/mulan-go/mulan/app
go get github.com/mulan-go/mulan/web
```

`app` 模块的 Go 包名为 `mulan`，可以显式设置导入别名：

```go
import mulan "github.com/mulan-go/mulan/app"
```

## 快速开始

创建一个 HTTP 服务项目：

```bash
mkdir mulan-demo
cd mulan-demo
go mod init example.com/mulan-demo
go get github.com/mulan-go/mulan/app github.com/mulan-go/mulan/web
```

新建 `main.go`：

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	mulan "github.com/mulan-go/mulan/app"
	"github.com/mulan-go/mulan/web"
)

type Config struct {
	Web web.Config `json:"web" yaml:"web"`
}

func main() {
	cfg := &Config{}
	meta := &mulan.Meta{
		Name:        "demo",
		Description: "Mulan HTTP demo",
		Version:     "dev",
	}

	app := mulan.New(meta, cfg).
		AddFlagSet(web.FlagSet(8080)).
		BindFlags()

	app.AddService(web.New(&cfg.Web, &web.Info{
		Name:    meta.Name,
		Version: meta.Version,
	}, func(router gin.IRouter) {
		router.GET("/hello", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Hello, Mulan!"})
		})
	}))

	if err := app.Execute(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

启动服务，默认监听 `127.0.0.1:8080`：

```bash
go run .
```

在另一个终端访问：

```bash
curl http://127.0.0.1:8080/hello
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/version
```

`app` 负责配置加载和服务启动，并在收到 `SIGTERM` 时调用服务的 `Shutdown`，收到中断信号（如 Ctrl+C）时调用 `Close`。自定义服务只需实现 `Serve() error`、`Shutdown(context.Context) error` 和 `Close() error`。

## 配置

`app` 支持配置文件、环境变量和命令行参数，并在创建应用时尝试加载当前目录下的 `.env`。

上述示例可以使用以下 `config.yaml`：

```yaml
web:
  host: 127.0.0.1
  port: 8080
  prefix: /
```

```bash
# 指定配置文件；未指定时查找当前目录下的 config.*
go run . --config config.yaml

# 通过绑定的命令行参数覆盖端口
go run . --web.port 9090

# 环境变量前缀来自应用名 demo，配置键中的点和连字符转为下划线
DEMO_WEB_PORT=9090 go run .

# 查看应用版本
go run . version
```

扩展启动逻辑可以使用 `SetPreInit`；它在配置加载完成后、服务启动前执行。需要远程配置时，可按 [Vault 使用指南](skills/references/mulan-vault.md) 接入。

## 本地开发

```bash
git clone https://github.com/mulan-go/mulan.git
cd mulan
```

根目录是 Go workspace，测试命令应在对应模块中执行。例如：

```bash
cd vault
go test -race ./...
```

从仓库根目录测试所有模块：

```bash
for module in app auth captcha dbx heartbeat log passwd rdb req rsp vault web; do
  (cd "$module" && go test ./...) || exit 1
done
```

部分 Redis 集成测试需要可用的 Redis 服务，可通过 `REDIS_ADDR` 配置地址；服务不可用时，这些测试会跳过。

各模块独立发布，Git 标签使用 `<模块名>/vX.Y.Z` 格式，例如 `app/vX.Y.Z`。本地跨模块开发由 `go.work` 关联工作区内的源码。

更多开发约定见 [`AGENTS.md`](AGENTS.md)，模块参考文档位于 [`skills/references`](skills/references)。许可证信息见模块目录中的 `LICENSE` 文件，例如 [`app/LICENSE`](app/LICENSE)。
