import { useEffect, useState, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, type Issue, type Event, type User, type Project, type IssueTag } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Label } from '@/components/ui/label'
import { Check, X, EyeOff, RotateCcw, ChevronDown, ChevronLeft, ChevronRight, Clock, Trash2, ExternalLink, Plus } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { IssueDetailSkeleton } from '@/components/ui/skeleton'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { useKeyboardShortcut } from '@/lib/use-keyboard'

export default function IssueDetail() {
  const { projectId, issueId } = useParams<{ projectId: string; issueId: string }>()
  const navigate = useNavigate()
  const [issue, setIssue] = useState<Issue | null>(null)
  const [project, setProject] = useState<Project | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [expandedEvent, setExpandedEvent] = useState<string | null>(null)
  const [showResolve, setShowResolve] = useState(false)
  const [showSnooze, setShowSnooze] = useState(false)
  const [cooldownOption, setCooldownOption] = useState('default')
  const [snoozeOption, setSnoozeOption] = useState('2h')
  const [loading, setLoading] = useState(true)
  const [eventOffset, setEventOffset] = useState(0)
  const [eventTotal, setEventTotal] = useState(0)
  const [showDelete, setShowDelete] = useState(false)
  const [creatingJira, setCreatingJira] = useState(false)
  const [issueTags, setIssueTags] = useState<IssueTag[]>([])
  const [tagInput, setTagInput] = useState('')
  const eventLimit = 25

  useEffect(() => {
    if (!projectId || !issueId) return
    Promise.all([
      api.getProject(projectId).then(setProject),
      api.getIssue(projectId, issueId).then(setIssue),
      api.listEvents(projectId, issueId, { limit: eventLimit, offset: 0 }).then(r => {
        setEvents(r.events)
        setEventTotal(r.total)
      }),
      api.listUsers().then(setUsers),
      api.listIssueTags(projectId, issueId).then(setIssueTags),
    ]).finally(() => setLoading(false))
  }, [projectId, issueId])

  useEffect(() => {
    if (!projectId || !issueId || eventOffset === 0) return
    api.listEvents(projectId, issueId, { limit: eventLimit, offset: eventOffset }).then(r => {
      setEvents(r.events)
      setEventTotal(r.total)
    })
  }, [projectId, issueId, eventOffset])

  const handleAddTag = async () => {
    if (!projectId || !issueId || !tagInput.includes(':')) return
    const [key, ...rest] = tagInput.split(':')
    const value = rest.join(':')
    if (!key || !value) return
    await api.addIssueTag(projectId, issueId, key.trim(), value.trim())
    setIssueTags(await api.listIssueTags(projectId, issueId))
    setTagInput('')
  }

  const handleRemoveTag = async (key: string, value: string) => {
    if (!projectId || !issueId) return
    await api.removeIssueTag(projectId, issueId, key, value)
    setIssueTags(await api.listIssueTags(projectId, issueId))
  }

  const handleCreateJiraTicket = async () => {
    if (!projectId || !issueId) return
    setCreatingJira(true)
    try {
      const result = await api.createJiraTicket(projectId, issueId)
      setIssue(prev => prev ? { ...prev, jira_ticket_key: result.key, jira_ticket_url: result.url } : prev)
      toast.success(`Jira ticket ${result.key} created`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create Jira ticket')
    } finally {
      setCreatingJira(false)
    }
  }

  const updateStatus = async (status: string, cooldown?: number) => {
    if (!projectId || !issueId) return
    const data: { status: string; cooldown_minutes?: number } = { status }
    if (cooldown && cooldown > 0) data.cooldown_minutes = cooldown
    const updated = await api.updateIssueStatus(projectId, issueId, data)
    setIssue(updated)
    const labels: Record<string, string> = { resolved: 'Issue resolved', ignored: 'Issue ignored', open: 'Issue reopened', snoozed: 'Issue snoozed' }
    toast.success(labels[status] || `Status changed to ${status}`)
  }

  const cooldownOptions = [
    {
      value: 'default',
      label: project
        ? `Project default (${project.default_cooldown_minutes > 0 ? formatMinutes(project.default_cooldown_minutes) : 'No cooldown'})`
        : 'Project default',
      useDefault: true,
    },
    { value: 'none', label: 'No cooldown', minutes: 0 },
    { value: '2h', label: '2 hours', minutes: 120 },
    { value: '2d', label: '2 days', minutes: 2880 },
    { value: '1w', label: '1 week', minutes: 10080 },
  ]

  const handleResolve = async () => {
    const opt = cooldownOptions.find(o => o.value === cooldownOption)
    if (opt && 'useDefault' in opt && opt.useDefault) {
      await updateStatus('resolved')
      setShowResolve(false)
      return
    }
    await updateStatus('resolved', opt?.minutes ?? 0)
    setShowResolve(false)
  }

  const snoozeOptions = [
    { value: '2h', label: '2 hours', minutes: 120 },
    { value: '2d', label: '2 days', minutes: 2880 },
    { value: '1w', label: '1 week', minutes: 10080 },
    { value: '10x', label: '10 more events', events: 10 },
    { value: '100x', label: '100 more events', events: 100 },
    { value: '1000x', label: '1,000 more events', events: 1000 },
  ]

  const handleSnooze = async () => {
    if (!projectId || !issueId) return
    const opt = snoozeOptions.find(o => o.value === snoozeOption)
    if (!opt) return
    const data: { status: string; snooze_minutes?: number; snooze_event_threshold?: number } = { status: 'snoozed' }
    if ('minutes' in opt) data.snooze_minutes = opt.minutes
    if ('events' in opt) data.snooze_event_threshold = opt.events
    const updated = await api.updateIssueStatus(projectId, issueId, data)
    setIssue(updated)
    setShowSnooze(false)
  }

  const handleAssign = async (userId: string) => {
    if (!projectId || !issueId) return
    const updated = await api.assignIssue(projectId, issueId, userId || null)
    setIssue(updated)
    toast.success(userId ? 'Issue assigned' : 'Issue unassigned')
  }

  const isActionable = issue && (issue.status === 'open' || issue.status === 'reopened')
  const isErrorLevel = issue && issue.level !== 'info' && issue.level !== 'debug'

  const shortcuts = useMemo(() => ({
    r: () => { if (isActionable && isErrorLevel) setShowResolve(true) },
    s: () => { if (isActionable) setShowSnooze(true) },
    i: () => { if (isActionable && isErrorLevel) updateStatus('ignored') },
    escape: () => { if (projectId) navigate(`/projects/${projectId}`) },
  }), [isActionable, isErrorLevel, projectId])

  useKeyboardShortcut(shortcuts)

  if (loading || !issue) {
    return <IssueDetailSkeleton />
  }

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        { label: 'Issues', to: `/projects/${projectId}` },
        { label: issue.title },
      ]} />

      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold mb-2">{issue.title}</h1>
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant={issue.level === 'error' || issue.level === 'fatal' ? 'error' : 'warning'}>
              {issue.level}
            </Badge>
            <Badge variant={issue.status === 'open' || issue.status === 'reopened' ? 'error' : issue.status === 'snoozed' ? 'warning' : 'success'}>
              {issue.status}
            </Badge>
            {issue.status === 'snoozed' && issue.snooze_until && (
              <span className="text-xs text-muted-foreground font-mono">
                until {new Date(issue.snooze_until).toLocaleString()}
              </span>
            )}
            {issue.status === 'snoozed' && issue.snooze_event_threshold && (
              <span className="text-xs text-muted-foreground font-mono">
                until {issue.snooze_event_threshold - (issue.event_count - issue.snooze_events_at_start)} more events
              </span>
            )}
            {issue.status === 'resolved' && issue.cooldown_until && (
              <span className="text-xs text-muted-foreground font-mono">
                cooldown until {new Date(issue.cooldown_until).toLocaleString()}
              </span>
            )}
            <span className="text-sm text-muted-foreground">
              <span className="font-mono">{issue.event_count}</span> events
              <span className="mx-1.5 opacity-40">&middot;</span>
              <span className="font-mono text-xs">{issue.platform}</span>
            </span>
          </div>

          {/* Tags */}
          <div className="flex flex-wrap items-center gap-1.5 mt-2">
            {issueTags.map(t => (
              <span key={`${t.key}:${t.value}`} className="inline-flex items-center gap-1 text-xs font-mono px-2 py-0.5 rounded bg-primary/10 text-primary/80">
                {t.key}:{t.value}
                <button onClick={() => handleRemoveTag(t.key, t.value)} className="hover:text-destructive">
                  <X className="h-3 w-3" />
                </button>
              </span>
            ))}
            <form onSubmit={e => { e.preventDefault(); handleAddTag() }} className="inline-flex items-center gap-1">
              <Input
                value={tagInput}
                onChange={e => setTagInput(e.target.value)}
                placeholder="key:value"
                className="h-6 w-28 text-xs px-2 py-0"
              />
              <button type="submit" disabled={!tagInput.includes(':')} className="text-muted-foreground hover:text-foreground disabled:opacity-30">
                <Plus className="h-3.5 w-3.5" />
              </button>
            </form>
          </div>
        </div>

        <div className="flex gap-2 shrink-0">
          {(issue.status === 'open' || issue.status === 'reopened') && (
            <>
              {issue.level !== 'info' && issue.level !== 'debug' && (
                <Button size="sm" onClick={() => setShowResolve(true)}>
                  <Check className="h-4 w-4 mr-1" /> Resolve
                </Button>
              )}
              <Button size="sm" variant="secondary" onClick={() => setShowSnooze(true)}>
                <Clock className="h-4 w-4 mr-1" /> Snooze
              </Button>
              {issue.level !== 'info' && issue.level !== 'debug' && (
                <Button size="sm" variant="secondary" onClick={() => updateStatus('ignored')}>
                  <EyeOff className="h-4 w-4 mr-1" /> Ignore
                </Button>
              )}
            </>
          )}
          {issue.status === 'resolved' && (
            <Button size="sm" variant="outline" onClick={() => updateStatus('open')}>
              <RotateCcw className="h-4 w-4 mr-1" /> Reopen
            </Button>
          )}
          {(issue.status === 'ignored' || issue.status === 'snoozed') && (
            <Button size="sm" variant="outline" onClick={() => updateStatus('open')}>
              <RotateCcw className="h-4 w-4 mr-1" /> Reopen
            </Button>
          )}
          {project?.jira_base_url && !issue.jira_ticket_key && (
            <Button size="sm" variant="secondary" onClick={handleCreateJiraTicket} disabled={creatingJira}>
              {creatingJira ? 'Creating...' : 'Jira'}
            </Button>
          )}
          {issue.jira_ticket_key && issue.jira_ticket_url && (
            <a href={issue.jira_ticket_url} target="_blank" rel="noopener noreferrer">
              <Button size="sm" variant="outline">
                {issue.jira_ticket_key} <ExternalLink className="h-3.5 w-3.5 ml-1" />
              </Button>
            </a>
          )}
          <Button size="sm" variant="outline" className="text-destructive hover:bg-destructive/10" onClick={() => setShowDelete(true)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3 mb-6">
        <Card className="animate-slide-up stagger-1">
          <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">First Seen</CardTitle></CardHeader>
          <CardContent><p className="text-sm font-mono">{new Date(issue.first_seen).toLocaleString()}</p></CardContent>
        </Card>
        <Card className="animate-slide-up stagger-2">
          <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">Last Seen</CardTitle></CardHeader>
          <CardContent><p className="text-sm font-mono">{new Date(issue.last_seen).toLocaleString()}</p></CardContent>
        </Card>
        <Card className="animate-slide-up stagger-3">
          <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">Assigned To</CardTitle></CardHeader>
          <CardContent>
            <Select
              value={issue.assigned_to || ''}
              onChange={e => handleAssign(e.target.value)}
              className="h-9 text-sm"
            >
              <option value="">Unassigned</option>
              {users.map(u => (
                <option key={u.id} value={u.id}>{u.name || u.email}</option>
              ))}
            </Select>
          </CardContent>
        </Card>
      </div>

      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">
          Events <span className="text-muted-foreground font-mono text-sm ml-1">({eventTotal})</span>
        </h2>
        {eventTotal > eventLimit && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">
              {eventOffset + 1}-{Math.min(eventOffset + eventLimit, eventTotal)} of {eventTotal}
            </span>
            <Button
              variant="outline" size="sm"
              disabled={eventOffset === 0}
              onClick={() => setEventOffset(Math.max(0, eventOffset - eventLimit))}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="outline" size="sm"
              disabled={eventOffset + eventLimit >= eventTotal}
              onClick={() => setEventOffset(eventOffset + eventLimit)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>
      <div className="border border-border/60 rounded-lg divide-y divide-border/60 overflow-hidden">
        {events.map(event => (
          <div key={event.id}>
            <button
              className="w-full flex items-center justify-between p-4 hover:bg-accent/50 transition-colors text-left"
              onClick={() => setExpandedEvent(expandedEvent === event.id ? null : event.id)}
            >
              <div>
                <p className="font-medium text-sm">{event.message}</p>
                <p className="text-xs text-muted-foreground font-mono mt-0.5">
                  {new Date(event.timestamp).toLocaleString()}
                  {event.environment && <span className="ml-2 text-muted-foreground/70">{event.environment}</span>}
                  {event.release && <span className="ml-2 text-muted-foreground/70">{event.release}</span>}
                  {event.server_name && <span className="ml-2 text-muted-foreground/70">{event.server_name}</span>}
                </p>
              </div>
              <ChevronDown className={cn('h-4 w-4 text-muted-foreground transition-transform duration-200', expandedEvent === event.id && 'rotate-180')} />
            </button>
            {expandedEvent === event.id && (
              <div className="px-4 pb-4 animate-fade-in">
                <EventData data={event.data} />
              </div>
            )}
          </div>
        ))}
      </div>

      <Dialog open={showResolve} onOpenChange={setShowResolve}>
        <DialogContent>
          <DialogTitle>Resolve Issue</DialogTitle>
          <DialogDescription className="sr-only">Choose a cooldown period before the issue can reopen</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <Label>Cooldown before reopen</Label>
              <RadioGroup value={cooldownOption} onValueChange={setCooldownOption} className="mt-2">
                {cooldownOptions.map(opt => (
                  <div key={opt.value} className="flex items-center gap-2.5">
                    <RadioGroupItem value={opt.value} id={`cooldown-${opt.value}`} />
                    <Label
                      htmlFor={`cooldown-${opt.value}`}
                      className={cn(
                        'cursor-pointer transition-colors font-normal',
                        cooldownOption === opt.value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground/80'
                      )}
                    >
                      {opt.label}
                    </Label>
                  </div>
                ))}
              </RadioGroup>
              <p className="text-xs text-muted-foreground mt-3">
                During cooldown, new events won't reopen the issue. After cooldown, if events arrive, it reopens.
              </p>
              {project && (
                <p className="text-xs text-muted-foreground mt-1">
                  Project default: {project.default_cooldown_minutes > 0 ? formatMinutes(project.default_cooldown_minutes) : 'No cooldown'}.
                </p>
              )}
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowResolve(false)}>
                <X className="h-4 w-4 mr-1" /> Cancel
              </Button>
              <Button onClick={handleResolve}>
                <Check className="h-4 w-4 mr-1" /> Resolve
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title="Delete Issue"
        description={`Delete "${issue.title}" and all its ${issue.event_count} events? This cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={async () => {
          if (!projectId || !issueId) return
          await api.deleteIssues(projectId, [issueId])
          toast.success('Issue deleted')
          navigate(`/projects/${projectId}`)
        }}
      />

      <Dialog open={showSnooze} onOpenChange={setShowSnooze}>
        <DialogContent>
          <DialogTitle>Snooze Issue</DialogTitle>
          <DialogDescription className="sr-only">Choose when to be reminded about this issue</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <Label>Reopen after</Label>
              <RadioGroup value={snoozeOption} onValueChange={setSnoozeOption} className="mt-2">
                <p className="text-xs text-muted-foreground mb-1">Time</p>
                {snoozeOptions.filter(o => 'minutes' in o).map(opt => (
                  <div key={opt.value} className="flex items-center gap-2.5">
                    <RadioGroupItem value={opt.value} id={`snooze-${opt.value}`} />
                    <Label
                      htmlFor={`snooze-${opt.value}`}
                      className={cn(
                        'cursor-pointer transition-colors font-normal',
                        snoozeOption === opt.value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground/80'
                      )}
                    >
                      {opt.label}
                    </Label>
                  </div>
                ))}
                <p className="text-xs text-muted-foreground mb-1 mt-3">Event count</p>
                {snoozeOptions.filter(o => 'events' in o).map(opt => (
                  <div key={opt.value} className="flex items-center gap-2.5">
                    <RadioGroupItem value={opt.value} id={`snooze-${opt.value}`} />
                    <Label
                      htmlFor={`snooze-${opt.value}`}
                      className={cn(
                        'cursor-pointer transition-colors font-normal',
                        snoozeOption === opt.value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground/80'
                      )}
                    >
                      {opt.label}
                    </Label>
                  </div>
                ))}
              </RadioGroup>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowSnooze(false)}>
                <X className="h-4 w-4 mr-1" /> Cancel
              </Button>
              <Button onClick={handleSnooze}>
                <Clock className="h-4 w-4 mr-1" /> Snooze
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function formatMinutes(minutes: number) {
  if (minutes <= 0) return 'No cooldown'
  if (minutes % 10080 === 0) {
    const weeks = minutes / 10080
    return `${weeks} week${weeks === 1 ? '' : 's'}`
  }
  if (minutes % 1440 === 0) {
    const days = minutes / 1440
    return `${days} day${days === 1 ? '' : 's'}`
  }
  if (minutes % 60 === 0) {
    const hours = minutes / 60
    return `${hours} hour${hours === 1 ? '' : 's'}`
  }
  return `${minutes} minute${minutes === 1 ? '' : 's'}`
}

function SectionHeader({ title, count }: { title: string; count?: number }) {
  return (
    <h3 className="text-sm font-semibold text-foreground/90 mb-2 flex items-center gap-2">
      {title}
      {count !== undefined && <span className="text-xs font-mono text-muted-foreground">({count})</span>}
    </h3>
  )
}

function ExceptionSection({ exception }: { exception: { values?: Array<{ type: string; value: string; stacktrace?: { frames?: Array<{ filename: string; function: string; lineno: number; colno?: number; in_app?: boolean }> } }> } }) {
  if (!exception?.values) return null
  return (
    <div className="space-y-4">
      <SectionHeader title="Exception" />
      {exception.values.map((exc, i) => (
        <div key={i} className="rounded-lg border border-border/60 overflow-hidden">
          <div className="px-4 py-3 bg-red-500/5 border-b border-red-500/10">
            <span className="font-mono text-sm font-bold text-red-400">{exc.type}</span>
            <span className="font-mono text-sm text-red-300/70">: {exc.value}</span>
          </div>
          {exc.stacktrace?.frames && (
            <div className="bg-[#0d1117] overflow-x-auto">
              <table className="w-full text-xs font-mono">
                <tbody>
                  {[...exc.stacktrace.frames].reverse().map((frame, j) => (
                    <tr
                      key={j}
                      className={cn(
                        'border-l-2 transition-colors hover:bg-white/[0.02]',
                        frame.in_app
                          ? 'border-l-primary bg-primary/[0.03] text-foreground'
                          : 'border-l-transparent text-muted-foreground'
                      )}
                    >
                      <td className="px-3 py-1.5 text-right text-muted-foreground/40 select-none w-12 shrink-0">
                        {frame.lineno}
                      </td>
                      <td className="px-3 py-1.5">
                        <span className="text-muted-foreground">{frame.filename}</span>
                        <span className="text-muted-foreground/40"> in </span>
                        <span className={frame.in_app ? 'text-primary font-semibold' : 'text-foreground/60'}>
                          {frame.function}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

const BREADCRUMB_COLORS: Record<string, string> = {
  query: 'text-purple-400',
  http: 'text-blue-400',
  navigation: 'text-cyan-400',
  error: 'text-red-400',
  warning: 'text-amber-400',
  info: 'text-sky-400',
  debug: 'text-slate-400',
  default: 'text-emerald-400',
  log: 'text-slate-400',
}

function BreadcrumbsSection({ breadcrumbs }: { breadcrumbs: { values?: Array<{ type?: string; category?: string; message?: string; data?: Record<string, unknown>; level?: string; timestamp?: string | number }> } }) {
  const values = breadcrumbs?.values
  if (!values?.length) return null
  return (
    <div>
      <SectionHeader title="Breadcrumbs" count={values.length} />
      <div className="rounded-lg border border-border/60 overflow-hidden bg-[#0d1117]">
        <table className="w-full text-xs font-mono">
          <thead>
            <tr className="border-b border-border/40 text-muted-foreground/60">
              <th className="px-3 py-2 text-left w-[140px]">Time</th>
              <th className="px-3 py-2 text-left w-[100px]">Category</th>
              <th className="px-3 py-2 text-left">Message / Data</th>
              <th className="px-3 py-2 text-left w-[60px]">Level</th>
            </tr>
          </thead>
          <tbody>
            {values.map((crumb, i) => {
              const ts = crumb.timestamp
                ? typeof crumb.timestamp === 'number'
                  ? new Date(crumb.timestamp * 1000).toLocaleTimeString()
                  : new Date(crumb.timestamp).toLocaleTimeString()
                : ''
              const cat = crumb.category || crumb.type || ''
              const colorKey = cat.includes('query') ? 'query' : cat.includes('http') ? 'http' : cat.includes('navigation') ? 'navigation' : crumb.level || 'default'
              const color = BREADCRUMB_COLORS[colorKey] || BREADCRUMB_COLORS.default
              const message = crumb.message || (crumb.data ? JSON.stringify(crumb.data) : '')
              return (
                <tr key={i} className="border-b border-border/20 hover:bg-white/[0.02] transition-colors">
                  <td className="px-3 py-1.5 text-muted-foreground/50 whitespace-nowrap">{ts}</td>
                  <td className={cn('px-3 py-1.5 whitespace-nowrap', color)}>{cat}</td>
                  <td className="px-3 py-1.5 text-foreground/80 break-all">{message}</td>
                  <td className="px-3 py-1.5 text-muted-foreground/50">{crumb.level}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function RequestSection({ request }: { request: { method?: string; url?: string; headers?: Record<string, string>; query_string?: string; data?: unknown; env?: Record<string, string> } }) {
  if (!request) return null
  return (
    <div>
      <SectionHeader title="Request" />
      <div className="rounded-lg border border-border/60 overflow-hidden">
        {(request.method || request.url) && (
          <div className="px-4 py-3 bg-blue-500/5 border-b border-blue-500/10 flex items-center gap-2">
            {request.method && (
              <span className="font-mono text-sm font-bold text-blue-400">{request.method}</span>
            )}
            {request.url && (
              <span className="font-mono text-sm text-blue-300/70 break-all">{request.url}</span>
            )}
          </div>
        )}
        <div className="bg-[#0d1117] divide-y divide-border/20">
          {request.query_string && (
            <div className="px-4 py-2">
              <span className="text-xs text-muted-foreground/50">Query: </span>
              <span className="text-xs font-mono text-foreground/80">{request.query_string}</span>
            </div>
          )}
          {request.headers && Object.keys(request.headers).length > 0 && (
            <div className="px-4 py-2">
              <p className="text-xs text-muted-foreground/50 mb-1">Headers</p>
              <table className="w-full text-xs font-mono">
                <tbody>
                  {Object.entries(request.headers).map(([key, val]) => (
                    <tr key={key} className="hover:bg-white/[0.02]">
                      <td className="py-0.5 pr-3 text-muted-foreground whitespace-nowrap align-top">{key}</td>
                      <td className="py-0.5 text-foreground/80 break-all">{val}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {request.data != null && (
            <div className="px-4 py-2">
              <p className="text-xs text-muted-foreground/50 mb-1">Body</p>
              <pre className="text-xs font-mono text-foreground/80 overflow-x-auto">
                {typeof request.data === 'string' ? request.data : JSON.stringify(request.data as object, null, 2)}
              </pre>
            </div>
          )}
          {request.env && Object.keys(request.env).length > 0 && (
            <div className="px-4 py-2">
              <p className="text-xs text-muted-foreground/50 mb-1">Environment</p>
              <table className="w-full text-xs font-mono">
                <tbody>
                  {Object.entries(request.env).map(([key, val]) => (
                    <tr key={key} className="hover:bg-white/[0.02]">
                      <td className="py-0.5 pr-3 text-muted-foreground whitespace-nowrap align-top">{key}</td>
                      <td className="py-0.5 text-foreground/80 break-all">{val}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function TagsSection({ tags }: { tags: Record<string, string> | Array<[string, string]> }) {
  const entries = Array.isArray(tags) ? tags : Object.entries(tags)
  if (!entries.length) return null
  return (
    <div>
      <SectionHeader title="Tags" count={entries.length} />
      <div className="flex flex-wrap gap-1.5">
        {entries.map(([key, val], i) => (
          <span key={i} className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-[#0d1117] px-2 py-1 text-xs font-mono">
            <span className="text-muted-foreground">{key}:</span>
            <span className="text-foreground/90">{val}</span>
          </span>
        ))}
      </div>
    </div>
  )
}

function ContextsSection({ contexts }: { contexts: Record<string, Record<string, unknown>> }) {
  const entries = Object.entries(contexts).filter(([, v]) => v && typeof v === 'object' && Object.keys(v).length > 0)
  if (!entries.length) return null
  return (
    <div>
      <SectionHeader title="Contexts" />
      <div className="grid gap-3 md:grid-cols-2">
        {entries.map(([name, ctx]) => (
          <div key={name} className="rounded-lg border border-border/60 overflow-hidden">
            <div className="px-3 py-2 bg-accent/30 border-b border-border/40">
              <span className="text-xs font-semibold text-foreground/80 capitalize">{name}</span>
            </div>
            <div className="bg-[#0d1117] px-3 py-2">
              <table className="w-full text-xs font-mono">
                <tbody>
                  {Object.entries(ctx).filter(([k]) => k !== 'type').map(([key, val]) => (
                    <tr key={key} className="hover:bg-white/[0.02]">
                      <td className="py-0.5 pr-3 text-muted-foreground whitespace-nowrap align-top">{key}</td>
                      <td className="py-0.5 text-foreground/80 break-all">{typeof val === 'object' ? JSON.stringify(val) : String(val)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function UserSection({ user }: { user: { id?: string; email?: string; username?: string; ip_address?: string; [key: string]: unknown } }) {
  const entries = Object.entries(user).filter(([, v]) => v != null && v !== '')
  if (!entries.length) return null
  return (
    <div>
      <SectionHeader title="User" />
      <div className="flex flex-wrap gap-1.5">
        {entries.map(([key, val]) => (
          <span key={key} className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-[#0d1117] px-2 py-1 text-xs font-mono">
            <span className="text-muted-foreground">{key}:</span>
            <span className="text-foreground/90">{String(val)}</span>
          </span>
        ))}
      </div>
    </div>
  )
}

function EventData({ data }: { data: Record<string, unknown> }) {
  const exception = data.exception as { values?: Array<{ type: string; value: string; stacktrace?: { frames?: Array<{ filename: string; function: string; lineno: number; colno?: number; in_app?: boolean }> } }> } | undefined
  const request = data.request as { method?: string; url?: string; headers?: Record<string, string>; query_string?: string; data?: unknown; env?: Record<string, string> } | undefined
  const breadcrumbs = data.breadcrumbs as { values?: Array<{ type?: string; category?: string; message?: string; data?: Record<string, unknown>; level?: string; timestamp?: string | number }> } | undefined
  const tags = data.tags as Record<string, string> | Array<[string, string]> | undefined
  const contexts = data.contexts as Record<string, Record<string, unknown>> | undefined
  const user = data.user as { id?: string; email?: string; username?: string; ip_address?: string; [key: string]: unknown } | undefined

  const hasStructured = exception?.values || request || breadcrumbs?.values?.length || tags || contexts || user

  if (!hasStructured) {
    return (
      <pre className="bg-[#0d1117] rounded-lg p-4 text-xs font-mono overflow-x-auto max-h-96 text-foreground/80 border border-border/60">
        {JSON.stringify(data, null, 2)}
      </pre>
    )
  }

  return (
    <div className="space-y-5">
      {exception && <ExceptionSection exception={exception} />}
      {request && <RequestSection request={request} />}
      {breadcrumbs?.values && breadcrumbs.values.length > 0 && <BreadcrumbsSection breadcrumbs={breadcrumbs} />}
      {tags && <TagsSection tags={tags} />}
      {contexts && <ContextsSection contexts={contexts} />}
      {user && <UserSection user={user} />}
    </div>
  )
}
