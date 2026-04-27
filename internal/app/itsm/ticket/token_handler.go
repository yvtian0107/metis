package ticket

import (
	. "metis/internal/app/itsm/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// TokenHandler exposes execution token APIs.
type TokenHandler struct {
	repo      *TokenRepository
	ticketSvc *TicketService
}

func NewTokenHandler(i do.Injector) (*TokenHandler, error) {
	repo := do.MustInvoke[*TokenRepository](i)
	ticketSvc := do.MustInvoke[*TicketService](i)
	return &TokenHandler{repo: repo, ticketSvc: ticketSvc}, nil
}

// List returns all execution tokens for a ticket.
// GET /api/v1/itsm/tickets/:id/tokens
func (h *TokenHandler) List(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid ticket id")
		return
	}

	// Check ticket exists
	if _, err := h.ticketSvc.Get(uint(ticketID)); err != nil {
		handler.Fail(c, http.StatusNotFound, "ticket not found")
		return
	}

	tokens, err := h.repo.ListByTicket(uint(ticketID))
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]ExecutionTokenResponse, len(tokens))
	for i, t := range tokens {
		result[i] = t.ToResponse()
	}
	handler.OK(c, result)
}
