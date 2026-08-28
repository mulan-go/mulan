package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/app/service"
)

// 默认超时配置
const (
	defaultReadTimeout    = 30 * time.Second
	defaultWriteTimeout   = 30 * time.Second
	defaultIdleTimeout    = 120 * time.Second
	defaultReadHeaderTime = 10 * time.Second
	defaultMaxHeaderBytes = 1 << 20 // 1MB
)

var _ service.Servicer = (*Service)(nil)

type Info struct {
	Name    string
	Version string
	Commit  string
	BuildAt string
}

type Service struct {
	conf             *Config
	info             *Info
	routerFn         func(gin.IRouter)
	versionHandler   gin.HandlerFunc
	healthzHandler   gin.HandlerFunc
	readinessHandler gin.HandlerFunc
	engine           *gin.Engine
	server           *http.Server
	isBuild          bool
}

func (s *Service) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Service) Close() error {
	err := s.server.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Service) Server() *http.Server    { return s.server }
func (s *Service) Routes() []gin.RouteInfo { return s.engine.Routes() }
func (s *Service) Engine() *gin.Engine     { return s.engine }

func (s *Service) Serve() error {
	if !s.isBuild {
		s.Build()
	}
	zap.L().Info("HTTP Server Listening on",
		zap.String("host", s.conf.Host),
		zap.Int("port", s.conf.Port),
	)
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Service) SetVersionHandler(h gin.HandlerFunc) {
	s.versionHandler = h
}

func (s *Service) SetReadinessHandler(h gin.HandlerFunc) {
	s.readinessHandler = h
}

func (s *Service) SetHealthzHandler(h gin.HandlerFunc) {
	s.healthzHandler = h
}

func New(conf *Config, info *Info, fn func(gin.IRouter)) *Service {
	return &Service{conf: conf, info: info, routerFn: fn}
}

func (s *Service) Build() *Service {
	if s.conf.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	ginLog := zap.L().Named("gin")

	s.engine = gin.New()

	s.engine.Use(
		// Recovery
		ginzap.RecoveryWithZap(ginLog, true),
		// RequestID
		requestid.New(),
		// Logger
		defaultLogger(ginLog),
	)

	if s.versionHandler == nil {
		s.versionHandler = VersionHandler(s.info)
	}
	if s.healthzHandler == nil {
		s.healthzHandler = DefaultHealthHandler
	}
	if s.readinessHandler == nil {
		s.readinessHandler = s.healthzHandler
	}

	if s.conf.Pprof {
		if isLoopbackHost(s.conf.Host) {
			pprof.RouteRegister(s.engine, "/pprof")
		} else {
			zap.L().Warn("pprof disabled on non-loopback listener")
		}
	}

	if s.conf.Metrics {
		m := ginmetrics.GetMonitor()
		m.SetMetricPath("/metrics")
		m.SetExcludePaths(loggerSkipPaths)
		m.SetSlowTime(10)
		m.Use(s.engine)
	}

	// Register Router
	if s.routerFn != nil {
		api := s.engine.Group(s.conf.Prefix)
		s.routerFn(api)
	}

	s.registerDefaultGET("/version", s.versionHandler)
	s.registerDefaultGET("/health", s.healthzHandler)
	s.registerDefaultGET("/healthz", s.healthzHandler)
	s.registerDefaultGET("/ready", s.readinessHandler)

	// 配置 HTTP Server 超时防止慢速连接攻击
	s.server = &http.Server{
		Addr:              s.conf.Addr(),
		Handler:           s.engine,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		ReadHeaderTimeout: defaultReadHeaderTime,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
	s.isBuild = true
	return s
}

func (s *Service) registerDefaultGET(path string, handler gin.HandlerFunc) {
	for _, route := range s.engine.Routes() {
		if route.Method == http.MethodGet && route.Path == path {
			return
		}
	}
	s.engine.GET(path, handler)
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
