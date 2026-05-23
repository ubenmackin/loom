import { useState, useEffect } from 'react'
import { Check } from 'lucide-react'

interface EditableTitleProps {
  value: string
  onSave: (value: string) => void
}

export default function EditableTitle({ value, onSave }: EditableTitleProps) {
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(value)

  // Sync editValue when value changes externally (e.g., data loads)
  useEffect(() => {
    if (!editing) {
      setEditValue(value)
    }
  }, [value, editing])

  const handleSave = () => {
    if (editValue.trim() && editValue !== value) {
      onSave(editValue.trim())
    }
    setEditing(false)
  }

  const handleCancel = () => {
    setEditing(false)
    setEditValue(value)
  }

  if (editing) {
    return (
      <div className="mt-2 flex gap-2">
        <input
          type="text"
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSave()
            if (e.key === 'Escape') handleCancel()
          }}
          className="flex-1 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
          autoFocus
        />
        <button
          onClick={handleSave}
          className="glow-button px-3 py-2"
        >
          <Check size={14} />
        </button>
      </div>
    )
  }

  return (
    <button
      onClick={() => {
        setEditing(true)
        setEditValue(value)
      }}
      className="mt-1 text-left text-sm font-bold text-neutral-800 dark:text-light-neutral hover:text-loom-600 dark:hover:text-purple-active transition-colors w-full"
    >
      {value}
    </button>
  )
}
