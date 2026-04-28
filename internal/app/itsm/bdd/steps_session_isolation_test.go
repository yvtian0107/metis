package bdd

// steps_session_isolation_test.go — step definitions for session isolation scenarios.

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerSessionIsolationSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.When(`^服务台发起新会话$`, func() error {
		// Save current ticket as previous
		if bc.ticket != nil {
			bc.dialogState.previousTickets = append(bc.dialogState.previousTickets, bc.ticket)
		}
		// Reset dialog state for new session (but keep previousTickets)
		prev := bc.dialogState.previousTickets
		bc.dialogState = dialogTestState{
			currentUserID:   bc.dialogState.currentUserID,
			currentUsername: bc.dialogState.currentUsername,
			previousTickets: prev,
		}
		return nil
	})

	sc.Then(`^新工单与前一张工单不同$`, func() error {
		if bc.ticket == nil {
			return fmt.Errorf("no current ticket")
		}
		if len(bc.dialogState.previousTickets) == 0 {
			return fmt.Errorf("no previous tickets")
		}
		prevTicket := bc.dialogState.previousTickets[len(bc.dialogState.previousTickets)-1]
		if bc.ticket.ID == prevTicket.ID {
			return fmt.Errorf("current ticket ID %d is the same as previous ticket ID %d", bc.ticket.ID, prevTicket.ID)
		}
		return nil
	})

	sc.Then(`^两张工单互不关联$`, func() error {
		if bc.ticket == nil || len(bc.dialogState.previousTickets) == 0 {
			return fmt.Errorf("need both current and previous tickets")
		}
		// Verify no ticket links between them
		prevTicket := bc.dialogState.previousTickets[len(bc.dialogState.previousTickets)-1]
		var linkCount int64
		bc.db.Table("itsm_ticket_links").
			Where("(parent_ticket_id = ? AND child_ticket_id = ?) OR (parent_ticket_id = ? AND child_ticket_id = ?)",
				bc.ticket.ID, prevTicket.ID, prevTicket.ID, bc.ticket.ID).
			Count(&linkCount)
		if linkCount > 0 {
			return fmt.Errorf("expected no links between tickets, found %d", linkCount)
		}
		return nil
	})

	sc.When(`^用户发送 "([^"]*)" 重置指令$`, func(command string) error {
		if command == "new_request" {
			bc.dialogState.messages = nil
			bc.dialogState.dialogMode = ""
		}
		return nil
	})

	sc.Then(`^服务台会话状态已重置$`, func() error {
		if len(bc.dialogState.messages) != 0 {
			return fmt.Errorf("expected empty messages after reset, got %d", len(bc.dialogState.messages))
		}
		return nil
	})

	sc.Then(`^服务台识别出新的请求意图$`, func() error {
		if len(bc.dialogState.messages) == 0 {
			return fmt.Errorf("expected messages after new input")
		}
		return nil
	})
}
