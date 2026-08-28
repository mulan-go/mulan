package rsp

import (
	"fmt"
	"sync"
)

var (
	Errors = map[int]*AppError{}
	lock   sync.Mutex
)

var _ error = (*AppError)(nil)

type AppError struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

func (a *AppError) Error() string { return fmt.Sprintf("%d: %s", a.Code, a.Msg) }

func (a *AppError) WithMsg(msg string) *AppError {
	return &AppError{Code: a.Code, Msg: a.Msg + " : " + msg}
}
func (a *AppError) WithError(err error) *AppError {
	return &AppError{Code: a.Code, Msg: a.Msg + " : " + err.Error()}
}

func New(system SystemCode, biz BizCode, code int, msg string) *AppError {
	return newAppError(code+int(biz)*1000+int(system)*1000*100, msg)
}

func newAppError(code int, msg string) *AppError {
	c := &AppError{Code: code, Msg: msg}
	lock.Lock()
	if _c, ok := Errors[code]; ok {
		panic(fmt.Sprintf("code %d is duplicate, found: %+v", code, _c))
	}
	Errors[code] = c
	lock.Unlock()
	return c
}
