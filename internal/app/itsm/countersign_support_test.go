package itsm

// countersign_support_test.go — Countersign workflow LLM generation and service publish helpers for BDD tests.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// countersignCollaborationSpec is the collaboration spec for the multi-role countersign service.
const countersignCollaborationSpec = `这是一个多角色并行处理申请服务。
用户来找服务台时，先把申请标题、目标系统、时间窗口、申请原因和期望结果这些信息问清楚，再整理成可以确认的申请摘要。
申请人提交后，先进入一个多角色并行处理处理节点，需要信息部的网络管理员（position_department: it/network_admin）和安全管理员（position_department: it/security_admin）并行完成处理。
你必须使用 execution_mode: "parallel"，在 activities 数组中同时列出两个处理人，participant_type 使用 position_department。
只有当全部并行处理处理都完成后，工单才可以汇聚到信息部运维管理员（position_department: it/ops_admin）的最终单签处理节点。
最终单签处理完成后直接结束流程，不需要额外生成取消分支。`

// countersignCasePayload defines test data for a single countersign BDD scenario.
type countersignCasePayload struct {
	Summary  string
	FormData map[string]any
}

// countersignCasePayloads provides test case payloads for countersign BDD.
var countersignCasePayloads = map[string]countersignCasePayload{
	"standard": {
		Summary: "多角色并行处理申请：需要网络和安全管理员同时处理的变更请求。",
		FormData: map[string]any{
			"title":         "防火墙策略变更",
			"target_system": "prod-firewall-01",
			"time_window":   "今晚 22:00 到 23:00",
			"reason":        "需要调整防火墙策略以支持新的微服务通信，涉及网络和安全双重处理。",
			"expected":      "允许 10.0.1.0/24 网段访问 10.0.2.0/24 的 8443 端口",
		},
	},
}

// generateCountersignWorkflow calls the LLM to generate a countersign workflow JSON.
func generateCountersignWorkflow(cfg llmConfig) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.buildUserMessage(countersignCollaborationSpec, "", lastErrors)

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

// publishCountersignSmartService creates the full service configuration for countersign BDD tests:
// ServiceCatalog + Priority + Agent + ServiceDefinition with LLM-generated workflow JSON.
func publishCountersignSmartService(bc *bddContext) error {
	// 1. Generate workflow via LLM
	workflowJSON, err := generateCountersignWorkflow(bc.llmCfg)
	if err != nil {
		return fmt.Errorf("generate countersign workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "安全与网络服务",
		Code:     "sec-net",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "普通",
		Code:     "normal-cs",
		Value:    3,
		Color:    "#52c41a",
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
		Name:              "多角色并行处理申请",
		Code:              "countersign-request",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: countersignCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}
