package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestClientStatus_String(t *testing.T) {
	tests := []struct {
		status   ClientStatus
		expected string
	}{
		{StatusUnknown, "unknown"},
		{StatusOnline, "online"},
		{StatusOffline, "offline"},
		{StatusTimeout, "timeout"},
		{ClientStatus(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("ClientStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClientStatus_MarshalJSON(t *testing.T) {
	tests := []struct {
		status   ClientStatus
		expected string
	}{
		{StatusOnline, `"online"`},
		{StatusOffline, `"offline"`},
		{StatusUnknown, `"unknown"`},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got, err := tt.status.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON() error = %v", err)
				return
			}
			if string(got) != tt.expected {
				t.Errorf("MarshalJSON() = %v, want %v", string(got), tt.expected)
			}
		})
	}
}

func TestClientInfo_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		lastHeartbeat time.Time
		timeout       time.Duration
		expected      bool
	}{
		{
			name:          "not expired - recent heartbeat",
			lastHeartbeat: now,
			timeout:       30 * time.Second,
			expected:      false,
		},
		{
			name:          "expired - old heartbeat",
			lastHeartbeat: now.Add(-60 * time.Second),
			timeout:       30 * time.Second,
			expected:      true,
		},
		{
			name:          "edge case - just under timeout",
			lastHeartbeat: now.Add(-29 * time.Second),
			timeout:       30 * time.Second,
			expected:      false,
		},
		{
			name:          "expired - just over timeout",
			lastHeartbeat: now.Add(-31 * time.Second),
			timeout:       30 * time.Second,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientInfo{
				LastHeartbeat: tt.lastHeartbeat,
			}
			if got := c.IsExpired(tt.timeout); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClientInfo_JSON(t *testing.T) {
	client := &ClientInfo{
		ID:            "test-id",
		Name:          "test-client",
		IP:            "192.168.1.1",
		Port:          8080,
		Metadata:      map[string]string{"key": "value"},
		LastHeartbeat: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:        StatusOnline,
		RegisteredAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded ClientInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.ID != client.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, client.ID)
	}
	if decoded.Name != client.Name {
		t.Errorf("Name = %v, want %v", decoded.Name, client.Name)
	}
	if decoded.IP != client.IP {
		t.Errorf("IP = %v, want %v", decoded.IP, client.IP)
	}
}

func TestServerInfo_JSON(t *testing.T) {
	server := &ServerInfo{
		ID:       "server-1",
		Name:     "main-server",
		Hostname: "10.0.0.1",
		Metadata: map[string]string{"version": "1.0"},
	}

	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded ServerInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.ID != server.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, server.ID)
	}
	if decoded.Name != server.Name {
		t.Errorf("Name = %v, want %v", decoded.Name, server.Name)
	}
}

func TestWSMessage_JSON(t *testing.T) {
	msg := &WSMessage{
		Type: MsgTypeHeartbeat,
		Payload: &HeartbeatReq{
			ID:       "client-1",
			Metadata: map[string]string{"cpu": "50%"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Type != MsgTypeHeartbeat {
		t.Errorf("Type = %v, want %v", decoded.Type, MsgTypeHeartbeat)
	}
}

func TestStoreConfig_Options(t *testing.T) {
	cfg := DefaultStoreConfig()

	// 验证默认值
	if cfg.KeyPrefix != "heartbeat:" {
		t.Errorf("KeyPrefix = %v, want heartbeat:", cfg.KeyPrefix)
	}
	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want 30s", cfg.DefaultTimeout)
	}
	if cfg.CleanupInterval != 10*time.Second {
		t.Errorf("CleanupInterval = %v, want 10s", cfg.CleanupInterval)
	}
	if !cfg.AutoCleanup {
		t.Errorf("AutoCleanup = %v, want true", cfg.AutoCleanup)
	}

	// 测试选项函数
	ApplyOptions(cfg,
		WithKeyPrefix("custom:"),
		WithDefaultTimeout(60*time.Second),
		WithCleanupInterval(20*time.Second),
		WithAutoCleanup(false),
	)

	if cfg.KeyPrefix != "custom:" {
		t.Errorf("KeyPrefix = %v, want custom:", cfg.KeyPrefix)
	}
	if cfg.DefaultTimeout != 60*time.Second {
		t.Errorf("DefaultTimeout = %v, want 60s", cfg.DefaultTimeout)
	}
	if cfg.CleanupInterval != 20*time.Second {
		t.Errorf("CleanupInterval = %v, want 20s", cfg.CleanupInterval)
	}
	if cfg.AutoCleanup {
		t.Errorf("AutoCleanup = %v, want false", cfg.AutoCleanup)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// 确保常量值正确
	if MsgTypeRegister != "register" {
		t.Errorf("MsgTypeRegister = %v, want register", MsgTypeRegister)
	}
	if MsgTypeHeartbeat != "heartbeat" {
		t.Errorf("MsgTypeHeartbeat = %v, want heartbeat", MsgTypeHeartbeat)
	}
	if MsgTypeBroadcast != "broadcast" {
		t.Errorf("MsgTypeBroadcast = %v, want broadcast", MsgTypeBroadcast)
	}
	if MsgTypePing != "ping" {
		t.Errorf("MsgTypePing = %v, want ping", MsgTypePing)
	}
	if MsgTypePong != "pong" {
		t.Errorf("MsgTypePong = %v, want pong", MsgTypePong)
	}
}
