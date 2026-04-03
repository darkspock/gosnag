import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Project } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ProjectCardsSkeleton } from '@/components/ui/skeleton'
import { Plus, FolderOpen, TrendingUp, TrendingDown, Minus } from 'lucide-react'
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
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listProjects().then(setProjects).finally(() => setLoading(false))
  }, [])

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
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-1" /> New Project
          </Button>
        )}
      </div>

      {projects.length === 0 ? (
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
          {projects.map(p => {
            const thisWeek = p.errors_this_week ?? 0
            const lastWeek = p.errors_last_week ?? 0
            const diff = thisWeek - lastWeek
            const trend = p.trend ?? []

            return (
              <Link key={p.id} to={`/projects/${p.id}`}>
                <Card className="transition-all duration-200 cursor-pointer hover:-translate-y-0.5 hover:border-border/80 overflow-hidden">
                  <div className="h-1" style={{ backgroundColor: projectColor(p.name) }} />
                  <CardHeader className="pb-2">
                    <CardTitle className="text-lg">{p.name}</CardTitle>
                    <p className="text-sm text-muted-foreground font-mono">{p.slug}</p>
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
