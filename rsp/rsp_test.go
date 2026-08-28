package rsp_test

import (
	"encoding/json"
	"testing"

	"github.com/mulan-go/mulan/rsp"
)

func jsonString(v any) string {
	buf, _ := json.Marshal(v)
	return string(buf)
}
func TestMsg(t *testing.T) {
	t.Log(jsonString(rsp.M("test")))
	t.Log(jsonString(rsp.OK()))
	t.Log(jsonString(rsp.M("aaaaaaaaaaaaaaaa")))
	t.Log(jsonString(rsp.OK()))
	t.Log(jsonString(rsp.E(rsp.DatabaseUnknown)))
	t.Log(jsonString(rsp.OK()))
}
