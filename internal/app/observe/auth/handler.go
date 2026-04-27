package auth

import (
	"errors"
	"fmt"
	. "metis/internal/app/observe/token"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

type AuthHandler struct {
	svc *IntegrationTokenService
}

func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	return &AuthHandler{
		svc: do.MustInvoke[*IntegrationTokenService](i),
	}, nil
}

// Verify GET /api/v1/observe/auth/verify
// Used by Traefik ForwardAuth. Returns 200 with identity headers on success, 401 on failure.
func (h *AuthHandler) Verify(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		c.Status(http.StatusUnauthorized)
		return
	}

	raw := strings.TrimPrefix(auth, "Bearer ")
	result, err := h.svc.Verify(raw)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusUnauthorized)
		return
	}

	c.Header("X-Metis-User-Id", fmt.Sprintf("%d", result.UserID))
	c.Header("X-Metis-Token-Id", fmt.Sprintf("%d", result.TokenID))
	c.Header("X-Metis-Scope", result.Scope)
	c.Header("X-Metis-Org-Id", "")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
