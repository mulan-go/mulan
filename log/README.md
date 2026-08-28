# Mulan-Ext Log

基于 `zap` 的日志库，支持文件滚动、HTTP 异步发送等功能。

## 功能特性

- ✅ 基于 `uber-go/zap` 高性能日志库
- ✅ 支持多种输出适配器（stdout、文件、HTTP）
- ✅ 文件自动滚动（基于大小、时间、数量）
- ✅ HTTP 异步批量发送
- ✅ 资源自动清理
- ✅ 灵活的配置选项

## 安装

```bash
go get github.com/mulan-go/mulan/log
```

## 快速开始

### 基础使用

```go
package main

import (
    "github.com/mulan-go/mulan/log"
    "go.uber.org/zap"
)

func main() {
    // 创建日志实例
    logger, err := log.New("my-app")
    if err != nil {
        panic(err)
    }
    defer logger.Close() // 确保资源清理
    
    // 使用全局 logger
    zap.L().Info("Hello World", zap.String("key", "value"))
}
```

### 配置使用（DSN 格式）

```go
logger, err := log.NewWithConfig(&log.Config{
    Level:    "debug",  // 日志级别
    Adaptors: []string{
        // 文件输出 DSN
        "file:///var/log/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
        
        // HTTP 输出 DSN
        "http://localhost:3000/logs?timeout=10s&buffer-size=1024&batch-size=100&max-retries=3",
    },
})
```

### 本地调试输出

本地模式默认使用彩色 console 输出，并把默认级别设为 `debug`：

```go
logger, err := log.NewWithConfig(&log.Config{
    Mode: "local",
})
```

也可以显式指定控制台级别：

```go
logger, err := log.NewWithConfig(&log.Config{
    Mode:         "local",
    ConsoleLevel: "debug",
})
```

### 服务端 JSON 输出

服务端模式默认使用 JSON 输出，并把默认级别设为 `info`：

```go
logger, err := log.NewWithConfig(&log.Config{
    Mode: "server",
})
```

服务端常见配置示例：

```go
logger, err := log.NewWithConfig(&log.Config{
    Mode:         "server",
    Level:        "info",
    ConsoleLevel: "info",
    Adaptors: []string{
        "file:///var/log/app.log?max-size=100m&max-age=30d",
    },
})
```

## 配置说明

### 基础配置

| 配置项         | 类型     | 默认值      | 说明                                        |
| -------------- | -------- | ----------- | ------------------------------------------- |
| `Mode`         | string   | `""`        | 运行模式：`local`, `server`                 |
| `Level`        | string   | `"info"`    | 默认日志级别：debug, info, warn, error      |
| `ConsoleLevel` | string   | 继承 `Level` | 控制台日志级别                              |
| `Format`       | string   | `"console"` | 控制台格式：`console`, `json`               |
| `JSON`         | bool     | `false`     | 兼容旧配置；为 true 时控制台输出 JSON       |
| `Adaptors`     | []string | `[]`        | 输出适配器 DSN 列表                         |

级别规则：

- 控制台默认使用 `ConsoleLevel`，未设置时继承 `Level`。
- 文件和 HTTP 适配器默认继承 `Level`。
- 文件和 HTTP DSN 可通过 `?level=debug` 之类的参数覆盖自己的输出级别。

## 适配器 DSN 格式

### 文件适配器

**格式：** `file://<path>?<params>`

**示例：**
```go
Adaptors: []string{
    "file:///var/log/app.log",
    "file:///var/log/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
}
```

**参数说明：**

| 参数          | 类型   | 默认值 | 说明                                                      |
| ------------- | ------ | ------ | --------------------------------------------------------- |
| `max-size`    | string | `100m` | 文件最大大小，支持 `m`/`mb`/`g`/`gb` (如 `100m`, `1g`)    |
| `max-backups` | int    | `10`   | 保留旧文件数量                                            |
| `max-age`     | string | `30d`  | 保留旧文件天数，支持 `d`/`day`/`days` (如 `30d`, `7days`) |
| `compress`    | string | `none` | 压缩格式：`gzip` 或 `none`                                |
| `level`       | string | 继承全局 | 当前文件适配器的日志级别                                  |

**特性：**
- ✅ 自动创建目录
- ✅ 基于大小自动滚动
- ✅ 支持 gzip 压缩
- ✅ 按时间和数量自动清理

### HTTP 适配器

**格式：** `http(s)://<host>/<path>?<params>`

**示例：**
```go
Adaptors: []string{
    "http://localhost:3000/logs",
    "https://logs.example.com/api/v1/logs?timeout=5s&buffer-size=512&batch-size=50&max-retries=3",
}
```

**参数说明：**

| 参数          | 类型          | 默认值 | 说明                                     |
| ------------- | ------------- | ------ | ---------------------------------------- |
| `timeout`     | time.Duration | `10s`  | HTTP 请求超时时间 (如 `5s`, `30s`, `1m`) |
| `buffer-size` | int           | `1024` | 异步缓冲区大小（日志条数）               |
| `batch-size`  | int           | `100`  | 批量发送大小（每批日志条数）             |
| `max-retries` | int           | `3`    | 最大重试次数                             |
| `level`       | string        | 继承全局 | 当前 HTTP 适配器的日志级别               |

**特性：**
- ✅ 异步批量发送
- ✅ 自动重试机制
- ✅ 非阻塞写入
- ✅ 优雅关闭

## 最佳实践

```go
func InitLogger() (*log.Logger, error) {
    cfg := &log.Config{
        Level: getEnv("LOG_LEVEL", "info"),
        Adaptors: []string{
            // 文件日志，带滚动和压缩
            "file:///var/log/app.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip",
            
            // 可选：发送到日志收集服务
            // "http://logs.example.com/api/logs?timeout=5s&batch-size=100",
        },
    }
    
    return log.NewWithConfig(cfg)
}

func main() {
    logger, err := InitLogger()
    if err != nil {
        panic(err)
    }
    defer logger.Close() // 确保资源清理
    
    // 业务逻辑
}
```

### DSN 示例

```go
// 1. 基础文件输出
"file:///var/log/app.log"

// 2. 文件输出，1GB 滚动，保留 7 天
"file:///var/log/app.log?max-size=1g&max-age=7d"

// 3. 文件输出，带 gzip 压缩
"file:///var/log/app.log?max-size=50m&compress=gzip"

// 4. HTTP 输出，基础配置
"http://localhost:3000/logs"

// 5. HTTPS 输出，自定义参数
"https://logs.example.com/api/v1/logs?timeout=5s&buffer-size=512&batch-size=50&max-retries=5"

// 6. 多个适配器
Adaptors: []string{
    "file:///var/log/app.log?max-size=100m&compress=gzip",
    "file:///var/log/error.log?max-size=50m",  // 错误日志单独文件
    "http://logs.example.com/api/logs?batch-size=100",
}
```

## 许可证

本项目基于 [MIT](LICENSE) 开源。
