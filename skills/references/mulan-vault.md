# Mulan Vault - 远程配置注入

`mulan-vault` 使用 HashiCorp Vault AppRole 登录，从 KV v2 读取配置并写入全局 Viper。

## 安装与导入

```bash
go get github.com/mulan-go/mulan/vault
```

```go
import "github.com/mulan-go/mulan/vault"
```

## 配置

```go
type Config struct {
	Addr     string
	RoleID   string
	SecretID string
	Mount    string
}
```

`Mount` 为空时默认使用 `config`。无参数初始化读取：

```text
VAULT_ADDR
VAULT_ROLE_ID
VAULT_SECRET_ID
VAULT_MOUNT
```

`Addr`、`RoleID`、`SecretID` 必填。缺失时返回 `ErrInvalidConfig`。

## 应用层接入

应用启动层只保留一次薄调用：

```go
app := mulan.New(meta, config.Conf).
	SetPreInit(initRuntime).
	AddFlagSet(config.FlagSet()...).
	BindFlags()

vault.Inject(meta.Name)

if err := app.Execute(context.Background()); err != nil {
	return err
}
```

调用链：

```text
vault.Inject(name)
  -> 从 VAULT_* 构建客户端
  -> AppRole Login
  -> KV v2 Get(mount, name)
  -> viper.Set(key, value)
```

Vault 登录、令牌设置、KV 路径和配置写入都留在 `mulan-vault`，不要在应用 `boot` 包复制。

## 对外 API

```go
func Init() (*Vault, error)
func NewVault(cfg *Config) (*Vault, error)
func GetVault() *Vault
func Inject(name string, configs ...*Config)

func (v *Vault) Client() (*vault.Client, error)
func (v *Vault) Login(ctx context.Context) error
func (v *Vault) Inject(name string)
func (v *Vault) InjectContext(ctx context.Context, name string) error
func (v *Vault) KeepAlive(ctx context.Context)
```

### `Inject`

```go
vault.Inject("webx-ez")
```

- 无显式配置时读取 `VAULT_*`
- 配置缺失时记录 `vault remote config is disabled` 警告并返回
- 初始化、登录或读取失败时记录警告，不 panic，不返回错误
- 成功时把 Secret `Data` 的每个键写入 Viper

需要调用方处理错误时，使用显式 API：

```go
v, err := vault.NewVault(cfg)
if err != nil {
	return err
}
if err := v.Login(ctx); err != nil {
	return err
}
if err := v.InjectContext(ctx, serviceName); err != nil {
	return err
}
```

`InjectContext` 只读取配置，不自动登录；`(*Vault).Inject` 会先登录再读取。

## 错误

```go
var (
	ErrInvalidConfig  error
	ErrNotInitClient  error
	ErrSecretNotFound error
)
```

- 空 Secret 名称会返回参数错误
- KV Secret 不存在、已删除或已销毁时包装 `ErrSecretNotFound`；合法的空 Data map 可以正常读取
- nil 或未初始化客户端返回 `ErrNotInitClient`

## 长连接令牌续期

一次性启动配置注入无需调用 `KeepAlive`。长期使用 Vault 客户端时：

```go
if err := v.Login(ctx); err != nil {
	return err
}
v.KeepAlive(ctx)
```

后台任务在 Token 剩余 TTL 的一半处尝试续期，请求增量为 24 小时，但后续调度以 Vault 实际返回的 TTL 为准。不可续期或已过期的 Token 会重新执行 AppRole 登录；续期失败时也会重新登录。登录失败后最多等待 5 秒重试；已有 Token 仍有效且可续期时，会再次尝试续期，等待时间不超过其剩余 TTL 的一半。后续 `Login` 替换 Token 时立即按新 TTL 重新调度。TTL 为 0 的 Token 不会过期，无需续期。`ctx` 取消后停止等待（包括等待其他 Token 请求完成）并取消正在进行的请求。

## 配置优先级与安全约束

- `viper.Set` 的值优先级高于配置文件、环境变量和默认值
- Vault 凭据只从专用 `VAULT_*` 环境变量或显式 `Config` 传入
- 不记录 `RoleID`、`SecretID`、Token 或 Secret 数据
- Secret 名称通常使用 `meta.Name`，保持部署路径稳定
- `GetVault` 与默认实例替换有并发保护
