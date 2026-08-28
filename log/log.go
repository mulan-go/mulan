package log

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 包装 zap.Logger，提供清理功能
type Logger struct {
	*zap.Logger
	closers []io.Closer
}

type MultiHandler struct {
	cores   []zapcore.Core
	closers []io.Closer
}

type resolvedConfig struct {
	level        zapcore.Level
	consoleLevel zapcore.Level
	format       string
}

// Close 关闭所有资源
func (l *Logger) Close() error {
	var firstErr error
	for _, closer := range l.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// New 创建日志实例（简化版）
func New(name ...string) (*Logger, error) {
	return NewWithConfig(&Config{Level: "info"}, name...)
}

// NewWithConfig 根据配置创建日志实例
func NewWithConfig(cfg *Config, name ...string) (*Logger, error) {
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}

	handler, err := newMultiHandler(cfg, resolved)
	if err != nil {
		return nil, err
	}

	zapLogger := zap.New(
		zapcore.NewTee(handler.cores...),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	zapLogger = zapLogger.WithOptions(zap.AddCallerSkip(cfg.Skip))
	if len(name) > 0 {
		zapLogger = zapLogger.Named(name[0]).With(zap.String("service", name[0]))
	}
	zap.ReplaceGlobals(zapLogger)
	return &Logger{Logger: zapLogger, closers: handler.closers}, nil
}

func resolveConfig(cfg *Config) (resolvedConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	defaultLevel := zapcore.InfoLevel

	switch mode {
	case "", "server", "prod", "production":
		if format == "" && mode != "" {
			format = "json"
		}
	case "local", "dev", "development", "debug":
		defaultLevel = zapcore.DebugLevel
		if format == "" {
			format = "console"
		}
	default:
		return resolvedConfig{}, fmt.Errorf("invalid log mode %q", cfg.Mode)
	}

	if cfg.JSON && format == "" {
		format = "json"
	}
	if format == "" {
		format = "console"
	}
	if format != "console" && format != "json" {
		return resolvedConfig{}, fmt.Errorf("invalid log format %q", cfg.Format)
	}

	level, err := parseLevelOrDefault(cfg.Level, defaultLevel)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	consoleLevel, err := parseLevelOrDefault(cfg.ConsoleLevel, level)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("invalid console log level %q: %w", cfg.ConsoleLevel, err)
	}

	return resolvedConfig{
		level:        level,
		consoleLevel: consoleLevel,
		format:       format,
	}, nil
}

func parseLevelOrDefault(level string, fallback zapcore.Level) (zapcore.Level, error) {
	if strings.TrimSpace(level) == "" {
		return fallback, nil
	}
	return zapcore.ParseLevel(level)
}

func newMultiHandler(cfg *Config, resolved resolvedConfig) (*MultiHandler, error) {
	adaptorEncoder := zapcore.NewJSONEncoder(jsonEncoderConfig())
	consoleEncoder := newConsoleEncoder(resolved.format)

	handler := &MultiHandler{
		cores: []zapcore.Core{
			zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), resolved.consoleLevel),
		},
	}

	for _, adaptorDSN := range cfg.Adaptors {
		core, closer, err := createAdaptorCore(adaptorDSN, adaptorEncoder, resolved.level)
		if err != nil {
			if closer != nil {
				_ = closer.Close()
			}
			continue
		}
		handler.cores = append(handler.cores, core)
		if closer != nil {
			handler.closers = append(handler.closers, closer)
		}
	}

	return handler, nil
}

func jsonEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.EpochMillisTimeEncoder
	cfg.EncodeDuration = zapcore.StringDurationEncoder
	return cfg
}

func consoleEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeDuration = zapcore.StringDurationEncoder
	cfg.EncodeTime = zapcore.EpochMillisTimeEncoder
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return cfg
}

func newConsoleEncoder(format string) zapcore.Encoder {
	if format == "json" {
		return zapcore.NewJSONEncoder(jsonEncoderConfig())
	}
	return zapcore.NewConsoleEncoder(consoleEncoderConfig())
}

// createAdaptorCore 根据 DSN 创建对应的 Core
func createAdaptorCore(dsn string, encoder zapcore.Encoder, lvl zapcore.Level) (zapcore.Core, io.Closer, error) {
	schema, _, ok := strings.Cut(dsn, "://")
	if !ok {
		return nil, nil, fmt.Errorf("invalid adaptor DSN: %s", dsn)
	}
	switch schema {
	case "file":
		opts, err := parseFileOptions(dsn)
		if err != nil {
			return nil, nil, err
		}
		writer, closer, err := newFileWriter(opts)
		if err != nil {
			return nil, nil, err
		}
		if opts.LevelSet {
			lvl = opts.Level
		}
		return zapcore.NewCore(encoder, writer, lvl), closer, nil
	case "http":
		opts, err := parseHTTPOptions(dsn)
		if err != nil {
			return nil, nil, err
		}
		writer, closer, err := newHTTPWriter(opts)
		if err != nil {
			return nil, nil, err
		}
		if opts.LevelSet {
			lvl = opts.Level
		}
		return zapcore.NewCore(encoder, writer, lvl), closer, nil
	default:
		return nil, nil, fmt.Errorf("unsupported scheme: %s", schema)
	}
}
