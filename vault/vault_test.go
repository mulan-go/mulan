package vault

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := f(r)
	if response != nil && response.Request == nil {
		response.Request = r
	}
	return response, err
}

func newTestVault(t *testing.T, mount string, transport roundTripFunc) *Vault {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = "http://vault.test"
	cfg.HttpClient = &http.Client{Transport: transport}
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatalf("vault.NewClient() error = %v", err)
	}
	return &Vault{
		roleID:       "role",
		secretID:     &approle.SecretID{FromString: "secret"},
		mount:        mount,
		client:       client,
		tokenLock:    make(chan struct{}, 1),
		leaseChanged: make(chan struct{}),
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestInjectContextUsesConfiguredMount(t *testing.T) {
	const (
		mount = "application"
		name  = "service"
	)
	v := newTestVault(t, mount, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.Path, "/v1/"+mount+"/data/"+name; got != want {
			t.Errorf("request path = %q, want %q", got, want)
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{"data":{"data":{"region":"cn-east-1"},"metadata":{}}}`), nil
	})
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := v.InjectContext(context.Background(), name); err != nil {
		t.Fatalf("InjectContext() error = %v", err)
	}
	if got := viper.GetString("region"); got != "cn-east-1" {
		t.Fatalf("injected region = %q, want cn-east-1", got)
	}
}

func TestUnavailableSecret(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{}`},
		{name: "deleted", body: `{"data":{"data":null,"metadata":{"version":2,"deletion_time":"2026-10-08T00:00:00Z","destroyed":false}}}`},
		{name: "destroyed", body: `{"data":{"data":null,"metadata":{"version":2,"deletion_time":"","destroyed":true}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newTestVault(t, defaultMount, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, tt.body), nil
			})
			data, err := v.GetSecret(context.Background(), "service")
			if data != nil || !errors.Is(err, ErrSecretNotFound) {
				t.Errorf("GetSecret() = (%v, %v), want (nil, ErrSecretNotFound)", data, err)
			}
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set("region", "previous-region")
			if err := v.InjectContext(context.Background(), "service"); !errors.Is(err, ErrSecretNotFound) {
				t.Errorf("InjectContext() error = %v, want ErrSecretNotFound", err)
			}
			if got := viper.GetString("region"); got != "previous-region" {
				t.Errorf("region = %q, want previous-region", got)
			}
		})
	}
}

func TestGetSecretAcceptsEmptyData(t *testing.T) {
	v := newTestVault(t, defaultMount, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"data":{},"metadata":{}}}`), nil
	})
	data, err := v.GetSecret(context.Background(), "service")
	if err != nil || data == nil || len(data) != 0 {
		t.Fatalf("GetSecret() = (%v, %v), want non-nil empty map and nil error", data, err)
	}
}

func TestLoginSetsToken(t *testing.T) {
	v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.Path, "/v1/auth/approle/login"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{"auth":{"client_token":"test-token"}}`), nil
	})
	if err := v.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	client, err := v.Client()
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if got := client.Token(); got != "test-token" {
		t.Fatalf("client token = %q, want test-token", got)
	}
}

func TestLoginRejectsEmptyToken(t *testing.T) {
	v := newTestVault(t, defaultMount, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"auth":{}}`), nil
	})

	if err := v.Login(context.Background()); err == nil {
		t.Fatal("Login() error = nil, want empty token error")
	}
}

func TestInjectLogsInBeforeReadingSecret(t *testing.T) {
	const (
		name  = "service"
		mount = "application"
		token = "test-token"
	)

	var requestPaths []string
	v := newTestVault(t, mount, func(r *http.Request) (*http.Response, error) {
		requestPaths = append(requestPaths, r.URL.Path)
		switch r.URL.Path {
		case "/v1/auth/approle/login":
			return jsonResponse(http.StatusOK, `{"auth":{"client_token":"`+token+`"}}`), nil
		case "/v1/" + mount + "/data/" + name:
			if got := r.Header.Get("X-Vault-Token"); got != token {
				t.Errorf("X-Vault-Token = %q, want %q", got, token)
			}
			return jsonResponse(http.StatusOK, `{"data":{"data":{"region":"cn-east-1"},"metadata":{}}}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})

	viper.Reset()
	t.Cleanup(viper.Reset)
	v.Inject(name)

	if got := viper.GetString("region"); got != "cn-east-1" {
		t.Fatalf("injected region = %q, want cn-east-1", got)
	}
	if got, want := strings.Join(requestPaths, ","), "/v1/auth/approle/login,/v1/application/data/service"; got != want {
		t.Fatalf("request paths = %q, want %q", got, want)
	}
}

func TestClientRejectsNilVault(t *testing.T) {
	var v *Vault
	if _, err := v.Client(); !errors.Is(err, ErrNotInitClient) {
		t.Fatalf("Client() error = %v, want ErrNotInitClient", err)
	}
}

func TestInjectWithoutConfigOnlyWarns(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_ROLE_ID", "")
	t.Setenv("VAULT_SECRET_ID", "")
	t.Setenv("VAULT_MOUNT", "")

	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	previousLogger := zap.L()
	zap.ReplaceGlobals(logger)
	t.Cleanup(func() { zap.ReplaceGlobals(previousLogger) })

	Inject("service")

	if logs.Len() != 1 {
		t.Fatalf("warning count = %d, want 1", logs.Len())
	}
	if got := logs.All()[0].Message; got != "vault remote config is disabled" {
		t.Fatalf("warning message = %q, want vault remote config is disabled", got)
	}
}
