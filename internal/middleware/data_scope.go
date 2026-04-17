package middleware

import (
	"github.com/gin-gonic/gin"

	"metis/internal/app"
	"metis/internal/model"
)

// RoleScopeGetter is a function that retrieves a role's DataScope and (for custom type)
// the explicitly configured department IDs.
type RoleScopeGetter func(roleCode string) (model.DataScope, []uint, error)

// DataScopeMiddleware resolves the current user's data-visibility scope and injects
// it into the Gin context as "deptScope" (*[]uint):
//
//   - nil  → DataScopeAll or resolver absent — no department filtering
//   - &[]uint{}  → DataScopeSelf — only records owned by the user
//   - &[]uint{1,2,3} → explicit department IDs to filter by
//
// The middleware is nil-safe: if resolver is nil (Org App not installed) the
// middleware always sets deptScope = nil regardless of the role's DataScope field.
func DataScopeMiddleware(resolver app.OrgResolver, getScopeForRole RoleScopeGetter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, _ := c.Get("userId")
		userID, _ := userIDVal.(uint)

		userRoleVal, _ := c.Get("userRole")
		roleCode, _ := userRoleVal.(string)

		// No resolver means Org App not installed → all data visible
		if resolver == nil || roleCode == "" || userID == 0 {
			c.Set("deptScope", (*[]uint)(nil))
			c.Next()
			return
		}

		scope, customDeptIDs, err := getScopeForRole(roleCode)
		if err != nil {
			// Graceful degradation: treat as ALL scope on lookup failure
			c.Set("deptScope", (*[]uint)(nil))
			c.Next()
			return
		}

		switch scope {
		case model.DataScopeAll:
			c.Set("deptScope", (*[]uint)(nil))

		case model.DataScopeSelf:
			empty := []uint{}
			c.Set("deptScope", &empty)

		case model.DataScopeDept:
			deptIDs, err := resolver.GetUserDeptScope(userID, false)
			if err != nil || len(deptIDs) == 0 {
				empty := []uint{}
				c.Set("deptScope", &empty)
			} else {
				c.Set("deptScope", &deptIDs)
			}

		case model.DataScopeDeptAndSub:
			deptIDs, err := resolver.GetUserDeptScope(userID, true)
			if err != nil || len(deptIDs) == 0 {
				empty := []uint{}
				c.Set("deptScope", &empty)
			} else {
				c.Set("deptScope", &deptIDs)
			}

		case model.DataScopeCustom:
			if len(customDeptIDs) == 0 {
				empty := []uint{}
				c.Set("deptScope", &empty)
			} else {
				c.Set("deptScope", &customDeptIDs)
			}

		default:
			c.Set("deptScope", (*[]uint)(nil))
		}

		c.Next()
	}
}
