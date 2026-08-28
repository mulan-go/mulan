package web

import (
	"strconv"

	"github.com/spf13/pflag"
)

//go:generate structx -struct Config
type Config struct {
	Prefix   string `json:"prefix" yaml:"prefix" default:"/"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Host     string `json:"host" yaml:"host" default:"127.0.0.1"`
	Port     int    `json:"port" yaml:"port"`
	Debug    bool   `json:"debug" yaml:"debug"`
	Pprof    bool   `json:"pprof" yaml:"pprof"`
	Metrics  bool   `json:"metrics" yaml:"metrics"`
}

func FlagSet(defaultPort int) *pflag.FlagSet {
	fs := pflag.NewFlagSet("web", pflag.ContinueOnError)
	fs.Bool("web.debug", false, "HTTP Debug Mode")
	fs.String("web.prefix", "/", "HTTP API Route Prefix")
	fs.String("web.endpoint", "", "HTTP Domain Endpoint")
	fs.String("web.host", "127.0.0.1", "HTTP Listen Address")
	fs.Int("web.port", defaultPort, "HTTP Listen Port")
	fs.Bool("web.pprof", false, "Enable PProf")
	fs.Bool("web.metrics", false, "Enable metrics")
	return fs
}

func (c *Config) FlagSet(defaultPort int) *pflag.FlagSet {
	return FlagSet(defaultPort)
}

func (c *Config) Addr() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

func (c *Config) GetEndpoint() string {
	if c.Endpoint == "" {
		c.Endpoint = c.Addr()
	}
	return c.Endpoint
}
