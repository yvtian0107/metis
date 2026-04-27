import { useState, useMemo, useCallback } from "react"
import { useQuery } from "@tanstack/react-query"
import { Input } from "@/components/ui/input"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { api } from "@/lib/api"
import { ChevronRight } from "lucide-react"

interface DeptNode {
  id: number
  name: string
  code: string
  children?: DeptNode[]
}

interface DeptPickerProps {
  /** Department ID stored as string, e.g. "5" */
  value?: string
  onChange: (value: string) => void
  disabled?: boolean
  readOnly?: boolean
  placeholder?: string
}

function findInTree(nodes: DeptNode[], targetId: number): string | null {
  for (const n of nodes) {
    if (n.id === targetId) return n.name
    if (n.children) {
      const found = findInTree(n.children, targetId)
      if (found) return found
    }
  }
  return null
}

export function DeptPicker({ value, onChange, disabled, readOnly, placeholder }: DeptPickerProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")

  // Load department tree via query
  const { data: tree = [], isError: treeFailed } = useQuery({
    queryKey: ["form-dept-tree"],
    queryFn: () =>
      api.get<{ items: DeptNode[] }>("/api/v1/org/departments/tree")
        .then(res => res.items ?? []),
    staleTime: 300_000,
  })

  // Derive the display label synchronously from value + tree
  const selectedLabel = useMemo(() => {
    if (!value) return ""
    const numId = parseInt(value, 10)
    if (isNaN(numId)) return value
    return findInTree(tree, numId) || value
  }, [value, tree])

  const flatFilter = useCallback((nodes: DeptNode[], keyword: string): DeptNode[] => {
    const result: DeptNode[] = []
    const walk = (ns: DeptNode[]) => {
      for (const n of ns) {
        if (n.name.toLowerCase().includes(keyword.toLowerCase())) {
          result.push(n)
        }
        if (n.children) walk(n.children)
      }
    }
    walk(nodes)
    return result.slice(0, 20)
  }, [])

  if (readOnly || disabled) {
    return <Input value={selectedLabel} disabled readOnly placeholder={placeholder} />
  }

  if (treeFailed) {
    return (
      <Input
        type="number"
        value={value ?? ""}
        onChange={e => onChange(e.target.value)}
        placeholder="输入部门 ID（部门树不可用）"
      />
    )
  }

  const filtered = search ? flatFilter(tree, search) : []

  const renderNodes = (nodes: DeptNode[], depth = 0): React.ReactNode =>
    nodes.map(n => (
      <CommandItem
        key={n.id}
        value={String(n.id)}
        onSelect={() => {
          onChange(String(n.id))
          setOpen(false)
          setSearch("")
        }}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        {n.children && n.children.length > 0 && <ChevronRight className="mr-1 h-3 w-3" />}
        {n.name}
      </CommandItem>
    ))

  const renderTree = (nodes: DeptNode[], depth = 0): React.ReactNode =>
    nodes.flatMap(n => [
      <CommandItem
        key={n.id}
        value={String(n.id)}
        onSelect={() => {
          onChange(String(n.id))
          setOpen(false)
          setSearch("")
        }}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        {n.children && n.children.length > 0 && <ChevronRight className="mr-1 h-3 w-3" />}
        {n.name}
      </CommandItem>,
      ...(n.children ? (renderTree(n.children, depth + 1) as React.ReactNode[]) : []),
    ])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        >
          <span className={selectedLabel ? "" : "text-muted-foreground"}>
            {selectedLabel || placeholder || "选择部门"}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-[300px] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="搜索部门..."
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            {search ? (
              filtered.length === 0 ? (
                <CommandEmpty>未找到部门</CommandEmpty>
              ) : (
                <CommandGroup>{renderNodes(filtered)}</CommandGroup>
              )
            ) : tree.length === 0 ? (
              <CommandEmpty>加载中...</CommandEmpty>
            ) : (
              <CommandGroup>{renderTree(tree)}</CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
