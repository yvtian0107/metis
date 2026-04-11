package middleware

import (
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

// Whitelisted paths that skip Casbin permission checking.
// These are either public or only require authentication (not fine-grained permissions).
var casbinWhitelist = map[string]bool{
	"/api/v1/auth/login":    true,
	"/api/v1/auth/refresh":  true,
	"/api/v1/auth/logout":   true,
	"/api/v1/auth/me":       true,
	"/api/v1/auth/password": true,
	"/api/v1/auth/profile":  true,
}

var casbinWhitelistPrefixes = []string{
	"/api/v1/site-info",
	"/api/v1/menus/user-tree",
	"/api/v1/notifications",
	"/api/v1/auth/connections",
	"/api/v1/auth/sso",
	"/api/v1/auth/check-domain",
	"/api/v1/auth/2fa",
}

// CasbinAuth returns a Gin middleware that checks permissions via Casbin enforcer.
func CasbinAuth(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// Skip whitelisted routes
		if casbinWhitelist[path] {
			c.Next()
			return
		}
		for _, prefix := range casbinWhitelistPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// Get role from JWT context
		userRole, _ := c.Get("userRole")
		roleCode, _ := userRole.(string)
		if roleCode == "" {
			abortJSON(c, http.StatusForbidden, "forbidden: insufficient permission")
			return
		}

		// Check with Casbin
		allowed, err := enforcer.Enforce(roleCode, path, method)
		if err != nil {
			abortJSON(c, http.StatusInternalServerError, "permission check failed")
			return
		}

		if !allowed {
			abortJSON(c, http.StatusForbidden, "forbidden: insufficient permission")
			return
		}

		c.Next()
	}
}
