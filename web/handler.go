package web

import (
	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/rsp"
)

type Handler interface {
	Register(gin.IRouter)
}

func ErrCodeHandler(c *gin.Context) {
	c.JSON(200, rsp.S(rsp.Errors))
}

func VersionHandler(info *Info) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":     info.Name,
			"version":  info.Version,
			"commit":   info.Commit,
			"built_at": info.BuildAt,
		})
	}
}

func DefaultHealthHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "code": 0})
}
