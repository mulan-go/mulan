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
	tokenRenewIncrement = 24 * time.Hour
	tokenRetryInterval  = 5 * time.Second
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

	// Serialize token updates while allowing waiting callers to cancel.
	tokenLock chan struct{}
	lease     *tokenLease
}

type tokenLease struct {
	expiresAt time.Time
	renewable bool
	changed   chan struct{}
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
		roleID:    cfg.RoleID,
		secretID:  &approle.SecretID{FromString: cfg.SecretID},
		mount:     cfg.mount(),
		client:    client,
		tokenLock: make(chan struct{}, 1),
	}
	setDefault(v)
	return v, nil
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

func (v *Vault) GetSecret(
	ctx context.Context,
	name string,
) (map[string]any, error) {
	client, err := v.Client()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("read vault config: secret name is required")
	}
	secret, err := client.KVv2(v.mount).Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("read vault config %q: %w", name, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("%w: %q", ErrSecretNotFound, name)
	}
	return secret.Data, nil
}

func (v *Vault) InjectContext(
	ctx context.Context,
	name string,
) error {
	data, err := v.GetSecret(ctx, name)
	if err != nil {
		return err
	}
	for key, value := range data {
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
	if err := v.lockToken(ctx); err != nil {
		return err
	}
	defer v.unlockToken()
	startedAt := time.Now()
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
	v.recordTokenLease(authInfo.Auth, startedAt)
	return nil
}

// KeepAlive 启动后台 goroutine 按实际 TTL 提前续期 Token，不可续期或续期失败时重新登录。
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
	var retry bool
	var failedLease *tokenLease
	for ctx.Err() == nil {
		if err := v.lockToken(ctx); err != nil {
			return
		}
		lease := v.lease
		v.unlockToken()

		var timer *time.Timer
		var renewalDue <-chan time.Time
		var changed <-chan struct{}
		if lease != nil {
			changed = lease.changed
		}
		if retry && lease == failedLease {
			delay := tokenRetryInterval
			if lease != nil && lease.renewable {
				if remaining := time.Until(lease.expiresAt); remaining > 0 {
					delay = min(delay, max(remaining/2, time.Nanosecond))
				}
			}
			timer = time.NewTimer(delay)
		} else if lease != nil && !lease.expiresAt.IsZero() {
			timer = time.NewTimer(max(time.Until(lease.expiresAt)/2, 0))
		}
		if timer != nil {
			renewalDue = timer.C
		}
		// Non-expiring tokens only wait for cancellation or a replacement lease.
		// Without a lease, log in immediately unless a previous attempt failed.
		if lease != nil || timer != nil {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case <-changed:
				if timer != nil {
					timer.Stop()
				}
				continue
			case <-renewalDue:
				timer.Stop()
			}
		}
		if ctx.Err() != nil {
			return
		}
		if lease != nil && lease.renewable && time.Now().Before(lease.expiresAt) {
			if err := v.renewToken(ctx, lease); err == nil {
				retry = false
				continue
			} else if ctx.Err() != nil {
				return
			} else {
				zap.L().Warn("vault token renew failed, attempting re-login", zap.Error(err))
			}
		}
		if err := v.reLogin(ctx); err == nil {
			retry = false
			continue
		} else if ctx.Err() != nil {
			return
		} else {
			zap.L().Warn("vault re-login failed", zap.Error(err))
		}
		retry = true
		failedLease = lease
	}
}

func (v *Vault) renewToken(ctx context.Context, lease *tokenLease) error {
	if err := v.lockToken(ctx); err != nil {
		return err
	}
	defer v.unlockToken()
	if v.lease != lease {
		return nil
	}
	startedAt := time.Now()
	secret, err := v.client.Auth().Token().RenewSelfWithContext(ctx, int(tokenRenewIncrement/time.Second))
	if err != nil {
		return err
	}
	if secret == nil || secret.Auth == nil {
		return errors.New("vault token renewal returned no auth data")
	}
	v.recordTokenLease(secret.Auth, startedAt)
	return nil
}

// recordTokenLease must be called with tokenLock held. Use the request start time
// to avoid overestimating the remaining TTL when the response is delayed.
func (v *Vault) recordTokenLease(auth *vault.SecretAuth, startedAt time.Time) {
	lease := &tokenLease{
		renewable: auth.Renewable,
		changed:   make(chan struct{}),
	}
	if auth.LeaseDuration > 0 {
		lease.expiresAt = startedAt.Add(time.Duration(auth.LeaseDuration) * time.Second)
	}
	if v.lease != nil {
		close(v.lease.changed)
	}
	v.lease = lease
}

func (v *Vault) lockToken(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case v.tokenLock <- struct{}{}:
		// Both cases can be ready when cancellation races with acquisition.
		if err := ctx.Err(); err != nil {
			v.unlockToken()
			return err
		}
		return nil
	}
}

func (v *Vault) unlockToken() {
	<-v.tokenLock
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

func (v *Vault) Mount() string { return v.mount }

func (v *Vault) Client() (*vault.Client, error) {
	if v == nil || v.client == nil {
		return nil, ErrNotInitClient
	}
	return v.client, nil
}
