package runtime

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// KnowledgeSourceHandler exposes REST endpoints for the independent source pool.
type KnowledgeSourceHandler struct {
	svc       *KnowledgeSourceService
	repo      *KnowledgeSourceRepo
	assetRepo *KnowledgeAssetRepo
}

func NewKnowledgeSourceHandler(i do.Injector) (*KnowledgeSourceHandler, error) {
	return &KnowledgeSourceHandler{
		svc:       do.MustInvoke[*KnowledgeSourceService](i),
		repo:      do.MustInvoke[*KnowledgeSourceRepo](i),
		assetRepo: do.MustInvoke[*KnowledgeAssetRepo](i),
	}, nil
}

// sourceID is a shorthand for parsing the :id path param.
func sourceID(c *gin.Context) (uint, bool) {
	return handler.ParseUintParam(c, "id")
}

// --- Upload file source ---

func (h *KnowledgeSourceHandler) Upload(c *gin.Context) {
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

	format := detectFormat(file.Filename)
	if format == "" {
		handler.Fail(c, http.StatusBadRequest, "unsupported file format")
		return
	}

	// Read content for text-based formats; for binary, leave empty (async extract)
	var content string
	if format == SourceFormatMarkdown || format == SourceFormatText {
		data, err := io.ReadAll(f)
		if err != nil {
			handler.Fail(c, http.StatusInternalServerError, "failed to read file")
			return
		}
		content = string(data)
	}

	title := c.PostForm("title")
	if title == "" {
		title = file.Filename
	}

	src, err := h.svc.CreateFileSource(title, format, file.Filename, file.Size, content)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_source.upload")
	c.Set("audit_resource", "ai_knowledge_source")
	c.Set("audit_resource_id", strconv.Itoa(int(src.ID)))
	c.Set("audit_summary", "Uploaded knowledge source: "+src.Title)

	handler.OK(c, src.ToResponse())
}

// --- Add URL source ---

type addURLReq struct {
	Title         string `json:"title"`
	SourceURL     string `json:"sourceUrl" binding:"required,url"`
	CrawlDepth    int    `json:"crawlDepth"`
	URLPattern    string `json:"urlPattern"`
	CrawlEnabled  bool   `json:"crawlEnabled"`
	CrawlSchedule string `json:"crawlSchedule"`
}

func (h *KnowledgeSourceHandler) AddURL(c *gin.Context) {
	var req addURLReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	src, err := h.svc.CreateURLSource(req.Title, req.SourceURL, req.CrawlDepth, req.URLPattern, req.CrawlEnabled, req.CrawlSchedule)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_source.add_url")
	c.Set("audit_resource", "ai_knowledge_source")
	c.Set("audit_resource_id", strconv.Itoa(int(src.ID)))
	c.Set("audit_summary", "Added URL source: "+src.SourceURL)

	handler.OK(c, src.ToResponse())
}

// --- Add text source ---

type addTextReq struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func (h *KnowledgeSourceHandler) AddText(c *gin.Context) {
	var req addTextReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	src, err := h.svc.CreateTextSource(req.Title, req.Content)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_source.add_text")
	c.Set("audit_resource", "ai_knowledge_source")
	c.Set("audit_resource_id", strconv.Itoa(int(src.ID)))
	c.Set("audit_summary", "Added text source: "+src.Title)

	handler.OK(c, src.ToResponse())
}

// --- List sources ---

func (h *KnowledgeSourceHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := SourceListParams{
		Format:        c.Query("format"),
		ExtractStatus: c.Query("status"),
		Keyword:       c.Query("keyword"),
		Page:          page,
		PageSize:      pageSize,
	}

	sources, total, err := h.svc.List(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Enrich with ref counts
	type sourceWithRef struct {
		KnowledgeSourceResponse
		RefCount int `json:"refCount"`
	}
	items := make([]sourceWithRef, 0, len(sources))
	for i := range sources {
		resp := sources[i].ToResponse()
		refCount, _ := h.svc.GetRefCount(sources[i].ID)
		items = append(items, sourceWithRef{
			KnowledgeSourceResponse: resp,
			RefCount:                int(refCount),
		})
	}

	handler.OK(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

// --- Get source detail ---

func (h *KnowledgeSourceHandler) Get(c *gin.Context) {
	id, ok := sourceID(c)
	if !ok {
		return
	}

	src, err := h.svc.Get(id)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	resp := src.ToResponse()
	refCount, _ := h.svc.GetRefCount(id)
	resp.RefCount = int(refCount)

	handler.OK(c, resp)
}

// --- Get source content ---

func (h *KnowledgeSourceHandler) GetContent(c *gin.Context) {
	id, ok := sourceID(c)
	if !ok {
		return
	}

	src, err := h.svc.Get(id)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"id":      src.ID,
		"title":   src.Title,
		"content": src.Content,
	})
}

// --- Delete source ---

func (h *KnowledgeSourceHandler) Delete(c *gin.Context) {
	id, ok := sourceID(c)
	if !ok {
		return
	}

	refIDs, err := h.svc.Delete(id)
	if err != nil {
		if len(refIDs) > 0 {
			// 409 Conflict — source is referenced
			c.JSON(http.StatusConflict, handler.R{
				Code:    -1,
				Message: err.Error(),
				Data:    gin.H{"referencingAssetIds": refIDs},
			})
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_source.delete")
	c.Set("audit_resource", "ai_knowledge_source")
	c.Set("audit_resource_id", strconv.Itoa(int(id)))

	handler.OK(c, nil)
}

// --- Get referencing assets ---

func (h *KnowledgeSourceHandler) GetReferences(c *gin.Context) {
	id, ok := sourceID(c)
	if !ok {
		return
	}

	assets, err := h.svc.GetReferencingAssets(id)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]KnowledgeAssetResponse, 0, len(assets))
	for i := range assets {
		items = append(items, assets[i].ToResponse())
	}

	handler.OK(c, items)
}

// --- Utilities ---

// detectFormat returns the source format based on file extension.
func detectFormat(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return SourceFormatMarkdown
	case ".txt":
		return SourceFormatText
	case ".pdf":
		return SourceFormatPDF
	case ".docx":
		return SourceFormatDocx
	case ".xlsx":
		return SourceFormatXlsx
	case ".pptx":
		return SourceFormatPptx
	default:
		return ""
	}
}
