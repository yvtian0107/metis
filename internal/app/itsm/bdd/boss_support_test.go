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
const bossCollaborationSpec = `员工在 IT 服务台提交高风险变更协同申请时，服务台需要确认申请主题、申请类别、风险等级、期望完成时间、变更窗口、影响范围、回滚要求、影响模块，以及每一项变更明细。
申请类别包括生产变更、访问授权和应急支持；风险等级包括低、中、高；回滚要求包括需要和不需要；影响模块可选择网关、支付、监控和订单。变更明细需要说明系统、资源、权限级别、生效时段和变更理由，权限级别包括只读和读写。
申请提交后，先交给总部处理人处理；总部处理人完成后，再交给信息部运维管理员处理。运维管理员完成处理后流程结束。`

const bossFormSchema = `{"version":1,"fields":[{"key":"subject","type":"text","label":"申请主题"},{"key":"request_category","type":"select","label":"申请类别","options":[{"label":"生产变更","value":"prod_change"},{"label":"访问授权","value":"access_grant"},{"label":"应急支持","value":"emergency_support"}]},{"key":"risk_level","type":"radio","label":"风险等级","options":[{"label":"低","value":"low"},{"label":"中","value":"medium"},{"label":"高","value":"high"}]},{"key":"expected_finish_time","type":"datetime","label":"期望完成时间"},{"key":"change_window","type":"date_range","label":"变更窗口"},{"key":"impact_scope","type":"textarea","label":"影响范围"},{"key":"rollback_required","type":"select","label":"回滚要求","options":[{"label":"需要","value":"required"},{"label":"不需要","value":"not_required"}]},{"key":"impact_modules","type":"multi_select","label":"影响模块","options":[{"label":"网关","value":"gateway"},{"label":"支付","value":"payment"},{"label":"监控","value":"monitoring"},{"label":"订单","value":"order"}]},{"key":"change_items","type":"table","label":"变更明细表","props":{"columns":[{"key":"system","type":"text","label":"系统"},{"key":"resource","type":"text","label":"资源"},{"key":"permission_level","type":"select","label":"权限级别","options":[{"label":"只读","value":"read"},{"label":"读写","value":"read_write"}]},{"key":"effective_range","type":"date_range","label":"生效时段"},{"key":"reason","type":"text","label":"变更理由"}]}}]}`

const bossGenerationContext = `

## 已有申请表单契约
该服务已经配置申请确认表单。生成参考路径时必须复用这些字段 key、类型和选项值；引用表单字段时必须使用 form.<key>。

- 申请主题: key=` + "`subject`" + `, type=` + "`text`" + `
- 申请类别: key=` + "`request_category`" + `, type=` + "`select`" + `, options=` + "`prod_change/access_grant/emergency_support`" + `
- 风险等级: key=` + "`risk_level`" + `, type=` + "`radio`" + `, options=` + "`low/medium/high`" + `
- 期望完成时间: key=` + "`expected_finish_time`" + `, type=` + "`datetime`" + `
- 变更窗口: key=` + "`change_window`" + `, type=` + "`date_range`" + `
- 影响范围: key=` + "`impact_scope`" + `, type=` + "`textarea`" + `
- 回滚要求: key=` + "`rollback_required`" + `, type=` + "`select`" + `, options=` + "`required/not_required`" + `
- 影响模块: key=` + "`impact_modules`" + `, type=` + "`multi_select`" + `, options=` + "`gateway/payment/monitoring/order`" + `
- 变更明细表: key=` + "`change_items`" + `, type=` + "`table`" + `, columns=` + "`system/resource/permission_level/effective_range/reason`" + `；permission_level options=` + "`read/read_write`" + `

## 按需查询到的组织上下文
以下组织结构映射来自本次按需工具查询。生成人工处理节点参与人时，特定部门中的特定岗位使用 position_department，并填入 department_code 与 position_code；不要输出具体用户。

- 参与人解析：department_hint=` + "`总部`" + `, position_hint=` + "`总部处理人`" + `
  - 候选：type=` + "`position_department`" + `, department_code=` + "`headquarters`" + `（总部）, position_code=` + "`serial_reviewer`" + `（总部处理人）, candidate_count=1
- 参与人解析：department_hint=` + "`信息部`" + `, position_hint=` + "`运维管理员`" + `
  - 候选：type=` + "`position_department`" + `, department_code=` + "`it`" + `（信息部）, position_code=` + "`ops_admin`" + `（运维管理员）, candidate_count=1

## 高风险变更串行处理约束
参考路径必须表达申请人表单、总部处理人、信息部运维管理员和公共结束节点。总部处理人完成后才能进入信息部运维管理员；两级人工节点的 rejected 出边都应指向公共结束节点，不能退回申请人补充。
`

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
			"request_category":     "prod_change",
			"risk_level":           "high",
			"expected_finish_time": "2026-05-01 12:00",
			"change_window":        []string{"2026-04-30 22:00", "2026-05-01 02:00"},
			"impact_scope":         "支付业务全链路",
			"rollback_required":    "required",
			"impact_modules":       []string{"gateway", "payment"},
			"change_items": []map[string]any{
				{
					"system":           "payment-gateway",
					"resource":         "pgw-admin",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-04-30 22:00", "2026-05-01 02:00"},
					"reason":           "升级网关核心路由规则",
				},
				{
					"system":           "payment-core",
					"resource":         "pay-readonly",
					"permission_level": "read",
					"effective_range":  []string{"2026-04-30 22:00", "2026-05-01 02:00"},
					"reason":           "只读查看支付日志验证切换结果",
				},
			},
		},
	},
	"requester-2": {
		Summary: "生产变更申请 - 监控系统告警规则调整",
		FormData: map[string]any{
			"subject":              "监控系统告警规则调整",
			"request_category":     "prod_change",
			"risk_level":           "high",
			"expected_finish_time": "2026-05-02 10:00",
			"change_window":        []string{"2026-05-01 20:00", "2026-05-01 23:00"},
			"impact_scope":         "监控告警全链路",
			"rollback_required":    "not_required",
			"impact_modules":       []string{"monitoring", "order"},
			"change_items": []map[string]any{
				{
					"system":           "monitor-center",
					"resource":         "monitor-admin",
					"permission_level": "read_write",
					"effective_range":  []string{"2026-05-01 20:00", "2026-05-01 23:00"},
					"reason":           "调整P0级告警阈值和通知策略",
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
		userMsg := svc.BuildUserMessage(bossCollaborationSpec, bossGenerationContext, lastErrors)

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
		IntakeFormSchema:  JSONField(bossFormSchema),
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}
