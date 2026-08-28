package mulan_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/app"
	"github.com/mulan-go/mulan/app/service"
)

type (
	WebConfig struct {
		Port int `json:"port" yaml:"port"`
	}

	WebService struct {
		engine *gin.Engine
		server *http.Server
		cfg    *WebConfig
	}
)

var _ service.Servicer = (*WebService)(nil)

func (w *WebService) Serve() error {
	go w.server.ListenAndServe()
	return nil
}

func (w *WebService) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return w.server.Shutdown(shutdownCtx)
}

func (w *WebService) Close() error {
	return w.server.Close()
}

func NewWebService(cfg *WebConfig, route func(gin.IRouter)) *WebService {
	engine := gin.Default()
	route(engine)
	return &WebService{
		engine: engine,
		server: &http.Server{
			Handler: engine,
			Addr:    fmt.Sprintf(":%d", cfg.Port),
		},
		cfg: cfg,
	}
}

type Config struct {
	Web WebConfig `json:"web" yaml:"web"`
}

var (
	Version string = "1.0.0"
	Commit  string = "dev"
	BuildAt string = time.Now().Format(time.RFC3339)

	Conf = &Config{}
)

func Example() {
	meta := &mulan.Meta{
		ID:          "com.virzz.mulan.example",
		Name:        "example",
		Description: "ExampleService",
		Version:     Version,
		Commit:      Commit,
		BuildAt:     BuildAt,
	}
	std := mulan.New(meta, Conf)

	routerFunc := func(api gin.IRouter) {
		api.Handle("GET", "/", func(c *gin.Context) {
			c.String(200, "Hello, World!")
		})
	}
	webSrv := NewWebService(&Conf.Web, routerFunc)

	std.AddService(webSrv)

	if err := std.Execute(context.Background()); err != nil {
		panic(err)
	}
}
