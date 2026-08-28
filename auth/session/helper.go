package session

import (
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// HasRole 检查用户是否拥有指定角色
func HasRole(c *gin.Context, role string) bool {
	return slices.Contains(c.GetStringSlice(CtxKeyRoles), role)
}

// HasRoles 检查用户是否拥有所有指定角色
func HasRoles(c *gin.Context, roles ...string) bool {
	if len(roles) == 0 {
		return true
	}
	_roles := c.GetStringSlice(CtxKeyRoles)
	roleSet := make(map[string]struct{}, len(_roles))
	for _, role := range _roles {
		roleSet[role] = struct{}{}
	}
	for _, role := range roles {
		if _, exists := roleSet[role]; !exists {
			return false
		}
	}
	return true
}

// populateContext 将session数据填充到gin.Context
func populateContext(c *gin.Context, data Data) {
	roles := data.Roles()
	c.Set(CtxKeyID, data.ID())
	c.Set(CtxKeyAccount, data.Account())
	c.Set(CtxKeyState, data.State())
	c.Set(CtxKeyRoles, roles)
	c.Set(CtxKeyIsAdmin, slices.Contains(roles, RoleAdmin))
	for k, v := range data.Items() {
		c.Set(k, v)
	}
}

// extractToken 从请求中提取token（按优先级）
func extractToken(c *gin.Context, name string, allowCookie bool) string {
	// 1. 尝试从自定义Header获取
	headerKeys := []string{"X-Token", "X-Api-Key", name, "X-" + name}
	for _, key := range headerKeys {
		if token := strings.TrimSpace(c.GetHeader(key)); token != "" {
			return token
		}
	}
	// 2. 尝试从Authorization Header获取
	if auth := c.GetHeader("Authorization"); auth != "" {
		if token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")); token != "" {
			return token
		}
	}
	if allowCookie {
		// 3. 尝试从Cookie获取
		if token, err := c.Cookie(name); err == nil {
			if token = strings.TrimSpace(token); token != "" {
				return token
			}
		}
	}
	return ""
}
