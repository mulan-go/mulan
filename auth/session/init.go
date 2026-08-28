package session

import (
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/rdb"
)

const (
	DefaultKey = "github.com/mulan-go/mulan/auth"
	TokenKey   = "github.com/mulan-go/mulan/auth/session"

	CtxKeyID      = "id"
	CtxKeyAccount = "account"
	CtxKeyState   = "state"
	CtxKeyRoles   = "roles"
	CtxKeyIsAdmin = "is_admin"
	RoleAdmin     = "admin"
)

var tokenValid = regexp.MustCompile(`^[a-f0-9]{40}$`)

// Default 获取当前请求的Session
func Default(c *gin.Context) *Session {
	if value, exists := c.Get(DefaultKey); exists {
		if sess, ok := value.(*Session); ok {
			return sess
		}
	}
	panic("Session does not init or type mismatch")
}

// Init 初始化Session中间件
func Init(cfg *Config) (gin.HandlerFunc, error) {
	var mw gin.HandlerFunc

	name := cfg.Name
	if name == "" {
		name = "token"
	}

	// 初始化 Auth Token 中间件
	switch cfg.Driver {
	// 使用 Redis 作为存储
	case "rdb":
		// 初始化连接 Redis
		client, err := rdb.New(&cfg.RDB)
		if err != nil {
			return nil, err
		}
		store, err := NewRedisStore(client, cfg.TTL)
		if err != nil {
			return nil, err
		}
		mw = newMiddleware(name, store, cfg.HeaderOnly)
	// 使用文件系统作为存储
	case "fs":
		store, err := NewFsStore(cfg.Dir, cfg.TTL)
		if err != nil {
			return nil, err
		}
		mw = newMiddleware(name, store, cfg.HeaderOnly)
	// 默认使用内存存储
	default:
		store := NewMemStore(cfg.TTL)
		mw = newMiddleware(name, store, cfg.HeaderOnly)
	}
	return mw, nil
}

func Mw(name string, store Store, data ...Data) gin.HandlerFunc {
	return newMiddleware(name, store, false, data...)
}

func newMiddleware(name string, store Store, headerOnly bool, data ...Data) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取token
		token := extractToken(c, name, !headerOnly)
		if token != "" && !tokenValid.MatchString(token) {
			token = ""
		}
		// 创建或获取Data实例
		var _data Data
		if len(data) > 0 {
			_data = data[0]
			_data.Clear().SetToken(token)
		} else {
			_data = &DefaultData{Token_: token}
		}
		// 创建Session
		sess := NewSession(c, store, _data)
		c.Set(DefaultKey, sess)
		c.Set(TokenKey, token)

		// 如果Session有效，设置用户信息到Context
		if !sess.IsNil {
			populateContext(c, sess.Data())
		}

		c.Next()

		// 请求处理完后，如果 token 存在（可能是新生成的），设置到 Header 和 Cookie
		if t := sess.Token(); t != "" {
			c.Header("X-Token", t)
			if !headerOnly {
				c.SetCookie(name, t, sess.maxAge, "/", "", sess.secure, sess.httpOnly)
			}
		}
	}
}
