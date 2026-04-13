"use client"

import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Search, ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"
import type { TreeNode } from "./types"
import { filterTree, collectAllIds } from "./types"

function DepartmentTreeItem({
  node,
  selectedId,
  onSelect,
  expanded,
  onToggleExpand,
  depth,
}: {
  node: TreeNode
  selectedId: number | null
  onSelect: (id: number) => void
  expanded: Set<number>
  onToggleExpand: (id: number) => void
  depth: number
}) {
  const hasChildren = node.children && node.children.length > 0
  const isExpanded = expanded.has(node.id)
  const isSelected = selectedId === node.id
  return (
    <div className="space-y-1">
      <button
        type="button"
        onClick={() => onSelect(node.id)}
        className={cn(
          "group flex w-full min-w-0 items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm transition-colors duration-150",
          isSelected
            ? "bg-accent text-accent-foreground"
            : "text-foreground/88 hover:bg-accent/60"
        )}
        style={{ paddingLeft: `${depth * 14 + 12}px` }}
      >
        {hasChildren ? (
          <span
            className={cn(
              "flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors",
              isSelected ? "text-foreground/80" : "group-hover:text-foreground"
            )}
            onClick={(e) => {
              e.stopPropagation()
              onToggleExpand(node.id)
            }}
          >
            <ChevronRight
              className={cn(
                "h-3.5 w-3.5 transition-transform duration-200",
                isExpanded && "rotate-90"
              )}
            />
          </span>
        ) : (
          <span className="w-5 shrink-0" />
        )}
        <span className="truncate flex-1 font-medium">{node.name}</span>
        {node.memberCount > 0 && (
          <span
            className={cn(
              "shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium tabular-nums",
              isSelected
                ? "bg-background text-foreground"
                : "bg-muted text-muted-foreground"
            )}
          >
            {node.memberCount}
          </span>
        )}
      </button>
      {hasChildren && isExpanded && (
        <div>
          {node.children!.map((child) => (
            <DepartmentTreeItem
              key={child.id}
              node={child}
              selectedId={selectedId}
              onSelect={onSelect}
              expanded={expanded}
              onToggleExpand={onToggleExpand}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}

interface DepartmentTreeProps {
  treeData: TreeNode[] | undefined
  treeLoading: boolean
  selectedDeptId: number | null
  onSelectDept: (id: number) => void
  expanded: Set<number>
  onSetExpanded: (expanded: Set<number>) => void
}

export function DepartmentTree({
  treeData,
  treeLoading,
  selectedDeptId,
  onSelectDept,
  expanded,
  onSetExpanded,
}: DepartmentTreeProps) {
  const { t } = useTranslation(["org", "common"])
  const [deptSearch, setDeptSearch] = useState("")

  const filteredTree = useMemo(() => {
    if (!treeData) return []
    return filterTree(treeData, deptSearch)
  }, [treeData, deptSearch])

  const allTreeIds = useMemo(() => collectAllIds(treeData ?? []), [treeData])

  const isAllExpanded = useMemo(() => {
    if (allTreeIds.length === 0) return false
    return allTreeIds.every((id) => expanded.has(id))
  }, [allTreeIds, expanded])

  function toggleExpand(id: number) {
    const next = new Set(expanded)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    onSetExpanded(next)
  }

  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border bg-card">
      <div className="border-b px-4 py-3">
        <div className="flex items-start justify-between gap-3">
          <h3 className="text-sm font-medium text-foreground">
            {t("org:departments.title")}
          </h3>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={() => {
              if (treeData && treeData.length > 0) {
                onSetExpanded(isAllExpanded ? new Set() : new Set(allTreeIds))
              }
            }}
          >
            {isAllExpanded ? t("common:collapseAll") : t("common:expandAll")}
          </Button>
        </div>
        <div className="relative mt-3">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t("org:assignments.searchDept")}
            value={deptSearch}
            onChange={(e) => setDeptSearch(e.target.value)}
            className="h-9 pl-8"
          />
        </div>
      </div>

      {treeLoading ? (
        <div className="flex flex-1 items-center px-4 text-sm text-muted-foreground">
          {t("common:loading")}
        </div>
      ) : filteredTree.length === 0 ? (
        <div className="flex flex-1 items-center justify-center px-4 text-sm text-muted-foreground">
          {t("org:departments.empty")}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto px-2 py-3">
          {filteredTree.map((node) => (
            <DepartmentTreeItem
              key={node.id}
              node={node}
              selectedId={selectedDeptId}
              onSelect={onSelectDept}
              expanded={expanded}
              onToggleExpand={toggleExpand}
              depth={0}
            />
          ))}
        </div>
      )}
    </section>
  )
}
