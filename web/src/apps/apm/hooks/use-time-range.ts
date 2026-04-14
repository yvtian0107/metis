import { useState, useCallback } from "react"

export interface TimeRange {
  start: string
  end: string
  label: string
}

const PRESETS = [
  { label: "last15m", minutes: 15 },
  { label: "last1h", minutes: 60 },
  { label: "last6h", minutes: 360 },
  { label: "last24h", minutes: 1440 },
  { label: "last7d", minutes: 10080 },
] as const

function makeRange(minutes: number, label: string): TimeRange {
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return {
    start: start.toISOString(),
    end: end.toISOString(),
    label,
  }
}

export function useTimeRange(defaultPreset: string = "last1h") {
  const preset = PRESETS.find((p) => p.label === defaultPreset) ?? PRESETS[1]
  const [range, setRange] = useState<TimeRange>(() => makeRange(preset.minutes, preset.label))

  const selectPreset = useCallback((label: string) => {
    const p = PRESETS.find((p) => p.label === label)
    if (p) {
      setRange(makeRange(p.minutes, p.label))
    }
  }, [])

  const refresh = useCallback(() => {
    const p = PRESETS.find((p) => p.label === range.label)
    if (p) {
      setRange(makeRange(p.minutes, p.label))
    }
  }, [range.label])

  return { range, selectPreset, refresh, presets: PRESETS }
}
