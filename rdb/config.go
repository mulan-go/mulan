package rdb

import (
	"strconv"

	"github.com/spf13/pflag"
)

func FlagSet(name string) *pflag.FlagSet {
	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	fs.Bool(name+".debug", false, "Database Debug Mode")
	fs.String(name+".host", "127.0.0.1", "Database Host")
	fs.Int(name+".port", 6379, "Database Port")
	fs.Int(name+".db", 0, "Database Index")
	fs.String(name+".pass", "", "Database Password")
	return fs
}

type Config struct {
	Host  string `json:"host" yaml:"host"`
	Pass  string `json:"pass" yaml:"pass"`
	Port  int    `json:"port" yaml:"port"`
	DB    int    `json:"db" yaml:"db"`
	Debug bool   `json:"debug" yaml:"debug"`
}

func (c *Config) FlagSet(name string) *pflag.FlagSet { return FlagSet(name) }

func (c *Config) Addr() string { return c.Host + ":" + strconv.Itoa(c.Port) }

func (c *Config) R() *Config { return c }
