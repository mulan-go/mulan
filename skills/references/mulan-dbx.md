# Mulan DBX - 多驱动数据库

```go
import "github.com/mulan-go/mulan/dbx"
```

`mulan-dbx` 是基于 `GORM` 的轻量数据库扩展库。  
它会根据配置中的 `DSN` 自动识别数据库驱动，帮你更快接入数据库。

适合这些场景：

- 快速接入 `MySQL`
- 快速接入 `PostgreSQL`
- 快速接入 `SQLite`
- 统一管理连接池配置
- 统一接入 GORM 日志
- 在 Mulan 项目里作为数据库初始化层使用

---

## 主要能力

- 自动按 `DSN` 识别数据库类型
- 支持 `MySQL`
- 支持 `PostgreSQL`
- 支持 `SQLite`
- 支持统一连接池配置
- 支持 GORM Debug 日志
- 提供基础模型 `Model`
- 支持直接创建 GORM 实例或完全自定义驱动

---

## 安装

```bash
go get github.com/mulan-go/mulan/dbx
```

---

## 支持的 DSN

### MySQL

```text
mysql://user:pass@127.0.0.1:3306/dbname
```

### PostgreSQL

```text
postgres://user:pass@127.0.0.1:5432/dbname
```

### SQLite 内存库

```text
sqlite3://:memory:
```

### SQLite 文件库

```text
sqlite3://testdata/app.db
```

---

## 核心配置

### `Config`

```go
type Config struct {
	Conn    *ConnConfig
	Args    map[string]string
	DSN     string
	User    string
	Pass    string
	Name    string
	Debug   bool
	Migrate bool
}
```

### 字段说明

| 字段      | 说明                  |
| --------- | --------------------- |
| `DSN`     | 数据库连接地址        |
| `User`    | 覆盖 DSN 中的用户名   |
| `Pass`    | 覆盖 DSN 中的密码     |
| `Name`    | 覆盖 DSN 中的数据库名 |
| `Args`    | 追加或覆盖查询参数    |
| `Debug`   | 是否开启 GORM Debug   |
| `Migrate` | 是否启用自动迁移标记  |
| `Conn`    | 连接池配置            |

---

### `ConnConfig`

```go
type ConnConfig struct {
	Idle     int
	Open     int
	Lifetime int
}
```

### 字段说明

| 字段       | 说明                     |
| ---------- | ------------------------ |
| `Idle`     | 最大空闲连接数           |
| `Open`     | 最大打开连接数           |
| `Lifetime` | 连接最大复用时间，单位秒 |

---

## 常用 API

### 自动识别驱动

```go
dbx.Auto(cfg *Config) (*gorm.DB, error)
```

适合大多数场景。  
只要给出 `DSN`，库会自动选择对应驱动。

---

### 直接创建数据库实例

```go
dbx.New(dialector gorm.Dialector, cfg *Config, opts ...gorm.Option) (*gorm.DB, error)
```

适合这些情况：

- 你想手动指定驱动
- 你想传入自定义 `gorm.Config`
- 你想完全控制 GORM 初始化行为

---

### 创建日志器

```go
dbx.NewLogger(isDebug bool) gLogger.Interface
```

用于创建 GORM 日志器。

---

### 默认 GORM 配置

```go
dbx.DefaultConfig() *gorm.Config
```

用于拿到库内部推荐的默认配置。

---

## 基础模型

```go
type Model struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	UUID      uuid.UUID `gorm:"type:uuid;unique;column:uuid"`
	CreatedAt int64     `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64     `gorm:"autoUpdateTime;column:updated_at"`
}
```

`BeforeCreate` 在 UUID 为空时由应用生成 UUIDv7，失败时回退 UUIDv4。字段不再声明 PostgreSQL `gen_random_uuid()` 默认值，因此 SQLite、MySQL、PostgreSQL 使用相同生成路径。

还提供：

```go
type StringSlice = datatypes.JSONSlice[string]

type Modeler interface {
	TableName() string
	Defaults() []Modeler
	GetID() uint64
}
```

---

## 快速开始

### 1. 最简单用法

```go
package main

import (
	"log"

	"github.com/mulan-go/mulan/dbx"
)

func main() {
	cfg := &dbx.Config{
		DSN: "sqlite3://:memory:",
	}

	db, err := dbx.Auto(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = db
}
```

---

### 2. 使用 MySQL

```go
package main

import (
	"log"

	"github.com/mulan-go/mulan/dbx"
)

func main() {
	cfg := &dbx.Config{
		DSN:   "mysql://root:123456@127.0.0.1:3306/mulan",
		Debug: true,
		Args: map[string]string{
			"parseTime": "true",
			"loc":       "Local",
		},
	}

	db, err := dbx.Auto(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = db
}
```

---

### 3. 使用 PostgreSQL

```go
package main

import (
	"log"

	"github.com/mulan-go/mulan/dbx"
)

func main() {
	cfg := &dbx.Config{
		DSN: "postgres://postgres:123456@127.0.0.1:5432/mulan",
		Args: map[string]string{
			"sslmode":  "disable",
			"TimeZone": "Asia/Shanghai",
		},
	}

	db, err := dbx.Auto(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = db
}
```

---

### 4. 使用 SQLite

```go
package main

import (
	"log"

	"github.com/mulan-go/mulan/dbx"
)

func main() {
	cfg := &dbx.Config{
		DSN: "sqlite3://testdata/app.db",
	}

	db, err := dbx.Auto(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = db
}
```

---

## 常见写法

### 覆盖用户名、密码、数据库名

```go
cfg := (&dbx.Config{
	DSN:  "postgres://olduser:oldpass@127.0.0.1:5432/olddb",
	User: "newuser",
	Pass: "newpass",
	Name: "newdb",
}).Parse()

println(cfg.String())
```

适合：

- 本地和线上账号不同
- 同一份 DSN 模板按环境覆盖
- 通过配置中心动态拼接数据库信息

---

### 追加查询参数

```go
cfg := (&dbx.Config{
	DSN: "postgres://postgres:123456@127.0.0.1:5432/mulan",
}).
	WithArgs("sslmode", "disable").
	WithArgs("TimeZone", "Asia/Shanghai").
	Parse()
```

适合：

- 给 PostgreSQL 增加 `sslmode`
- 给 MySQL 增加 `parseTime`
- 统一注入时区等参数

---

### 自定义 GORM 配置

```go
package main

import (
	"log"

	"github.com/glebarez/sqlite"
	"github.com/mulan-go/mulan/dbx"
	"gorm.io/gorm"
)

func main() {
	cfg := (&dbx.Config{
		DSN: "sqlite3://:memory:",
	}).Parse()

	db, err := dbx.New(
		sqlite.Open(cfg.String()),
		cfg,
		&gorm.Config{
			Logger: dbx.NewLogger(cfg.Debug),
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = db
}
```

如果你想完全控制驱动和 GORM 配置，优先用这种方式。

---

## 连接池建议

你可以通过 `Conn` 控制连接池：

```go
cfg := &dbx.Config{
	DSN: "postgres://postgres:123456@127.0.0.1:5432/mulan",
	Conn: &dbx.ConnConfig{
		Idle:     10,
		Open:     100,
		Lifetime: 3600,
	},
}
```

建议：

- 小服务：`Idle=5~10`
- 常规 API 服务：`Open=50~100`
- `Lifetime` 按数据库和连接代理策略调整

---

## 默认行为

库内部默认会设置一部分 GORM 配置：

- `QueryFields: true`
- `DisableForeignKeyConstraintWhenMigrating: true`
- `IgnoreRelationshipsWhenMigrating: true`

这样做的目的通常是：

- 查询字段更明确
- 降低迁移时外键带来的问题
- 减少复杂关系自动迁移副作用

---

## 在 Mulan 项目中的推荐用法

### 1. 放进配置结构

```go
type Config struct {
	DB dbx.Config `json:"db" yaml:"db"`
}
```

---

### 2. 应用启动时初始化

```go
func InitDB(cfg *dbx.Config) (*gorm.DB, error) {
	return dbx.Auto(cfg)
}
```

---

### 3. 业务层使用 `WithContext(ctx)`

```go
func (r *Repo) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&user).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &user, nil
}
```

---

## 最佳实践

- 所有数据库操作必须使用 `WithContext(ctx)`
- 优先在初始化阶段统一创建 `*gorm.DB`
- Repo 层只做数据访问，不要混入业务逻辑
- Repo 层错误建议用 `errors.WithStack(err)` 包装
- 参数和 DSN 统一从配置读取，不要散落在代码里
- 多数场景优先使用 `dbx.Auto(...)`
- 只有在需要自定义驱动或 GORM 配置时，再使用 `dbx.New(...)`

---

## 常见组合

在 Mulan 生态里，`mulan-dbx` 常和这些模块一起使用：

- `mulan`：应用启动
- `mulan-web`：HTTP 服务
- `mulan-req`：请求绑定
- `mulan-rsp`：统一响应
- `mulan-rdb`：Redis
- `mulan-auth`：认证鉴权

---

## 小结

如果你想要的是：

- 少写驱动初始化代码
- 按 DSN 自动切换数据库
- 保持 GORM 接入方式统一
- 在 Mulan 项目里快速完成数据库初始化

那 `mulan-dbx` 就很适合放在项目的数据库接入层。
