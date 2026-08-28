# Mulan Ext DBX (GORM)

`mulan-dbx` 是一个基于 GORM 的轻量数据库扩展库。  
它会根据配置中的 `DSN` 自动选择数据库驱动，帮助业务项目更快接入数据库。

## 特性

- [x] 按 `DSN` 自动识别数据库类型
- [x] 支持 `MySQL`
- [x] 支持 `PostgreSQL`
- [x] 支持 `SQLite`（Pure-Go）
- [x] 提供统一的连接池配置
- [x] 提供 GORM Logger 接入
- [x] 提供通用基础模型 `Model`

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

## 快速开始

### 1. 自动选择驱动

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

### 2. 使用 MySQL

```go
package main

import (
	"log"

	"github.com/mulan-go/mulan/dbx"
)

func main() {
	cfg := &dbx.Config{
		DSN:  "mysql://root:123456@127.0.0.1:3306/mulan",
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

## 配置说明

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

### `ConnConfig`

```go
type ConnConfig struct {
	Idle     int
	Open     int
	Lifetime int
}
```

| 字段       | 说明                     |
| ---------- | ------------------------ |
| `Idle`     | 最大空闲连接数           |
| `Open`     | 最大打开连接数           |
| `Lifetime` | 连接最大复用时间，单位秒 |

---

## 常见用法

### 覆盖用户名、密码、数据库名

```go
cfg := (&dbx.Config{
	DSN: "postgres://olduser:oldpass@127.0.0.1:5432/olddb",
	User: "newuser",
	Pass: "newpass",
	Name: "newdb",
}).Parse()

println(cfg.String())
```

### 追加参数

```go
cfg := (&dbx.Config{
	DSN: "postgres://postgres:123456@127.0.0.1:5432/mulan",
}).
	WithArgs("sslmode", "disable").
	WithArgs("TimeZone", "Asia/Shanghai").
	Parse()
```

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

> 如果你需要完全自定义驱动和 GORM 配置，建议直接使用 `dbx.New(...)`。  
> 如果你只想按 `DSN` 自动识别数据库，建议使用 `dbx.Auto(...)`。

---

## 默认行为说明

库内部默认会设置一部分 GORM 配置：

- `QueryFields: true`
- `DisableForeignKeyConstraintWhenMigrating: true`
- `IgnoreRelationshipsWhenMigrating: true`

这几个默认值更偏向“基础库保守模式”，目的是：

- 避免迁移时自动创建外键
- 降低关系迁移带来的误操作风险
- 让库项目默认行为更稳一些

如果你的业务项目有特殊要求，建议自己传入 `gorm.Config` 做覆盖。

---

## 基础模型

项目内置了通用基础模型：

```go
type Model struct {
	ID        uint64
	UUID      uuid.UUID
	CreatedAt int64
	UpdatedAt int64
}
```

特性：

- `ID` 自增主键
- `UUID` 自动生成
- `CreatedAt` 自动创建时间
- `UpdatedAt` 自动更新时间

示例：

```go
type User struct {
	dbx.Model
	Name string `gorm:"column:name"`
}
```

---

## 日志说明

### 开启日志

```go
cfg := &dbx.Config{
	DSN:   "sqlite3://:memory:",
	Debug: true,
}
```

开启后会输出更详细的 SQL 日志，适合开发和排查问题。

### 日志建议

- 开发环境：可开启 `Debug`
- 测试环境：按需开启
- 生产环境：建议关闭详细 SQL 日志

---

## 安全注意事项

这是库项目接入时最容易忽略的部分。

### 1. 不要直接打印完整 DSN

错误示例：

```go
log.Println(cfg.DSN)
```

因为 DSN 里通常包含：

- 用户名
- 密码
- 主机地址
- 数据库名

直接打印会泄露敏感信息。

### 2. `Debug` 只建议在开发环境开启

开启调试后，SQL 和参数可能一起输出。  
如果你的业务 SQL 参数中包含敏感字段，会有泄露风险，例如：

- 密码
- Token
- 手机号
- 邮箱
- 身份证号

### 3. 生产环境应最小化日志暴露

建议：

- 关闭详细 SQL 日志
- 不要打印完整数据库连接串
- 不要把数据库账号密码写入普通业务日志

### 4. 自动迁移要谨慎使用

虽然库默认对迁移行为做了保守处理，但生产环境仍不建议随意执行自动迁移。  
更推荐：

- 开发环境可用
- 测试环境按流程执行
- 生产环境走明确的数据库变更流程

### 5. 配置文件要做好权限控制

如果你把数据库配置写在配置文件中，建议：

- 不提交真实密码到公开仓库
- 使用环境变量或密钥管理服务
- 限制配置文件读取权限

---

## 测试说明

当前推荐的测试策略：

- 单元测试：默认执行
- SQLite 内存库测试：默认执行
- 真实 MySQL / PostgreSQL 集成测试：建议单独执行，不作为默认测试路径

执行测试：

```bash
go test ./...
```

---

## 常见问题

### 1. 为什么 `Auto()` 会报不支持的协议？

请检查 `DSN` 是否带有正确协议头，例如：

- `mysql://`
- `postgres://`
- `sqlite3://`

### 2. 为什么 SQLite 内存库适合测试？

因为它：

- 不依赖外部数据库服务
- 启动快
- 适合单元测试和冒烟测试

### 3. 什么时候用 `Auto()`，什么时候用 `New()`？

建议：

- 简单接入：用 `Auto()`
- 需要精细控制 dialector / gorm.Config：用 `New()`

---

## 建议的使用方式

### 开发环境

- 使用 `sqlite3://:memory:` 或本地数据库
- 可开启 `Debug`
- 可按需自动迁移

### 测试环境

- 优先 SQLite
- 集成测试再连接真实数据库
- 日志按需开启

### 生产环境

- 使用明确的 MySQL / PostgreSQL 配置
- 关闭 Debug
- 避免打印敏感配置
- 自动迁移谨慎使用

---

## 后续建议

如果要把 `mulan-dbx` 作为长期维护的公共基础库，建议继续补强：

- [ ] 增加更明确的配置校验
- [ ] 增加敏感信息脱敏能力
- [ ] 增加错误分类
- [ ] 增加更多 README 示例
- [ ] 拆分单元测试与集成测试

---

## License

MIT
