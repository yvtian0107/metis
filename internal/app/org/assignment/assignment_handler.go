package assignment

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/app/org/department"
	"metis/internal/app/org/domain"
	"metis/internal/app/org/position"
	"metis/internal/handler"
	"metis/internal/service"
)

type AddUserPositionRequest struct {
	DepartmentID uint `json:"departmentId" binding:"required"`
	PositionID   uint `json:"positionId" binding:"required"`
	IsPrimary    bool `json:"isPrimary"`
}

type UpdateAssignmentRequest struct {
	PositionID *uint `json:"positionId"`
	IsPrimary  *bool `json:"isPrimary"`
}

type SetUserDeptPositionsRequest struct {
	PositionIDs       []uint `json:"positionIds" binding:"required"`
	PrimaryPositionID *uint  `json:"primaryPositionId"`
}

type AssignmentHandler struct {
	svc     *AssignmentService
	userSvc *service.UserService
}

func NewAssignmentHandler(i do.Injector) (*AssignmentHandler, error) {
	svc := do.MustInvoke[*AssignmentService](i)
	userSvc := do.MustInvoke[*service.UserService](i)
	return &AssignmentHandler{svc: svc, userSvc: userSvc}, nil
}

func (h *AssignmentHandler) GetUserPositions(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	items, err := h.svc.GetUserPositions(userID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, gin.H{"items": items})
}

func (h *AssignmentHandler) AddUserPosition(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req AddUserPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.assignment.create")
	c.Set("audit_resource", "user_position")
	c.Set("audit_resource_id", c.Param("id"))

	up, err := h.svc.AddUserPosition(userID, req.DepartmentID, req.PositionID, req.IsPrimary)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyAssigned),
			errors.Is(err, ErrPositionAlreadyAssigned),
			errors.Is(err, department.ErrDepartmentNotFound),
			errors.Is(err, ErrDepartmentInactive),
			errors.Is(err, position.ErrPositionNotFound),
			errors.Is(err, ErrPositionInactive),
			errors.Is(err, ErrPositionNotAllowedInDept):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "added user position")
	handler.OK(c, up)
}

func (h *AssignmentHandler) RemoveUserPosition(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	assignID, err := parseAssignID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid assignmentId")
		return
	}

	c.Set("audit_action", "org.assignment.delete")
	c.Set("audit_resource", "user_position")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(assignID), 10))

	if err := h.svc.RemoveUserPosition(userID, assignID); err != nil {
		if errors.Is(err, ErrAssignmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "removed user position")
	handler.OK(c, nil)
}

func (h *AssignmentHandler) UpdateUserPosition(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	assignID, err := parseAssignID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid assignmentId")
		return
	}

	var req UpdateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.assignment.update")
	c.Set("audit_resource", "user_position")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(assignID), 10))

	if err := h.svc.UpdateUserPosition(userID, assignID, req.PositionID, req.IsPrimary); err != nil {
		if errors.Is(err, ErrAssignmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated user position")
	handler.OK(c, nil)
}

func (h *AssignmentHandler) ListUsers(c *gin.Context) {
	deptIDStr := c.Query("departmentId")
	if deptIDStr == "" {
		handler.Fail(c, http.StatusBadRequest, "departmentId is required")
		return
	}
	deptID, err := strconv.ParseUint(deptIDStr, 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid departmentId")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	keyword := c.Query("keyword")

	items, total, err := h.svc.ListDepartmentMembers(uint(deptID), keyword, page, pageSize)
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

func (h *AssignmentHandler) SetPrimary(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	assignID, err := parseAssignID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid assignmentId")
		return
	}

	if err := h.svc.SetPrimary(userID, assignID); err != nil {
		if errors.Is(err, ErrAssignmentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, nil)
}

func (h *AssignmentHandler) SetUserDeptPositions(c *gin.Context) {
	userID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	deptID, err := parseDeptID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid deptId")
		return
	}

	var req SetUserDeptPositionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "org.assignment.batch_update")
	c.Set("audit_resource", "user_position")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.SetUserDeptPositions(userID, deptID, req.PositionIDs, req.PrimaryPositionID); err != nil {
		switch {
		case errors.Is(err, department.ErrDepartmentNotFound),
			errors.Is(err, ErrDepartmentInactive),
			errors.Is(err, position.ErrPositionNotFound),
			errors.Is(err, ErrPositionInactive),
			errors.Is(err, ErrPositionNotAllowedInDept):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "batch updated user positions in department")
	handler.OK(c, nil)
}

func parseAssignID(c *gin.Context) (uint, error) {
	idStr := c.Param("assignmentId")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func parseDeptID(c *gin.Context) (uint, error) {
	idStr := c.Param("deptId")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
