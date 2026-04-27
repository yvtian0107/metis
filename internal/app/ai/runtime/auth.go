package runtime

import "github.com/gin-gonic/gin"

func requireUserID(c *gin.Context) (uint, bool) {
	userID, ok := c.Get("userId")
	if !ok {
		return 0, false
	}
	id, ok := userID.(uint)
	if !ok {
		return 0, false
	}
	return id, true
}
