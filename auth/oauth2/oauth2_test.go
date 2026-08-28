package oauth2_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	authoauth2 "github.com/mulan-go/mulan/auth/oauth2"
	"github.com/mulan-go/mulan/auth/session"
)

const testCookieSecret = "0123456789abcdef0123456789abcdef"

type oauthProviderRecorder struct {
	tokenCalls atomic.Int32
	mu         sync.Mutex
	tokenForm  url.Values
}

func (r *oauthProviderRecorder) setTokenForm(form url.Values) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokenForm = form
}

func (r *oauthProviderRecorder) verifier() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokenForm.Get("code_verifier")
}

func newOAuthProvider(t *testing.T) (*httptest.Server, *oauthProviderRecorder) {
	t.Helper()
	recorder := &oauthProviderRecorder{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, req *http.Request) {
		recorder.tokenCalls.Add(1)
		_ = req.ParseForm()
		recorder.setTokenForm(req.Form)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer access-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"42","login":"alice","roles":["admin","developer"]}`))
	})
	return httptest.NewServer(mux), recorder
}

func newOAuthRouter(t *testing.T, providerURL string) (*gin.Engine, *authoauth2.Client) {
	t.Helper()
	client, err := authoauth2.New(&authoauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost/oauth2/callback",
		AuthURL:      providerURL + "/authorize",
		TokenURL:     providerURL + "/token",
		UserInfoURL:  providerURL + "/userinfo",
		Scopes:       []string{"profile"},
		CookieSecret: testCookieSecret,
		SuccessURL:   "/default",
	})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(session.Mw("token", session.NewMemStore()))
	router.GET("/oauth2/login", client.LoginHandler())
	router.GET("/oauth2/callback", client.SessionCallback())
	router.GET("/seed", func(c *gin.Context) {
		sess := session.Default(c)
		sess.SetAccount("old-account")
		sess.SetRoles([]string{"admin"})
		if err := sess.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/me", session.AuthMW(), session.RoleMW("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"account": c.GetString(session.CtxKeyAccount),
			"subject": c.GetString("oauth2_subject"),
		})
	})
	return router, client
}

func TestAuthorizationCodeFlowCreatesSession(t *testing.T) {
	provider, recorder := newOAuthProvider(t)
	defer provider.Close()
	router, _ := newOAuthRouter(t, provider.URL)

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodGet, "/oauth2/login?return_to=%2Fdashboard%3Ftab%3Dprofile", nil)
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusFound {
		t.Fatalf("login status: got %d want %d", login.Code, http.StatusFound)
	}
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := authURL.Query()
	if query.Get("state") == "" {
		t.Fatal("authorization URL has no state")
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatalf("authorization URL has invalid PKCE parameters: %s", authURL.RawQuery)
	}
	stateCookie := responseCookie(t, login, authoauth2.DefaultCookieName)

	callback := httptest.NewRecorder()
	callbackRequest := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=valid-code&state="+url.QueryEscape(query.Get("state")), nil)
	callbackRequest.AddCookie(stateCookie)
	router.ServeHTTP(callback, callbackRequest)
	if callback.Code != http.StatusFound {
		t.Fatalf("callback status: got %d want %d body=%s", callback.Code, http.StatusFound, callback.Body.String())
	}
	if callback.Header().Get("Location") != "/dashboard?tab=profile" {
		t.Fatalf("callback redirect: %q", callback.Header().Get("Location"))
	}
	verifier := recorder.verifier()
	challenge := sha256.Sum256([]byte(verifier))
	if got, want := query.Get("code_challenge"), base64.RawURLEncoding.EncodeToString(challenge[:]); got != want {
		t.Fatalf("PKCE verifier mismatch: got challenge %q want %q", got, want)
	}
	if recorder.tokenCalls.Load() != 1 {
		t.Fatalf("token endpoint calls: got %d want 1", recorder.tokenCalls.Load())
	}

	token := callback.Header().Get("X-Token")
	if token == "" {
		t.Fatal("callback did not create a session token")
	}
	me := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(me, meRequest)
	if me.Code != http.StatusOK {
		t.Fatalf("authenticated request status: got %d want %d body=%s", me.Code, http.StatusOK, me.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["account"] != "alice" || body["subject"] != "42" {
		t.Fatalf("unexpected session identity: %#v", body)
	}
}

func TestCallbackRejectsStateMismatchBeforeTokenExchange(t *testing.T) {
	provider, recorder := newOAuthProvider(t)
	defer provider.Close()
	router, _ := newOAuthRouter(t, provider.URL)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oauth2/login", nil))
	authURL, _ := url.Parse(login.Header().Get("Location"))
	cookie := responseCookie(t, login, authoauth2.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")+"tampered"), nil)
	request.AddCookie(cookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusBadRequest || !strings.Contains(callback.Body.String(), "invalid_state") {
		t.Fatalf("unexpected callback response: status=%d body=%s", callback.Code, callback.Body.String())
	}
	if recorder.tokenCalls.Load() != 0 {
		t.Fatalf("token endpoint called after state mismatch: %d", recorder.tokenCalls.Load())
	}
}

func TestCallbackRotatesExistingSession(t *testing.T) {
	provider, _ := newOAuthProvider(t)
	defer provider.Close()
	router, _ := newOAuthRouter(t, provider.URL)

	seed := httptest.NewRecorder()
	router.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed", nil))
	oldToken := seed.Header().Get("X-Token")
	if oldToken == "" {
		t.Fatal("seed request did not create a session")
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodGet, "/oauth2/login", nil)
	loginRequest.AddCookie(&http.Cookie{Name: "token", Value: oldToken})
	router.ServeHTTP(login, loginRequest)
	authURL, _ := url.Parse(login.Header().Get("Location"))
	stateCookie := responseCookie(t, login, authoauth2.DefaultCookieName)

	callback := httptest.NewRecorder()
	callbackRequest := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	callbackRequest.AddCookie(&http.Cookie{Name: "token", Value: oldToken})
	callbackRequest.AddCookie(stateCookie)
	router.ServeHTTP(callback, callbackRequest)
	newToken := callback.Header().Get("X-Token")
	if callback.Code != http.StatusFound || newToken == "" || newToken == oldToken {
		t.Fatalf("session was not rotated: status=%d old=%q new=%q body=%s", callback.Code, oldToken, newToken, callback.Body.String())
	}

	oldSession := httptest.NewRecorder()
	oldRequest := httptest.NewRequest(http.MethodGet, "/me", nil)
	oldRequest.Header.Set("X-Token", oldToken)
	router.ServeHTTP(oldSession, oldRequest)
	if oldSession.Code != http.StatusUnauthorized {
		t.Fatalf("old session remains valid: status=%d", oldSession.Code)
	}
}

func TestLoginRejectsExternalReturnURL(t *testing.T) {
	provider, _ := newOAuthProvider(t)
	defer provider.Close()
	router, _ := newOAuthRouter(t, provider.URL)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oauth2/login?return_to=https%3A%2F%2Fevil.example", nil))
	authURL, _ := url.Parse(login.Header().Get("Location"))
	cookie := responseCookie(t, login, authoauth2.DefaultCookieName)

	callback := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	request.AddCookie(cookie)
	router.ServeHTTP(callback, request)
	if callback.Code != http.StatusFound || callback.Header().Get("Location") != "/default" {
		t.Fatalf("unsafe return URL was not replaced: status=%d location=%q", callback.Code, callback.Header().Get("Location"))
	}
}

func TestConfigRejectsWeakCookieSecret(t *testing.T) {
	_, err := authoauth2.New(&authoauth2.Config{
		ClientID:     "client",
		RedirectURL:  "http://localhost/callback",
		AuthURL:      "https://provider.example/authorize",
		TokenURL:     "https://provider.example/token",
		CookieSecret: "weak",
	})
	if err == nil || !strings.Contains(err.Error(), "at least 32 bytes") {
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
