package department

import (
	"errors"
	"metis/internal/app/org/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type CreateDepartmentRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=64"`
	ParentID    *uint  `json:"parentId"`
	ManagerID   *uint  `json:"managerId"`
	Sort        int    `json:"sort"`
	Description string `json:"description" binding:"max=255"`
}

type UpdateDepartmentRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=128"`
	Code        *string `json:"code" binding:"omitempty,max=64"`
	ParentID    *uint   `json:"parentId"`
	ManagerID   *uint   `json:"managerId"`
	Sort        *int    `json:"sort"`
	Description *string `json:"description" binding:"omitempty,max=255"`
	IsActive    *bool   `json:"isActive"`
}

type DepartmentHandler struct {
	svc *DepartmentService
}

func NewDepartmentHandler(i do.Injector) (*DepartmentHandler, error) {
	svc := do.MustInvoke[*DepartmentService](i)
	return &DepartmentHandler{svc: svc}, nil
}

func (h *DepartmentHandler) Create(c *gin.Context) {
	var req CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.department.create")
	c.Set("audit_resource", "department")

	dept, err := h.svc.Create(req.Name, req.Code, req.ParentID, req.ManagerID, req.Sort, req.Description)
	if err != nil {
		if errors.Is(err, ErrDepartmentCodeExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(dept.ID)))
	c.Set("audit_summary", "created department: "+dept.Name)
	handler.OK(c, dept.ToResponse())
}

func (h *DepartmentHandler) List(c *gin.Context) {
	items, err := h.svc.ListAll()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.DepartmentResponse, len(items))
	for i, d := range items {
		result[i] = d.ToResponse()
	}
	handler.OK(c, gin.H{"items": result})
}

func (h *DepartmentHandler) Tree(c *gin.Context) {
	tree, err := h.svc.Tree()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, gin.H{"items": tree})
}

func (h *DepartmentHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	dept, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, dept.ToResponse())
}

func (h *DepartmentHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.department.update")
	c.Set("audit_resource", "department")
	c.Set("audit_resource_id", c.Param("id"))

	dept, err := h.svc.Update(id, UpdateDepartmentInput{
		Name:        req.Name,
		Code:        req.Code,
		ParentID:    req.ParentID,
		ManagerID:   req.ManagerID,
		Sort:        req.Sort,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrDepartmentCodeExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated department: "+dept.Name)
	handler.OK(c, dept.ToResponse())
}

func (h *DepartmentHandler) Delete(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "org.department.delete")
	c.Set("audit_resource", "department")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		switch {
		case errors.Is(err, ErrDepartmentNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrDepartmentHasChildren), errors.Is(err, ErrDepartmentHasMembers):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "deleted department")
	handler.OK(c, nil)
}

type SetAllowedPositionsRequest struct {
	PositionIDs []uint `json:"positionIds"`
}

func (h *DepartmentHandler) GetAllowedPositions(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	positions, err := h.svc.GetAllowedPositions(id)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, gin.H{"items": positions})
}

func (h *DepartmentHandler) SetAllowedPositions(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req SetAllowedPositionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.department.setPositions")
	c.Set("audit_resource", "department")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.SetAllowedPositions(id, req.PositionIDs); err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "set allowed positions for department")
	handler.OK(c, nil)
}
