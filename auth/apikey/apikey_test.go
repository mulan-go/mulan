package apikey_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/auth/apikey"
)

func newRouter(cfg *apikey.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apikey.Mw(cfg))
	r.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func performRequest(r http.Handler, method, path string, headers map[string]string, cookies map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMw_AllowsRequestWhenConfigIsEmpty(t *testing.T) {
	r := newRouter(&apikey.Config{})

	w := performRequest(r, http.MethodGet, "/protected", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_AllowsRequestWhenOnlyEmptyValuesConfigured(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:   "X-API-Key",
		Value:  "",
		Values: []string{"", "   "},
	})

	w := performRequest(r, http.MethodGet, "/protected", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_RejectsRequestWhenKeyMissing(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:  "X-API-Key",
		Value: "secret-key",
	})

	w := performRequest(r, http.MethodGet, "/protected", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMw_AllowsRequestWithCustomHeader(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:  "X-API-Key",
		Value: "secret-key",
	})

	w := performRequest(r, http.MethodGet, "/protected", map[string]string{
		"X-API-Key": "secret-key",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_AllowsRequestWithCookie(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:  "api_token",
		Value: "secret-key",
	})

	w := performRequest(r, http.MethodGet, "/protected", nil, map[string]string{
		"api_token": "secret-key",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_AllowsRequestWithAuthorizationBearer(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:  "X-API-Key",
		Value: "secret-key",
	})

	w := performRequest(r, http.MethodGet, "/protected", map[string]string{
		"Authorization": "Bearer secret-key",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_AllowsRequestWithValuesList(t *testing.T) {
	name := "X-API-Key"
	r := newRouter(&apikey.Config{
		Name:   name,
		Values: []string{"secret-a", "secret-b"},
	})

	w := performRequest(r, http.MethodGet, "/protected", map[string]string{
		name: "secret-b",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMw_RejectsRequestWithWrongKey(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:   "X-API-Key",
		Values: []string{"secret-a", "secret-b"},
	})

	w := performRequest(r, http.MethodGet, "/protected", map[string]string{
		"X-API-Key": "secret-c",
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMw_TrimSpacesFromHeaderAndCookie(t *testing.T) {
	r := newRouter(&apikey.Config{
		Name:  "X-API-Key",
		Value: "secret-key",
	})

	w := performRequest(r, http.MethodGet, "/protected", map[string]string{
		"X-API-Key": "  secret-key  ",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected header auth status %d, got %d", http.StatusOK, w.Code)
	}

	w = performRequest(r, http.MethodGet, "/protected", nil, map[string]string{
		"X-API-Key": "  secret-key  ",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected cookie auth status %d, got %d", http.StatusOK, w.Code)
	}
}
