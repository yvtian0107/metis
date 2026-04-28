import { describe, expect, test } from "bun:test"
import type { Edge, Node } from "@xyflow/react"
import {
  collectDraftIssues,
  defaultNodeData,
  defaultWorkflowData,
  prepareWorkflowForSave,
} from "./workflow-contract"
import type { WFNodeData } from "./types"

const t = (key: string) => key

describe("workflow contract", () => {
  test("creates default classic skeleton with backend field names", () => {
    const data = defaultWorkflowData(t)

    expect(data.nodes.map((node) => node.id)).toEqual(["start", "end"])
    expect(data.edges).toHaveLength(1)
    expect(data.edges[0].data).toMatchObject({ outcome: "completed", default: true })
    expect(data.nodes[0].data).toMatchObject({ nodeType: "start" })
  })

  test("uses backend snake_case data for configurable node defaults", () => {
    expect(defaultNodeData("wait", "等待")).toMatchObject({ wait_mode: "signal" })
    expect(defaultNodeData("timer", "定时")).toMatchObject({ wait_mode: "timer" })
    expect(defaultNodeData("parallel", "并行")).toMatchObject({ gateway_direction: "fork" })
    expect(defaultNodeData("inclusive", "包含")).toMatchObject({ gateway_direction: "fork" })
  })

  test("detects runnable-node draft issues", () => {
    const nodes: Node[] = [
      node("start", "start"),
      node("form", "form"),
      node("action", "action"),
      node("script", "script", { assignments: [] }),
      node("sub", "subprocess"),
      node("end", "end"),
    ]
    const edges: Edge[] = [
      edge("e1", "start", "form"),
      edge("e2", "form", "action"),
      edge("e3", "action", "script"),
      edge("e4", "script", "sub"),
      edge("e5", "sub", "end"),
    ]

    const messages = collectDraftIssues(nodes, edges).map((issue) => issue.message)

    expect(messages).toContain("人工节点缺少参与人")
    expect(messages).toContain("动作节点缺少 action_id")
    expect(messages).toContain("脚本节点缺少变量赋值")
    expect(messages).toContain("子流程节点缺少 subprocess_def")
  })

  test("keeps clean backend contract when saving", () => {
    const nodes: Node[] = [
      {
        ...node("action", "action", { action_id: 9, _workflowState: "active" }),
        className: "ring",
      },
    ]
    const edges: Edge[] = [
      {
        ...edge("e1", "action", "end", { isDefault: true, readonly: true, visited: true, failed: true }),
      },
    ]

    const saved = prepareWorkflowForSave(nodes, edges)

    expect(saved.nodes[0].data).toEqual({ label: "action", nodeType: "action", action_id: 9 })
    expect(saved.edges[0].data).toEqual({ default: true })
  })

  test("removes transient layout metadata when saving", () => {
    const nodes: Node[] = [
      node("form", "form", { participants: [{ type: "user", value: "1" }], _layoutDirection: "TB" }),
    ]

    const saved = prepareWorkflowForSave(nodes, [])

    expect(saved.nodes[0].data).toEqual({
      label: "form",
      nodeType: "form",
      participants: [{ type: "user", value: "1" }],
    })
  })

  test("accepts all current frontend-configurable classic node data shapes", () => {
    const data: Record<string, WFNodeData> = {
      form: { label: "表单", nodeType: "form", participants: [{ type: "requester" }], formSchema: { version: 1, fields: [] } },
      process: { label: "处理", nodeType: "process", participants: [{ type: "user", value: "7" }], formSchema: { version: 1, fields: [] } },
      action: { label: "动作", nodeType: "action", action_id: 1 },
      script: { label: "脚本", nodeType: "script", assignments: [{ variable: "x", expression: "1 + 1" }] },
      notify: { label: "通知", nodeType: "notify", channel_id: 2, template: "hello" },
      exclusive: { label: "排他", nodeType: "exclusive" },
      parallel: { label: "并行", nodeType: "parallel", gateway_direction: "fork" },
      inclusive: { label: "包含", nodeType: "inclusive", gateway_direction: "join" },
      wait: { label: "等待", nodeType: "wait", wait_mode: "timer", duration: "1h" },
      subprocess: { label: "子流程", nodeType: "subprocess", subprocess_def: { nodes: [], edges: [] } },
      end: { label: "结束", nodeType: "end" },
    }

    expect(Object.values(data).every((item) => "nodeType" in item && "label" in item)).toBe(true)
    expect(data.action).not.toHaveProperty("actionId")
    expect(data.wait).not.toHaveProperty("waitMode")
    expect(data.script).not.toHaveProperty("scriptAssignments")
    expect(data.subprocess).not.toHaveProperty("subprocessJson")
  })
})

function node(id: string, type: WFNodeData["nodeType"], data: Partial<WFNodeData> = {}): Node {
  return {
    id,
    type,
    position: { x: 0, y: 0 },
    data: { label: id, nodeType: type, ...data } as unknown as Record<string, unknown>,
  }
}

function edge(id: string, source: string, target: string, data: Record<string, unknown> = {}): Edge {
  return {
    id,
    source,
    target,
    data,
  }
}
