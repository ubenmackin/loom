import { useState, useRef, useEffect, useMemo } from 'react'
import { ChevronDown } from 'lucide-react'
import { useProjectFilterStore } from '../stores/project'
import { useProjects } from '../hooks/useProjects'
import type { Project } from '../types'

export default function ProjectPicker() {
  const { selectedProjectId, setSelectedProjectId } = useProjectFilterStore()

  const { data: projects = [], isLoading } = useProjects()

  const [searchTerm, setSearchTerm] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)

  const dropdownRef = useRef<HTMLDivElement>(null)

  // Click-outside listener
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const selectedProject = useMemo(
    () => projects.find((p) => p.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  )

  const filteredProjects = useMemo(
    () =>
      searchTerm
        ? projects.filter((p) =>
            p.name.toLowerCase().includes(searchTerm.toLowerCase()),
          )
        : projects,
    [projects, searchTerm],
  )

  const toggleOpen = () => {
    setIsOpen((prev) => !prev)
    setSearchTerm('')
    setHighlightedIndex(-1)
  }

  const handleSelect = (project: Project) => {
    setSelectedProjectId(project.id)
    setIsOpen(false)
    setSearchTerm('')
    setHighlightedIndex(-1)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsOpen(false)
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlightedIndex((prev) =>
        prev < filteredProjects.length - 1 ? prev + 1 : 0,
      )
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlightedIndex((prev) =>
        prev > 0 ? prev - 1 : filteredProjects.length - 1,
      )
    }
    if (e.key === 'Enter' && highlightedIndex >= 0) {
      e.preventDefault()
      handleSelect(filteredProjects[highlightedIndex])
    }
  }

  return (
    <div ref={dropdownRef} className="relative">
      {/* Trigger button / selected indicator */}
      <button
        onClick={toggleOpen}
        role="combobox"
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-controls="project-listbox"
        className="flex items-center gap-1.5 rounded-none border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark px-2.5 py-1 font-mono text-xs text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest transition-colors"
      >
        {selectedProject ? (
          <>
            <span className="text-neutral-400 dark:text-neutral-500">
              Project:
            </span>
            <span className="text-slate-700 dark:text-neutral-300">
              {selectedProject.name}
            </span>
          </>
        ) : (
          <span className="text-neutral-400 dark:text-neutral-500 italic">
            {isLoading ? 'Loading...' : 'Select project...'}
          </span>
        )}
        <ChevronDown size={12} className="text-neutral-400 dark:text-neutral-500" />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div
          className="absolute z-20 mt-1 w-56 border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark"
          role="listbox"
          id="project-listbox"
          onMouseDown={(e) => e.preventDefault()}
          onKeyDown={handleKeyDown}
        >
          {/* Search input */}
          <div className="p-1.5">
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => {
                setSearchTerm(e.target.value)
                setHighlightedIndex(-1)
              }}
              placeholder="Search..."
              autoFocus
              className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent px-2 py-1 font-mono text-xs text-neutral-800 dark:text-light-neutral placeholder:text-neutral-400 dark:placeholder:text-neutral-500 outline-none"
            />
          </div>

          {/* Separator */}
          <div className="border-t border-gray-200 dark:border-gray-border" />

          {/* Options list */}
          <div className="max-h-40 overflow-y-auto">
            {filteredProjects.length > 0 ? (
              filteredProjects.map((project, index) => (
                <button
                  key={project.id}
                  role="option"
                  aria-selected={project.id === selectedProjectId}
                  onClick={(e) => {
                    if (e.button !== 0) return
                    handleSelect(project)
                  }}
                  className={`w-full text-left px-3 py-1.5 font-mono text-xs transition-colors ${
                    project.id === selectedProjectId
                      ? 'text-slate-700 dark:text-neutral-200 bg-gray-100 dark:bg-charcoal-darkest'
                      : index === highlightedIndex
                        ? 'text-slate-700 dark:text-neutral-200 bg-gray-50 dark:bg-charcoal-dark/50'
                        : 'text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest'
                  }`}
                >
                  {project.name}
                </button>
              ))
            ) : (
              <div className="px-3 py-2 font-mono text-xs text-neutral-400 dark:text-neutral-500">
                No projects found
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}