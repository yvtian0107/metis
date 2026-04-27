package ticket

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type TimelineRepo struct {
	db *database.DB
}

func NewTimelineRepo(i do.Injector) (*TimelineRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &TimelineRepo{db: db}, nil
}

func (r *TimelineRepo) Create(t *TicketTimeline) error {
	return r.db.Create(t).Error
}

func (r *TimelineRepo) CreateInTx(tx *gorm.DB, t *TicketTimeline) error {
	return tx.Create(t).Error
}

func (r *TimelineRepo) ListByTicket(ticketID uint) ([]TicketTimeline, error) {
	var items []TicketTimeline
	if err := r.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
