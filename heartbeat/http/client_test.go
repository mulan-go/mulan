package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mulan-go/mulan/heartbeat/core"
)

func TestClient_NewClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	// 验证默认配置
	if client.cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("ServerURL = %v, want http://localhost:8080", client.cfg.ServerURL)
	}
	if client.cfg.HeartbeatPeriod != 10*time.Second {
		t.Errorf("HeartbeatPeriod = %v, want 10s", client.cfg.HeartbeatPeriod)
	}
	if client.clientID == "" {
		t.Error("ClientID should not be empty")
	}
}

func TestClient_NewClient_WithConfig(t *testing.T) {
	cfg := &ClientConfig{
		ServerURL:       "http://example.com:9090",
		ClientID:        "custom-id",
		ClientName:      "test-client",
		HeartbeatPeriod: 5 * time.Second,
	}

	client := NewClient(cfg)

	if client.cfg.ServerURL != "http://example.com:9090" {
		t.Errorf("ServerURL = %v, want http://example.com:9090", client.cfg.ServerURL)
	}
	if client.clientID != "custom-id" {
		t.Errorf("ClientID = %v, want custom-id", client.clientID)
	}
}

func TestClient_Callbacks(t *testing.T) {
	client := NewClient(nil)

	connectedCalled := false
	disconnectedCalled := false
	heartbeatCalled := false

	client.OnConnected(func(server *core.ServerInfo) {
		connectedCalled = true
	})

	client.OnDisconnected(func(err error) {
		disconnectedCalled = true
	})

	client.OnHeartbeat(func(rsp *core.HeartbeatRsp) {
		heartbeatCalled = true
	})

	// 验证回调已设置
	if client.onConnected == nil {
		t.Error("onConnected callback not set")
	}
	if client.onDisconnected == nil {
		t.Error("onDisconnected callback not set")
	}
	if client.onHeartbeat == nil {
		t.Error("onHeartbeat callback not set")
	}

	_ = connectedCalled
	_ = disconnectedCalled
	_ = heartbeatCalled
}

func TestClient_Register(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/heartbeat/register" {
			t.Errorf("Path = %v, want /heartbeat/register", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Method = %v, want POST", r.Method)
		}

		rsp := &core.RegisterRsp{
			ClientID: "assigned-id",
			Server: &core.ServerInfo{
				ID:   "server-1",
				Name: "Test Server",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rsp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		ServerURL:  server.URL,
		ClientName: "test-client",
	})

	err := client.Register(context.Background())
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if client.clientID != "assigned-id" {
		t.Errorf("ClientID = %v, want assigned-id", client.clientID)
	}
	if client.serverInfo == nil {
		t.Error("ServerInfo should not be nil")
	}
	if client.serverInfo.ID != "server-1" {
		t.Errorf("ServerInfo.ID = %v, want server-1", client.serverInfo.ID)
	}
}

func TestClient_Heartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/heartbeat/heartbeat" {
			t.Errorf("Path = %v, want /heartbeat/heartbeat", r.URL.Path)
		}

		var req core.HeartbeatReq
		json.NewDecoder(r.Body).Decode(&req)

		if req.ID != "client-1" {
			t.Errorf("Request ID = %v, want client-1", req.ID)
		}

		rsp := &core.HeartbeatRsp{
			OK:        true,
			Timestamp: time.Now(),
			Server: &core.ServerInfo{
				ID: "server-1",
			},
			Clients:  5,
			Interval: 10,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rsp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		ServerURL: server.URL,
		ClientID:  "client-1",
	})

	metadata := map[string]string{"cpu": "50%"}
	rsp, err := client.Heartbeat(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	if !rsp.OK {
		t.Error("OK should be true")
	}
	if rsp.Clients != 5 {
		t.Errorf("Clients = %d, want 5", rsp.Clients)
	}
}

func TestClient_Unregister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/heartbeat/unregister" {
			t.Errorf("Path = %v, want /heartbeat/unregister", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Method = %v, want DELETE", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		ServerURL: server.URL,
		ClientID:  "client-1",
	})

	err := client.Unregister(context.Background())
	if err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
}

func TestClient_IsRunning(t *testing.T) {
	client := NewClient(nil)

	if client.IsRunning() {
		t.Error("Client should not be running initially")
	}
}

func TestClient_ClientID(t *testing.T) {
	client := NewClient(&ClientConfig{
		ClientID: "my-client",
	})

	if client.ClientID() != "my-client" {
		t.Errorf("ClientID() = %v, want my-client", client.ClientID())
	}
}

func TestClient_ServerInfo(t *testing.T) {
	client := NewClient(nil)

	if client.ServerInfo() != nil {
		t.Error("ServerInfo should be nil before connecting")
	}
}

func TestClientError(t *testing.T) {
	err := &ClientError{
		StatusCode: 404,
		Message:    "not found",
	}

	if err.Error() != "not found" {
		t.Errorf("Error() = %v, want not found", err.Error())
	}
}

func TestClient_Register_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		ServerURL: server.URL,
	})

	err := client.Register(context.Background())
	if err == nil {
		t.Error("Register() should return error")
	}

	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Errorf("Error type = %T, want *ClientError", err)
	} else if clientErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", clientErr.StatusCode)
	}
}

func TestClient_Heartbeat_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("client not found"))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		ServerURL: server.URL,
		ClientID:  "non-existent",
	})

	_, err := client.Heartbeat(context.Background(), nil)
	if err == nil {
		t.Error("Heartbeat() should return error")
	}
}

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("ServerURL = %v, want http://localhost:8080", cfg.ServerURL)
	}
	if cfg.HeartbeatPeriod != 10*time.Second {
		t.Errorf("HeartbeatPeriod = %v, want 10s", cfg.HeartbeatPeriod)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("RetryCount = %v, want 3", cfg.RetryCount)
	}
	if !cfg.AutoRegister {
		t.Error("AutoRegister should be true by default")
	}
}
