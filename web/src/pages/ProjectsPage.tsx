import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Pencil, Trash2 } from 'lucide-react'
import { fetchProjects, createProject, updateProject, deleteProject } from '../api/client'
import type { Project } from '../types'
import { relativeTime } from '../utils/relativeTime'
import AsyncBoundary from '../components/AsyncBoundary'
import ConfirmModal from '../components/ConfirmModal'
import FieldLabel from '../components/FieldLabel'

export default function ProjectsPage() {
  const queryClient = useQueryClient()

  const {
    data: projects,
    isLoading,
    error: queryError,
    refetch,
  } = useQuery<Project[]>({
    queryKey: ['projects'],
    queryFn: fetchProjects,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  const createMutation = useMutation({
    mutationFn: (data: { name: string; description?: string; repo_path?: string; language?: string; build_command?: string }) =>
      createProject(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Project> }) =>
      updateProject(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  // Form state
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingProject, setEditingProject] = useState<Project | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [language, setLanguage] = useState('')
  const [buildCommand, setBuildCommand] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null)

  const openCreateModal = () => {
    setEditingProject(null)
    setName('')
    setDescription('')
    setRepoPath('')
    setLanguage('')
    setBuildCommand('')
    setFormError(null)
    setIsModalOpen(true)
  }

  const openEditModal = (project: Project) => {
    setEditingProject(project)
    setName(project.name)
    setDescription(project.description ?? '')
    setRepoPath(project.repo_path ?? '')
    setLanguage(project.language ?? '')
    setBuildCommand(project.build_command ?? '')
    setFormError(null)
    setIsModalOpen(true)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) {
      setFormError('Name is required')
      return
    }

    const payload = {
      name: name.trim(),
      description: description.trim() || undefined,
      repo_path: repoPath.trim() || undefined,
      language: language.trim() || undefined,
      build_command: buildCommand.trim() || undefined,
    }

    if (editingProject) {
      updateMutation.mutate(
        { id: editingProject.id, data: payload },
        {
          onSuccess: () => {
            handleCancel()
          },
        }
      )
    } else {
      createMutation.mutate(payload, {
        onSuccess: () => {
          handleCancel()
        },
      })
    }
  }

  const handleCancel = () => {
    setEditingProject(null)
    setName('')
    setDescription('')
    setRepoPath('')
    setLanguage('')
    setBuildCommand('')
    setFormError(null)
    setIsModalOpen(false)
  }

  const handleDelete = (projectId: string) => {
    setDeleteTargetId(projectId)
  }

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
          Projects
        </span>
        <button
          onClick={openCreateModal}
          className="glow-button"
        >
          CREATE PROJECT
        </button>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        <AsyncBoundary
          isLoading={isLoading}
          error={queryError}
          onRetry={refetch}
          isEmpty={!projects || projects.length === 0}
          emptyMessage="No projects found"
        >
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-border">
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Name
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Description
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Repo Path
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Language
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Build Command
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Created At
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {projects?.map((project) => (
                <tr key={project.id} className="border-b border-gray-200 dark:border-gray-border hover:bg-gray-50 dark:hover:bg-charcoal-darkest">
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral">
                    {project.name}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral max-w-[200px] truncate">
                    {project.description ?? '-'}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral max-w-[180px] truncate">
                    {project.repo_path ?? '-'}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral">
                    {project.language ?? '-'}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral max-w-[180px] truncate">
                    <code className="text-[11px] bg-gray-100 dark:bg-charcoal-darkest px-1.5 py-0.5 rounded-none">
                      {project.build_command ?? '-'}
                    </code>
                  </td>
                  <td className="px-4 py-3 font-mono text-[10px] text-neutral-500 dark:text-neutral-400 whitespace-nowrap">
                    {relativeTime(project.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={() => openEditModal(project)}
                        disabled={isPending}
                        className="p-1.5 rounded-none text-neutral-500 hover:text-amber-primary dark:hover:text-amber-primary transition-colors"
                        aria-label="Edit project"
                      >
                        <Pencil size={14} />
                      </button>
                      <button
                        onClick={() => handleDelete(project.id)}
                        disabled={deleteMutation.isPending}
                        className="p-1.5 rounded-none text-neutral-500 hover:text-red-500 transition-colors"
                        aria-label="Delete project"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </AsyncBoundary>
      </div>

      {/* Create/Edit Project Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
          <div className="bg-white dark:bg-charcoal-dark rounded-none shadow-none border border-gray-200 dark:border-gray-border w-[480px] max-h-[90vh] overflow-y-auto">
            {/* Header */}
            <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-border flex items-center justify-between">
              <h2 className="text-[10px] uppercase tracking-widest text-neutral-800 dark:text-light-neutral font-bold">
                {editingProject ? 'Edit Project' : 'Create Project'}
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
              {/* Name */}
              <div>
                <FieldLabel>
                  Name <span className="text-red-500">*</span>
                </FieldLabel>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value)
                    if (formError) setFormError(null)
                  }}
                  placeholder="Enter project name"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                  autoFocus
                  required
                />
              </div>

              {/* Description */}
              <div>
                <FieldLabel>Description</FieldLabel>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Enter project description"
                  rows={3}
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono resize-none"
                />
              </div>

              {/* Repo Path */}
              <div>
                <FieldLabel>Repo Path</FieldLabel>
                <input
                  type="text"
                  value={repoPath}
                  onChange={(e) => setRepoPath(e.target.value)}
                  placeholder="e.g. /path/to/repo or github.com/org/repo"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                />
              </div>

              {/* Language */}
              <div>
                <FieldLabel>Language</FieldLabel>
                <input
                  type="text"
                  value={language}
                  onChange={(e) => setLanguage(e.target.value)}
                  placeholder="e.g. Go, TypeScript, Python"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                />
              </div>

              {/* Build Command */}
              <div>
                <FieldLabel>Build Command</FieldLabel>
                <input
                  type="text"
                  value={buildCommand}
                  onChange={(e) => setBuildCommand(e.target.value)}
                  placeholder="e.g. make build, npm run build"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                />
              </div>

              {/* Error */}
              {formError && (
                <p className="font-mono text-xs text-red-500">{formError}</p>
              )}

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
                  disabled={isPending}
                >
                  {isPending
                    ? 'SAVING...'
                    : editingProject
                      ? 'UPDATE'
                      : 'CREATE'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        open={deleteTargetId !== null}
        title="Delete Project"
        message="Are you sure you want to delete this project? This action cannot be undone."
        onConfirm={() => {
          if (deleteTargetId) {
            deleteMutation.mutate(deleteTargetId)
          }
          setDeleteTargetId(null)
        }}
        onCancel={() => setDeleteTargetId(null)}
      />
    </div>
  )
}