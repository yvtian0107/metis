package definition

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type KnowledgeDocHandler struct {
	svc *KnowledgeDocService
}

func NewKnowledgeDocHandler(i do.Injector) (*KnowledgeDocHandler, error) {
	svc := do.MustInvoke[*KnowledgeDocService](i)
	return &KnowledgeDocHandler{svc: svc}, nil
}

func (h *KnowledgeDocHandler) Upload(c *gin.Context) {
	serviceID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid service id")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if !IsAllowedExtension(header.Filename) {
		handler.Fail(c, http.StatusBadRequest, "不支持的文件类型")
		return
	}

	if header.Size > MaxFileSize {
		handler.Fail(c, http.StatusBadRequest, "文件大小超过限制")
		return
	}

	c.Set("audit_action", "itsm.knowledge-doc.upload")
	c.Set("audit_resource", "service_knowledge_document")

	doc, err := h.svc.Upload(serviceID, header.Filename, header.Size, file)
	if err != nil {
		if errors.Is(err, ErrServiceDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(doc.ID)))
	c.Set("audit_summary", "uploaded knowledge doc: "+doc.FileName)
	handler.OK(c, doc.ToResponse())
}

func (h *KnowledgeDocHandler) List(c *gin.Context) {
	serviceID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid service id")
		return
	}

	docs, err := h.svc.List(serviceID)
	if err != nil {
		if errors.Is(err, ErrServiceDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	var resp []ServiceKnowledgeDocumentResponse
	for _, d := range docs {
		resp = append(resp, d.ToResponse())
	}
	if resp == nil {
		resp = []ServiceKnowledgeDocumentResponse{}
	}
	handler.OK(c, resp)
}

func (h *KnowledgeDocHandler) Delete(c *gin.Context) {
	serviceID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid service id")
		return
	}
	docID, err := strconv.ParseUint(c.Param("docId"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid document id")
		return
	}

	c.Set("audit_action", "itsm.knowledge-doc.delete")
	c.Set("audit_resource", "service_knowledge_document")
	c.Set("audit_resource_id", c.Param("docId"))

	if err := h.svc.Delete(serviceID, uint(docID)); err != nil {
		if errors.Is(err, ErrServiceDefNotFound) || errors.Is(err, ErrKnowledgeDocNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted knowledge doc")
	handler.OK(c, nil)
}
