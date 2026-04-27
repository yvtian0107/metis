import { useState, useMemo, useCallback, useRef } from "react"
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
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { api } from "@/lib/api"

interface UserOption {
  id: number
  username: string
  email: string
  avatar: string
}

interface UserPickerProps {
  /** User ID stored as string, e.g. "42" */
  value?: string
  onChange: (value: string) => void
  disabled?: boolean
  readOnly?: boolean
  placeholder?: string
}

export function UserPicker({ value, onChange, disabled, readOnly, placeholder }: UserPickerProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const [options, setOptions] = useState<UserOption[]>([])
  const [loading, setLoading] = useState(false)
  const [fallback, setFallback] = useState(false)
  // Label set by the user's most recent selection in the dropdown
  const [pickedLabel, setPickedLabel] = useState<{ id: string; label: string } | null>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  // Resolve value to display name via query (for initial/external values)
  const numId = value ? parseInt(value, 10) : NaN
  const { data: resolvedUsername } = useQuery({
    queryKey: ["form-user-resolve", numId],
    queryFn: () =>
      api.get<{ id: number; username: string }>(`/api/v1/users/${numId}`)
        .then(res => res.username || String(numId)),
    enabled: !!value && !isNaN(numId),
    staleTime: 300_000,
  })

  const selectedLabel = useMemo(() => {
    if (!value) return ""
    // If the user just picked this value from the dropdown, use that label
    if (pickedLabel && pickedLabel.id === value) return pickedLabel.label
    // Otherwise use the resolved username from the query
    if (resolvedUsername) return resolvedUsername
    // Fallback: show the raw value while resolving
    return value
  }, [value, pickedLabel, resolvedUsername])

  const doSearch = useCallback((keyword: string) => {
    if (!keyword.trim()) {
      setOptions([])
      return
    }
    setLoading(true)
    const params = new URLSearchParams({ page: "1", pageSize: "10", keyword })
    api.get<{ items: UserOption[] }>(`/api/v1/users?${params}`)
      .then(res => {
        setOptions(res.items ?? [])
        setFallback(false)
      })
      .catch(() => {
        setFallback(true)
        setOptions([])
      })
      .finally(() => setLoading(false))
  }, [])

  const handleSearchChange = useCallback((val: string) => {
    setSearch(val)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => doSearch(val), 300)
  }, [doSearch])

  if (readOnly || disabled) {
    return <Input value={selectedLabel} disabled readOnly placeholder={placeholder} />
  }

  if (fallback) {
    return (
      <Input
        type="number"
        value={value ?? ""}
        onChange={e => onChange(e.target.value)}
        placeholder="输入用户 ID（用户搜索不可用）"
      />
    )
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex h-9 w-full items-center justify-between rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        >
          <span className={selectedLabel ? "" : "text-muted-foreground"}>
            {selectedLabel || placeholder || "选择用户"}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-[300px] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="搜索用户..."
            value={search}
            onValueChange={handleSearchChange}
          />
          <CommandList>
            {loading ? (
              <CommandEmpty>搜索中...</CommandEmpty>
            ) : options.length === 0 ? (
              <CommandEmpty>{search ? "未找到用户" : "输入关键词搜索"}</CommandEmpty>
            ) : (
              options.map(u => (
                <CommandItem
                  key={u.id}
                  value={String(u.id)}
                  onSelect={() => {
                    const id = String(u.id)
                    setPickedLabel({ id, label: u.username })
                    onChange(id)
                    setOpen(false)
                    setSearch("")
                    setOptions([])
                  }}
                >
                  <span className="font-medium">{u.username}</span>
                  {u.email && <span className="ml-2 text-muted-foreground">{u.email}</span>}
                </CommandItem>
              ))
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
