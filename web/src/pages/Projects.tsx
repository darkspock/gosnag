import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Project, type ProjectGroup } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ProjectCardsSkeleton } from '@/components/ui/skeleton'
import { Plus, FolderOpen, TrendingUp, TrendingDown, Minus, X, Pencil, Star } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'

const PROJECT_COLORS = ['#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899', '#06b6d4']

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

function projectColor(name: string) {
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return PROJECT_COLORS[Math.abs(hash) % PROJECT_COLORS.length]
}

export default function Projects() {
  const { user } = useAuth()
  const [projects, setProjects] = useState<Project[]>([])
  const [groups, setGroups] = useState<ProjectGroup[]>([])
  const [activeGroup, setActiveGroup] = useState<string | null>(null)
  const [favoriteIds, setFavoriteIds] = useState<Set<string>>(new Set())
  const [showCreate, setShowCreate] = useState(false)
  const [showCreateGroup, setShowCreateGroup] = useState(false)
  const [editingGroup, setEditingGroup] = useState<ProjectGroup | null>(null)
  const [groupName, setGroupName] = useState('')
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)

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
  ).sort((a, b) => {
    const aFav = favoriteIds.has(a.id) ? 0 : 1
    const bFav = favoriteIds.has(b.id) ? 0 : 1
    return aFav - bFav
  })

  const handleCreateGroup = async () => {
    if (!groupName.trim()) return
    await api.createGroup(groupName.trim())
    setGroups(await api.listGroups())
    setGroupName('')
    setShowCreateGroup(false)
    toast.success('Group created')
  }

  const handleUpdateGroup = async () => {
    if (!editingGroup || !groupName.trim()) return
    await api.updateGroup(editingGroup.id, groupName.trim())
    setGroups(await api.listGroups())
    setGroupName('')
    setEditingGroup(null)
    toast.success('Group renamed')
  }

  const handleDeleteGroup = async (id: string) => {
    await api.deleteGroup(id)
    setGroups(await api.listGroups())
    if (activeGroup === id) setActiveGroup(null)
    toast.success('Group deleted')
  }

  const handleCreate = async () => {
    if (!name.trim()) return
    await api.createProject({ name: name.trim() })
    toast.success('Project created')
    setName('')
    setShowCreate(false)
    const updated = await api.listProjects()
    setProjects(updated)
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
        {user?.role === 'admin' && (
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setGroupName(''); setShowCreateGroup(true) }}>
              <Plus className="h-4 w-4 mr-1" /> New Group
            </Button>
            <Button onClick={() => setShowCreate(true)}>
              <Plus className="h-4 w-4 mr-1" /> New Project
            </Button>
          </div>
        )}
      </div>

      {/* Group tabs — only show if there are groups */}
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
              {user?.role === 'admin' && (
                <div className="hidden group-hover:flex items-center gap-0.5 absolute -right-1 -top-1">
                  <button
                    onClick={(e) => { e.stopPropagation(); setEditingGroup(g); setGroupName(g.name) }}
                    className="p-0.5 rounded bg-muted hover:bg-muted/80"
                  >
                    <Pencil className="h-2.5 w-2.5 text-muted-foreground" />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeleteGroup(g.id) }}
                    className="p-0.5 rounded bg-muted hover:bg-destructive/20"
                  >
                    <X className="h-2.5 w-2.5 text-muted-foreground" />
                  </button>
                </div>
              )}
            </div>
          ))}
          {user?.role === 'admin' && (
            <button
              onClick={() => { setGroupName(''); setShowCreateGroup(true) }}
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
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filteredProjects.map(p => {
            const thisWeek = p.errors_this_week ?? 0
            const lastWeek = p.errors_last_week ?? 0
            const diff = thisWeek - lastWeek
            const trend = p.trend ?? []

            return (
              <Link key={p.id} to={`/projects/${p.id}`}>
                <Card className="transition-all duration-200 cursor-pointer hover:-translate-y-0.5 hover:border-border/80 overflow-hidden">
                  <div className="h-1" style={{ backgroundColor: projectColor(p.name) }} />
                  <CardHeader className="pb-2">
                    <div className="flex items-start justify-between">
                      <div>
                        <CardTitle className="text-lg">{p.name}</CardTitle>
                        <p className="text-sm text-muted-foreground font-mono">{p.slug}</p>
                      </div>
                      <button
                        onClick={(e) => toggleFavorite(p.id, e)}
                        className="p-1 -mr-1 -mt-1 transition-colors"
                      >
                        <Star
                          className={cn(
                            'h-4 w-4',
                            favoriteIds.has(p.id)
                              ? 'fill-amber-400 text-amber-400'
                              : 'text-muted-foreground/30 hover:text-muted-foreground'
                          )}
                        />
                      </button>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-0 space-y-3">
                    {/* Sparkline */}
                    <div className="h-10">
                      <ProjectSparkline data={trend} color={projectColor(p.name)} />
                    </div>

                    {/* Stats row */}
                    <div className="grid grid-cols-2 gap-3 pt-1 border-t border-border/40">
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
              </Link>
            )
          })}
        </div>
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
          <DialogTitle>Rename Group</DialogTitle>
          <DialogDescription className="sr-only">Enter a new name for the group</DialogDescription>
          <div className="mt-4 space-y-4">
            <Input
              value={groupName}
              onChange={e => setGroupName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleUpdateGroup()}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setEditingGroup(null)}>Cancel</Button>
              <Button onClick={handleUpdateGroup}>Rename</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
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
