// Package oidc implements OpenID Connect authentication for Gin.
package oidc

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	authoauth2 "github.com/mulan-go/mulan/auth/oauth2"
	"github.com/spf13/pflag"
)

const DefaultCookieName = "mulan_oidc_state"

type Config struct {
	IssuerURL           string   `json:"issuer_url" yaml:"issuer_url"`
	ClientID            string   `json:"client_id" yaml:"client_id"`
	ClientSecret        string   `json:"client_secret" yaml:"client_secret"`
	RedirectURL         string   `json:"redirect_url" yaml:"redirect_url"`
	Scopes              []string `json:"scopes" yaml:"scopes"`
	CookieName          string   `json:"cookie_name" yaml:"cookie_name"`
	CookieSecret        string   `json:"cookie_secret" yaml:"cookie_secret"`
	CookieSecure        bool     `json:"cookie_secure" yaml:"cookie_secure"`
	StateTTL            int      `json:"state_ttl" yaml:"state_ttl"`
	SuccessURL          string   `json:"success_url" yaml:"success_url"`
	FetchUserInfo       bool     `json:"fetch_userinfo" yaml:"fetch_userinfo"`
	AllowInsecureIssuer bool     `json:"allow_insecure_issuer" yaml:"allow_insecure_issuer"`
}

func (c *Config) FlagSet() *pflag.FlagSet { return FlagSet() }

func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("oidc", pflag.ContinueOnError)
	fs.String("oidc.issuer-url", "", "OIDC issuer URL")
	fs.String("oidc.client-id", "", "OIDC client ID")
	fs.String("oidc.client-secret", "", "OIDC client secret")
	fs.String("oidc.redirect-url", "", "OIDC callback URL")
	fs.StringSlice("oidc.scopes", []string{"openid", "profile", "email"}, "OIDC scopes")
	fs.String("oidc.cookie-name", DefaultCookieName, "OIDC transient state cookie name")
	fs.String("oidc.cookie-secret", "", "OIDC state signing secret (at least 32 bytes)")
	fs.Bool("oidc.cookie-secure", false, "always mark OIDC state cookie Secure")
	fs.Int("oidc.state-ttl", int(10*time.Minute/time.Second), "OIDC state lifetime in seconds")
	fs.String("oidc.success-url", "/", "OIDC post-login redirect path")
	fs.Bool("oidc.fetch-userinfo", false, "fetch and validate OIDC UserInfo")
	fs.Bool("oidc.allow-insecure-issuer", false, "allow a non-HTTPS issuer URL")
	return fs
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("oidc: config is nil")
	}
	for name, value := range map[string]string{
		"issuer_url":    c.IssuerURL,
		"client_id":     c.ClientID,
		"redirect_url":  c.RedirectURL,
		"cookie_secret": c.CookieSecret,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("oidc: %s is required", name)
		}
	}
	issuer, err := url.Parse(c.IssuerURL)
	if err != nil || issuer.Host == "" {
		return errors.New("oidc: issuer_url must be an absolute URL")
	}
	if issuer.Scheme != "https" && !c.AllowInsecureIssuer && !isLoopbackHost(issuer.Hostname()) {
		return errors.New("oidc: issuer_url must use HTTPS")
	}
	if issuer.Scheme != "http" && issuer.Scheme != "https" {
		return errors.New("oidc: issuer_url scheme must be http or https")
	}
	if issuer.RawQuery != "" || issuer.Fragment != "" {
		return errors.New("oidc: issuer_url cannot contain a query or fragment")
	}
	if !slices.Contains(c.Scopes, "openid") && len(c.Scopes) > 0 {
		return errors.New("oidc: scopes must contain openid")
	}
	endpointConfig := &authoauth2.Config{
		ClientID:     c.ClientID,
		RedirectURL:  c.RedirectURL,
		AuthURL:      "https://placeholder.invalid/authorize",
		TokenURL:     "https://placeholder.invalid/token",
		CookieName:   c.CookieName,
		CookieSecret: c.CookieSecret,
		StateTTL:     c.StateTTL,
		SuccessURL:   c.SuccessURL,
	}
	if err := endpointConfig.Validate(); err != nil {
		return fmt.Errorf("oidc: invalid client configuration: %w", err)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizedScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email"}
	}
	return append([]string(nil), scopes...)
}
