import { useState } from 'react'
import { X } from 'lucide-react'
import SharpTag from './SharpTag'

export interface CreateStoryData {
  title: string
  description: string
  priority: number
  requires_build: boolean
  requires_review: boolean
}

interface CreateStoryFormProps {
  open: boolean
  onSubmit: (data: CreateStoryData) => void
  onCancel: () => void
}

export default function CreateStoryForm({ open, onSubmit, onCancel }: CreateStoryFormProps) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState(0)
  const [requiresBuild, setRequiresBuild] = useState(false)
  const [requiresReview, setRequiresReview] = useState(false)
  const [error, setError] = useState('')

  if (!open) return null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      setError('Title is required')
      return
    }
    setError('')
    onSubmit({
      title: title.trim(),
      description: description.trim(),
      priority,
      requires_build: requiresBuild,
      requires_review: requiresReview,
    })
    // Reset form
    setTitle('')
    setDescription('')
    setPriority(0)
    setRequiresBuild(false)
    setRequiresReview(false)
  }

  const handleCancel = () => {
    setTitle('')
    setDescription('')
    setPriority(0)
    setRequiresBuild(false)
    setRequiresReview(false)
    setError('')
    onCancel()
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
      <div className="bg-white dark:bg-charcoal-dark rounded-none shadow-none border border-gray-200 dark:border-gray-border w-[560px] max-h-[80vh] overflow-y-auto">
        {/* Header */}
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-border flex items-center justify-between">
          <h2 className="text-[10px] uppercase tracking-widest text-neutral-800 dark:text-light-neutral font-bold">
            Create Story
          </h2>
          <button
            onClick={handleCancel}
            className="p-1 rounded-none text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors"
            aria-label="Close"
          >
            <X size={16} />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="px-4 py-4 space-y-4">
          {/* Title */}
          <div>
            <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
              Title <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => {
                setTitle(e.target.value)
                if (error) setError('')
              }}
              placeholder="Story title..."
              className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
              autoFocus
            />
            {error && (
              <p className="mt-1 font-mono text-xs text-red-500">{error}</p>
            )}
          </div>

          {/* Description */}
          <div>
            <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              placeholder="Markdown description..."
              className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            />
          </div>

          {/* Priority */}
          <div>
            <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
              Priority
            </label>
            <input
              type="number"
              value={priority}
              onChange={(e) => setPriority(parseInt(e.target.value, 10) || 0)}
              className="w-20 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
            />
          </div>

          {/* Requires Build */}
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={requiresBuild}
              onChange={(e) => setRequiresBuild(e.target.checked)}
              className="rounded-none accent-purple-active"
            />
            <SharpTag label="BUILD" variant="amber" />
            <span className="text-xs text-neutral-500 dark:text-neutral-400">
              Requires build
            </span>
          </div>

          {/* Requires Review */}
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={requiresReview}
              onChange={(e) => setRequiresReview(e.target.checked)}
              className="rounded-none accent-purple-active"
            />
            <SharpTag label="REVIEW" variant="success" />
            <span className="text-xs text-neutral-500 dark:text-neutral-400">
              Requires review
            </span>
          </div>

          {/* Actions */}
          <div className="pt-3 border-t border-gray-200 dark:border-gray-border flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={handleCancel}
              className="px-4 py-2 rounded-none border border-gray-300 dark:border-gray-border text-xs font-bold uppercase tracking-wider text-neutral-600 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="glow-button"
            >
              Create Story
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
