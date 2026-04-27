package desk

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	. "metis/internal/app/itsm/config"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/handler"
)

const serviceDeskSurfacePersistTimeout = 2 * time.Second

type serviceDeskMessageStore interface {
	StoreMessageContext(ctx context.Context, sessionID uint, role, content string, metadata []byte, tokenCount int) (*ai.SessionMessage, error)
}

type ServiceDeskHandler struct {
	db             *gorm.DB
	configProvider *EngineConfigService
	stateStore     *tools.SessionStateStore
	operator       *tools.Operator
	sessionSvc     serviceDeskMessageStore
}

func NewServiceDeskHandler(i do.Injector) (*ServiceDeskHandler, error) {
	db := do.MustInvoke[*database.DB](i)
	configProvider := do.MustInvoke[*EngineConfigService](i)
	stateStore := do.MustInvoke[*tools.SessionStateStore](i)
	operator := do.MustInvoke[*tools.Operator](i)
	sessionSvc := do.MustInvoke[*ai.SessionService](i)
	return &ServiceDeskHandler{db: db.DB, configProvider: configProvider, stateStore: stateStore, operator: operator, sessionSvc: sessionSvc}, nil
}

func (h *ServiceDeskHandler) verifyServiceDeskSession(c *gin.Context) (uint, uint, bool) {
	sid, err := strconv.Atoi(c.Param("sid"))
	if err != nil || sid <= 0 {
		handler.Fail(c, http.StatusBadRequest, "invalid session id")
		return 0, 0, false
	}

	userID := c.GetUint("userId")
	intakeAgentID := h.configProvider.IntakeAgentID()
	if intakeAgentID == 0 {
		handler.Fail(c, http.StatusBadRequest, "服务受理岗未上岗")
		return 0, 0, false
	}
	var row struct {
		ID uint
	}
	if err := h.db.Table("ai_agent_sessions AS s").
		Where("s.id = ? AND s.user_id = ? AND s.agent_id = ?", sid, userID, intakeAgentID).
		Select("s.id").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handler.Fail(c, http.StatusNotFound, "session not found")
			return 0, 0, false
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return 0, 0, false
	}
	return uint(sid), userID, true
}

func (h *ServiceDeskHandler) State(c *gin.Context) {
	sid, _, ok := h.verifyServiceDeskSession(c)
	if !ok {
		return
	}

	state, err := h.stateStore.GetState(sid)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, gin.H{
		"state":              state,
		"nextExpectedAction": tools.NextExpectedAction(state),
	})
}

func (h *ServiceDeskHandler) SubmitDraft(c *gin.Context) {
	sid, userID, ok := h.verifyServiceDeskSession(c)
	if !ok {
		return
	}

	var req tools.DraftSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := tools.SubmitDraft(h.operator, h.stateStore, sid, userID, req)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var submittedSurfaceMeta []byte
	if result.Surface != nil {
		submittedSurfaceMeta, _ = json.Marshal(map[string]any{"ui_surface": result.Surface})
	}

	handler.OK(c, result)
	h.persistSubmittedSurface(sid, submittedSurfaceMeta)
}

func (h *ServiceDeskHandler) persistSubmittedSurface(sessionID uint, metadata []byte) {
	store := h.sessionSvc
	if store == nil || len(metadata) == 0 {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), serviceDeskSurfacePersistTimeout)
		defer cancel()
		if _, err := store.StoreMessageContext(ctx, sessionID, ai.MessageRoleAssistant, "", metadata, 0); err != nil {
			slog.Warn("persist service desk submitted surface failed", "sessionID", sessionID, "error", err)
		}
	}()
}
