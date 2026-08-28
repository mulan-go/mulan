package apikey

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
)

type Config struct {
	Name   string   `json:"name" yaml:"name"`
	Value  string   `json:"value" yaml:"value"`
	Values []string `json:"values" yaml:"values"`
}

func (c *Config) FlagSet() *pflag.FlagSet { return FlagSet() }

func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("apikey", pflag.ContinueOnError)
	fs.String("apikey.name", "apikey", "APIKey Name")
	fs.String("apikey.value", "", "APIKey Value")
	fs.StringSlice("apikey.values", []string{}, "APIKey Value List")
	return fs
}

func Mw(cfg *Config) func(*gin.Context) {
	apikeys := map[string]struct{}{}
	if cfg.Value != "" {
		apikeys[cfg.Value] = struct{}{}
	}
	for _, key := range cfg.Values {
		if key = strings.TrimSpace(key); key != "" {
			apikeys[key] = struct{}{}
		}
	}
	if len(apikeys) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "apikey"
	}
	return func(c *gin.Context) {
		current := strings.TrimSpace(c.GetHeader(name))
		if current == "" {
			current, _ = c.Cookie(name)
			current = strings.TrimSpace(current)
		}
		if current == "" {
			current = strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		}
		for apikey := range apikeys {
			if apikey == current {
				c.Next()
				return
			}
		}
		c.AbortWithStatus(401)
	}
}
