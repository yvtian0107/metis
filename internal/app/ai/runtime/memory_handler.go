package runtime

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type MemoryHandler struct {
	svc *MemoryService
}

func NewMemoryHandler(i do.Injector) (*MemoryHandler, error) {
	return &MemoryHandler{
		svc: do.MustInvoke[*MemoryService](i),
	}, nil
}

func (h *MemoryHandler) List(c *gin.Context) {
	agentID, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	memories, err := h.svc.List(uint(agentID), userID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]AgentMemoryResponse, len(memories))
	for i, m := range memories {
		items[i] = m.ToResponse()
	}
	handler.OK(c, items)
}

type createMemoryReq struct {
	Key     string `json:"key" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func (h *MemoryHandler) Create(c *gin.Context) {
	agentID, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createMemoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	m := &AgentMemory{
		AgentID: uint(agentID),
		UserID:  userID,
		Key:     req.Key,
		Content: req.Content,
		Source:  MemorySourceUserSet,
	}
	if err := h.svc.Upsert(m); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, m.ToResponse())
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	agentID, _ := strconv.Atoi(c.Param("id"))
	mid, _ := strconv.Atoi(c.Param("mid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.DeleteForAgentUser(uint(mid), uint(agentID), userID); err != nil {
		if errors.Is(err, ErrMemoryNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, nil)
}
