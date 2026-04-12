import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, isApiError, type Ticket, type Activity, type User, type Project, type IssueComment, type SuspectCommit } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Check, ExternalLink, ArrowUpRight, Pencil, Send, ChevronRight, AlertTriangle } from 'lucide-react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { RichEditor, RichViewer } from '@/components/ui/rich-editor'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { Breadcrumb } from '@/components/ui/breadcrumb'

const STATUS_CONFIG: Record<string, { label: string; color: string; bg: string; step: number }> = {
  acknowledged: { label: 'Acknowledged', color: 'text-amber-400', bg: 'bg-amber-500', step: 0 },
  in_progress:  { label: 'In Progress',  color: 'text-blue-400',  bg: 'bg-blue-500',  step: 1 },
  in_review:    { label: 'In Review',     color: 'text-purple-400', bg: 'bg-purple-500', step: 2 },
  done:         { label: 'Done',          color: 'text-emerald-400', bg: 'bg-emerald-500', step: 3 },
  wontfix:      { label: "Won't Fix",     color: 'text-slate-400', bg: 'bg-slate-500', step: 3 },
  escalated:    { label: 'Escalated',     color: 'text-orange-400', bg: 'bg-orange-500', step: -1 },
}

const TRANSITION_LABEL: Record<string, string> = {
  in_progress: 'Start Working',
  in_review: 'Submit for Review',
  done: 'Mark Done',
  wontfix: "Won't Fix",
  escalated: 'Escalate',
  acknowledged: 'Reopen',
}

const TRANSITION_VARIANT: Record<string, 'default' | 'outline' | 'secondary'> = {
  done: 'default',
  in_progress: 'default',
  in_review: 'secondary',
  wontfix: 'outline',
  escalated: 'outline',
  acknowledged: 'outline',
}

const WORKFLOW_STEPS = ['Acknowledged', 'In Progress', 'In Review', 'Done']

export default function TicketDetail() {
  const { projectId, ticketId } = useParams<{ projectId: string; ticketId: string }>()
  const { user: currentUser } = useAuth()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [project, setProject] = useState<Project | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [transitions, setTransitions] = useState<string[]>([])
  const [activities, setActivities] = useState<Activity[]>([])
  const [loading, setLoading] = useState(true)
  const [issueTitle, setIssueTitle] = useState('')
  const [issueCulprit, setIssueCulprit] = useState('')
  const [issueLevel, setIssueLevel] = useState('')
  const [issueEventCount, setIssueEventCount] = useState(0)
  const [issueFirstSeen, setIssueFirstSeen] = useState('')
  const [issueLastSeen, setIssueLastSeen] = useState('')
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const [editingDescription, setEditingDescription] = useState(false)
  const [descriptionDraft, setDescriptionDraft] = useState('')
  const [comments, setComments] = useState<IssueComment[]>([])
  const [commentBody, setCommentBody] = useState('')
  const [submittingComment, setSubmittingComment] = useState(false)
  const [suspectCommits, setSuspectCommits] = useState<SuspectCommit[]>([])

  const [showResolution, setShowResolution] = useState(false)
  const [resolutionType, setResolutionType] = useState('fixed')
  const [fixReference, setFixReference] = useState('')
  const [resolutionNotes, setResolutionNotes] = useState('')

  useEffect(() => {
    if (!projectId || !ticketId) return
    Promise.all([
      api.getProject(projectId).then(setProject),
      api.getTicket(projectId, ticketId).then(async t => {
        setTicket(t)
        // Only fetch issue-related data if ticket has a linked issue
        if (t.issue_id) {
          try {
            const issue = await api.getIssue(projectId, t.issue_id)
            setIssueTitle(issue.title)
            setIssueCulprit(issue.culprit || '')
            setIssueLevel(issue.level || '')
            setIssueEventCount(issue.event_count || 0)
            setIssueFirstSeen(issue.first_seen || '')
            setIssueLastSeen(issue.last_seen || '')
            api.listActivities(projectId, t.issue_id, { limit: 100 }).then(r => setActivities(r.activities)).catch(() => {})
            api.listComments(projectId, t.issue_id).then(setComments).catch(() => {})
            api.getSuspectCommits(projectId, t.issue_id).then(r => setSuspectCommits(r.commits || [])).catch(() => {})
          } catch { /* */ }
        }
        api.getTicketTransitions(projectId, ticketId).then(r => setTransitions(r.transitions)).catch(() => {})
      }),
      api.listUsers().then(setUsers),
    ]).finally(() => setLoading(false))
  }, [projectId, ticketId])

  const refreshTicket = async () => {
    if (!projectId || !ticketId) return
    try {
      const t = await api.getTicket(projectId, ticketId)
      setTicket(t)
      api.getTicketTransitions(projectId, ticketId).then(r => setTransitions(r.transitions)).catch(() => {})
      if (t.issue_id) {
        api.listActivities(projectId, t.issue_id, { limit: 100 }).then(r => setActivities(r.activities)).catch(() => {})
      }
    } catch { /* */ }
  }

  const handleStatusChange = async (newStatus: string, force = false) => {
    if (!projectId || !ticketId || !ticket) return
    if (newStatus === 'done' && !force) {
      setResolutionType('fixed')
      setFixReference('')
      setResolutionNotes('')
      setShowResolution(true)
      return
    }
    try {
      const updated = await api.updateTicket(projectId, ticketId, { status: newStatus, force: force || undefined })
      setTicket(updated)
      api.getTicketTransitions(projectId, ticketId).then(r => setTransitions(r.transitions)).catch(() => {})
      if (ticket.issue_id) {
        api.listActivities(projectId, ticket.issue_id, { limit: 100 }).then(r => setActivities(r.activities)).catch(() => {})
      }
      toast.success(`Ticket → ${STATUS_CONFIG[newStatus]?.label || newStatus}${force ? ' (forced)' : ''}`)
    } catch (e: unknown) {
      if (isApiError(e) && e.status === 409 && e.body.can_force) {
        if (confirm(`Transition from "${ticket.status}" to "${newStatus}" is not standard. Force it?`)) {
          handleStatusChange(newStatus, true)
        }
      } else {
        toast.error(e instanceof Error ? e.message : 'Failed to update ticket')
      }
    }
  }

  const handleResolve = async () => {
    if (!projectId || !ticketId || !ticket) return
    try {
      const updated = await api.updateTicket(projectId, ticketId, {
        status: 'done',
        resolution_type: resolutionType,
        fix_reference: fixReference || undefined,
        resolution_notes: resolutionNotes || undefined,
      })
      setTicket(updated)
      setShowResolution(false)
      await refreshTicket()
      toast.success('Ticket resolved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to resolve ticket')
    }
  }

  const handleAssign = async (userId: string | null) => {
    if (!projectId || !ticketId) return
    try {
      const updated = await api.updateTicket(projectId, ticketId, { assigned_to: userId || '' })
      setTicket(updated)
      await refreshTicket()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to assign')
    }
  }

  const handlePriorityChange = async (priority: number) => {
    if (!projectId || !ticketId) return
    try {
      const updated = await api.updateTicket(projectId, ticketId, { priority })
      setTicket(updated)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to change priority')
    }
  }

  const handleSaveTitle = async () => {
    if (!projectId || !ticketId) return
    const updated = await api.updateTicket(projectId, ticketId, { title: titleDraft })
    setTicket(updated)
    setEditingTitle(false)
  }

  const handleSaveDescription = async () => {
    if (!projectId || !ticketId) return
    try {
      const updated = await api.updateTicket(projectId, ticketId, { description: descriptionDraft })
      setTicket(updated)
      setEditingDescription(false)
      toast.success('Description saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save description')
    }
  }

  const handleAddComment = async () => {
    if (!projectId || !ticket || !commentBody.trim()) return
    setSubmittingComment(true)
    try {
      const created = await api.createComment(projectId, ticket.issue_id, commentBody.trim())
      setComments(prev => [...prev, created])
      setCommentBody('')
    } finally {
      setSubmittingComment(false)
    }
  }

  if (loading) return <div className="py-16 text-center text-muted-foreground animate-fade-in">Loading...</div>
  if (!ticket) return <div className="py-16 text-center text-muted-foreground">Ticket not found</div>

  const statusCfg = STATUS_CONFIG[ticket.status] || STATUS_CONFIG.acknowledged
  // priorityCfg available for future sidebar enhancements

  return (
    <div className="animate-fade-in">
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        { label: project?.name || '', to: `/projects/${projectId}` },
        { label: 'Tickets', to: `/projects/${projectId}/tickets` },
        { label: ticket.title || issueTitle || 'Ticket' },
      ]} />

      <div className="flex flex-col lg:flex-row gap-6">
        {/* Main content — left */}
        <div className="flex-1 min-w-0">

          {/* Header */}
          <div className="mb-6">
            {issueCulprit && (
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-1 font-mono">
                {issueCulprit}
              </div>
            )}
            {editingTitle ? (
              <div className="flex items-center gap-2 mb-1">
                <input
                  value={titleDraft}
                  onChange={e => setTitleDraft(e.target.value)}
                  className="text-lg font-semibold bg-background border rounded px-2 py-1 flex-1 focus:outline-none focus:ring-1 focus:ring-primary"
                  autoFocus
                  onKeyDown={e => {
                    if (e.key === 'Enter') handleSaveTitle()
                    if (e.key === 'Escape') setEditingTitle(false)
                  }}
                />
                <Button size="sm" onClick={handleSaveTitle}>Save</Button>
                <Button size="sm" variant="outline" onClick={() => setEditingTitle(false)}>Cancel</Button>
              </div>
            ) : (
              <h1
                className="text-lg font-semibold leading-tight cursor-pointer group"
                onClick={() => { setTitleDraft(ticket.title || issueTitle); setEditingTitle(true) }}
              >
                {ticket.title || issueTitle || 'Untitled ticket'}
                <Pencil className="h-3 w-3 inline ml-2 opacity-0 group-hover:opacity-40 transition-opacity" />
              </h1>
            )}
            {ticket.title && ticket.title !== issueTitle && (
              <p className="text-xs text-muted-foreground mt-0.5 truncate">{issueTitle}</p>
            )}

            {/* Workflow progress */}
            {ticket.status !== 'escalated' && ticket.status !== 'wontfix' && (
              <div className="flex items-center gap-0 mt-4">
                {WORKFLOW_STEPS.map((step, i) => {
                  const isActive = i === statusCfg.step
                  const isPast = i < statusCfg.step
                  return (
                    <div key={step} className="flex items-center flex-1 last:flex-none">
                      <div className={cn(
                        'flex items-center justify-center h-7 rounded-full text-[10px] font-medium transition-all',
                        isActive ? cn(statusCfg.bg, 'text-white px-3') :
                        isPast ? 'bg-emerald-500/20 text-emerald-400 px-3' :
                        'bg-muted text-muted-foreground/40 px-3'
                      )}>
                        {isPast ? <Check className="h-3 w-3 mr-0.5" /> : null}
                        {step}
                      </div>
                      {i < WORKFLOW_STEPS.length - 1 && (
                        <div className={cn(
                          'flex-1 h-px mx-1',
                          isPast || isActive ? 'bg-emerald-500/30' : 'bg-border'
                        )} />
                      )}
                    </div>
                  )
                })}
              </div>
            )}

            {ticket.status === 'escalated' && (
              <div className="mt-3 rounded-md bg-orange-500/10 border border-orange-500/20 px-3 py-2 text-sm flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-orange-400 shrink-0" />
                <span>Escalated to external tracker</span>
                {ticket.escalated_url && (
                  <a href={ticket.escalated_url} target="_blank" rel="noopener noreferrer" className="ml-auto font-medium text-orange-400 hover:underline flex items-center gap-1">
                    {ticket.escalated_key || 'View'} <ExternalLink className="h-3 w-3" />
                  </a>
                )}
              </div>
            )}

            {ticket.status === 'wontfix' && (
              <div className="mt-3 rounded-md bg-slate-500/10 border border-slate-500/20 px-3 py-2 text-sm text-slate-400">
                Marked as won't fix
              </div>
            )}

            {ticket.status === 'done' && ticket.resolution_type && (
              <div className="mt-3 rounded-md bg-emerald-500/10 border border-emerald-500/20 px-3 py-2 text-sm">
                <div className="flex items-center gap-2">
                  <Check className="h-4 w-4 text-emerald-400" />
                  <span className="font-medium text-emerald-400">{ticket.resolution_type.replace('_', ' ')}</span>
                  {ticket.fix_reference && <span className="font-mono text-xs text-muted-foreground ml-2">{ticket.fix_reference}</span>}
                </div>
                {ticket.resolution_notes && <p className="text-xs text-muted-foreground mt-1 ml-6">{ticket.resolution_notes}</p>}
              </div>
            )}
          </div>

          {/* Description */}
          <div className="mb-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60">Description</h3>
              {!editingDescription && (
                <button
                  onClick={() => { setDescriptionDraft(ticket.description || ''); setEditingDescription(true) }}
                  className="text-xs text-muted-foreground/50 hover:text-foreground transition-colors"
                >
                  <Pencil className="h-3 w-3" />
                </button>
              )}
            </div>
            {editingDescription ? (
              <div>
                <RichEditor
                  content={descriptionDraft}
                  onChange={setDescriptionDraft}
                  placeholder="Describe the problem, investigation notes, root cause..."
                  onImageUpload={api.uploadImage}
                />
                <div className="flex justify-end gap-2 mt-2">
                  <Button size="sm" variant="outline" onClick={() => setEditingDescription(false)}>Cancel</Button>
                  <Button size="sm" onClick={handleSaveDescription}>Save</Button>
                </div>
              </div>
            ) : ticket.description ? (
              <div
                className="cursor-pointer rounded-md p-3 -mx-3 hover:bg-card/50 transition-colors"
                onClick={() => { setDescriptionDraft(ticket.description || ''); setEditingDescription(true) }}
              >
                <RichViewer content={ticket.description} />
              </div>
            ) : (
              <button
                onClick={() => { setDescriptionDraft(''); setEditingDescription(true) }}
                className="w-full text-left text-sm text-muted-foreground/40 italic rounded-md border border-dashed p-3 hover:border-muted-foreground/30 hover:text-muted-foreground/60 transition-colors"
              >
                Click to add a description...
              </button>
            )}
          </div>

          {/* Suspect Commits */}
          {suspectCommits.length > 0 && (
            <div className="mb-6">
              <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-2">Suspect Commits</h3>
              <div className="space-y-1">
                {suspectCommits.map((c, i) => (
                  <a
                    key={c.sha}
                    href={c.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-start gap-2 px-2 py-1.5 rounded-md hover:bg-accent/50 transition-colors text-xs group"
                  >
                    <span className={cn(
                      'mt-0.5 h-4 w-4 rounded-full flex items-center justify-center shrink-0 text-[9px] font-bold',
                      i === 0 ? 'bg-red-500/20 text-red-400' : 'bg-amber-500/20 text-amber-400'
                    )}>{i + 1}</span>
                    <div className="min-w-0 flex-1">
                      <span className="font-mono text-primary/70 mr-1.5">{c.sha.slice(0, 7)}</span>
                      <span className="text-foreground">{c.message}</span>
                      <div className="text-muted-foreground mt-0.5">
                        {c.author} &middot; {new Date(c.timestamp).toLocaleDateString()}
                        {c.files.length > 0 && <> &middot; <span className="text-primary/60">{c.files.length} matching</span></>}
                      </div>
                    </div>
                    <ExternalLink className="h-3 w-3 mt-1 text-muted-foreground/20 group-hover:text-primary/50 shrink-0" />
                  </a>
                ))}
              </div>
            </div>
          )}

          {/* Comments + Activity unified stream */}
          <div className="mb-6">
            <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-3">
              Activity & Comments
            </h3>

            <div className="relative">
              {/* Timeline line */}
              <div className="absolute left-3 top-0 bottom-0 w-px bg-border" />

              <div className="space-y-0">
                {/* Merge activities and comments, sort by time ascending */}
                {[
                  ...activities.map(a => ({ type: 'activity' as const, data: a, time: new Date(a.created_at).getTime() })),
                  ...comments.map(c => ({ type: 'comment' as const, data: c, time: new Date(c.created_at).getTime() })),
                ].sort((a, b) => a.time - b.time).map((item) => {
                  if (item.type === 'comment') {
                    const c = item.data as IssueComment
                    return (
                      <div key={`c-${c.id}`} className="relative pl-9 py-2">
                        <div className="absolute left-1 top-3">
                          {c.user_avatar ? (
                            <img src={c.user_avatar} alt="" className="h-5 w-5 rounded-full ring-2 ring-background" />
                          ) : (
                            <div className="h-5 w-5 rounded-full bg-primary/20 flex items-center justify-center text-[9px] font-bold text-primary ring-2 ring-background">
                              {(c.user_name || c.user_email || '?')[0]?.toUpperCase()}
                            </div>
                          )}
                        </div>
                        <div className="rounded-md border bg-card p-3">
                          <div className="flex items-center gap-2 mb-1">
                            <span className="text-xs font-medium">{c.user_name || c.user_email}</span>
                            <span className="text-[10px] text-muted-foreground">{new Date(c.created_at).toLocaleString()}</span>
                          </div>
                          <div className="text-sm prose prose-sm prose-invert max-w-none [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded [&_code]:text-xs [&_a]:text-primary [&_p]:my-0.5">
                            <Markdown remarkPlugins={[remarkGfm]}>{c.body.replace(/@([\w.-]+)/g, '**@$1**')}</Markdown>
                          </div>
                        </div>
                      </div>
                    )
                  }

                  const a = item.data as Activity
                  return (
                    <div key={`a-${a.id}`} className="relative pl-9 py-1.5">
                      <div className="absolute left-1.5 top-2.5">
                        <div className={cn(
                          'h-4 w-4 rounded-full flex items-center justify-center ring-2 ring-background',
                          a.user_id ? 'bg-muted' : 'bg-muted'
                        )}>
                          <div className="h-1.5 w-1.5 rounded-full bg-muted-foreground/40" />
                        </div>
                      </div>
                      <p className="text-xs text-muted-foreground/70">
                        <span className="font-medium text-muted-foreground">{a.user_name || 'System'}</span>
                        {' '}
                        {a.action === 'ticket_status_changed' ? (
                          <>moved to <span className={cn('font-medium', STATUS_CONFIG[a.new_value || '']?.color)}>{STATUS_CONFIG[a.new_value || '']?.label || a.new_value}</span></>
                        ) : a.action === 'ticket_created' ? (
                          <>created this ticket</>
                        ) : a.action === 'ticket_assigned' ? (
                          <>assigned the ticket</>
                        ) : a.action === 'ticket_priority_changed' ? (
                          <>changed priority</>
                        ) : a.action === 'status_changed' ? (
                          <>changed issue status to <span className="font-mono">{a.new_value}</span></>
                        ) : a.action === 'auto_reopened' ? (
                          <>issue auto-reopened</>
                        ) : a.action === 'first_seen' ? (
                          <>issue detected</>
                        ) : (
                          <>{a.action.replace(/_/g, ' ')}</>
                        )}
                        <span className="ml-1.5 opacity-50">{new Date(a.created_at).toLocaleString()}</span>
                      </p>
                    </div>
                  )
                })}
              </div>
            </div>

            {/* Add comment (only for tickets with a linked issue) */}
            {ticket.issue_id && <div className="relative pl-9 pt-3">
              <div className="absolute left-1 top-4">
                {currentUser?.avatar_url ? (
                  <img src={currentUser.avatar_url} alt="" className="h-5 w-5 rounded-full ring-2 ring-background" />
                ) : (
                  <div className="h-5 w-5 rounded-full bg-primary/20 flex items-center justify-center text-[9px] font-bold text-primary ring-2 ring-background">
                    {(currentUser?.name || currentUser?.email || '?')[0]?.toUpperCase()}
                  </div>
                )}
              </div>
              <div className="flex gap-2 items-end">
                <textarea
                  value={commentBody}
                  onChange={e => setCommentBody(e.target.value)}
                  placeholder="Add a comment... (Markdown supported)"
                  className="flex-1 text-sm rounded-md border bg-card px-3 py-2 min-h-[44px] max-h-[200px] resize-none focus:outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground/30"
                  onKeyDown={e => {
                    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                      e.preventDefault()
                      handleAddComment()
                    }
                  }}
                />
                <Button size="sm" disabled={!commentBody.trim() || submittingComment} onClick={handleAddComment} className="h-[44px] px-3">
                  <Send className="h-4 w-4" />
                </Button>
              </div>
            </div>}
          </div>
        </div>

        {/* Sidebar — right */}
        <div className="lg:w-[280px] shrink-0">
          <div className="lg:sticky lg:top-20 space-y-4">

            {/* Actions */}
            <div className="rounded-lg border bg-card p-3 space-y-2">
              {transitions.map(t => (
                <Button
                  key={t}
                  size="sm"
                  variant={TRANSITION_VARIANT[t] || 'outline'}
                  className="w-full justify-start"
                  onClick={() => handleStatusChange(t)}
                >
                  <ChevronRight className="h-3.5 w-3.5 mr-1.5 opacity-50" />
                  {TRANSITION_LABEL[t] || t.replace('_', ' ')}
                </Button>
              ))}

              {/* Force transitions */}
              {['acknowledged', 'in_progress', 'in_review', 'done', 'wontfix', 'escalated']
                .filter(s => s !== ticket.status && !transitions.includes(s)).length > 0 && (
                <div className="pt-2 border-t">
                  <p className="text-[10px] text-muted-foreground/50 mb-1.5">Force:</p>
                  <div className="flex flex-wrap gap-1">
                    {['acknowledged', 'in_progress', 'in_review', 'done', 'wontfix', 'escalated']
                      .filter(s => s !== ticket.status && !transitions.includes(s))
                      .map(s => (
                        <button
                          key={s}
                          className="text-[10px] px-1.5 py-0.5 rounded border border-border/50 text-muted-foreground/40 hover:text-muted-foreground hover:border-border transition-colors"
                          onClick={() => handleStatusChange(s)}
                        >
                          {STATUS_CONFIG[s]?.label || s}
                        </button>
                      ))}
                  </div>
                </div>
              )}
            </div>

            {/* Details */}
            <div className="rounded-lg border bg-card p-3 space-y-3">
              <div>
                <label className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">Assignee</label>
                <Select
                  value={ticket.assigned_to || ''}
                  onChange={e => handleAssign(e.target.value || null)}
                  className="h-7 text-xs mt-0.5"
                >
                  <option value="">Unassigned</option>
                  {users.map(u => (
                    <option key={u.id} value={u.id}>{u.name || u.email}</option>
                  ))}
                </Select>
              </div>

              <div>
                <label className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">Priority</label>
                <Select
                  value={String(ticket.priority)}
                  onChange={e => handlePriorityChange(parseInt(e.target.value))}
                  className="h-7 text-xs mt-0.5"
                >
                  <option value="90">P1 Critical</option>
                  <option value="70">P2 High</option>
                  <option value="50">P3 Medium</option>
                  <option value="25">P4 Low</option>
                </Select>
              </div>

              <div className="pt-2 border-t space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-[10px] text-muted-foreground/50 uppercase tracking-wider">Level</span>
                  <span className={cn('text-xs font-medium',
                    issueLevel === 'fatal' || issueLevel === 'error' ? 'text-red-400' :
                    issueLevel === 'warning' ? 'text-amber-400' : 'text-muted-foreground'
                  )}>{issueLevel}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[10px] text-muted-foreground/50 uppercase tracking-wider">Events</span>
                  <span className="text-xs font-mono text-muted-foreground">{issueEventCount}</span>
                </div>
                {issueFirstSeen && (
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-muted-foreground/50 uppercase tracking-wider">First seen</span>
                    <span className="text-[10px] text-muted-foreground">{new Date(issueFirstSeen).toLocaleString()}</span>
                  </div>
                )}
                {issueLastSeen && (
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-muted-foreground/50 uppercase tracking-wider">Last seen</span>
                    <span className="text-[10px] text-muted-foreground">{new Date(issueLastSeen).toLocaleString()}</span>
                  </div>
                )}
                <div className="flex items-center justify-between">
                  <span className="text-[10px] text-muted-foreground/50 uppercase tracking-wider">Created</span>
                  <span className="text-[10px] text-muted-foreground">{new Date(ticket.created_at).toLocaleDateString()}</span>
                </div>
              </div>

              {ticket.issue_id && (
                <Link
                  to={`/projects/${projectId}/issues/${ticket.issue_id}`}
                  className="flex items-center gap-1.5 text-xs text-primary hover:underline pt-2 border-t"
                >
                  <ArrowUpRight className="h-3 w-3" />
                  View issue details
                </Link>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Resolution Dialog */}
      <Dialog open={showResolution} onOpenChange={setShowResolution}>
        <DialogContent>
          <DialogTitle>Resolve Ticket</DialogTitle>
          <DialogDescription className="sr-only">Provide resolution details</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <label className="text-sm font-medium">Resolution type</label>
              <Select value={resolutionType} onChange={e => setResolutionType(e.target.value)} className="mt-1">
                <option value="fixed">Fixed</option>
                <option value="wontfix">Won't fix</option>
                <option value="duplicate">Duplicate</option>
                <option value="cannot_reproduce">Cannot reproduce</option>
                <option value="by_design">By design</option>
              </Select>
            </div>
            <div>
              <label className="text-sm font-medium">Fix reference</label>
              <Input
                value={fixReference}
                onChange={e => setFixReference(e.target.value)}
                placeholder="Commit SHA, PR URL, release version..."
                className="mt-1"
              />
            </div>
            <div>
              <label className="text-sm font-medium">Notes</label>
              <textarea
                value={resolutionNotes}
                onChange={e => setResolutionNotes(e.target.value)}
                placeholder="What was the fix?"
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm min-h-[80px] resize-y"
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowResolution(false)}>Cancel</Button>
              <Button onClick={handleResolve}>
                <Check className="h-4 w-4 mr-1" /> Resolve
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
