package rsp

import (
	"go.uber.org/zap"
)

type IDRsp[T uint64 | string] struct {
	ID   T      `json:"id,omitempty"`
	UUID string `json:"uuid,omitempty"`
}

type Rsp[T any] struct {
	*AppError `json:",inline"`
	Data      T `json:"data"`
}

func S[T any](data T) *Rsp[T] { return &Rsp[T]{AppError: Success, Data: data} }

type Resp = Rsp[any]
type RspNone = Rsp[struct{}]

func UnImplemented() *RspNone  { return &RspNone{AppError: ErrUnImplemented} }
func E(err *AppError) *RspNone { return &RspNone{AppError: err} }
func OK() *RspNone             { return &RspNone{AppError: Success} }
func M(msg string) *RspNone    { return &RspNone{AppError: &AppError{Code: Success.Code, Msg: msg}} }

// C 将错误转换为响应，内部错误信息仅记录日志，不返回给客户端
// 防止敏感的内部错误信息泄露
func C(err error) *RspNone {
	// 仅在日志中记录详细错误，不暴露给客户端
	zap.L().Error("internal error", zap.Error(err))
	return &RspNone{AppError: ErrUnknownErr}
}

// CWithMsg 返回自定义消息的错误响应，同时记录原始错误
// 用于需要给用户友好提示但不暴露内部错误的场景
func CWithMsg(err error, msg string) *RspNone {
	zap.L().Error("internal error", zap.Error(err), zap.String("user_msg", msg))
	return &RspNone{AppError: &AppError{Code: ErrUnknownErr.Code, Msg: msg}}
}
