package vault

import (
	"errors"
	"testing"
)

func TestConfigBuild(t *testing.T) {
	cfg := &Config{
		Addr:     "https://vault.example.com",
		RoleID:   "role",
		SecretID: "secret",
	}

	clientConfig, err := cfg.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if clientConfig.Address != cfg.Addr {
		t.Fatalf("Build() address = %q, want %q", clientConfig.Address, cfg.Addr)
	}
	if got := cfg.mount(); got != defaultMount {
		t.Fatalf("mount() = %q, want %q", got, defaultMount)
	}
}

func TestConfigBuildRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "nil"},
		{name: "missing address", cfg: &Config{RoleID: "role", SecretID: "secret"}},
		{name: "missing role", cfg: &Config{Addr: "https://vault.example.com", SecretID: "secret"}},
		{name: "missing secret", cfg: &Config{Addr: "https://vault.example.com", RoleID: "role"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cfg.Build()
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Build() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestNewVaultUsesProvidedConfig(t *testing.T) {
	cfg := &Config{
		Addr:     "https://vault.example.com",
		RoleID:   "configured-role",
		SecretID: "configured-secret",
		Mount:    "application",
	}

	v, err := NewVault(cfg)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	client, err := v.Client()
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if got := client.Address(); got != cfg.Addr {
		t.Fatalf("client address = %q, want %q", got, cfg.Addr)
	}
	if v.roleID != cfg.RoleID || v.secretID.FromString != cfg.SecretID || v.mount != cfg.Mount {
		t.Fatalf("NewVault() did not preserve the provided config")
	}
	if GetVault() != v {
		t.Fatalf("GetVault() did not return the new client")
	}
}

func TestInitUsesEnvironmentConfig(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.example.com")
	t.Setenv("VAULT_ROLE_ID", "environment-role")
	t.Setenv("VAULT_SECRET_ID", "environment-secret")
	t.Setenv("VAULT_MOUNT", "environment-mount")

	v, err := Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	client, err := v.Client()
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if got := client.Address(); got != "https://vault.example.com" {
		t.Fatalf("client address = %q, want https://vault.example.com", got)
	}
	if v.roleID != "environment-role" || v.secretID.FromString != "environment-secret" || v.mount != "environment-mount" {
		t.Fatal("Init() did not preserve the environment config")
	}
}
