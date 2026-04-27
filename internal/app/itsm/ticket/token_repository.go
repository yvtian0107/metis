package ticket

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

// TokenRepository handles queries for execution tokens.
type TokenRepository struct {
	db *database.DB
}

func NewTokenRepository(i do.Injector) (*TokenRepository, error) {
	db := do.MustInvoke[*database.DB](i)
	return &TokenRepository{db: db}, nil
}

// ListByTicket returns all tokens for a given ticket, ordered by creation time.
func (r *TokenRepository) ListByTicket(ticketID uint) ([]ExecutionToken, error) {
	var tokens []ExecutionToken
	err := r.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&tokens).Error
	return tokens, err
}
