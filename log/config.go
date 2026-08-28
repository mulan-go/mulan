package log

import "github.com/spf13/pflag"

type Config struct {
	Level        string   `json:"level" yaml:"level"`                // 默认日志级别: debug, info, warn, error
	ConsoleLevel string   `json:"console_level" yaml:"consoleLevel"` // 控制台日志级别，默认继承 Level
	Mode         string   `json:"mode" yaml:"mode"`                  // 运行模式: local, server
	Format       string   `json:"format" yaml:"format"`              // 控制台格式: console, json
	Adaptors     []string `json:"adaptors" yaml:"adaptors"`          // 输出适配器 DSN 列表
	Skip         int      `json:"skip" yaml:"skip"`                  // 跳过调用栈层数
	JSON         bool     `json:"json" yaml:"json"`                  // 是否输出 JSON 格式
}

func (c *Config) FlagSet() *pflag.FlagSet { return FlagSet() }

func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("log", pflag.ContinueOnError)
	fs.String("log.level", "info", "log level: debug, info, warn, error")
	fs.String("log.console-level", "", "console log level: debug, info, warn, error")
	fs.String("log.mode", "", "log mode: local, server")
	fs.String("log.format", "", "console log format: console, json")
	fs.StringSlice("log.adaptors", []string{}, "log adaptors DSN (e.g., file:///var/log/app.log?max-size=100m&max-age=30d)")
	fs.Int("log.skip", 1, "log skip caller stack frames")
	fs.Bool("log.json", false, "log output JSON format")
	return fs
}
