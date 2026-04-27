package bdd

// boss_support_test.go — Boss serial process workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the Boss workflow
// from the collaboration spec, matching the VPN/DB Backup BDD approach.

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	"time"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// bossCollaborationSpec is the collaboration spec for the Boss high-risk change request service.
// Based on seed.go, with strengthened serial process ordering and completion wording.
const bossCollaborationSpec = `用户在 IT 服务台提交高风险变更协同申请。服务台需要收集申请主题、申请类别、风险等级、期望完成时间、变更开始时间、变更结束时间、影响范围、回滚要求、影响模块以及变更明细表。
申请类别必须支持：生产变更(prod_change)、访问授权(access_grant)、应急支持(emergency_support)。
风险等级必须支持：低(low)、中(medium)、高(high)。
回滚要求必须支持：需要(required)、不需要(not_required)。
影响模块必须支持多选：网关(gateway)、支付(payment)、监控(monitoring)、订单(order)。
变更明细表至少包含系统、资源、权限级别、生效时段、变更理由。权限级别必须支持：只读(read)、读写(read_write)。
申请提交后，先交给指定用户 serial-reviewer 处理，处理参与者类型必须使用 user。首级处理完成后才能安排二级处理。
serial-reviewer 处理完成后，再交给信息部的运维管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 ops_admin。
运维管理员处理完成后，流程必须立即结束，不再创建任何新的处理、处理或通知活动。不要生成取消分支。`

// bossCasePayload defines test data for a single Boss BDD scenario.
type bossCasePayload struct {
	Summary  string
	FormData map[string]any
}

// bossCasePayloads provides 2 test case payloads for the Boss BDD scenarios.
var bossCasePayloads = map[string]bossCasePayload{
	"requester-1": {
		Summary: "生产变更申请 - 支付系统网关升级",
		FormData: map[string]any{
			"subject":              "支付系统网关升级",
			"category":             "prod_change",
			"risk_level":           "high",
			"expected_completion":  "2026-05-01",
			"change_start":         "2026-04-30 22:00",
			"change_end":           "2026-05-01 02:00",
			"impact_scope":         "支付业务全链路",
			"rollback_requirement": "required",
			"impact_module":        []string{"gateway", "payment"},
			"resource_items": []map[string]any{
				{
					"system_name":      "payment-gateway",
					"resource_account": "pgw-admin",
					"permission_level": "read_write",
					"target_operation": "升级网关核心路由规则",
				},
				{
					"system_name":      "payment-core",
					"resource_account": "pay-readonly",
					"permission_level": "read",
					"target_operation": "只读查看支付日志验证切换结果",
				},
			},
		},
	},
	"requester-2": {
		Summary: "生产变更申请 - 监控系统告警规则调整",
		FormData: map[string]any{
			"subject":              "监控系统告警规则调整",
			"category":             "prod_change",
			"risk_level":           "high",
			"expected_completion":  "2026-05-02",
			"change_start":         "2026-05-01 20:00",
			"change_end":           "2026-05-01 23:00",
			"impact_scope":         "监控告警全链路",
			"rollback_requirement": "not_required",
			"impact_module":        []string{"monitoring", "order"},
			"resource_items": []map[string]any{
				{
					"system_name":      "monitor-center",
					"resource_account": "monitor-admin",
					"permission_level": "read_write",
					"target_operation": "调整P0级告警阈值和通知策略",
				},
			},
		},
	},
}

// generateBossWorkflow calls the LLM to generate a Boss workflow JSON from the collaboration spec.
func generateBossWorkflow(cfg llmConfig) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.BuildUserMessage(bossCollaborationSpec, "", lastErrors)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, err := client.Chat(ctx, llm.ChatRequest{
			Model: cfg.model,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: PathBuilderSystemPrompt},
				{Role: llm.RoleUser, Content: userMsg},
			},
			MaxTokens: 4096,
		})
		cancel()

		if err != nil {
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("LLM call failed after %d attempts: %w", attempt+1, err)
		}

		workflowJSON, extractErr := ExtractJSON(resp.Content)
		if extractErr != nil {
			lastErrors = []engine.ValidationError{
				{Message: fmt.Sprintf("输出不是有效 JSON: %v", extractErr)},
			}
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("JSON extraction failed after %d attempts: %w", attempt+1, extractErr)
		}

		validationErrors := engine.ValidateWorkflow(workflowJSON)
		var blockingErrors []engine.ValidationError
		for _, e := range validationErrors {
			if !e.IsWarning() {
				blockingErrors = append(blockingErrors, e)
			}
		}

		if len(blockingErrors) == 0 {
			return workflowJSON, nil
		}

		lastErrors = blockingErrors
		if attempt < maxRetries {
			continue
		}

		return nil, fmt.Errorf("workflow validation failed after %d attempts: %v", attempt+1, blockingErrors)
	}

	return nil, fmt.Errorf("workflow generation failed")
}

// publishBossSmartService creates the full service configuration for Boss BDD tests:
// ServiceCatalog + Priority + Agent + ServiceDefinition (no ServiceActions).
func publishBossSmartService(bc *bddContext) error {
	// 1. Generate workflow via LLM
	workflowJSON, err := generateBossWorkflow(bc.llmCfg)
	if err != nil {
		return fmt.Errorf("generate boss workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "变更管理",
		Code:     "change-management",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "高",
		Code:     "high-boss",
		Value:    1,
		Color:    "#f5222d",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	// 4. Seed Agent record (process decision agent)
	agent := &ai.Agent{
		Name:         "流程决策智能体",
		Type:         "assistant",
		IsActive:     true,
		Visibility:   "private",
		Strategy:     "react",
		SystemPrompt: decisionAgentSystemPrompt,
		MaxTokens:    2048,
		MaxTurns:     1,
		CreatedBy:    1,
	}
	if err := bc.db.Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	bc.db.Model(agent).Update("temperature", 0)

	// 5. ServiceDefinition with engine_type=smart (no ServiceActions for Boss)
	svc := &ServiceDefinition{
		Name:              "高风险变更协同申请（Boss）",
		Code:              "boss-serial-change-request",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: bossCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}
