# Mulan RDB - Redis 扩展

```go
import "github.com/mulan-go/mulan/rdb"
```

`mulan-rdb` 是一个轻量的 Redis 客户端封装。  
基于 `go-redis/v9`，提供统一配置、命令行参数和调试能力。

---

## 适用场景

- 业务服务接入 Redis
- 缓存、会话、限流、分布式状态存储
- 配合 `mulan-auth`、`mulan-heartbeat` 等组件使用
- 需要统一配置和快速初始化 Redis 客户端

---

## 安装

```bash
go get github.com/mulan-go/mulan/rdb
```

---

## 核心类型

### `Config`

```go
type Config struct {
    Host  string `json:"host" yaml:"host"`
    Pass  string `json:"pass" yaml:"pass"`
    Port  int    `json:"port" yaml:"port"`
    DB    int    `json:"db" yaml:"db"`
    Debug bool   `json:"debug" yaml:"debug"`
}
```

### 字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `Host` | `string` | Redis 地址 |
| `Port` | `int` | Redis 端口 |
| `Pass` | `string` | Redis 密码 |
| `DB` | `int` | Redis 数据库编号 |
| `Debug` | `bool` | 是否开启调试 Hook |

---

## API

### 创建客户端

```go
rdb.New(cfg *Config) (*redis.Client, error)
```

说明：

- 内部会创建 `redis.Client`
- 初始化后会自动执行 `PING`
- 如果连接失败，会直接返回错误
- `Debug=true` 时会挂载调试 Hook

### Redis 空值错误

```go
rdb.Nil
```

它是 `redis.Nil` 的别名。

常用于判断 key 不存在：

```go
if errors.Is(err, rdb.Nil) {
    // key 不存在
}
```

### 配置辅助方法

```go
cfg.Addr() string
cfg.FlagSet(name string) *pflag.FlagSet
cfg.R() *Config
rdb.FlagSet(name string) *pflag.FlagSet
```

说明：

- `Addr()`：返回 `host:port`
- `FlagSet(name)`：生成对应 Redis 配置命令行参数
- `R()`：返回自身，常用于链式或统一接口适配

---

## 命令行参数

`FlagSet(name)` 会生成一组参数：

```text
{name}.debug
{name}.host
{name}.port
{name}.db
{name}.pass
```

例如：

```go
fs := rdb.FlagSet("redis")
```

会生成：

```text
redis.debug
redis.host
redis.port
redis.db
redis.pass
```

---

## 快速开始

### 1. 最小示例

```go
package main

import (
    "context"
    "log"

    "github.com/mulan-go/mulan/rdb"
)

func main() {
    cfg := &rdb.Config{
        Host: "127.0.0.1",
        Port: 6379,
        DB:   0,
    }

    client, err := rdb.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    err = client.Set(context.Background(), "hello", "world", 0).Err()
    if err != nil {
        log.Fatal(err)
    }
}
```

### 2. 判断 key 是否存在

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    "github.com/mulan-go/mulan/rdb"
)

func main() {
    client, err := rdb.New(&rdb.Config{
        Host: "127.0.0.1",
        Port: 6379,
        DB:   0,
    })
    if err != nil {
        log.Fatal(err)
    }

    val, err := client.Get(context.Background(), "not-exists").Result()
    if errors.Is(err, rdb.Nil) {
        fmt.Println("key not found")
        return
    }
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(val)
}
```

### 3. 开启调试

```go
cfg := &rdb.Config{
    Host:  "127.0.0.1",
    Port:  6379,
    DB:    0,
    Debug: true,
}

client, err := rdb.New(cfg)
```

适合本地开发排查 Redis 请求。

---

## 与 Mulan 配合使用

### 放到配置结构里

```go
type Config struct {
    Redis rdb.Config `json:"redis" yaml:"redis"`
}
```

### 注册命令行参数

```go
app.AddFlagSet(conf.Redis.FlagSet("redis"))
```

### 初始化客户端

```go
client, err := rdb.New(&conf.Redis)
if err != nil {
    return err
}
```

---

## 常见用法

### 字符串缓存

```go
err := client.Set(ctx, "user:1:name", "alice", time.Minute*10).Err()
val, err := client.Get(ctx, "user:1:name").Result()
```

### 计数器

```go
n, err := client.Incr(ctx, "login:count").Result()
```

### 设置过期

```go
err := client.Set(ctx, "sms:code:13800138000", "123456", time.Minute*5).Err()
```

### 哈希结构

```go
err := client.HSet(ctx, "user:1", map[string]any{
    "name": "alice",
    "age":  18,
}).Err()
```

---

## 最佳实践

1. 统一复用 `*redis.Client`，不要每次请求都新建
2. 所有操作都传 `context.Context`
3. 对缓存 key 做统一前缀约定
4. 对 `rdb.Nil` 单独判断，不要当成系统错误
5. 生产环境建议关闭 `Debug`
6. 密码不要写死，优先从配置文件或环境变量读取

---

## 推荐 key 设计

建议使用统一前缀，便于排查和清理：

```text
app:user:1
app:session:token:xxx
app:captcha:login:uuid
app:rate_limit:login:ip:127.0.0.1
```

---

## 错误处理建议

```go
val, err := client.Get(ctx, key).Result()
switch {
case err == nil:
	// 使用 val
case errors.Is(err, rdb.Nil):
	// key 不存在，按缓存未命中处理
default:
	return err
}
```

`rdb.Nil` 是 `redis.Nil` 的别名。缓存未命中与连接、超时等系统错误必须分开处理。

---

## 小结

- `rdb.New` 创建并 Ping 单节点 Redis 客户端
- `Config` 管理地址、密码、DB 与 Debug Hook
- 客户端应全局复用，所有命令传入调用链 `ctx`
- 缓存未命中使用 `errors.Is(err, rdb.Nil)` 判断
