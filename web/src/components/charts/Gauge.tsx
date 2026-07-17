interface GaugeProps {
  value: number
  max: number
  label: string
  className?: string
}

function getThresholdColor(ratio: number): string {
  if (ratio < 0.5) return '#22c55e' // green
  if (ratio < 0.8) return '#f59e0b' // amber
  return '#ef4444' // red
}

export default function Gauge({ value, max, label, className }: GaugeProps) {
  const ratio = max > 0 ? Math.min(value / max, 1) : 0
  const color = getThresholdColor(ratio)

  // SVG arc parameters for a semi-circle gauge
  const size = 120
  const strokeWidth = 10
  const radius = (size - strokeWidth) / 2
  const cx = size / 2
  const cy = size / 2 + 4 // shifted down slightly for the semi-circle

  // Arc from -180° to 0° (left to right, bottom half)
  const startAngle = Math.PI // 180° (left)
  const endAngle = 2 * Math.PI // 360° / 0° (right)

  // Background arc (full semi-circle)
  const bgArc = describeArc(cx, cy, radius, startAngle, endAngle)
  // Foreground arc (proportional to value)
  const valueAngle = startAngle + ratio * Math.PI
  const valueArc = describeArc(cx, cy, radius, startAngle, valueAngle)

  return (
    <div className={`flex flex-col items-center ${className ?? ''}`}>
      <svg width={size} height={size * 0.65} viewBox={`0 0 ${size} ${size * 0.65}`}>
        {/* Background arc */}
        <path
          d={bgArc}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          className="text-neutral-200 dark:text-neutral-700"
        />
        {/* Value arc */}
        <path
          d={valueArc}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
        />
        {/* Center text */}
        <text
          x={cx}
          y={cy - 4}
          textAnchor="middle"
          className="fill-neutral-800 dark:fill-neutral-200"
          fontSize="18"
          fontWeight="bold"
          fontFamily="ui-monospace, monospace"
        >
          {value}
        </text>
        <text
          x={cx}
          y={cy + 12}
          textAnchor="middle"
          className="fill-neutral-500 dark:fill-neutral-500"
          fontSize="9"
          fontFamily="ui-monospace, monospace"
        >
          / {max}
        </text>
      </svg>
      <span className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mt-1">
        {label}
      </span>
    </div>
  )
}

/**
 * Build an SVG path `d` attribute for an arc from startAngle to endAngle.
 * Angles in radians: PI = left, 2*PI = right (semi-circle from left to right).
 */
function describeArc(
  cx: number,
  cy: number,
  r: number,
  startAngle: number,
  endAngle: number,
): string {
  const x1 = cx + r * Math.cos(startAngle)
  const y1 = cy + r * Math.sin(startAngle)
  const x2 = cx + r * Math.cos(endAngle)
  const y2 = cy + r * Math.sin(endAngle)

  const sweep = endAngle - startAngle
  const largeArcFlag = sweep > Math.PI ? 1 : 0

  return `M ${x1} ${y1} A ${r} ${r} 0 ${largeArcFlag} 1 ${x2} ${y2}`
}
