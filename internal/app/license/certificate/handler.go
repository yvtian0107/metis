package certificate

import (
	"encoding/json"
	"errors"
	"fmt"
	"metis/internal/app/license/domain"
	licenseepkg "metis/internal/app/license/licensee"
	productpkg "metis/internal/app/license/product"
	"metis/internal/app/license/registration"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// --- domain.License request types ---

type IssueLicenseRequest struct {
	ProductID              uint            `json:"productId" binding:"required"`
	LicenseeID             uint            `json:"licenseeId" binding:"required"`
	PlanID                 *uint           `json:"planId"`
	PlanName               string          `json:"planName" binding:"required"`
	RegistrationCode       string          `json:"registrationCode" binding:"required"`
	AutoCreateRegistration bool            `json:"autoCreateRegistration"`
	ConstraintValues       json.RawMessage `json:"constraintValues"`
	ValidFrom              string          `json:"validFrom" binding:"required"`
	ValidUntil             *string         `json:"validUntil"`
	Notes                  string          `json:"notes"`
}

type RenewLicenseRequest struct {
	ValidUntil *string `json:"validUntil"`
}

type UpgradeLicenseRequest struct {
	ProductID              uint            `json:"productId" binding:"required"`
	LicenseeID             uint            `json:"licenseeId" binding:"required"`
	PlanID                 *uint           `json:"planId"`
	PlanName               string          `json:"planName" binding:"required"`
	RegistrationCode       string          `json:"registrationCode" binding:"required"`
	AutoCreateRegistration bool            `json:"autoCreateRegistration"`
	ConstraintValues       json.RawMessage `json:"constraintValues"`
	ValidFrom              string          `json:"validFrom" binding:"required"`
	ValidUntil             *string         `json:"validUntil"`
	Notes                  string          `json:"notes"`
}

type CreateLicenseRegistrationRequest struct {
	ProductID  *uint      `json:"productId"`
	LicenseeID *uint      `json:"licenseeId"`
	Code       string     `json:"code" binding:"required"`
	Source     string     `json:"source"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

type GenerateLicenseRegistrationRequest struct {
	ProductID  *uint `json:"productId"`
	LicenseeID *uint `json:"licenseeId"`
}

// --- LicenseHandler ---

type LicenseHandler struct {
	licenseSvc *LicenseService
}

func NewLicenseHandler(i do.Injector) (*LicenseHandler, error) {
	return &LicenseHandler{
		licenseSvc: do.MustInvoke[*LicenseService](i),
	}, nil
}

func parseLicenseDate(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t, err = time.Parse("2006-01-02", value)
	}
	return t, err
}

func parseOptionalLicenseDate(value *string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	t, err := parseLicenseDate(*value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (h *LicenseHandler) Issue(c *gin.Context) {
	var req IssueLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	validFrom, err := parseLicenseDate(req.ValidFrom)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid validFrom date format")
		return
	}

	validUntil, err := parseOptionalLicenseDate(req.ValidUntil)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid validUntil date format")
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "license.issue")
	c.Set("audit_resource", "license")

	license, err := h.licenseSvc.IssueLicense(IssueLicenseParams{
		ProductID:              req.ProductID,
		LicenseeID:             req.LicenseeID,
		PlanID:                 req.PlanID,
		PlanName:               req.PlanName,
		RegistrationCode:       req.RegistrationCode,
		AutoCreateRegistration: req.AutoCreateRegistration,
		ConstraintValues:       req.ConstraintValues,
		ValidFrom:              validFrom,
		ValidUntil:             validUntil,
		Notes:                  req.Notes,
		IssuedBy:               userID.(uint),
	})
	if err != nil {
		if errors.Is(err, productpkg.ErrProductNotFound) || errors.Is(err, ErrProductNotPublished) ||
			errors.Is(err, licenseepkg.ErrLicenseeNotFound) || errors.Is(err, ErrLicenseeNotActive) ||
			errors.Is(err, ErrProductKeyNotFound) || errors.Is(err, ErrRegistrationNotFound) ||
			errors.Is(err, ErrRegistrationAlreadyBound) || errors.Is(err, ErrRegistrationExpired) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(license.ID)))
	c.Set("audit_summary", "签发许可: "+license.PlanName)
	handler.OK(c, license.ToResponse())
}

func (h *LicenseHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	var productID, licenseeID uint
	if v := c.Query("productId"); v != "" {
		id, _ := strconv.ParseUint(v, 10, 64)
		productID = uint(id)
	}
	if v := c.Query("licenseeId"); v != "" {
		id, _ := strconv.ParseUint(v, 10, 64)
		licenseeID = uint(id)
	}

	params := LicenseListParams{
		ProductID:       productID,
		LicenseeID:      licenseeID,
		Status:          c.Query("status"),
		LifecycleStatus: c.Query("lifecycleStatus"),
		Keyword:         c.Query("keyword"),
		Page:            page,
		PageSize:        pageSize,
	}

	items, total, err := h.licenseSvc.ListLicenses(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.LicenseResponse, len(items))
	for i, item := range items {
		resp := item.License.ToResponse()
		resp.ProductName = item.ProductName
		resp.LicenseeName = item.LicenseeName
		result[i] = resp
	}

	handler.OK(c, gin.H{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *LicenseHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	detail, err := h.licenseSvc.GetLicense(id)
	if err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := detail.License.ToResponse()
	resp.ProductName = detail.ProductName
	resp.ProductCode = detail.ProductCode
	resp.LicenseeName = detail.LicenseeName
	resp.LicenseeCode = detail.LicenseeCode
	resp.LicenseKey = detail.ProductLicenseKey
	handler.OK(c, resp)
}

func (h *LicenseHandler) Revoke(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "license.revoke")
	c.Set("audit_resource", "license")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.licenseSvc.RevokeLicense(id, userID.(uint)); err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrLicenseAlreadyRevoked) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "吊销许可")
	handler.OK(c, nil)
}

func (h *LicenseHandler) Export(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	format := c.DefaultQuery("format", "v1")
	licFile, filename, err := h.licenseSvc.ExportLicFile(id, format)
	if err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrRevokedLicenseNoExport) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(licFile))
}

func (h *LicenseHandler) Renew(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req RenewLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	validUntil, err := parseOptionalLicenseDate(req.ValidUntil)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid validUntil date format")
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "license.renew")
	c.Set("audit_resource", "license")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.licenseSvc.RenewLicense(id, validUntil, userID.(uint)); err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrLicenseAlreadyRevoked) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "续期许可")
	handler.OK(c, nil)
}

func (h *LicenseHandler) Upgrade(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpgradeLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	validFrom, err := parseLicenseDate(req.ValidFrom)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid validFrom date format")
		return
	}

	validUntil, err := parseOptionalLicenseDate(req.ValidUntil)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid validUntil date format")
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "license.upgrade")
	c.Set("audit_resource", "license")
	c.Set("audit_resource_id", c.Param("id"))

	license, err := h.licenseSvc.UpgradeLicense(id, IssueLicenseParams{
		ProductID:              req.ProductID,
		LicenseeID:             req.LicenseeID,
		PlanID:                 req.PlanID,
		PlanName:               req.PlanName,
		RegistrationCode:       req.RegistrationCode,
		AutoCreateRegistration: req.AutoCreateRegistration,
		ConstraintValues:       req.ConstraintValues,
		ValidFrom:              validFrom,
		ValidUntil:             validUntil,
		Notes:                  req.Notes,
		IssuedBy:               userID.(uint),
	})
	if err != nil {
		if errors.Is(err, ErrLicenseNotFound) || errors.Is(err, ErrLicenseAlreadyRevoked) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, productpkg.ErrProductNotFound) || errors.Is(err, ErrProductNotPublished) ||
			errors.Is(err, licenseepkg.ErrLicenseeNotFound) || errors.Is(err, ErrLicenseeNotActive) ||
			errors.Is(err, ErrProductKeyNotFound) || errors.Is(err, ErrRegistrationNotFound) ||
			errors.Is(err, ErrRegistrationAlreadyBound) || errors.Is(err, ErrRegistrationExpired) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(license.ID)))
	c.Set("audit_summary", "升级许可")
	handler.OK(c, license.ToResponse())
}

func (h *LicenseHandler) Suspend(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "license.suspend")
	c.Set("audit_resource", "license")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.licenseSvc.SuspendLicense(id, userID.(uint)); err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrLicenseAlreadyRevoked) || errors.Is(err, ErrLicenseAlreadySuspended) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "暂停许可")
	handler.OK(c, nil)
}

func (h *LicenseHandler) Reactivate(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "license.reactivate")
	c.Set("audit_resource", "license")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.licenseSvc.ReactivateLicense(id); err != nil {
		if errors.Is(err, ErrLicenseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrLicenseAlreadyRevoked) || errors.Is(err, ErrLicenseNotSuspended) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "恢复许可")
	handler.OK(c, nil)
}

// --- domain.LicenseRegistration handlers ---

func (h *LicenseHandler) CreateRegistration(c *gin.Context) {
	var req CreateLicenseRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "license.registration.create")
	c.Set("audit_resource", "license_registration")

	lr, err := h.licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
		ProductID:  req.ProductID,
		LicenseeID: req.LicenseeID,
		Code:       req.Code,
		Source:     req.Source,
		ExpiresAt:  req.ExpiresAt,
	})
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(lr.ID)))
	c.Set("audit_summary", "created license registration")
	handler.OK(c, lr)
}

func (h *LicenseHandler) ListRegistrations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	var productID, licenseeID uint
	if v := c.Query("productId"); v != "" {
		id, _ := strconv.ParseUint(v, 10, 64)
		productID = uint(id)
	}
	if v := c.Query("licenseeId"); v != "" {
		id, _ := strconv.ParseUint(v, 10, 64)
		licenseeID = uint(id)
	}
	available := c.Query("available") == "true"

	items, total, err := h.licenseSvc.ListLicenseRegistrations(registration.LicenseRegistrationListParams{
		ProductID:  productID,
		LicenseeID: licenseeID,
		Available:  available,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *LicenseHandler) GenerateRegistration(c *gin.Context) {
	var req GenerateLicenseRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "license.registration.generate")
	c.Set("audit_resource", "license_registration")

	lr, err := h.licenseSvc.GenerateLicenseRegistration(req.ProductID, req.LicenseeID)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(lr.ID)))
	c.Set("audit_summary", "generated license registration")
	handler.OK(c, lr)
}
