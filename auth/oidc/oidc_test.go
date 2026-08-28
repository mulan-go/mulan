package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	authoidc "github.com/mulan-go/mulan/auth/oidc"
	"github.com/mulan-go/mulan/auth/session"
)

const oidcCookieSecret = "abcdef0123456789abcdef0123456789"

type fakeOIDCProvider struct {
	server  *httptest.Server
	key     *rsa.PrivateKey
	mu      sync.Mutex
	nonce   string
	form    url.Values
	subject string
	expired bool
}

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	provider := &fakeOIDCProvider{key: key, subject: "oidc-user"}
	provider.server = httptest.NewServer(http.HandlerFunc(provider.serveHTTP))
	return provider
}

func (p *fakeOIDCProvider) close() { p.server.Close() }

func (p *fakeOIDCProvider) setNonce(nonce string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce = nonce
}

func (p *fakeOIDCProvider) setSubject(subject string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subject = subject
}

func (p *fakeOIDCProvider) setExpired(expired bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expired = expired
}

func (p *fakeOIDCProvider) verifier() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.form.Get("code_verifier")
}

func (p *fakeOIDCProvider) snapshot() (string, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.nonce, p.subject, p.expired
}

func (p *fakeOIDCProvider) serveHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/.well-known/openid-configuration":
		writeJSON(w, map[string]any{
			"issuer":                                p.server.URL,
			"authorization_endpoint":                p.server.URL + "/authorize",
			"token_endpoint":                        p.server.URL + "/token",
			"jwks_uri":                              p.server.URL + "/keys",
			"userinfo_endpoint":                     p.server.URL + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case "/keys":
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &p.key.PublicKey,
			KeyID:     "test-key",
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}}})
	case "/token":
		_ = req.ParseForm()
		p.mu.Lock()
		p.form = req.Form
		p.mu.Unlock()
		nonce, _, expired := p.snapshot()
		rawIDToken, err := p.signIDToken(nonce, expired)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"access_token": "oidc-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     rawIDToken,
		})
	case "/userinfo":
		if req.Header.Get("Authorization") != "Bearer oidc-access-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, subject, _ := p.snapshot()
		writeJSON(w, map[string]any{"sub": subject, "name": "Bob Example"})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (p *fakeOIDCProvider) signIDToken(nonce string, expired bool) (string, error) {
	now := time.Now()
	expiresAt := now.Add(time.Hour)
	if expired {
		expiresAt = now.Add(-time.Minute)
	}
	payload, err := json.Marshal(map[string]any{
		"iss":                p.server.URL,
		"sub":                "oidc-user",
		"aud":                "oidc-client",
		"exp":                expiresAt.Unix(),
		"iat":                now.Unix(),
		"nonce":              nonce,
		"preferred_username": "bob",
		"email":              "bob@example.com",
		"roles":              []string{"admin", "operator"},
	})
	if err != nil {
		return "", err
	}
	privateJWK := &jose.JSONWebKey{
		Key:       p.key,
		KeyID:     "test-key",
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateJWK}, nil)
	if err != nil {
		return "", err
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return signed.CompactSerialize()
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func newOIDCRouter(t *testing.T, provider *fakeOIDCProvider) (*gin.Engine, *authoidc.Authenticator) {
	t.Helper()
	authenticator, err := authoidc.New(t.Context(), &authoidc.Config{
		IssuerURL:     provider.server.URL,
		ClientID:      "oidc-client",
		ClientSecret:  "oidc-secret",
		RedirectURL:   "http://localhost/oidc/callback",
		CookieSecret:  oidcCookieSecret,
		SuccessURL:    "/console",
		FetchUserInfo: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(session.Mw("token", session.NewMemStore()))
	router.GET("/oidc/login", authenticator.LoginHandler())
	router.GET("/oidc/callback", authenticator.SessionCallback())
	router.GET("/me", session.AuthMW(), session.RoleMW("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"account": c.GetString(session.CtxKeyAccount),
			"subject": c.GetString("oidc_subject"),
		})
	})
	return router, authenticator
}

func TestOIDCAuthorizationCodeFlowVerifiesIDTokenAndCreatesSession(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	defer provider.close()
	router, _ := newOIDCRouter(t, provider)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oidc/login?return_to=%2Fsettings", nil))
	if login.Code != http.StatusFound {
		t.Fatalf("login status: got %d want %d body=%s", login.Code, http.StatusFound, login.Body.String())
	}
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := authURL.Query()
	if query.Get("nonce") == "" || query.Get("state") == "" {
		t.Fatalf("authorization URL is missing state or nonce: %s", authURL.RawQuery)
	}
	provider.setNonce(query.Get("nonce"))
	stateCookie := responseCookie(t, login, authoidc.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oidc/callback?code=valid-code&state="+url.QueryEscape(query.Get("state")), nil)
	request.AddCookie(stateCookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusFound || callback.Header().Get("Location") != "/settings" {
		t.Fatalf("callback failed: status=%d location=%q body=%s", callback.Code, callback.Header().Get("Location"), callback.Body.String())
	}
	verifier := provider.verifier()
	challenge := sha256.Sum256([]byte(verifier))
	if got, want := query.Get("code_challenge"), base64.RawURLEncoding.EncodeToString(challenge[:]); got != want {
		t.Fatalf("PKCE verifier mismatch: got %q want %q", got, want)
	}

	token := callback.Header().Get("X-Token")
	if token == "" {
		t.Fatal("callback did not create a session")
	}
	me := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/me", nil)
	meRequest.Header.Set("X-Token", token)
	router.ServeHTTP(me, meRequest)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"account":"bob"`) || !strings.Contains(me.Body.String(), `"subject":"oidc-user"`) {
		t.Fatalf("unexpected authenticated response: status=%d body=%s", me.Code, me.Body.String())
	}
}

func TestOIDCCallbackRejectsNonceMismatch(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	defer provider.close()
	router, _ := newOIDCRouter(t, provider)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oidc/login", nil))
	authURL, _ := url.Parse(login.Header().Get("Location"))
	provider.setNonce("different-nonce")
	cookie := responseCookie(t, login, authoidc.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oidc/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	request.AddCookie(cookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusBadRequest || !strings.Contains(callback.Body.String(), "invalid_nonce") {
		t.Fatalf("nonce mismatch was accepted: status=%d body=%s", callback.Code, callback.Body.String())
	}
}

func TestOIDCCallbackRejectsExpiredIDToken(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	defer provider.close()
	router, _ := newOIDCRouter(t, provider)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oidc/login", nil))
	authURL, _ := url.Parse(login.Header().Get("Location"))
	provider.setNonce(authURL.Query().Get("nonce"))
	provider.setExpired(true)
	cookie := responseCookie(t, login, authoidc.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oidc/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	request.AddCookie(cookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusBadRequest || !strings.Contains(callback.Body.String(), "invalid_id_token") {
		t.Fatalf("expired ID token was accepted: status=%d body=%s", callback.Code, callback.Body.String())
	}
}

func TestOIDCCallbackRejectsUserInfoSubjectMismatch(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	defer provider.close()
	router, _ := newOIDCRouter(t, provider)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oidc/login", nil))
	authURL, _ := url.Parse(login.Header().Get("Location"))
	provider.setNonce(authURL.Query().Get("nonce"))
	provider.setSubject("another-user")
	cookie := responseCookie(t, login, authoidc.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oidc/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	request.AddCookie(cookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusBadGateway || !strings.Contains(callback.Body.String(), "userinfo_failed") {
		t.Fatalf("UserInfo subject mismatch was accepted: status=%d body=%s", callback.Code, callback.Body.String())
	}
}

func TestConfigRequiresOpenIDScope(t *testing.T) {
	err := (&authoidc.Config{
		IssuerURL:    "https://issuer.example",
		ClientID:     "client",
		RedirectURL:  "https://app.example/callback",
		Scopes:       []string{"profile"},
		CookieSecret: oidcCookieSecret,
	}).Validate()
	if err == nil || !strings.Contains(err.Error(), "must contain openid") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func responseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("response cookie %q not found", name)
	return nil
}
