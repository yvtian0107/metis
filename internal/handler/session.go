package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"metis/internal/service"
)

type SessionHandler struct {
	sessionSvc *service.SessionService
}

func (h *SessionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	currentJTI, _ := c.Get("tokenJTI")
	jti, _ := currentJTI.(string)

	result, err := h.sessionSvc.ListSessions(page, pageSize, jti)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, gin.H{
		"items":    result.Items,
		"total":    result.Total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *SessionHandler) Kick(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Fail(c, http.StatusBadRequest, "invalid session id")
		return
	}

	currentJTI, _ := c.Get("tokenJTI")
	jti, _ := currentJTI.(string)

	if err := h.sessionSvc.KickSession(uint(id), jti); err != nil {
		switch {
		case errors.Is(err, service.ErrSessionNotFound):
			Fail(c, http.StatusNotFound, "session not found")
		case errors.Is(err, service.ErrCannotKickSelf):
			Fail(c, http.StatusBadRequest, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "session.kick")
	c.Set("audit_resource", "session")
	c.Set("audit_resource_id", strconv.FormatUint(id, 10))
	c.Set("audit_summary", "踢出会话")
	OK(c, nil)
}
