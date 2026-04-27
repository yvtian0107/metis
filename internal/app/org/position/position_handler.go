package position

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/app/org/domain"
	"metis/internal/handler"
)

type CreatePositionRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=64"`
	Description string `json:"description" binding:"max=255"`
}

type UpdatePositionRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=128"`
	Code        *string `json:"code" binding:"omitempty,max=64"`
	Description *string `json:"description" binding:"omitempty,max=255"`
	IsActive    *bool   `json:"isActive"`
}

type PositionHandler struct {
	svc *PositionService
}

func NewPositionHandler(i do.Injector) (*PositionHandler, error) {
	svc := do.MustInvoke[*PositionService](i)
	return &PositionHandler{svc: svc}, nil
}

func (h *PositionHandler) Create(c *gin.Context) {
	var req CreatePositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.position.create")
	c.Set("audit_resource", "position")

	pos, err := h.svc.Create(req.Name, req.Code, req.Description)
	if err != nil {
		if errors.Is(err, ErrPositionCodeExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(pos.ID)))
	c.Set("audit_summary", "created position: "+pos.Name)
	handler.OK(c, pos.ToResponse())
}

func (h *PositionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	params := PositionListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.svc.ListWithUsage(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *PositionHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	pos, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrPositionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, pos.ToResponse())
}

func (h *PositionHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdatePositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.position.update")
	c.Set("audit_resource", "position")
	c.Set("audit_resource_id", c.Param("id"))

	pos, err := h.svc.Update(id, req.Name, req.Code, req.Description, req.IsActive)
	if err != nil {
		if errors.Is(err, ErrPositionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrPositionCodeExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated position: "+pos.Name)
	handler.OK(c, pos.ToResponse())
}

func (h *PositionHandler) Delete(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "org.position.delete")
	c.Set("audit_resource", "position")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrPositionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrPositionInUse) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted position")
	handler.OK(c, nil)
}
