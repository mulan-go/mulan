# Mulan Heartbeat - 心跳检测

`mulan-heartbeat` 是一个心跳检测组件，支持：

- HTTP 心跳
- WebSocket 心跳
- 内存存储
- Redis 存储

适合做：

- 客户端在线状态管理
- 服务探活
- Agent / Worker 存活上报
- 分布式节点状态追踪

---

## 安装

```bash
go get github.com/mulan-go/mulan/heartbeat
```

---

## 常用导入

```go
import (
    "github.com/mulan-go/mulan/heartbeat/core"
    hbhttp "github.com/mulan-go/mulan/heartbeat/http"
    "github.com/mulan-go/mulan/heartbeat/store"
    "github.com/mulan-go/mulan/heartbeat/ws"
)
```

如果使用 Redis，还需要：

```go
import "github.com/redis/go-redis/v9"
```

> 注意：这里的导入路径要用 `github.com/mulan-go/mulan/heartbeat/...`，不是旧路径。

---

## 核心能力

- 支持 **HTTP POST 轮询**
- 支持 **WebSocket 长连接**
- 支持 **Memory Store**
- 支持 **Redis Store**
- 可注册到现有 Gin 服务
- 支持客户端注册、心跳、注销、列表查询
- 支持服务端广播和连接事件回调

---

## 典型模式

### 1. Clients -> Server

客户端定时上报心跳，服务端记录在线状态。

适合：

- 普通业务客户端
- 轮询式在线检测
- 对长连接要求不高的场景

### 2. Server -> Clients -> Server

服务端和客户端通过 WebSocket 保持连接，支持广播和实时状态管理。

适合：

- 实时监控
- 在线终端管理
- 大量长连接客户端

---

## HTTP 模式

### 创建内存存储

```go
s := store.NewMemoryStore(
    core.WithDefaultTimeout(30*time.Second),
    core.WithCleanupInterval(10*time.Second),
)
defer s.Close()
```

### 创建 HTTP Handler

```go
h := hbhttp.NewHandler(s, &core.ServerInfo{
    Name: "my-server",
}, nil)
```

### 注册路由

默认前缀：

```go
h.RegisterRoutes(r)
```

默认会注册到：

```text
/heartbeat/*
```

自定义前缀：

```go
h.RegisterRoutesWithPrefix(r, "/api/v1/hb")
```

也可以单独注册：

```go
r.POST("/custom/register", h.Register())
r.POST("/custom/heartbeat", h.Heartbeat())
r.POST("/custom/unregister", h.Unregister())
r.POST("/custom/clients", h.ListClients())
r.POST("/custom/server", h.ServerInfoHandler())
r.POST("/custom/health", h.Health())
```

---

## HTTP 服务端完整示例

```go
package main

import (
    "time"

    "github.com/gin-gonic/gin"
    "github.com/mulan-go/mulan/heartbeat/core"
    hbhttp "github.com/mulan-go/mulan/heartbeat/http"
    "github.com/mulan-go/mulan/heartbeat/store"
)

func main() {
    s := store.NewMemoryStore(
        core.WithDefaultTimeout(30*time.Second),
        core.WithCleanupInterval(10*time.Second),
    )
    defer s.Close()

    h := hbhttp.NewHandler(s, &core.ServerInfo{
        Name: "my-server",
    }, nil)

    r := gin.Default()
    h.RegisterRoutes(r)

    _ = r.Run(":8080")
}
```

---

## HTTP 客户端示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/mulan-go/mulan/heartbeat/core"
    hbhttp "github.com/mulan-go/mulan/heartbeat/http"
)

func main() {
    client := hbhttp.NewClient(&hbhttp.ClientConfig{
        ServerURL:       "http://localhost:8080",
        ClientName:      "my-client",
        HeartbeatPeriod: 10 * time.Second,
    })

    client.OnConnected(func(server *core.ServerInfo) {
        fmt.Println("connected to", server.Name)
    })

    if err := client.Start(context.Background()); err != nil {
        panic(err)
    }

    select {}
}
```

---

## HTTP API 列表

所有接口统一使用 **POST**。

| 路径                    | 说明           | Body                        |
| ----------------------- | -------------- | --------------------------- |
| `/heartbeat/register`   | 注册客户端     | `{"name":"...","ip":"..."}` |
| `/heartbeat/heartbeat`  | 上报心跳       | `{"id":"..."}`              |
| `/heartbeat/unregister` | 注销客户端     | `{"id":"..."}`              |
| `/heartbeat/clients`    | 获取客户端列表 | `{"only_online":true}`      |
| `/heartbeat/server`     | 获取服务端信息 | 空                          |
| `/heartbeat/health`     | 健康检查       | 空                          |

---

## WebSocket 模式

### 服务端示例

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

    server.Hub().OnClientJoin(func(clientID string) {
        fmt.Println("client joined:", clientID)
    })

    if err := server.Serve(); err != nil {
        panic(err)
    }
}
```

### 客户端示例

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
        fmt.Println("connected to", server.Name)
    })

    client.OnBroadcast(func(msg *core.WSMessage) {
        fmt.Println("broadcast:", msg.Payload)
    })

    if err := client.Start(context.Background()); err != nil {
        panic(err)
    }

    select {}
}
```

---

## Redis 存储

如果是多实例部署，建议用 Redis。

```go
package main

import (
    "time"

    "github.com/mulan-go/mulan/heartbeat/core"
    "github.com/mulan-go/mulan/heartbeat/store"
    "github.com/redis/go-redis/v9"
)

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    s := store.NewRedisStore(
        rdb,
        core.WithKeyPrefix("myapp:heartbeat:"),
        core.WithDefaultTimeout(30*time.Second),
    )
    defer s.Close()

    _ = s
}
```

---

## 常用 HTTP Handler 方法

HTTP 模块提供这些 `gin.HandlerFunc`：

| 方法                    | 说明           |
| ----------------------- | -------------- |
| `h.Register()`          | 注册客户端     |
| `h.Heartbeat()`         | 上报心跳       |
| `h.Unregister()`        | 注销客户端     |
| `h.ListClients()`       | 查询客户端列表 |
| `h.ServerInfoHandler()` | 查询服务端信息 |
| `h.Health()`            | 健康检查       |

---

## WebSocket 消息类型

常见消息类型：

| 类型            | 说明            |
| --------------- | --------------- |
| `register`      | 注册 / 欢迎消息 |
| `heartbeat`     | 心跳消息        |
| `broadcast`     | 广播消息        |
| `ping` / `pong` | 保活消息        |

---

## 和 Mulan 生态一起使用

常见组合：

- `mulan-web`：把 HTTP heartbeat 路由挂进 Gin 服务
- `mulan-rdb`：提供 Redis 连接
- `mulan-auth`：结合登录态做客户端身份校验
- `mulan-rsp`：统一业务接口响应格式

示例思路：

1. 用 `mulan-rdb` 初始化 Redis
2. 用 `store.NewRedisStore(...)` 创建心跳存储
3. 用 `hbhttp.NewHandler(...)` 创建 heartbeat handler
4. 注册到 `mulan-web` 路由中

---

## 最佳实践

- 单机部署可直接用 `MemoryStore`
- 多实例部署优先用 `RedisStore`
- HTTP 模式适合简单可靠场景
- WebSocket 模式适合实时场景
- 心跳超时时间要大于客户端上报周期
- 清理间隔不要过短，避免无效开销
- 如果要区分业务来源，建议加统一前缀，如 `app:heartbeat:`

---

## 推荐配置思路

### 开发环境

- 内存存储
- HTTP 模式
- 较短超时，便于调试

### 生产环境

- Redis 存储
- 根据业务选择 HTTP 或 WebSocket
- 设置统一 key 前缀
- 配置合理的超时时间和清理间隔

---

## 小结

`mulan-heartbeat` 适合做在线状态管理这类通用能力：

- 想简单接入，用 **HTTP + Memory**
- 想多实例部署，用 **HTTP + Redis**
- 想实时通信，用 **WebSocket + Redis/Memory**

如果你在 Mulan 项目里需要客户端在线检测，这个组件可以直接用，不必重复造轮子。
