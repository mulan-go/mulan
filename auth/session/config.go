package session

import (
	"github.com/spf13/pflag"

	"github.com/mulan-go/mulan/rdb"
)

type Config struct {
	Name       string     `json:"name" yaml:"name"`
	TTL        int        `json:"ttl" yaml:"ttl"`
	Driver     string     `json:"driver" yaml:"driver"`
	RDB        rdb.Config `json:"rdb" yaml:"rdb"`
	Dir        string     `json:"dir" yaml:"dir"`
	HeaderOnly bool       `json:"header_only" yaml:"header_only"`
}

func (c *Config) FlagSet() *pflag.FlagSet { return FlagSet() }

func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("session", pflag.ContinueOnError)
	fs.String("session.name", "token", "Session Token Name")
	fs.Int("session.ttl", 0, "session ttl")
	fs.String("session.driver", "memory", "session driver")
	fs.Bool("session.header-only", true, "accept and return session tokens through headers only")
	// driver redis
	fs.String("session.rdb.host", "127.0.0.1", "session rdb host")
	fs.String("session.rdb.pass", "", "session rdb pass")
	fs.Int("session.rdb.port", 6379, "session rdb port")
	fs.Int("session.rdb.db", 0, "session rdb db")
	fs.Bool("session.rdb.debug", false, "session rdb debug")
	// driver fs
	fs.String("session.dir", "", "session fs dir")
	return fs
}
