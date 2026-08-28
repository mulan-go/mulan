package core

import (
	"encoding/json"
	"time"
)

// ClientStatus 客户端状态
type ClientStatus int

const (
	StatusUnknown ClientStatus = iota // 未知
	StatusOnline                      // 在线
	StatusOffline                     // 离线
	StatusTimeout                     // 超时
)

func (s ClientStatus) String() string {
	switch s {
	case StatusOnline:
		return "online"
	case StatusOffline:
		return "offline"
	case StatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

func (s ClientStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *ClientStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "online":
		*s = StatusOnline
	case "offline":
		*s = StatusOffline
	case "timeout":
		*s = StatusTimeout
	default:
		*s = StatusUnknown
	}
	return nil
}

// ClientInfo 客户端信息
type ClientInfo struct {
	ID            string            `json:"id"`             // 客户端唯一标识
	Name          string            `json:"name"`           // 客户端名称
	IP            string            `json:"ip"`             // 客户端 IP
	Port          int               `json:"port,omitempty"` // 客户端端口
	Metadata      map[string]string `json:"metadata"`       // 额外元数据
	LastHeartbeat time.Time         `json:"last_heartbeat"` // 最后心跳时间
	Status        ClientStatus      `json:"status"`         // 状态
	RegisteredAt  time.Time         `json:"registered_at"`  // 注册时间
}

// IsExpired 检查是否超时
func (c *ClientInfo) IsExpired(timeout time.Duration) bool {
	return time.Since(c.LastHeartbeat) > timeout
}

// ServerInfo 服务端信息
type ServerInfo struct {
	ID       string            `json:"id"`       // 服务端标识
	Name     string            `json:"name"`     // 服务端名称
	Hostname string            `json:"hostname"` // 服务端主机名
	Metadata map[string]string `json:"metadata"` // 额外元数据
}

// RegisterReq 注册请求
type RegisterReq struct {
	ID       string            `json:"id"`             // 客户端 ID（可选，为空则自动生成）
	Name     string            `json:"name"`           // 客户端名称
	IP       string            `json:"ip"`             // 客户端 IP
	Port     int               `json:"port,omitempty"` // 客户端端口
	Metadata map[string]string `json:"metadata"`       // 额外元数据
}

// RegisterRsp 注册响应
type RegisterRsp struct {
	ClientID string      `json:"client_id"` // 分配的客户端 ID
	Server   *ServerInfo `json:"server"`    // 服务端信息
}

// HeartbeatReq 心跳请求
type HeartbeatReq struct {
	ID       string            `json:"id"`       // 客户端 ID
	Metadata map[string]string `json:"metadata"` // 动态元数据
}

// HeartbeatRsp 心跳响应
type HeartbeatRsp struct {
	OK        bool        `json:"ok"`                 // 是否成功
	Server    *ServerInfo `json:"server"`             // 服务端信息
	Timestamp time.Time   `json:"timestamp"`          // 服务端时间
	Clients   int         `json:"clients,omitempty"`  // 在线客户端数（可选）
	Message   string      `json:"message,omitempty"`  // 消息（可选）
	Interval  int         `json:"interval,omitempty"` // 建议心跳间隔（秒）
}

// UnregisterReq 注销请求
type UnregisterReq struct {
	ID string `json:"id"` // 客户端 ID
}

// ClientListRsp 客户端列表响应
type ClientListRsp struct {
	Total   int           `json:"total"`   // 总数
	Clients []*ClientInfo `json:"clients"` // 客户端列表
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string      `json:"type"`    // 消息类型: register, heartbeat, broadcast, ping, pong
	Payload interface{} `json:"payload"` // 消息内容
}

// 消息类型常量
const (
	MsgTypeRegister   = "register"
	MsgTypeHeartbeat  = "heartbeat"
	MsgTypeBroadcast  = "broadcast"
	MsgTypePing       = "ping"
	MsgTypePong       = "pong"
	MsgTypeDisconnect = "disconnect"
	MsgTypeError      = "error"
)
