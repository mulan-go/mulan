package web

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	loggerSkipPaths = []string{
		"/health",
		"/healthz",
		"/ready",
		"/version",
		"/metrics",
		"/pprof",
	}
	loggerSkipMethods = []string{
		http.MethodOptions,
		http.MethodHead,
		http.MethodTrace,
	}
)

func RequestID(c *gin.Context) string {
	return requestid.Get(c)
}

func ZapRequestID(c *gin.Context) zap.Field {
	return zap.String("requestid", requestid.Get(c))
}

func defaultLogger(gl ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.GinzapWithConfig(gl, &ginzap.Config{
		TimeFormat: time.RFC3339,
		Skipper: func(c *gin.Context) bool {
			if slices.Contains(loggerSkipMethods, c.Request.Method) {
				return true
			}
			if c.Writer.Status() == http.StatusNotFound {
				return true
			}
			for _, path := range loggerSkipPaths {
				if strings.HasSuffix(c.Request.URL.Path, path) {
					return true
				}
			}
			return false
		},
		Context: func(c *gin.Context) []zap.Field {
			fields := []zapcore.Field{
				zap.String("referer", c.Request.Referer()),
				ZapRequestID(c),
			}
			return fields
		},
	})
}
