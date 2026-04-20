package itsm

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/app/itsm/engine"
	"metis/internal/handler"
)

type TicketHandler struct {
	svc         *TicketService
	timelineSvc *TimelineService
}

func NewTicketHandler(i do.Injector) (*TicketHandler, error) {
	svc := do.MustInvoke[*TicketService](i)
	timelineSvc := do.MustInvoke[*TimelineService](i)
	return &TicketHandler{svc: svc, timelineSvc: timelineSvc}, nil
}

type CreateTicketRequest struct {
	Title       string    `json:"title" binding:"required,max=256"`
	Description string    `json:"description"`
	ServiceID   uint      `json:"serviceId" binding:"required"`
	PriorityID  uint      `json:"priorityId" binding:"required"`
	FormData    JSONField `json:"formData"`
}

func (h *TicketHandler) Create(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	requesterID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.create")
	c.Set("audit_resource", "ticket")

	ticket, err := h.svc.Create(CreateTicketInput{
		Title:       req.Title,
		Description: req.Description,
		ServiceID:   req.ServiceID,
		PriorityID:  req.PriorityID,
		FormData:    req.FormData,
	}, requesterID)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceDefNotFound):
			handler.Fail(c, http.StatusBadRequest, "service not found")
		case errors.Is(err, ErrServiceNotActive):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrPriorityNotFound):
			handler.Fail(c, http.StatusBadRequest, "priority not found")
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(ticket.ID)))
	c.Set("audit_summary", "created ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := TicketListParams{
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}

	if v := c.Query("priorityId"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			uid := uint(id)
			params.PriorityID = &uid
		}
	}
	if v := c.Query("serviceId"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			uid := uint(id)
			params.ServiceID = &uid
		}
	}
	if v := c.Query("assigneeId"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			uid := uint(id)
			params.AssigneeID = &uid
		}
	}
	if v := c.Query("requesterId"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			uid := uint(id)
			params.RequesterID = &uid
		}
	}

	items, total, err := h.svc.List(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]TicketResponse, len(items))
	for i, t := range items {
		result[i] = t.ToResponse()
	}
	handler.OK(c, gin.H{"items": result, "total": total})
}

func (h *TicketHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	ticket, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrTicketNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, ticket.ToResponse())
}

type AssignTicketRequest struct {
	AssigneeID uint `json:"assigneeId" binding:"required"`
}

func (h *TicketHandler) Assign(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.assign")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Assign(id, req.AssigneeID, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "assigned ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) Complete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.complete")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Complete(id, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "completed ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) Cancel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req CancelTicketInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.cancel")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Cancel(id, req.Reason, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "cancelled ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

type WithdrawTicketInput struct {
	Reason string `json:"reason"`
}

func (h *TicketHandler) Withdraw(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req WithdrawTicketInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.withdraw")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Withdraw(id, req.Reason, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotRequester):
			handler.Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrTicketClaimed):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "withdrew ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) Mine(c *gin.Context) {
	userID, _ := c.Get("userId")
	requesterID := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := c.Query("status")

	items, total, err := h.svc.Mine(requesterID, status, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]TicketResponse, len(items))
	for i, t := range items {
		result[i] = t.ToResponse()
	}
	handler.OK(c, gin.H{"items": result, "total": total})
}

func (h *TicketHandler) Todo(c *gin.Context) {
	userID, _ := c.Get("userId")
	uid := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	keyword := c.Query("keyword")
	status := c.Query("status")

	items, total, err := h.svc.Todo(uid, keyword, status, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]TicketResponse, len(items))
	for i, t := range items {
		result[i] = t.ToResponse()
	}
	handler.OK(c, gin.H{"items": result, "total": total})
}

func (h *TicketHandler) History(c *gin.Context) {
	userID, _ := c.Get("userId")
	uid := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := HistoryListParams{
		UserID:   &uid,
		Page:     page,
		PageSize: pageSize,
	}

	if v := c.Query("assigneeId"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			uid := uint(id)
			params.AssigneeID = &uid
		}
	}
	if v := c.Query("startDate"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			params.StartDate = &t
		}
	}
	if v := c.Query("endDate"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			params.EndDate = &end
		}
	}

	items, total, err := h.svc.History(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]TicketResponse, len(items))
	for i, t := range items {
		result[i] = t.ToResponse()
	}
	handler.OK(c, gin.H{"items": result, "total": total})
}

func (h *TicketHandler) Timeline(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	items, err := h.timelineSvc.ListByTicket(id)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]TicketTimelineResponse, len(items))
	for i, t := range items {
		result[i] = t.ToResponse()
	}
	handler.OK(c, result)
}

type ProgressTicketRequest struct {
	ActivityID uint            `json:"activityId" binding:"required"`
	Outcome    string          `json:"outcome" binding:"required"`
	Result     json.RawMessage `json:"result"`
}

func (h *TicketHandler) Progress(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req ProgressTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.progress")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Progress(id, req.ActivityID, req.Outcome, req.Result, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, engine.ErrActivityNotFound), errors.Is(err, engine.ErrActivityNotActive):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "progressed ticket: "+ticket.Code+" outcome="+req.Outcome)
	handler.OK(c, ticket.ToResponse())
}

type SignalTicketRequest struct {
	ActivityID uint            `json:"activityId" binding:"required"`
	Outcome    string          `json:"outcome" binding:"required"`
	Data       json.RawMessage `json:"data"`
}

func (h *TicketHandler) Signal(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req SignalTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.signal")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Signal(id, req.ActivityID, req.Outcome, req.Data, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrActivityNotWait), errors.Is(err, engine.ErrActivityNotActive):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "signalled ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) Activities(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	activities, err := h.svc.GetActivities(id, operatorID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, activities)
}

// --- Smart engine override handlers ---

func (h *TicketHandler) ConfirmActivity(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	activityID, err := strconv.ParseUint(c.Param("aid"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid activity id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.confirm_activity")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.ConfirmActivity(id, uint(activityID), operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, engine.ErrActivityNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNotApprover):
			handler.Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrActivityAlready):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "confirmed AI decision for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

type RejectActivityRequest struct {
	Reason string `json:"reason"`
}

func (h *TicketHandler) RejectActivity(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	activityID, err := strconv.ParseUint(c.Param("aid"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid activity id")
		return
	}

	var req RejectActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.reject_activity")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.RejectActivity(id, uint(activityID), req.Reason, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, engine.ErrActivityNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNotApprover):
			handler.Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrActivityAlready):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "rejected AI decision for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

type OverrideJumpRequest struct {
	ActivityType string `json:"activityType" binding:"required"`
	AssigneeID   *uint  `json:"assigneeId"`
	Reason       string `json:"reason" binding:"required"`
}

func (h *TicketHandler) OverrideJump(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req OverrideJumpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.override_jump")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.OverrideJump(id, req.ActivityType, req.AssigneeID, req.Reason, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "override jump for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

type OverrideReassignRequest struct {
	ActivityID    uint   `json:"activityId" binding:"required"`
	NewAssigneeID uint   `json:"newAssigneeId" binding:"required"`
	Reason        string `json:"reason" binding:"required"`
}

func (h *TicketHandler) OverrideReassign(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req OverrideReassignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.override_reassign")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.OverrideReassign(id, req.ActivityID, req.NewAssigneeID, req.Reason, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "override reassign for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

func (h *TicketHandler) RetryAI(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.retry_ai")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.RetryAI(id, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "retry AI for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// --- Approval handlers ---

func (h *TicketHandler) Approvals(c *gin.Context) {
	userID, _ := c.Get("userId")
	uid := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	items, total, err := h.svc.Approvals(uid, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *TicketHandler) ApprovalCount(c *gin.Context) {
	userID, _ := c.Get("userId")
	uid := userID.(uint)

	count, err := h.svc.ApprovalCount(uid)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"count": count})
}

type ApproveActivityRequest struct{}

func (h *TicketHandler) ApproveActivity(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	activityID, err := strconv.ParseUint(c.Param("aid"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid activity id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.approve_activity")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.ApproveActivity(id, uint(activityID), operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, engine.ErrActivityNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNotApprover):
			handler.Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrActivityAlready):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "approved activity for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

type DenyActivityRequest struct {
	Reason string `json:"reason"`
}

func (h *TicketHandler) DenyActivity(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	activityID, err := strconv.ParseUint(c.Param("aid"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid activity id")
		return
	}

	var req DenyActivityRequest
	_ = c.ShouldBindJSON(&req) // reason is optional

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.deny_activity")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.DenyActivity(id, uint(activityID), operatorID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, engine.ErrActivityNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNotApprover):
			handler.Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrActivityAlready):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "denied activity for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// SLAPause handles PUT /api/v1/itsm/tickets/:id/sla/pause
func (h *TicketHandler) SLAPause(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.sla_pause")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.SLAPause(id, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrSLAAlreadyPaused):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "paused SLA for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// SLAResume handles PUT /api/v1/itsm/tickets/:id/sla/resume
func (h *TicketHandler) SLAResume(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.sla_resume")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.SLAResume(id, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrSLANotPaused):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "resumed SLA for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// Transfer handles POST /api/v1/itsm/tickets/:id/transfer
func (h *TicketHandler) Transfer(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req TransferInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.transfer")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Transfer(id, req.ActivityID, req.TargetUserID, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrNoActiveAssignment):
			handler.Fail(c, http.StatusForbidden, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "transferred task for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// Delegate handles POST /api/v1/itsm/tickets/:id/delegate
func (h *TicketHandler) Delegate(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req DelegateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.delegate")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Delegate(id, req.ActivityID, req.TargetUserID, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrNoActiveAssignment):
			handler.Fail(c, http.StatusForbidden, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "delegated task for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}

// Claim handles POST /api/v1/itsm/tickets/:id/claim
func (h *TicketHandler) Claim(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req ClaimInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")
	operatorID := userID.(uint)

	c.Set("audit_action", "itsm.ticket.claim")
	c.Set("audit_resource", "ticket")
	c.Set("audit_resource_id", c.Param("id"))

	ticket, err := h.svc.Claim(id, req.ActivityID, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTicketNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTicketTerminal):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrNoActiveAssignment):
			handler.Fail(c, http.StatusForbidden, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "claimed task for ticket: "+ticket.Code)
	handler.OK(c, ticket.ToResponse())
}
