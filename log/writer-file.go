package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// fileWriterCloser 文件写入器的包装，支持滚动和关闭
type fileWriterCloser struct {
	zapcore.WriteSyncer
	closer io.Closer
}

func (f *fileWriterCloser) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

// newFileWriter 创建带滚动功能的文件写入器
func newFileWriter(opts *FileOptions) (zapcore.WriteSyncer, io.Closer, error) {
	if opts.Path == "" {
		return nil, nil, fmt.Errorf("file path is empty")
	}
	// 确保目录存在
	dir := filepath.Dir(opts.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	// 使用 lumberjack 实现日志滚动
	logger := &lumberjack.Logger{
		Filename:   opts.Path,
		MaxSize:    opts.MaxSize,            // MB
		MaxBackups: opts.MaxBackups,         // 保留的旧文件数量
		MaxAge:     opts.MaxAge,             // 天
		Compress:   opts.Compress == "gzip", // 是否压缩
		LocalTime:  true,                    // 使用本地时间
	}
	wrapper := &fileWriterCloser{
		WriteSyncer: zapcore.AddSync(logger),
		closer:      logger,
	}
	return wrapper, wrapper, nil
}
