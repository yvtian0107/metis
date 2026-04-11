package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"metis/internal/model"
	"metis/internal/repository"
	"metis/internal/service"
)

type UserHandler struct {
	userSvc *service.UserService
	connRepo *repository.UserConnectionRepo
}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	keyword := c.Query("keyword")

	params := repository.ListParams{
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	}

	if isActiveStr := c.Query("isActive"); isActiveStr != "" {
		val := isActiveStr == "true"
		params.IsActive = &val
	}

	result, err := h.userSvc.List(params)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response (strip passwords)
	items := make([]model.UserResponse, len(result.Items))
	for i, u := range result.Items {
		items[i] = u.ToResponse()
		// Attach connections for login method display
		if conns, err := h.connRepo.FindByUserID(u.ID); err == nil {
			connResps := make([]model.UserConnectionResponse, len(conns))
			for j, c := range conns {
				connResps[j] = c.ToResponse()
			}
			items[i].Connections = connResps
		}
	}

	OK(c, gin.H{
		"items":    items,
		"total":    result.Total,
		"page":     page,
		"pageSize": pageSize,
	})
}

type createUserReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	RoleID   uint   `json:"roleId" binding:"required"`
}

func (h *UserHandler) Create(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.userSvc.Create(req.Username, req.Password, req.Email, req.Phone, req.RoleID)
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "user.create")
	c.Set("audit_resource", "user")
	c.Set("audit_summary", "创建用户 "+req.Username)
	OK(c, user.ToResponse())
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	user, err := h.userSvc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			Fail(c, http.StatusNotFound, "user not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, user.ToResponse())
}

type updateUserReq struct {
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Avatar   *string `json:"avatar"`
	Locale   *string `json:"locale"`
	Timezone *string `json:"timezone"`
	RoleID   *uint   `json:"roleId"`
	IsActive *bool   `json:"isActive"`
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	currentUserID := c.GetUint("userId")
	user, err := h.userSvc.Update(id, currentUserID, service.UpdateUserParams{
		Email:    req.Email,
		Phone:    req.Phone,
		Avatar:   req.Avatar,
		Locale:   req.Locale,
		Timezone: req.Timezone,
		RoleID:   req.RoleID,
		IsActive: req.IsActive,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrCannotSelf):
			Fail(c, http.StatusBadRequest, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "user.update")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "更新用户")
	OK(c, user.ToResponse())
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	currentUserID := c.GetUint("userId")
	if err := h.userSvc.Delete(id, currentUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			Fail(c, http.StatusNotFound, "user not found")
		case errors.Is(err, service.ErrCannotSelf):
			Fail(c, http.StatusBadRequest, "cannot delete self")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "user.delete")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "删除用户")
	OK(c, nil)
}

type resetPasswordReq struct {
	Password string `json:"password" binding:"required"`
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.userSvc.ResetPassword(id, req.Password); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			Fail(c, http.StatusNotFound, "user not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "user.reset_password")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "重置用户密码")
	OK(c, nil)
}

func (h *UserHandler) Activate(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	user, err := h.userSvc.Activate(id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			Fail(c, http.StatusNotFound, "user not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "user.activate")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "启用用户")
	OK(c, user.ToResponse())
}

func (h *UserHandler) Deactivate(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	currentUserID := c.GetUint("userId")
	user, err := h.userSvc.Deactivate(id, currentUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			Fail(c, http.StatusNotFound, "user not found")
		case errors.Is(err, service.ErrCannotSelf):
			Fail(c, http.StatusBadRequest, "cannot deactivate self")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "user.deactivate")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "禁用用户")
	OK(c, user.ToResponse())
}

func (h *UserHandler) Unlock(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	if err := h.userSvc.UnlockUser(id); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			Fail(c, http.StatusNotFound, "user not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "user.unlock")
	c.Set("audit_resource", "user")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "解锁用户")
	OK(c, nil)
}

func parseIDParam(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid id")
		return 0, err
	}
	return uint(id), nil
}
