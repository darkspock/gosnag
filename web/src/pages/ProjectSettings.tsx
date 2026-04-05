import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, type ProjectWithDSN, type AlertConfig, type APIToken, type JiraRule } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Bell, Copy, Key, Pencil, Plus, Settings, ShieldAlert, Trash2, Workflow } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'

const ALL_LEVELS = ['fatal', 'error', 'warning', 'info', 'debug'] as const

const LEVEL_COLORS: Record<string, string> = {
  fatal: 'bg-red-500/20 text-red-400 border-red-500/30',
  error: 'bg-red-500/20 text-red-400 border-red-500/30',
  warning: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  info: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  debug: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
}

type SettingsSection = 'general' | 'alerts' | 'tokens' | 'integrations' | 'danger'

export default function ProjectSettings() {
  const { projectId } = useParams<{ projectId: string }>()
  const { user } = useAuth()
  const navigate = useNavigate()
  const [project, setProject] = useState<ProjectWithDSN | null>(null)
  const [alerts, setAlerts] = useState<AlertConfig[]>([])
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [showTokenForm, setShowTokenForm] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenPermission, setTokenPermission] = useState('read')
  const [tokenExpiresIn, setTokenExpiresIn] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [showDeleteToken, setShowDeleteToken] = useState<string | null>(null)
  const [tokenCopied, setTokenCopied] = useState(false)

  // Jira state
  const [jiraBaseUrl, setJiraBaseUrl] = useState('')
  const [jiraEmail, setJiraEmail] = useState('')
  const [jiraApiToken, setJiraApiToken] = useState('')
  const [jiraProjectKey, setJiraProjectKey] = useState('')
  const [jiraIssueType, setJiraIssueType] = useState('Bug')
  const [jiraTesting, setJiraTesting] = useState(false)
  const [jiraRules, setJiraRules] = useState<JiraRule[]>([])
  const [showJiraRuleForm, setShowJiraRuleForm] = useState(false)
  const [editingRule, setEditingRule] = useState<JiraRule | null>(null)
  const [ruleName, setRuleName] = useState('')
  const [ruleLevelFilter, setRuleLevelFilter] = useState('')
  const [ruleMinEvents, setRuleMinEvents] = useState('')
  const [ruleMinUsers, setRuleMinUsers] = useState('')
  const [ruleTitlePattern, setRuleTitlePattern] = useState('')
  const [showDeleteRule, setShowDeleteRule] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [defaultCooldown, setDefaultCooldown] = useState('60')
  const [warningAsError, setWarningAsError] = useState(false)
  const [activeSection, setActiveSection] = useState<SettingsSection>('general')
  const [savingGeneral, setSavingGeneral] = useState(false)
  const [savingJira, setSavingJira] = useState(false)
  const [copied, setCopied] = useState(false)
  const [loading, setLoading] = useState(true)

  // Confirm dialogs
  const [showDeleteProject, setShowDeleteProject] = useState(false)
  const [showDeleteAlert, setShowDeleteAlert] = useState<string | null>(null)

  // Alert form state
  const [showAlertForm, setShowAlertForm] = useState(false)
  const [editingAlert, setEditingAlert] = useState<AlertConfig | null>(null)
  const [alertType, setAlertType] = useState('email')
  const [alertConfig, setAlertConfig] = useState('')
  const [alertLevels, setAlertLevels] = useState<string[]>([])
  const [alertPattern, setAlertPattern] = useState('')

  const isAdmin = user?.role === 'admin'

  const applyProjectState = (p: ProjectWithDSN) => {
    setProject(p)
    setName(p.name)
    setSlug(p.slug)
    setDefaultCooldown(String(p.default_cooldown_minutes ?? 60))
    setWarningAsError(p.warning_as_error)
    setJiraBaseUrl(p.jira_base_url || '')
    setJiraEmail(p.jira_email || '')
    setJiraApiToken('')
    setJiraProjectKey(p.jira_project_key || '')
    setJiraIssueType(p.jira_issue_type || 'Bug')
  }

  const refreshProject = async (id: string) => {
    const updated = await api.getProject(id)
    applyProjectState(updated)
    return updated
  }

  const buildProjectPayload = () => ({
    name,
    slug,
    default_cooldown_minutes: parseInt(defaultCooldown) || 0,
    warning_as_error: warningAsError,
    jira_base_url: jiraBaseUrl,
    jira_email: jiraEmail,
    jira_api_token: jiraApiToken,
    jira_project_key: jiraProjectKey,
    jira_issue_type: jiraIssueType,
  })

  useEffect(() => {
    if (!projectId) return
    Promise.all([
      api.getProject(projectId).then(applyProjectState),
      api.listAlerts(projectId).then(setAlerts),
      api.listTokens(projectId).then(setTokens),
      api.listJiraRules(projectId).then(setJiraRules),
    ]).finally(() => setLoading(false))
  }, [projectId])

  useEffect(() => {
    if (isAdmin) return
    if (activeSection === 'integrations' || activeSection === 'danger') {
      setActiveSection('general')
    }
  }, [activeSection, isAdmin])

  const handleSaveGeneral = async () => {
    if (!projectId) return
    setSavingGeneral(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      toast.success('General settings saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save general settings')
    } finally {
      setSavingGeneral(false)
    }
  }

  const handleSaveJira = async () => {
    if (!projectId) return
    setSavingJira(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      toast.success('Jira settings saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save Jira settings')
    } finally {
      setSavingJira(false)
    }
  }

  const handleDelete = async () => {
    if (!projectId) return
    await api.deleteProject(projectId)
    toast.success('Project deleted')
    navigate('/')
  }

  const handleCopyDSN = () => {
    if (project?.dsn) {
      navigator.clipboard.writeText(project.dsn)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      toast.success('DSN copied to clipboard')
    }
  }

  const openAddAlert = () => {
    setEditingAlert(null)
    setAlertType('email')
    setAlertConfig('')
    setAlertLevels([])
    setAlertPattern('')
    setShowAlertForm(true)
  }

  const openEditAlert = (a: AlertConfig) => {
    setEditingAlert(a)
    setAlertType(a.alert_type)
    setAlertConfig(
      a.alert_type === 'email'
        ? (a.config as { recipients?: string[] }).recipients?.join(', ') || ''
        : (a.config as { webhook_url?: string }).webhook_url || ''
    )
    setAlertLevels(a.level_filter ? a.level_filter.split(',') : [])
    setAlertPattern(a.title_pattern || '')
    setShowAlertForm(true)
  }

  const toggleLevel = (level: string) => {
    setAlertLevels(prev =>
      prev.includes(level) ? prev.filter(l => l !== level) : [...prev, level]
    )
  }

  const handleSaveAlert = async () => {
    if (!projectId) return
    try {
      const config = alertType === 'email'
        ? { recipients: alertConfig.split(',').map(s => s.trim()).filter(Boolean) }
        : { webhook_url: alertConfig.trim() }
      const levelFilter = alertLevels.join(',')

      if (editingAlert) {
        await api.updateAlert(projectId, editingAlert.id, {
          config,
          enabled: editingAlert.enabled,
          level_filter: levelFilter,
          title_pattern: alertPattern,
        })
      } else {
        await api.createAlert(projectId, {
          alert_type: alertType,
          config,
          enabled: true,
          level_filter: levelFilter,
          title_pattern: alertPattern,
        })
      }
      setAlerts(await api.listAlerts(projectId))
      setShowAlertForm(false)
      toast.success(editingAlert ? 'Alert updated' : 'Alert created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save alert')
    }
  }

  const handleToggleAlert = async (a: AlertConfig) => {
    if (!projectId) return
    const config = a.alert_type === 'email'
      ? { recipients: (a.config as { recipients?: string[] }).recipients || [] }
      : { webhook_url: (a.config as { webhook_url?: string }).webhook_url || '' }
    await api.updateAlert(projectId, a.id, {
      config,
      enabled: !a.enabled,
      level_filter: a.level_filter,
      title_pattern: a.title_pattern,
    })
    setAlerts(await api.listAlerts(projectId))
  }

  const handleDeleteAlert = async (alertId: string) => {
    if (!projectId) return
    await api.deleteAlert(projectId, alertId)
    setAlerts(await api.listAlerts(projectId))
    toast.success('Alert deleted')
  }

  const handleCreateToken = async () => {
    if (!projectId) return
    try {
      const expiresIn = tokenExpiresIn ? parseInt(tokenExpiresIn) : undefined
      const result = await api.createToken(projectId, {
        name: tokenName,
        permission: tokenPermission,
        expires_in: expiresIn,
      })
      setNewToken(result.token)
      setTokens(await api.listTokens(projectId))
      setTokenName('')
      setTokenPermission('read')
      setTokenExpiresIn('')
      toast.success('API token created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create token')
    }
  }

  const handleDeleteToken = async (tokenId: string) => {
    if (!projectId) return
    await api.deleteToken(projectId, tokenId)
    setTokens(await api.listTokens(projectId))
    toast.success('Token revoked')
  }

  const handleTestJira = async () => {
    if (!projectId) return
    setJiraTesting(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      const result = await api.testJiraConnection(projectId)
      if (result.ok) {
        toast.success('Jira connection successful')
      } else {
        toast.error(result.error || 'Connection failed')
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Connection test failed')
    } finally {
      setJiraTesting(false)
    }
  }

  const openAddRule = () => {
    setEditingRule(null)
    setRuleName('')
    setRuleLevelFilter('')
    setRuleMinEvents('')
    setRuleMinUsers('')
    setRuleTitlePattern('')
    setShowJiraRuleForm(true)
  }

  const openEditRule = (r: JiraRule) => {
    setEditingRule(r)
    setRuleName(r.name)
    setRuleLevelFilter(r.level_filter)
    setRuleMinEvents(r.min_events > 0 ? String(r.min_events) : '')
    setRuleMinUsers(r.min_users > 0 ? String(r.min_users) : '')
    setRuleTitlePattern(r.title_pattern)
    setShowJiraRuleForm(true)
  }

  const handleSaveRule = async () => {
    if (!projectId) return
    try {
      const data = {
        name: ruleName,
        enabled: editingRule ? editingRule.enabled : true,
        level_filter: ruleLevelFilter,
        min_events: parseInt(ruleMinEvents) || 0,
        min_users: parseInt(ruleMinUsers) || 0,
        title_pattern: ruleTitlePattern,
      }
      if (editingRule) {
        await api.updateJiraRule(projectId, editingRule.id, data)
      } else {
        await api.createJiraRule(projectId, data)
      }
      setJiraRules(await api.listJiraRules(projectId))
      setShowJiraRuleForm(false)
      toast.success(editingRule ? 'Rule updated' : 'Rule created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save rule')
    }
  }

  const handleToggleRule = async (r: JiraRule) => {
    if (!projectId) return
    await api.updateJiraRule(projectId, r.id, {
      name: r.name, enabled: !r.enabled, level_filter: r.level_filter,
      min_events: r.min_events, min_users: r.min_users, title_pattern: r.title_pattern,
    })
    setJiraRules(await api.listJiraRules(projectId))
  }

  const handleDeleteRule = async (ruleId: string) => {
    if (!projectId) return
    await api.deleteJiraRule(projectId, ruleId)
    setJiraRules(await api.listJiraRules(projectId))
    toast.success('Rule deleted')
  }

  const handleCopyToken = () => {
    if (newToken) {
      navigator.clipboard.writeText(newToken)
      setTokenCopied(true)
      setTimeout(() => setTokenCopied(false), 2000)
      toast.success('Token copied to clipboard')
    }
  }

  const formatAlertDestination = (a: AlertConfig) => {
    if (a.alert_type === 'email') {
      return (a.config as { recipients?: string[] }).recipients?.join(', ') || ''
    }
    return (a.config as { webhook_url?: string }).webhook_url || ''
  }

  const canTestJira = Boolean(
    jiraBaseUrl &&
    jiraProjectKey &&
    jiraEmail &&
    (jiraApiToken || project?.jira_api_token_set)
  )

  const sections = [
    {
      id: 'general' as const,
      label: 'General',
      description: 'Identity, DSN, defaults, and warning behavior',
      icon: Settings,
    },
    {
      id: 'alerts' as const,
      label: 'Alerts',
      description: `${alerts.length} configured notification flow${alerts.length === 1 ? '' : 's'}`,
      icon: Bell,
    },
    {
      id: 'tokens' as const,
      label: 'API Tokens',
      description: `${tokens.length} project token${tokens.length === 1 ? '' : 's'} available`,
      icon: Key,
    },
    ...(isAdmin
      ? [
          {
            id: 'integrations' as const,
            label: 'Integrations',
            description: project?.jira_api_token_set ? 'Jira connected and ready' : 'Connect Jira and automation',
            icon: Workflow,
          },
          {
            id: 'danger' as const,
            label: 'Danger Zone',
            description: 'Project deletion and destructive actions',
            icon: ShieldAlert,
          },
        ]
      : []),
  ]

  if (loading) return (
    <div className="text-center py-12">
      <div className="inline-block h-6 w-6 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
    </div>
  )

  return (
    <div className="space-y-6">
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        { label: project?.name || '', to: `/projects/${projectId}` },
        { label: 'Settings' },
      ]} />

      <div className="space-y-2">
        <h1 className="text-2xl font-semibold">Project Settings</h1>
        <p className="max-w-3xl text-sm text-muted-foreground">
          Settings are split by responsibility so project defaults, integrations, notifications, access, and destructive
          actions do not compete on the same screen.
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[240px,minmax(0,1fr)]">
        <aside className="lg:sticky lg:top-6 h-max">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Sections</CardTitle>
              <CardDescription>Jump between configuration areas without scrolling through unrelated forms.</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-2 sm:grid-cols-2 lg:grid-cols-1">
              {sections.map(section => {
                const Icon = section.icon
                const isActive = activeSection === section.id
                return (
                  <button
                    key={section.id}
                    type="button"
                    onClick={() => setActiveSection(section.id)}
                    className={cn(
                      'w-full rounded-lg border px-3 py-3 text-left transition-colors',
                      isActive
                        ? 'border-primary/40 bg-primary/5 shadow-sm'
                        : 'border-border/60 hover:border-border hover:bg-accent/40'
                    )}
                  >
                    <div className="flex items-start gap-3">
                      <Icon className={cn('mt-0.5 h-4 w-4', isActive ? 'text-primary' : 'text-muted-foreground')} />
                      <div className="min-w-0">
                        <div className="text-sm font-medium">{section.label}</div>
                        <p className="mt-1 text-xs text-muted-foreground">{section.description}</p>
                      </div>
                    </div>
                  </button>
                )
              })}
            </CardContent>
          </Card>
        </aside>

        <div className="space-y-6">
          {activeSection === 'general' && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">General</p>
                <h2 className="text-xl font-semibold">Project identity and client setup</h2>
                <p className="text-sm text-muted-foreground">
                  Core project metadata, DSN delivery, and default issue handling live here.
                </p>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">DSN (Client Key)</CardTitle>
                  <CardDescription>Use this DSN in your Sentry SDK configuration. This value is read-only.</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono break-all">
                      {project?.dsn}
                    </code>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button variant="outline" size="icon" onClick={handleCopyDSN}>
                          <Copy className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Copy DSN</TooltipContent>
                    </Tooltip>
                  </div>
                  {copied && <p className="mt-1 text-xs text-emerald-400">Copied!</p>}
                </CardContent>
              </Card>

              {isAdmin && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Project Defaults</CardTitle>
                    <CardDescription>These values shape how new events are grouped and how resolution behaves by default.</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-5">
                    <div className="grid gap-4 md:grid-cols-2">
                      <div>
                        <label className="text-sm font-medium">Name</label>
                        <Input value={name} onChange={e => setName(e.target.value)} className="mt-1" />
                      </div>
                      <div>
                        <label className="text-sm font-medium">Slug</label>
                        <Input value={slug} onChange={e => setSlug(e.target.value)} className="mt-1" />
                      </div>
                    </div>

                    <div className="grid gap-4 md:grid-cols-2">
                      <div>
                        <label className="text-sm font-medium">Default Cooldown</label>
                        <Select value={defaultCooldown} onChange={e => setDefaultCooldown(e.target.value)} className="mt-1">
                          <option value="0">No cooldown</option>
                          <option value="60">1 hour</option>
                          <option value="120">2 hours</option>
                          <option value="1440">1 day</option>
                          <option value="2880">2 days</option>
                          <option value="10080">1 week</option>
                        </Select>
                        <p className="mt-1 text-xs text-muted-foreground">
                          Used when resolving issues with the project default option.
                        </p>
                      </div>

                      <div>
                        <label className="text-sm font-medium">Warning Handling</label>
                        <Select
                          value={warningAsError ? 'error' : 'warning'}
                          onChange={e => setWarningAsError(e.target.value === 'error')}
                          className="mt-1"
                        >
                          <option value="warning">Keep warnings separate</option>
                          <option value="error">Treat warnings as errors</option>
                        </Select>
                        <p className="mt-1 text-xs text-muted-foreground">
                          Promote incoming warning events to error issues when enabled.
                        </p>
                      </div>
                    </div>

                    <div className="flex flex-col-reverse gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                      <p className="text-xs text-muted-foreground">General settings save independently from tokens, alerts, and Jira rules.</p>
                      <Button onClick={handleSaveGeneral} disabled={savingGeneral}>
                        {savingGeneral ? 'Saving...' : 'Save General Settings'}
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              )}
            </>
          )}

          {activeSection === 'alerts' && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Alerts</p>
                <h2 className="text-xl font-semibold">Notification routing</h2>
                <p className="text-sm text-muted-foreground">
                  Alerts are operational objects. Create, enable, disable, and delete them without touching project defaults.
                </p>
              </div>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-base">Alert Destinations</CardTitle>
                    <CardDescription>Email and webhook routes for issue notifications.</CardDescription>
                  </div>
                  {isAdmin && (
                    <Button size="sm" variant="outline" onClick={openAddAlert}>
                      <Plus className="mr-1 h-4 w-4" /> Add Alert
                    </Button>
                  )}
                </CardHeader>
                <CardContent>
                  {alerts.length === 0 ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No alerts configured yet.</p>
                      {isAdmin && (
                        <p className="mt-1 text-xs text-muted-foreground/60">
                          Add an alert to notify a team or automation target when issues arrive.
                        </p>
                      )}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {alerts.map(a => (
                        <div key={a.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium capitalize">{a.alert_type}</span>
                              <button
                                onClick={() => handleToggleAlert(a)}
                                className={cn(
                                  'rounded px-1.5 py-0.5 text-xs transition-colors',
                                  a.enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-muted text-muted-foreground'
                                )}
                              >
                                {a.enabled ? 'Active' : 'Disabled'}
                              </button>
                            </div>
                            {isAdmin && (
                              <div className="flex items-center gap-1">
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditAlert(a)}>
                                      <Pencil className="h-3.5 w-3.5" />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent>Edit alert</TooltipContent>
                                </Tooltip>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeleteAlert(a.id)}>
                                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent>Delete alert</TooltipContent>
                                </Tooltip>
                              </div>
                            )}
                          </div>
                          <p className="mt-1 truncate text-xs text-muted-foreground">{formatAlertDestination(a)}</p>
                          <div className="mt-2 flex flex-wrap items-center gap-1.5">
                            {a.level_filter ? (
                              a.level_filter.split(',').map(l => (
                                <span key={l} className={cn('rounded border px-1.5 py-0.5 text-xs', LEVEL_COLORS[l])}>
                                  {l}
                                </span>
                              ))
                            ) : (
                              <span className="text-xs text-muted-foreground">All levels</span>
                            )}
                            {a.title_pattern && (
                              <>
                                <span className="mx-0.5 text-xs text-muted-foreground/40">&middot;</span>
                                <span className="font-mono text-xs text-muted-foreground">contains "{a.title_pattern}"</span>
                              </>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </>
          )}

          {activeSection === 'tokens' && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">API Tokens</p>
                <h2 className="text-xl font-semibold">Project-scoped access</h2>
                <p className="text-sm text-muted-foreground">
                  Create scoped credentials for external systems without mixing them into the main settings form.
                </p>
              </div>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1">
                    <CardTitle className="flex items-center gap-2 text-base">
                      <Key className="h-4 w-4" /> API Tokens
                    </CardTitle>
                    <CardDescription>One token per integration is easier to rotate and revoke safely.</CardDescription>
                  </div>
                  {isAdmin && (
                    <Button size="sm" variant="outline" onClick={() => { setShowTokenForm(true); setNewToken(null) }}>
                      <Plus className="mr-1 h-4 w-4" /> Create Token
                    </Button>
                  )}
                </CardHeader>
                <CardContent>
                  {tokens.length === 0 && !newToken ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No API tokens yet.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">
                        Create a token to access this project's API from CI, scripts, or external dashboards.
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {tokens.map(t => (
                        <div key={t.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{t.name}</span>
                              <span
                                className={cn(
                                  'rounded px-1.5 py-0.5 text-xs',
                                  t.permission === 'readwrite'
                                    ? 'bg-amber-500/15 text-amber-400'
                                    : 'bg-blue-500/15 text-blue-400'
                                )}
                              >
                                {t.permission}
                              </span>
                            </div>
                            {isAdmin && (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeleteToken(t.id)}>
                                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Revoke token</TooltipContent>
                              </Tooltip>
                            )}
                          </div>
                          <div className="mt-1 flex flex-wrap gap-3 text-xs text-muted-foreground">
                            <span>Created {new Date(t.created_at).toLocaleDateString()}</span>
                            {t.last_used_at && <span>Last used {new Date(t.last_used_at).toLocaleDateString()}</span>}
                            {t.expires_at && <span>Expires {new Date(t.expires_at).toLocaleDateString()}</span>}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </>
          )}

          {activeSection === 'integrations' && isAdmin && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Integrations</p>
                <h2 className="text-xl font-semibold">Jira connection and automation</h2>
                <p className="text-sm text-muted-foreground">
                  Connect Jira once, then manage ticket automation rules independently from project metadata.
                </p>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Jira Cloud</CardTitle>
                  <CardDescription>Connection details for manual and automatic Jira ticket creation.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-5">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Jira URL</label>
                      <Input value={jiraBaseUrl} onChange={e => setJiraBaseUrl(e.target.value)} placeholder="https://company.atlassian.net" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Project Key</label>
                      <Input value={jiraProjectKey} onChange={e => setJiraProjectKey(e.target.value)} placeholder="e.g. DEV" className="mt-1" />
                    </div>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Email</label>
                      <Input value={jiraEmail} onChange={e => setJiraEmail(e.target.value)} placeholder="user@company.com" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">API Token</label>
                      <Input
                        type="password"
                        value={jiraApiToken}
                        onChange={e => setJiraApiToken(e.target.value)}
                        placeholder={project?.jira_api_token_set ? '••••••••• (configured)' : 'Jira API token'}
                        className="mt-1"
                      />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Leave blank to keep the currently stored credential.
                      </p>
                    </div>
                  </div>

                  <div>
                    <label className="text-sm font-medium">Issue Type</label>
                    <Select value={jiraIssueType} onChange={e => setJiraIssueType(e.target.value)} className="mt-1">
                      <option value="Bug">Bug</option>
                      <option value="Task">Task</option>
                      <option value="Story">Story</option>
                    </Select>
                  </div>

                  <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs text-muted-foreground">
                      {project?.jira_api_token_set ? 'A Jira API token is already stored for this project.' : 'No Jira API token stored yet.'}
                    </p>
                    <div className="flex flex-col-reverse gap-2 sm:flex-row">
                      <Button variant="outline" onClick={handleTestJira} disabled={jiraTesting || !canTestJira}>
                        {jiraTesting ? 'Testing...' : 'Test Connection'}
                      </Button>
                      <Button onClick={handleSaveJira} disabled={savingJira}>
                        {savingJira ? 'Saving...' : 'Save Jira Settings'}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-base">Jira Auto-Creation Rules</CardTitle>
                    <CardDescription>Rules determine when GoSnag should open Jira tickets automatically.</CardDescription>
                  </div>
                  <Button size="sm" variant="outline" onClick={openAddRule} disabled={!jiraBaseUrl}>
                    <Plus className="mr-1 h-4 w-4" /> Add Rule
                  </Button>
                </CardHeader>
                <CardContent>
                  {!jiraBaseUrl ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">Configure Jira first.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">
                        Rules become available once the Jira base URL is set.
                      </p>
                    </div>
                  ) : jiraRules.length === 0 ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No auto-creation rules yet.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">
                        Add a rule to automatically create Jira tickets when issues match conditions.
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {jiraRules.map(r => (
                        <div key={r.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{r.name}</span>
                              <button
                                onClick={() => handleToggleRule(r)}
                                className={cn(
                                  'rounded px-1.5 py-0.5 text-xs transition-colors',
                                  r.enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-muted text-muted-foreground'
                                )}
                              >
                                {r.enabled ? 'Active' : 'Disabled'}
                              </button>
                            </div>
                            <div className="flex items-center gap-1">
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditRule(r)}>
                                    <Pencil className="h-3.5 w-3.5" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Edit rule</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeleteRule(r.id)}>
                                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Delete rule</TooltipContent>
                              </Tooltip>
                            </div>
                          </div>
                          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                            {r.level_filter && <span>Levels: {r.level_filter}</span>}
                            {r.min_events > 0 && <span>Min events: {r.min_events}</span>}
                            {r.min_users > 0 && <span>Min users: {r.min_users}</span>}
                            {r.title_pattern && <span className="font-mono">Pattern: {r.title_pattern}</span>}
                            {!r.level_filter && r.min_events === 0 && r.min_users === 0 && !r.title_pattern && (
                              <span>All issues (no conditions)</span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </>
          )}

          {activeSection === 'danger' && isAdmin && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Danger Zone</p>
                <h2 className="text-xl font-semibold">Destructive actions</h2>
                <p className="text-sm text-muted-foreground">
                  Keep irreversible actions isolated from normal configuration to reduce accidental clicks.
                </p>
              </div>

              <Card className="border-destructive/30">
                <CardHeader>
                  <CardTitle className="text-base text-destructive">Delete Project</CardTitle>
                  <CardDescription>
                    This removes the project, its issues, events, alerts, tokens, and integration settings.
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                  <p className="max-w-2xl text-sm text-muted-foreground">
                    Use this only when you are certain the project and all historical data should disappear permanently.
                  </p>
                  <Button variant="destructive" onClick={() => setShowDeleteProject(true)}>
                    <Trash2 className="mr-1 h-4 w-4" /> Delete Project
                  </Button>
                </CardContent>
              </Card>
            </>
          )}
        </div>
      </div>

      {/* Create Token Dialog */}
      <Dialog open={showTokenForm} onOpenChange={open => { if (!open) { setShowTokenForm(false); setNewToken(null) } }}>
        <DialogContent>
          <DialogTitle>Create API Token</DialogTitle>
          <DialogDescription className="sr-only">Create a new API token for external access</DialogDescription>
          {newToken ? (
            <div className="mt-4 space-y-4">
              <p className="text-sm text-amber-400">Copy this token now. It won't be shown again.</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 bg-muted px-3 py-2 rounded text-sm font-mono break-all">{newToken}</code>
                <Button variant="outline" size="icon" onClick={handleCopyToken}>
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              {tokenCopied && <p className="text-xs text-emerald-400">Copied!</p>}
              <p className="text-xs text-muted-foreground">
                Use this token as: <code className="text-xs">Authorization: Bearer {newToken.substring(0, 12)}...</code>
              </p>
              <div className="flex justify-end">
                <Button onClick={() => { setShowTokenForm(false); setNewToken(null) }}>Done</Button>
              </div>
            </div>
          ) : (
            <div className="mt-4 space-y-4">
              <div>
                <label className="text-sm font-medium">Name</label>
                <Input
                  value={tokenName}
                  onChange={e => setTokenName(e.target.value)}
                  placeholder="e.g. CI/CD, Monitoring, Dashboard"
                  className="mt-1"
                />
              </div>
              <div>
                <label className="text-sm font-medium">Permission</label>
                <Select value={tokenPermission} onChange={e => setTokenPermission(e.target.value)} className="mt-1">
                  <option value="read">Read only — list and view issues</option>
                  <option value="readwrite">Read & Write — also resolve, assign, delete</option>
                </Select>
              </div>
              <div>
                <label className="text-sm font-medium">Expires in</label>
                <Select value={tokenExpiresIn} onChange={e => setTokenExpiresIn(e.target.value)} className="mt-1">
                  <option value="">Never</option>
                  <option value="30">30 days</option>
                  <option value="90">90 days</option>
                  <option value="365">1 year</option>
                </Select>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setShowTokenForm(false)}>Cancel</Button>
                <Button onClick={handleCreateToken} disabled={!tokenName.trim()}>Create</Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Jira Rule Form Dialog */}
      <Dialog open={showJiraRuleForm} onOpenChange={setShowJiraRuleForm}>
        <DialogContent>
          <DialogTitle>{editingRule ? 'Edit Rule' : 'Add Rule'}</DialogTitle>
          <DialogDescription className="sr-only">Configure Jira auto-creation rule</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <label className="text-sm font-medium">Name</label>
              <Input value={ruleName} onChange={e => setRuleName(e.target.value)} placeholder="e.g. Critical errors" className="mt-1" />
            </div>
            <div>
              <label className="text-sm font-medium">Level filter</label>
              <p className="text-xs text-muted-foreground mb-1">Comma-separated. Empty = all levels.</p>
              <Input value={ruleLevelFilter} onChange={e => setRuleLevelFilter(e.target.value)} placeholder="e.g. fatal,error" className="mt-1" />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium">Min events</label>
                <Input type="number" value={ruleMinEvents} onChange={e => setRuleMinEvents(e.target.value)} placeholder="0" className="mt-1" />
              </div>
              <div>
                <label className="text-sm font-medium">Min users</label>
                <Input type="number" value={ruleMinUsers} onChange={e => setRuleMinUsers(e.target.value)} placeholder="0" className="mt-1" />
              </div>
            </div>
            <div>
              <label className="text-sm font-medium">Title pattern</label>
              <p className="text-xs text-muted-foreground mb-1">Regex or plain text. Empty = match all.</p>
              <Input value={ruleTitlePattern} onChange={e => setRuleTitlePattern(e.target.value)} placeholder="e.g. database|timeout" className="mt-1" />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowJiraRuleForm(false)}>Cancel</Button>
              <Button onClick={handleSaveRule} disabled={!ruleName.trim()}>{editingRule ? 'Save' : 'Add'}</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete Rule Confirm */}
      <ConfirmDialog
        open={!!showDeleteRule}
        onOpenChange={open => { if (!open) setShowDeleteRule(null) }}
        title="Delete Rule"
        description="This auto-creation rule will be permanently deleted."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (showDeleteRule) handleDeleteRule(showDeleteRule) }}
      />

      {/* Revoke Token Confirm */}
      <ConfirmDialog
        open={!!showDeleteToken}
        onOpenChange={open => { if (!open) setShowDeleteToken(null) }}
        title="Revoke Token"
        description="This token will be permanently revoked. Any systems using it will lose access immediately."
        confirmLabel="Revoke"
        variant="destructive"
        onConfirm={() => { if (showDeleteToken) handleDeleteToken(showDeleteToken) }}
      />

      {/* Delete Project Confirm */}
      <ConfirmDialog
        open={showDeleteProject}
        onOpenChange={setShowDeleteProject}
        title="Delete Project"
        description="This will permanently delete this project and all its issues, events, and alerts. This action cannot be undone."
        confirmLabel="Delete Project"
        variant="destructive"
        onConfirm={handleDelete}
      />

      {/* Delete Alert Confirm */}
      <ConfirmDialog
        open={!!showDeleteAlert}
        onOpenChange={open => { if (!open) setShowDeleteAlert(null) }}
        title="Delete Alert"
        description="This alert will be permanently deleted. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (showDeleteAlert) handleDeleteAlert(showDeleteAlert) }}
      />

      {/* Add/Edit Alert Dialog */}
      <Dialog open={showAlertForm} onOpenChange={setShowAlertForm}>
        <DialogContent>
          <DialogTitle>{editingAlert ? 'Edit Alert' : 'Add Alert'}</DialogTitle>
          <DialogDescription className="sr-only">Configure alert settings</DialogDescription>
          <div className="mt-4 space-y-4">
            {!editingAlert && (
              <div>
                <label className="text-sm font-medium">Type</label>
                <Select value={alertType} onChange={e => setAlertType(e.target.value)} className="mt-1">
                  <option value="email">Email</option>
                  <option value="slack">Slack</option>
                </Select>
              </div>
            )}
            <div>
              <label className="text-sm font-medium">
                {alertType === 'email' ? 'Recipients (comma separated)' : 'Webhook URL'}
              </label>
              <Input
                value={alertConfig}
                onChange={e => setAlertConfig(e.target.value)}
                placeholder={alertType === 'email' ? 'dev@example.com, ops@example.com' : 'https://hooks.slack.com/...'}
                className="mt-1"
              />
            </div>
            <div>
              <label className="text-sm font-medium">Levels</label>
              <p className="text-xs text-muted-foreground mb-2">Select which levels trigger this alert. None selected = all levels.</p>
              <div className="flex flex-wrap gap-2">
                {ALL_LEVELS.map(level => (
                  <button
                    key={level}
                    onClick={() => toggleLevel(level)}
                    className={cn(
                      'text-xs px-2.5 py-1.5 rounded border transition-colors',
                      alertLevels.includes(level)
                        ? LEVEL_COLORS[level]
                        : 'border-border/60 text-muted-foreground hover:text-foreground hover:border-border'
                    )}
                  >
                    {level}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="text-sm font-medium">Title filter</label>
              <p className="text-xs text-muted-foreground mb-1">Only alert when the issue title matches. Leave empty for all issues.</p>
              <Input
                value={alertPattern}
                onChange={e => setAlertPattern(e.target.value)}
                placeholder="e.g. database or ^Fatal.*timeout$"
                className="mt-1"
              />
              <p className="text-xs text-muted-foreground mt-1">Plain text = contains match. Supports regex.</p>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowAlertForm(false)}>Cancel</Button>
              <Button onClick={handleSaveAlert}>{editingAlert ? 'Save' : 'Add'}</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
