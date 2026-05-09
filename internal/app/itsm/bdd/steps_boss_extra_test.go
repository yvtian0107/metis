package bdd

// steps_boss_extra_test.go — additional step definitions for Boss BDD scenarios
// covering: secondary gate rejection, admin cancel, form field assertion,
// timeline completeness, applicant self-process, and secondary gate de-duplication.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
)

// ---------------------------------------------------------------------------
// Given steps
// ---------------------------------------------------------------------------

// givenBossTicketCreatedNewCases handles the new case keys introduced for boss_main_paths.feature
// (access-grant-1, emergency-1, multi-module-1) and delegates requester-1/requester-2 to the
// existing boss_support_test.go handler.
func (bc *bddContext) givenBossTicketCreatedExtended(username, caseKey string) error {
	// Delegate existing keys to original handler.
	if _, ok := bossCasePayloads[caseKey]; ok {
		return bc.givenBossTicketCreated(username, caseKey)
	}

	// Handle new keys.
	payload, ok := bossExtendedCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown boss case key %q", caseKey)
	}

	return bc.createBossTicket(username, fmt.Sprintf("BOSS-%s", caseKey), payload.Summary, payload.FormData, bc.service.WorkflowJSON)
}

// givenBossTicketCreatedExtendedAlias handles the aliased variant from boss_main_paths.feature.
func (bc *bddContext) givenBossTicketCreatedExtendedAlias(username, alias, caseKey string) error {
	if err := bc.givenBossTicketCreatedExtended(username, caseKey); err != nil {
		return err
	}
	if alias != "" {
		bc.tickets[alias] = bc.ticket
	}
	return nil
}

// givenBossSecondGatePositionInactive disables all members of the secondary gate (it/ops_admin).
func (bc *bddContext) givenBossSecondGatePositionInactive() error {
	return bc.givenBossPositionInactive("it", "ops_admin")
}

// ---------------------------------------------------------------------------
// When steps
// ---------------------------------------------------------------------------

// whenAdminCancelsBossTicket calls SmartEngine.Cancel to simulate an admin cancellation for Boss tickets.
func (bc *bddContext) whenAdminCancelsBossTicket(reason string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bc.smartEngine.Cancel(ctx, bc.db, engine.CancelParams{
		TicketID:   bc.ticket.ID,
		Reason:     reason,
		OperatorID: 1, // admin
	}); err != nil {
		return fmt.Errorf("admin cancel ticket: %w", err)
	}

	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// ---------------------------------------------------------------------------
// Then steps
// ---------------------------------------------------------------------------

// thenTicketFormDataContainsFields checks that the ticket form_data JSON includes
// all the specified comma-separated field keys with non-empty values.
func (bc *bddContext) thenTicketFormDataContainsFields(rawKeys string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}

	var formData map[string]any
	if err := json.Unmarshal([]byte(bc.ticket.FormData), &formData); err != nil {
		return fmt.Errorf("parse form_data: %w", err)
	}

	for _, rawKey := range strings.Split(rawKeys, ",") {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		val, ok := formData[key]
		if !ok {
			keys := make([]string, 0, len(formData))
			for k := range formData {
				keys = append(keys, k)
			}
			return fmt.Errorf("form_data missing key %q; got: %v", key, keys)
		}
		// Check non-nil and non-empty-string.
		switch v := val.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("form_data[%q] is empty string", key)
			}
		case nil:
			return fmt.Errorf("form_data[%q] is nil", key)
		}
	}
	return nil
}

// thenTimelineAtLeastNEvents checks that the ticket's timeline contains at least n events.
func (bc *bddContext) thenTimelineAtLeastNEvents(minCount int) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).Where("ticket_id = ?", bc.ticket.ID).Count(&count).Error; err != nil {
		return fmt.Errorf("count timeline: %w", err)
	}
	if int(count) < minCount {
		return fmt.Errorf("expected at least %d timeline events, got %d", minCount, count)
	}
	return nil
}

// thenApplicantClaimShouldFail asserts that the ticket requester cannot claim/process the current activity.
func (bc *bddContext) thenApplicantClaimShouldFail() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	// Find the requester username.
	var requesterUsername string
	for username, user := range bc.usersByName {
		if user.ID == bc.ticket.RequesterID {
			requesterUsername = username
			break
		}
	}
	if requesterUsername == "" {
		// Fallback: use boss-requester-1.
		requesterUsername = "boss-requester-1"
	}
	return bc.thenClaimShouldFail(requesterUsername)
}

// thenActiveSecondGateTaskCount asserts the active task count for the secondary gate.
func (bc *bddContext) thenActiveSecondGateTaskCount(expected int) error {
	return bc.thenActiveProcessActivityCountForDepartmentPositionIs("it", "ops_admin", expected)
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerBossExtraSteps(sc *godog.ScenarioContext, bc *bddContext) {
	// Overrides givenBossTicketCreated to handle new case keys transparently.
	// The original `^"([^"]*)" 已创建高风险变更工单，场景为 "([^"]*)"$` is already
	// registered in registerBossSteps. We add an extended variant for new case keys.
	sc.Given(`^"([^"]*)" 已创建高风险变更扩展工单，场景为 "([^"]*)"$`, bc.givenBossTicketCreatedExtended)
	sc.Given(`^"([^"]*)" 已创建高风险变更扩展工单 "([^"]*)"，场景为 "([^"]*)"$`, bc.givenBossTicketCreatedExtendedAlias)
	sc.Given(`^次关岗位 "it/ops_admin" 处理人已停用$`, bc.givenBossSecondGatePositionInactive)

	sc.When(`^管理员取消当前工单，原因为 "([^"]*)"$`, bc.whenAdminCancelsBossTicket)

	sc.Then(`^工单表单数据包含字段 "([^"]*)"$`, bc.thenTicketFormDataContainsFields)
	sc.Then(`^时间线至少包含 (\d+) 个事件$`, bc.thenTimelineAtLeastNEvents)
	sc.Then(`^申请人不能认领当前工单$`, bc.thenApplicantClaimShouldFail)
	sc.Then(`^次关活跃处理任务数为 (\d+)$`, bc.thenActiveSecondGateTaskCount)
}

// ---------------------------------------------------------------------------
// Extended case payloads (new scenarios in boss_main_paths.feature)
// ---------------------------------------------------------------------------

// bossExtendedCasePayloads supplements bossCasePayloads with new scenario data.
var bossExtendedCasePayloads = map[string]bossCasePayload{
	// BS-002: access_grant category
	"access-grant-1": {
		Summary: "订单监控平台临时访问授权",
		FormData: map[string]any{
			"subject":              "订单监控平台临时访问授权",
			"request_category":     "access_grant",
			"risk_level":           "medium",
			"expected_finish_time": "2026-05-13 18:00",
			"change_window":        []string{"2026-05-13 14:00", "2026-05-13 17:00"},
			"impact_scope":         "订单监控大盘、告警排查、链路追踪",
			"rollback_required":    "not_required",
			"impact_modules":       []string{"monitoring", "order"},
			"change_items": []map[string]any{
				{
					"system":           "order-monitor-prod",
					"resource":         "monitor-auditor",
					"permission_level": "read",
					"effective_range":  []string{"2026-05-13 14:00", "2026-05-13 17:00"},
					"reason":           "只读查看订单链路告警、监控图表和异常明细",
				},
				{
					"system":           "order-service-prod",
					"resource":         "order-audit",
					"permission_level": "read",
					"effective_range":  []string{"2026-05-13 14:00", "2026-05-13 17:00"},
					"reason":           "在授权窗口内读取订单服务运行日志并核对异常请求样本",
				},
			},
		},
	},
	// BS-003: emergency_support category
	"emergency-1": {
		Summary: "生产故障紧急支持协同申请",
		FormData: map[string]any{
			"subject":              "生产故障紧急支持协同申请",
			"request_category":     "emergency_support",
			"risk_level":           "high",
			"expected_finish_time": "2026-05-10 23:30",
			"change_window":        []string{"2026-05-10 20:30", "2026-05-10 23:00"},
			"impact_scope":         "生产故障止血、监控恢复、订单恢复校验",
			"rollback_required":    "not_required",
			"impact_modules":       []string{"monitoring", "order", "payment"},
			"change_items": []map[string]any{
				{
					"system":           "monitor-center-prod",
					"resource":         "monitor-admin",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-10 20:30", "2026-05-10 23:00"},
					"reason":           "调整监控告警阈值并临时启用应急通知策略",
				},
				{
					"system":           "order-core-prod",
					"resource":         "order-support",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-10 20:30", "2026-05-10 23:00"},
					"reason":           "执行订单恢复脚本并修正故障窗口内异常任务状态",
				},
				{
					"system":           "payment-reconcile-prod",
					"resource":         "pay-ops",
					"permission_level": "read",
					"effective_range":  []string{"2026-05-10 20:30", "2026-05-10 23:00"},
					"reason":           "只读核对应急恢复后的支付补偿结果",
				},
			},
		},
	},
	// BS-004: multi-module multi-item prod_change
	"multi-module-1": {
		Summary: "核心交易链路联合变更申请",
		FormData: map[string]any{
			"subject":              "核心交易链路联合变更申请",
			"request_category":     "prod_change",
			"risk_level":           "high",
			"expected_finish_time": "2026-05-15 05:00",
			"change_window":        []string{"2026-05-15 00:30", "2026-05-15 04:30"},
			"impact_scope":         "网关接入、支付处理、订单状态流转、监控告警联动",
			"rollback_required":    "required",
			"impact_modules":       []string{"gateway", "payment", "monitoring", "order"},
			"change_items": []map[string]any{
				{
					"system":           "api-gateway-prod",
					"resource":         "gateway-release",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-15 00:30", "2026-05-15 04:30"},
					"reason":           "发布新网关路由配置并切换灰度流量",
				},
				{
					"system":           "payment-engine-prod",
					"resource":         "payment-dba",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-15 00:30", "2026-05-15 04:30"},
					"reason":           "执行支付链路配置切换和连接池参数调整",
				},
				{
					"system":           "order-center-prod",
					"resource":         "order-readonly",
					"permission_level": "read",
					"effective_range":  []string{"2026-05-15 00:30", "2026-05-15 04:30"},
					"reason":           "变更期间只读核对订单状态流转与补偿任务积压",
				},
				{
					"system":           "monitor-hub-prod",
					"resource":         "monitor-admin",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-15 00:30", "2026-05-15 04:30"},
					"reason":           "更新变更专属监控看板与告警收敛规则",
				},
			},
		},
	},
}
