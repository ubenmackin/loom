import { BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Cell } from 'recharts'

interface BarDataItem {
  name: string
  value: number
  color: string
}

interface HorizontalBarChartProps {
  data: BarDataItem[]
  className?: string
}

export default function HorizontalBarChart({ data, className }: HorizontalBarChartProps) {
  return (
    <div className={className}>
      <ResponsiveContainer width="100%" height={Math.max(data.length * 32, 60)}>
        <BarChart
          data={data}
          layout="vertical"
          margin={{ top: 0, right: 8, bottom: 0, left: 0 }}
        >
          <XAxis type="number" hide />
          <YAxis
            type="category"
            dataKey="name"
            width={100}
            tick={{
              fontSize: 11,
              fill: 'currentColor',
              fontFamily: 'ui-monospace, monospace',
            }}
            axisLine={false}
            tickLine={false}
            className="text-neutral-600 dark:text-neutral-400"
          />
          <Bar dataKey="value" radius={[0, 2, 2, 0]} barSize={16} isAnimationActive={false}>
            {data.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
