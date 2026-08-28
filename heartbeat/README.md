# Mulan Heartbeat

心跳检测组件，支持 HTTP 和 WebSocket 两种协议，内存和 Redis 两种存储方式。

## 功能特性

- **双协议支持**：HTTP (POST 轮询) 和 WebSocket (长连接)
- **可配置存储**：内存存储（单实例）或 Redis（分布式）
- **灵活集成**：HTTP 提供 `gin.HandlerFunc`，可轻松集成到现有 Gin 服务
- **两种工作模式**：
  - Clients → Server：客户端主动上报心跳
  - Server → Clients → Server：服务端发现模式，支持广播

## 安装

```bash
go get github.com/mulan-go/mulan/heartbeat
```

## 快速开始

### HTTP 模式 - 集成到 Gin 服务

```go
package main

import (
    "time"

    "github.com/gin-gonic/gin"
    "github.com/mulan-go/mulan/heartbeat/core"
    "github.com/mulan-go/mulan/heartbeat/http"
    "github.com/mulan-go/mulan/heartbeat/store"
)

func main() {
    // 创建内存存储
    s := store.NewMemoryStore(
        core.WithDefaultTimeout(30*time.Second),
        core.WithCleanupInterval(10*time.Second),
    )
    defer s.Close()

    // 创建 Handler
    h := http.NewHandler(s, &core.ServerInfo{
        Name: "my-server",
    }, nil)

    // 创建 Gin 引擎并注册路由
    r := gin.Default()
    h.RegisterRoutes(r) // 注册到 /heartbeat/*

    // 也可以自定义前缀
    // h.RegisterRoutesWithPrefix(r, "/api/v1/hb")

    // 或者单独注册某个 handler
    // r.POST("/custom/register", h.Register())

    r.Run(":8080")
}
```

### HTTP 客户端

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/mulan-go/mulan/heartbeat/core"
    "github.com/mulan-go/mulan/heartbeat/http"
)

func main() {
    client := http.NewClient(&http.ClientConfig{
        ServerURL:       "http://localhost:8080",
        ClientName:      "my-client",
        HeartbeatPeriod: 10 * time.Second,
    })

    // 设置回调
    client.OnConnected(func(server *core.ServerInfo) {
        fmt.Println("Connected to", server.Name)
    })

    // 启动
    if err := client.Start(context.Background()); err != nil {
        panic(err)
    }

    // 等待信号退出
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    client.Stop(context.Background())
}
```

### WebSocket 模式

#### 服务端

```go
package main

import (
    "fmt"

    "github.com/mulan-go/mulan/heartbeat/store"
    "github.com/mulan-go/mulan/heartbeat/ws"
)

func main() {
    s := store.NewMemoryStore()
    defer s.Close()

    server := ws.NewServer(nil, s, nil)

    // 设置回调
    server.Hub().OnClientJoin(func(clientID string) {
        fmt.Println("Client joined:", clientID)
    })

    if err := server.Serve(); err != nil {
        panic(err)
    }

    // ... 等待退出
}
```

#### 客户端

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/mulan-go/mulan/heartbeat/core"
    "github.com/mulan-go/mulan/heartbeat/ws"
)

func main() {
    client := ws.NewClient(&ws.ClientConfig{
        ServerURL:       "ws://localhost:8081",
        ClientName:      "my-ws-client",
        HeartbeatPeriod: 10 * time.Second,
    })

    client.OnConnected(func(server *core.ServerInfo) {
        fmt.Println("Connected to", server.Name)
    })

    client.OnBroadcast(func(msg *core.WSMessage) {
        fmt.Println("Broadcast:", msg.Payload)
    })

    if err := client.Start(context.Background()); err != nil {
        panic(err)
    }

    // ... 等待退出
}
```

### 使用 Redis 存储

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/mulan-go/mulan/heartbeat/core"
    "github.com/mulan-go/mulan/heartbeat/store"
)

rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

s := store.NewRedisStore(rdb,
    core.WithKeyPrefix("myapp:heartbeat:"),
    core.WithDefaultTimeout(30*time.Second),
)
defer s.Close()
```

## HTTP Handler API

HTTP 模块提供以下 `gin.HandlerFunc` 方法：

| 方法                    | 描述           |
| ----------------------- | -------------- |
| `h.Register()`          | 注册客户端     |
| `h.Heartbeat()`         | 上报心跳       |
| `h.Unregister()`        | 注销客户端     |
| `h.ListClients()`       | 获取客户端列表 |
| `h.ServerInfoHandler()` | 获取服务端信息 |
| `h.Health()`            | 健康检查       |

### 路由注册方式

```go
// 方式1: 批量注册（默认前缀 /heartbeat）
h.RegisterRoutes(router)

// 方式2: 自定义前缀批量注册
h.RegisterRoutesWithPrefix(router, "/api/v1/hb")

// 方式3: 单独注册到自定义路径
router.POST("/my/register", h.Register())
router.POST("/my/heartbeat", h.Heartbeat())
router.GET("/my/health", h.Health())
```

## HTTP API 接口

所有接口统一使用 **POST** 方法：

| 路径                  | 描述           | Body                           |
| --------------------- | -------------- | ------------------------------ |
| /heartbeat/register   | 注册客户端     | `{"name": "...", "ip": "..."}` |
| /heartbeat/heartbeat  | 上报心跳       | `{"id": "..."}`                |
| /heartbeat/unregister | 注销客户端     | `{"id": "..."}`                |
| /heartbeat/clients    | 获取客户端列表 | `{"only_online": true}` (可选) |
| /heartbeat/server     | 获取服务端信息 | 空                             |
| /heartbeat/health     | 健康检查       | 空                             |

### WebSocket 消息类型

| 类型      | 描述          |
| --------- | ------------- |
| register  | 注册/欢迎消息 |
| heartbeat | 心跳消息      |
| broadcast | 广播消息      |
| ping/pong | 保活消息      |

## 架构图

```
┌─────────────┐     HTTP POST      ┌─────────────┐
│   Client 1  │───────────────────▶│             │
└─────────────┘                    │             │
                                   │  Gin Server │
┌─────────────┐    WebSocket       │  + Handler  │     ┌─────────────┐
│   Client 2  │◀──────────────────▶│             │────▶│    Store    │
└─────────────┘                    │             │     │ Memory/Redis│
                                   │             │     └─────────────┘
┌─────────────┐     HTTP POST      │             │
│   Client N  │───────────────────▶│             │
└─────────────┘                    └─────────────┘
```

## 配置项

### 服务端配置

```yaml
server:
  server_id: ""
  server_name: "heartbeat-server"
  http:
    heartbeat_period: 10s
  ws:
    host: "0.0.0.0"
    port: 8081
    path: "/ws"
    broadcast_interval: 30s

store:
  type: "memory"  # memory 或 redis
  key_prefix: "heartbeat:"
  timeout: 30s
  cleanup_interval: 10s
  redis:
    host: "127.0.0.1"
    port: 6379
    db: 0
```

### 客户端配置

```yaml
client:
  mode: "http"  # http 或 ws
  http:
    server_url: "http://localhost:8080"
    heartbeat_period: 10s
    timeout: 5s
    retry_count: 3
    auto_register: true
  ws:
    server_url: "ws://localhost:8081"
    path: "/ws"
    heartbeat_period: 10s
    reconnect_delay: 5s
    max_reconnects: -1  # -1 表示无限重连
```

## License

MIT License
