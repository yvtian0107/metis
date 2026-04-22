package itsm

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/handler"
)

type ServiceDeskHandler struct {
	db         *gorm.DB
	stateStore *tools.SessionStateStore
}

func NewServiceDeskHandler(i do.Injector) (*ServiceDeskHandler, error) {
	db := do.MustInvoke[*database.DB](i)
	stateStore := do.MustInvoke[*tools.SessionStateStore](i)
	return &ServiceDeskHandler{db: db.DB, stateStore: stateStore}, nil
}

func (h *ServiceDeskHandler) State(c *gin.Context) {
	sid, err := strconv.Atoi(c.Param("sid"))
	if err != nil || sid <= 0 {
		handler.Fail(c, http.StatusBadRequest, "invalid session id")
		return
	}

	userID := c.GetUint("userId")
	var row struct {
		ID uint
	}
	if err := h.db.Table("ai_agent_sessions AS s").
		Joins("JOIN ai_agents AS a ON a.id = s.agent_id").
		Where("s.id = ? AND s.user_id = ? AND a.code = ?", sid, userID, "itsm.servicedesk").
		Select("s.id").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handler.Fail(c, http.StatusNotFound, "session not found")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	state, err := h.stateStore.GetState(uint(sid))
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, gin.H{
		"state":              state,
		"nextExpectedAction": tools.NextExpectedAction(state),
	})
}
