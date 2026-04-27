package processdef

import (
	"encoding/json"
	"errors"
	"metis/internal/app/node/domain"
	nodeprocess "metis/internal/app/node/process"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type CreateProcessDefRequest struct {
	Name           string          `json:"name" binding:"required,max=128"`
	DisplayName    string          `json:"displayName" binding:"required,max=128"`
	Description    string          `json:"description"`
	StartCommand   string          `json:"startCommand" binding:"required,max=512"`
	StopCommand    string          `json:"stopCommand" binding:"max=512"`
	ReloadCommand  string          `json:"reloadCommand" binding:"max=512"`
	Env            json.RawMessage `json:"env"`
	ConfigFiles    json.RawMessage `json:"configFiles"`
	ProbeType      string          `json:"probeType"`
	ProbeConfig    json.RawMessage `json:"probeConfig"`
	RestartPolicy  string          `json:"restartPolicy"`
	MaxRestarts    int             `json:"maxRestarts"`
	ResourceLimits json.RawMessage `json:"resourceLimits"`
}

type UpdateProcessDefRequest struct {
	DisplayName    *string          `json:"displayName" binding:"omitempty,max=128"`
	Description    *string          `json:"description"`
	StartCommand   *string          `json:"startCommand" binding:"omitempty,max=512"`
	StopCommand    *string          `json:"stopCommand" binding:"omitempty,max=512"`
	ReloadCommand  *string          `json:"reloadCommand" binding:"omitempty,max=512"`
	Env            *json.RawMessage `json:"env"`
	ConfigFiles    *json.RawMessage `json:"configFiles"`
	ProbeType      *string          `json:"probeType"`
	ProbeConfig    *json.RawMessage `json:"probeConfig"`
	RestartPolicy  *string          `json:"restartPolicy"`
	MaxRestarts    *int             `json:"maxRestarts"`
	ResourceLimits *json.RawMessage `json:"resourceLimits"`
}

type ProcessDefHandler struct {
	processDefSvc  *ProcessDefService
	nodeProcessSvc *nodeprocess.NodeProcessService
}

func NewProcessDefHandler(i do.Injector) (*ProcessDefHandler, error) {
	return &ProcessDefHandler{
		processDefSvc:  do.MustInvoke[*ProcessDefService](i),
		nodeProcessSvc: do.MustInvoke[*nodeprocess.NodeProcessService](i),
	}, nil
}

func (h *ProcessDefHandler) Create(c *gin.Context) {
	var req CreateProcessDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "processDef.create")
	c.Set("audit_resource", "process_def")

	pd := &domain.ProcessDef{
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		StartCommand:   req.StartCommand,
		StopCommand:    req.StopCommand,
		ReloadCommand:  req.ReloadCommand,
		Env:            domain.JSONMap(req.Env),
		ConfigFiles:    domain.JSONArray(req.ConfigFiles),
		ProbeType:      req.ProbeType,
		ProbeConfig:    domain.JSONMap(req.ProbeConfig),
		RestartPolicy:  req.RestartPolicy,
		MaxRestarts:    req.MaxRestarts,
		ResourceLimits: domain.JSONMap(req.ResourceLimits),
	}
	if pd.ProbeType == "" {
		pd.ProbeType = domain.ProbeTypeNone
	}
	if pd.RestartPolicy == "" {
		pd.RestartPolicy = domain.RestartPolicyAlways
	}
	if pd.MaxRestarts == 0 {
		pd.MaxRestarts = 10
	}

	if err := h.processDefSvc.Create(pd); err != nil {
		if errors.Is(err, ErrProcessDefNameExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(pd.ID)))
	c.Set("audit_summary", "created process definition: "+pd.Name)
	handler.OK(c, pd.ToResponse())
}

func (h *ProcessDefHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := ProcessDefListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.processDefSvc.List(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.ProcessDefResponse, len(items))
	for i, item := range items {
		result[i] = item.ToResponse()
	}

	handler.OK(c, gin.H{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *ProcessDefHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	pd, err := h.processDefSvc.Get(id)
	if err != nil {
		if errors.Is(err, ErrProcessDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, pd.ToResponse())
}

func (h *ProcessDefHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateProcessDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "processDef.update")
	c.Set("audit_resource", "process_def")
	c.Set("audit_resource_id", c.Param("id"))

	updates := map[string]any{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.StartCommand != nil {
		updates["start_command"] = *req.StartCommand
	}
	if req.StopCommand != nil {
		updates["stop_command"] = *req.StopCommand
	}
	if req.ReloadCommand != nil {
		updates["reload_command"] = *req.ReloadCommand
	}
	if req.Env != nil {
		updates["env"] = domain.JSONMap(*req.Env)
	}
	if req.ConfigFiles != nil {
		updates["config_files"] = domain.JSONArray(*req.ConfigFiles)
	}
	if req.ProbeType != nil {
		updates["probe_type"] = *req.ProbeType
	}
	if req.ProbeConfig != nil {
		updates["probe_config"] = domain.JSONMap(*req.ProbeConfig)
	}
	if req.RestartPolicy != nil {
		updates["restart_policy"] = *req.RestartPolicy
	}
	if req.MaxRestarts != nil {
		updates["max_restarts"] = *req.MaxRestarts
	}
	if req.ResourceLimits != nil {
		updates["resource_limits"] = domain.JSONMap(*req.ResourceLimits)
	}

	pd, err := h.processDefSvc.Update(id, updates)
	if err != nil {
		if errors.Is(err, ErrProcessDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated process definition: "+pd.Name)
	handler.OK(c, pd.ToResponse())
}

func (h *ProcessDefHandler) Delete(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "processDef.delete")
	c.Set("audit_resource", "process_def")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.processDefSvc.Delete(id); err != nil {
		if errors.Is(err, ErrProcessDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted process definition")
	handler.OK(c, nil)
}

func (h *ProcessDefHandler) ListNodes(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	items, err := h.nodeProcessSvc.ListNodesByProcessDefID(id)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, items)
}
