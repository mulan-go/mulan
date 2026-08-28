# Mulan - 启动框架参考

`mulan` 是一个 Go 应用启动框架，主要解决这些问题：

- 命令行入口统一
- 配置文件加载
- 环境变量覆盖配置
- 日志初始化
- 多服务生命周期管理
- 优雅关闭

适合拿来做服务程序的启动层，比如：

- HTTP 服务
- 多服务组合启动
- 带配置和环境变量的微服务入口
- 需要 `Serve / Shutdown / Close` 统一管理的程序

---

## 安装

```go
go get github.com/mulan-go/mulan/app
```

---

## 导入

```go
import "github.com/mulan-go/mulan/app"
```

---

## 核心类型

### Meta

应用自身信息。

```go
type Meta struct {
    ID          string
    Name        string
    CNName      string
    Description string
    Version     string
    Commit      string
    BuildAt     string
}
```

常用字段说明：

- `ID`：应用唯一标识
- `Name`：应用名
- `CNName`：中文名
- `Description`：应用描述
- `Version`：版本号
- `Commit`：提交号
- `BuildAt`：构建时间

这些信息会用于 `version` 命令和运行期展示。

---

### App

启动入口对象。

```go
type App struct {
    *Meta
}
```

它负责：

- 管理根命令
- 加载配置
- 绑定环境变量
- 注册服务
- 执行启动逻辑
- 处理退出流程

---

### Servicer

接入 `App` 的服务需要实现这个接口：

```go
type Servicer interface {
    Serve() error
    Shutdown(context.Context) error
    Close() error
}
```

说明：

- `Serve()`：启动服务
- `Shutdown(ctx)`：优雅关闭
- `Close()`：强制关闭

---

## 主要 API

### 回调类型

```go
type ActionFunc func(cmd *cobra.Command, args []string) error
type PreInitFunc func(context.Context) error
```

`PreInitFunc` 在根命令执行前、配置加载完成后调用。

### 创建应用

```go
mulan.New(meta *Meta, cfg any) *App
```

示例：

```go
meta := &mulan.Meta{
    ID:          "com.example.demo",
    Name:        "demo",
    CNName:      "示例服务",
    Description: "demo service",
    Version:     "1.0.0",
    Commit:      "dev",
    BuildAt:     "2026-04-09T18:47:14+08:00",
}

app := mulan.New(meta, config.Conf)
```

---

### 链式配置

```go
app.SetPreInit(f PreInitFunc) *App
app.SetLogger(log *zap.Logger) *App
app.SetVersionCmd(cmd *cobra.Command) *App
app.AddService(srvs ...service.Servicer) *App
app.AddCommand(cmd ...*cobra.Command) *App
app.AddFlagSet(fs ...*pflag.FlagSet) *App
app.BindFlags() *App
```

说明：

- `SetPreInit(...)`：设置启动前初始化逻辑
- `SetLogger(...)`：设置自定义日志
- `SetVersionCmd(...)`：替换已注册的内置 `version` 命令
- `AddService(...)`：注册一个或多个服务
- `AddCommand(...)`：添加自定义命令
- `AddFlagSet(...)`：添加参数集
- `BindFlags()`：绑定参数到配置

---

### 执行启动

```go
app.Execute(ctx context.Context, action ...ActionFunc) error
```

通常这是主函数的最后一步。

---

### 获取对象

```go
app.RootCmd() *cobra.Command
app.Conf() any
```

说明：

- `RootCmd()`：拿到根命令，适合继续扩展 CLI
- `Conf()`：拿到当前配置对象

---

## 快速开始

最小示例：

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/mulan-go/mulan/app"
)

type Config struct {
    Web WebConfig `json:"web" yaml:"web"`
}

type WebConfig struct {
    Port int `json:"port" yaml:"port"`
}

type WebService struct {
    engine *gin.Engine
    server *http.Server
    cfg    *WebConfig
}

func NewWebService(cfg *WebConfig) *WebService {
    engine := gin.Default()
    engine.GET("/", func(c *gin.Context) {
        c.String(200, "Hello, Mulan!")
    })

    return &WebService{
        engine: engine,
        server: &http.Server{
            Addr:    fmt.Sprintf(":%d", cfg.Port),
            Handler: engine,
        },
        cfg: cfg,
    }
}

func (w *WebService) Serve() error {
    err := w.server.ListenAndServe()
    if errors.Is(err, http.ErrServerClosed) {
        return nil
    }
    return err
}

func (w *WebService) Shutdown(ctx context.Context) error {
    shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return w.server.Shutdown(shutdownCtx)
}

func (w *WebService) Close() error {
    return w.server.Close()
}

func main() {
    conf := &Config{}

    meta := &mulan.Meta{
        ID:          "com.example.demo",
        Name:        "demo",
        CNName:      "示例服务",
        Description: "demo service",
        Version:     "1.0.0",
        Commit:      "dev",
        BuildAt:     time.Now().Format(time.RFC3339),
    }

    app := mulan.New(meta, conf)

    webSrv := NewWebService(&conf.Web)
    app.AddService(webSrv)

    if err := app.Execute(context.Background()); err != nil {
        panic(err)
    }
}
```

---

## 常见用法

### 1. 注册多个服务

```go
app.AddService(webSrv, grpcSrv, jobSrv)
```

适合：

- HTTP 服务
- 定时任务服务
- 消息消费服务
- 内部管理服务

---

### 2. 启动前初始化

```go
app := mulan.New(meta, config.Conf).
    SetPreInit(func(ctx context.Context) error {
        // 初始化数据库、缓存、依赖注入等
        return nil
    })
```

适合放这些逻辑：

- 初始化数据库
- 初始化 Redis
- 初始化日志
- 加载全局资源
- 注册内部组件

---

### 3. 添加命令行参数

```go
app.AddFlagSet(config.FlagSet()...)
```

通常会把各模块自己的参数集合并进来。

---

### 4. 添加自定义命令

```go
cmd := &cobra.Command{
    Use:   "seed",
    Short: "初始化测试数据",
    RunE: func(cmd *cobra.Command, args []string) error {
        return nil
    },
}

app.AddCommand(cmd)
```

适合做这些命令：

- 数据初始化
- 管理员创建
- 数据修复
- 本地调试命令

---

## 配置加载规则

框架会按下面顺序处理配置：

1. 命令行参数指定的配置文件
2. 当前目录默认配置文件 `config.*`
3. 环境变量覆盖

常用参数：

```text
-c, --config   配置文件路径
-d, --debug    调试级别，可重复传入
```

示例：

```bash
go run . --config ./config.yaml
go run . -d
go run . -d -d
```

---

## 环境变量规则

执行 `app.Execute()` 时，会自动绑定环境变量。

规则：

- 前缀通常是应用名 `app.Name`
- `.` 会转成 `_`
- `-` 会转成 `_`

例如应用名：

```text
demo
```

配置：

```yaml
web:
  port: 8080
```

可以这样覆盖：

```bash
DEMO_WEB_PORT=8080
```

---

## .env 支持

创建应用后，会自动尝试读取当前目录下的 `.env` 文件。

示例：

```env
DEMO_WEB_PORT=8080
```

适合本地开发环境。

---

## version 命令

框架内置了：

- `version`
- `v`

示例：

```bash
go run . version
go run . v
```

一般会输出：

- Name
- CNName
- ID
- Version
- Commit
- BuildAt
- Description

如果没有显式传入 `Commit` 或 `BuildAt`，框架会尝试从构建信息里读取。

需要自定义输出时，使用 `SetVersionCmd`；`AddCommand` 会拒绝同名的 `version` 命令。

```go
app.SetVersionCmd(&cobra.Command{
    Use: "version",
    RunE: func(cmd *cobra.Command, _ []string) error {
        _, err := fmt.Fprintln(cmd.OutOrStdout(), meta.Version)
        return err
    },
})
```

替换时框架会先移除默认命令，再注册新命令；`v` 别名需要由自定义命令自行声明。

---

## 推荐项目接入方式

在 Mulan 生态里，通常这样组合：

- `mulan`：负责应用启动和服务托管
- `mulan-web`：负责 HTTP 服务
- `mulan-rdb`：负责 Redis
- `mulan-dbx`：负责数据库
- `mulan-req`：负责请求绑定
- `mulan-rsp`：负责统一响应

示例思路：

1. 用 `mulan.New(...)` 创建应用
2. 初始化配置和日志
3. 初始化 DB / Redis
4. 创建 `mulan-web` 服务
5. `app.AddService(webSrv)`
6. `app.Execute(...)`

---

## 最佳实践

### 1. `main.go` 只保留启动逻辑

建议 `main.go` 只做这些事：

- 创建配置对象
- 创建 `Meta`
- 创建 `App`
- 注册服务
- 执行启动

业务初始化拆到别的包里。

---

### 2. 所有服务都实现统一生命周期

把 HTTP、任务、消费者都包装成 `Servicer`，这样统一托管更清晰。

---

### 3. 预初始化里做依赖装配

例如：

- 数据库连接
- Redis 连接
- 业务组件初始化
- 全局依赖注册

不要把这些逻辑散落在 `main.go` 各处。

---

### 4. 元信息尽量完整

建议至少填写：

- `Name`
- `Version`
- `Commit`
- `BuildAt`

这样排查线上问题会方便很多。

---

### 5. 配置优先走配置文件 + 环境变量

这样更适合容器和云环境部署。

---

## 常见问题

### Q1：`mulan` 适合直接写业务吗？

不建议。  
它更适合做“启动层”和“服务管理层”，业务逻辑还是放到你自己的模块中。

---

### Q2：什么时候用 `Close()`，什么时候用 `Shutdown()`？

- 优先用 `Shutdown(ctx)`：优雅关闭
- `Close()`：兜底强制关闭

---

### Q3：可以只注册一个服务吗？

可以。  
最常见就是只注册一个 HTTP 服务。

---

## 总结

`mulan` 的定位很明确：

- 不是 Web 框架
- 不是 ORM
- 不是业务框架
- 它是应用启动和生命周期管理框架

如果你在做 Go 服务项目，想把：

- 启动入口
- 配置
- 命令行
- 服务托管
- 优雅退出

统一起来，那 `mulan` 很适合放在最外层。
