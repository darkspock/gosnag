import { useEffect, useState, useMemo, useRef } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuth } from '@/lib/use-auth'
import { api, type Issue, type Event, type User, type Project, type IssueTag, type IssueComment, type Ticket, type SuspectCommit, type ReleaseInfo, type MergeSuggestion, type AIAnalysis } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Label } from '@/components/ui/label'
import { Check, X, EyeOff, Bookmark, RotateCcw, ChevronDown, ChevronLeft, ChevronRight, Clock, Trash2, ExternalLink, Plus, Copy, MessageSquare, Pencil, Send, GitMerge, Sparkles, ClipboardCopy } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { IssueDetailSkeleton } from '@/components/ui/skeleton'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { useKeyboardShortcut } from '@/lib/use-keyboard'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

export default function IssueDetail() {
  const { projectId, issueId } = useParams<{ projectId: string; issueId: string }>()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
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
  const [titleExpanded, setTitleExpanded] = useState(false)
  const [creatingJira, setCreatingJira] = useState(false)
  const [creatingGithub, setCreatingGithub] = useState(false)
  const [issueTags, setIssueTags] = useState<IssueTag[]>([])
  const [tagInput, setTagInput] = useState('')
  const [followed, setFollowed] = useState(false)
  const [followers, setFollowers] = useState<{ id: string; name: string; email: string }[]>([])
  const [comments, setComments] = useState<IssueComment[]>([])
  const [commentBody, setCommentBody] = useState('')
  const [editingComment, setEditingComment] = useState<string | null>(null)
  const [editBody, setEditBody] = useState('')
  const [submittingComment, setSubmittingComment] = useState(false)
  const [commentPreview, setCommentPreview] = useState(false)
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const [mentionIndex, setMentionIndex] = useState(0)
  const commentRef = useRef<HTMLTextAreaElement>(null)
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [creatingTicket, setCreatingTicket] = useState(false)
  const [suspectCommits, setSuspectCommits] = useState<SuspectCommit[]>([])
  const [releaseInfo, setReleaseInfo] = useState<ReleaseInfo | null>(null)
  const [mergeSuggestion, setMergeSuggestion] = useState<MergeSuggestion | null>(null)
  const [analysis, setAnalysis] = useState<AIAnalysis | null>(null)
  const [analyzing, setAnalyzing] = useState(false)
  const [analysisExpanded, setAnalysisExpanded] = useState(true)
  const eventLimit = 25

  useEffect(() => {
    if (!projectId || !issueId) return
    Promise.all([
      api.getProject(projectId).then(setProject),
      api.getIssue(projectId, issueId).then(i => { setIssue(i); setFollowed(!!i.followed); setFollowers(i.followers || []) }),
      api.listEvents(projectId, issueId, { limit: eventLimit, offset: 0 }).then(r => {
        setEvents(r.events)
        setEventTotal(r.total)
      }),
      api.listUsers().then(setUsers),
      api.listIssueTags(projectId, issueId).then(setIssueTags),
      api.listComments(projectId, issueId).then(setComments),
      api.getTicketByIssue(projectId, issueId).then(r => { if (r.ticket) setTicket(r.ticket) }).catch(() => {}),
      api.getSuspectCommits(projectId, issueId).then(r => setSuspectCommits(r.commits || [])).catch(() => {}),
      api.getReleaseInfo(projectId, issueId).then(setReleaseInfo).catch(() => {}),
      api.getMergeSuggestion(projectId, issueId).then(r => setMergeSuggestion(r.suggestion)).catch(() => {}),
      api.getAnalysis(projectId, issueId).then(r => setAnalysis(r.analysis)).catch(() => {}),
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

  const handleCreateTicket = async () => {
    if (!projectId || !issueId) return
    setCreatingTicket(true)
    try {
      const t = await api.createTicket(projectId, issueId)
      setTicket(t)
      toast.success('Ticket created')
      navigate(`/projects/${projectId}/tickets/${t.id}`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create ticket')
    } finally {
      setCreatingTicket(false)
    }
  }

  const mentionUsers = useMemo(() => {
    if (mentionQuery === null) return []
    const q = mentionQuery.toLowerCase()
    return users.filter(u => (u.name || u.email).toLowerCase().includes(q)).slice(0, 5)
  }, [mentionQuery, users])

  const handleCommentChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setCommentBody(val)
    // Detect @mention trigger
    const pos = e.target.selectionStart
    const before = val.slice(0, pos)
    const match = before.match(/@([\w.-]*)$/)
    if (match) {
      setMentionQuery(match[1])
      setMentionIndex(0)
    } else {
      setMentionQuery(null)
    }
  }

  const insertMention = (user: User) => {
    const textarea = commentRef.current
    if (!textarea) return
    const pos = textarea.selectionStart
    const before = commentBody.slice(0, pos)
    const after = commentBody.slice(pos)
    const mentionStart = before.lastIndexOf('@')
    const handle = user.email.split('@')[0] // use email prefix as handle
    const newBody = before.slice(0, mentionStart) + `@${handle} ` + after
    setCommentBody(newBody)
    setMentionQuery(null)
    textarea.focus()
  }

  const handleCommentKeyDown = (e: React.KeyboardEvent) => {
    if (mentionQuery !== null && mentionUsers.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setMentionIndex(i => Math.min(i + 1, mentionUsers.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setMentionIndex(i => Math.max(i - 1, 0))
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        insertMention(mentionUsers[mentionIndex])
      } else if (e.key === 'Escape') {
        setMentionQuery(null)
      }
    }
  }

  const handleCreateGithubIssue = async () => {
    if (!projectId || !issueId) return
    setCreatingGithub(true)
    try {
      const result = await api.createGithubIssue(projectId, issueId)
      setIssue(prev => prev ? { ...prev, github_issue_number: result.number, github_issue_url: result.url } : prev)
      toast.success(`GitHub issue #${result.number} created`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create GitHub issue')
    } finally {
      setCreatingGithub(false)
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

  const handleAnalyze = async () => {
    if (!projectId || !issueId) return
    setAnalyzing(true)
    try {
      const result = await api.analyzeIssue(projectId, issueId)
      setAnalysis(result)
      setAnalysisExpanded(true)
      toast.success('Analysis complete')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Analysis failed')
    } finally {
      setAnalyzing(false)
    }
  }

  const handleCopyAnalysisToTicket = async () => {
    if (!projectId || !analysis || !ticket) return
    const content = `<h3>AI Root Cause Analysis</h3>\n<p><strong>Summary:</strong> ${analysis.summary}</p>\n<h4>Evidence</h4>\n<ul>${analysis.evidence.map(e => `<li>${e}</li>`).join('')}</ul>\n<h4>Suggested Fix</h4>\n<p>${analysis.suggested_fix}</p>`
    const existing = ticket.description || ''
    const newDesc = existing ? `${existing}\n<hr/>\n${content}` : content
    try {
      const updated = await api.updateTicket(projectId, ticket.id, { description: newDesc })
      setTicket(updated)
      toast.success('Analysis copied to ticket')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to copy')
    }
  }

  const handleAcceptMerge = async () => {
    if (!projectId || !issueId || !mergeSuggestion) return
    try {
      await api.acceptMergeSuggestion(projectId, issueId)
      // Perform the actual merge
      await api.mergeIssues(projectId, mergeSuggestion.target_issue_id, [issueId])
      toast.success('Issues merged')
      navigate(`/projects/${projectId}/issues/${mergeSuggestion.target_issue_id}`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to merge')
    }
  }

  const handleDismissMerge = async () => {
    if (!projectId || !issueId) return
    try {
      await api.dismissMergeSuggestion(projectId, issueId)
      setMergeSuggestion(null)
      toast.success('Suggestion dismissed')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to dismiss')
    }
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
        { label: project?.name || 'Issues', to: `/projects/${projectId}` },
        { label: issue.title },
      ]} />

      {mergeSuggestion && (
        <div className="mb-4 rounded-lg border border-primary/30 bg-primary/5 p-4">
          <div className="flex items-start gap-3">
            <GitMerge className="h-5 w-5 text-primary shrink-0 mt-0.5" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium">AI Merge Suggestion</p>
              <p className="text-sm text-muted-foreground mt-0.5">
                This issue looks similar to{' '}
                <Link to={`/projects/${projectId}/issues/${mergeSuggestion.target_issue_id}`} className="text-primary hover:underline font-medium">
                  {mergeSuggestion.target_issue_title}
                </Link>
                {' '}({Math.round(mergeSuggestion.confidence * 100)}% confidence)
              </p>
              {mergeSuggestion.reason && (
                <p className="text-xs text-muted-foreground mt-1">{mergeSuggestion.reason}</p>
              )}
              <div className="flex gap-2 mt-3">
                <Button size="sm" onClick={handleAcceptMerge}>
                  <GitMerge className="h-3.5 w-3.5 mr-1" /> Merge into target
                </Button>
                <Button size="sm" variant="outline" onClick={handleDismissMerge}>
                  Dismiss
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="flex items-start justify-between mb-6">
        <div>
          {project?.issue_display_mode === 'detailed' ? (() => {
            const colonIdx = issue.title.indexOf(': ')
            const exceptionType = colonIdx > 0 && colonIdx < 60 ? issue.title.slice(0, colonIdx) : ''
            const message = exceptionType ? issue.title.slice(colonIdx + 2) : issue.title
            return (
              <>
                {issue.culprit && (
                  <h1 className="text-xl font-semibold mb-1">{issue.culprit}</h1>
                )}
                <p className="text-base text-muted-foreground mb-2">
                  {exceptionType ? <>{exceptionType}{message && <> - {message}</>}</> : issue.title}
                </p>
              </>
            )
          })() : (
            <>
              <h1
                className={cn(
                  'text-xl font-semibold mb-1',
                  !titleExpanded && 'line-clamp-3 cursor-pointer'
                )}
                onClick={() => !titleExpanded && setTitleExpanded(true)}
                title={!titleExpanded ? 'Click to expand' : undefined}
              >{issue.title}</h1>
              {issue.culprit && (
                <p className="text-sm text-muted-foreground font-mono mb-2">{issue.culprit}</p>
              )}
            </>
          )}
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground flex-nowrap">
            <Badge variant={issue.level === 'error' || issue.level === 'fatal' ? 'error' : 'warning'}>
              {issue.level}
            </Badge>
            <Badge variant={issue.status === 'open' || issue.status === 'reopened' ? 'error' : issue.status === 'snoozed' ? 'warning' : 'success'}>
              {issue.status}
            </Badge>
            {issue.status === 'snoozed' && issue.snooze_until && (
              <span className="font-mono">until {new Date(issue.snooze_until).toLocaleString()}</span>
            )}
            {issue.status === 'snoozed' && issue.snooze_event_threshold && (
              <span className="font-mono">until {issue.snooze_event_threshold - (issue.event_count - issue.snooze_events_at_start)} more events</span>
            )}
            {issue.status === 'resolved' && issue.cooldown_until && (
              <span className="font-mono">cooldown until {new Date(issue.cooldown_until).toLocaleString()}</span>
            )}
            <span className="opacity-30 shrink-0">&middot;</span>
            <span className="font-mono shrink-0">{issue.event_count} events</span>
            <span className="opacity-30 shrink-0">&middot;</span>
            <span className="shrink-0">{issue.platform}</span>
            <span className="opacity-30 shrink-0">&middot;</span>
            <span className="font-mono shrink-0">{new Date(issue.first_seen).toLocaleDateString()} &ndash; {new Date(issue.last_seen).toLocaleDateString()}</span>
            {followers.length > 0 && (
              <>
                <span className="opacity-30 shrink-0">&middot;</span>
                <span className="truncate min-w-0">
                  <Bookmark className="h-3 w-3 inline mr-0.5 text-primary fill-primary" />
                  Followed by {followers.map(f => f.name || f.email).join(', ')}
                </span>
              </>
            )}
          </div>
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground mt-1.5">
            <Select
              value={issue.assigned_to || ''}
              onChange={e => handleAssign(e.target.value)}
              className="h-5 text-xs w-auto py-0 px-1"
            >
              <option value="">Unassigned</option>
              {users.map(u => (
                <option key={u.id} value={u.id}>{u.name || u.email}</option>
              ))}
            </Select>
            {issueTags.map(t => (
              <span key={`${t.key}:${t.value}`} className="inline-flex items-center gap-0.5 font-mono px-1.5 py-0 rounded bg-primary/10 text-primary/80">
                {t.key}:{t.value}
                <button onClick={() => handleRemoveTag(t.key, t.value)} className="hover:text-destructive">
                  <X className="h-2.5 w-2.5" />
                </button>
              </span>
            ))}
            <form onSubmit={e => { e.preventDefault(); handleAddTag() }} className="inline-flex items-center gap-0.5">
              <Input
                value={tagInput}
                onChange={e => setTagInput(e.target.value)}
                placeholder="key:value"
                className="h-5 w-20 text-xs px-1.5 py-0"
              />
              <button type="submit" disabled={!tagInput.includes(':')} className="text-muted-foreground hover:text-foreground disabled:opacity-30">
                <Plus className="h-3 w-3" />
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
          {project?.github_owner && !issue.github_issue_number && (
            <Button size="sm" variant="secondary" onClick={handleCreateGithubIssue} disabled={creatingGithub}>
              {creatingGithub ? 'Creating...' : 'GitHub'}
            </Button>
          )}
          {issue.github_issue_number && issue.github_issue_url && (
            <a href={issue.github_issue_url} target="_blank" rel="noopener noreferrer">
              <Button size="sm" variant="outline">
                #{issue.github_issue_number} <ExternalLink className="h-3.5 w-3.5 ml-1" />
              </Button>
            </a>
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
          {project?.workflow_mode === 'managed' && !ticket && (
            <Button size="sm" variant="default" onClick={handleCreateTicket} disabled={creatingTicket}>
              {creatingTicket ? 'Creating...' : 'Manage'}
            </Button>
          )}
          {ticket && (
            <Link to={`/projects/${projectId}/tickets/${ticket.id}`}>
              <Button size="sm" variant="outline">
                View ticket
              </Button>
            </Link>
          )}
          <Button
            size="sm"
            variant={followed ? 'default' : 'outline'}
            onClick={async () => {
              if (!projectId || !issueId) return
              if (followed) {
                await api.unfollowIssue(projectId, issueId)
                setFollowed(false)
                const updated = await api.getIssue(projectId, issueId)
                setFollowers(updated.followers || [])
                toast.success('Unfollowed')
              } else {
                await api.followIssue(projectId, issueId)
                setFollowed(true)
                const updated = await api.getIssue(projectId, issueId)
                setFollowers(updated.followers || [])
                toast.success('Following')
              }
            }}
          >
            <Bookmark className={cn("h-4 w-4 mr-1", followed && "fill-current")} /> {followed ? 'Following' : 'Follow'}
          </Button>
          <Button size="sm" variant="outline" className="text-destructive hover:bg-destructive/10" onClick={() => setShowDelete(true)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>


      {/* Release Info */}
      {releaseInfo && releaseInfo.first_release && releaseInfo.first_release !== 'unknown' && (
        <div className="mb-6 rounded-lg border bg-card/50 p-4">
          <h2 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-2">Release</h2>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <div>
              <span className="text-muted-foreground">First seen in </span>
              <span className="font-mono font-medium">{releaseInfo.first_release}</span>
            </div>
            {releaseInfo.commit_sha && (
              <a
                href={releaseInfo.commit_url}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono text-xs text-primary/70 hover:underline"
              >
                {releaseInfo.commit_sha.slice(0, 7)}
              </a>
            )}
            {releaseInfo.diff_url && releaseInfo.previous_release && (
              <a
                href={releaseInfo.diff_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-primary hover:underline"
              >
                Diff from {releaseInfo.previous_release}
              </a>
            )}
            {releaseInfo.deployed_at && (
              <div className="text-xs text-muted-foreground">
                Deployed {releaseInfo.deploy_environment && <span className="font-medium">{releaseInfo.deploy_environment}</span>}{' '}
                {new Date(releaseInfo.deployed_at).toLocaleString()}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Suspect Commits */}
      {suspectCommits.length > 0 && (
        <div className="mb-8">
          <h2 className="text-lg font-semibold mb-3">Suspect Commits</h2>
          <div className="space-y-2">
            {suspectCommits.map((c, i) => (
              <a
                key={c.sha}
                href={c.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-start gap-3 px-3 py-2.5 rounded-md border hover:bg-accent/50 transition-colors group"
              >
                <div className={cn(
                  'mt-0.5 h-5 w-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold',
                  i === 0 ? 'bg-red-500/20 text-red-400' : 'bg-amber-500/20 text-amber-400'
                )}>
                  {i + 1}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-primary/70">{c.sha.slice(0, 7)}</span>
                    <span className="text-sm font-medium truncate">{c.message}</span>
                    <ExternalLink className="h-3 w-3 text-muted-foreground/30 group-hover:text-primary/50 shrink-0 transition-colors" />
                  </div>
                  <div className="flex items-center gap-2 mt-0.5 text-xs text-muted-foreground">
                    <span>{c.author}</span>
                    <span className="opacity-40">&middot;</span>
                    <span>{new Date(c.timestamp).toLocaleDateString()}</span>
                    {c.files.length > 0 && (
                      <>
                        <span className="opacity-40">&middot;</span>
                        <span className="text-primary/60">{c.files.length} matching {c.files.length === 1 ? 'file' : 'files'}</span>
                      </>
                    )}
                  </div>
                </div>
              </a>
            ))}
          </div>
        </div>
      )}

      {/* Last Event (always expanded) */}
      {events.length > 0 && (
        <>
          <h2 className="text-lg font-semibold mb-4">Last Event</h2>
          <div className="border border-border/60 rounded-lg overflow-hidden mb-8">
            <div className="flex items-start justify-between p-4">
              <div>
                <p className="font-medium text-sm">{events[0].message}</p>
                <p className="text-xs text-muted-foreground font-mono mt-0.5">
                  {new Date(events[0].timestamp).toLocaleString()}
                  {events[0].environment && <span className="ml-2 text-muted-foreground/70">{events[0].environment}</span>}
                  {events[0].release && <span className="ml-2 text-muted-foreground/70">{events[0].release}</span>}
                  {events[0].server_name && <span className="ml-2 text-muted-foreground/70">{events[0].server_name}</span>}
                </p>
              </div>
              <div className="flex gap-2 shrink-0 ml-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    navigator.clipboard.writeText(formatEventSummary(events[0].data))
                    toast.success('Summary copied')
                  }}
                >
                  <Copy className="h-3.5 w-3.5 mr-1" /> Copy summary
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    navigator.clipboard.writeText(JSON.stringify(events[0].data, null, 2))
                    toast.success('Full event copied')
                  }}
                >
                  <Copy className="h-3.5 w-3.5 mr-1" /> Copy full
                </Button>
              </div>
            </div>
            <div className="px-4 pb-4">
              <EventData data={events[0].data} project={project} />
            </div>
          </div>
        </>
      )}

      {/* AI Analysis */}
      {project?.ai_enabled && project?.ai_root_cause && (
        <div className="mb-8">
          <div className="flex items-center justify-between mb-4">
            <button
              onClick={() => setAnalysisExpanded(!analysisExpanded)}
              className="flex items-center gap-2 text-lg font-semibold hover:text-primary transition-colors"
            >
              <Sparkles className="h-5 w-5" />
              AI Analysis
              {analysis && <span className="text-xs font-normal text-muted-foreground">(v{analysis.version})</span>}
              <ChevronDown className={cn('h-4 w-4 transition-transform', !analysisExpanded && '-rotate-90')} />
            </button>
            <div className="flex gap-2">
              {analysis && ticket && (
                <Button size="sm" variant="outline" onClick={handleCopyAnalysisToTicket}>
                  <ClipboardCopy className="h-3.5 w-3.5 mr-1" /> Copy to ticket
                </Button>
              )}
              <Button size="sm" variant={analysis ? 'outline' : 'default'} onClick={handleAnalyze} disabled={analyzing}>
                <Sparkles className={cn('h-3.5 w-3.5 mr-1', analyzing && 'animate-pulse')} />
                {analyzing ? 'Analyzing...' : analysis ? 'Regenerate' : 'Analyze'}
              </Button>
            </div>
          </div>
          {analysisExpanded && analysis && (
            <div className="rounded-lg border bg-card p-4 space-y-4">
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-1">Summary</h3>
                <p className="text-sm">{analysis.summary}</p>
              </div>
              {analysis.evidence && analysis.evidence.length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-1">Evidence</h3>
                  <ul className="list-disc list-inside text-sm space-y-1">
                    {analysis.evidence.map((e, i) => <li key={i} className="text-muted-foreground">{e}</li>)}
                  </ul>
                </div>
              )}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground/60 mb-1">Suggested Fix</h3>
                <div className="text-sm prose prose-invert prose-sm max-w-none">
                  <Markdown remarkPlugins={[remarkGfm]}>{analysis.suggested_fix}</Markdown>
                </div>
              </div>
              <p className="text-xs text-muted-foreground/40">
                Generated by {analysis.model} on {new Date(analysis.created_at).toLocaleString()}
              </p>
            </div>
          )}
          {analysisExpanded && !analysis && !analyzing && (
            <p className="text-sm text-muted-foreground italic">No analysis yet. Click "Analyze" to generate a root cause analysis.</p>
          )}
        </div>
      )}

      {/* Comments */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <MessageSquare className="h-5 w-5" />
          Comments
          {comments.length > 0 && <span className="text-sm font-normal text-muted-foreground">({comments.length})</span>}
        </h2>

        {comments.map(c => (
          <div key={c.id} className="flex gap-3 mb-4">
            <div className="shrink-0 mt-0.5">
              {c.user_avatar ? (
                <img src={c.user_avatar} alt="" className="h-7 w-7 rounded-full" />
              ) : (
                <div className="h-7 w-7 rounded-full bg-primary/20 flex items-center justify-center text-xs font-medium text-primary">
                  {(c.user_name || c.user_email)[0]?.toUpperCase()}
                </div>
              )}
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-0.5">
                <span className="text-sm font-medium">{c.user_name || c.user_email}</span>
                <span className="text-xs text-muted-foreground">{new Date(c.created_at).toLocaleString()}</span>
                {c.updated_at !== c.created_at && (
                  <span className="text-xs text-muted-foreground italic">(edited)</span>
                )}
              </div>
              {editingComment === c.id ? (
                <div className="flex gap-2">
                  <textarea
                    value={editBody}
                    onChange={e => setEditBody(e.target.value)}
                    className="flex-1 text-sm rounded-md border border-border bg-background px-3 py-2 min-h-[60px] resize-none focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                  <div className="flex flex-col gap-1">
                    <Button size="sm" disabled={!editBody.trim()} onClick={async () => {
                      if (!projectId || !issueId) return
                      const updated = await api.updateComment(projectId, issueId, c.id, editBody.trim())
                      setComments(prev => prev.map(x => x.id === c.id ? { ...x, body: updated.body, updated_at: updated.updated_at } : x))
                      setEditingComment(null)
                      toast.success('Comment updated')
                    }}>
                      <Check className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => setEditingComment(null)}>
                      <X className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="text-sm prose prose-sm prose-invert max-w-none [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded [&_code]:text-xs [&_a]:text-primary [&_p]:my-1">
                  <Markdown remarkPlugins={[remarkGfm]}>{c.body.replace(/@([\w.-]+)/g, '**@$1**')}</Markdown>
                </div>
              )}
              {editingComment !== c.id && (currentUser?.id === c.user_id || currentUser?.role === 'admin') && (
                <div className="flex gap-2 mt-1">
                  <button
                    onClick={() => { setEditingComment(c.id); setEditBody(c.body) }}
                    className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                  >
                    <Pencil className="h-3 w-3 inline mr-0.5" /> Edit
                  </button>
                  <button
                    onClick={async () => {
                      if (!projectId || !issueId) return
                      await api.deleteComment(projectId, issueId, c.id)
                      setComments(prev => prev.filter(x => x.id !== c.id))
                      toast.success('Comment deleted')
                    }}
                    className="text-xs text-muted-foreground hover:text-destructive transition-colors"
                  >
                    <Trash2 className="h-3 w-3 inline mr-0.5" /> Delete
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}

        <form
          onSubmit={async e => {
            e.preventDefault()
            if (!projectId || !issueId || !commentBody.trim() || submittingComment) return
            setSubmittingComment(true)
            try {
              const created = await api.createComment(projectId, issueId, commentBody.trim())
              setComments(prev => [...prev, created])
              setCommentBody('')
              toast.success('Comment added')
            } finally {
              setSubmittingComment(false)
            }
          }}
          className="flex gap-2 items-start"
        >
          <div className="shrink-0 mt-0.5">
            {currentUser?.avatar_url ? (
              <img src={currentUser.avatar_url} alt="" className="h-7 w-7 rounded-full" />
            ) : (
              <div className="h-7 w-7 rounded-full bg-primary/20 flex items-center justify-center text-xs font-medium text-primary">
                {(currentUser?.name || currentUser?.email || '?')[0]?.toUpperCase()}
              </div>
            )}
          </div>
          <div className="flex-1">
            {commentPreview ? (
              <div className="text-sm prose prose-sm prose-invert max-w-none rounded-md border border-border bg-background px-3 py-2 min-h-[60px] [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded [&_code]:text-xs [&_a]:text-primary [&_p]:my-1">
                <Markdown remarkPlugins={[remarkGfm]}>{commentBody || '*Nothing to preview*'}</Markdown>
              </div>
            ) : (
              <div className="relative">
                <textarea
                  ref={commentRef}
                  value={commentBody}
                  onChange={handleCommentChange}
                  onKeyDown={handleCommentKeyDown}
                  placeholder="Add a comment... (supports Markdown, @mention users)"
                  className="w-full text-sm rounded-md border border-border bg-background px-3 py-2 min-h-[60px] resize-none focus:outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground/50"
                />
                {mentionQuery !== null && mentionUsers.length > 0 && (
                  <div className="absolute bottom-full left-0 mb-1 w-56 rounded-md border bg-popover shadow-lg z-50">
                    {mentionUsers.map((u, i) => (
                      <button
                        key={u.id}
                        type="button"
                        className={cn(
                          'w-full text-left px-3 py-1.5 text-sm hover:bg-accent',
                          i === mentionIndex && 'bg-accent'
                        )}
                        onMouseDown={e => { e.preventDefault(); insertMention(u) }}
                      >
                        <span className="font-medium">{u.name || u.email}</span>
                        {u.name && <span className="text-xs text-muted-foreground ml-1">{u.email}</span>}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            <button
              type="button"
              onClick={() => setCommentPreview(!commentPreview)}
              className="text-xs text-muted-foreground hover:text-foreground mt-1 transition-colors"
            >
              {commentPreview ? 'Edit' : 'Preview'}
            </button>
          </div>
          <Button type="submit" size="sm" disabled={!commentBody.trim() || submittingComment} className="mt-0.5">
            <Send className="h-4 w-4" />
          </Button>
        </form>
      </div>

      {/* All Events */}
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
                <div className="flex justify-end gap-2 mb-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(formatEventSummary(event.data))
                      toast.success('Summary copied')
                    }}
                  >
                    <Copy className="h-3.5 w-3.5 mr-1" /> Copy summary
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(JSON.stringify(event.data, null, 2))
                      toast.success('Full event copied')
                    }}
                  >
                    <Copy className="h-3.5 w-3.5 mr-1" /> Copy full
                  </Button>
                </div>
                <EventData data={event.data} project={project} />
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

function buildSourceURL(filename: string, lineno: number, project: Project | null): string | null {
  if (!project?.repo_provider || !project.repo_owner || !project.repo_name) return null
  // Skip library paths
  const lower = filename.toLowerCase()
  if (['node_modules/', 'vendor/', 'site-packages/', 'lib/python', '.gem/', '/usr/lib/'].some(p => lower.includes(p))) return null
  const cleanPath = project.repo_path_strip ? filename.replace(project.repo_path_strip, '') : filename
  const branch = project.repo_default_branch || 'main'
  if (project.repo_provider === 'github') {
    return `https://github.com/${project.repo_owner}/${project.repo_name}/blob/${branch}/${cleanPath}#L${lineno}`
  }
  if (project.repo_provider === 'bitbucket') {
    return `https://bitbucket.org/${project.repo_owner}/${project.repo_name}/src/${branch}/${cleanPath}#lines-${lineno}`
  }
  return null
}

function ExceptionSection({ exception, project }: { exception: { values?: Array<{ type: string; value: string; stacktrace?: { frames?: Array<{ filename: string; function: string; lineno: number; colno?: number; in_app?: boolean }> } }> }; project: Project | null }) {
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
                  {[...exc.stacktrace.frames].reverse().map((frame, j) => {
                    const sourceUrl = buildSourceURL(frame.filename, frame.lineno, project)
                    return (
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
                        {sourceUrl ? (
                          <a href={sourceUrl} target="_blank" rel="noopener noreferrer" className="hover:underline">
                            <span className="text-primary/70">{frame.filename}</span>
                          </a>
                        ) : (
                          <span className="text-muted-foreground">{frame.filename}</span>
                        )}
                        <span className="text-muted-foreground/40"> in </span>
                        <span className={frame.in_app ? 'text-primary font-semibold' : 'text-foreground/60'}>
                          {frame.function}
                        </span>
                        {sourceUrl && (
                          <a href={sourceUrl} target="_blank" rel="noopener noreferrer" className="ml-2 text-muted-foreground/30 hover:text-primary/60 transition-colors" title="View in repository">
                            <ExternalLink className="h-3 w-3 inline" />
                          </a>
                        )}
                      </td>
                    </tr>
                    )
                  })}
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

function formatEventSummary(data: Record<string, unknown>): string {
  const lines: string[] = []

  // Error
  const exc = data.exception as { values?: Array<{ type: string; value: string; stacktrace?: { frames?: Array<{ filename: string; function: string; lineno: number; in_app?: boolean }> } }> } | undefined
  if (exc?.values?.length) {
    const last = exc.values[exc.values.length - 1]
    lines.push(`Error: ${last.type}: ${last.value}`)
  } else if (data.message) {
    lines.push(`Error: ${data.message}`)
  }

  // Server
  if (data.server_name) {
    lines.push(`Server: ${data.server_name}`)
  }

  // Request: URL + body
  const req = data.request as { method?: string; url?: string; data?: unknown; query_string?: string } | undefined
  if (req) {
    if (req.method || req.url) {
      lines.push(`URL: ${req.method || ''} ${req.url || ''}`.trim())
    }
    if (req.query_string) {
      lines.push(`Query: ${req.query_string}`)
    }
    if (req.data != null) {
      const body = typeof req.data === 'string' ? req.data : JSON.stringify(req.data)
      lines.push(`Body: ${body}`)
    }
  }

  // Stacktrace: function + line only
  if (exc?.values?.length) {
    for (const e of exc.values) {
      if (!e.stacktrace?.frames?.length) continue
      lines.push('')
      lines.push('Stacktrace:')
      const frames = [...e.stacktrace.frames].reverse()
      for (const f of frames) {
        const marker = f.in_app ? '>' : ' '
        const fn = f.function || '(unknown)'
        lines.push(`  ${marker} ${fn} (${f.filename}:${f.lineno})`)
      }
    }
  }

  return lines.join('\n')
}

function EventData({ data, project }: { data: Record<string, unknown>; project?: Project | null }) {
  const [activeTab, setActiveTab] = useState('')
  const exception = data.exception as { values?: Array<{ type: string; value: string; stacktrace?: { frames?: Array<{ filename: string; function: string; lineno: number; colno?: number; in_app?: boolean }> } }> } | undefined
  const request = data.request as { method?: string; url?: string; headers?: Record<string, string>; query_string?: string; data?: unknown; env?: Record<string, string> } | undefined
  const breadcrumbs = data.breadcrumbs as { values?: Array<{ type?: string; category?: string; message?: string; data?: Record<string, unknown>; level?: string; timestamp?: string | number }> } | undefined
  const tags = data.tags as Record<string, string> | Array<[string, string]> | undefined
  const contexts = data.contexts as Record<string, Record<string, unknown>> | undefined
  const user = data.user as { id?: string; email?: string; username?: string; ip_address?: string; [key: string]: unknown } | undefined

  const tabs: { id: string; label: string; count?: number }[] = []
  if (exception?.values?.length) tabs.push({ id: 'stacktrace', label: 'Stacktrace' })
  if (request) tabs.push({ id: 'request', label: 'Request' })
  if (user && Object.values(user).some(v => v != null && v !== '')) tabs.push({ id: 'user', label: 'User' })
  if (contexts && Object.entries(contexts).some(([, v]) => v && typeof v === 'object' && Object.keys(v).length > 0)) tabs.push({ id: 'context', label: 'Context' })
  if (breadcrumbs?.values?.length) tabs.push({ id: 'breadcrumbs', label: 'Breadcrumbs', count: breadcrumbs.values.length })
  if (tags) {
    const tagEntries = Array.isArray(tags) ? tags : Object.entries(tags)
    if (tagEntries.length) tabs.push({ id: 'tags', label: 'Tags', count: tagEntries.length })
  }
  tabs.push({ id: 'raw', label: 'JSON' })

  const current = activeTab || tabs[0]?.id || 'raw'

  if (tabs.length === 1) {
    return (
      <pre className="bg-[#0d1117] rounded-lg p-4 text-xs font-mono overflow-x-auto max-h-96 text-foreground/80 border border-border/60">
        {JSON.stringify(data, null, 2)}
      </pre>
    )
  }

  return (
    <div>
      <div className="flex gap-0 border-b border-border/60 mb-4">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              'px-3 py-2 text-xs font-medium border-b-2 transition-colors -mb-px',
              current === tab.id
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground/80'
            )}
          >
            {tab.label}
            {tab.count !== undefined && <span className="ml-1 text-muted-foreground/60">({tab.count})</span>}
          </button>
        ))}
      </div>
      {current === 'stacktrace' && exception && <ExceptionSection exception={exception} project={project || null} />}
      {current === 'request' && request && <RequestSection request={request} />}
      {current === 'user' && user && <UserSection user={user} />}
      {current === 'context' && contexts && <ContextsSection contexts={contexts} />}
      {current === 'breadcrumbs' && breadcrumbs && <BreadcrumbsSection breadcrumbs={breadcrumbs} />}
      {current === 'tags' && tags && <TagsSection tags={tags} />}
      {current === 'raw' && (
        <pre className="bg-[#0d1117] rounded-lg p-4 text-xs font-mono overflow-x-auto max-h-96 text-foreground/80 border border-border/60">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </div>
  )
}
