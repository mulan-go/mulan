package oidc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	authoauth2 "github.com/mulan-go/mulan/auth/oauth2"
	xoauth2 "golang.org/x/oauth2"
)

const (
	CtxKeyIdentity     = "github.com/mulan-go/mulan/auth/oidc/identity"
	DefaultHTTPTimeout = 15 * time.Second
)

var (
	ErrMissingIDToken = errors.New("oidc: token response does not contain an ID token")
	ErrInvalidIDToken = errors.New("oidc: ID token verification failed")
	ErrInvalidNonce   = errors.New("oidc: ID token nonce is invalid")
	ErrUserInfo       = errors.New("oidc: UserInfo verification failed")
)

type Identity struct {
	Token      *xoauth2.Token
	IDToken    *gooidc.IDToken
	RawIDToken string
	Claims     map[string]any
	UserInfo   map[string]any
	ReturnTo   string
}

type CallbackFunc func(*gin.Context, *Identity)

type Option func(*options)

type options struct {
	httpClient *http.Client
}

func WithHTTPClient(client *http.Client) Option {
	return func(opts *options) { opts.httpClient = client }
}

type Authenticator struct {
	config     Config
	provider   *gooidc.Provider
	verifier   *gooidc.IDTokenVerifier
	flow       *authoauth2.Client
	httpClient *http.Client
}

func New(ctx context.Context, cfg *Config, options ...Option) (*Authenticator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	opts := optionsFrom(options)
	if opts.httpClient == nil {
		opts.httpClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	ctx = gooidc.ClientContext(ctx, opts.httpClient)
	issuer := strings.TrimSpace(cfg.IssuerURL)
	provider, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover provider: %w", err)
	}

	normalized := *cfg
	normalized.IssuerURL = issuer
	normalized.ClientID = strings.TrimSpace(normalized.ClientID)
	normalized.RedirectURL = strings.TrimSpace(normalized.RedirectURL)
	normalized.Scopes = normalizedScopes(normalized.Scopes)
	if normalized.CookieName = strings.TrimSpace(normalized.CookieName); normalized.CookieName == "" {
		normalized.CookieName = DefaultCookieName
	}

	endpoint := provider.Endpoint()
	flowConfig := &authoauth2.Config{
		ClientID:               normalized.ClientID,
		ClientSecret:           normalized.ClientSecret,
		RedirectURL:            normalized.RedirectURL,
		AuthURL:                endpoint.AuthURL,
		TokenURL:               endpoint.TokenURL,
		AuthStyle:              endpoint.AuthStyle,
		Scopes:                 normalized.Scopes,
		CookieName:             normalized.CookieName,
		CookieSecret:           normalized.CookieSecret,
		CookieSecure:           normalized.CookieSecure,
		StateTTL:               normalized.StateTTL,
		SuccessURL:             normalized.SuccessURL,
		AllowInsecureEndpoints: normalized.AllowInsecureIssuer,
	}
	flowOptions := []authoauth2.Option{authoauth2.WithHTTPClient(opts.httpClient)}
	flow, err := authoauth2.New(flowConfig, flowOptions...)
	if err != nil {
		return nil, err
	}
	return &Authenticator{
		config:     normalized,
		provider:   provider,
		verifier:   provider.Verifier(&gooidc.Config{ClientID: normalized.ClientID}),
		flow:       flow,
		httpClient: opts.httpClient,
	}, nil
}

func optionsFrom(values []Option) options {
	var opts options
	for _, option := range values {
		if option != nil {
			option(&opts)
		}
	}
	return opts
}

func (a *Authenticator) Config() Config {
	cfg := a.config
	cfg.Scopes = append([]string(nil), a.config.Scopes...)
	return cfg
}

func (a *Authenticator) OAuth2Config() xoauth2.Config { return a.flow.OAuth2Config() }

func (a *Authenticator) AuthorizationURL(c *gin.Context, returnTo string, options ...xoauth2.AuthCodeOption) (string, error) {
	nonce, err := authoauth2.RandomToken()
	if err != nil {
		return "", err
	}
	options = append(options, xoauth2.SetAuthURLParam("nonce", nonce))
	return a.flow.AuthorizationURL(c, &authoauth2.AuthorizationRequest{
		ReturnTo: returnTo,
		Nonce:    nonce,
		Options:  options,
	})
}

func (a *Authenticator) LoginHandler(options ...xoauth2.AuthCodeOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		authURL, err := a.AuthorizationURL(c, c.Query("return_to"), options...)
		if err != nil {
			abort(c, err)
			return
		}
		c.Redirect(http.StatusFound, authURL)
	}
}

func (a *Authenticator) Authenticate(c *gin.Context) (*Identity, error) {
	result, err := a.flow.Exchange(c)
	if err != nil {
		return nil, err
	}
	rawIDToken, ok := result.Token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return nil, ErrMissingIDToken
	}
	ctx := c.Request.Context()
	if a.httpClient != nil {
		ctx = gooidc.ClientContext(ctx, a.httpClient)
	}
	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidIDToken, err)
	}
	if result.Nonce == "" || idToken.Nonce == "" || subtle.ConstantTimeCompare([]byte(result.Nonce), []byte(idToken.Nonce)) != 1 {
		return nil, ErrInvalidNonce
	}
	claims, err := decodeIDTokenClaims(idToken)
	if err != nil {
		return nil, fmt.Errorf("%w: decode claims: %v", ErrInvalidIDToken, err)
	}
	identity := &Identity{
		Token:      result.Token,
		IDToken:    idToken,
		RawIDToken: rawIDToken,
		Claims:     claims,
		ReturnTo:   result.ReturnTo,
	}
	if a.config.FetchUserInfo {
		userInfo, err := a.provider.UserInfo(ctx, xoauth2.StaticTokenSource(result.Token))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUserInfo, err)
		}
		if userInfo.Subject == "" || subtle.ConstantTimeCompare([]byte(userInfo.Subject), []byte(idToken.Subject)) != 1 {
			return nil, fmt.Errorf("%w: subject does not match ID token", ErrUserInfo)
		}
		identity.UserInfo, err = decodeUserInfoClaims(userInfo)
		if err != nil {
			return nil, fmt.Errorf("%w: decode claims: %v", ErrUserInfo, err)
		}
	}
	return identity, nil
}

func decodeIDTokenClaims(token *gooidc.IDToken) (map[string]any, error) {
	var raw json.RawMessage
	if err := token.Claims(&raw); err != nil {
		return nil, err
	}
	return decodeClaims(raw)
}

func decodeUserInfoClaims(userInfo *gooidc.UserInfo) (map[string]any, error) {
	var raw json.RawMessage
	if err := userInfo.Claims(&raw); err != nil {
		return nil, err
	}
	return decodeClaims(raw)
}

func decodeClaims(raw []byte) (map[string]any, error) {
	claims := make(map[string]any)
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (a *Authenticator) CallbackHandler(next CallbackFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := a.Authenticate(c)
		if err != nil {
			abort(c, err)
			return
		}
		c.Set(CtxKeyIdentity, identity)
		if next != nil {
			next(c, identity)
			return
		}
		finish(c, identity.ReturnTo)
	}
}

func IdentityFromContext(c *gin.Context) (*Identity, bool) {
	value, ok := c.Get(CtxKeyIdentity)
	if !ok {
		return nil, false
	}
	identity, ok := value.(*Identity)
	return identity, ok
}

func abort(c *gin.Context, err error) {
	if errors.Is(err, authoauth2.ErrInvalidState) ||
		errors.Is(err, authoauth2.ErrMissingCode) ||
		errors.Is(err, authoauth2.ErrProvider) ||
		errors.Is(err, authoauth2.ErrExchange) {
		authoauth2.Abort(c, err)
		return
	}
	status := http.StatusBadRequest
	code := "invalid_id_token"
	switch {
	case errors.Is(err, ErrMissingIDToken):
		code = "missing_id_token"
	case errors.Is(err, ErrInvalidNonce):
		code = "invalid_nonce"
	case errors.Is(err, ErrUserInfo):
		status = http.StatusBadGateway
		code = "userinfo_failed"
	case errors.Is(err, ErrInvalidIDToken):
		code = "invalid_id_token"
	default:
		status = http.StatusInternalServerError
		code = "oidc_failed"
	}
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}

func finish(c *gin.Context, returnTo string) {
	if returnTo != "" {
		c.Redirect(http.StatusFound, returnTo)
		return
	}
	c.Status(http.StatusNoContent)
}
