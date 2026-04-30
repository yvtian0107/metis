package bdd

// server_access_support_test.go — Server access workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the server access workflow
// from the collaboration spec, matching the VPN BDD approach.

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	"sync"
	"time"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// serverAccessCollaborationSpec is the collaboration spec for the production server temporary access service.
const serverAccessCollaborationSpec = `员工在 IT 服务台申请生产服务器临时访问时，服务台需要确认要访问的服务器或资源范围、访问时段、本次操作目的，以及为什么需要临时进入生产环境。

访问原因通常分为三类：应用发布、进程排障、日志排查、磁盘清理、主机巡检、生产运维操作偏主机和应用运维，交给信息部运维管理员处理；网络抓包、连通性诊断、ACL 调整、负载均衡变更、防火墙策略调整偏网络诊断与策略处理，交给信息部网络管理员处理；安全审计、入侵排查、漏洞修复验证、取证分析、合规检查偏安全与合规风险，交给信息部信息安全管理员处理。

处理人完成处理后流程结束。`

const serverAccessGenerationContext = `

## 已有申请表单契约
该服务已经配置申请确认表单。生成参考路径时必须复用这些字段 key、类型和选项值；排他网关条件引用表单字段时必须使用 form.<key>。

- 访问服务器: key=` + "`target_servers`" + `, type=` + "`textarea`" + `
- 访问时段: key=` + "`access_window`" + `, type=` + "`date_range`" + `
- 操作目的: key=` + "`operation_purpose`" + `, type=` + "`textarea`" + `
- 访问原因: key=` + "`access_reason`" + `, type=` + "`textarea`" + `

## 按需查询到的组织上下文
生成人工处理节点参与人时，使用以下已解析的组织映射：

- 参与人候选：type=` + "`position_department`" + `, department=信息部（code: ` + "`it`" + `）, position=运维管理员（code: ` + "`ops_admin`" + `）
- 参与人候选：type=` + "`position_department`" + `, department=信息部（code: ` + "`it`" + `）, position=网络管理员（code: ` + "`network_admin`" + `）
- 参与人候选：type=` + "`position_department`" + `, department=信息部（code: ` + "`it`" + `）, position=信息安全管理员（code: ` + "`security_admin`" + `）
`

// serverAccessCasePayload defines test data for a single server access BDD scenario.
type serverAccessCasePayload struct {
	Summary          string
	OpenMessage      string
	FormData         map[string]any
	ExpectedPosition string // expected position code for routing assertion
}

// serverAccessCasePayloads provides 4 test case payloads covering all 3 branches + 1 boundary case.
var serverAccessCasePayloads = map[string]serverAccessCasePayload{
	"ops": {
		Summary:     "生产服务器临时访问申请：需要登录生产机排查应用进程异常。",
		OpenMessage: "生产环境一台应用主机进程异常，我需要临时上去看日志并处理。",
		FormData: map[string]any{
			"target_servers":    "prod-app-02",
			"access_window":     "今晚 20:00 到 21:00",
			"operation_purpose": "登录生产应用主机检查进程状态和应用日志。",
			"access_reason":     "排查生产应用进程异常，确认日志和运行状态。",
		},
		ExpectedPosition: "ops_admin",
	},
	"network": {
		Summary:     "生产服务器临时访问申请：需要登录生产机配合网络链路诊断。",
		OpenMessage: "我们要抓包核对生产链路连通性，请先帮我整理访问申请。",
		FormData: map[string]any{
			"target_servers":    "prod-gateway-01",
			"access_window":     "今晚 21:00 到 22:30",
			"operation_purpose": "抓包核对生产链路连通性和负载均衡后的访问路径。",
			"access_reason":     "配合抓包和链路诊断，核对负载均衡后的网络访问路径。",
		},
		ExpectedPosition: "network_admin",
	},
	"security": {
		Summary:     "生产服务器临时访问申请：需要进入生产机做安全审计取证分析。",
		OpenMessage: "安全这边要上生产机核查审计痕迹并做取证分析，先帮我整理申请。",
		FormData: map[string]any{
			"target_servers":    "prod-app-03",
			"access_window":     "今晚 23:00 到 23:45",
			"operation_purpose": "登录生产应用主机核查审计痕迹并做取证分析。",
			"access_reason":     "核查安全审计痕迹并完成取证分析，确认是否存在异常访问。",
		},
		ExpectedPosition: "security_admin",
	},
	"boundary_security": {
		Summary:     "生产服务器临时访问申请：需要在异常访问核查过程中进入生产机保全证据。",
		OpenMessage: "这次不是单纯排障，我需要上生产机先核对异常访问痕迹并保全证据。",
		FormData: map[string]any{
			"target_servers":    "prod-app-04",
			"access_window":     "今晚 19:30 到 20:30",
			"operation_purpose": "登录生产机核对异常访问痕迹并固定相关日志。",
			"access_reason":     "结合异常访问核查、日志固定和证据保全判断是否需要进一步安全处置。",
		},
		ExpectedPosition: "security_admin",
	},
}

var serverAccessWorkflowCache = struct {
	sync.Mutex
	byLLM map[string]json.RawMessage
}{byLLM: map[string]json.RawMessage{}}

// generateServerAccessWorkflow calls the LLM to generate a server access workflow JSON
// from the collaboration spec. Same pattern as generateVPNWorkflow.
func generateServerAccessWorkflow(cfg llmConfig) (json.RawMessage, error) {
	cacheKey := cfg.baseURL + "\n" + cfg.model
	serverAccessWorkflowCache.Lock()
	if cached := serverAccessWorkflowCache.byLLM[cacheKey]; len(cached) > 0 {
		out := append(json.RawMessage(nil), cached...)
		serverAccessWorkflowCache.Unlock()
		return out, nil
	}
	serverAccessWorkflowCache.Unlock()

	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.BuildUserMessage(serverAccessCollaborationSpec, serverAccessGenerationContext, lastErrors)

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
			serverAccessWorkflowCache.Lock()
			serverAccessWorkflowCache.byLLM[cacheKey] = append(json.RawMessage(nil), workflowJSON...)
			serverAccessWorkflowCache.Unlock()
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

// publishServerAccessSmartService creates the full service configuration for server access BDD tests:
// ServiceCatalog + Priority + Agent + ServiceDefinition with LLM-generated workflow JSON.
func publishServerAccessSmartService(bc *bddContext) error {
	// 1. Generate workflow via LLM
	workflowJSON, err := generateServerAccessWorkflow(bc.llmCfg)
	if err != nil {
		return fmt.Errorf("generate server access workflow: %w", err)
	}

	// 2. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "基础设施服务",
		Code:     "infra-compute",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 3. Priority
	priority := &Priority{
		Name:     "紧急",
		Code:     "urgent-sa",
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
	// Override GORM default temperature to 0 (don't send temperature to models that reject it)
	bc.db.Model(agent).Update("temperature", 0)

	// 5. ServiceDefinition with engine_type=smart
	svc := &ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-request",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: serverAccessCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	return nil
}
