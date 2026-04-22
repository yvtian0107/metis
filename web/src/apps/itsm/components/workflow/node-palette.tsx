import { useTranslation } from "react-i18next"
import type { NodeType } from "./types"
import { WORKFLOW_NODE_GROUPS, getNodeAccent } from "./visual-data"
import { WorkflowNodeIconGlyph } from "./visual"

export function NodePalette() {
  const { t } = useTranslation("itsm")

  function onDragStart(event: React.DragEvent, nodeType: NodeType) {
    event.dataTransfer.setData("application/reactflow-nodetype", nodeType)
    event.dataTransfer.effectAllowed = "move"
  }

  return (
    <aside className="flex w-[232px] shrink-0 flex-col overflow-y-auto border-r border-border/55 bg-white/48 px-3 py-3">
      <div className="mb-3 px-1">
        <div className="text-sm font-semibold tracking-[-0.01em]">{t("workflow.nodeTypes")}</div>
        <div className="mt-1 text-xs leading-5 text-muted-foreground">{t("workflow.paletteHint")}</div>
      </div>
      <div className="space-y-4">
        {WORKFLOW_NODE_GROUPS.map((group) => (
          <section key={group.label}>
            <div className="mb-1.5 px-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground/72">
            {t(group.label)}
            </div>
            <div className="space-y-1.5">
              {group.types.map((nt) => {
                const color = getNodeAccent(nt)
                return (
                  <div
                    key={nt}
                    draggable
                    onDragStart={(e) => onDragStart(e, nt)}
                    className="group flex min-h-[3.125rem] cursor-grab items-center gap-2.5 rounded-xl border border-border/64 bg-white/72 px-2.5 py-2 transition hover:border-primary/35 hover:bg-white active:cursor-grabbing"
                  >
                    <div
                      className="flex size-7 shrink-0 items-center justify-center rounded-lg text-white"
                      style={{ backgroundColor: color }}
                    >
                      <WorkflowNodeIconGlyph nodeType={nt} />
                    </div>
                    <div className="min-w-0">
                      <div className="truncate text-[13px] font-medium text-foreground">{t(`workflow.node.${nt}`)}</div>
                      <div className="truncate text-[11px] text-muted-foreground">{t(`workflow.nodeDesc.${nt}`)}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          </section>
        ))}
      </div>
    </aside>
  )
}
