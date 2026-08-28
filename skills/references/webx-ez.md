# webx-ez 项目模板

`webx-ez` 是当前 Mulan 生态的 Go HTTP 服务模板。它已接入启动生命周期、Vault 远程配置、日志、GORM、Session、API Key、验证码、Swagger、静态前端和维护命令。

## 当前目录边界

```text
webx-ez/
├── main.go                    # 元信息、构建版本、进程退出
├── config/                    # 总配置、默认模板、校验、脱敏
├── api/
│   ├── router.go              # 路由装配
│   ├── security.go            # 安全响应头、Body 限制、ready
│   ├── frontend_embed.go      # 内嵌前端资源
│   └── v1/                    # HTTP Handler 与 DTO
├── biz/biz.go                 # Repo / Service 依赖装配
├── domain/
│   ├── model.go               # 模型注册、迁移
│   └── user/                  # 用户 Model、Repo、Service
├── internal/
│   ├── boot/                  # 应用、服务、命令装配
│   ├── cmd/                   # config 与维护命令
│   └── common/                # DB、中间件、限流、共享实例
├── pkg/                       # 可被外部复用的无业务基础包
├── api/dist/                  # go:embed 前端产物
├── config.example.yaml
└── config.example.json
```

保持依赖方向：

```text
api/v1 -> biz -> domain service -> domain repo -> GORM
boot -> config/common/biz/api
```

业务规则放 `domain/<name>`，跨领域实例装配放 `biz`，基础设施初始化放 `internal/common`，进程装配放 `internal/boot`。

## 启动流程

```text
main
  -> boot.Start(meta)
  -> mulan.New + SetPreInit + AddFlagSet + BindFlags
  -> vault.Inject(meta.Name)
  -> 隐藏业务运行不需要直接暴露的模块 flags
  -> 注册 mulan-web 服务和根命令
  -> app.Execute
     -> 加载文件 / 环境变量 / flags
     -> initRuntime
        -> config.Validate
        -> log.NewWithConfig
        -> common.Init: DB + Ping + Migrate + Session + APIKey
        -> biz.Init: Repo + Service
     -> web.Service.Serve
```

`boot.Start` 返回错误，由 `main` 写入 stderr 并以状态码 1 退出；启动失败不应 panic。

### 启动装配

```go
func Start(meta *mulan.Meta) error {
	common.Meta = meta

	app = mulan.New(meta, config.Conf).
		SetPreInit(initRuntime).
		AddFlagSet(config.FlagSet()...).
		BindFlags()

	vault.Inject(meta.Name)
	hideInternalFlags(app)
	app.AddService(newWebService(meta))
	registerCommands(app.RootCmd(), meta, zap.L())

	return app.Execute(context.Background())
}
```

Vault 业务逻辑必须留在 `mulan-vault`；模板只调用 `vault.Inject(meta.Name)`。

## 配置

```go
type Config struct {
	Log     log.Config     `json:"log" yaml:"log"`
	Web     web.Config     `json:"web" yaml:"web"`
	DB      dbx.Config     `json:"db" yaml:"db"`
	Apikey  apikey.Config  `json:"apikey" yaml:"apikey"`
	Session session.Config `json:"session" yaml:"session"`
	App     App            `json:"app" yaml:"app"`
}

type App struct {
	Swagger  bool              `json:"swagger" yaml:"swagger"`
	Frontend map[string]string `json:"frontend" yaml:"frontend"`
}
```

`config.FlagSet()` 注册 Log、Web、DB、API Key、Session 和 App flags。Session CLI 使用 `session.header-only`，配置结构使用 `session.header_only`，模板通过 Viper alias 对齐。

### 推荐配置

```yaml
log:
  mode: server
  level: info
  format: json
  adaptors: []

web:
  prefix: "/"
  host: "0.0.0.0"
  port: 8080
  debug: false
  pprof: false
  metrics: false

db:
  dsn: "postgres://user:pass@127.0.0.1:5432/app?sslmode=disable"
  debug: false
  migrate: false
  conn:
    idle: 20
    open: 250
    lifetime: 3600

apikey:
  name: "apikey"
  value: ""
  values: []

session:
  name: "token"
  ttl: 604800
  driver: "memory"
  header_only: true

app:
  swagger: false
  frontend: {}
```

### 启动校验

`config.Validate` 会拒绝：

- 空 `db.dsn`、不支持的 scheme、MySQL/PostgreSQL 缺 host
- 超出 `1..65535` 的 Web 端口
- 不以 `/` 开头的 Web Prefix
- 非回环 `web.host` 上启用 Pprof 或 Metrics
- 未知 Session driver
- FS Session 未配置目录
- Redis Session 缺 host 或有效端口
- `session.header_only = false`
- 未知日志级别
- 外部前端路由无 `/`、目标不存在或与保留路由冲突

注意：`mulan-web` 本身允许在公网监听器启用 Metrics；`webx-ez` 的配置校验更严格，Pprof 和 Metrics 都要求回环地址。

### 配置脱敏

`config show` 使用 `config.Redacted`，会遮盖：

- DB DSN、密码及 Args 中的 pass/secret/token
- API Key 单值与列表
- Session Redis 密码
- 日志 adaptor URL 的 userinfo 与敏感 query

新增 Secret 字段时同步扩展脱敏逻辑和测试。

## Vault 远程配置

无显式 Vault 配置时，启动读取：

```text
VAULT_ADDR
VAULT_ROLE_ID
VAULT_SECRET_ID
VAULT_MOUNT
```

Secret 路径是 KV v2 的 `{mount}/data/{meta.Name}`。配置不完整时远程配置被禁用并记录警告；不影响本地配置继续加载。

## 基础设施初始化

`internal/common.Init(ctx)` 的当前 contract：

1. `db.dsn` 必填
2. `dbx.Auto` 创建 DB
3. `PingContext` 必须成功，否则启动失败
4. `db.migrate = true` 时迁移 `domain.Models`
5. `session.Init` 创建 Session 中间件
6. Header-only 模式再包一层 Cookie 删除护栏
7. `apikey.Mw` 创建 API Key 中间件
8. 初始化共享基础服务

DB 是硬依赖。不要让服务在 DB 未连接时降级启动。

## 路由

所有模板路由先经过：

- 安全响应头
- 1 MiB Request Body 限制

`mulan-web` 固定路由不受 `web.prefix` 分组影响：

```text
GET /version
GET /health
GET /metrics       # web.metrics=true
/pprof/*           # web.pprof=true 且 loopback
```

模板路由位于 `web.prefix` 下：

```text
GET  /                         内嵌 index.html
GET  /static/*filepath         内嵌静态资源；不存在时 404
GET  /favicon.ico
GET  /swagger/*any             app.swagger=true
GET  /apikey/test              API Key 鉴权
GET  /ready                    DB 和 Session 就绪检查

POST /api/auth/captcha
POST /api/auth/login
POST /api/auth/version
POST /api/auth/info            需有效 Session
POST /api/auth/logout          需有效 Session
POST /api/auth/change_password 需有效 Session
```

外部静态目录由 `app.frontend` 以 `route -> path` 映射注册。保留前缀包括 `/api`、`/static`、`/swagger`、`/ready`、`/apikey`、`/favicon.ico`。

## 认证链路

Session 只接受 Header token。客户端可使用：

```http
Authorization: Bearer <token>
X-Token: <token>
```

登录成功后 token 同时出现在响应 JSON 和响应头。模板不读取或写入 Session Cookie。

登录流程：

1. `req.Bind` 校验账号、密码、验证码 ID/答案
2. 按 IP、账号、设备限流
3. 验证一次性验证码
4. `domain/user.Service.Authenticate` 校验启用状态和密码
5. 写入用户 ID、账号、状态、角色与 `auth_version`
6. 保存 Session 并返回 token

受保护请求会重新读取当前用户，并核对 Session 中的 `auth_version` 和角色。用户禁用、角色变化或密码变更后，旧 Session 会被清理。

密码长度按 UTF-8 字节限制为 8 至 72。修改密码使用 `id + auth_version` 乐观并发更新，成功后递增版本并清理当前 Session。

## 领域实现模板

### Handler

```go
func (h *Handler) Handle(c *gin.Context) {
	logger := zap.L().With(web.ZapRequestID(c))
	var input Request
	if err := req.Bind(c, &input); err != nil {
		logger.Error("bind request", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusBadRequest,
			rsp.E(rsp.ErrParamInvalid.WithError(err)))
		return
	}

	item, err := h.service.Run(c.Request.Context(), input)
	if err != nil {
		logger.Error("run service", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, rsp.C(err))
		return
	}
	c.JSON(http.StatusOK, rsp.S(item))
}
```

### Repo

```go
func (r *GormRepo) FindByID(ctx context.Context, id uint64) (*Model, error) {
	var item Model
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
```

约束：

- Handler 只做协议转换、鉴权上下文和响应
- Service 不依赖 `gin.Context`
- Repo 所有 DB 操作使用 `WithContext(ctx)`
- 内部错误用 `rsp.C` 屏蔽，业务错误映射成明确 `AppError`
- Model 的密码等敏感字段使用 `json:"-"`

## 命令

### 配置命令

```bash
./webx-ez config generate
./webx-ez config validate -c config.yaml
./webx-ez config show -c config.yaml
```

- `generate`：输出默认 JSON 模板
- `validate`：加载并校验配置
- `show`：输出脱敏后的生效配置

### 维护命令

维护命令直接注册在根命令：

```bash
./webx-ez migrate -c config.yaml
./webx-ez deploy -c config.yaml
./webx-ez init_data -c config.yaml
./webx-ez reset_admin -c config.yaml
```

- `migrate` / `deploy`：只执行结构迁移
- `init_data`：事务内幂等插入缺失的默认数据，不删除已有数据
- `reset_admin`：生成随机密码、递增 `auth_version`、写入 `.admin.txt`

`User.Defaults` 也会生成管理员密码并写 `.admin.txt`。若管理员已存在，`OnConflict DoNothing` 不会应用新密码，此时文件可能不代表数据库中的现有密码；需要真实重置时使用 `reset_admin`。读取后应安全删除 `.admin.txt`。

### systemd 命令

systemd 操作也是根命令：

```text
install remove enable disable start stop restart reload status kill
```

没有 `systemd` 父命令，不要写成 `webx-ez systemd install`。

## 静态前端与 Swagger

- `api/dist` 通过 `go:embed all:dist` 编入二进制
- `/static/*` 只返回真实存在的文件，不回退 HTML
- `/` 返回内嵌 `index.html`
- 外部前端目录用 `app.frontend` 注册
- Swagger 默认关闭，启用后开放 `/swagger/*any`

前端产物更新后重新构建 Go 二进制。

## 安全与运行约束

- 响应头包含 CSP、`nosniff`、DENY frame、no-referrer 和 Permissions-Policy
- Request Body 上限 1 MiB
- Session 强制 Header-only
- Captcha 与登录分别限流，最多维护 4096 个限流键
- Pprof、Metrics 在模板层只允许回环监听器
- DB 连接与 Ping 失败即拒绝启动
- `/ready` 额外检查 DB Ping 和 Session 初始化
- 配置展示必须走脱敏副本
- Secret、密码 hash、Session token 不写普通日志

## 验证基线

修改模板后至少执行：

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

涉及独立模块行为时，同时在对应模块目录运行其测试。工作区编译通过不代表对应 `<目录>/vX.Y.Z` 标签已经发布或应用的 `go.mod` 已更新。
