package itsm

// db_backup_support_test.go — DB backup whitelist workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the db backup whitelist workflow
// from the collaboration spec, matching the VPN/Server Access BDD approach.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// dbBackupCollaborationSpec is the collaboration spec for the production database backup whitelist temporary access service.
const dbBackupCollaborationSpec = `这是一个数据库备份白名单临时放行服务。
用户来找服务台时，要先把目标数据库、来源 IP、放行时间窗和申请原因这些信息问清楚，再整理成可以确认的申请摘要。
信息收集完成后，你需要先调用预检动作（precheck）验证参数合法性，根据预检结果决定下一步。
预检使用后，交给信息部的数据库管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 db_admin。
处理完成后，你需要调用放行动作（apply）完成实际的白名单配置。放行动作执行成功后，流程立即结束，不再创建任何新的处理、处理或通知活动。
不要让申请人在表单里自己选择处理类别，流程决策智能体应根据上下文在运行时决定。
不需要额外生成取消分支。`

// dbBackupCasePayload defines test data for a single db backup whitelist BDD scenario.
type dbBackupCasePayload struct {
	Summary     string
	FormData    map[string]any
	TicketAlias string // alias for multi-ticket scenarios
}

// dbBackupCasePayloads provides 2 test case payloads for the BDD scenarios.
var dbBackupCasePayloads = map[string]dbBackupCasePayload{
	"requester-1": {
		Summary: "数据库备份白名单临时放行申请：需要从应用服务器访问生产数据库做数据备份。",
		FormData: map[string]any{
			"database_name":    "prod-mysql-01",
			"source_ip":        "10.20.30.50",
			"whitelist_window": "今晚 22:00 到 23:00",
			"access_reason":    "应用服务器需要在维护窗口内做全量数据备份。",
		},
	},
	"requester-2": {
		Summary: "数据库备份白名单临时放行申请：需要从跳板机访问生产数据库做增量备份。",
		FormData: map[string]any{
			"database_name":    "prod-postgres-02",
			"source_ip":        "10.20.30.51",
			"whitelist_window": "明早 02:00 到 04:00",
			"access_reason":    "跳板机发起增量备份，需要临时放行白名单。",
		},
	},
}

// generateDbBackupWorkflow calls the LLM to generate a db backup whitelist workflow JSON
// from the collaboration spec. Same pattern as generateVPNWorkflow.
func generateDbBackupWorkflow(cfg llmConfig) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.buildUserMessage(dbBackupCollaborationSpec, "", lastErrors)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, err := client.Chat(ctx, llm.ChatRequest{
			Model: cfg.model,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: itsmGeneratorSystemPrompt},
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

		workflowJSON, extractErr := extractJSON(resp.Content)
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

// publishDbBackupSmartService creates the full service configuration for db backup whitelist BDD tests:
// ServiceCatalog + Priority + Agent + ServiceDefinition + 2 ServiceActions (precheck + apply).
func publishDbBackupSmartService(bc *bddContext) error {
	// 0. Initialize LocalActionReceiver (if not already done).
	if bc.actionReceiver == nil {
		bc.actionReceiver = NewLocalActionReceiver()
	}

	// 1. Generate workflow via LLM
	workflowJSON, err := generateDbBackupWorkflow(bc.llmCfg)
	if err != nil {
		return fmt.Errorf("generate db backup workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "数据库服务",
		Code:     "db-services",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "紧急",
		Code:     "urgent-db",
		Value:    2,
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

	// 5. ServiceDefinition with engine_type=smart
	svc := &ServiceDefinition{
		Name:              "数据库备份白名单临时放行",
		Code:              "db-backup-whitelist",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: dbBackupCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	// 6. Create 2 ServiceActions: precheck + apply (URLs point to LocalActionReceiver)
	precheckConfig, _ := json.Marshal(engine.ActionConfig{
		URL:     bc.actionReceiver.URL("/precheck"),
		Method:  "POST",
		Body:    `{"ticket_code":"{{ticket.code}}","database":"{{ticket.form_data.database_name}}","source_ip":"{{ticket.form_data.source_ip}}"}`,
		Timeout: 10,
		Retries: 0, // no retries in test
	})
	precheckAction := &ServiceAction{
		Name:        "数据库备份白名单预检",
		Code:        "db_backup_whitelist_precheck",
		Description: "验证数据库备份白名单放行参数的合法性",
		ActionType:  "http",
		ConfigJSON:  JSONField(precheckConfig),
		ServiceID:   svc.ID,
		IsActive:    true,
	}
	if err := bc.db.Create(precheckAction).Error; err != nil {
		return fmt.Errorf("create precheck action: %w", err)
	}
	bc.serviceActions["db_backup_whitelist_precheck"] = precheckAction

	applyConfig, _ := json.Marshal(engine.ActionConfig{
		URL:     bc.actionReceiver.URL("/apply"),
		Method:  "POST",
		Body:    `{"ticket_code":"{{ticket.code}}","database":"{{ticket.form_data.database_name}}","whitelist_window":"{{ticket.form_data.whitelist_window}}"}`,
		Timeout: 10,
		Retries: 0,
	})
	applyAction := &ServiceAction{
		Name:        "数据库备份白名单放行",
		Code:        "db_backup_whitelist_apply",
		Description: "执行数据库备份白名单放行配置",
		ActionType:  "http",
		ConfigJSON:  JSONField(applyConfig),
		ServiceID:   svc.ID,
		IsActive:    true,
	}
	if err := bc.db.Create(applyAction).Error; err != nil {
		return fmt.Errorf("create apply action: %w", err)
	}
	bc.serviceActions["db_backup_whitelist_apply"] = applyAction

	// 7. Re-wire engines with syncActionSubmitter (actionReceiver is now set).
	orgSvc := &testOrgService{db: bc.db}
	resolver := engine.NewParticipantResolver(orgSvc)
	bc.engine = engine.NewClassicEngine(resolver, &noopSubmitter{}, nil)
	submitter := &syncActionSubmitter{db: bc.db, classicEngine: bc.engine}
	executor := &testDecisionExecutor{db: bc.db, llmCfg: bc.llmCfg}
	userProvider := &testUserProvider{db: bc.db}
	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, submitter, &bddConfigProvider{bc: bc})
	bc.smartEngine.SetActionExecutor(engine.NewActionExecutor(bc.db))

	return nil
}
