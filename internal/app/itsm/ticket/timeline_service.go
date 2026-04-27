package ticket

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"
)

type TimelineService struct {
	repo *TimelineRepo
}

func NewTimelineService(i do.Injector) (*TimelineService, error) {
	repo := do.MustInvoke[*TimelineRepo](i)
	return &TimelineService{repo: repo}, nil
}

func (s *TimelineService) Record(ticketID uint, operatorID uint, eventType, message string, details JSONField) error {
	tl := &TicketTimeline{
		TicketID:   ticketID,
		OperatorID: operatorID,
		EventType:  eventType,
		Message:    message,
		Details:    details,
	}
	return s.repo.Create(tl)
}

func (s *TimelineService) ListByTicket(ticketID uint) ([]TicketTimeline, error) {
	return s.repo.ListByTicket(ticketID)
}
