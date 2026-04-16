import { useEffect, useState, useRef, useMemo } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, type Issue, type Project, type IssueCounts, type DeployAnalysis } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { ChevronLeft, ChevronRight, ChevronDown, Settings, Trash2, Filter, Search, List, AlignLeft, Bookmark, Ticket, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { IssueListSkeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { useKeyboardShortcut } from '@/lib/use-keyboard'

const STATUS_COLORS: Record<string, 'error' | 'success' | 'warning' | 'secondary' | 'outline'> = {
  open: 'error',
  reopened: 'warning',
  resolved: 'success',
  ignored: 'secondary',
  snoozed: 'warning',
}

const LEVEL_COLORS: Record<string, 'error' | 'warning' | 'secondary' | 'outline'> = {
  fatal: 'error',
  error: 'error',
  warning: 'warning',
  info: 'secondary',
  debug: 'outline',
}

const LEVEL_BORDER: Record<string, string> = {
  fatal: 'border-l-red-500',
  error: 'border-l-red-500',
  warning: 'border-l-amber-500',
  info: 'border-l-blue-500',
  debug: 'border-l-slate-500',
}

type Section = 'errors' | 'warnings' | 'info'

const SECTION_LEVEL: Record<Section, string> = {
  errors: 'errors',
  warnings: 'warning',
  info: 'info_only',
}

const COUNT_LEVELS: Record<Section, string> = {
  errors: 'errors',
  warnings: 'warning',
  info: 'info_only',
}

async function fetchSectionCounts(projectId: string): Promise<Record<Section, IssueCounts>> {
  const [errors, warnings, info] = await Promise.all([
    api.getIssueCounts(projectId, { level: COUNT_LEVELS.errors }),
    api.getIssueCounts(projectId, { level: COUNT_LEVELS.warnings }),
    api.getIssueCounts(projectId, { level: COUNT_LEVELS.info }),
  ])

  return { errors, warnings, info }
}

function formatRelativeTime(date: string, now: number) {
  const diff = now - new Date(date).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export default function IssueList() {
  const { projectId } = useParams<{ projectId: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [issues, setIssues] = useState<Issue[]>([])
  const [total, setTotal] = useState(0)
  const [project, setProject] = useState<Project | null>(null)
  const [errorCounts, setErrorCounts] = useState<IssueCounts | null>(null)
  const [warningCounts, setWarningCounts] = useState<IssueCounts | null>(null)
  const [infoCounts, setInfoCounts] = useState<IssueCounts | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [fetchedAt, setFetchedAt] = useState<number | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)
  const [showBulkDelete, setShowBulkDelete] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showMerge, setShowMerge] = useState(false)
  const [showMobileFilters, setShowMobileFilters] = useState(false)
  const [releases, setReleases] = useState<string[]>([])
  const [deployHealth, setDeployHealth] = useState<DeployAnalysis | null>(null)
  const [ticketCounts, setTicketCounts] = useState<Record<string, number>>({})
  const [searchInput, setSearchInput] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const searchTimerRef = useRef<ReturnType<typeof setTimeout>>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const [displayMode, setDisplayMode] = useState<'classic' | 'detailed'>(() => {
    return (localStorage.getItem('issue-display-mode') as 'classic' | 'detailed') || 'classic'
  })

  const shortcuts = useMemo(() => ({
    '/': (e: KeyboardEvent) => {
      e.preventDefault()
      searchInputRef.current?.focus()
    },
  }), [])

  useKeyboardShortcut(shortcuts)

  // Collapse state: errors open by default, warnings and info collapsed
  const [collapsed, setCollapsed] = useState<Record<Section, boolean>>({
    errors: false,
    warnings: true,
    info: true,
  })

  const status = searchParams.get('status') || ''
  const level = searchParams.get('level') || 'errors'
  const sort = searchParams.get('sort') || 'last_seen'
  const activeFilter = searchParams.get('filter') || ''
  const releaseFilter = searchParams.get('release') || ''
  const offset = parseInt(searchParams.get('offset') || '0', 10)
  const limit = 25

  // Which section is active?
  const activeSection: Section | null =
    level === 'errors' || level === 'errors_w' ? 'errors' :
    level === 'warning' ? 'warnings' :
    level === 'info_only' || level === 'informational' ? 'info' :
    null

  useEffect(() => {
    if (!projectId) return
    api.getProject(projectId).then(p => {
      setProject(p)
      if (p.workflow_mode === 'managed') {
        api.getTicketCounts(projectId).then(setTicketCounts).catch(() => setTicketCounts({}))
      } else {
        setTicketCounts({})
      }
      // Use project default if user hasn't manually overridden
      if (!localStorage.getItem('issue-display-mode') && p.issue_display_mode) {
        setDisplayMode(p.issue_display_mode as 'classic' | 'detailed')
      }
    })
    api.listIssueReleases(projectId).then(setReleases).catch(() => {})
    api.getDeployHealth(projectId).then(r => setDeployHealth(r.analysis)).catch(() => {})
  }, [projectId])

  useEffect(() => {
    if (!projectId) return

    fetchSectionCounts(projectId).then(counts => {
      setErrorCounts(counts.errors)
      setWarningCounts(counts.warnings)
      setInfoCounts(counts.info)
    })
  }, [projectId])

  // Fetch issue list
  useEffect(() => {
    if (!projectId) return
    const params: Parameters<typeof api.listIssues>[1] = { status, level, limit, offset }
    if (activeFilter === 'today') params.today = true
    if (activeFilter === 'assigned_to_me') params.assigned_to = 'me'
    if (activeFilter === 'assigned_any') params.assigned_any = true
    if (releaseFilter) params.release = releaseFilter
    if (searchQuery) {
      params.search = searchQuery
      // Also search by tag if it looks like key:value
      if (/^[^\s:]+:[^\s:]+$/.test(searchQuery)) {
        params.tag = searchQuery
      }
    }
    api.listIssues(projectId, params)
      .then(res => {
        setFetchedAt(Date.now())
        setError('')
        setIssues(res.issues)
        setTotal(res.total)
      })
      .catch(err => {
        setFetchedAt(Date.now())
        setError(err.message)
        setIssues([])
        setTotal(0)
      })
      .finally(() => setLoading(false))
  }, [projectId, status, level, activeFilter, releaseFilter, offset, refreshKey, searchQuery])

  // Refresh counts only on manual refresh (not on initial load — already fetched above)
  useEffect(() => {
    if (!projectId || refreshKey === 0) return

    fetchSectionCounts(projectId).then(counts => {
      setErrorCounts(counts.errors)
      setWarningCounts(counts.warnings)
      setInfoCounts(counts.info)
    })
  }, [projectId, refreshKey])

  const setFilter = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams)
    if (value) params.set(key, value)
    else params.delete(key)
    if (key !== 'offset') params.delete('offset')
    setError('')
    setSearchParams(params)
  }

  const handleSectionFilter = (section: Section, filterStatus: string, special?: string) => {
    const params = new URLSearchParams()
    params.set('level', SECTION_LEVEL[section])
    if (filterStatus) params.set('status', filterStatus)
    if (special) params.set('filter', special)
    if (sort !== 'last_seen') params.set('sort', sort)
    // Expand this section when clicking a filter
    setError('')
    setCollapsed({ errors: true, warnings: true, info: true, [section]: false })
    setSearchParams(params)
  }

  const toggleCollapse = (section: Section) => {
    setCollapsed(prev => {
      const willOpen = prev[section]
      if (willOpen) {
        return { errors: true, warnings: true, info: true, [section]: false }
      }
      return { ...prev, [section]: true }
    })
  }

  const isActive = (section: Section, filterStatus: string, special?: string) => {
    if (activeSection !== section) return false
    if (special) return activeFilter === special
    if (filterStatus) return status === filterStatus && !activeFilter
    return !status && !activeFilter
  }

  const sectionColors: Record<Section, { active: string; chevron: string }> = {
    errors: { active: 'bg-red-500/10 text-red-400', chevron: 'text-red-400/60' },
    warnings: { active: 'bg-amber-500/10 text-amber-400', chevron: 'text-amber-400/60' },
    info: { active: 'bg-blue-500/10 text-blue-400', chevron: 'text-blue-400/60' },
  }

  const totalTickets = Object.entries(ticketCounts).reduce((sum, [key, value]) => {
    if (key === 'assigned_to_me_pending') return sum
    return sum + value
  }, 0)
  const assignedPendingTickets = ticketCounts.assigned_to_me_pending || 0

  const renderTicketsLink = (withTopMargin = false) => (
    <Link
      to={`/projects/${projectId}/tickets`}
      className={cn(
        'flex items-center gap-2 px-2 py-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors rounded-md hover:bg-accent/50',
        withTopMargin && 'mt-2'
      )}
    >
      <Ticket className="h-3.5 w-3.5 shrink-0" />
      <span className="flex-1 min-w-0">Tickets</span>
      <span className="inline-flex items-center rounded-full border border-border/60 bg-muted/40 px-1.5 py-0.5 text-[11px] font-medium leading-none text-foreground/80">
        {totalTickets}
      </span>
      {assignedPendingTickets > 0 && (
        <span className="inline-flex min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 py-0.5 text-[11px] font-semibold leading-none text-white">
          {assignedPendingTickets}
        </span>
      )}
    </Link>
  )

  const renderSection = (
    section: Section,
    label: string,
    counts: IssueCounts | null,
    filters: { label: string; status: string; special?: string; indent?: boolean }[]
  ) => {
    const isOpen = !collapsed[section]
    const isSectionActive = activeSection === section
    const colors = sectionColors[section]

    return (
      <div>
        <div className="flex items-center gap-0.5">
          <button
            onClick={() => toggleCollapse(section)}
            className="p-0.5 rounded hover:bg-accent/50 transition-colors"
          >
            <ChevronDown className={cn(
              'h-3.5 w-3.5 transition-transform',
              !isOpen && '-rotate-90',
              isSectionActive ? colors.chevron : 'text-muted-foreground/40'
            )} />
          </button>
          <button
            onClick={() => handleSectionFilter(section, '')}
            className={cn(
              'flex items-center justify-between flex-1 px-1.5 py-1.5 rounded-md text-sm font-semibold transition-colors',
              isSectionActive && !status && !activeFilter
                ? colors.active
                : 'text-foreground/80 hover:bg-accent/50'
            )}
          >
            <span>{label}</span>
            <span className="font-mono text-xs">{counts?.total ?? 0}</span>
          </button>
        </div>

        {isOpen && (
          <nav className="mt-0.5 space-y-0.5 ml-4">
            {filters.map(f => (
              <SidebarItem
                key={f.label + f.status + (f.special || '')}
                label={f.label}
                count={
                  f.special === 'today' ? (counts?.today ?? 0) :
                  f.special === 'assigned_to_me' ? (counts?.assigned_to_me ?? 0) :
                  f.special === 'assigned_any' ? (counts?.assigned_any ?? 0) :
                  f.status ? (counts?.by_status[f.status] ?? 0) :
                  (counts?.total ?? 0)
                }
                active={f.special ? isActive(section, '', f.special) : f.status ? isActive(section, f.status) : (isSectionActive && !status && !activeFilter)}
                indent={f.indent}
                onClick={() => handleSectionFilter(section, f.status, f.special)}
              />
            ))}
          </nav>
        )}
      </div>
    )
  }

  const errorsFilters = [
    { label: 'Introduced Today', status: '', special: 'today' },
    { label: 'Open', status: 'open' },
    { label: 'Assigned to me', status: '', special: 'assigned_to_me', indent: true },
    { label: 'Assigned to anyone', status: '', special: 'assigned_any', indent: true },
    { label: 'Reopened', status: 'reopened' },
    { label: 'Resolved', status: 'resolved' },
    { label: 'Snoozed', status: 'snoozed' },
    { label: 'Ignored', status: 'ignored' },
  ]

  const warningsFilters = [
    { label: 'Introduced Today', status: '', special: 'today' },
    { label: 'Open', status: 'open' },
    { label: 'Assigned to me', status: '', special: 'assigned_to_me', indent: true },
    { label: 'Assigned to anyone', status: '', special: 'assigned_any', indent: true },
    { label: 'Reopened', status: 'reopened' },
    { label: 'Resolved', status: 'resolved' },
    { label: 'Snoozed', status: 'snoozed' },
    { label: 'Ignored', status: 'ignored' },
  ]

  const infoFilters = [
    { label: 'Introduced Today', status: '', special: 'today' },
    { label: 'Open', status: 'open' },
    { label: 'Assigned to me', status: '', special: 'assigned_to_me', indent: true },
    { label: 'Assigned to anyone', status: '', special: 'assigned_any', indent: true },
    { label: 'Reopened', status: 'reopened' },
    { label: 'Resolved', status: 'resolved' },
    { label: 'Snoozed', status: 'snoozed' },
    { label: 'Ignored', status: 'ignored' },
  ]

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        ...(project?.group_name && project?.group_id ? [{ label: project.group_name, to: `/?group=${project.group_id}` }] : []),
        { label: project?.name || '' },
      ]} />

      {deployHealth && (
        <div className={cn(
          'mb-4 rounded-lg border p-4 flex items-start gap-3',
          deployHealth.severity === 'critical'
            ? 'border-red-500/40 bg-red-500/10'
            : 'border-amber-500/40 bg-amber-500/10'
        )}>
          <AlertTriangle className={cn(
            'h-5 w-5 shrink-0 mt-0.5',
            deployHealth.severity === 'critical' ? 'text-red-400' : 'text-amber-400'
          )} />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium">
              Deploy Health: <span className="uppercase">{deployHealth.severity}</span>
            </p>
            <p className="text-sm text-muted-foreground mt-0.5">{deployHealth.summary}</p>
            {deployHealth.recommended_action && (
              <p className="text-xs text-muted-foreground mt-1">
                Recommended: <span className="font-medium">{deployHealth.recommended_action}</span>
              </p>
            )}
          </div>
          <button
            onClick={() => setDeployHealth(null)}
            className="text-muted-foreground hover:text-foreground shrink-0"
          >
            <span className="sr-only">Dismiss</span>
            &times;
          </button>
        </div>
      )}

      {/* Mobile Filter Sheet */}
      <Sheet open={showMobileFilters} onOpenChange={setShowMobileFilters}>
        <SheetContent>
          <SheetTitle>Filters</SheetTitle>
          <SheetDescription className="sr-only">Filter issues by level and status</SheetDescription>
          <div className="mt-4 space-y-3" onClick={() => setShowMobileFilters(false)}>
            {renderSection('errors', 'Errors', errorCounts, errorsFilters)}
            <div className="border-t border-border/40" />
            {renderSection('warnings', 'Warnings', warningCounts, warningsFilters)}
            <div className="border-t border-border/40" />
            {renderSection('info', 'Info', infoCounts, infoFilters)}
            <div className="border-t border-border/40">
              {project?.workflow_mode === 'managed' && renderTicketsLink(true)}
              <Link
                to={`/projects/${projectId}/settings`}
                className={cn(
                  'flex items-center gap-2 px-2 py-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors rounded-md hover:bg-accent/50',
                  project?.workflow_mode === 'managed' ? '' : 'mt-2'
                )}
              >
                <Settings className="h-3.5 w-3.5" />
                Settings
              </Link>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <div className="flex gap-6">
        {/* Sidebar */}
        <aside className="w-56 shrink-0 hidden md:block">
          <div className="sticky top-20 space-y-3">
            {renderSection('errors', 'Errors', errorCounts, errorsFilters)}

            <div className="border-t border-border/40" />
            {renderSection('warnings', 'Warnings', warningCounts, warningsFilters)}

            <div className="border-t border-border/40" />
            {renderSection('info', 'Info', infoCounts, infoFilters)}

            <div className="border-t border-border/40">
              {project?.workflow_mode === 'managed' && renderTicketsLink(true)}
              <Link
                to={`/projects/${projectId}/settings`}
                className={cn(
                  'flex items-center gap-2 px-2 py-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors rounded-md hover:bg-accent/50',
                  project?.workflow_mode === 'managed' ? '' : 'mt-2'
                )}
              >
                <Settings className="h-3.5 w-3.5" />
                Settings
              </Link>
            </div>
          </div>
        </aside>

        {/* Main content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5 mb-4">
            <Button
              variant="outline" size="sm"
              className="md:hidden"
              onClick={() => setShowMobileFilters(true)}
            >
              <Filter className="h-4 w-4 mr-1" /> Filters
            </Button>
            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/50" />
              <Input
                ref={searchInputRef}
                value={searchInput}
                onChange={e => {
                  const val = e.target.value
                  setSearchInput(val)
                  if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
                  searchTimerRef.current = setTimeout(() => setSearchQuery(val), 300)
                }}
                placeholder="Search issues..."
                className="h-8 pl-8 text-xs"
              />
            </div>
            {releases.length > 0 && (
              <Select
                value={releaseFilter}
                onChange={e => setFilter('release', e.target.value)}
                className="h-8 w-auto text-xs max-w-[180px]"
              >
                <option value="">All releases</option>
                {releases.map(r => (
                  <option key={r} value={r}>{r}</option>
                ))}
              </Select>
            )}
            <Select
              value={sort}
              onChange={e => setFilter('sort', e.target.value)}
              className="h-8 w-auto text-xs"
            >
              <option value="last_seen">Last seen</option>
              <option value="first_seen">First seen</option>
              <option value="event_count">Events</option>
              <option value="priority">Priority</option>
            </Select>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-8 w-8 p-0"
                  onClick={() => {
                    const next = displayMode === 'classic' ? 'detailed' : 'classic'
                    setDisplayMode(next)
                    localStorage.setItem('issue-display-mode', next)
                  }}
                >
                  {displayMode === 'classic' ? <AlignLeft className="h-3.5 w-3.5" /> : <List className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>{displayMode === 'classic' ? 'Switch to detailed view' : 'Switch to classic view'}</TooltipContent>
            </Tooltip>
            <Link to={`/projects/${projectId}/settings`} className="md:hidden">
              <Button variant="outline" size="sm">
                <Settings className="h-4 w-4" />
              </Button>
            </Link>
          </div>

          {error ? (
            <div className="text-center py-12">
              <p className="text-destructive text-sm">{error}</p>
            </div>
          ) : loading ? (
            <IssueListSkeleton />
          ) : issues.length === 0 ? (
            <div className="text-center py-16 text-muted-foreground">
              <div className="relative inline-block mb-4">
                <svg className="h-12 w-12 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                </svg>
              </div>
              <p className="text-base font-medium">
                {status ? `No ${status} issues` : activeFilter === 'today' ? 'No new issues today' : activeFilter === 'assigned_to_me' ? 'No issues assigned to you' : 'No issues found'}
              </p>
              <p className="text-sm mt-1 text-muted-foreground/60">
                {status === 'resolved' ? 'Nothing has been resolved yet.' : status === 'ignored' ? 'No ignored issues.' : 'When errors occur, they\'ll appear here.'}
              </p>
            </div>
          ) : (
            <div className="border rounded-lg overflow-hidden">
              {/* Header */}
              <div className="flex items-center px-4 py-2 border-b border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground uppercase tracking-wide">
                <div className="flex-1 min-w-0">Issue</div>
                <div className="w-16 text-right hidden sm:block">Events</div>
                <div className="w-16 text-right hidden sm:block">Users</div>
                <div className="w-24 text-right hidden md:block">Trend</div>
              </div>
              <div className="divide-y divide-border/60">
              {[...issues].sort((a, b) => {
                if (sort === 'priority') return b.priority - a.priority
                if (sort === 'event_count') return b.event_count - a.event_count
                if (sort === 'first_seen') return new Date(b.first_seen).getTime() - new Date(a.first_seen).getTime()
                return new Date(b.last_seen).getTime() - new Date(a.last_seen).getTime()
              }).map(issue => (
                <div
                  key={issue.id}
                  className={`flex items-center p-4 hover:bg-accent/50 transition-colors border-l-2 ${LEVEL_BORDER[issue.level] || 'border-l-transparent'}`}
                >
                  <input
                    type="checkbox"
                    checked={selectedIds.has(issue.id)}
                    onChange={e => {
                      const next = new Set(selectedIds)
                      if (e.target.checked) next.add(issue.id)
                      else next.delete(issue.id)
                      setSelectedIds(next)
                    }}
                    className="mr-3 shrink-0 accent-primary h-4 w-4 cursor-pointer"
                    onClick={e => e.stopPropagation()}
                  />
                  <Link
                    to={`/projects/${projectId}/issues/${issue.id}`}
                    className="flex items-center min-w-0 flex-1"
                  >
                  <div className="min-w-0 flex-1">
                    {displayMode === 'classic' ? (
                      <>
                        <div className="flex items-center gap-2 mb-1">
                          {issue.followed && (
                            <Bookmark className="h-3.5 w-3.5 text-primary fill-primary shrink-0" />
                          )}
                          <Badge variant={LEVEL_COLORS[issue.level] || 'outline'} className="text-xs">
                            {issue.level}
                          </Badge>
                          <Badge variant={STATUS_COLORS[issue.status] || 'outline'} className="text-xs">
                            {issue.status}
                          </Badge>
                          {issue.status === 'resolved' && issue.cooldown_until && (
                            <span className="text-xs text-muted-foreground font-mono">cooldown until {new Date(issue.cooldown_until).toLocaleString()}</span>
                          )}
                          {issue.priority !== 50 && (
                            <span className={cn(
                              'text-xs font-mono px-1.5 py-0.5 rounded',
                              issue.priority > 75 ? 'bg-red-500/15 text-red-400' :
                              issue.priority > 50 ? 'bg-amber-500/15 text-amber-400' :
                              issue.priority > 25 ? 'bg-blue-500/15 text-blue-400' :
                              'bg-muted text-muted-foreground'
                            )}>
                              P{issue.priority}
                            </span>
                          )}
                        </div>
                        <p className="font-medium truncate">{issue.title}</p>
                        {issue.tags && issue.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1 mt-0.5">
                            {issue.tags.map(t => (
                              <span key={`${t.key}:${t.value}`} className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary/80">
                                {t.key}:{t.value}
                              </span>
                            ))}
                          </div>
                        )}
                        <p className="text-sm text-muted-foreground">
                          <span className="font-mono text-xs">{issue.platform}</span>
                          {issue.first_release && (
                            <>
                              <span className="mx-1.5 opacity-40">&middot;</span>
                              <span className="font-mono text-xs text-primary/70">{issue.first_release}</span>
                            </>
                          )}
                          <span className="mx-1.5 opacity-40">&middot;</span>
                          {fetchedAt ? formatRelativeTime(issue.last_seen, fetchedAt) : new Date(issue.last_seen).toLocaleString()}
                          <span className="mx-1.5 opacity-40">&middot;</span>
                          <span className="text-xs">first {fetchedAt ? formatRelativeTime(issue.first_seen, fetchedAt) : new Date(issue.first_seen).toLocaleString()}</span>
                          <span className="sm:hidden">
                            <span className="mx-1.5 opacity-40">&middot;</span>
                            <span className="font-mono text-xs">{issue.event_count} ev</span>
                            <span className="mx-1.5 opacity-40">&middot;</span>
                            <span className="font-mono text-xs">{issue.user_count || 0} usr</span>
                          </span>
                        </p>
                      </>
                    ) : (
                      <DetailedIssueRow issue={issue} fetchedAt={fetchedAt} />
                    )}
                  </div>
                  <div className="w-16 text-right ml-4 shrink-0 hidden sm:block">
                    <p className="text-sm font-semibold font-mono">{issue.event_count}</p>
                  </div>
                  <div className="w-16 text-right ml-2 shrink-0 hidden sm:block">
                    <p className="text-sm font-mono text-muted-foreground">{issue.user_count || 0}</p>
                  </div>
                  <div className="w-24 ml-2 shrink-0 hidden md:flex justify-end">
                    <Sparkline data={issue.trend || []} />
                  </div>
                  </Link>
                </div>
              ))}
              </div>
            </div>
          )}

          {issues.length > 0 && (
            <div className="flex items-center justify-between mt-4">
              <p className="text-sm text-muted-foreground">
                {total > limit
                  ? `${offset + 1}-${Math.min(offset + limit, total)} of ${total}`
                  : `${total} issue${total === 1 ? '' : 's'}`}
              </p>
              <div className="flex gap-1">
                {total > limit && (
                  <>
                    <Button
                      variant="outline" size="sm"
                      disabled={offset === 0}
                      onClick={() => setFilter('offset', String(Math.max(0, offset - limit)))}
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="outline" size="sm"
                      disabled={offset + limit >= total}
                      onClick={() => setFilter('offset', String(offset + limit))}
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </>
                )}
                {selectedIds.size >= 2 && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="outline" size="sm"
                        className="ml-2"
                        onClick={() => setShowMerge(true)}
                      >
                        Merge ({selectedIds.size})
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Merge selected issues into one</TooltipContent>
                  </Tooltip>
                )}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline" size="sm"
                      className="text-destructive hover:bg-destructive/10 ml-2"
                      onClick={() => setShowBulkDelete(true)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete page of issues</TooltipContent>
                </Tooltip>
              </div>
            </div>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={showBulkDelete}
        onOpenChange={setShowBulkDelete}
        title="Delete Issues"
        description={selectedIds.size > 0
          ? `Delete ${selectedIds.size} selected issue(s) and all their events? This action cannot be undone.`
          : `Delete ${issues.length} issues on this page and all their events? This action cannot be undone.`}
        confirmLabel={selectedIds.size > 0 ? `Delete ${selectedIds.size} Selected` : 'Delete All'}
        variant="destructive"
        onConfirm={async () => {
          if (!projectId) return
          const ids = selectedIds.size > 0 ? [...selectedIds] : issues.map(i => i.id)
          const result = await api.deleteIssues(projectId, ids)
          toast.success(`${result.deleted} issues deleted`)
          setSelectedIds(new Set())
          setRefreshKey(k => k + 1)
        }}
      />

      {showMerge && (
        <MergeDialog
          issues={issues.filter(i => selectedIds.has(i.id))}
          onClose={() => setShowMerge(false)}
          onMerge={async (primaryId) => {
            if (!projectId) return
            const secondaryIds = [...selectedIds].filter(id => id !== primaryId)
            await api.mergeIssues(projectId, primaryId, secondaryIds)
            toast.success(`${secondaryIds.length} issues merged`)
            setSelectedIds(new Set())
            setShowMerge(false)
            setRefreshKey(k => k + 1)
          }}
        />
      )}
    </div>
  )
}

function MergeDialog({ issues, onClose, onMerge }: { issues: Issue[]; onClose: () => void; onMerge: (primaryId: string) => Promise<void> }) {
  // Default primary = issue with most events
  const sorted = [...issues].sort((a, b) => b.event_count - a.event_count)
  const [primaryId, setPrimaryId] = useState(sorted[0]?.id || '')
  const [merging, setMerging] = useState(false)

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent>
        <DialogTitle>Merge Issues</DialogTitle>
        <DialogDescription className="sr-only">Select the primary issue to merge into</DialogDescription>
        <p className="text-sm text-muted-foreground mt-2">
          Select the primary issue. The other {issues.length - 1} issue(s) will be merged into it. Their events will be reassigned and future events with the same fingerprint will go to the primary.
        </p>
        <div className="mt-4 space-y-2 max-h-64 overflow-y-auto">
          {sorted.map(issue => (
            <label
              key={issue.id}
              className={cn(
                'flex items-center gap-3 p-3 border rounded-md cursor-pointer transition-colors',
                primaryId === issue.id ? 'border-primary bg-primary/5' : 'border-border hover:border-border/80'
              )}
            >
              <input
                type="radio"
                name="primary"
                checked={primaryId === issue.id}
                onChange={() => setPrimaryId(issue.id)}
                className="accent-primary"
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium truncate">{issue.title}</p>
                <p className="text-xs text-muted-foreground">
                  {issue.event_count} events &middot; {issue.level} &middot; {issue.platform}
                </p>
              </div>
            </label>
          ))}
        </div>
        <div className="flex justify-end gap-2 mt-4">
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button
            disabled={merging}
            onClick={async () => {
              setMerging(true)
              await onMerge(primaryId)
              setMerging(false)
            }}
          >
            {merging ? 'Merging...' : `Merge into selected`}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function parseIssueTitle(title: string): { exceptionType: string; message: string } {
  const colonIdx = title.indexOf(': ')
  if (colonIdx > 0 && colonIdx < 60) {
    return {
      exceptionType: title.slice(0, colonIdx),
      message: title.slice(colonIdx + 2),
    }
  }
  return { exceptionType: '', message: title }
}

function DetailedIssueRow({ issue, fetchedAt }: { issue: Issue; fetchedAt: number | null }) {
  const { exceptionType, message } = parseIssueTitle(issue.title)

  return (
    <>
      <div className="flex items-center gap-1.5 min-w-0">
        {issue.followed && (
          <Bookmark className="h-3 w-3 text-primary fill-primary shrink-0" />
        )}
        <Badge variant={LEVEL_COLORS[issue.level] || 'outline'} className="text-[10px] shrink-0">
          {issue.level}
        </Badge>
        <Badge variant={STATUS_COLORS[issue.status] || 'outline'} className="text-[10px] shrink-0">
          {issue.status}
        </Badge>
        {issue.culprit ? (
          <span className="font-semibold text-sm truncate">{issue.culprit}</span>
        ) : !exceptionType ? (
          <span className="font-semibold text-sm truncate">{issue.title}</span>
        ) : null}
        {issue.priority !== 50 && (
          <span className={cn(
            'text-[10px] font-mono px-1 py-0.5 rounded shrink-0',
            issue.priority > 75 ? 'bg-red-500/15 text-red-400' :
            issue.priority > 50 ? 'bg-amber-500/15 text-amber-400' :
            issue.priority > 25 ? 'bg-blue-500/15 text-blue-400' :
            'bg-muted text-muted-foreground'
          )}>
            P{issue.priority}
          </span>
        )}
      </div>
      {exceptionType && (
        <p className="text-sm text-muted-foreground truncate mt-0.5">
          {exceptionType}{message && <> - {message}</>}
        </p>
      )}
      {issue.tags && issue.tags.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-0.5">
          {issue.tags.map(t => (
            <span key={`${t.key}:${t.value}`} className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary/80">
              {t.key}:{t.value}
            </span>
          ))}
        </div>
      )}
      <p className="text-xs text-muted-foreground/60 mt-0.5">
        {fetchedAt ? formatRelativeTime(issue.last_seen, fetchedAt) : new Date(issue.last_seen).toLocaleString()}
        {issue.first_seen !== issue.last_seen && (
          <>
            <span className="mx-1 opacity-40">&ndash;</span>
            {fetchedAt ? formatRelativeTime(issue.first_seen, fetchedAt) : new Date(issue.first_seen).toLocaleString()}
          </>
        )}
        {issue.first_release && (
          <>
            <span className="mx-1.5 opacity-40">&middot;</span>
            <span className="font-mono text-primary/70">{issue.first_release}</span>
          </>
        )}
      </p>
    </>
  )
}

function Sparkline({ data }: { data: number[] }) {
  const max = Math.max(...data, 1)
  const w = 80
  const h = 24
  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w
    const y = h - (v / max) * (h - 2) - 1
    return `${x},${y}`
  }).join(' ')
  const hasData = data.some(v => v > 0)

  if (!hasData) {
    return (
      <svg width={w} height={h} className="opacity-30">
        <line x1={0} y1={h / 2} x2={w} y2={h / 2} stroke="currentColor" strokeWidth={1} className="text-muted-foreground/40" />
      </svg>
    )
  }

  return (
    <svg width={w} height={h}>
      <polyline
        points={points}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
        className="text-muted-foreground/70"
      />
    </svg>
  )
}

function SidebarItem({ label, count, active, indent, onClick }: {
  label: string; count: number; active: boolean; indent?: boolean; onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex items-center justify-between w-full px-2 py-1.5 rounded-md text-sm transition-colors',
        indent && 'pl-5',
        active
          ? 'bg-primary/10 text-primary font-medium'
          : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
      )}
    >
      <span>{label}</span>
      <span className={cn(
        'font-mono text-xs',
        active ? 'text-primary' : 'text-muted-foreground/60'
      )}>
        {count}
      </span>
    </button>
  )
}
