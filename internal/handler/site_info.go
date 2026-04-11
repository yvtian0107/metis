package handler

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"metis/internal/model"
	"metis/internal/version"
)

const (
	keySiteAppName  = "system.app_name"
	keySiteLogo     = "system.logo"
	keySiteLocale   = "system.locale"
	keySiteTimezone = "system.timezone"
	defaultAppName  = "Metis"
	maxLogoBytes    = 2 * 1024 * 1024 // 2MB
)

type siteInfoResp struct {
	AppName   string `json:"appName"`
	HasLogo   bool   `json:"hasLogo"`
	Locale    string `json:"locale"`
	Timezone  string `json:"timezone"`
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildTime string `json:"buildTime"`
}

func (h *Handler) GetSiteInfo(c *gin.Context) {
	appName := defaultAppName
	if cfg, err := h.sysCfg.Get(keySiteAppName); err == nil {
		appName = cfg.Value
	}

	hasLogo := false
	if cfg, err := h.sysCfg.Get(keySiteLogo); err == nil && cfg.Value != "" {
		hasLogo = true
	}

	locale := "zh-CN"
	if cfg, err := h.sysCfg.Get(keySiteLocale); err == nil && cfg.Value != "" {
		locale = cfg.Value
	}

	timezone := "UTC"
	if cfg, err := h.sysCfg.Get(keySiteTimezone); err == nil && cfg.Value != "" {
		timezone = cfg.Value
	}

	OK(c, siteInfoResp{
		AppName:   appName,
		HasLogo:   hasLogo,
		Locale:    locale,
		Timezone:  timezone,
		Version:   version.Version,
		GitCommit: version.GitCommit,
		BuildTime: version.BuildTime,
	})
}

type updateSiteInfoReq struct {
	AppName string `json:"appName" binding:"required"`
}

func (h *Handler) UpdateSiteInfo(c *gin.Context) {
	var req updateSiteInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "appName is required")
		return
	}

	cfg := &model.SystemConfig{
		Key:    keySiteAppName,
		Value:  req.AppName,
		Remark: "系统名称",
	}
	if err := h.sysCfg.Set(cfg); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.GetSiteInfo(c)
}

func (h *Handler) GetLogo(c *gin.Context) {
	cfg, err := h.sysCfg.Get(keySiteLogo)
	if err != nil || cfg.Value == "" {
		Fail(c, http.StatusNotFound, "logo not found")
		return
	}

	// Parse data URL: data:image/png;base64,xxxx
	contentType, data, ok := parseDataURL(cfg.Value)
	if !ok {
		Fail(c, http.StatusInternalServerError, "invalid logo data")
		return
	}

	c.Data(http.StatusOK, contentType, data)
}

type uploadLogoReq struct {
	Data string `json:"data" binding:"required"`
}

func (h *Handler) UploadLogo(c *gin.Context) {
	var req uploadLogoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "data is required")
		return
	}

	// Validate format: data:image/*;base64,...
	contentType, data, ok := parseDataURL(req.Data)
	if !ok || !strings.HasPrefix(contentType, "image/") {
		Fail(c, http.StatusBadRequest, "invalid data URL format, must be data:image/*;base64,...")
		return
	}

	// Check decoded size
	if len(data) > maxLogoBytes {
		Fail(c, http.StatusBadRequest, "logo exceeds 2MB limit")
		return
	}

	cfg := &model.SystemConfig{
		Key:    keySiteLogo,
		Value:  req.Data,
		Remark: "系统Logo",
	}
	if err := h.sysCfg.Set(cfg); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, nil)
}

func (h *Handler) DeleteLogo(c *gin.Context) {
	if err := h.sysCfg.Delete(keySiteLogo); err != nil {
		Fail(c, http.StatusNotFound, "logo not found")
		return
	}
	OK(c, nil)
}

// parseDataURL parses "data:<mediatype>;base64,<data>" and returns mediatype + decoded bytes.
func parseDataURL(dataURL string) (contentType string, data []byte, ok bool) {
	// Must start with "data:"
	if !strings.HasPrefix(dataURL, "data:") {
		return "", nil, false
	}
	rest := dataURL[5:]

	// Split on ";base64,"
	idx := strings.Index(rest, ";base64,")
	if idx < 0 {
		return "", nil, false
	}

	contentType = rest[:idx]
	encoded := rest[idx+8:]

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, false
	}

	return contentType, data, true
}
