package process

import (
	"errors"
	"metis/internal/app/node/domain"
	nodelog "metis/internal/app/node/log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type BindProcessRequest struct {
	ProcessDefID uint `json:"processDefId" binding:"required"`
}

type NodeProcessHandler struct {
	nodeProcessSvc *NodeProcessService
	logSvc         *nodelog.NodeProcessLogService
}

func NewNodeProcessHandler(i do.Injector) (*NodeProcessHandler, error) {
	return &NodeProcessHandler{
		nodeProcessSvc: do.MustInvoke[*NodeProcessService](i),
		logSvc:         do.MustInvoke[*nodelog.NodeProcessLogService](i),
	}, nil
}

func (h *NodeProcessHandler) Bind(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	var req BindProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "nodeProcess.bind")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	np, err := h.nodeProcessSvc.Bind(nodeID, req.ProcessDefID)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) || errors.Is(err, ErrProcessDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrNodeProcessExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "bound process to node")
	handler.OK(c, np.ToResponse())
}

func (h *NodeProcessHandler) List(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	items, err := h.nodeProcessSvc.ListByNodeID(nodeID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.NodeProcessResponse, len(items))
	for i, item := range items {
		resp := item.NodeProcess.ToResponse()
		resp.ProcessName = item.ProcessName
		resp.DisplayName = item.DisplayName
		result[i] = resp
	}

	handler.OK(c, result)
}

func (h *NodeProcessHandler) Unbind(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	c.Set("audit_action", "nodeProcess.unbind")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeProcessSvc.Unbind(nodeID, processDefID); err != nil {
		if errors.Is(err, ErrNodeProcessNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "unbound process from node")
	handler.OK(c, nil)
}

func (h *NodeProcessHandler) Start(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	c.Set("audit_action", "nodeProcess.start")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeProcessSvc.Start(nodeID, processDefID); err != nil {
		if errors.Is(err, ErrNodeProcessNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "started process on node")
	handler.OK(c, nil)
}

func (h *NodeProcessHandler) Stop(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	c.Set("audit_action", "nodeProcess.stop")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeProcessSvc.Stop(nodeID, processDefID); err != nil {
		if errors.Is(err, ErrNodeProcessNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "stopped process on node")
	handler.OK(c, nil)
}

func (h *NodeProcessHandler) Restart(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	c.Set("audit_action", "nodeProcess.restart")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeProcessSvc.Restart(nodeID, processDefID); err != nil {
		if errors.Is(err, ErrNodeProcessNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "restarted process on node")
	handler.OK(c, nil)
}

func (h *NodeProcessHandler) Reload(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	c.Set("audit_action", "nodeProcess.reload")
	c.Set("audit_resource", "node_process")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeProcessSvc.Reload(nodeID, processDefID); err != nil {
		if errors.Is(err, ErrNodeProcessNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "reloaded process config on node")
	handler.OK(c, nil)
}

func (h *NodeProcessHandler) Logs(c *gin.Context) {
	nodeID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid node id")
		return
	}

	processDefID, err := domain.ParseProcessDefID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid process id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	stream := c.Query("stream")

	result, err := h.logSvc.List(nodelog.LogListParams{
		NodeID:       nodeID,
		ProcessDefID: processDefID,
		Stream:       stream,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, result)
}
