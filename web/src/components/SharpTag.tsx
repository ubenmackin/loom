interface SharpTagProps {
  label: string
  variant?: 'default' | 'primary' | 'amber' | 'success' | 'error'
}

const VARIANT_CLASSES: Record<NonNullable<SharpTagProps['variant']>, string> = {
  default: '',
  primary: 'sharp-tag-primary',
  amber: 'sharp-tag-amber',
  success: 'sharp-tag-success',
  error: 'sharp-tag-error',
}

export default function SharpTag({ label, variant = 'default' }: SharpTagProps) {
  return (
    <span className={`sharp-tag ${VARIANT_CLASSES[variant]}`}>
      [{label}]
    </span>
  )
}
