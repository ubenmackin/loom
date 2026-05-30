import { useState, useMemo, useRef, useCallback } from 'react'
import { X } from 'lucide-react'
import SharpTag from './SharpTag'
import type { User, Session } from '../types'


interface AssigneeOption {
  id: string
  name: string
  type: 'human' | 'session'
}

interface AssigneeSelectorProps {
  value: string
  assigneeType: string
  users: User[]
  sessions: Session[]
  onChange: (value: string, type: string) => void
}

export default function AssigneeSelector({
  value,
  assigneeType,
  users,
  sessions,
  onChange,
}: AssigneeSelectorProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const itemClickedRef = useRef(false)

  const assigneeOptions: AssigneeOption[] = useMemo(
    () => [
      ...users.map((u) => ({ id: u.id, name: u.display_name || u.username, type: 'human' as const })),
      ...sessions.map((s) => ({ id: s.id, name: s.id, type: 'session' as const })),
    ],
    [users, sessions],
  )

  const getAssigneeName = useCallback(
    (id: string): string => {
      const option = assigneeOptions.find((o) => o.id === id)
      return option?.name ?? id
    },
    [assigneeOptions],
  )

  const filteredOptions = useMemo(
    () =>
      searchTerm
        ? assigneeOptions.filter((o) => o.name.toLowerCase().includes(searchTerm.toLowerCase()))
        : assigneeOptions,
    [assigneeOptions, searchTerm],
  )

  const handleBlur = useCallback(() => {
    // Only close if a dropdown item was NOT clicked (onMouseDown fires before onBlur)
    setTimeout(() => {
      if (!itemClickedRef.current) {
        setShowDropdown(false)
      }
      itemClickedRef.current = false
    }, 0)
  }, [])

  const handleSelect = useCallback(
    (opt: AssigneeOption) => {
      itemClickedRef.current = true
      onChange(opt.id, opt.type)
      setSearchTerm('')
      setShowDropdown(false)
    },
    [onChange],
  )

  const handleClear = useCallback(() => {
    onChange('', '')
  }, [onChange])

  return (
    <div className="relative">
      {value ? (
        <div className="flex items-center gap-2 mb-1">
          <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
            {getAssigneeName(value)}
          </span>
          <SharpTag
            label={assigneeType === 'session' ? 'AGENT' : 'USER'}
            variant={assigneeType === 'session' ? 'amber' : 'primary'}
          />
          <button
            onClick={handleClear}
            className="text-[10px] uppercase tracking-wider text-red-500 hover:text-red-400 transition-colors"
          >
            <X size={14} />
          </button>
        </div>
      ) : (
        <>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value)
              setShowDropdown(true)
            }}
            onFocus={() => setShowDropdown(true)}
            onBlur={handleBlur}
            placeholder="Search users or agents..."
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          />
          {showDropdown && (
            <div className="absolute z-20 w-full mt-1 border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark max-h-48 overflow-y-auto">
              {filteredOptions.length > 0 ? (
                filteredOptions.map((opt) => (
                  <button
                    key={opt.id}
                    onMouseDown={() => handleSelect(opt)}
                    className="w-full text-left px-3 py-2 hover:bg-neutral-100 dark:hover:bg-neutral-800 flex items-center gap-2"
                  >
                    <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                      {opt.name}
                    </span>
                    <SharpTag
                      label={opt.type === 'session' ? 'AGENT' : 'USER'}
                      variant={opt.type === 'session' ? 'amber' : 'primary'}
                    />
                  </button>
                ))
              ) : (
                <div className="px-3 py-2 font-mono text-xs text-neutral-400 dark:text-neutral-500">
                  No matches
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}