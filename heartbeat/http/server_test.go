package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/rsp"

	"github.com/mulan-go/mulan/heartbeat/core"
	"github.com/mulan-go/mulan/heartbeat/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// APIResponse 通用 API 响应结构
type APIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func newTestHandler() (*Handler, *store.MemoryStore, *gin.Engine) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	h := NewHandler(s, &core.ServerInfo{
		ID:   "test-server",
		Name: "Test Server",
	}, nil)

	engine := gin.New()
	h.RegisterRoutesWithPrefix(engine, "/heartbeat")

	return h, s, engine
}

func TestHandler_NewHandler(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	h := NewHandler(s, nil, nil)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}

	// 验证自动生成 ServerInfo
	if h.ServerInfo() == nil {
		t.Error("ServerInfo should not be nil")
	}
	if h.ServerInfo().ID == "" {
		t.Error("ServerInfo.ID should not be empty")
	}
}

func TestHandler_Register(t *testing.T) {
	_, _, engine := newTestHandler()

	req := &core.RegisterReq{
		Name: "test-client",
		IP:   "192.168.1.100",
		Port: 9000,
		Metadata: map[string]string{
			"version": "1.0",
		},
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/register", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &apiRsp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData core.RegisterRsp
	if err := json.Unmarshal(apiRsp.Data, &rspData); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if rspData.ClientID == "" {
		t.Error("ClientID should not be empty")
	}
	if rspData.Server == nil {
		t.Error("Server should not be nil")
	}
	if rspData.Server.ID != "test-server" {
		t.Errorf("Server.ID = %v, want test-server", rspData.Server.ID)
	}
}

func TestHandler_Register_WithCustomID(t *testing.T) {
	_, _, engine := newTestHandler()

	req := &core.RegisterReq{
		ID:   "custom-client-id",
		Name: "test-client",
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/register", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData core.RegisterRsp
	_ = json.Unmarshal(apiRsp.Data, &rspData)

	if rspData.ClientID != "custom-client-id" {
		t.Errorf("ClientID = %v, want custom-client-id", rspData.ClientID)
	}
}

func TestHandler_Heartbeat(t *testing.T) {
	_, s, engine := newTestHandler()

	// 先注册客户端
	_ = s.Register(context.Background(), &core.ClientInfo{ID: "client-1", Name: "test"})

	req := &core.HeartbeatReq{
		ID: "client-1",
		Metadata: map[string]string{
			"cpu": "50%",
		},
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/heartbeat", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &apiRsp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData core.HeartbeatRsp
	if err := json.Unmarshal(apiRsp.Data, &rspData); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if !rspData.OK {
		t.Error("OK should be true")
	}
	if rspData.Server == nil {
		t.Error("Server should not be nil")
	}
	if rspData.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestHandler_Heartbeat_NotFound(t *testing.T) {
	_, _, engine := newTestHandler()

	req := &core.HeartbeatReq{
		ID: "non-existent",
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/heartbeat", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusNotFound)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	// NotFound code
	if apiRsp.Code != rsp.ErrNotFound.Code {
		t.Errorf("Code = %d, want %d (NotFound)", apiRsp.Code, rsp.ErrNotFound.Code)
	}
}

func TestHandler_Heartbeat_MissingID(t *testing.T) {
	_, _, engine := newTestHandler()

	req := &core.HeartbeatReq{
		ID: "", // Empty ID
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/heartbeat", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	// ParamInvalid code
	if apiRsp.Code != rsp.ErrParamInvalid.Code {
		t.Errorf("Code = %d, want %d (ParamInvalid)", apiRsp.Code, rsp.ErrParamInvalid.Code)
	}
}

func TestHandler_Unregister(t *testing.T) {
	_, s, engine := newTestHandler()

	// 先注册客户端
	_ = s.Register(context.Background(), &core.ClientInfo{ID: "client-1", Name: "test"})

	req := &core.UnregisterReq{
		ID: "client-1",
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/unregister", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	// 验证已删除
	count, _ := s.Count(context.Background())
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}
}

func TestHandler_Unregister_NotFound(t *testing.T) {
	_, _, engine := newTestHandler()

	req := &core.UnregisterReq{
		ID: "non-existent",
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/unregister", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusNotFound)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	if apiRsp.Code != rsp.ErrNotFound.Code {
		t.Errorf("Code = %d, want %d (NotFound)", apiRsp.Code, rsp.ErrNotFound.Code)
	}
}

func TestHandler_ListClients(t *testing.T) {
	_, s, engine := newTestHandler()

	// 注册多个客户端
	for i := 0; i < 3; i++ {
		_ = s.Register(context.Background(), &core.ClientInfo{
			ID:   "client-" + string(rune('1'+i)),
			Name: "test",
		})
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/clients", nil)
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &apiRsp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData core.ClientListRsp
	if err := json.Unmarshal(apiRsp.Data, &rspData); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if rspData.Total != 3 {
		t.Errorf("Total = %d, want 3", rspData.Total)
	}
	if len(rspData.Clients) != 3 {
		t.Errorf("Clients len = %d, want 3", len(rspData.Clients))
	}
}

func TestHandler_ListClients_OnlineOnly(t *testing.T) {
	_, s, engine := newTestHandler()

	// 注册客户端
	_ = s.Register(context.Background(), &core.ClientInfo{ID: "client-1", Name: "test"})

	req := &ListClientsReq{OnlyOnline: true}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/clients", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData core.ClientListRsp
	_ = json.Unmarshal(apiRsp.Data, &rspData)

	if rspData.Total != 1 {
		t.Errorf("Total = %d, want 1", rspData.Total)
	}
}

func TestHandler_ServerInfo(t *testing.T) {
	_, _, engine := newTestHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/server", nil)
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &apiRsp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData ServerInfoRsp
	if err := json.Unmarshal(apiRsp.Data, &rspData); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if rspData.Server == nil {
		t.Error("server should not be nil")
	}
}

func TestHandler_Health(t *testing.T) {
	_, _, engine := newTestHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/health", nil)
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var apiRsp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &apiRsp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if apiRsp.Code != rsp.Success.Code {
		t.Errorf("Code = %d, want %d", apiRsp.Code, rsp.Success.Code)
	}

	var rspData HealthRsp
	if err := json.Unmarshal(apiRsp.Data, &rspData); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if rspData.Status != "ok" {
		t.Errorf("status = %v, want ok", rspData.Status)
	}
}

func TestHandler_RegisterRoutesWithPrefix(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	h := NewHandler(s, nil, nil)
	engine := gin.New()

	// 使用自定义前缀
	h.RegisterRoutesWithPrefix(engine, "/api/v1/hb")

	// 测试路由是否注册成功
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/hb/health", nil)
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_IndividualHandlers(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	h := NewHandler(s, nil, nil)
	engine := gin.New()

	// 单独注册 handler（都用 POST）
	engine.POST("/custom/register", h.Register())
	engine.POST("/custom/health", h.Health())

	// 测试自定义路由
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/custom/health", nil)
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	_, _, engine := newTestHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat/register", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var apiRsp APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &apiRsp)

	// ParamInvalid code
	if apiRsp.Code != rsp.ErrParamInvalid.Code {
		t.Errorf("Code = %d, want %d (ParamInvalid)", apiRsp.Code, rsp.ErrParamInvalid.Code)
	}
}

func TestHandler_Store(t *testing.T) {
	s := store.NewMemoryStore(core.WithAutoCleanup(false))
	defer s.Close()

	h := NewHandler(s, nil, nil)

	if h.Store() != s {
		t.Error("Store() should return the store instance")
	}
}

func TestDefaultHandlerConfig(t *testing.T) {
	cfg := DefaultHandlerConfig()

	if cfg.HeartbeatPeriod != 10*1e9 { // 10 seconds in nanoseconds
		t.Errorf("HeartbeatPeriod = %v, want 10s", cfg.HeartbeatPeriod)
	}
}
