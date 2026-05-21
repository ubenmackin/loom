interface StatCardProps {
  label: string
  value: string | number
}

export default function StatCard({ label, value }: StatCardProps) {
  return (
    <div className="border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark p-3 rounded-none shadow-none">
      <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted">
        {label}
      </div>
      <div className="font-mono text-xl text-neutral-800 dark:text-light-neutral mt-1">
        {value}
      </div>
    </div>
  )
}
