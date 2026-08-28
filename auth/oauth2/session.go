package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mulan-go/mulan/auth/session"
)

type SessionMapper func(*gin.Context, *Identity, *session.Session) error

func (a *Client) SessionCallback(mapper ...SessionMapper) gin.HandlerFunc {
	selected := DefaultSessionMapper
	if len(mapper) > 0 && mapper[0] != nil {
		selected = mapper[0]
	}
	return a.CallbackHandler(func(c *gin.Context, identity *Identity) {
		sess := session.Default(c)
		sess.Data().Clear()
		if err := sess.Clear(); err != nil {
			Abort(c, fmt.Errorf("oauth2: rotate session: %w", err))
			return
		}
		if err := selected(c, identity, sess); err != nil {
			Abort(c, err)
			return
		}
		if err := sess.Save(); err != nil {
			Abort(c, fmt.Errorf("oauth2: save session: %w", err))
			return
		}
		finish(c, identity.ReturnTo)
	})
}

func DefaultSessionMapper(_ *gin.Context, identity *Identity, sess *session.Session) error {
	if identity == nil || len(identity.UserInfo) == 0 {
		return errors.New("oauth2: user info is required for session authentication")
	}
	return ApplyClaims(sess, identity.UserInfo, "oauth2")
}

func ApplyClaims(sess *session.Session, claims map[string]any, namespace string) error {
	if sess == nil {
		return errors.New("oauth2: session is nil")
	}
	subject := firstString(claims, "sub", "id")
	account := firstString(claims, "preferred_username", "login", "username", "email", "name", "sub", "id")
	if subject == "" || account == "" {
		return errors.New("oauth2: user info does not contain a usable subject and account")
	}
	sess.SetID(0)
	if id, ok := firstUint64(claims, "id", "sub"); ok {
		sess.SetID(id)
	}
	sess.SetAccount(account)
	roles := append(claimStrings(claims["roles"]), claimStrings(claims["groups"])...)
	sess.SetRoles(uniqueStrings(roles))
	if namespace == "" {
		namespace = "oauth2"
	}
	sess.SetValues(namespace+"_subject", subject)
	sess.SetValues(namespace+"_claims", cloneClaims(claims))
	return nil
}

func firstString(claims map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := claimString(claims[key]); value != "" {
			return value
		}
	}
	return ""
}

func claimString(value any) string {
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		if value >= 0 && value <= math.MaxUint64 && math.Trunc(value) == value {
			return strconv.FormatUint(uint64(value), 10)
		}
	case uint64:
		return strconv.FormatUint(value, 10)
	case uint:
		return strconv.FormatUint(uint64(value), 10)
	case int:
		if value >= 0 {
			return strconv.FormatUint(uint64(value), 10)
		}
	case int64:
		if value >= 0 {
			return strconv.FormatUint(uint64(value), 10)
		}
	}
	return ""
}

func firstUint64(claims map[string]any, keys ...string) (uint64, bool) {
	for _, key := range keys {
		value := claimString(claims[key])
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func claimStrings(value any) []string {
	var values []string
	switch value := value.(type) {
	case []string:
		values = value
	case []any:
		values = make([]string, 0, len(value))
		for _, item := range value {
			if item := claimString(item); item != "" {
				values = append(values, item)
			}
		}
	case string:
		values = []string{value}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneClaims(claims map[string]any) map[string]any {
	result := make(map[string]any, len(claims))
	for key, value := range claims {
		result[key] = value
	}
	return result
}
