package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mulan-go/mulan/heartbeat/core"
)

func newTestStore() *MemoryStore {
	return NewMemoryStore(core.WithAutoCleanup(false))
}

func TestMemoryStore_Register(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()
	client := &core.ClientInfo{
		ID:   "client-1",
		Name: "test-client",
		IP:   "192.168.1.1",
	}

	err := s.Register(ctx, client)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// 验证注册成功
	got, err := s.Get(ctx, "client-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.ID != client.ID {
		t.Errorf("ID = %v, want %v", got.ID, client.ID)
	}
	if got.Status != core.StatusOnline {
		t.Errorf("Status = %v, want %v", got.Status, core.StatusOnline)
	}
	if got.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should not be zero")
	}
	if got.LastHeartbeat.IsZero() {
		t.Error("LastHeartbeat should not be zero")
	}
}

func TestMemoryStore_Unregister(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	client := &core.ClientInfo{ID: "client-1", Name: "test"}
	_ = s.Register(ctx, client)

	// 注销客户端
	err := s.Unregister(ctx, "client-1")
	if err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	// 验证已删除
	_, err = s.Get(ctx, "client-1")
	if !errors.Is(err, core.ErrClientNotFound) {
		t.Errorf("Get() error = %v, want ErrClientNotFound", err)
	}
}

func TestMemoryStore_Unregister_NotFound(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	err := s.Unregister(ctx, "non-existent")
	if !errors.Is(err, core.ErrClientNotFound) {
		t.Errorf("Unregister() error = %v, want ErrClientNotFound", err)
	}
}

func TestMemoryStore_Heartbeat(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	client := &core.ClientInfo{ID: "client-1", Name: "test"}
	_ = s.Register(ctx, client)

	// 等待一点时间
	time.Sleep(10 * time.Millisecond)

	// 发送心跳
	metadata := map[string]string{"cpu": "50%", "memory": "1GB"}
	err := s.Heartbeat(ctx, "client-1", metadata)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	// 验证心跳时间更新
	got, _ := s.Get(ctx, "client-1")
	if got.LastHeartbeat.Before(client.RegisteredAt) {
		t.Error("LastHeartbeat should be updated")
	}

	// 验证 metadata 合并
	if got.Metadata["cpu"] != "50%" {
		t.Errorf("Metadata[cpu] = %v, want 50%%", got.Metadata["cpu"])
	}
	if got.Metadata["memory"] != "1GB" {
		t.Errorf("Metadata[memory] = %v, want 1GB", got.Metadata["memory"])
	}
}

func TestMemoryStore_Heartbeat_NotFound(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	err := s.Heartbeat(ctx, "non-existent", nil)
	if !errors.Is(err, core.ErrClientNotFound) {
		t.Errorf("Heartbeat() error = %v, want ErrClientNotFound", err)
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	_, err := s.Get(ctx, "non-existent")
	if !errors.Is(err, core.ErrClientNotFound) {
		t.Errorf("Get() error = %v, want ErrClientNotFound", err)
	}
}

func TestMemoryStore_List(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	// 注册多个客户端
	for i := 0; i < 3; i++ {
		client := &core.ClientInfo{
			ID:   "client-" + string(rune('1'+i)),
			Name: "test",
		}
		_ = s.Register(ctx, client)
	}

	clients, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(clients) != 3 {
		t.Errorf("List() len = %v, want 3", len(clients))
	}
}

func TestMemoryStore_ListOnline(t *testing.T) {
	s := NewMemoryStore(
		core.WithAutoCleanup(false),
		core.WithDefaultTimeout(50*time.Millisecond),
	)
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	client1 := &core.ClientInfo{ID: "client-1", Name: "test1"}
	client2 := &core.ClientInfo{ID: "client-2", Name: "test2"}
	_ = s.Register(ctx, client1)
	_ = s.Register(ctx, client2)

	// 验证都在线
	online, _ := s.ListOnline(ctx)
	if len(online) != 2 {
		t.Errorf("ListOnline() len = %v, want 2", len(online))
	}

	// 等待超时
	time.Sleep(60 * time.Millisecond)

	// 只给 client-1 发送心跳
	_ = s.Heartbeat(ctx, "client-1", nil)

	// 验证只有 client-1 在线
	online, _ = s.ListOnline(ctx)
	if len(online) != 1 {
		t.Errorf("ListOnline() len = %v, want 1", len(online))
	}
	if online[0].ID != "client-1" {
		t.Errorf("Online client ID = %v, want client-1", online[0].ID)
	}
}

func TestMemoryStore_Count(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	// 初始为 0
	count, _ := s.Count(ctx)
	if count != 0 {
		t.Errorf("Count() = %v, want 0", count)
	}

	// 注册客户端
	for i := 0; i < 5; i++ {
		client := &core.ClientInfo{ID: "client-" + string(rune('1'+i))}
		_ = s.Register(ctx, client)
	}

	count, _ = s.Count(ctx)
	if count != 5 {
		t.Errorf("Count() = %v, want 5", count)
	}
}

func TestMemoryStore_CountOnline(t *testing.T) {
	s := NewMemoryStore(
		core.WithAutoCleanup(false),
		core.WithDefaultTimeout(50*time.Millisecond),
	)
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	for i := 0; i < 3; i++ {
		client := &core.ClientInfo{ID: "client-" + string(rune('1'+i))}
		_ = s.Register(ctx, client)
	}

	// 都在线
	count, _ := s.CountOnline(ctx)
	if count != 3 {
		t.Errorf("CountOnline() = %v, want 3", count)
	}

	// 等待超时
	time.Sleep(60 * time.Millisecond)

	// 只有一个活跃
	_ = s.Heartbeat(ctx, "client-1", nil)

	count, _ = s.CountOnline(ctx)
	if count != 1 {
		t.Errorf("CountOnline() = %v, want 1", count)
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	for i := 0; i < 3; i++ {
		client := &core.ClientInfo{
			ID:            "client-" + string(rune('1'+i)),
			LastHeartbeat: time.Now().Add(-60 * time.Second), // 已超时
		}
		s.clients.Store(client.ID, client)
	}

	// 清理超时客户端 (30s 超时)
	cleaned, err := s.Cleanup(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if cleaned != 3 {
		t.Errorf("Cleanup() = %v, want 3", cleaned)
	}

	// 验证已清理
	count, _ := s.Count(ctx)
	if count != 0 {
		t.Errorf("Count() = %v, want 0", count)
	}
}

func TestMemoryStore_Close(t *testing.T) {
	s := newTestStore()

	// 关闭存储
	err := s.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// 关闭后操作应返回错误
	ctx := context.Background()

	err = s.Register(ctx, &core.ClientInfo{ID: "test"})
	if !errors.Is(err, core.ErrStoreClosed) {
		t.Errorf("Register() after close error = %v, want ErrStoreClosed", err)
	}

	_, err = s.Get(ctx, "test")
	if !errors.Is(err, core.ErrStoreClosed) {
		t.Errorf("Get() after close error = %v, want ErrStoreClosed", err)
	}

	err = s.Heartbeat(ctx, "test", nil)
	if !errors.Is(err, core.ErrStoreClosed) {
		t.Errorf("Heartbeat() after close error = %v, want ErrStoreClosed", err)
	}

	// 多次关闭应该安全
	err = s.Close()
	if err != nil {
		t.Errorf("Double Close() error = %v", err)
	}
}

func TestMemoryStore_AutoCleanup(t *testing.T) {
	s := NewMemoryStore(
		core.WithAutoCleanup(true),
		core.WithDefaultTimeout(50*time.Millisecond),
		core.WithCleanupInterval(30*time.Millisecond),
	)
	defer s.Close()

	ctx := context.Background()

	// 注册客户端
	client := &core.ClientInfo{ID: "client-1"}
	_ = s.Register(ctx, client)

	// 验证注册成功
	count, _ := s.Count(ctx)
	if count != 1 {
		t.Errorf("Count() = %v, want 1", count)
	}

	// 等待自动清理触发 (超时 50ms + 清理间隔 30ms + buffer)
	time.Sleep(150 * time.Millisecond)

	// 验证已被清理
	count, _ = s.Count(ctx)
	if count != 0 {
		t.Errorf("Count() after auto cleanup = %v, want 0", count)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	s := newTestStore()
	defer s.Close()

	ctx := context.Background()
	done := make(chan bool)

	// 并发注册
	for i := 0; i < 10; i++ {
		go func(id int) {
			client := &core.ClientInfo{
				ID:   "client-" + string(rune('A'+id)),
				Name: "test",
			}
			_ = s.Register(ctx, client)
			done <- true
		}(i)
	}

	// 等待所有注册完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = s.List(ctx)
			_, _ = s.Count(ctx)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	count, _ := s.Count(ctx)
	if count != 10 {
		t.Errorf("Count() = %v, want 10", count)
	}
}
