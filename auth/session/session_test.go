package session_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mulan-go/mulan/auth/session"
	"github.com/mulan-go/mulan/rdb"
)

func handlerInfo(c *gin.Context) {
	sess := session.Default(c)
	v := sess.Data()
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		c.String(500, "Marshal"+err.Error())
		return
	}
	err = json.Unmarshal(buf, v)
	if err != nil {
		c.String(500, "Unmarshal"+err.Error())
		return
	}
	c.JSON(200, v)
}

func handlerLogin(c *gin.Context) {
	v := session.Default(c)
	v.SetID(1)
	v.SetAccount("test")
	v.SetRoles([]string{"admin"})
	v.SetValues("aaaa", "aaaaa")
	v.SetValues("vvvv", "asdveasd")
	v.Save()
	c.String(200, v.Token())
}

func setupRouter(cfg *session.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mw, err := session.Init(cfg)
	if err != nil {
		panic(err)
	}
	r.Use(mw)
	r.GET("/login", handlerLogin)
	r.GET("/info", session.AuthMW(), handlerInfo)
	return r
}

func TestTokenMem(t *testing.T) {
	r := setupRouter(&session.Config{})

	// 构建返回值
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w1, req1)
	rsp1 := w1.Result()
	body, _ := io.ReadAll(rsp1.Body)
	tokenStr := string(body)
	t.Log(tokenStr)

	req2, _ := http.NewRequest("GET", "/info", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenStr)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	rsp2 := w2.Result()
	body, _ = io.ReadAll(rsp2.Body)
	t.Log(string(body))
}

func TestTokenRedis(t *testing.T) {
	t.Skip()
	r := setupRouter(&session.Config{Driver: "rdb", RDB: rdb.Config{
		Host: "localhost",
		Port: 6379,
	}})

	// 构建返回值
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w1, req1)
	rsp1 := w1.Result()
	body, _ := io.ReadAll(rsp1.Body)
	tokenStr := string(body)
	t.Log(tokenStr)

	req2, _ := http.NewRequest("GET", "/info", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenStr)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	rsp2 := w2.Result()
	body, _ = io.ReadAll(rsp2.Body)
	t.Log(string(body))
}
