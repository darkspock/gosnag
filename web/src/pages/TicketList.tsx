import { useEffect, useState, useCallback } from 'react'
import { Link, useParams, useSearchParams, useNavigate } from 'react-router-dom'
import { api, type Project, type TicketWithIssue, type User } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ChevronLeft, ChevronRight, Plus } from 'lucide-react'
import { toast } from '@/lib/use-toast'
import { cn } from '@/lib/utils'
import { Breadcrumb } from '@/components/ui/breadcrumb'

const STATUS_STYLE: Record<string, string> = {
  acknowledged: 'bg-amber-500/15 text-amber-400',
  in_progress: 'bg-blue-500/15 text-blue-400',
  in_review: 'bg-purple-500/15 text-purple-400',
  done: 'bg-emerald-500/15 text-emerald-400',
  wontfix: 'bg-slate-500/15 text-slate-400',
  escalated: 'bg-orange-500/15 text-orange-400',
}

const STATUS_LABEL: Record<string, string> = {
  acknowledged: 'Acknowledged',
  in_progress: 'In Progress',
  in_review: 'In Review',
  done: 'Done',
  wontfix: "Won't Fix",
  escalated: 'Escalated',
}

const LEVEL_COLORS: Record<string, 'error' | 'warning' | 'secondary' | 'outline'> = {
  fatal: 'error',
  error: 'error',
  warning: 'warning',
  info: 'secondary',
  debug: 'outline',
}

const PRIORITY_LABEL: Record<number, string> = { 90: 'P1', 70: 'P2', 50: 'P3', 25: 'P4' }

export default function TicketList() {
  const { projectId } = useParams<{ projectId: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [project, setProject] = useState<Project | null>(null)
  const [tickets, setTickets] = useState<TicketWithIssue[]>([])
  const [total, setTotal] = useState(0)
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(true)
  const [users, setUsers] = useState<User[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [newTitle, setNewTitle] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [newPriority, setNewPriority] = useState('50')
  const [newAssignee, setNewAssignee] = useState('')
  const navigate = useNavigate()

  const status = searchParams.get('status') || ''
  const offset = parseInt(searchParams.get('offset') || '0', 10)
  const limit = 25

  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    if (key !== 'offset') next.delete('offset')
    setSearchParams(next)
  }

  const fetchData = useCallback(async () => {
    if (!projectId) return
    const [p, t, c, u] = await Promise.all([
      api.getProject(projectId),
      api.listTickets(projectId, { status, limit, offset }),
      api.getTicketCounts(projectId),
      api.listUsers(),
    ])
    setProject(p)
    setTickets(t.tickets)
    setTotal(t.total)
    setCounts(c)
    setUsers(u)
    setLoading(false)
  }, [projectId, status, limit, offset])

  const handleCreate = async () => {
    if (!projectId || !newTitle.trim()) return
    try {
      const ticket = await api.createManualTicket(projectId, {
        title: newTitle.trim(),
        description: newDescription.trim() || undefined,
        priority: parseInt(newPriority) || 50,
        assigned_to: newAssignee || undefined,
      })
      setShowCreate(false)
      setNewTitle('')
      setNewDescription('')
      setNewPriority('50')
      setNewAssignee('')
      toast.success('Ticket created')
      navigate(`/projects/${projectId}/tickets/${ticket.id}`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create ticket')
    }
  }

  useEffect(() => { fetchData() }, [fetchData])

  const totalAll = Object.values(counts).reduce((a, b) => a + b, 0)

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        ...(project?.group_name ? [{ label: project.group_name, to: '/' }] : []),
        { label: project?.name || '', to: `/projects/${projectId}` },
        { label: 'Tickets' },
      ]} />

      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">Tickets</h1>
        <div className="flex gap-2">
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-1" /> New Ticket
          </Button>
          <Link to={`/projects/${projectId}/board`}>
            <button className="text-xs text-muted-foreground hover:text-foreground px-2 py-1 rounded border">
              Board View
            </button>
          </Link>
          <Link to={`/projects/${projectId}`}>
            <button className="text-xs text-muted-foreground hover:text-foreground px-2 py-1 rounded border">
              Issues
            </button>
          </Link>
        </div>
      </div>

      {/* Status filter tabs */}
      <div className="flex gap-1 mb-4 flex-wrap">
        <button
          className={cn(
            'px-3 py-1.5 text-xs rounded-md border transition-colors',
            !status ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
          )}
          onClick={() => setFilter('status', '')}
        >
          All ({totalAll})
        </button>
        {['acknowledged', 'in_progress', 'in_review', 'done', 'wontfix', 'escalated'].map(s => (
          <button
            key={s}
            className={cn(
              'px-3 py-1.5 text-xs rounded-md border transition-colors',
              status === s ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
            )}
            onClick={() => setFilter('status', s)}
          >
            {STATUS_LABEL[s]} ({counts[s] || 0})
          </button>
        ))}
      </div>

      {loading ? (
        <div className="py-12 text-center text-muted-foreground">Loading...</div>
      ) : tickets.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground">
          <p className="text-base font-medium">No tickets</p>
          <p className="text-sm mt-1">Tickets are created from the issue detail page.</p>
        </div>
      ) : (
        <>
          <div className="space-y-1">
            {tickets.map(t => (
              <Link
                key={t.id}
                to={`/projects/${projectId}/tickets/${t.id}`}
                className="flex items-center gap-3 px-3 py-2.5 rounded-md border hover:bg-accent/50 transition-colors"
              >
                <span className={cn('text-[10px] font-medium px-2 py-0.5 rounded-full shrink-0', STATUS_STYLE[t.status])}>
                  {STATUS_LABEL[t.status]}
                </span>
                <Badge variant={LEVEL_COLORS[t.issue_level] || 'outline'} className="text-[10px] shrink-0">
                  {t.issue_level}
                </Badge>
                {t.priority !== 50 && (
                  <span className={cn(
                    'text-[10px] font-mono px-1 py-0.5 rounded shrink-0',
                    t.priority >= 90 ? 'bg-red-500/15 text-red-400' :
                    t.priority >= 70 ? 'bg-amber-500/15 text-amber-400' :
                    'bg-blue-500/15 text-blue-400'
                  )}>
                    {PRIORITY_LABEL[t.priority] || `P${t.priority}`}
                  </span>
                )}
                <span className="text-sm font-medium truncate min-w-0 flex-1">{t.issue_title}</span>
                <span className="text-xs text-muted-foreground font-mono shrink-0">{t.issue_event_count} ev</span>
                {t.assignee_name && (
                  <span className="text-xs text-muted-foreground truncate max-w-[100px] shrink-0">{t.assignee_name}</span>
                )}
                {t.escalated_key && (
                  <span className="text-xs font-mono text-orange-400 shrink-0">{t.escalated_key}</span>
                )}
              </Link>
            ))}
          </div>

          {/* Pagination */}
          {total > limit && (
            <div className="flex items-center justify-between mt-4 pt-4 border-t">
              <span className="text-xs text-muted-foreground">
                {offset + 1}–{Math.min(offset + limit, total)} of {total}
              </span>
              <div className="flex gap-1">
                <Button
                  variant="outline" size="sm" disabled={offset === 0}
                  onClick={() => setFilter('offset', String(Math.max(0, offset - limit)))}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline" size="sm" disabled={offset + limit >= total}
                  onClick={() => setFilter('offset', String(offset + limit))}
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {/* Create Ticket Dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogTitle>New Ticket</DialogTitle>
          <DialogDescription className="sr-only">Create a manual ticket</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <label className="text-sm font-medium">Title</label>
              <Input
                value={newTitle}
                onChange={e => setNewTitle(e.target.value)}
                placeholder="Describe the task or issue..."
                className="mt-1"
                autoFocus
              />
            </div>
            <div>
              <label className="text-sm font-medium">Description</label>
              <textarea
                value={newDescription}
                onChange={e => setNewDescription(e.target.value)}
                placeholder="Additional context... (optional)"
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm min-h-[80px] resize-y"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium">Priority</label>
                <Select value={newPriority} onChange={e => setNewPriority(e.target.value)} className="mt-1">
                  <option value="90">P1 Critical</option>
                  <option value="70">P2 High</option>
                  <option value="50">P3 Medium</option>
                  <option value="25">P4 Low</option>
                </Select>
              </div>
              <div>
                <label className="text-sm font-medium">Assignee</label>
                <Select value={newAssignee} onChange={e => setNewAssignee(e.target.value)} className="mt-1">
                  <option value="">Assign to me</option>
                  {users.map(u => (
                    <option key={u.id} value={u.id}>{u.name || u.email}</option>
                  ))}
                </Select>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button onClick={handleCreate} disabled={!newTitle.trim()}>Create Ticket</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
