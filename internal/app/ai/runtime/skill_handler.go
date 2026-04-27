package runtime

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type SkillHandler struct {
	svc *SkillService
}

func NewSkillHandler(i do.Injector) (*SkillHandler, error) {
	return &SkillHandler{
		svc: do.MustInvoke[*SkillService](i),
	}, nil
}

func (h *SkillHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	skills, total, err := h.svc.List(SkillListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]SkillResponse, len(skills))
	for i, s := range skills {
		items[i] = s.ToResponse()
	}
	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *SkillHandler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	skill, err := h.svc.Get(uint(id))
	if err != nil {
		if errors.Is(err, ErrSkillNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, skill.ToResponse())
}

func (h *SkillHandler) ImportGitHub(c *gin.Context) {
	var req ImportGitHubReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	skill, err := h.svc.InstallFromGitHub(req.URL)
	if err != nil {
		if errors.Is(err, ErrInvalidGitHubURL) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrNotImplemented) {
			handler.Fail(c, http.StatusNotImplemented, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "skill.create")
	c.Set("audit_resource", "ai_skill")
	c.Set("audit_resource_id", strconv.Itoa(int(skill.ID)))
	c.Set("audit_summary", "Imported skill from GitHub: "+skill.Name)

	handler.OK(c, skill.ToResponse())
}

func (h *SkillHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "file is required")
		return
	}

	f, err := file.Open()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer f.Close()

	skill, err := h.svc.InstallFromUpload(f)
	if err != nil {
		if errors.Is(err, ErrInvalidSkillPackage) || errors.Is(err, ErrInvalidManifest) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "skill.create")
	c.Set("audit_resource", "ai_skill")
	c.Set("audit_resource_id", strconv.Itoa(int(skill.ID)))
	c.Set("audit_summary", "Uploaded skill: "+skill.Name)

	handler.OK(c, skill.ToResponse())
}

type updateSkillReq struct {
	AuthType   string `json:"authType"`
	AuthConfig string `json:"authConfig"`
}

func (h *SkillHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req updateSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	authType := req.AuthType
	if authType == "" {
		authType = AuthTypeNone
	}

	skill, err := h.svc.Update(uint(id), authType, req.AuthConfig)
	if err != nil {
		if errors.Is(err, ErrSkillNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "skill.update")
	c.Set("audit_resource", "ai_skill")
	c.Set("audit_resource_id", strconv.Itoa(int(skill.ID)))
	c.Set("audit_summary", "Updated skill: "+skill.Name)

	handler.OK(c, skill.ToResponse())
}

type toggleSkillReq struct {
	IsActive bool `json:"isActive"`
}

func (h *SkillHandler) ToggleActive(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req toggleSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	skill, err := h.svc.ToggleActive(uint(id), req.IsActive)
	if err != nil {
		if errors.Is(err, ErrSkillNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, skill.ToResponse())
}

func (h *SkillHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "skill.delete")
	c.Set("audit_resource", "ai_skill")
	c.Set("audit_resource_id", c.Param("id"))

	handler.OK(c, nil)
}

// Package returns the full skill content for Agent download (internal API).
func (h *SkillHandler) Package(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	pkg, err := h.svc.GetPackage(uint(id))
	if err != nil {
		if errors.Is(err, ErrSkillNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, pkg)
}
