package ticket

import (
	"encoding/json"
	"errors"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/handler"
)

// VariableHandler exposes process variable APIs.
type VariableHandler struct {
	svc *VariableService
}

func NewVariableHandler(i do.Injector) (*VariableHandler, error) {
	svc := do.MustInvoke[*VariableService](i)
	return &VariableHandler{svc: svc}, nil
}

// List returns all process variables for a ticket.
// GET /api/v1/itsm/tickets/:id/variables
func (h *VariableHandler) List(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid ticket id")
		return
	}

	vars, err := h.svc.ListByTicket(uint(ticketID))
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]ProcessVariableResponse, len(vars))
	for i, v := range vars {
		result[i] = v.ToResponse()
	}
	handler.OK(c, result)
}

// Update modifies a single process variable by key (admin only).
// PUT /api/v1/itsm/tickets/:id/variables/:key
func (h *VariableHandler) Update(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid ticket id")
		return
	}

	key := c.Param("key")
	if key == "" {
		handler.Fail(c, http.StatusBadRequest, "variable key is required")
		return
	}

	var req struct {
		Value     any    `json:"value"`
		ValueType string `json:"valueType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing variable (root scope default)
	existing, err := h.svc.GetVariable(uint(ticketID), "root", key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handler.Fail(c, http.StatusNotFound, "variable not found")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Determine value type (keep existing if not provided)
	valueType := existing.ValueType
	if req.ValueType != "" {
		valueType = req.ValueType
	}

	// Validate value matches type
	serialized := SerializeValue(req.Value)
	if err := validateValueType(serialized, valueType); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetUint("userId")
	existing.Value = serialized
	existing.ValueType = valueType
	existing.Source = fmt.Sprintf("manual:%d", userID)

	if err := h.svc.SetVariable(nil, existing); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, existing.ToResponse())
}

// validateValueType checks that the serialized value is consistent with the declared type.
func validateValueType(value, valueType string) error {
	if value == "" {
		return nil
	}
	switch valueType {
	case ValueTypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("value is not a valid number")
		}
	case ValueTypeBoolean:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("value is not a valid boolean")
		}
	case ValueTypeJSON:
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("value is not valid JSON")
		}
	}
	return nil
}
