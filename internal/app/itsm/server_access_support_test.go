package itsm

// server_access_support_test.go — Server access workflow LLM generation and service publish helpers for BDD tests.
//
// Uses the LLM (gated by LLM_TEST_* env vars) to generate the server access workflow
// from the collaboration spec, matching the VPN BDD approach.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

// serverAccessCollaborationSpec is the collaboration spec for the production server temporary access service.
const serverAccessCollaborationSpec = `这是一个生产服务器临时访问申请服务。
用户来找服务台时，先把访问账号、目标主机、来源 IP、访问时段和访问目的这些信息问清楚，再整理成可以确认的申请摘要。
常见的应用排障、主机巡检、日志查看、进程处理、磁盘清理和一般生产运维访问，交给信息部的运维管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 ops_admin。
网络抓包、链路诊断、ACL 调整、负载均衡检查、防火墙策略核对和其他网络侧访问，交给信息部的网络管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 network_admin。
安全审计、取证分析、漏洞修复验证、入侵排查、合规核查和其他高敏访问，交给信息部的安全管理员岗位处理，处理参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 security_admin。
不要让申请人在表单里自己选择处理类别，流程决策智能体应根据访问目的和访问原因在运行时判断应该流转到哪个处理岗位。
处理完成后直接结束流程，不需要额外生成取消分支。`

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
			"access_account": "ops.reader",
			"target_host":    "prod-app-02",
			"source_ip":      "10.20.30.41",
			"access_window":  "今晚 20:00 到 21:00",
			"access_purpose": "排查生产应用进程异常，确认日志和运行状态。",
		},
		ExpectedPosition: "ops_admin",
	},
	"network": {
		Summary:     "生产服务器临时访问申请：需要登录生产机配合网络链路诊断。",
		OpenMessage: "我们要抓包核对生产链路连通性，请先帮我整理访问申请。",
		FormData: map[string]any{
			"access_account": "net.trace",
			"target_host":    "prod-gateway-01",
			"source_ip":      "10.20.30.42",
			"access_window":  "今晚 21:00 到 22:30",
			"access_purpose": "配合抓包和链路诊断，核对负载均衡后的网络访问路径。",
		},
		ExpectedPosition: "network_admin",
	},
	"security": {
		Summary:     "生产服务器临时访问申请：需要进入生产机做安全审计取证分析。",
		OpenMessage: "安全这边要上生产机核查审计痕迹并做取证分析，先帮我整理申请。",
		FormData: map[string]any{
			"access_account": "sec.audit",
			"target_host":    "prod-app-03",
			"source_ip":      "10.20.30.43",
			"access_window":  "今晚 23:00 到 23:45",
			"access_purpose": "核查安全审计痕迹并完成取证分析，确认是否存在异常访问。",
		},
		ExpectedPosition: "security_admin",
	},
	"boundary_security": {
		Summary:     "生产服务器临时访问申请：需要在异常访问核查过程中进入生产机保全证据。",
		OpenMessage: "这次不是单纯排障，我需要上生产机先核对异常访问痕迹并保全证据。",
		FormData: map[string]any{
			"access_account": "sec.boundary",
			"target_host":    "prod-app-04",
			"source_ip":      "10.20.30.44",
			"access_window":  "今晚 19:30 到 20:30",
			"access_purpose": "结合异常访问核查、日志固定和证据保全判断是否需要进一步安全处置。",
		},
		ExpectedPosition: "security_admin",
	},
}

// generateServerAccessWorkflow calls the LLM to generate a server access workflow JSON
// from the collaboration spec. Same pattern as generateVPNWorkflow.
func generateServerAccessWorkflow(cfg llmConfig) (json.RawMessage, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, cfg.baseURL, cfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	svc := &WorkflowGenerateService{}
	maxRetries := 3

	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := svc.buildUserMessage(serverAccessCollaborationSpec, "", lastErrors)

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
