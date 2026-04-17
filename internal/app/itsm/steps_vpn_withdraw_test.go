package itsm

// steps_vpn_withdraw_test.go — BDD step definitions for ticket withdrawal scenarios.

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"gorm.io/gorm"

	"metis/internal/app/itsm/engine"
)

func registerWithdrawSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.When(`^"([^"]*)" 撤回工单，原因为 "([^"]*)"$`, bc.whenWithdrawTicket)
	sc.When(`^"([^"]*)" 认领当前工单$`, bc.whenClaimCurrentTicket)
	sc.Then(`^操作失败$`, bc.thenOperationFailed)
	sc.Then(`^时间线包含 "([^"]*)"$`, bc.thenTimelineContains)
	sc.Then(`^时间线包含撤回记录$`, bc.thenTimelineContainsWithdrawn)
}

// whenWithdrawTicket attempts to withdraw the current ticket as the given user.
// Mirrors TicketService.Withdraw logic: check requester, claimed_at, then engine.Cancel.
func (bc *bddContext) whenWithdrawTicket(username, reason string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("unknown user %q", username)
	}
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Refresh ticket from DB.
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		bc.lastErr = err
		return nil
	}

	// Check requester.
	if bc.ticket.RequesterID != user.ID {
		bc.lastErr = ErrNotRequester
		return nil
	}

	// Check terminal.
	if bc.ticket.IsTerminal() {
		bc.lastErr = ErrTicketTerminal
		return nil
	}

	// Check claimed assignments.
	var claimedCount int64
	bc.db.Model(&TicketAssignment{}).
		Where("ticket_id = ? AND claimed_at IS NOT NULL", bc.ticket.ID).
		Count(&claimedCount)
	if claimedCount > 0 {
		bc.lastErr = ErrTicketClaimed
		return nil
	}

	// Delegate to engine.Cancel with withdrawn event type.
	msg := "工单已撤回"
	if reason != "" {
		msg = "工单已撤回: " + reason
	}
	err := bc.db.Transaction(func(tx *gorm.DB) error {
		return bc.engine.Cancel(context.Background(), tx, engine.CancelParams{
			TicketID:   bc.ticket.ID,
			Reason:     reason,
			OperatorID: user.ID,
			EventType:  "withdrawn",
			Message:    msg,
		})
	})
	bc.lastErr = err
	return nil
}

// whenClaimCurrentTicket claims the current ticket's active activity for the given user.
func (bc *bddContext) whenClaimCurrentTicket(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("unknown user %q", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	now := time.Now()
	if err := bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", activity.ID).
		Updates(map[string]any{
			"assignee_id": user.ID,
			"claimed_at":  now,
		}).Error; err != nil {
		return fmt.Errorf("claim assignment: %w", err)
	}

	return nil
}

// thenOperationFailed asserts that the last operation produced an error.
func (bc *bddContext) thenOperationFailed() error {
	if bc.lastErr == nil {
		return fmt.Errorf("expected operation to fail, but it succeeded")
	}
	return nil
}

// thenTimelineContains asserts that the ticket timeline contains a message with the given text.
func (bc *bddContext) thenTimelineContains(text string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var count int64
	bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND message LIKE ?", bc.ticket.ID, "%"+text+"%").
		Count(&count)
	if count == 0 {
		return fmt.Errorf("expected timeline to contain %q, but no matching entry found", text)
	}
	return nil
}

// thenTimelineContainsWithdrawn asserts that the ticket timeline has a "withdrawn" event.
func (bc *bddContext) thenTimelineContainsWithdrawn() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var count int64
	bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type = ?", bc.ticket.ID, "withdrawn").
		Count(&count)
	if count == 0 {
		return fmt.Errorf("expected timeline to contain withdrawn event, but none found")
	}
	return nil
}
