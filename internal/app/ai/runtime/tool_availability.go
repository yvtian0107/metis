package runtime

import "fmt"

var riskDisabledBuiltinTools = map[string]string{
	"http_request":   "外部网络访问需要域名白名单、超时、审计和敏感信息防泄露策略后才能开放。",
	"execute_script": "代码执行需要沙箱、资源限制、语言白名单、审计和结果脱敏策略后才能开放。",
}

var plannedBuiltinTools = map[string]string{
	"read_document": "文档节点读取接口和权限模型尚未稳定，当前仅作为规划能力展示。",
}

type toolAvailability struct {
	IsExecutable bool
	Status       string
	Reason       string
}

func classifyToolAvailability(tool Tool, hasRuntimeHandler bool) toolAvailability {
	if reason, ok := riskDisabledBuiltinTools[tool.Name]; ok {
		return toolAvailability{
			IsExecutable: false,
			Status:       ToolAvailabilityRiskDisabled,
			Reason:       reason,
		}
	}
	if reason, ok := plannedBuiltinTools[tool.Name]; ok {
		return toolAvailability{
			IsExecutable: false,
			Status:       ToolAvailabilityUnimplemented,
			Reason:       reason,
		}
	}
	if !hasRuntimeHandler {
		return toolAvailability{
			IsExecutable: false,
			Status:       ToolAvailabilityUnimplemented,
			Reason:       fmt.Sprintf("工具 %q 尚未注册运行时 handler。", tool.Name),
		}
	}
	if !tool.IsActive {
		return toolAvailability{
			IsExecutable: false,
			Status:       ToolAvailabilityInactive,
			Reason:       "工具已被管理员全局停用。",
		}
	}
	return toolAvailability{
		IsExecutable: true,
		Status:       ToolAvailabilityAvailable,
	}
}

func applyToolAvailability(resp ToolResponse, availability toolAvailability, boundAgentCount int64) ToolResponse {
	resp.IsExecutable = availability.IsExecutable
	resp.AvailabilityStatus = availability.Status
	resp.AvailabilityReason = availability.Reason
	resp.BoundAgentCount = boundAgentCount
	return resp
}
