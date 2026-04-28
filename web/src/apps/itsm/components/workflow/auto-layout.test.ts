import { describe, expect, test } from "bun:test"
import { Position, type Edge, type Node } from "@xyflow/react"
import { applyViewerLayout, applyWorkflowAutoLayout } from "./auto-layout"
import type { WFNodeData } from "./types"

describe("workflow auto layout", () => {
  test("lays out a linear workflow left to right for generated previews", () => {
    const layouted = applyViewerLayout(
      [node("start", "start"), node("form", "form"), node("end", "end")],
      [edge("e1", "start", "form"), edge("e2", "form", "end")],
    )

    expectFinitePositions(layouted)
    expect(layouted[0].position.x).toBeLessThan(layouted[1].position.x)
    expect(layouted[1].position.x).toBeLessThan(layouted[2].position.x)
    expect(layouted[1].targetPosition).toBe(Position.Left)
    expect(layouted[1].sourcePosition).toBe(Position.Right)
    expect((layouted[1].data as unknown as WFNodeData)._layoutDirection).toBe("LR")
  })

  test("keeps exclusive gateway branches separated across vertical lanes", () => {
    const layouted = applyViewerLayout(
      [
        node("start", "start"),
        node("gateway", "exclusive"),
        node("network", "process"),
        node("security", "process"),
        node("end", "end"),
      ],
      [
        edge("e1", "start", "gateway"),
        edge("e2", "gateway", "network"),
        edge("e3", "gateway", "security"),
        edge("e4", "network", "end"),
        edge("e5", "security", "end"),
      ],
    )
    const byId = new Map(layouted.map((item) => [item.id, item]))
    const network = byId.get("network")
    const security = byId.get("security")

    expect(network).toBeDefined()
    expect(security).toBeDefined()
    expect(Math.abs((network?.position.x ?? 0) - (security?.position.x ?? 0))).toBeLessThan(8)
    expect(Math.abs((network?.position.y ?? 0) - (security?.position.y ?? 0))).toBeGreaterThan(96)
  })

  test("supports multiple end nodes without overlapping them", () => {
    const layouted = applyViewerLayout(
      [
        node("start", "start"),
        node("gateway", "exclusive"),
        node("network", "process"),
        node("security", "process"),
        node("end_network", "end"),
        node("end_security", "end"),
      ],
      [
        edge("e1", "start", "gateway"),
        edge("e2", "gateway", "network"),
        edge("e3", "gateway", "security"),
        edge("e4", "network", "end_network"),
        edge("e5", "security", "end_security"),
      ],
    )
    const ends = layouted.filter((item) => item.id.startsWith("end_"))

    expect(ends).toHaveLength(2)
    expectFinitePositions(ends)
    expect(Math.abs(ends[0].position.y - ends[1].position.y)).toBeGreaterThan(96)
    expect(Math.abs(ends[0].position.x - ends[1].position.x)).toBeLessThan(8)
  })

  test("keeps classic auto layout left to right by default", () => {
    const layouted = applyWorkflowAutoLayout(
      [node("start", "start"), node("form", "form"), node("end", "end")],
      [edge("e1", "start", "form"), edge("e2", "form", "end")],
    )

    expect(layouted[0].position.x).toBeLessThan(layouted[1].position.x)
    expect(layouted[1].position.x).toBeLessThan(layouted[2].position.x)
    expect(layouted[1].targetPosition).toBe(Position.Left)
    expect(layouted[1].sourcePosition).toBe(Position.Right)
    expect((layouted[1].data as unknown as WFNodeData)._layoutDirection).toBe("LR")
  })
})

function node(id: string, nodeType: WFNodeData["nodeType"]): Node {
  return {
    id,
    type: nodeType,
    position: { x: Number.NaN, y: Number.NaN },
    data: { label: id, nodeType } as unknown as Record<string, unknown>,
  }
}

function edge(id: string, source: string, target: string): Edge {
  return {
    id,
    source,
    target,
  }
}

function expectFinitePositions(nodes: Node[]) {
  for (const item of nodes) {
    expect(Number.isFinite(item.position.x)).toBe(true)
    expect(Number.isFinite(item.position.y)).toBe(true)
  }
}
