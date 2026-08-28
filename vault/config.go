package vault

import (
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

const defaultMount = "config"

type Config struct {
	Addr     string
	RoleID   string
	SecretID string
	Mount    string
}

func (c *Config) Build() (*vault.Config, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	cfg := vault.DefaultConfig()
	if cfg.Error != nil {
		return nil, fmt.Errorf("build vault client config: %w", cfg.Error)
	}
	cfg.Address = c.Addr
	return cfg, nil
}

func (c *Config) validate() error {
	if c == nil {
		return fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	if c.Addr == "" {
		return fmt.Errorf("%w: addr is required", ErrInvalidConfig)
	}
	if c.RoleID == "" {
		return fmt.Errorf("%w: role ID is required", ErrInvalidConfig)
	}
	if c.SecretID == "" {
		return fmt.Errorf("%w: secret ID is required", ErrInvalidConfig)
	}
	return nil
}

func (c *Config) mount() string {
	if c.Mount == "" {
		return defaultMount
	}
	return c.Mount
}
