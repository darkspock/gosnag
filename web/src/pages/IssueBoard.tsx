import { useEffect, useState, useCallback } from 'react'
import { Link, useParams } from 'react-router-dom'
import { DndContext, DragOverlay, useDroppable, useDraggable, PointerSensor, useSensor, useSensors, type DragStartEvent, type DragEndEvent } from '@dnd-kit/core'
import { api, isApiError, type Project, type TicketWithIssue } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { Breadcrumb } from '@/components/ui/breadcrumb'

const MAIN_COLUMNS = [
  { id: 'acknowledged', label: 'Triage', color: 'border-t-amber-500' },
  { id: 'in_progress', label: 'In Progress', color: 'border-t-blue-500' },
  { id: 'in_review', label: 'In Review', color: 'border-t-purple-500' },
]

const BOTTOM_LANES = [
  { id: 'done', label: 'Done', color: 'border-t-emerald-500' },
  { id: 'wontfix', label: "Won't Fix", color: 'border-t-slate-500' },
  { id: 'escalated', label: 'Escalated', color: 'border-t-orange-500' },
]

const LEVEL_COLORS: Record<string, 'error' | 'warning' | 'secondary' | 'outline'> = {
  fatal: 'error',
  error: 'error',
  warning: 'warning',
  info: 'secondary',
  debug: 'outline',
}

const PRIORITY_LABEL: Record<number, string> = { 90: 'P1', 70: 'P2', 50: 'P3', 25: 'P4' }

function Column({ id, label, color, tickets, counts }: {
  id: string; label: string; color: string; tickets: TicketWithIssue[]; counts: Record<string, number>
}) {
  const { setNodeRef, isOver } = useDroppable({ id, disabled: id === 'escalated' })
  return (
    <div
      ref={setNodeRef}
      className={cn(
        'flex flex-col min-w-[200px] flex-1 rounded-lg border border-t-2 bg-card/50',
        color,
        isOver && 'ring-2 ring-primary/40'
      )}
    >
      <div className="px-3 py-2 flex items-center justify-between border-b">
        <span className="text-sm font-medium">{label}</span>
        <span className="text-xs text-muted-foreground font-mono">{counts[id] || 0}</span>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-2 min-h-[200px]">
        {tickets.map(t => (
          <TicketCard key={t.id} ticket={t} />
        ))}
        {tickets.length === 0 && (
          <div className="text-center py-8 text-xs text-muted-foreground/50">No tickets</div>
        )}
      </div>
    </div>
  )
}

function BottomLane({ id, label, color, tickets, counts }: {
  id: string; label: string; color: string; tickets: TicketWithIssue[]; counts: Record<string, number>
}) {
  const { setNodeRef, isOver } = useDroppable({ id })
  return (
    <div
      ref={setNodeRef}
      className={cn(
        'flex-1 min-w-[200px] rounded-lg border border-t-2 bg-card/50',
        color,
        isOver && 'ring-2 ring-primary/40'
      )}
    >
      <div className="px-3 py-1.5 flex items-center justify-between border-b">
        <span className="text-xs font-medium">{label}</span>
        <span className="text-[10px] text-muted-foreground font-mono">{counts[id] || 0}</span>
      </div>
      <div className="p-1.5 space-y-1 max-h-[180px] overflow-y-auto">
        {tickets.map(t => (
          <Link key={t.id} to={`/projects/${t.project_id}/tickets/${t.id}`} className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-accent/50 text-xs">
            <Badge variant={LEVEL_COLORS[t.issue_level] || 'outline'} className="text-[9px] shrink-0">{t.issue_level}</Badge>
            <span className="truncate">{t.issue_title}</span>
            {t.assignee_name && <span className="text-muted-foreground shrink-0 ml-auto">{t.assignee_name.split(' ')[0]}</span>}
          </Link>
        ))}
        {tickets.length === 0 && (
          <div className="text-center py-3 text-[10px] text-muted-foreground/50">Empty</div>
        )}
      </div>
    </div>
  )
}

function TicketCard({ ticket, overlay }: { ticket: TicketWithIssue; overlay?: boolean }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: ticket.id,
    disabled: ticket.status === 'escalated',
    data: { ticket },
  })

  const style = transform ? {
    transform: `translate(${transform.x}px, ${transform.y}px)`,
  } : undefined

  return (
    <div
      ref={overlay ? undefined : setNodeRef}
      style={style}
      {...(overlay ? {} : { ...listeners, ...attributes })}
      className={cn(
        'rounded-md border bg-card p-2.5 cursor-grab active:cursor-grabbing',
        isDragging && 'opacity-30',
        overlay && 'shadow-xl ring-2 ring-primary/30',
        ticket.status === 'escalated' && 'cursor-default opacity-75'
      )}
    >
      <Link
        to={`/projects/${ticket.project_id}/tickets/${ticket.id}`}
        className="block"
        onClick={e => { if (isDragging) e.preventDefault() }}
      >
        <div className="flex items-center gap-1.5 mb-1">
          <Badge variant={LEVEL_COLORS[ticket.issue_level] || 'outline'} className="text-[10px]">
            {ticket.issue_level}
          </Badge>
          {ticket.priority !== 50 && (
            <span className={cn(
              'text-[10px] font-mono px-1 py-0.5 rounded',
              ticket.priority >= 90 ? 'bg-red-500/15 text-red-400' :
              ticket.priority >= 70 ? 'bg-amber-500/15 text-amber-400' :
              'bg-blue-500/15 text-blue-400'
            )}>
              {PRIORITY_LABEL[ticket.priority] || `P${ticket.priority}`}
            </span>
          )}
          {ticket.escalated_key && (
            <span className="text-[10px] font-mono text-orange-400">{ticket.escalated_key}</span>
          )}
        </div>
        <p className="text-sm font-medium truncate">{ticket.issue_title}</p>
        <div className="flex items-center justify-between mt-1.5">
          <span className="text-[10px] text-muted-foreground font-mono">{ticket.issue_event_count} events</span>
          {ticket.assignee_name && (
            <span className="text-[10px] text-muted-foreground truncate max-w-[100px]">{ticket.assignee_name}</span>
          )}
        </div>
        {ticket.due_date && (
          <div className={cn(
            'text-[10px] mt-1',
            new Date(ticket.due_date) < new Date() ? 'text-red-400' :
            new Date(ticket.due_date).getTime() - Date.now() < 86400000 ? 'text-amber-400' :
            'text-muted-foreground'
          )}>
            Due {new Date(ticket.due_date).toLocaleDateString()}
          </div>
        )}
      </Link>
    </div>
  )
}

export default function IssueBoard() {
  const { projectId } = useParams<{ projectId: string }>()
  const [project, setProject] = useState<Project | null>(null)
  const [tickets, setTickets] = useState<TicketWithIssue[]>([])
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(true)
  const [activeTicket, setActiveTicket] = useState<TicketWithIssue | null>(null)
  const [forceConfirm, setForceConfirm] = useState<{ ticketId: string; from: string; to: string } | null>(null)

  const handleForceTransition = async () => {
    if (!forceConfirm || !projectId) return
    const { ticketId, from, to } = forceConfirm
    setForceConfirm(null)

    // Optimistic update
    setTickets(prev => prev.map(t => t.id === ticketId ? { ...t, status: to } : t))
    setCounts(prev => ({
      ...prev,
      [from]: Math.max(0, (prev[from] || 0) - 1),
      [to]: (prev[to] || 0) + 1,
    }))

    try {
      await api.updateTicket(projectId, ticketId, { status: to, force: true })
      toast.success(`Ticket → ${to.replace('_', ' ')} (forced)`)
    } catch (e: unknown) {
      // Revert
      setTickets(prev => prev.map(t => t.id === ticketId ? { ...t, status: from } : t))
      setCounts(prev => ({
        ...prev,
        [from]: (prev[from] || 0) + 1,
        [to]: Math.max(0, (prev[to] || 0) - 1),
      }))
      toast.error(e instanceof Error ? e.message : 'Failed to force transition')
    }
  }

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  const fetchData = useCallback(async () => {
    if (!projectId) return
    const [p, t, c] = await Promise.all([
      api.getProject(projectId),
      api.listTickets(projectId, { limit: 200 }),
      api.getTicketCounts(projectId),
    ])
    setProject(p)
    setTickets(t.tickets)
    setCounts(c)
    setLoading(false)
  }, [projectId])

  useEffect(() => { fetchData() }, [fetchData])

  const handleDragStart = (event: DragStartEvent) => {
    const t = event.active.data.current?.ticket as TicketWithIssue | undefined
    if (t) setActiveTicket(t)
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    setActiveTicket(null)
    const { active, over } = event
    if (!over || !projectId) return

    const ticketId = active.id as string
    const newStatus = over.id as string
    const ticket = tickets.find(t => t.id === ticketId)
    if (!ticket || ticket.status === newStatus) return

    // Optimistic update
    setTickets(prev => prev.map(t => t.id === ticketId ? { ...t, status: newStatus } : t))
    setCounts(prev => ({
      ...prev,
      [ticket.status]: Math.max(0, (prev[ticket.status] || 0) - 1),
      [newStatus]: (prev[newStatus] || 0) + 1,
    }))

    try {
      await api.updateTicket(projectId, ticketId, { status: newStatus })
      toast.success(`Ticket → ${newStatus.replace('_', ' ')}`)
    } catch (e: unknown) {
      // Revert optimistic update
      setTickets(prev => prev.map(t => t.id === ticketId ? { ...t, status: ticket.status } : t))
      setCounts(prev => ({
        ...prev,
        [ticket.status]: (prev[ticket.status] || 0) + 1,
        [newStatus]: Math.max(0, (prev[newStatus] || 0) - 1),
      }))

      // Non-standard transition: offer force option
      if (isApiError(e) && e.status === 409 && e.body.can_force) {
        setForceConfirm({ ticketId, from: ticket.status, to: newStatus })
      } else {
        toast.error(e instanceof Error ? e.message : 'Invalid transition')
      }
    }
  }

  if (loading) return <div className="p-8 text-center text-muted-foreground">Loading board...</div>

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        ...(project?.group_name ? [{ label: project.group_name, to: '/' }] : []),
        { label: project?.name || '', to: `/projects/${projectId}` },
        { label: 'Board' },
      ]} />

      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">Board</h1>
        <div className="flex gap-2">
          <Link to={`/projects/${projectId}`}>
            <button className="text-xs text-muted-foreground hover:text-foreground px-2 py-1 rounded border">
              List View
            </button>
          </Link>
        </div>
      </div>

      <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
        {/* Main workflow columns */}
        <div className="flex gap-3 overflow-x-auto pb-4">
          {MAIN_COLUMNS.map(col => (
            <Column
              key={col.id}
              id={col.id}
              label={col.label}
              color={col.color}
              tickets={tickets.filter(t => t.status === col.id)}
              counts={counts}
            />
          ))}
        </div>

        {/* Done, Won't Fix & Escalated in a horizontal row */}
        <div className="flex gap-3 mt-3 overflow-x-auto">
          {BOTTOM_LANES.map(lane => (
            <BottomLane
              key={lane.id}
              id={lane.id}
              label={lane.label}
              color={lane.color}
              tickets={tickets.filter(t => t.status === lane.id)}
              counts={counts}
            />
          ))}
        </div>

        <DragOverlay>
          {activeTicket && <TicketCard ticket={activeTicket} overlay />}
        </DragOverlay>
      </DndContext>

      {/* Force transition confirmation */}
      {forceConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-card border rounded-lg p-6 max-w-sm mx-4 shadow-xl">
            <h3 className="text-base font-semibold mb-2">Non-standard transition</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Moving from <span className="font-mono font-medium text-foreground">{forceConfirm.from.replace('_', ' ')}</span> to{' '}
              <span className="font-mono font-medium text-foreground">{forceConfirm.to.replace('_', ' ')}</span> is not a standard workflow step. Do you want to force this transition?
            </p>
            <div className="flex justify-end gap-2">
              <button
                className="px-3 py-1.5 text-sm rounded-md border hover:bg-accent transition-colors"
                onClick={() => setForceConfirm(null)}
              >
                Cancel
              </button>
              <button
                className="px-3 py-1.5 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
                onClick={handleForceTransition}
              >
                Force transition
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
