package log

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

var (
	reDays = regexp.MustCompile(`^(\d+)(d|day|days)?$`)
	reSize = regexp.MustCompile(`^(\d+)(m|mb|g|gb)?$`)
)

// FileOptions 文件适配器选项
type FileOptions struct {
	Path       string
	Compress   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Level      zapcore.Level
	LevelSet   bool
}

// HTTPOptions HTTP 适配器选项
type HTTPOptions struct {
	URL        string        // HTTP URL
	Timeout    time.Duration // 超时时间
	BufferSize int           // 缓冲区大小
	BatchSize  int           // 批量发送大小
	MaxRetries int           // 最大重试次数
	Level      zapcore.Level
	LevelSet   bool
}

// parseFileOptions 解析文件适配器 DSN
// 格式: file:///path/to/file.log?max-size=100m&max-backups=10&max-age=30d&compress=gzip
func parseFileOptions(dsn string) (*FileOptions, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid file DSN: %w", err)
	}
	if u.Scheme != "file" {
		return nil, fmt.Errorf("invalid scheme for file: %s", u.Scheme)
	}
	opts := &FileOptions{
		Path:       u.Path,
		MaxSize:    100,               // 默认 100MB
		MaxBackups: 10,                // 默认保留 10 个
		MaxAge:     30,                // 默认保留 30 天
		Compress:   "none",            // 默认不压缩
		Level:      zapcore.InfoLevel, // 默认 info 级别
	}
	query := u.Query()
	// 解析 max-size
	if v := query.Get("max-size"); v != "" {
		size, err := parseSizeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid max-size: %w", err)
		}
		opts.MaxSize = size
	}
	// 解析 max-backups
	if v := query.Get("max-backups"); v != "" {
		backups, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid max-backups: %w", err)
		}
		opts.MaxBackups = backups
	}
	// 解析 max-age
	if v := query.Get("max-age"); v != "" {
		age, err := parseDurationString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid max-age: %w", err)
		}
		opts.MaxAge = age
	}
	// 解析 compress
	if v := query.Get("compress"); v != "" {
		if v != "gzip" && v != "none" {
			return nil, fmt.Errorf("invalid compress format: %s (supported: gzip, none)", v)
		}
		opts.Compress = v
	}
	// 解析 level
	if v := query.Get("level"); v != "" {
		lvl, err := zapcore.ParseLevel(v)
		if err != nil {
			return nil, fmt.Errorf("invalid level: %w", err)
		}
		opts.Level = lvl
		opts.LevelSet = true
	}
	return opts, nil
}

// parseHTTPOptions 解析 HTTP 适配器 DSN
// 格式: http://localhost:3000/logs?timeout=10s&buffer-size=1024&batch-size=100&max-retries=3
func parseHTTPOptions(dsn string) (*HTTPOptions, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP DSN: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme for HTTP: %s", u.Scheme)
	}

	// 构建基础 URL (不带查询参数)
	baseURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}

	opts := &HTTPOptions{
		URL:        baseURL.String(),
		Timeout:    10 * time.Second,  // 默认 10s
		BufferSize: 1024,              // 默认 1024 条
		BatchSize:  100,               // 默认 100 条
		MaxRetries: 3,                 // 默认重试 3 次
		Level:      zapcore.InfoLevel, // 默认 info 级别
	}

	query := u.Query()

	// 解析 timeout
	if v := query.Get("timeout"); v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		opts.Timeout = timeout
	}

	// 解析 buffer-size
	if v := query.Get("buffer-size"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid buffer-size: %w", err)
		}
		opts.BufferSize = size
	}

	// 解析 batch-size
	if v := query.Get("batch-size"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid batch-size: %w", err)
		}
		opts.BatchSize = size
	}

	// 解析 max-retries
	if v := query.Get("max-retries"); v != "" {
		retries, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid max-retries: %w", err)
		}
		opts.MaxRetries = retries
	}

	// 解析 level
	if v := query.Get("level"); v != "" {
		lvl, err := zapcore.ParseLevel(v)
		if err != nil {
			return nil, fmt.Errorf("invalid level: %w", err)
		}
		opts.Level = lvl
		opts.LevelSet = true
	}
	return opts, nil
}

// parseSizeString 解析大小字符串 (支持 10m, 100m, 1g 等)
func parseSizeString(s string) (int, error) {
	matches := reSize.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid size format: %s (expected: 10m, 100mb, 1g, etc.)", s)
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	unit := matches[2]
	switch unit {
	case "", "m", "mb":
		return num, nil
	case "g", "gb":
		return num * 1024, nil
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}

// parseDurationString 解析时长字符串 (支持 1d, 7d, 30d 等)
func parseDurationString(s string) (int, error) {
	matches := reDays.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format: %s (expected: 1d, 7days, 30d, etc.)", s)
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return num, nil
}
