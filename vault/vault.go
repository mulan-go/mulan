package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	tokenRenewInterval  = 12 * time.Hour
	tokenRenewIncrement = 24 * time.Hour
)

var (
	vaultStd   *Vault
	vaultStdMu sync.RWMutex

	ErrInvalidConfig  = errors.New("invalid vault config")
	ErrNotInitClient  = errors.New("vault client is not initialized")
	ErrSecretNotFound = vault.ErrSecretNotFound
)

type Vault struct {
	roleID   string
	secretID *approle.SecretID
	mount    string
	client   *vault.Client
}

func Init() (*Vault, error) {
	return NewVault(&Config{
		Addr:     os.Getenv("VAULT_ADDR"),
		RoleID:   os.Getenv("VAULT_ROLE_ID"),
		SecretID: os.Getenv("VAULT_SECRET_ID"),
		Mount:    os.Getenv("VAULT_MOUNT"),
	})
}

func NewVault(cfg *Config) (*Vault, error) {
	clientConfig, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	client, err := vault.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}

	v := &Vault{
		roleID:   cfg.RoleID,
		secretID: &approle.SecretID{FromString: cfg.SecretID},
		mount:    cfg.mount(),
		client:   client,
	}
	setDefault(v)
	return v, nil
}

func (v *Vault) Client() (*vault.Client, error) {
	if v == nil || v.client == nil {
		return nil, ErrNotInitClient
	}
	return v.client, nil
}

func (v *Vault) Inject(name string) {
	ctx := context.Background()
	if err := v.Login(ctx); err != nil {
		zap.L().Warn("failed to login vault", zap.Error(err))
		return
	}
	err := v.InjectContext(ctx, name)
	if err != nil {
		zap.L().Warn("failed to inject vault config",
			zap.String("name", name),
			zap.Error(err),
		)
	}
}

func (v *Vault) InjectContext(
	ctx context.Context,
	name string,
) error {
	client, err := v.Client()
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("read vault config: secret name is required")
	}

	secret, err := client.KVv2(v.mount).Get(ctx, name)
	if err != nil {
		return fmt.Errorf("read vault config %q: %w", name, err)
	}
	if secret == nil {
		return fmt.Errorf("%w: %q", ErrSecretNotFound, name)
	}

	for key, value := range secret.Data {
		viper.Set(key, value)
	}
	return nil
}

func (v *Vault) Login(ctx context.Context) error {
	client, err := v.Client()
	if err != nil {
		return err
	}

	auth, err := approle.NewAppRoleAuth(v.roleID, v.secretID)
	if err != nil {
		return fmt.Errorf("create AppRole auth: %w", err)
	}
	authInfo, err := client.Auth().Login(ctx, auth)
	if err != nil {
		return fmt.Errorf("AppRole login: %w", err)
	}
	token, err := authInfo.TokenID()
	if err != nil {
		return fmt.Errorf("read AppRole token: %w", err)
	}
	if token == "" {
		return errors.New("AppRole login returned an empty token")
	}
	client.SetToken(token)
	return nil
}

// KeepAlive 启动后台 goroutine 定期续期 Token，续期失败时自动 re-login。
// 适用于需要长期保持 Vault 会话的服务（如 cicd）；一次性登录取配置的服务无需调用。
func (v *Vault) KeepAlive(ctx context.Context) {
	if _, err := v.Client(); err != nil {
		zap.L().Warn("failed to keep vault token alive", zap.Error(err))
		return
	}
	go v.keepTokenAlive(ctx)
}

func (v *Vault) reLogin(ctx context.Context) error {
	if err := v.Login(ctx); err != nil {
		return err
	}
	zap.L().Info("vault token re-acquired via AppRole login")
	return nil
}

func (v *Vault) keepTokenAlive(ctx context.Context) {
	ticker := time.NewTicker(tokenRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := v.client.Auth().Token().RenewSelfWithContext(ctx, int(tokenRenewIncrement/time.Second))
			if err == nil {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			zap.L().Warn("vault token renew failed, attempting re-login", zap.Error(err))
			if reErr := v.reLogin(ctx); reErr != nil {
				zap.L().Warn("vault re-login failed", zap.Error(reErr))
			}
		}
	}
}

func setDefault(v *Vault) {
	vaultStdMu.Lock()
	vaultStd = v
	vaultStdMu.Unlock()
}

// Export

func GetVault() *Vault {
	vaultStdMu.RLock()
	defer vaultStdMu.RUnlock()
	return vaultStd
}

func Inject(name string, configs ...*Config) {
	var (
		v   *Vault
		err error
	)
	if len(configs) == 0 || configs[0] == nil {
		v, err = Init()
	} else {
		v, err = NewVault(configs[0])
	}
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			zap.L().Warn("vault remote config is disabled",
				zap.Error(err),
			)
			return
		}
		zap.L().Warn("failed to initialize vault",
			zap.Error(err),
		)
		return
	}
	v.Inject(name)
}
