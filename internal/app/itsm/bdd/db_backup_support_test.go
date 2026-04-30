package bdd

// db_backup_support_test.go — DB backup whitelist workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the db backup whitelist workflow
// from the collaboration spec, matching the VPN/Server Access BDD approach.

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

// dbBackupCollaborationSpec is the collaboration spec for the production database backup whitelist temporary access service.
const dbBackupCollaborationSpec = `员工在 IT 服务台申请生产数据库备份白名单临时放行时，服务台需要确认目标数据库、发起备份访问的来源 IP、白名单放行时间窗，以及这次临时放行的申请原因。
申请资料收齐后，系统会先做一次白名单参数预检，确认数据库、来源 IP、放行窗口和申请原因满足放行前置条件。预检通过后，交给信息部数据库管理员处理。
数据库管理员完成处理后，系统执行备份白名单放行；放行成功后流程结束。驳回时不进入补充或返工，流程按驳回结果结束。`

const dbBackupFormSchema = `{"version":1,"fields":[{"key":"database_name","type":"text","label":"目标数据库"},{"key":"source_ip","type":"text","label":"来源 IP"},{"key":"whitelist_window","type":"text","label":"白名单放行时间窗"},{"key":"access_reason","type":"textarea","label":"申请原因"}]}`

const dbBackupGenerationContext = `

## 已有申请表单契约
该服务已经配置申请确认表单。生成参考路径时必须复用这些字段 key、类型和选项值；引用表单字段时必须使用 form.<key>。

- 目标数据库: key=` + "`database_name`" + `, type=` + "`text`" + `
- 来源 IP: key=` + "`source_ip`" + `, type=` + "`text`" + `
- 白名单放行时间窗: key=` + "`whitelist_window`" + `, type=` + "`text`" + `
- 申请原因: key=` + "`access_reason`" + `, type=` + "`textarea`" + `

## 按需查询到的组织上下文
以下组织结构映射来自本次按需工具查询。生成人工处理节点参与人时，特定部门中的特定岗位使用 position_department，并填入 department_code 与 position_code；不要输出具体用户。

- 参与人解析：department_hint=` + "`信息部`" + `, position_hint=` + "`数据库管理员`" + `
  - 候选：type=` + "`position_department`" + `, department_code=` + "`it`" + `（信息部）, position_code=` + "`db_admin`" + `（数据库管理员）, candidate_count=1

## 数据库备份白名单运行时动作约束
该服务的预检和放行动作由智能引擎运行时执行；参考路径 workflow_json 也必须展示这两个动作节点，让用户能在流程图上看到完整业务链路。
如果可用动作列表存在 code=` + "`db_backup_whitelist_precheck`" + ` 和 code=` + "`db_backup_whitelist_apply`" + `，必须生成两个 type="action" 节点，并使用对应的数字 action_id。
推荐路径：申请人表单 -> 备份白名单预检 action -> 数据库管理员人工处理 -> 执行备份白名单放行 action -> 结束。
数据库管理员 rejected 出边必须直接指向公共结束节点，不能经过放行动作节点，且不能退回申请人补充。
运行时仍由智能引擎优先通过 decision.execute_action 同步执行预检和放行动作，不要因为 workflow_json 中有 action 节点就改变为异步动作活动。
协作规范没有定义补充或返工路径；人工 process 节点的 rejected 出边应指向公共结束节点，不能退回申请人补充。
`

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
	"missing-window": {
		Summary: "数据库备份白名单临时放行申请：缺少放行窗口。",
		FormData: map[string]any{
			"database_name": "prod-mysql-01",
			"source_ip":     "10.20.30.52",
			"access_reason": "生产数据库备份任务需要临时放行白名单，但申请未给出放行时间窗。",
		},
	},
	"ambiguous-window": {
		Summary: "数据库备份白名单临时放行申请：放行窗口过于模糊。",
		FormData: map[string]any{
			"database_name":    "prod-mysql-02",
			"source_ip":        "10.20.30.53",
			"whitelist_window": "明天晚上",
			"access_reason":    "生产数据库备份任务需要临时放行白名单，但只给了模糊时段。",
		},
	},
	"apply-failure": {
		Summary: "数据库备份白名单临时放行申请：放行动作失败后恢复。",
		FormData: map[string]any{
			"database_name":    "prod-oracle-01",
			"source_ip":        "10.20.30.54",
			"whitelist_window": "2026-05-01 22:00:00 ~ 2026-05-01 23:00:00",
			"access_reason":    "生产数据库备份任务需要临时放行白名单，并验证失败后恢复重试。",
		},
	},
}

// generateDbBackupWorkflow calls the LLM to generate a db backup whitelist workflow JSON
// from the collaboration spec. Same pattern as generateVPNWorkflow.
func generateDbBackupWorkflow(cfg llmConfig, actionsContext string) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.BuildUserMessage(dbBackupCollaborationSpec, dbBackupGenerationContext+actionsContext, lastErrors)

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

// publishDbBackupSmartService creates the full service configuration for db backup whitelist BDD tests:
// ServiceCatalog + Priority + Agent + ServiceDefinition + 2 ServiceActions (precheck + apply).
func publishDbBackupSmartService(bc *bddContext) error {
	// 0. Initialize LocalActionReceiver (if not already done).
	if bc.actionReceiver == nil {
		bc.actionReceiver = NewLocalActionReceiver()
	}

	// 1. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "数据库服务",
		Code:     "db-services",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 2. Priority
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

	// 3. Seed Agent record (process decision agent)
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

	// 4. ServiceDefinition with engine_type=smart. WorkflowJSON is generated after
	// actions are created so the reference path can bind real action_id values.
	svc := &ServiceDefinition{
		Name:              "数据库备份白名单临时放行",
		Code:              "db-backup-whitelist",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(`{"nodes":[],"edges":[]}`),
		CollaborationSpec: dbBackupCollaborationSpec,
		IntakeFormSchema:  JSONField(dbBackupFormSchema),
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	// 5. Create 2 ServiceActions: precheck + apply (URLs point to LocalActionReceiver)
	precheckConfig, _ := json.Marshal(engine.ActionConfig{
		URL:     bc.actionReceiver.URL("/precheck"),
		Method:  "POST",
		Body:    `{"ticket_code":"{{ticket.code}}","database_name":"{{ticket.form_data.database_name}}","source_ip":"{{ticket.form_data.source_ip}}","whitelist_window":"{{ticket.form_data.whitelist_window}}","access_reason":"{{ticket.form_data.access_reason}}"}`,
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
		Body:    `{"ticket_code":"{{ticket.code}}","database_name":"{{ticket.form_data.database_name}}","source_ip":"{{ticket.form_data.source_ip}}","whitelist_window":"{{ticket.form_data.whitelist_window}}"}`,
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

	actionsContext := fmt.Sprintf(`

## 可用动作（Action）列表
以下动作可在工作流中作为 action 类型节点使用：

- **%s**（id: %d, code: %s）：%s
- **%s**（id: %d, code: %s）：%s
`, precheckAction.Name, precheckAction.ID, precheckAction.Code, precheckAction.Description,
		applyAction.Name, applyAction.ID, applyAction.Code, applyAction.Description)

	// 6. Generate workflow via LLM with real action IDs, then persist it.
	workflowJSON, err := generateDbBackupWorkflow(bc.llmCfg, actionsContext)
	if err != nil {
		return fmt.Errorf("generate db backup workflow: %w", err)
	}
	if err := bc.db.Model(svc).Update("workflow_json", JSONField(workflowJSON)).Error; err != nil {
		return fmt.Errorf("update db backup workflow json: %w", err)
	}
	svc.WorkflowJSON = JSONField(workflowJSON)

	// 7. Re-wire engines with syncActionSubmitter (actionReceiver is now set).
	orgSvc := &testOrgService{db: bc.db}
	resolver := engine.NewParticipantResolver(orgSvc)
	bc.engine = engine.NewClassicEngine(resolver, &noopSubmitter{}, nil)
	submitter := &syncActionSubmitter{db: bc.db, classicEngine: bc.engine}
	executor := &testDecisionExecutor{db: bc.db, llmCfg: bc.llmCfg, recordToolCall: bc.recordToolCall, recordToolResult: bc.recordToolResult}
	userProvider := &testUserProvider{db: bc.db}
	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, submitter, &bddConfigProvider{bc: bc})
	bc.smartEngine.SetActionExecutor(engine.NewActionExecutor(bc.db))

	return nil
}
