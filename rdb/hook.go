package rdb

import (
	"context"
	"net"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 敏感命令列表，这些命令的参数可能包含敏感数据
var sensitiveCommands = map[string]struct{}{
	"SET":    {},
	"SETEX":  {},
	"SETNX":  {},
	"HSET":   {},
	"HMSET":  {},
	"AUTH":   {},
	"CONFIG": {},
}

type DebugHook struct{}

// 当创建网络连接时调用
func (DebugHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// maskSensitiveCmd 隐藏敏感命令的参数值
func maskSensitiveCmd(cmd redis.Cmder) string {
	args := cmd.Args()
	if len(args) == 0 {
		return cmd.String()
	}
	cmdName := strings.ToUpper(args[0].(string))
	if _, ok := sensitiveCommands[cmdName]; !ok {
		return cmd.String()
	}
	// 对敏感命令只显示命令名和key，隐藏value
	var sb strings.Builder
	sb.WriteString(cmdName)
	if len(args) > 1 {
		sb.WriteString(" ")
		sb.WriteString(args[1].(string))
	}
	if len(args) > 2 {
		sb.WriteString(" [REDACTED]")
	}
	return sb.String()
}

// 执行命令时调用
func (DebugHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		zap.L().Debug("redis", zap.String("cmd", maskSensitiveCmd(cmd)))
		return next(ctx, cmd)
	}
}

// 执行管道命令时调用
func (DebugHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}
