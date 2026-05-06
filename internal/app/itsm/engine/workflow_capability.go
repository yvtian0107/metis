package engine

import "metis/internal/app/itsm/contract"

type WorkflowCapability struct {
	Version   string                            `json:"version"`
	NodeTypes map[string]WorkflowNodeCapability `json:"nodeTypes"`
}

type WorkflowNodeCapability struct {
	Type           string   `json:"type"`
	Executable     bool     `json:"executable"`
	RequiredFields []string `json:"requiredFields,omitempty"`
	DisabledReason string   `json:"disabledReason,omitempty"`
}

func WorkflowCapabilities() WorkflowCapability {
	nodes := map[string]WorkflowNodeCapability{}
	for _, nodeType := range []string{
		NodeStart,
		NodeEnd,
		NodeForm,
		NodeApprove,
		NodeProcess,
		NodeAction,
		NodeNotify,
		NodeWait,
		NodeExclusive,
		NodeParallel,
		NodeInclusive,
		NodeScript,
		NodeSubprocess,
		NodeTimer,
		NodeSignal,
		NodeBTimer,
		NodeBError,
	} {
		nt := contract.NodeType(nodeType)
		capability := WorkflowNodeCapability{
			Type:       nodeType,
			Executable: nt.IsExecutable(),
		}
		if !capability.Executable {
			capability.DisabledReason = "当前后端尚未实现该节点运行逻辑"
		}
		switch nodeType {
		case NodeForm, NodeApprove, NodeProcess:
			capability.RequiredFields = []string{"participants"}
		case NodeAction:
			capability.RequiredFields = []string{"action_id"}
		case NodeParallel, NodeInclusive:
			capability.RequiredFields = []string{"gateway_direction"}
		case NodeWait:
			capability.RequiredFields = []string{"wait_mode"}
		case NodeScript:
			capability.RequiredFields = []string{"assignments"}
		case NodeSubprocess:
			capability.RequiredFields = []string{"subprocess_def"}
		}
		nodes[nodeType] = capability
	}
	return WorkflowCapability{
		Version:   "2026-04-30.v1",
		NodeTypes: nodes,
	}
}
