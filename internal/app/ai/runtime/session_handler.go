package runtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type SessionHandler struct {
	svc     *SessionService
	gateway *AgentGateway
}

func NewSessionHandler(i do.Injector) (*SessionHandler, error) {
	return &SessionHandler{
		svc:     do.MustInvoke[*SessionService](i),
		gateway: do.MustInvoke[*AgentGateway](i),
	}, nil
}

type createSessionReq struct {
	AgentID uint `json:"agentId" binding:"required"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	session, err := h.svc.Create(req.AgentID, userID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, session.ToResponse())
}

func (h *SessionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	agentID, _ := strconv.Atoi(c.DefaultQuery("agentId", "0"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	sessions, total, err := h.svc.List(SessionListParams{
		AgentID:  uint(agentID),
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]AgentSessionResponse, len(sessions))
	for i, s := range sessions {
		items[i] = s.ToResponse()
	}
	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *SessionHandler) Get(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	session, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	messages, err := h.svc.GetMessages(session.ID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	msgResp := make([]SessionMessageResponse, len(messages))
	for i, m := range messages {
		msgResp[i] = m.ToResponse()
	}

	handler.OK(c, gin.H{
		"session":  session.ToResponse(),
		"messages": msgResp,
	})
}

func (h *SessionHandler) Delete(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.svc.GetOwned(uint(sid), userID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.svc.Delete(uint(sid)); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "session.delete")
	c.Set("audit_resource", "ai_session")
	c.Set("audit_resource_id", c.Param("sid"))

	handler.OK(c, nil)
}

type sendMessageReq struct {
	Content string   `json:"content"`
	Images  []string `json:"images"` // base64 encoded images or URLs
}

type chatReq struct {
	Messages []chatMessage `json:"messages"`
	Trigger  string        `json:"trigger"`
}

type chatMessage struct {
	Role  string         `json:"role"`
	Parts []chatPart     `json:"parts"`
	Text  string         `json:"text"`
	Data  map[string]any `json:"data"`
}

type chatPart struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	URL       string `json:"url"`
	MediaType string `json:"mediaType"`
}

func latestUserInput(messages []chatMessage) (string, []string) {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != "user" {
			continue
		}
		var text strings.Builder
		images := make([]string, 0)
		if message.Text != "" {
			text.WriteString(message.Text)
		}
		for _, part := range message.Parts {
			switch part.Type {
			case "text":
				text.WriteString(part.Text)
			case "file":
				if part.URL != "" && strings.HasPrefix(part.MediaType, "image/") {
					images = append(images, part.URL)
				}
			}
		}
		return strings.TrimSpace(text.String()), images
	}
	return "", nil
}

func imagesMetadata(images []string) json.RawMessage {
	if len(images) == 0 {
		return nil
	}
	meta := map[string]any{"images": images}
	metadata, _ := json.Marshal(meta)
	return metadata
}

func (h *SessionHandler) SendMessage(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Content == "" && len(req.Images) == 0 {
		handler.Fail(c, http.StatusBadRequest, "content or images required")
		return
	}

	session, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	msg, err := h.svc.StoreMessage(session.ID, MessageRoleUser, req.Content, imagesMetadata(req.Images), 0)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.svc.UpdateStatus(session.ID, SessionStatusRunning); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// TODO: Trigger Gateway execution asynchronously
	// For now, just store the message and return 202
	c.JSON(http.StatusAccepted, handler.R{Code: 0, Message: "ok", Data: msg.ToResponse()})
}

func (h *SessionHandler) Chat(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	isRegenerate := req.Trigger == "regenerate-message"
	content, images := latestUserInput(req.Messages)
	if !isRegenerate {
		if content == "" && len(images) == 0 {
			handler.Fail(c, http.StatusBadRequest, "content or images required")
			return
		}
	}

	session, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if !isRegenerate {
		if _, err := h.svc.StoreMessage(session.ID, MessageRoleUser, content, imagesMetadata(images), 0); err != nil {
			handler.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := h.svc.UpdateStatus(session.ID, SessionStatusRunning); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeGatewayStream(c, session.ID, userID)
}

func (h *SessionHandler) Stream(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.svc.GetOwned(uint(sid), userID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeGatewayStream(c, uint(sid), userID)
}

func (h *SessionHandler) writeGatewayStream(c *gin.Context, sessionID uint, userID uint) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Vercel-AI-UI-Message-Stream", "v1")

	streamReader, err := h.gateway.Run(c.Request.Context(), sessionID, userID)
	if err != nil {
		data, _ := json.Marshal(map[string]any{"type": "error", "errorText": err.Error()})
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString("data: " + string(data) + "\n\n")
		_, _ = c.Writer.WriteString("data: [DONE]\n\n")
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	defer streamReader.Close()

	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 4096)
		n, err := streamReader.Read(buf)
		if err != nil {
			return false
		}
		w.Write(buf[:n])
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
		return true
	})
}

func (h *SessionHandler) Cancel(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	session, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if session.Status != SessionStatusRunning {
		handler.OK(c, nil) // Idempotent no-op
		return
	}

	// Signal executor cancellation via Gateway
	h.gateway.Cancel(session.ID, userID)
	if err := h.svc.UpdateStatus(session.ID, SessionStatusCancelled); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, nil)
}

type updateSessionReq struct {
	Title  *string `json:"title"`
	Pinned *bool   `json:"pinned"`
}

func (h *SessionHandler) Update(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req updateSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.svc.GetOwned(uint(sid), userID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	if len(updates) == 0 {
		handler.Fail(c, http.StatusBadRequest, "no fields to update")
		return
	}

	if err := h.svc.Update(uint(sid), updates); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, nil)
}

type editMessageReq struct {
	Content string `json:"content" binding:"required"`
}

func (h *SessionHandler) EditMessage(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	mid, _ := strconv.Atoi(c.Param("mid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req editMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.svc.GetOwned(uint(sid), userID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	msg, err := h.svc.EditMessage(uint(sid), uint(mid), req.Content)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, msg.ToResponse())
}

func (h *SessionHandler) Continue(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	session, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if session.Status == SessionStatusRunning {
		handler.Fail(c, http.StatusConflict, "session is already running")
		return
	}

	// Update status to running, then trigger stream via normal SSE flow
	if err := h.svc.UpdateStatus(session.ID, SessionStatusRunning); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, nil)
}

const maxImageSize = 5 * 1024 * 1024 // 5MB

func (h *SessionHandler) UploadImage(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("sid"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Verify session exists
	_, err := h.svc.GetOwned(uint(sid), userID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "file is required")
		return
	}

	// Validate file size
	if file.Size > maxImageSize {
		handler.Fail(c, http.StatusBadRequest, "image exceeds 5MB limit")
		return
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		handler.Fail(c, http.StatusBadRequest, "invalid file type, must be an image")
		return
	}

	// Open and read file
	f, err := file.Open()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to read file")
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Convert to base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)

	// Store image reference in session storage (or return directly for client use)
	handler.OK(c, gin.H{"url": dataURL})
}
