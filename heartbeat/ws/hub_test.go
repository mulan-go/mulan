package ws

import (
	"testing"
	"time"

	"github.com/mulan-go/mulan/heartbeat/core"
	"github.com/mulan-go/mulan/heartbeat/store"
)

func TestHub_NewHub(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	serverInfo := &core.ServerInfo{
		ID:   "test-server",
		Name: "Test Server",
	}

	hub := NewHub(s, serverInfo)
	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}

	if hub.store != s {
		t.Error("Store not set correctly")
	}
	if hub.serverInfo != serverInfo {
		t.Error("ServerInfo not set correctly")
	}
}

func TestHub_RunAndStop(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	// 启动 Hub
	hub.Run()

	// 给一点时间让协程启动
	time.Sleep(10 * time.Millisecond)

	// 停止 Hub
	hub.Stop()

	// 验证停止成功（不会 panic）
}

func TestHub_ClientCount(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	// 初始为 0
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("ClientCount() = %d, want 0", count)
	}
}

func TestHub_ClientIDs(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	// 初始为空
	ids := hub.ClientIDs()
	if len(ids) != 0 {
		t.Errorf("ClientIDs() len = %d, want 0", len(ids))
	}
}

func TestHub_SendTo_NoClient(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	msg := &core.WSMessage{
		Type:    core.MsgTypePing,
		Payload: nil,
	}

	// 发送给不存在的客户端应该返回 false
	if hub.SendTo("non-existent", msg) {
		t.Error("SendTo() should return false for non-existent client")
	}
}

func TestHub_Callbacks(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	joinCalled := false
	leaveCalled := false

	hub.OnClientJoin(func(clientID string) {
		joinCalled = true
	})

	hub.OnClientLeave(func(clientID string) {
		leaveCalled = true
	})

	// 验证回调已设置
	if hub.onClientJoin == nil {
		t.Error("onClientJoin callback not set")
	}
	if hub.onClientLeave == nil {
		t.Error("onClientLeave callback not set")
	}

	// 注意：实际调用需要通过 WebSocket 连接，这里只验证设置
	_ = joinCalled
	_ = leaveCalled
}

func TestConn_Send(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	// 创建一个模拟的 Conn（没有真实的 websocket.Conn）
	conn := &Conn{
		ID:      "test-client",
		hub:     hub,
		sendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}

	msg := &core.WSMessage{
		Type:    core.MsgTypePing,
		Payload: map[string]string{"test": "value"},
	}

	// 发送消息
	err := conn.Send(msg)
	if err != nil {
		t.Errorf("Send() error = %v", err)
	}

	// 验证消息在通道中
	select {
	case data := <-conn.sendCh:
		if len(data) == 0 {
			t.Error("Received empty data")
		}
	default:
		t.Error("No message in send channel")
	}
}

func TestConn_Send_Closed(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	conn := &Conn{
		ID:      "test-client",
		hub:     hub,
		sendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
		closed:  true, // 已关闭
	}

	msg := &core.WSMessage{
		Type: core.MsgTypePing,
	}

	// 发送到已关闭的连接应该不报错
	err := conn.Send(msg)
	if err != nil {
		t.Errorf("Send() to closed conn error = %v", err)
	}
}

func TestConn_Close(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	hub := NewHub(s, &core.ServerInfo{ID: "test"})

	conn := &Conn{
		ID:      "test-client",
		hub:     hub,
		sendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}

	// 关闭连接
	conn.Close()

	// 验证已关闭
	if !conn.closed {
		t.Error("Connection should be marked as closed")
	}

	// 多次关闭应该安全
	conn.Close()
}
