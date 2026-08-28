package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/mulan-go/mulan/web"
)

func TestNew(t *testing.T) {
	logger, err := zap.NewDevelopment(zap.AddCaller())
	if err != nil {
		t.Fatal(err)
	}
	undo := zap.ReplaceGlobals(logger)
	defer undo()

	cfg := &web.Config{
		Host:  "127.0.0.1",
		Port:  3003,
		Pprof: false,
	}
	webInfo := &web.Info{
		Name:    "test",
		Version: "1.0.0",
		Commit:  "dev",
		BuildAt: "2026-07-30T00:00:00Z",
	}
	webSrv := web.New(cfg, webInfo, func(api gin.IRouter) {
		api.Handle("GET", "/aaa", func(c *gin.Context) {
			zap.L().Info("Hello, World!")
			c.String(200, "Hello, World!")
		})
	})
	response := httptest.NewRecorder()
	webSrv.Build().Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/aaa", nil))
	if response.Code != http.StatusOK || response.Body.String() != "Hello, World!" {
		t.Fatalf("response = (%d, %q)", response.Code, response.Body.String())
	}
}

func TestMetricsCanBeExposedOnPublicListener(t *testing.T) {
	service := web.New(&web.Config{
		Host: "0.0.0.0", Port: 8080, Pprof: true, Metrics: true,
	}, &web.Info{Name: "test"}, nil).Build()
	metricsExposed := false
	for _, route := range service.Routes() {
		if strings.HasPrefix(route.Path, "/pprof") {
			t.Fatalf("pprof route exposed on public listener: %s", route.Path)
		}
		if route.Path == "/metrics" {
			metricsExposed = true
		}
	}
	if !metricsExposed {
		t.Fatal("metrics route is not exposed on public listener")
	}
}

func TestServeReturnsListenError(t *testing.T) {
	service := web.New(&web.Config{
		Host: "invalid host", Port: 8080,
	}, &web.Info{Name: "test"}, nil)
	if err := service.Serve(); err == nil {
		t.Fatal("Serve() error = nil")
	}
}

func TestVersionHandlerIsServiceScoped(t *testing.T) {
	first := web.New(&web.Config{}, &web.Info{Name: "first"}, nil).Build()
	second := web.New(&web.Config{}, &web.Info{Name: "second"}, nil).Build()

	for _, test := range []struct {
		service *web.Service
		want    string
	}{
		{service: first, want: "first"},
		{service: second, want: "second"},
	} {
		response := httptest.NewRecorder()
		test.service.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/version", nil))
		var body struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Name != test.want {
			t.Fatalf("name = %q, want %q", body.Name, test.want)
		}
	}
}

func TestHandlerOverridesAreServiceScoped(t *testing.T) {
	first := web.New(&web.Config{}, &web.Info{Name: "first"}, nil)
	first.SetVersionHandler(func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})
	first.SetHealthzHandler(func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})
	first.SetReadinessHandler(func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	second := web.New(&web.Config{}, &web.Info{Name: "second"}, nil)
	first.Build()
	second.Build()

	for path, want := range map[string]int{
		"/version": http.StatusCreated,
		"/healthz": http.StatusAccepted,
		"/ready":   http.StatusNoContent,
	} {
		response := httptest.NewRecorder()
		first.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Fatalf("first %s status = %d, want %d", path, response.Code, want)
		}
	}

	for _, path := range []string{"/version", "/healthz", "/ready"} {
		response := httptest.NewRecorder()
		second.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("second %s status = %d", path, response.Code)
		}
	}
}

func TestApplicationProbeRoutesTakePrecedence(t *testing.T) {
	service := web.New(&web.Config{Prefix: "/"}, &web.Info{Name: "test"}, func(router gin.IRouter) {
		router.GET("/ready", func(c *gin.Context) { c.String(http.StatusOK, "application-ready") })
		router.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "application-healthz") })
	}).Build()

	for path, want := range map[string]string{
		"/ready":   "application-ready",
		"/healthz": "application-healthz",
	} {
		response := httptest.NewRecorder()
		service.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Body.String() != want {
			t.Fatalf("%s body = %q, want %q", path, response.Body.String(), want)
		}
	}
}

func TestDefaultLoggerSkipsDiagnosticAndNotFoundRequests(t *testing.T) {
	core, observed := observer.New(zap.InfoLevel)
	undo := zap.ReplaceGlobals(zap.New(core))
	defer undo()

	service := web.New(&web.Config{Prefix: "/"}, &web.Info{Name: "test"}, func(router gin.IRouter) {
		router.GET("/business", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	}).Build()
	for _, path := range []string{"/healthz", "/ready", "/missing"} {
		response := httptest.NewRecorder()
		service.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	}
	if observed.Len() != 0 {
		t.Fatalf("diagnostic/not-found log count = %d", observed.Len())
	}

	response := httptest.NewRecorder()
	service.Engine().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/business", nil))
	if observed.Len() != 1 {
		t.Fatalf("business log count = %d", observed.Len())
	}
}
