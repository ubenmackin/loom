interface ConfirmModalProps {
  open: boolean
  title: string
  message: string
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmModal({ open, title, message, onConfirm, onCancel }: ConfirmModalProps) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-charcoal-dark rounded-none shadow-none border border-gray-200 dark:border-gray-border w-full max-w-md mx-4">
        {/* Header */}
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-border">
          <h2 className="text-sm font-bold text-neutral-800 dark:text-light-neutral uppercase tracking-wider">
            {title}
          </h2>
        </div>

        {/* Body */}
        <div className="px-4 py-4">
          <p className="text-sm text-neutral-600 dark:text-neutral-300">
            {message}
          </p>
        </div>

        {/* Actions */}
        <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-border flex items-center justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-4 py-2 rounded-none border border-gray-300 dark:border-gray-border text-xs font-bold uppercase tracking-wider text-neutral-600 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="glow-button"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  )
}
