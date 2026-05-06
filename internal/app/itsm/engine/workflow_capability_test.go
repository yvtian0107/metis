package engine

import "testing"

func TestWorkflowCapabilitiesHideNonExecutableNodes(t *testing.T) {
	caps := WorkflowCapabilities()
	if caps.Version == "" {
		t.Fatalf("capability version must be set")
	}
	if node, ok := caps.NodeTypes[NodeAction]; !ok || !node.Executable || node.DisabledReason != "" {
		t.Fatalf("action capability = %+v", node)
	}
	if node, ok := caps.NodeTypes[NodeTimer]; !ok || node.Executable || node.DisabledReason == "" {
		t.Fatalf("timer capability = %+v", node)
	}
	if node, ok := caps.NodeTypes[NodeSignal]; !ok || node.Executable || node.DisabledReason == "" {
		t.Fatalf("signal capability = %+v", node)
	}
}
