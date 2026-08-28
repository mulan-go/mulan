# Mulan Log - 日志扩展

```go
import "github.com/mulan-go/mulan/log"
```

`mulan-log` 是基于 `zap` 的日志扩展库。  
提供统一日志初始化、文件滚动、HTTP 异步发送和资源清理能力。

适合这些场景：

- 服务启动时统一初始化日志
- 输出到终端、文件、HTTP 日志收集服务
- 需要日志滚动和压缩
- 需要优雅关闭日志资源

---

## 安装

```bash
go get github.com/mulan-go/mulan/log
```

---

## 核心能力

- 基于 `uber-go/zap`
- 默认输出到 `stdout`
- 支持多个输出适配器
- 支持文件滚动
- 支持 HTTP 异步批量发送
- 自动替换全局 `zap.L()`
- 提供 `Close()` 统一清理资源

---

## 核心类型

### `Logger`

```go
type Logger struct {
	*zap.Logger
	closers []io.Closer
}
```

说明：

- 内部包装了 `*zap.Logger`
- 额外持有需要关闭的资源
- 用完后建议执行 `logger.Close()`

### `Config`

```go
type Config struct {
	Level        string   `json:"level" yaml:"level"`
	ConsoleLevel string   `json:"console_level" yaml:"consoleLevel"`
	Mode         string   `json:"mode" yaml:"mode"`
	Format       string   `json:"format" yaml:"format"`
	Adaptors     []string `json:"adaptors" yaml:"adaptors"`
	Skip         int      `json:"skip" yaml:"skip"`
	JSON         bool     `json:"json" yaml:"json"`
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `Mode` | `local/dev/development/debug` 或 `server/prod/production` |
| `Level` | 适配器默认级别；也是控制台级别的回退值 |
| `ConsoleLevel` | 控制台独立级别；为空时继承 `Level` |
| `Format` | 控制台格式：`console` 或 `json` |
| `Adaptors` | 文件、HTTP 输出目标的 DSN 列表 |
| `Skip` | 调整 caller 层级 |
| `JSON` | 兼容旧配置；仅在 `Format` 为空时选择 JSON |

解析规则：

- `local` 默认 `debug + console`
- `server` 默认 `info + json`
- 未设置 `Mode` 时默认 `info + console`
- DSN 的 `level` 可覆盖单个适配器级别

---

## API

### 快速创建

```go
log.New(name ...string) (*Logger, error)
```

说明：

- 默认日志级别为 `info`
- 默认输出到标准输出
- 如果传了 `name`，会自动加上 `service` 字段
- 会替换全局 logger

示例：

```go
logger, err := log.New("my-service")
if err != nil {
	panic(err)
}
defer logger.Close()
```

---

### 从配置创建

```go
log.NewWithConfig(cfg *Config, name ...string) (*Logger, error)
```

说明：

- 可以自定义日志级别
- 可以独立配置控制台级别与格式
- 可以配置多个输出适配器
- 会替换全局 logger
- 无法解析或无法创建的单个适配器会被跳过；控制台输出仍然保留

示例：

```go
logger, err := log.NewWithConfig(&log.Config{
	Mode:         "local",
	ConsoleLevel: "debug",
}, "my-service")
if err != nil {
	panic(err)
}
defer logger.Close()
```

---

### 关闭资源

```go
logger.Close() error
```

说明：

- 会关闭文件写入器
- 会关闭 HTTP 异步写入器
- 建议在服务退出前调用

---

## 适配器 DSN

`mulan-log` 通过 DSN 配置额外输出。

---

### 文件适配器

格式：

```text
file:///path/to/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip&level=info
```

常见示例：

```go
Adaptors: []string{
	"file:///var/log/app.log",
	"file:///var/log/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
}
```

常用参数：

| 参数          | 说明                            |
| ------------- | ------------------------------- |
| `level`       | 当前适配器的日志级别            |
| `max-size`    | 单文件最大大小，如 `100m`、`1g` |
| `max-backups` | 保留旧文件数量                  |
| `max-age`     | 保留时长，如 `30d`              |
| `compress`    | 压缩方式，常见为 `gzip`         |

特点：

- 自动创建目录
- 自动滚动文件
- 支持旧文件清理
- 支持 gzip 压缩

---

### HTTP 适配器

格式：

```text
http://host:port/path?timeout=10s&buffer-size=1024&batch-size=100&max-retries=3&level=warn
```

常见示例：

```go
Adaptors: []string{
	"http://localhost:3000/logs",
	"https://logs.example.com/api/logs?timeout=5s&batch-size=100",
}
```

常用参数：

| 参数          | 说明               |
| ------------- | ------------------ |
| `level`       | 当前适配器日志级别 |
| `timeout`     | 请求超时           |
| `buffer-size` | 异步缓冲区大小     |
| `batch-size`  | 批量发送条数       |
| `max-retries` | 最大重试次数       |

特点：

- 异步发送
- 批量投递
- 自动重试
- 适合接入日志平台

---

## 快速开始

### 1. 最小示例

```go
package main

import (
	"github.com/mulan-go/mulan/log"
	"go.uber.org/zap"
)

func main() {
	logger, err := log.New("demo")
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	zap.L().Info("service started", zap.String("env", "dev"))
}
```

---

### 2. 输出到文件

```go
package main

import (
	"github.com/mulan-go/mulan/log"
	"go.uber.org/zap"
)

func main() {
	logger, err := log.NewWithConfig(&log.Config{
		Level: "info",
		Adaptors: []string{
			"file:///var/log/demo/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
		},
	}, "demo")
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	zap.L().Info("hello file log")
}
```

---

### 3. 输出到 HTTP 服务

```go
logger, err := log.NewWithConfig(&log.Config{
	Level: "info",
	Adaptors: []string{
		"http://localhost:3000/logs?timeout=5s&buffer-size=512&batch-size=50&max-retries=3",
	},
}, "demo")
if err != nil {
	panic(err)
}
defer logger.Close()
```

---

### 4. 同时输出到多个目标

```go
logger, err := log.NewWithConfig(&log.Config{
	Level: "debug",
	Adaptors: []string{
		"file:///var/log/demo/app.log?max-size=100m&compress=gzip",
		"http://logs.example.com/api/logs?batch-size=100",
	},
}, "demo")
if err != nil {
	panic(err)
}
defer logger.Close()
```

---

## 在 Mulan 项目中的推荐用法

### 启动时初始化

```go
func InitLogger() (*log.Logger, error) {
	return log.NewWithConfig(&log.Config{
		Mode:  "server",
		Level: "info",
		Adaptors: []string{
			"file:///var/log/app/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
		},
	}, "my-service")
}
```

### `main.go` 中接入

```go
func main() {
	logger, err := InitLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	zap.L().Info("booting")
	// 继续初始化 app / web / db / rdb
}
```

---

## Handler 层建议

建议每个请求都带上 `requestid`。

```go
logger := zap.L().With(zap.String("requestid", requestid.Get(c)))
logger.Info("processing request", zap.String("path", c.Request.URL.Path))
```

如果项目已经使用 `mulan-web`，也可以用它的辅助方法：

```go
logger := zap.L().With(web.ZapRequestID(c))
logger.Info("processing request")
```

---

## Biz 层建议

日志尽量结构化，不要拼字符串。

```go
func Create(ctx context.Context, logger *zap.Logger, data *CreateData) error {
	logger.Info("creating user",
		zap.String("account", data.Account),
		zap.Uint64("user_id", data.ID),
	)
	return nil
}
```

推荐做法：

- `Handler` 负责补 `requestid`
- `Biz` 继续透传 `logger`
- `Repo` 层一般不主动记业务日志，重点返回错误

---

## 最佳实践

- 启动时只初始化一次 logger
- 退出前记得 `defer logger.Close()`
- 生产环境使用 `Mode: "server"`，按需追加文件或 HTTP 适配器
- 本地开发使用 `Mode: "local"` 保留彩色控制台输出
- 不要把敏感信息直接写进日志
- 错误日志尽量带上关键业务字段
- 二次封装日志时可用 `Skip` 调整调用位置

---

## 常见组合

常见搭配：

- `mulan`：应用启动与生命周期管理
- `mulan-web`：HTTP 服务与请求链路
- `mulan-rsp`：统一错误返回
- `mulan-rdb`：Redis
- `mulan-dbx`：数据库

---

## 小结

`mulan-log` 的定位很清楚：

- 统一初始化 `zap`
- 统一多输出目标
- 统一资源回收
- 统一服务日志风格

如果你在 Mulan 项目里需要一套简单、稳定、可扩展的日志方案，优先直接用它，不要重复造一层日志封装。
