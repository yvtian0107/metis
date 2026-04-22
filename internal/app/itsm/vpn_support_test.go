package itsm

// vpn_support_test.go — VPN workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the VPN workflow
// from the collaboration spec, matching the bklite-cloud approach.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// vpnCollaborationSpec is the collaboration spec for the VPN activation service.
// Mirrors the seed data in seed.go.
const vpnCollaborationSpec = `用户在 IT 服务台提交 VPN 开通申请。服务台需要收集 VPN 账号、设备与用途说明、访问原因。如果访问原因属于线上支持、故障排查、生产应急或网络接入问题，则交给信息部的网络管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 network_admin。如果访问原因属于外部协作、长期远程办公、跨境访问或安全合规事项，则交给信息部的信息安全管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 security_admin。处理完成后直接结束流程，不要生成取消分支。`

// vpnSampleFormData provides typical VPN request form values for BDD tests.
// The "request_kind" field drives the exclusive gateway routing.
var vpnSampleFormData = map[string]any{
	"request_kind": "network_support",
	"vpn_type":     "l2tp",
	"reason":       "需要远程访问内网开发环境",
}

// llmConfig holds LLM configuration loaded from environment variables.
type llmConfig struct {
	baseURL string
	apiKey  string
	model   string
}

// requireLLMConfig loads LLM config from env or skips the test.
func requireLLMConfig(t *testing.T) llmConfig {
	t.Helper()
	baseURL := os.Getenv("LLM_TEST_BASE_URL")
	apiKey := os.Getenv("LLM_TEST_API_KEY")
	model := os.Getenv("LLM_TEST_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		t.Skip("LLM integration test skipped: set LLM_TEST_BASE_URL, LLM_TEST_API_KEY, LLM_TEST_MODEL")
	}
	return llmConfig{baseURL: baseURL, apiKey: apiKey, model: model}
}

// hasLLMConfig checks whether LLM env vars are set (non-skip version for TestBDD).
func hasLLMConfig() bool {
	return os.Getenv("LLM_TEST_BASE_URL") != "" &&
		os.Getenv("LLM_TEST_API_KEY") != "" &&
		os.Getenv("LLM_TEST_MODEL") != ""
}

// generateVPNWorkflow calls the LLM to generate a VPN workflow JSON from the collaboration spec.
// It retries up to maxRetries times, feeding validation errors back to the LLM.
func generateVPNWorkflow(cfg llmConfig) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.buildUserMessage(vpnCollaborationSpec, "", lastErrors)

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
		// Filter to only blocking errors (not warnings)
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

		// Return last attempt with errors
		return nil, fmt.Errorf("workflow validation failed after %d attempts: %v", attempt+1, blockingErrors)
	}

	return nil, fmt.Errorf("workflow generation failed")
}

// publishVPNService creates the full service configuration for VPN BDD tests:
// ServiceCatalog + Priority + ServiceDefinition with LLM-generated workflow JSON.
func publishVPNService(bc *bddContext, cfg llmConfig) error {
	// 1. Generate workflow via LLM
	workflowJSON, err := generateVPNWorkflow(cfg)
	if err != nil {
		return fmt.Errorf("generate VPN workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "VPN服务",
		Code:     "vpn",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "普通",
		Code:     "normal",
		Value:    3,
		Color:    "#52c41a",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	// 4. ServiceDefinition
	svc := &ServiceDefinition{
		Name:              "VPN开通申请",
		Code:              "vpn-activation",
		CatalogID:         catalog.ID,
		EngineType:        "classic",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: vpnCollaborationSpec,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}

// decisionAgentSystemPrompt is the system prompt for the process decision agent.
const decisionAgentSystemPrompt = `你是流程决策智能体，负责为 ITSM 工单给出下一步可执行、可审计、可落地的流程决策。

你的核心职责：
1. 基于服务协作规范、流程定义、工单当前活动、历史活动、时间线和运行时上下文，判断当前只应该进入哪一个下一步
2. 当需要确定具体处理人、岗位或岗位+部门时，必须先依据组织架构和已知上下文做判断，不能凭空假设
3. 只能输出当前工单"下一步真正需要执行"的活动，不要把未来多步一次性展开
4. reasoning 必须解释判断依据，说明为什么是这个人、这个岗位，或者这个岗位+部门

决策原则：
1. 优先遵循明确规则，其次才是保守推断；不能为了让流程继续而编造参与者、节点或条件
2. 信息不足时，优先做保守决策：宁可指出需要人工介入，也不要输出高风险猜测
3. 处理节点仅支持使用/取消，处理节点仅支持提交结果
4. 如果动作执行失败，要明确指出流程被阻塞且需要人工处理

严格约束：
1. 不要跳过工具或上下文校验直接编造结论
2. 不允许输出未在流程或策略中出现的参与方式
3. 不允许把姓名当作 username，不允许把岗位名称当作岗位 code，不允许把部门名称当作部门 code
4. 不允许为了"看起来完整"而补全不存在的处理链

请始终输出结构化、保守且可审计的判断。`

// publishVPNSmartService creates a smart service definition for BDD tests:
// LLM-generated workflow JSON + Agent record + ServiceDefinition(engine_type=smart).
func publishVPNSmartService(bc *bddContext) error {
	// 1. Generate workflow via LLM (smart engine tool chain also needs workflow_json as context)
	workflowJSON, err := generateVPNWorkflow(bc.llmCfg)
	if err != nil {
		return fmt.Errorf("generate VPN workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "VPN服务(智能)",
		Code:     "vpn-smart",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "普通",
		Code:     "normal-smart",
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
		Temperature:  0.2,
		MaxTokens:    2048,
		MaxTurns:     1,
		CreatedBy:    1,
	}
	if err := bc.db.Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// 5. ServiceDefinition with engine_type=smart
	svc := &ServiceDefinition{
		Name:              "VPN开通申请(智能)",
		Code:              "vpn-activation-smart",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: vpnCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}

// missingParticipantWorkflowJSON is a static workflow fixture where the process node
// has no participant_type, used to test smart engine's graceful fallback.
var missingParticipantWorkflowJSON = json.RawMessage(`{
	"nodes": [
		{"id": "start", "type": "start", "label": "开始"},
		{"id": "process", "type": "process", "label": "处理", "config": {}},
		{"id": "end", "type": "end", "label": "结束"}
	],
	"edges": [
		{"source": "start", "target": "process"},
		{"source": "process", "target": "end", "condition": "completed"}
	]
}`)
