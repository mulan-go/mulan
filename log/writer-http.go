package log

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

// HTTPWriter 异步批量发送日志到 HTTP 端点
type HTTPWriter struct {
	ctx        context.Context
	client     *http.Client
	buffer     chan []byte
	cancel     context.CancelFunc
	url        string
	wg         sync.WaitGroup
	batchSize  int
	maxRetries int
}

// Write 实现 io.Writer 接口
func (w *HTTPWriter) Write(p []byte) (n int, err error) {
	// 复制数据避免外部修改
	data := make([]byte, len(p))
	copy(data, p)
	// 非阻塞写入
	select {
	case w.buffer <- data:
		return len(p), nil
	default:
		// 缓冲区满，直接丢弃（或者可以选择同步发送）
		return len(p), fmt.Errorf("log buffer full, dropping log")
	}
}

// Sync 实现 zapcore.WriteSyncer 接口
func (w *HTTPWriter) Sync() error { return nil }

// Close 关闭 writer 并等待所有日志发送完成
func (w *HTTPWriter) Close() error {
	w.cancel()
	close(w.buffer)
	w.wg.Wait()
	return nil
}

// worker 后台工作协程，批量发送日志
func (w *HTTPWriter) worker() {
	defer w.wg.Done()
	batch := make([][]byte, 0, w.batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			// 发送剩余日志
			if len(batch) > 0 {
				_ = w.sendBatch(batch)
			}
			return
		case data, ok := <-w.buffer:
			if !ok {
				// 通道关闭，发送剩余日志
				if len(batch) > 0 {
					_ = w.sendBatch(batch)
				}
				return
			}
			batch = append(batch, data)
			if len(batch) >= w.batchSize {
				_ = w.sendBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			// 定时发送
			if len(batch) > 0 {
				_ = w.sendBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatch 批量发送日志
func (w *HTTPWriter) sendBatch(batch [][]byte) error {
	if len(batch) == 0 {
		return nil
	}
	// 合并所有日志
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, data := range batch {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(bytes.TrimSpace(data))
	}
	buf.WriteByte(']')
	// 重试发送
	var lastErr error
	for i := 0; i <= w.maxRetries; i++ {
		if err := w.post(buf.Bytes()); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second) // 指数退避
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to send logs after %d retries: %w", w.maxRetries, lastErr)
}

// post 发送 HTTP 请求
func (w *HTTPWriter) post(data []byte) error {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()
	// 读取并丢弃响应体
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	return nil
}

// newHTTPWriter 创建 HTTP writer
func newHTTPWriter(opts *HTTPOptions) (zapcore.WriteSyncer, io.Closer, error) {
	if opts.URL == "" {
		return nil, nil, fmt.Errorf("HTTP URL is empty")
	}
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   opts.Timeout,
	}
	ctx, cancel := context.WithCancel(context.Background())
	writer := &HTTPWriter{
		url:        opts.URL,
		client:     client,
		buffer:     make(chan []byte, opts.BufferSize),
		batchSize:  opts.BatchSize,
		maxRetries: opts.MaxRetries,
		ctx:        ctx,
		cancel:     cancel,
	}
	// 启动后台工作协程
	writer.wg.Add(1)
	go writer.worker()
	return writer, writer, nil
}
