// Package oauth2 implements a secure OAuth 2.0 authorization-code flow for Gin.
package oauth2

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/pflag"
	xoauth2 "golang.org/x/oauth2"
)

const (
	DefaultCookieName = "mulan_oauth2_state"
	DefaultStateTTL   = 10 * time.Minute
	minimumSecretSize = 32
)

type Config struct {
	ClientID               string            `json:"client_id" yaml:"client_id"`
	ClientSecret           string            `json:"client_secret" yaml:"client_secret"`
	RedirectURL            string            `json:"redirect_url" yaml:"redirect_url"`
	AuthURL                string            `json:"auth_url" yaml:"auth_url"`
	TokenURL               string            `json:"token_url" yaml:"token_url"`
	UserInfoURL            string            `json:"userinfo_url" yaml:"userinfo_url"`
	Scopes                 []string          `json:"scopes" yaml:"scopes"`
	AuthStyle              xoauth2.AuthStyle `json:"auth_style" yaml:"auth_style"`
	CookieName             string            `json:"cookie_name" yaml:"cookie_name"`
	CookieSecret           string            `json:"cookie_secret" yaml:"cookie_secret"`
	CookieSecure           bool              `json:"cookie_secure" yaml:"cookie_secure"`
	StateTTL               int               `json:"state_ttl" yaml:"state_ttl"`
	SuccessURL             string            `json:"success_url" yaml:"success_url"`
	AllowInsecureEndpoints bool              `json:"allow_insecure_endpoints" yaml:"allow_insecure_endpoints"`
}

func (c *Config) FlagSet() *pflag.FlagSet { return FlagSet() }

func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("oauth2", pflag.ContinueOnError)
	fs.String("oauth2.client-id", "", "OAuth2 client ID")
	fs.String("oauth2.client-secret", "", "OAuth2 client secret")
	fs.String("oauth2.redirect-url", "", "OAuth2 callback URL")
	fs.String("oauth2.auth-url", "", "OAuth2 authorization endpoint")
	fs.String("oauth2.token-url", "", "OAuth2 token endpoint")
	fs.String("oauth2.userinfo-url", "", "OAuth2 user info endpoint")
	fs.StringSlice("oauth2.scopes", nil, "OAuth2 scopes")
	fs.Int("oauth2.auth-style", int(xoauth2.AuthStyleAutoDetect), "OAuth2 token endpoint auth style")
	fs.String("oauth2.cookie-name", DefaultCookieName, "OAuth2 transient state cookie name")
	fs.String("oauth2.cookie-secret", "", "OAuth2 state signing secret (at least 32 bytes)")
	fs.Bool("oauth2.cookie-secure", false, "always mark OAuth2 state cookie Secure")
	fs.Int("oauth2.state-ttl", int(DefaultStateTTL/time.Second), "OAuth2 state lifetime in seconds")
	fs.String("oauth2.success-url", "/", "OAuth2 post-login redirect path")
	fs.Bool("oauth2.allow-insecure-endpoints", false, "allow non-HTTPS OAuth2 endpoints")
	return fs
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("oauth2: config is nil")
	}
	for name, value := range map[string]string{
		"client_id":     c.ClientID,
		"redirect_url":  c.RedirectURL,
		"auth_url":      c.AuthURL,
		"token_url":     c.TokenURL,
		"cookie_secret": c.CookieSecret,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("oauth2: %s is required", name)
		}
	}
	if len(c.CookieSecret) < minimumSecretSize {
		return fmt.Errorf("oauth2: cookie_secret must contain at least %d bytes", minimumSecretSize)
	}
	if c.CookieName != "" {
		if err := (&http.Cookie{Name: c.CookieName, Value: "state"}).Valid(); err != nil {
			return fmt.Errorf("oauth2: invalid cookie_name: %w", err)
		}
	}
	for name, value := range map[string]string{
		"redirect_url": c.RedirectURL,
		"auth_url":     c.AuthURL,
		"token_url":    c.TokenURL,
	} {
		if err := validateAbsoluteURL(value, c.AllowInsecureEndpoints); err != nil {
			return fmt.Errorf("oauth2: invalid %s: %w", name, err)
		}
	}
	redirectURL, _ := url.Parse(c.RedirectURL)
	if redirectURL.Fragment != "" {
		return errors.New("oauth2: redirect_url cannot contain a fragment")
	}
	if c.UserInfoURL != "" {
		if err := validateAbsoluteURL(c.UserInfoURL, c.AllowInsecureEndpoints); err != nil {
			return fmt.Errorf("oauth2: invalid userinfo_url: %w", err)
		}
	}
	if c.StateTTL < 0 {
		return errors.New("oauth2: state_ttl cannot be negative")
	}
	switch c.AuthStyle {
	case xoauth2.AuthStyleAutoDetect, xoauth2.AuthStyleInParams, xoauth2.AuthStyleInHeader:
	default:
		return errors.New("oauth2: auth_style is invalid")
	}
	if c.SuccessURL != "" && safeReturnTo(c.SuccessURL) == "" {
		return errors.New("oauth2: success_url must be an absolute-path reference")
	}
	return nil
}

func validateAbsoluteURL(value string, allowHTTP bool) error {
	u, err := url.Parse(value)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("host is required")
	}
	if u.User != nil {
		return errors.New("embedded URL credentials are not allowed")
	}
	if u.Fragment != "" {
		return errors.New("fragment is not allowed")
	}
	if u.Scheme != "https" && !allowHTTP && !isLoopbackHost(u.Hostname()) {
		return errors.New("HTTPS is required")
	}
	return nil
}

func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(value, "//") || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") {
		return ""
	}
	return u.String()
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
