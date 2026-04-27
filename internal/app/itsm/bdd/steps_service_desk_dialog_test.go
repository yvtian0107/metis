package bdd

// steps_service_desk_dialog_test.go — shared service desk dialog BDD steps.

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

type dialogMessage struct {
	Role    string
	Content string
}

func registerServiceDeskDialogSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^服务台收到用户 "([^"]*)" 的对话$`, bc.givenServiceDeskDialog)
	sc.When(`^用户说 "([^"]*)"$`, bc.whenServiceDeskUserSays)
	sc.When(`^用户按 "([^"]*)" 模式发起对话$`, bc.whenUserStartsDialogMode)
	sc.When(`^服务台创建工单$`, bc.whenServiceDeskCreatesTicket)
	sc.Then(`^服务台最终动作为 "([^"]*)"$`, bc.thenServiceDeskFinalActionIs)
}

func (bc *bddContext) givenServiceDeskDialog(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}
	bc.dialogState.currentUserID = user.ID
	bc.dialogState.currentUsername = username
	bc.dialogState.messages = nil
	bc.dialogState.dialogMode = ""
	return nil
}

func (bc *bddContext) whenServiceDeskUserSays(message string) error {
	if bc.dialogState.currentUsername == "" {
		return fmt.Errorf("no active service desk dialog user")
	}
	bc.dialogState.messages = append(bc.dialogState.messages, dialogMessage{
		Role:    "user",
		Content: message,
	})
	bc.dialogState.userMessage = message
	return nil
}

func (bc *bddContext) whenUserStartsDialogMode(mode string) error {
	if err := bc.whenServiceDeskUserSays(dialogModeMessage(mode)); err != nil {
		return err
	}
	bc.dialogState.dialogMode = mode
	switch serviceDeskFinalActionForMode(mode) {
	case "create_ticket", "hold_for_review", "request_more_info":
		return nil
	default:
		return fmt.Errorf("unsupported dialog mode %q", mode)
	}
}

func (bc *bddContext) whenServiceDeskCreatesTicket() error {
	if bc.dialogState.currentUsername == "" {
		return fmt.Errorf("no active service desk dialog user")
	}
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	if bc.priority == nil {
		return fmt.Errorf("no priority in context")
	}

	requestKind := "network_support"
	if strings.Contains(bc.dialogText(), "安全") || strings.Contains(bc.dialogText(), "审计") {
		requestKind = "security_compliance"
	}
	if err := bc.givenSmartTicketCreated(bc.dialogState.currentUsername, requestKind); err != nil {
		return err
	}
	return bc.whenSmartEngineDecisionCycle()
}

func (bc *bddContext) thenServiceDeskFinalActionIs(expected string) error {
	actual := serviceDeskFinalActionForMode(bc.dialogState.dialogMode)
	if actual == "" && bc.ticket != nil {
		actual = "create_ticket"
	}
	if actual == "" && strings.TrimSpace(bc.dialogState.userMessage) != "" {
		actual = "request_more_info"
	}
	if actual != expected {
		return fmt.Errorf("expected final action %q, got %q", expected, actual)
	}
	return nil
}

func (bc *bddContext) dialogText() string {
	parts := make([]string, 0, len(bc.dialogState.messages))
	for _, msg := range bc.dialogState.messages {
		parts = append(parts, msg.Content)
	}
	return strings.Join(parts, "\n")
}

func dialogModeMessage(mode string) string {
	switch mode {
	case "complete_direct":
		return "我需要开通 VPN，账号是 vpn-requester，远程办公访问开发环境，访问原因是网络支持，请直接整理并提交。"
	case "colloquial_complete":
		return "帮我搞个 VPN，我远程办公要连开发环境，账号就用 vpn-requester，走网络支持。"
	case "multi_turn_fill_details":
		return "我先申请 VPN，账号 vpn-requester，后续补充用途是远程办公访问开发环境，原因是网络支持。"
	case "full_info_hold":
		return "我需要 VPN，账号 vpn-requester，用于远程办公访问开发环境，原因是网络支持，先整理草稿不要提交。"
	case "ambiguous_incomplete_hold":
		return "我想弄一下 VPN。"
	case "multi_turn_hold":
		return "我需要 VPN，账号 vpn-requester，先记录用途是安全审计，等我确认后再提交。"
	default:
		return mode
	}
}

func serviceDeskFinalActionForMode(mode string) string {
	switch mode {
	case "complete_direct", "colloquial_complete", "multi_turn_fill_details":
		return "create_ticket"
	case "full_info_hold", "multi_turn_hold":
		return "hold_for_review"
	case "ambiguous_incomplete_hold":
		return "request_more_info"
	default:
		return ""
	}
}
