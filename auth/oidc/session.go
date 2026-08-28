package oidc

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	authoauth2 "github.com/mulan-go/mulan/auth/oauth2"
	"github.com/mulan-go/mulan/auth/session"
)

type SessionMapper func(*gin.Context, *Identity, *session.Session) error

func (a *Authenticator) SessionCallback(mapper ...SessionMapper) gin.HandlerFunc {
	selected := DefaultSessionMapper
	if len(mapper) > 0 && mapper[0] != nil {
		selected = mapper[0]
	}
	return a.CallbackHandler(func(c *gin.Context, identity *Identity) {
		sess := session.Default(c)
		sess.Data().Clear()
		if err := sess.Clear(); err != nil {
			abort(c, fmt.Errorf("oidc: rotate session: %w", err))
			return
		}
		if err := selected(c, identity, sess); err != nil {
			abort(c, err)
			return
		}
		if err := sess.Save(); err != nil {
			abort(c, fmt.Errorf("oidc: save session: %w", err))
			return
		}
		finish(c, identity.ReturnTo)
	})
}

func DefaultSessionMapper(_ *gin.Context, identity *Identity, sess *session.Session) error {
	if identity == nil || len(identity.Claims) == 0 {
		return errors.New("oidc: verified ID token claims are required")
	}
	claims := make(map[string]any, len(identity.Claims)+len(identity.UserInfo))
	for key, value := range identity.Claims {
		claims[key] = value
	}
	for key, value := range identity.UserInfo {
		if _, exists := claims[key]; !exists {
			claims[key] = value
		}
	}
	return authoauth2.ApplyClaims(sess, claims, "oidc")
}
