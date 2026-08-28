package log_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mulan-go/mulan/log"
	"go.uber.org/zap"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func logPrint() {
	zap.L().Info("test info")
	zap.L().Warn("test warn")
	zap.L().Error("test error")
	zap.L().Debug("test debug")
	zap.L().Info("test info", zap.String("aa", "awdvews"))
	zap.L().Warn("test warn", zap.String("aa", "awdvews"))
	zap.L().Error("test error", zap.String("aa", "awdvews"))
	zap.L().Debug("test debug", zap.String("aa", "awdvews"))
}

func TestLocalModeConsoleOutputIsColorizedAndDebug(t *testing.T) {
	output := captureStdout(t, func() {
		logger, err := log.NewWithConfig(&log.Config{Mode: "local"})
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		zap.L().Debug("local debug")
		_ = logger.Sync()
	})

	if !strings.Contains(output, "local debug") {
		t.Fatalf("expected debug log in local output, got %q", output)
	}
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected color escape sequence in local output, got %q", output)
	}
}

func TestServerModeConsoleOutputIsJSONAndInfo(t *testing.T) {
	output := captureStdout(t, func() {
		logger, err := log.NewWithConfig(&log.Config{Mode: "server"})
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		zap.L().Debug("server debug")
		zap.L().Info("server info", zap.String("component", "api"))
		_ = logger.Sync()
	})

	if strings.Contains(output, "server debug") {
		t.Fatalf("expected server debug log to be filtered, got %q", output)
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &entry); err != nil {
		t.Fatalf("expected JSON log output, got %q: %v", output, err)
	}
	if entry["level"] != "info" {
		t.Fatalf("expected info level, got %#v", entry["level"])
	}
	if entry["msg"] != "server info" {
		t.Fatalf("expected server info message, got %#v", entry["msg"])
	}
	if entry["component"] != "api" {
		t.Fatalf("expected component field, got %#v", entry["component"])
	}
}

func TestLog(t *testing.T) {
	logger, err := log.NewWithConfig(&log.Config{Level: "info"})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()
}

func TestLogDebug(t *testing.T) {
	logger, err := log.NewWithConfig(&log.Config{Level: "debug"})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()
}

func TestLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "_test.log")
	t.Log("log file:", logFile)

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "debug",
		Adaptors: []string{"file://" + logFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()

	// 验证文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file not created")
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("log file content:", string(content))
}

func TestLogFileLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "_test.log")
	t.Log("log file:", logFile)

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "info",
		Adaptors: []string{"file://" + logFile + "?level=debug"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()

	// 验证文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file not created")
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("log file content:", string(content))
}

func TestLogFileInheritsGlobalLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "_test.log")

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "debug",
		JSON:     true,
		Adaptors: []string{"file://" + logFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	zap.L().Debug("file inherited debug")
	_ = logger.Sync()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "file inherited debug") {
		t.Fatalf("expected file adaptor to inherit debug level, got %q", string(content))
	}
}

func TestLogFileLevelOverridesGlobalLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "_test.log")

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "info",
		JSON:     true,
		Adaptors: []string{"file://" + logFile + "?level=debug"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	zap.L().Debug("file override debug")
	_ = logger.Sync()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "file override debug") {
		t.Fatalf("expected file adaptor to use debug override, got %q", string(content))
	}
}

func TestLogFileDSN(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "_test.log")

	// 使用 DSN 格式配置
	dsn := fmt.Sprintf("file://%s?max-size=10m&max-backups=5&max-age=7d&compress=gzip", logFile)
	t.Log("DSN:", dsn)

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "info",
		Adaptors: []string{dsn},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()

	// 验证文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file not created")
	}
}

func TestLogFileRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-rotation.log")

	// 使用 DSN 配置滚动
	dsn := fmt.Sprintf("file://%s?max-size=1m&max-backups=3&max-age=7d", logFile)

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "debug",
		Adaptors: []string{dsn},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 写入大量日志触发滚动
	for i := range 10000 {
		zap.L().Info("test log rotation", zap.Int("iteration", i))
	}

	t.Log("log file created:", logFile)
}

func TestLogHTTP(t *testing.T) {
	t.Skip("需要实际的 HTTP 服务端")

	// 使用 DSN 配置 HTTP
	dsn := "http://localhost:3003/log?timeout=5s&buffer-size=512&batch-size=50&max-retries=2"

	logger, err := log.NewWithConfig(&log.Config{
		Level:    "debug",
		Adaptors: []string{dsn},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()
	// 等待异步发送完成
	time.Sleep(2 * time.Second)
}

func TestLogWithName(t *testing.T) {
	logger, err := log.New("my-service")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()
}

func TestLogInvalidLevel(t *testing.T) {
	_, err := log.NewWithConfig(&log.Config{Level: "invalid"})
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestLogMultipleAdaptors(t *testing.T) {
	tmpDir := t.TempDir()
	logFile1 := filepath.Join(tmpDir, "test1.log")
	logFile2 := filepath.Join(tmpDir, "test2.log")

	logger, err := log.NewWithConfig(&log.Config{
		Level: "info",
		Adaptors: []string{
			"file://" + logFile1,
			"file://" + logFile2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logPrint()

	// 验证两个文件都被创建
	if _, err := os.Stat(logFile1); os.IsNotExist(err) {
		t.Error("log file 1 not created")
	}
	if _, err := os.Stat(logFile2); os.IsNotExist(err) {
		t.Error("log file 2 not created")
	}
}
