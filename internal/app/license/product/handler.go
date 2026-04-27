package product

import (
	"encoding/json"
	"errors"
	"fmt"
	licensecrypto "metis/internal/app/license/crypto"
	"metis/internal/app/license/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// --- Request types ---

type CreateProductRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=64"`
	Description string `json:"description"`
}

type UpdateProductRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=128"`
	Description *string `json:"description"`
}

type UpdateSchemaRequest struct {
	ConstraintSchema json.RawMessage `json:"constraintSchema" binding:"required"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreatePlanRequest struct {
	Name             string          `json:"name" binding:"required,max=128"`
	ConstraintValues json.RawMessage `json:"constraintValues"`
	SortOrder        int             `json:"sortOrder"`
}

type UpdatePlanRequest struct {
	Name             *string         `json:"name" binding:"omitempty,max=128"`
	ConstraintValues json.RawMessage `json:"constraintValues"`
	SortOrder        *int            `json:"sortOrder"`
}

type SetDefaultRequest struct {
	IsDefault bool `json:"isDefault"`
}

// --- ProductHandler ---

type ProductHandler struct {
	productSvc *ProductService
	licenseSvc LicenseOperations
}

type LicenseOperations interface {
	AssessKeyRotationImpact(productID uint) (*domain.RotateKeyImpact, error)
	BulkReissueLicenses(productID uint, ids []uint, issuedBy uint) (int, error)
}

func NewProductHandler(i do.Injector) (*ProductHandler, error) {
	return &ProductHandler{
		productSvc: do.MustInvoke[*ProductService](i),
		licenseSvc: do.MustInvoke[LicenseOperations](i),
	}, nil
}

func (h *ProductHandler) Create(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "product.create")
	c.Set("audit_resource", "license_product")

	product, err := h.productSvc.CreateProduct(req.Name, req.Code, req.Description)
	if err != nil {
		if errors.Is(err, ErrProductCodeExists) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, licensecrypto.ErrNoEncryptionKey) {
			handler.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(product.ID)))
	c.Set("audit_summary", "created product: "+product.Name)
	handler.OK(c, product.ToResponse())
}

func (h *ProductHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := ProductListParams{
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.productSvc.ListProducts(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]domain.ProductResponse, len(items))
	for i, item := range items {
		resp := item.Product.ToResponse()
		resp.PlanCount = item.PlanCount
		result[i] = resp
	}

	handler.OK(c, gin.H{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *ProductHandler) Get(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	product, err := h.productSvc.GetProduct(id)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, product.ToResponse())
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "product.update")
	c.Set("audit_resource", "license_product")
	c.Set("audit_resource_id", c.Param("id"))

	product, err := h.productSvc.UpdateProduct(id, UpdateProductParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated product: "+product.Name)
	handler.OK(c, product.ToResponse())
}

func (h *ProductHandler) UpdateSchema(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateSchemaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "product.update")
	c.Set("audit_resource", "license_product_schema")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.productSvc.UpdateConstraintSchema(id, req.ConstraintSchema); err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidConstraintSchema) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated product constraint schema")
	handler.OK(c, nil)
}

func (h *ProductHandler) UpdateStatus(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "product.update")
	c.Set("audit_resource", "license_product_status")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.productSvc.UpdateStatus(id, req.Status); err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "changed product status to "+req.Status)
	handler.OK(c, nil)
}

func (h *ProductHandler) RotateKey(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "product.rotateKey")
	c.Set("audit_resource", "license_product_key")
	c.Set("audit_resource_id", c.Param("id"))

	key, err := h.productSvc.RotateKey(id)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, licensecrypto.ErrNoEncryptionKey) {
			handler.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "rotated product key to version "+strconv.Itoa(key.Version))
	handler.OK(c, key.ToResponse())
}

func (h *ProductHandler) GetPublicKey(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	key, err := h.productSvc.GetPublicKey(id)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, key.ToResponse())
}

func (h *ProductHandler) RotateKeyImpact(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	impact, err := h.licenseSvc.AssessKeyRotationImpact(id)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) || errors.Is(err, domain.ErrProductKeyNotFound) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, impact)
}

type BulkReissueRequest struct {
	LicenseIDs []uint `json:"licenseIds"`
}

func (h *ProductHandler) BulkReissue(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req BulkReissueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ := c.Get("userId")

	c.Set("audit_action", "product.bulkReissue")
	c.Set("audit_resource", "license_product_key")
	c.Set("audit_resource_id", c.Param("id"))

	reissued, err := h.licenseSvc.BulkReissueLicenses(id, req.LicenseIDs, userID.(uint))
	if err != nil {
		if errors.Is(err, domain.ErrBulkReissueTooMany) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrProductKeyNotFound) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", fmt.Sprintf("bulk reissued %d licenses", reissued))
	handler.OK(c, gin.H{"reissued": reissued})
}

// --- PlanHandler ---

type PlanHandler struct {
	planSvc *PlanService
}

func NewPlanHandler(i do.Injector) (*PlanHandler, error) {
	return &PlanHandler{
		planSvc: do.MustInvoke[*PlanService](i),
	}, nil
}

func (h *PlanHandler) Create(c *gin.Context) {
	productID, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid product id")
		return
	}

	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "plan.create")
	c.Set("audit_resource", "license_plan")

	plan, err := h.planSvc.CreatePlan(productID, req.Name, req.ConstraintValues, req.SortOrder)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) || errors.Is(err, ErrPlanNameExists) || errors.Is(err, ErrInvalidConstraintValues) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(plan.ID)))
	c.Set("audit_summary", "created plan: "+plan.Name)
	handler.OK(c, plan.ToResponse())
}

func (h *PlanHandler) Update(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "plan.update")
	c.Set("audit_resource", "license_plan")
	c.Set("audit_resource_id", c.Param("id"))

	plan, err := h.planSvc.UpdatePlan(id, req.Name, req.ConstraintValues, req.SortOrder)
	if err != nil {
		if errors.Is(err, ErrPlanNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrPlanNameExists) || errors.Is(err, ErrInvalidConstraintValues) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated plan: "+plan.Name)
	handler.OK(c, plan.ToResponse())
}

func (h *PlanHandler) Delete(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "plan.delete")
	c.Set("audit_resource", "license_plan")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.planSvc.DeletePlan(id); err != nil {
		if errors.Is(err, ErrPlanNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted plan")
	handler.OK(c, nil)
}

func (h *PlanHandler) SetDefault(c *gin.Context) {
	id, err := domain.ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req SetDefaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "plan.setDefault")
	c.Set("audit_resource", "license_plan_default")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.planSvc.SetDefaultPlan(id, req.IsDefault); err != nil {
		if errors.Is(err, ErrPlanNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "set plan default: "+strconv.FormatBool(req.IsDefault))
	handler.OK(c, nil)
}

// --- Helpers ---
