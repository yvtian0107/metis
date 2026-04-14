import { LineChart, Line, ResponsiveContainer } from "recharts"

interface SparklineProps {
  data: { value: number }[]
  color?: string
  height?: number
}

export function Sparkline({ data, color = "hsl(var(--primary))", height = 24 }: SparklineProps) {
  if (!data || data.length === 0) return <div style={{ height }} />

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data}>
        <Line
          type="monotone"
          dataKey="value"
          stroke={color}
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
