package req_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/req"
)

type User struct {
	Name string `json:"name" form:"name"`
	Info string `json:"info" form:"info"`
	ID   uint64 `json:"id" form:"id" uri:"id"`
}

func TestBind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/u/:id", func(c *gin.Context) {
		var obj User
		err := req.Bind(c, &obj)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, obj)
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/u/123?info=hello,world", bytes.NewBufferString(`{"name": "test"}`))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	t.Log(w.Body.String())
}

type pageReq struct {
	Page int `form:"page" json:"page" default:"7"`
	Size int `form:"size" json:"size" default:"20"`
}

func TestBindDefaultTag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/list", func(c *gin.Context) {
		var obj pageReq
		if err := req.Bind(c, &obj); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, obj)
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/list", nil)
	router.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var got pageReq
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Page != 7 || got.Size != 20 {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestBindDefaultTagPartialQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/list", func(c *gin.Context) {
		var obj pageReq
		if err := req.Bind(c, &obj); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, obj)
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/list?page=3", nil)
	router.ServeHTTP(w, r)
	var got pageReq
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Page != 3 || got.Size != 20 {
		t.Fatalf("expected Page=3 Size=20, got %+v", got)
	}
}

func BenchmarkBind(b *testing.B) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/u/:id", func(c *gin.Context) {
		var obj User
		req.Bind(c, &obj)
	})
	w := httptest.NewRecorder()
	buf := bytes.NewBufferString(`{"name": "test"}`)
	r := httptest.NewRequest(http.MethodPost, "/u/123?info=test", buf)
	r.Header.Set("Content-Type", "application/json")

	for b.Loop() {
		router.ServeHTTP(w, r)
	}
}
