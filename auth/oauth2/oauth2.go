package oauth2

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	xoauth2 "golang.org/x/oauth2"
)

const (
	CtxKeyIdentity     = "github.com/mulan-go/mulan/auth/oauth2/identity"
	DefaultHTTPTimeout = 15 * time.Second
	maxUserInfoSize    = 1 << 20
)

var (
	ErrInvalidState = errors.New("oauth2: invalid state")
	ErrMissingCode  = errors.New("oauth2: authorization code is missing")
	ErrProvider     = errors.New("oauth2: provider rejected authorization")
	ErrExchange     = errors.New("oauth2: token exchange failed")
	ErrUserInfo     = errors.New("oauth2: user info request failed")
)

type ProviderError struct {
	Code        string
	Description string
	URI         string
}

func (e *ProviderError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("%v: %s", ErrProvider, e.Code)
	}
	return fmt.Sprintf("%v: %s: %s", ErrProvider, e.Code, e.Description)
}

func (e *ProviderError) Unwrap() error { return ErrProvider }

type AuthorizationRequest struct {
	ReturnTo string
	Nonce    string
	Options  []xoauth2.AuthCodeOption
}

type Result struct {
	Token    *xoauth2.Token
	ReturnTo string
	Nonce    string
}

type Identity struct {
	*Result
	UserInfo map[string]any
}

type CallbackFunc func(*gin.Context, *Identity)

type Option func(*clientOptions)

type clientOptions struct {
	httpClient *http.Client
	now        func() time.Time
}

func WithHTTPClient(client *http.Client) Option {
	return func(opts *clientOptions) { opts.httpClient = client }
}

// WithClock is intended for deterministic expiry testing.
func WithClock(now func() time.Time) Option {
	return func(opts *clientOptions) { opts.now = now }
}

type Client struct {
	config       Config
	oauth2Config xoauth2.Config
	httpClient   *http.Client
	now          func() time.Time
	stateTTL     time.Duration
	cookieSecure bool
}

type statePayload struct {
	Version   int    `json:"v"`
	State     string `json:"state"`
	Verifier  string `json:"verifier"`
	Nonce     string `json:"nonce,omitempty"`
	ReturnTo  string `json:"return_to,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
}

func New(cfg *Config, options ...Option) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := clientOptions{now: time.Now}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.now == nil {
		return nil, errors.New("oauth2: clock is nil")
	}
	if opts.httpClient == nil {
		opts.httpClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}

	normalized := *cfg
	normalized.ClientID = strings.TrimSpace(normalized.ClientID)
	normalized.RedirectURL = strings.TrimSpace(normalized.RedirectURL)
	normalized.AuthURL = strings.TrimSpace(normalized.AuthURL)
	normalized.TokenURL = strings.TrimSpace(normalized.TokenURL)
	normalized.UserInfoURL = strings.TrimSpace(normalized.UserInfoURL)
	if normalized.CookieName = strings.TrimSpace(normalized.CookieName); normalized.CookieName == "" {
		normalized.CookieName = DefaultCookieName
	}
	if normalized.StateTTL == 0 {
		normalized.StateTTL = int(DefaultStateTTL / time.Second)
	}
	normalized.SuccessURL = safeReturnTo(normalized.SuccessURL)
	normalized.Scopes = append([]string(nil), normalized.Scopes...)

	redirect, _ := url.Parse(normalized.RedirectURL)
	client := &Client{
		config:       normalized,
		httpClient:   opts.httpClient,
		now:          opts.now,
		stateTTL:     time.Duration(normalized.StateTTL) * time.Second,
		cookieSecure: normalized.CookieSecure || redirect.Scheme == "https",
		oauth2Config: xoauth2.Config{
			ClientID:     normalized.ClientID,
			ClientSecret: normalized.ClientSecret,
			RedirectURL:  normalized.RedirectURL,
			Scopes:       append([]string(nil), normalized.Scopes...),
			Endpoint: xoauth2.Endpoint{
				AuthURL:   normalized.AuthURL,
				TokenURL:  normalized.TokenURL,
				AuthStyle: normalized.AuthStyle,
			},
		},
	}
	return client, nil
}

func (a *Client) Config() Config {
	cfg := a.config
	cfg.Scopes = append([]string(nil), a.config.Scopes...)
	return cfg
}

func (a *Client) OAuth2Config() xoauth2.Config {
	cfg := a.oauth2Config
	cfg.Scopes = append([]string(nil), a.oauth2Config.Scopes...)
	return cfg
}

func RandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("oauth2: generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (a *Client) AuthorizationURL(c *gin.Context, request *AuthorizationRequest) (string, error) {
	state, err := RandomToken()
	if err != nil {
		return "", err
	}
	verifier, err := RandomToken()
	if err != nil {
		return "", err
	}
	payload := statePayload{
		Version:   1,
		State:     state,
		Verifier:  verifier,
		ExpiresAt: a.now().Add(a.stateTTL).Unix(),
	}
	var options []xoauth2.AuthCodeOption
	if request != nil {
		payload.ReturnTo = safeReturnTo(request.ReturnTo)
		payload.Nonce = request.Nonce
		options = append(options, request.Options...)
	}
	if payload.ReturnTo == "" {
		payload.ReturnTo = a.config.SuccessURL
	}
	if err := a.writeStateCookie(c, payload); err != nil {
		return "", err
	}
	options = append(options,
		xoauth2.SetAuthURLParam("state", state),
		xoauth2.S256ChallengeOption(verifier),
	)
	return a.oauth2Config.AuthCodeURL(state, options...), nil
}

func (a *Client) LoginHandler(options ...xoauth2.AuthCodeOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		authURL, err := a.AuthorizationURL(c, &AuthorizationRequest{
			ReturnTo: c.Query("return_to"),
			Options:  options,
		})
		if err != nil {
			Abort(c, err)
			return
		}
		c.Redirect(http.StatusFound, authURL)
	}
}

func (a *Client) Exchange(c *gin.Context) (*Result, error) {
	payload, err := a.readStateCookie(c)
	a.clearStateCookie(c)
	if err != nil {
		return nil, err
	}
	state := c.Query("state")
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(payload.State)) != 1 {
		return nil, ErrInvalidState
	}
	if providerCode := strings.TrimSpace(c.Query("error")); providerCode != "" {
		return nil, &ProviderError{
			Code:        providerCode,
			Description: strings.TrimSpace(c.Query("error_description")),
			URI:         strings.TrimSpace(c.Query("error_uri")),
		}
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		return nil, ErrMissingCode
	}
	token, err := a.oauth2Config.Exchange(a.context(c.Request.Context()), code, xoauth2.VerifierOption(payload.Verifier))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExchange, err)
	}
	return &Result{Token: token, ReturnTo: payload.ReturnTo, Nonce: payload.Nonce}, nil
}

func (a *Client) Authenticate(c *gin.Context) (*Identity, error) {
	result, err := a.Exchange(c)
	if err != nil {
		return nil, err
	}
	identity := &Identity{Result: result}
	if a.config.UserInfoURL != "" {
		identity.UserInfo = make(map[string]any)
		if err := a.UserInfo(c.Request.Context(), result.Token, &identity.UserInfo); err != nil {
			return nil, err
		}
	}
	return identity, nil
}

func (a *Client) UserInfo(ctx context.Context, token *xoauth2.Token, dst any) error {
	if a.config.UserInfoURL == "" {
		return errors.New("oauth2: userinfo_url is not configured")
	}
	if token == nil {
		return errors.New("oauth2: token is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.config.UserInfoURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUserInfo, err)
	}
	token.SetAuthHeader(req)
	resp, err := a.http().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUserInfo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxUserInfoSize))
		return fmt.Errorf("%w: unexpected HTTP status %d", ErrUserInfo, resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxUserInfoSize+1))
	if err != nil {
		return fmt.Errorf("%w: read response: %v", ErrUserInfo, err)
	}
	if len(buf) > maxUserInfoSize {
		return fmt.Errorf("%w: response exceeds %d bytes", ErrUserInfo, maxUserInfoSize)
	}
	decoder := json.NewDecoder(strings.NewReader(string(buf)))
	decoder.UseNumber()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrUserInfo, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrUserInfo, err)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func (a *Client) CallbackHandler(next CallbackFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := a.Authenticate(c)
		if err != nil {
			Abort(c, err)
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

func Abort(c *gin.Context, err error) {
	status := http.StatusBadRequest
	code := "invalid_request"
	switch {
	case errors.Is(err, ErrInvalidState):
		code = "invalid_state"
	case errors.Is(err, ErrProvider):
		code = "provider_error"
	case errors.Is(err, ErrMissingCode):
		code = "missing_code"
	case errors.Is(err, ErrExchange):
		status = http.StatusBadGateway
		code = "token_exchange_failed"
	case errors.Is(err, ErrUserInfo):
		status = http.StatusBadGateway
		code = "userinfo_failed"
	default:
		status = http.StatusInternalServerError
		code = "oauth2_failed"
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

func (a *Client) context(ctx context.Context) context.Context {
	if a.httpClient == nil {
		return ctx
	}
	return context.WithValue(ctx, xoauth2.HTTPClient, a.httpClient)
}

func (a *Client) http() *http.Client {
	if a.httpClient != nil {
		return a.httpClient
	}
	return http.DefaultClient
}

func (a *Client) writeStateCookie(c *gin.Context, payload statePayload) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("oauth2: encode state: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(buf)
	mac := hmac.New(sha256.New, []byte(a.config.CookieSecret))
	_, _ = mac.Write([]byte(body))
	value := body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.config.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(a.stateTTL / time.Second),
		Expires:  a.now().Add(a.stateTTL),
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *Client) readStateCookie(c *gin.Context) (statePayload, error) {
	cookie, err := c.Cookie(a.config.CookieName)
	if err != nil || len(cookie) > 4096 {
		return statePayload{}, ErrInvalidState
	}
	body, signature, ok := strings.Cut(cookie, ".")
	if !ok || body == "" || signature == "" || strings.Contains(signature, ".") {
		return statePayload{}, ErrInvalidState
	}
	providedMAC, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return statePayload{}, ErrInvalidState
	}
	mac := hmac.New(sha256.New, []byte(a.config.CookieSecret))
	_, _ = mac.Write([]byte(body))
	if !hmac.Equal(providedMAC, mac.Sum(nil)) {
		return statePayload{}, ErrInvalidState
	}
	buf, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return statePayload{}, ErrInvalidState
	}
	var payload statePayload
	if err := json.Unmarshal(buf, &payload); err != nil {
		return statePayload{}, ErrInvalidState
	}
	if payload.Version != 1 || payload.State == "" || payload.Verifier == "" || payload.ExpiresAt <= a.now().Unix() {
		return statePayload{}, ErrInvalidState
	}
	return payload, nil
}

func (a *Client) clearStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.config.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
