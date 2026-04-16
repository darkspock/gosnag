import { useEffect, useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { api, type Project, type ProjectGroup } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ProjectCardsSkeleton } from '@/components/ui/skeleton'
import { Plus, FolderOpen, TrendingUp, TrendingDown, Minus, X, Pencil, Star, GripVertical } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { IconPicker, resolveIcon, PROJECT_COLORS } from '@/components/ui/icon-picker'
import { DndContext, closestCenter, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core'
import { SortableContext, useSortable, rectSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'

function hashColor(name: string) {
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return PROJECT_COLORS[Math.abs(hash) % PROJECT_COLORS.length]
}

function formatRelative(date: string) {
  const diff = Date.now() - new Date(date).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export default function Projects() {
  const { user } = useAuth()
  const [projects, setProjects] = useState<Project[]>([])
  const [groups, setGroups] = useState<ProjectGroup[]>([])
  const [searchParams, setSearchParams] = useSearchParams()
  const activeGroup = searchParams.get('group')
  const setActiveGroup = (id: string | null) => {
    if (id) setSearchParams({ group: id })
    else setSearchParams({})
  }
  const [favoriteIds, setFavoriteIds] = useState<Set<string>>(new Set())
  const [showCreate, setShowCreate] = useState(false)
  const [showCreateGroup, setShowCreateGroup] = useState(false)
  const [editingGroup, setEditingGroup] = useState<ProjectGroup | null>(null)
  const [showDeleteGroup, setShowDeleteGroup] = useState<ProjectGroup | null>(null)
  const [groupName, setGroupName] = useState('')
  const [groupSlackWebhook, setGroupSlackWebhook] = useState('')
  const [clearGroupSlackWebhook, setClearGroupSlackWebhook] = useState(false)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)
  const isAdmin = user?.role === 'admin'

  // dnd-kit sensors: require 8px movement before activating (avoids conflicts with clicks)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  )

  useEffect(() => {
    Promise.all([
      api.listProjects().then(setProjects),
      api.listGroups().then(setGroups),
      api.listFavorites().then(ids => setFavoriteIds(new Set(ids))),
    ]).finally(() => setLoading(false))
  }, [])

  const toggleFavorite = async (projectId: string, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const next = new Set(favoriteIds)
    if (next.has(projectId)) {
      next.delete(projectId)
      await api.removeFavorite(projectId)
    } else {
      next.add(projectId)
      await api.addFavorite(projectId)
    }
    setFavoriteIds(next)
  }

  const filteredProjects = (activeGroup
    ? projects.filter(p => p.group_id === activeGroup)
    : projects
  ).sort((a, b) => (a.position ?? 0) - (b.position ?? 0))

  const handleIconColorChange = async (projectId: string, icon: string, color: string) => {
    setProjects(prev => prev.map(p => p.id === projectId ? { ...p, icon, color } : p))
    try {
      await api.updateProject(projectId, { icon, color })
    } catch {
      // revert on error
      setProjects(await api.listProjects())
    }
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const items = [...filteredProjects]
    const fromIdx = items.findIndex(p => p.id === active.id)
    const toIdx = items.findIndex(p => p.id === over.id)
    if (fromIdx < 0 || toIdx < 0) return

    const [moved] = items.splice(fromIdx, 1)
    items.splice(toIdx, 0, moved)

    const reorder = items.map((p, i) => ({ id: p.id, position: i }))

    // Optimistic update
    setProjects(prev => prev.map(p => {
      const r = reorder.find(x => x.id === p.id)
      return r ? { ...p, position: r.position } : p
    }))

    try {
      await api.reorderProjects(reorder)
    } catch {
      setProjects(await api.listProjects())
    }
  }

  const handleCreateGroup = async () => {
    if (!groupName.trim()) return
    await api.createGroup({
      name: groupName.trim(),
      default_slack_webhook_url: groupSlackWebhook.trim() || undefined,
    })
    setGroups(await api.listGroups())
    setGroupName('')
    setGroupSlackWebhook('')
    setClearGroupSlackWebhook(false)
    setShowCreateGroup(false)
    toast.success('Group created')
  }

  const handleUpdateGroup = async () => {
    if (!editingGroup || !groupName.trim()) return
    await api.updateGroup(editingGroup.id, {
      name: groupName.trim(),
      default_slack_webhook_url: groupSlackWebhook.trim() || undefined,
      clear_default_slack_webhook_url: clearGroupSlackWebhook,
    })
    setGroups(await api.listGroups())
    setGroupName('')
    setGroupSlackWebhook('')
    setClearGroupSlackWebhook(false)
    setEditingGroup(null)
    toast.success('Group updated')
  }

  const handleDeleteGroup = async (id: string) => {
    await api.deleteGroup(id)
    setGroups(await api.listGroups())
    if (activeGroup === id) setActiveGroup(null)
    toast.success('Group deleted')
  }

  const navigate = useNavigate()

  const handleCreate = async () => {
    if (!name.trim()) return
    const created = await api.createProject({ name: name.trim() })
    setName('')
    setShowCreate(false)

    // If in a group, assign the project to it
    if (activeGroup) {
      await api.updateProject(created.id, { group_id: activeGroup })
    }

    toast.success('Project created')
    // Navigate to settings so the user can configure it
    navigate(`/projects/${created.id}/settings`)
  }

  if (loading) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-semibold">Projects</h1>
        </div>
        <ProjectCardsSkeleton />
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold">Projects</h1>
        {isAdmin && (
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setGroupName(''); setGroupSlackWebhook(''); setClearGroupSlackWebhook(false); setShowCreateGroup(true) }}>
              <Plus className="h-4 w-4 mr-1" /> New Group
            </Button>
            <Button onClick={() => setShowCreate(true)}>
              <Plus className="h-4 w-4 mr-1" /> New Project
            </Button>
          </div>
        )}
      </div>

      {/* Group tabs */}
      {groups.length > 0 && (
        <div className="flex items-center gap-1 mb-4 border-b border-border/60 overflow-x-auto">
          <button
            onClick={() => setActiveGroup(null)}
            className={cn(
              'px-3 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
              activeGroup === null
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            All
          </button>
          {groups.map(g => (
            <div key={g.id} className="relative group flex items-center">
              <button
                onClick={() => setActiveGroup(g.id)}
                className={cn(
                  'px-3 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                  activeGroup === g.id
                    ? 'border-primary text-foreground'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                )}
              >
                {g.name}
                <span className="ml-1.5 text-xs text-muted-foreground/50">
                  {projects.filter(p => p.group_id === g.id).length}
                </span>
              </button>
              {isAdmin && (
                <div className="hidden group-hover:flex items-center gap-0.5 absolute -right-1 -top-1">
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      setEditingGroup(g)
                      setGroupName(g.name)
                      setGroupSlackWebhook('')
                      setClearGroupSlackWebhook(false)
                    }}
                    className="p-0.5 rounded bg-muted hover:bg-muted/80"
                  >
                    <Pencil className="h-2.5 w-2.5 text-muted-foreground" />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); setShowDeleteGroup(g) }}
                    className="p-0.5 rounded bg-muted hover:bg-destructive/20"
                  >
                    <X className="h-2.5 w-2.5 text-muted-foreground" />
                  </button>
                </div>
              )}
            </div>
          ))}
          {isAdmin && (
            <button
              onClick={() => { setGroupName(''); setGroupSlackWebhook(''); setClearGroupSlackWebhook(false); setShowCreateGroup(true) }}
              className="px-2 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <Plus className="h-4 w-4" />
            </button>
          )}
        </div>
      )}

      {filteredProjects.length === 0 ? (
        <Card className="border-dashed">
          <CardContent className="py-16 text-center text-muted-foreground">
            <div className="relative inline-block mb-4">
              <FolderOpen className="h-12 w-12 opacity-40" />
              <div className="absolute inset-0 blur-lg bg-primary/10" />
            </div>
            <p className="text-base">No projects yet. Create one to get started.</p>
          </CardContent>
        </Card>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={filteredProjects.map(p => p.id)} strategy={rectSortingStrategy}>
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 auto-rows-fr">
              {filteredProjects.map(p => (
                <SortableProjectCard
                  key={p.id}
                  project={p}
                  isAdmin={isAdmin}
                  isFavorite={favoriteIds.has(p.id)}
                  onToggleFavorite={toggleFavorite}
                  onIconColorChange={handleIconColorChange}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogTitle>Create Project</DialogTitle>
          <DialogDescription className="sr-only">Enter a name for the new project</DialogDescription>
          <div className="mt-4 space-y-4">
            <Input
              placeholder="Project name"
              value={name}
              onChange={e => setName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreate()}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button onClick={handleCreate}>Create</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Create Group Dialog */}
      <Dialog open={showCreateGroup} onOpenChange={setShowCreateGroup}>
        <DialogContent>
          <DialogTitle>Create Group</DialogTitle>
          <DialogDescription className="sr-only">Enter a name for the new group</DialogDescription>
          <div className="mt-4 space-y-4">
            <Input
              placeholder="e.g. Production, Development, Staging"
              value={groupName}
              onChange={e => setGroupName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreateGroup()}
              autoFocus
            />
            <div>
              <label className="text-sm font-medium">Shared Slack Webhook</label>
              <Input
                placeholder="https://hooks.slack.com/..."
                value={groupSlackWebhook}
                onChange={e => setGroupSlackWebhook(e.target.value)}
                className="mt-1"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Optional. Projects in this group can use it as the default destination for Slack alerts.
              </p>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowCreateGroup(false)}>Cancel</Button>
              <Button onClick={handleCreateGroup}>Create</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Rename Group Dialog */}
      <Dialog open={!!editingGroup} onOpenChange={open => { if (!open) setEditingGroup(null) }}>
        <DialogContent>
          <DialogTitle>Edit Group</DialogTitle>
          <DialogDescription className="sr-only">Update group settings</DialogDescription>
          <div className="mt-4 space-y-4">
            <Input
              value={groupName}
              onChange={e => setGroupName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleUpdateGroup()}
              autoFocus
            />
            <div>
              <label className="text-sm font-medium">Shared Slack Webhook</label>
              <Input
                placeholder={editingGroup?.default_slack_webhook_url_set ? 'Leave empty to keep current webhook' : 'https://hooks.slack.com/...'}
                value={groupSlackWebhook}
                onChange={e => {
                  setGroupSlackWebhook(e.target.value)
                  if (e.target.value.trim() !== '') {
                    setClearGroupSlackWebhook(false)
                  }
                }}
                className="mt-1"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                {editingGroup?.default_slack_webhook_url_set
                  ? 'A shared webhook is already configured for this group. Enter a new one to replace it.'
                  : 'Optional. Projects in this group can use it as the default destination for Slack alerts.'}
              </p>
              {editingGroup?.default_slack_webhook_url_set && (
                <label className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
                  <input
                    type="checkbox"
                    checked={clearGroupSlackWebhook}
                    onChange={e => {
                      setClearGroupSlackWebhook(e.target.checked)
                      if (e.target.checked) {
                        setGroupSlackWebhook('')
                      }
                    }}
                    className="rounded border-border"
                  />
                  Clear shared webhook for this group
                </label>
              )}
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setEditingGroup(null)}>Cancel</Button>
              <Button onClick={handleUpdateGroup}>Save</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!showDeleteGroup}
        onOpenChange={open => { if (!open) setShowDeleteGroup(null) }}
        title="Delete Group"
        description={`The group${showDeleteGroup ? ` "${showDeleteGroup.name}"` : ''} will be deleted. Projects inside it will remain, but they will no longer belong to any group.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (showDeleteGroup) handleDeleteGroup(showDeleteGroup.id) }}
      />
    </div>
  )
}

function SortableProjectCard({ project: p, isAdmin, isFavorite, onToggleFavorite, onIconColorChange }: {
  project: Project
  isAdmin: boolean
  isFavorite: boolean
  onToggleFavorite: (id: string, e: React.MouseEvent) => void
  onIconColorChange: (id: string, icon: string, color: string) => void
}) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: p.id, disabled: !isAdmin })
  const navigate = useNavigate()
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const thisWeek = p.errors_this_week ?? 0
  const lastWeek = p.errors_last_week ?? 0
  const diff = thisWeek - lastWeek
  const trend = p.trend ?? []
  const cardColor = p.color || hashColor(p.name)

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn('transition-opacity', isDragging && 'opacity-40 z-50')}
      onClick={() => { if (!isDragging) navigate(`/projects/${p.id}`) }}
    >
      <Card className="h-full flex flex-col transition-all duration-200 cursor-pointer hover:-translate-y-0.5 hover:border-border/80 overflow-hidden">
        <div className="h-1" style={{ backgroundColor: cardColor }} />
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between">
            <div className="flex items-start gap-2">
              {isAdmin ? (
                <IconPicker
                  value={p.icon}
                  color={p.color}
                  fallbackColor={hashColor(p.name)}
                  onChange={(icon, color) => onIconColorChange(p.id, icon, color)}
                />
              ) : p.icon ? (
                <span className="flex items-center justify-center h-8 w-8">
                  {resolveIcon(p.icon)}
                </span>
              ) : null}
              <div>
                <CardTitle className="text-lg">{p.name}</CardTitle>
                <p className="text-sm text-muted-foreground font-mono">{p.slug}</p>
              </div>
            </div>
            <div className="flex items-center gap-0.5">
              {isAdmin && (
                <span
                  ref={setActivatorNodeRef}
                  {...attributes}
                  {...listeners}
                  className="p-1 text-muted-foreground/20 cursor-grab active:cursor-grabbing touch-none"
                  onClick={e => e.stopPropagation()}
                >
                  <GripVertical className="h-3.5 w-3.5" />
                </span>
              )}
              <button
                onClick={(e) => { e.stopPropagation(); onToggleFavorite(p.id, e) }}
                className="p-1 -mr-1 transition-colors"
              >
                <Star
                  className={cn(
                    'h-4 w-4',
                    isFavorite
                      ? 'fill-amber-400 text-amber-400'
                      : 'text-muted-foreground/30 hover:text-muted-foreground'
                  )}
                />
              </button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-0 space-y-3 flex-1 flex flex-col">
          <div className="h-10">
            <ProjectSparkline data={trend} color={cardColor} />
          </div>
          <div className="grid grid-cols-2 gap-3 pt-1 border-t border-border/40 mt-auto">
            <div>
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground/60 mb-0.5">Latest Release</p>
              <p className="text-sm font-mono truncate">{p.latest_release || '-'}</p>
              {p.latest_event && (
                <p className="text-[10px] text-muted-foreground/50">{formatRelative(p.latest_event)}</p>
              )}
            </div>
            <div className="text-right">
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground/60 mb-0.5">Errors to review</p>
              <p className="text-2xl font-semibold font-mono leading-tight">{p.open_issues ?? 0}</p>
              {(thisWeek > 0 || lastWeek > 0) && (
                <p className="text-[10px] text-muted-foreground/50 flex items-center justify-end gap-0.5">
                  {diff > 0 ? (
                    <TrendingUp className="h-2.5 w-2.5 text-red-400" />
                  ) : diff < 0 ? (
                    <TrendingDown className="h-2.5 w-2.5 text-emerald-400" />
                  ) : (
                    <Minus className="h-2.5 w-2.5 text-muted-foreground/40" />
                  )}
                  <span className={diff > 0 ? 'text-red-400' : diff < 0 ? 'text-emerald-400' : ''}>
                    {Math.abs(diff)} last 7d
                  </span>
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function ProjectSparkline({ data, color }: { data: number[]; color: string }) {
  const max = Math.max(...data, 1)
  const w = 260
  const h = 40
  const hasData = data.some(v => v > 0)

  if (!hasData) {
    return (
      <svg width="100%" height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
        <line x1={0} y1={h / 2} x2={w} y2={h / 2} stroke="currentColor" strokeWidth={1} className="text-muted-foreground/20" />
      </svg>
    )
  }

  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w
    const y = h - (v / max) * (h - 4) - 2
    return `${x},${y}`
  }).join(' ')

  const fillPoints = `0,${h} ${points} ${w},${h}`

  return (
    <svg width="100%" height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <defs>
        <linearGradient id={`grad-${color.replace('#', '')}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.2} />
          <stop offset="100%" stopColor={color} stopOpacity={0} />
        </linearGradient>
      </defs>
      <polygon points={fillPoints} fill={`url(#grad-${color.replace('#', '')})`} />
      <polyline
        points={points}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
        opacity={0.7}
      />
    </svg>
  )
}
