package definition

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type WorkflowGenerateHandler struct {
	svc *WorkflowGenerateService
}

func NewWorkflowGenerateHandler(i do.Injector) (*WorkflowGenerateHandler, error) {
	return &WorkflowGenerateHandler{
		svc: do.MustInvoke[*WorkflowGenerateService](i),
	}, nil
}

func (h *WorkflowGenerateHandler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Generate(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrPathEngineNotConfigured) || errors.Is(err, ErrCollaborationSpecEmpty) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrPathEngineUpstream) {
			status = http.StatusBadGateway
		}
		handler.Fail(c, status, err.Error())
		return
	}

	c.Set("audit_action", "itsm.workflow.generate")
	c.Set("audit_resource", "itsm_workflow")
	c.Set("audit_summary", "Generated workflow from collaboration spec")

	handler.OK(c, resp)
}
