package node

import (
	"encoding/json"
	"errors"
	"metis/internal/app/node/command"
	"metis/internal/app/node/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type CreateNodeRequest struct {
	Name   string          `json:"name" binding:"required,max=128"`
	Labels json.RawMessage `json:"labels"`
}

type UpdateNodeRequest struct {
	Name   *string          `json:"name" binding:"omitempty,max=128"`
	Labels *json.RawMessage `json:"labels"`
}

type NodeHandler struct {
	nodeSvc     *NodeService
	commandRepo *command.NodeCommandRepo
}

func NewNodeHandler(i do.Injector) (*NodeHandler, error) {
	return &NodeHandler{
		nodeSvc:     do.MustInvoke[*NodeService](i),
		commandRepo: do.MustInvoke[*command.NodeCommandRepo](i),
	}, nil
}

func (h *NodeHandler) Create(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "node.create")
	c.Set("audit_resource", "node")

	result, err := h.nodeSvc.Create(req.Name, domain.JSONMap(req.Labels))
	if err != nil {
		if errors.Is(err, ErrNodeNameExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(result.Node.ID)))
	c.Set("audit_summary", "created node: "+result.Node.Name)

	resp := result.Node.ToResponse()
	handler.OK(c, gin.H{
		"node":  resp,
		"token": result.Token,
	})
}

func (h *NodeHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := NodeListParams{
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.nodeSvc.List(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.NodeResponse, len(items))
	for i, item := range items {
		resp := item.Node.ToResponse()
		resp.ProcessCount = item.ProcessCount
		result[i] = resp
	}

	handler.OK(c, gin.H{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *NodeHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	node, err := h.nodeSvc.Get(id)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, node.ToResponse())
}

func (h *NodeHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "node.update")
	c.Set("audit_resource", "node")
	c.Set("audit_resource_id", c.Param("id"))

	var labels *domain.JSONMap
	if req.Labels != nil {
		l := domain.JSONMap(*req.Labels)
		labels = &l
	}

	node, err := h.nodeSvc.Update(id, req.Name, labels)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrNodeNameExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated node: "+node.Name)
	handler.OK(c, node.ToResponse())
}

func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "node.delete")
	c.Set("audit_resource", "node")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.nodeSvc.Delete(id); err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted node")
	handler.OK(c, nil)
}

func (h *NodeHandler) RotateToken(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "node.rotateToken")
	c.Set("audit_resource", "node")
	c.Set("audit_resource_id", c.Param("id"))

	token, err := h.nodeSvc.RotateToken(id)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "rotated node token")
	handler.OK(c, gin.H{"token": token})
}

func (h *NodeHandler) ListCommands(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	cmds, total, err := h.commandRepo.ListByNodeIDPaginated(id, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.NodeCommandResponse, len(cmds))
	for i, cmd := range cmds {
		result[i] = cmd.ToResponse()
	}

	handler.OK(c, gin.H{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}
