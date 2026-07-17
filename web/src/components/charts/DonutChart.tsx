import { PieChart, Pie, Cell, ResponsiveContainer } from 'recharts'

interface DonutDataItem {
  name: string
  value: number
  color: string
}

interface DonutChartProps {
  data: DonutDataItem[]
  size?: number
  showLegend?: boolean
  className?: string
}

function LegendEntry({ payload }: { payload?: { value?: string; color?: string } }) {
  return (
    <span className="inline-flex items-center gap-1 text-xs font-mono text-neutral-600 dark:text-neutral-400">
      <span
        className="inline-block w-2 h-2 rounded-none"
        style={{ backgroundColor: payload?.color }}
      />
      {payload?.value}
    </span>
  )
}

export default function DonutChart({
  data,
  size = 160,
  showLegend = true,
  className,
}: DonutChartProps) {
  return (
    <div className={className}>
      <ResponsiveContainer width={size} height={size}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={size * 0.3}
            outerRadius={size * 0.42}
            dataKey="value"
            stroke="none"
            isAnimationActive={false}
          >
            {data.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Pie>
        </PieChart>
      </ResponsiveContainer>
      {showLegend && (
        <div className="flex flex-wrap gap-x-3 gap-y-1 mt-2 justify-center">
          {data.map((entry) => (
            <LegendEntry
              key={entry.name}
              payload={{ value: entry.name, color: entry.color }}
            />
          ))}
        </div>
      )}
    </div>
  )
}
