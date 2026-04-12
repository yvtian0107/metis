import { useEffect } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import type { CompileProgress, CompileStatus } from "../types"

interface UseCompileProgressOptions {
  kbId: number
  compileStatus: CompileStatus
  enabled: boolean
}

export function useCompileProgress({ kbId, compileStatus, enabled }: UseCompileProgressOptions) {
  const queryClient = useQueryClient()
  const isCompiling = compileStatus === "compiling"

  const { data: progress, isLoading } = useQuery({
    queryKey: ["ai-kb-compile-progress", kbId],
    queryFn: () => api.get<CompileProgress>(`/api/v1/ai/knowledge-bases/${kbId}/progress`),
    enabled: enabled && isCompiling,
    refetchInterval: isCompiling ? 2000 : false,
    refetchOnWindowFocus: isCompiling,
    staleTime: 0,
  })

  // Invalidate KB detail query when compilation completes to refresh node counts
  useEffect(() => {
    if (progress?.stage === "completed" && isCompiling) {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
    }
  }, [progress?.stage, isCompiling, kbId, queryClient])

  return {
    progress,
    isLoading: isLoading && isCompiling,
    isCompiling,
  }
}
