import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, type ProjectWithDSN, type AlertConfig, type APIToken, type JiraRule, type GithubRule, type ProjectGroup, type PriorityRule, type TagRule, type AIUsage, type RuleSuggestion } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Bell, Brain, Check, Copy, Gauge, Key, Loader2, MessageSquare, Pencil, Plus, Send, Settings, ShieldAlert, Sparkles, Tag, Trash2, X, Workflow } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from '@/lib/use-toast'
import { ConditionBuilder, type ConditionGroup, type ConditionNode } from '@/components/ui/condition-builder'

function legacyToConditions(a: { level_filter?: string; title_pattern?: string; exclude_pattern?: string; min_events?: number; min_velocity_1h?: number }): ConditionGroup {
  const conds: ConditionNode[] = []
  if (a.level_filter) {
    const levels = a.level_filter.split(',').map(s => s.trim()).filter(Boolean)
    if (levels.length === 1) {
      conds.push({ type: 'level', op: 'eq', value: levels[0] })
    } else if (levels.length > 1) {
      conds.push({ type: 'level', op: 'in', value: levels })
    }
  }
  if (a.title_pattern) {
    conds.push({ type: 'title', op: 'contains', value: a.title_pattern })
  }
  if (a.exclude_pattern) {
    conds.push({ type: 'title', op: 'not_contains', value: a.exclude_pattern })
  }
  if (a.min_events && a.min_events > 0) {
    conds.push({ type: 'total_events', op: 'gte', value: a.min_events })
  }
  if (a.min_velocity_1h && a.min_velocity_1h > 0) {
    conds.push({ type: 'velocity_1h', op: 'gte', value: a.min_velocity_1h })
  }
  return { operator: 'and', conditions: conds }
}

const LEVEL_COLORS: Record<string, string> = {
  fatal: 'bg-red-500/20 text-red-400 border-red-500/30',
  error: 'bg-red-500/20 text-red-400 border-red-500/30',
  warning: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  info: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  debug: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
}

type SettingsSection = 'general' | 'alerts' | 'tokens' | 'priority' | 'tags' | 'ai' | 'integrations' | 'danger'

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

  // GitHub state
  const [githubOwner, setGithubOwner] = useState('')
  const [githubRepo, setGithubRepo] = useState('')
  const [githubToken, setGithubToken] = useState('')
  const [githubLabels, setGithubLabels] = useState('bug')
  const [githubTesting, setGithubTesting] = useState(false)
  const [githubRules, setGithubRules] = useState<GithubRule[]>([])
  const [showGithubRuleForm, setShowGithubRuleForm] = useState(false)
  const [editingGithubRule, setEditingGithubRule] = useState<GithubRule | null>(null)
  const [ghRuleName, setGhRuleName] = useState('')
  const [ghRuleLevelFilter, setGhRuleLevelFilter] = useState('')
  const [ghRuleMinEvents, setGhRuleMinEvents] = useState('')
  const [ghRuleMinUsers, setGhRuleMinUsers] = useState('')
  const [ghRuleTitlePattern, setGhRuleTitlePattern] = useState('')
  const [showDeleteGithubRule, setShowDeleteGithubRule] = useState<string | null>(null)

  const [priorityRules, setPriorityRules] = useState<PriorityRule[]>([])
  const [showPriorityRuleForm, setShowPriorityRuleForm] = useState(false)
  const [editingPriorityRule, setEditingPriorityRule] = useState<PriorityRule | null>(null)
  const [prRuleName, setPrRuleName] = useState('')
  const [prRuleType, setPrRuleType] = useState('level_is')
  const [prPattern, setPrPattern] = useState('')
  const [prOperator, setPrOperator] = useState('gte')
  const [prThreshold, setPrThreshold] = useState('')
  const [prPoints, setPrPoints] = useState('')
  const [showDeletePriorityRule, setShowDeletePriorityRule] = useState<string | null>(null)
  const [recalcing, setRecalcing] = useState(false)
  // AI assistant dialog
  const [showAIAssistant, setShowAIAssistant] = useState(false)
  const [assistantMessages, setAssistantMessages] = useState<{ role: string; content: string }[]>([])
  const [assistantInput, setAssistantInput] = useState('')
  const [assistantLoading, setAssistantLoading] = useState(false)
  const [assistantSuggestions, setAssistantSuggestions] = useState<RuleSuggestion[]>([])
  const [assistantIncludeIssues, setAssistantIncludeIssues] = useState(false)
  const [tagRules, setTagRules] = useState<TagRule[]>([])
  const [showTagRuleForm, setShowTagRuleForm] = useState(false)
  const [editingTagRule, setEditingTagRule] = useState<TagRule | null>(null)
  const [trName, setTrName] = useState('')
  const [trPattern, setTrPattern] = useState('')
  const [trTagKey, setTrTagKey] = useState('')
  const [trTagValue, setTrTagValue] = useState('')
  const [showDeleteTagRule, setShowDeleteTagRule] = useState<string | null>(null)
  const [allGroups, setAllGroups] = useState<ProjectGroup[]>([])
  const [selectedGroupId, setSelectedGroupId] = useState<string>('')
  // AI state
  const [aiEnabled, setAiEnabled] = useState(false)
  const [aiModel, setAiModel] = useState('')
  const [aiMergeSuggestions, setAiMergeSuggestions] = useState(false)
  const [aiAutoMerge, setAiAutoMerge] = useState(false)
  const [aiTicketDescription, setAiTicketDescription] = useState(true)
  const [aiRootCause, setAiRootCause] = useState(false)
  const [aiProviderConfigured, setAiProviderConfigured] = useState(false)
  const [aiProviderName, setAiProviderName] = useState('')
  const [aiUsage, setAiUsage] = useState<AIUsage | null>(null)
  const [savingAI, setSavingAI] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [defaultCooldown, setDefaultCooldown] = useState('60')
  const [warningAsError, setWarningAsError] = useState(false)
  const [maxEventsPerIssue, setMaxEventsPerIssue] = useState('1000')
  const [issueDisplayMode, setIssueDisplayMode] = useState('classic')
  const [workflowMode, setWorkflowMode] = useState('simple')
  const [repoProvider, setRepoProvider] = useState('')
  const [repoOwner, setRepoOwner] = useState('')
  const [repoName, setRepoName] = useState('')
  const [repoDefaultBranch, setRepoDefaultBranch] = useState('main')
  const [repoToken, setRepoToken] = useState('')
  const [repoPathStrip, setRepoPathStrip] = useState('')
  const [repoTesting, setRepoTesting] = useState(false)
  const [savingRepo, setSavingRepo] = useState(false)
  const [activeSection, setActiveSection] = useState<SettingsSection>('general')
  const [savingGeneral, setSavingGeneral] = useState(false)
  const [savingJira, setSavingJira] = useState(false)
  const [savingGithub, setSavingGithub] = useState(false)
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
  const [alertConditions, setAlertConditions] = useState<ConditionGroup>({ operator: 'and', conditions: [] })

  const isAdmin = user?.role === 'admin'

  const applyProjectState = (p: ProjectWithDSN) => {
    setProject(p)
    setName(p.name)
    setSlug(p.slug)
    setDefaultCooldown(String(p.default_cooldown_minutes ?? 60))
    setWarningAsError(p.warning_as_error)
    setMaxEventsPerIssue(String(p.max_events_per_issue ?? 1000))
    setJiraBaseUrl(p.jira_base_url || '')
    setJiraEmail(p.jira_email || '')
    setJiraApiToken('')
    setJiraProjectKey(p.jira_project_key || '')
    setJiraIssueType(p.jira_issue_type || 'Bug')
    setGithubOwner(p.github_owner || '')
    setGithubRepo(p.github_repo || '')
    setGithubToken('')
    setGithubLabels(p.github_labels || 'bug')
    setIssueDisplayMode(p.issue_display_mode || 'classic')
    setWorkflowMode(p.workflow_mode || 'simple')
    setRepoProvider(p.repo_provider || '')
    setRepoOwner(p.repo_owner || '')
    setRepoName(p.repo_name || '')
    setRepoDefaultBranch(p.repo_default_branch || 'main')
    setRepoToken('')
    setRepoPathStrip(p.repo_path_strip || '')
    setSelectedGroupId(p.group_id || '')
    setAiEnabled(p.ai_enabled)
    setAiModel(p.ai_model || '')
    setAiMergeSuggestions(p.ai_merge_suggestions)
    setAiAutoMerge(p.ai_auto_merge)
    setAiTicketDescription(p.ai_ticket_description)
    setAiRootCause(p.ai_root_cause)
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
    max_events_per_issue: parseInt(maxEventsPerIssue) || 0,
    issue_display_mode: issueDisplayMode,
    workflow_mode: workflowMode,
    repo_provider: repoProvider,
    repo_owner: repoOwner,
    repo_name: repoName,
    repo_default_branch: repoDefaultBranch,
    repo_token: repoToken,
    repo_path_strip: repoPathStrip,
    jira_base_url: jiraBaseUrl,
    jira_email: jiraEmail,
    jira_api_token: jiraApiToken,
    jira_project_key: jiraProjectKey,
    jira_issue_type: jiraIssueType,
    github_token: githubToken,
    github_owner: githubOwner,
    github_repo: githubRepo,
    github_labels: githubLabels,
    group_id: selectedGroupId || null,
    ai_enabled: aiEnabled,
    ai_model: aiModel,
    ai_merge_suggestions: aiMergeSuggestions,
    ai_auto_merge: aiAutoMerge,
    ai_ticket_description: aiTicketDescription,
    ai_root_cause: aiRootCause,
  })

  useEffect(() => {
    if (!projectId) return
    Promise.all([
      api.getProject(projectId).then(applyProjectState),
      api.listAlerts(projectId).then(setAlerts),
      api.listTokens(projectId).then(setTokens),
      api.listJiraRules(projectId).then(setJiraRules),
      api.listGithubRules(projectId).then(setGithubRules),
      api.listGroups().then(setAllGroups),
      api.listPriorityRules(projectId).then(setPriorityRules),
      api.listTagRules(projectId).then(setTagRules),
      api.getAIStatus(projectId).then(s => { setAiProviderConfigured(s.provider_configured); setAiProviderName(s.provider) }).catch(() => {}),
      api.getAIUsage(projectId).then(setAiUsage).catch(() => {}),
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

  const handleSaveRepo = async () => {
    if (!projectId) return
    setSavingRepo(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      toast.success('Repository settings saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSavingRepo(false)
    }
  }

  const handleTestRepo = async () => {
    if (!projectId) return
    setRepoTesting(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      const result = await api.testRepoConnection(projectId)
      if (result.ok) {
        toast.success('Repository connection successful')
      } else {
        toast.error(result.error || 'Connection failed')
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Connection test failed')
    } finally {
      setRepoTesting(false)
    }
  }

  const handleSaveGithub = async () => {
    if (!projectId) return
    setSavingGithub(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      toast.success('GitHub settings saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save GitHub settings')
    } finally {
      setSavingGithub(false)
    }
  }

  const handleSaveAI = async () => {
    if (!projectId) return
    setSavingAI(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      const usage = await api.getAIUsage(projectId).catch(() => null)
      if (usage) setAiUsage(usage)
      toast.success('AI settings saved')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save AI settings')
    } finally {
      setSavingAI(false)
    }
  }

  const handleDelete = async () => {
    if (!projectId) return
    await api.deleteProject(projectId)
    toast.success('Project deleted')
    navigate('/')
  }

  const handleCopyDSN = (dsn?: string) => {
    if (dsn) {
      navigator.clipboard.writeText(dsn)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      toast.success('DSN copied to clipboard')
    }
  }

  const openAddAlert = () => {
    setEditingAlert(null)
    setAlertType('email')
    setAlertConfig('')
    setAlertConditions({ operator: 'and', conditions: [] })
    setShowAlertForm(true)
  }

  const openEditAlert = (a: AlertConfig) => {
    setEditingAlert(a)
    setAlertType(a.alert_type)
    setAlertConfig(
      a.alert_type === 'email'
        ? (a.config as { recipients?: string[] }).recipients?.join(', ') || ''
        : (a.config as { webhook_url?: string }).webhook_url || ''  // will be empty when redacted
    )
    // If conditions JSONB exists, use it; otherwise auto-convert legacy fields
    if (a.conditions) {
      setAlertConditions(a.conditions as unknown as ConditionGroup)
    } else {
      setAlertConditions(legacyToConditions(a))
    }
    setShowAlertForm(true)
  }

  const handleSaveAlert = async () => {
    if (!projectId) return
    try {
      const config = alertType === 'email'
        ? { recipients: alertConfig.split(',').map(s => s.trim()).filter(Boolean) }
        : { webhook_url: alertConfig.trim() }  // empty = server preserves existing
      const hasConditions = alertConditions.conditions.length > 0
      const conditionsPayload = hasConditions ? alertConditions : undefined

      // Always send legacy fields to preserve them for alerts without conditions
      const legacyFields = {
        level_filter: editingAlert?.level_filter || '',
        title_pattern: editingAlert?.title_pattern || '',
        min_events: editingAlert?.min_events || 0,
        min_velocity_1h: editingAlert?.min_velocity_1h || 0,
        exclude_pattern: editingAlert?.exclude_pattern || '',
      }

      if (editingAlert) {
        await api.updateAlert(projectId, editingAlert.id, {
          config,
          enabled: editingAlert.enabled,
          ...legacyFields,
          conditions: conditionsPayload,
        })
      } else {
        await api.createAlert(projectId, {
          alert_type: alertType,
          config,
          enabled: true,
          ...legacyFields,
          conditions: conditionsPayload,
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
      : { webhook_url: '' }  // empty = server preserves existing webhook
    await api.updateAlert(projectId, a.id, {
      config,
      enabled: !a.enabled,
      level_filter: a.level_filter,
      title_pattern: a.title_pattern,
      min_events: a.min_events || 0,
      min_velocity_1h: a.min_velocity_1h || 0,
      exclude_pattern: a.exclude_pattern || '',
      conditions: a.conditions || undefined,
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

  // GitHub handlers
  const handleTestGithub = async () => {
    if (!projectId) return
    setGithubTesting(true)
    try {
      await api.updateProject(projectId, buildProjectPayload())
      await refreshProject(projectId)
      const result = await api.testGithubConnection(projectId)
      if (result.ok) {
        toast.success('GitHub connection successful')
      } else {
        toast.error(result.error || 'Connection failed')
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Connection test failed')
    } finally {
      setGithubTesting(false)
    }
  }

  const openAddGithubRule = () => {
    setEditingGithubRule(null)
    setGhRuleName('')
    setGhRuleLevelFilter('')
    setGhRuleMinEvents('')
    setGhRuleMinUsers('')
    setGhRuleTitlePattern('')
    setShowGithubRuleForm(true)
  }

  const openEditGithubRule = (r: GithubRule) => {
    setEditingGithubRule(r)
    setGhRuleName(r.name)
    setGhRuleLevelFilter(r.level_filter)
    setGhRuleMinEvents(r.min_events > 0 ? String(r.min_events) : '')
    setGhRuleMinUsers(r.min_users > 0 ? String(r.min_users) : '')
    setGhRuleTitlePattern(r.title_pattern)
    setShowGithubRuleForm(true)
  }

  const handleSaveGithubRule = async () => {
    if (!projectId) return
    try {
      const data = {
        name: ghRuleName,
        enabled: editingGithubRule ? editingGithubRule.enabled : true,
        level_filter: ghRuleLevelFilter,
        min_events: parseInt(ghRuleMinEvents) || 0,
        min_users: parseInt(ghRuleMinUsers) || 0,
        title_pattern: ghRuleTitlePattern,
      }
      if (editingGithubRule) {
        await api.updateGithubRule(projectId, editingGithubRule.id, data)
      } else {
        await api.createGithubRule(projectId, data)
      }
      setGithubRules(await api.listGithubRules(projectId))
      setShowGithubRuleForm(false)
      toast.success(editingGithubRule ? 'Rule updated' : 'Rule created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save rule')
    }
  }

  const handleToggleGithubRule = async (r: GithubRule) => {
    if (!projectId) return
    await api.updateGithubRule(projectId, r.id, {
      name: r.name, enabled: !r.enabled, level_filter: r.level_filter,
      min_events: r.min_events, min_users: r.min_users, title_pattern: r.title_pattern,
    })
    setGithubRules(await api.listGithubRules(projectId))
  }

  const handleDeleteGithubRule = async (ruleId: string) => {
    if (!projectId) return
    await api.deleteGithubRule(projectId, ruleId)
    setGithubRules(await api.listGithubRules(projectId))
    toast.success('Rule deleted')
  }

  const RULE_TYPES = [
    { value: 'level_is', label: 'Level is', needsPattern: true, needsThreshold: false, needsPrompt: false },
    { value: 'platform_is', label: 'Platform is', needsPattern: true, needsThreshold: false, needsPrompt: false },
    { value: 'title_contains', label: 'Title contains', needsPattern: true, needsThreshold: false, needsPrompt: false },
    { value: 'title_not_contains', label: 'Title does not contain', needsPattern: true, needsThreshold: false, needsPrompt: false },
    { value: 'total_events', label: 'Total events', needsPattern: false, needsThreshold: true, needsPrompt: false },
    { value: 'velocity_1h', label: 'Events per hour', needsPattern: false, needsThreshold: true, needsPrompt: false },
    { value: 'velocity_24h', label: 'Events per 24h', needsPattern: false, needsThreshold: true, needsPrompt: false },
    { value: 'user_count', label: 'Affected users', needsPattern: false, needsThreshold: true, needsPrompt: false },
    ...(aiEnabled && aiProviderConfigured ? [{ value: 'ai_prompt', label: 'AI Prompt', needsPattern: false, needsThreshold: false, needsPrompt: true }] : []),
  ]

  const openAddPriorityRule = () => {
    setEditingPriorityRule(null)
    setPrRuleName('')
    setPrRuleType('level_is')
    setPrPattern('')
    setPrOperator('gte')
    setPrThreshold('')
    setPrPoints('')
    setShowPriorityRuleForm(true)
  }

  const openEditPriorityRule = (r: PriorityRule) => {
    setEditingPriorityRule(r)
    setPrRuleName(r.name)
    setPrRuleType(r.rule_type)
    setPrPattern(r.pattern)
    setPrOperator(r.operator)
    setPrThreshold(r.threshold > 0 ? String(r.threshold) : '')
    setPrPoints(String(r.points))
    setShowPriorityRuleForm(true)
  }

  const handleSavePriorityRule = async () => {
    if (!projectId) return
    const data = {
      name: prRuleName,
      rule_type: prRuleType,
      pattern: prPattern,
      operator: prOperator,
      threshold: parseInt(prThreshold) || 0,
      points: parseInt(prPoints) || 0,
      enabled: editingPriorityRule ? editingPriorityRule.enabled : true,
    }
    try {
      if (editingPriorityRule) {
        await api.updatePriorityRule(projectId, editingPriorityRule.id, data)
      } else {
        await api.createPriorityRule(projectId, data)
      }
      setPriorityRules(await api.listPriorityRules(projectId))
      setShowPriorityRuleForm(false)
      toast.success(editingPriorityRule ? 'Rule updated' : 'Rule created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to save rule')
    }
  }

  const handleTogglePriorityRule = async (r: PriorityRule) => {
    if (!projectId) return
    await api.updatePriorityRule(projectId, r.id, {
      ...r, enabled: !r.enabled,
    })
    setPriorityRules(await api.listPriorityRules(projectId))
  }

  const handleDeletePriorityRule = async (ruleId: string) => {
    if (!projectId) return
    await api.deletePriorityRule(projectId, ruleId)
    setPriorityRules(await api.listPriorityRules(projectId))
    toast.success('Rule deleted')
  }

  const handleRecalc = async () => {
    if (!projectId) return
    setRecalcing(true)
    try {
      const result = await api.recalcPriority(projectId)
      toast.success(`Recalculated ${result.recalculated} issues`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to recalculate')
    } finally {
      setRecalcing(false)
    }
  }

  const openAIAssistant = () => {
    setAssistantMessages([])
    setAssistantInput('')
    setAssistantSuggestions([])
    setShowAIAssistant(true)
  }

  const handleAssistantSend = async () => {
    if (!projectId || !assistantInput.trim() || assistantLoading) return
    const userMsg = { role: 'user', content: assistantInput.trim() }
    const newMessages = [...assistantMessages, userMsg]
    setAssistantMessages(newMessages)
    setAssistantInput('')
    setAssistantLoading(true)
    try {
      const result = await api.suggestPriorityRules(projectId, {
        include_issues: assistantIncludeIssues,
        messages: newMessages,
      })
      setAssistantMessages([...newMessages, { role: 'assistant', content: result.message }])
      if (result.suggestions?.length) setAssistantSuggestions(result.suggestions)
    } catch (e: unknown) {
      setAssistantMessages([...newMessages, { role: 'assistant', content: `Error: ${e instanceof Error ? e.message : 'Failed to get suggestions'}` }])
    } finally {
      setAssistantLoading(false)
    }
  }

  const handleAddSuggestion = async (s: RuleSuggestion) => {
    if (!projectId) return
    try {
      await api.createPriorityRule(projectId, {
        name: s.name,
        rule_type: s.rule_type,
        pattern: s.pattern || '',
        operator: s.operator || 'gte',
        threshold: s.threshold || 0,
        points: s.points,
        enabled: true,
      })
      setPriorityRules(await api.listPriorityRules(projectId))
      setAssistantSuggestions(prev => prev.filter(x => x !== s))
      toast.success(`Rule "${s.name}" created`)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create rule')
    }
  }

  const currentRuleType = RULE_TYPES.find(t => t.value === prRuleType)

  const openAddTagRule = () => {
    setEditingTagRule(null); setTrName(''); setTrPattern(''); setTrTagKey(''); setTrTagValue('')
    setShowTagRuleForm(true)
  }
  const openEditTagRule = (r: TagRule) => {
    setEditingTagRule(r); setTrName(r.name); setTrPattern(r.pattern); setTrTagKey(r.tag_key); setTrTagValue(r.tag_value)
    setShowTagRuleForm(true)
  }
  const handleSaveTagRule = async () => {
    if (!projectId) return
    const data = { name: trName, pattern: trPattern, tag_key: trTagKey, tag_value: trTagValue, enabled: editingTagRule ? editingTagRule.enabled : true }
    try {
      if (editingTagRule) await api.updateTagRule(projectId, editingTagRule.id, data)
      else await api.createTagRule(projectId, data)
      setTagRules(await api.listTagRules(projectId))
      setShowTagRuleForm(false)
      toast.success(editingTagRule ? 'Rule updated' : 'Rule created')
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed to save rule') }
  }
  const handleToggleTagRule = async (r: TagRule) => {
    if (!projectId) return
    await api.updateTagRule(projectId, r.id, { ...r, enabled: !r.enabled })
    setTagRules(await api.listTagRules(projectId))
  }
  const handleDeleteTagRule = async (ruleId: string) => {
    if (!projectId) return
    await api.deleteTagRule(projectId, ruleId)
    setTagRules(await api.listTagRules(projectId))
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
    const cfg = a.config as { webhook_url?: string; webhook_url_set?: boolean }
    if (cfg.webhook_url_set && !cfg.webhook_url) return 'Webhook configured'
    return cfg.webhook_url || ''
  }

  const canTestJira = Boolean(
    jiraBaseUrl &&
    jiraProjectKey &&
    jiraEmail &&
    (jiraApiToken || project?.jira_api_token_set)
  )

  const canTestGithub = Boolean(
    githubOwner &&
    githubRepo &&
    (githubToken || project?.github_token_set)
  )

  const sections = [
    {
      id: 'general' as const,
      label: 'General',
      badge: 'Core',
      icon: Settings,
    },
    {
      id: 'alerts' as const,
      label: 'Alerts',
      badge: `${alerts.length}`,
      icon: Bell,
    },
    {
      id: 'tokens' as const,
      label: 'API Tokens',
      badge: `${tokens.length}`,
      icon: Key,
    },
    {
      id: 'priority' as const,
      label: 'Priority',
      badge: `${priorityRules.length}`,
      icon: Gauge,
    },
    {
      id: 'tags' as const,
      label: 'Tags',
      badge: `${tagRules.length}`,
      icon: Tag,
    },
    ...(aiProviderConfigured
      ? [
          {
            id: 'ai' as const,
            label: 'AI',
            badge: aiEnabled ? 'On' : 'Off',
            icon: Brain,
          },
        ]
      : []),
    ...(isAdmin
      ? [
          {
            id: 'integrations' as const,
            label: 'Integrations',
            badge: (project?.jira_api_token_set || project?.github_token_set) ? 'Connected' : 'Setup',
            icon: Workflow,
          },
          {
            id: 'danger' as const,
            label: 'Danger Zone',
            badge: 'Admin',
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
        <p className="max-w-3xl text-sm text-muted-foreground">Switch sections without mixing unrelated forms and actions.</p>
      </div>

      <div className="flex gap-6">
        {/* Sidebar navigation */}
        <div className="hidden md:block w-48 shrink-0">
          <nav className="space-y-1 sticky top-4">
            {sections.map(section => {
              const Icon = section.icon
              const isActive = activeSection === section.id
              return (
                <button
                  key={section.id}
                  type="button"
                  onClick={() => setActiveSection(section.id)}
                  className={cn(
                    'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors',
                    isActive
                      ? 'bg-accent text-foreground font-medium'
                      : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  <span className="flex-1 text-left">{section.label}</span>
                  {section.badge !== '0' && section.badge !== 'Core' && (
                    <span className="text-[10px] text-muted-foreground/60">{section.badge}</span>
                  )}
                </button>
              )
            })}
          </nav>
        </div>

        {/* Mobile dropdown */}
        <div className="md:hidden w-full mb-4">
          <Select
            value={activeSection}
            onChange={e => setActiveSection(e.target.value as SettingsSection)}
          >
            {sections.map(s => (
              <option key={s.id} value={s.id}>{s.label}</option>
            ))}
          </Select>
        </div>

        <div className="flex-1 min-w-0 space-y-6">
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
                  <CardTitle className="text-base">Project ID</CardTitle>
                  <CardDescription>Use this ID when calling the API with a Bearer token.</CardDescription>
                </CardHeader>
                <CardContent>
                  <code className="rounded bg-muted px-3 py-2 text-sm font-mono break-all block">{projectId}</code>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">DSN (Client Key)</CardTitle>
                  <CardDescription>Use this DSN in your Sentry SDK configuration. The numeric DSN is compatible with all SDKs including Python.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs text-muted-foreground mb-1">Numeric (recommended — works with all SDKs)</p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono break-all">
                        {project?.dsn}
                      </code>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="outline" size="icon" onClick={() => handleCopyDSN(project?.dsn)}>
                            <Copy className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Copy DSN</TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground mb-1">UUID (legacy — for existing integrations)</p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono break-all">
                        {project?.legacy_dsn}
                      </code>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="outline" size="icon" onClick={() => handleCopyDSN(project?.legacy_dsn)}>
                            <Copy className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Copy legacy DSN</TooltipContent>
                      </Tooltip>
                    </div>
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

                    <div>
                      <label className="text-sm font-medium">Max Events per Issue</label>
                      <Input
                        type="number"
                        value={maxEventsPerIssue}
                        onChange={e => setMaxEventsPerIssue(e.target.value)}
                        min="0"
                        className="mt-1 max-w-xs"
                      />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Stop recording new events for an issue after this limit. Set to 0 for unlimited.
                      </p>
                    </div>

                    <div>
                      <label className="text-sm font-medium">Issue List Display</label>
                      <Select value={issueDisplayMode} onChange={e => setIssueDisplayMode(e.target.value)} className="mt-1 max-w-xs">
                        <option value="classic">Classic (badges + full title)</option>
                        <option value="detailed">Detailed (exception + endpoint + message)</option>
                      </Select>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Default display mode for the issue list. Users can toggle per-session.
                      </p>
                    </div>

                    <div>
                      <label className="text-sm font-medium">Workflow Mode</label>
                      <Select value={workflowMode} onChange={e => setWorkflowMode(e.target.value)} className="mt-1 max-w-xs">
                        <option value="simple">Simple (monitoring only)</option>
                        <option value="managed">Managed (tickets + board)</option>
                      </Select>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Simple: issues have basic statuses. Managed: enables tickets with workflow, assignment, board view, and escalation.
                      </p>
                    </div>

                    {allGroups.length > 0 && (
                      <div>
                        <label className="text-sm font-medium">Group</label>
                        <Select value={selectedGroupId} onChange={e => setSelectedGroupId(e.target.value)} className="mt-1">
                          <option value="">No group</option>
                          {allGroups.map(g => (
                            <option key={g.id} value={g.id}>{g.name}</option>
                          ))}
                        </Select>
                        <p className="mt-1 text-xs text-muted-foreground">
                          Organize this project into a group tab on the project list.
                        </p>
                      </div>
                    )}

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
                            {a.conditions && (a.conditions as unknown as ConditionGroup).conditions?.length > 0 ? (
                              <span className="text-xs text-muted-foreground">
                                {(a.conditions as unknown as ConditionGroup).conditions.length} condition{(a.conditions as unknown as ConditionGroup).conditions.length !== 1 ? 's' : ''} ({(a.conditions as unknown as ConditionGroup).operator?.toUpperCase()})
                              </span>
                            ) : a.level_filter ? (
                              a.level_filter.split(',').map(l => (
                                <span key={l} className={cn('rounded border px-1.5 py-0.5 text-xs', LEVEL_COLORS[l])}>
                                  {l}
                                </span>
                              ))
                            ) : (
                              <span className="text-xs text-muted-foreground">All levels</span>
                            )}
                            {!a.conditions && a.title_pattern && (
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
                  {' '}<a href="https://github.com/darkspock/GoSnag/blob/main/API.md" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">API documentation &rarr;</a>
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

          {activeSection === 'priority' && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Priority</p>
                <h2 className="text-xl font-semibold">Priority scoring rules</h2>
                <p className="text-sm text-muted-foreground">
                  Define rules that add or subtract points to calculate an issue's priority score (0–100). Base score is 50.
                </p>
              </div>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <CardTitle className="text-base">Rules</CardTitle>
                  <div className="flex gap-2">
                    {isAdmin && (
                      <Button size="sm" variant="outline" onClick={handleRecalc} disabled={recalcing}>
                        {recalcing ? 'Recalculating...' : 'Recalculate All'}
                      </Button>
                    )}
                    {isAdmin && aiEnabled && aiProviderConfigured && (
                      <Button size="sm" variant="outline" onClick={openAIAssistant}>
                        <Sparkles className="h-4 w-4 mr-1" /> AI Assistant
                      </Button>
                    )}
                    {isAdmin && (
                      <Button size="sm" variant="outline" onClick={openAddPriorityRule}>
                        <Plus className="h-4 w-4 mr-1" /> Add Rule
                      </Button>
                    )}
                  </div>
                </CardHeader>
                <CardContent>
                  {priorityRules.length === 0 ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No priority rules configured.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">All issues will have the default priority of 50.</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {priorityRules.map(r => (
                        <div key={r.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{r.name}</span>
                              <button
                                onClick={() => handleTogglePriorityRule(r)}
                                className={cn(
                                  'text-xs px-1.5 py-0.5 rounded cursor-pointer transition-colors',
                                  r.enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-muted text-muted-foreground'
                                )}
                              >
                                {r.enabled ? 'Active' : 'Disabled'}
                              </button>
                              <span className={cn(
                                'text-xs font-mono px-1.5 py-0.5 rounded',
                                r.rule_type === 'ai_prompt' ? 'bg-purple-500/15 text-purple-400' :
                                r.points > 0 ? 'bg-red-500/15 text-red-400' : 'bg-blue-500/15 text-blue-400'
                              )}>
                                {r.rule_type === 'ai_prompt' ? `\u00B1${r.points}` : `${r.points > 0 ? '+' : ''}${r.points}`}
                              </span>
                            </div>
                            {isAdmin && (
                              <div className="flex items-center gap-1">
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditPriorityRule(r)}>
                                      <Pencil className="h-3.5 w-3.5" />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent>Edit rule</TooltipContent>
                                </Tooltip>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeletePriorityRule(r.id)}>
                                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent>Delete rule</TooltipContent>
                                </Tooltip>
                              </div>
                            )}
                          </div>
                          <p className="mt-1 text-xs text-muted-foreground">
                            {r.rule_type === 'ai_prompt' ? (
                              <>
                                <span className="inline-flex items-center gap-1 rounded bg-purple-500/15 text-purple-400 px-1.5 py-0.5 mr-1">
                                  <Sparkles className="h-3 w-3" /> AI
                                </span>
                                {r.pattern.length > 60 ? r.pattern.slice(0, 60) + '...' : r.pattern}
                                {r.threshold > 0 && <span className="ml-2 text-muted-foreground/60">at {r.threshold} events</span>}
                              </>
                            ) : (
                              <>
                                {RULE_TYPES.find(t => t.value === r.rule_type)?.label || r.rule_type}
                                {r.pattern ? `: ${r.pattern}` : ''}
                                {r.threshold > 0 ? ` ${r.operator || '≥'} ${r.threshold}` : ''}
                              </>
                            )}
                          </p>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Priority Rule Form Dialog */}
              <Dialog open={showPriorityRuleForm} onOpenChange={setShowPriorityRuleForm}>
                <DialogContent>
                  <DialogTitle>{editingPriorityRule ? 'Edit Rule' : 'Add Rule'}</DialogTitle>
                  <DialogDescription className="sr-only">Configure priority scoring rule</DialogDescription>
                  <div className="mt-4 space-y-4">
                    <div>
                      <label className="text-sm font-medium">Name</label>
                      <Input value={prRuleName} onChange={e => setPrRuleName(e.target.value)} placeholder="e.g. Fatal errors" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Type</label>
                      <Select value={prRuleType} onChange={e => setPrRuleType(e.target.value)} className="mt-1">
                        {RULE_TYPES.map(t => (
                          <option key={t.value} value={t.value}>{t.label}</option>
                        ))}
                      </Select>
                    </div>
                    {currentRuleType?.needsPattern && (
                      <div>
                        <label className="text-sm font-medium">
                          {prRuleType === 'level_is' ? 'Level' : prRuleType === 'platform_is' ? 'Platform' : 'Pattern'}
                        </label>
                        {prRuleType === 'level_is' ? (
                          <Select value={prPattern} onChange={e => setPrPattern(e.target.value)} className="mt-1">
                            <option value="fatal">fatal</option>
                            <option value="error">error</option>
                            <option value="warning">warning</option>
                            <option value="info">info</option>
                            <option value="debug">debug</option>
                          </Select>
                        ) : (
                          <Input value={prPattern} onChange={e => setPrPattern(e.target.value)} placeholder={prRuleType.includes('title') ? 'e.g. database|timeout' : 'e.g. php'} className="mt-1" />
                        )}
                      </div>
                    )}
                    {currentRuleType?.needsPrompt && (
                      <>
                        <div>
                          <label className="text-sm font-medium">AI Prompt</label>
                          <textarea
                            value={prPattern}
                            onChange={e => setPrPattern(e.target.value)}
                            placeholder="e.g. Assign high points if this error affects payment, checkout, or authentication flows"
                            rows={4}
                            className="mt-1 w-full rounded-md border bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                          />
                          <p className="mt-1 text-xs text-muted-foreground">Describe what the AI should evaluate. It will receive the issue title, level, platform, and latest event data.</p>
                        </div>
                        <div>
                          <label className="text-sm font-medium">Event Threshold</label>
                          <Input type="number" min={1} value={prThreshold} onChange={e => setPrThreshold(e.target.value)} placeholder="e.g. 1" className="mt-1" />
                          <p className="mt-1 text-xs text-muted-foreground">AI evaluates when the issue reaches this many events. Use 1 for first-event triage.</p>
                        </div>
                      </>
                    )}
                    {currentRuleType?.needsThreshold && (
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="text-sm font-medium">Operator</label>
                          <Select value={prOperator} onChange={e => setPrOperator(e.target.value)} className="mt-1">
                            <option value="gte">≥ greater or equal</option>
                            <option value="gt">&gt; greater than</option>
                            <option value="lte">≤ less or equal</option>
                            <option value="lt">&lt; less than</option>
                            <option value="eq">= equals</option>
                          </Select>
                        </div>
                        <div>
                          <label className="text-sm font-medium">Threshold</label>
                          <Input type="number" value={prThreshold} onChange={e => setPrThreshold(e.target.value)} placeholder="e.g. 10" className="mt-1" />
                        </div>
                      </div>
                    )}
                    <div>
                      <label className="text-sm font-medium">Points</label>
                      <Input type="number" value={prPoints} onChange={e => setPrPoints(e.target.value)} placeholder={currentRuleType?.needsPrompt ? 'e.g. 25' : 'e.g. 20 or -10'} className="mt-1" />
                      <p className="mt-1 text-xs text-muted-foreground">
                        {currentRuleType?.needsPrompt
                          ? 'Maximum points the AI can assign. AI returns between -points and +points.'
                          : 'Positive = higher priority, negative = lower. Base score is 50, clamped to 0\u2013100.'}
                      </p>
                    </div>
                    <div className="flex justify-end gap-2">
                      <Button variant="outline" onClick={() => setShowPriorityRuleForm(false)}>Cancel</Button>
                      <Button onClick={handleSavePriorityRule} disabled={!prRuleName.trim() || !prPoints}>{editingPriorityRule ? 'Save' : 'Add'}</Button>
                    </div>
                  </div>
                </DialogContent>
              </Dialog>

              {/* Delete Priority Rule Confirm */}
              <ConfirmDialog
                open={!!showDeletePriorityRule}
                onOpenChange={open => { if (!open) setShowDeletePriorityRule(null) }}
                title="Delete Rule"
                description="This priority rule will be permanently deleted."
                confirmLabel="Delete"
                variant="destructive"
                onConfirm={() => { if (showDeletePriorityRule) handleDeletePriorityRule(showDeletePriorityRule) }}
              />

              {/* AI Rule Assistant Dialog */}
              <Dialog open={showAIAssistant} onOpenChange={setShowAIAssistant}>
                <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
                  <DialogTitle className="flex items-center gap-2">
                    <Sparkles className="h-5 w-5 text-purple-400" /> AI Rule Assistant
                  </DialogTitle>
                  <DialogDescription className="sr-only">AI-powered assistant for creating priority rules</DialogDescription>

                  {/* Include issues checkbox */}
                  <label className="flex items-center gap-2 text-sm text-muted-foreground">
                    <input
                      type="checkbox"
                      checked={assistantIncludeIssues}
                      onChange={e => setAssistantIncludeIssues(e.target.checked)}
                      className="rounded border-border"
                    />
                    Include recent issues as context
                  </label>

                  {/* Chat messages */}
                  <div className="flex-1 overflow-y-auto space-y-3 min-h-[200px] max-h-[400px] rounded-md border p-3">
                    {assistantMessages.length === 0 && (
                      <p className="text-sm text-muted-foreground text-center py-8">
                        Describe the kind of priority rules you want. For example: "I want to prioritize database errors and payment failures."
                      </p>
                    )}
                    {assistantMessages.map((m, i) => (
                      <div key={i} className={cn('flex', m.role === 'user' ? 'justify-end' : 'justify-start')}>
                        <div className={cn(
                          'rounded-lg px-3 py-2 text-sm max-w-[80%]',
                          m.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-muted'
                        )}>
                          {m.content}
                        </div>
                      </div>
                    ))}
                    {assistantLoading && (
                      <div className="flex justify-start">
                        <div className="bg-muted rounded-lg px-3 py-2 text-sm">
                          <Loader2 className="h-4 w-4 animate-spin" />
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Suggestions */}
                  {assistantSuggestions.length > 0 && (
                    <div className="space-y-2">
                      <p className="text-sm font-medium">Suggested Rules</p>
                      {assistantSuggestions.map((s, i) => (
                        <div key={i} className="rounded-md border p-3 space-y-1">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{s.name}</span>
                              <span className={cn(
                                'text-xs px-1.5 py-0.5 rounded',
                                s.rule_type === 'ai_prompt' ? 'bg-purple-500/15 text-purple-400' : 'bg-muted text-muted-foreground'
                              )}>
                                {RULE_TYPES.find(t => t.value === s.rule_type)?.label || s.rule_type}
                              </span>
                              <span className="text-xs font-mono text-muted-foreground">
                                {s.rule_type === 'ai_prompt' ? `\u00B1${s.points}` : `${s.points > 0 ? '+' : ''}${s.points}`} pts
                              </span>
                            </div>
                            <div className="flex items-center gap-1">
                              <Button size="sm" variant="outline" onClick={() => handleAddSuggestion(s)}>
                                <Check className="h-3.5 w-3.5 mr-1" /> Add
                              </Button>
                              <Button size="sm" variant="ghost" onClick={() => setAssistantSuggestions(prev => prev.filter(x => x !== s))}>
                                <X className="h-3.5 w-3.5" />
                              </Button>
                            </div>
                          </div>
                          <p className="text-xs text-muted-foreground">{s.explanation}</p>
                          {s.pattern && (
                            <p className="text-xs font-mono text-muted-foreground/60 truncate">
                              {s.rule_type === 'ai_prompt' ? 'Prompt: ' : 'Pattern: '}{s.pattern}
                            </p>
                          )}
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Input */}
                  <div className="flex gap-2">
                    <Input
                      value={assistantInput}
                      onChange={e => setAssistantInput(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleAssistantSend() } }}
                      placeholder="Describe what you want to prioritize..."
                      disabled={assistantLoading}
                      className="flex-1"
                    />
                    <Button onClick={handleAssistantSend} disabled={assistantLoading || !assistantInput.trim()} size="icon">
                      <Send className="h-4 w-4" />
                    </Button>
                  </div>
                </DialogContent>
              </Dialog>
            </>
          )}

          {activeSection === 'tags' && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Tags</p>
                <h2 className="text-xl font-semibold">Auto-tagging rules</h2>
                <p className="text-sm text-muted-foreground">
                  Define rules to automatically tag issues based on their title. Search issues by tag using <code className="text-xs">key:value</code> in the search box.
                </p>
              </div>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <CardTitle className="text-base">Rules</CardTitle>
                  {isAdmin && (
                    <Button size="sm" variant="outline" onClick={openAddTagRule}>
                      <Plus className="h-4 w-4 mr-1" /> Add Rule
                    </Button>
                  )}
                </CardHeader>
                <CardContent>
                  {tagRules.length === 0 ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No tag rules configured.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">Tags can still be added manually on each issue.</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {tagRules.map(r => (
                        <div key={r.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{r.name}</span>
                              <button onClick={() => handleToggleTagRule(r)} className={cn('text-xs px-1.5 py-0.5 rounded cursor-pointer transition-colors', r.enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-muted text-muted-foreground')}>
                                {r.enabled ? 'Active' : 'Disabled'}
                              </button>
                              <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary/80">
                                {r.tag_key}:{r.tag_value}
                              </span>
                            </div>
                            {isAdmin && (
                              <div className="flex items-center gap-1">
                                <Tooltip><TooltipTrigger asChild><Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditTagRule(r)}><Pencil className="h-3.5 w-3.5" /></Button></TooltipTrigger><TooltipContent>Edit rule</TooltipContent></Tooltip>
                                <Tooltip><TooltipTrigger asChild><Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeleteTagRule(r.id)}><Trash2 className="h-3.5 w-3.5 text-destructive" /></Button></TooltipTrigger><TooltipContent>Delete rule</TooltipContent></Tooltip>
                              </div>
                            )}
                          </div>
                          <p className="mt-1 text-xs text-muted-foreground font-mono">pattern: {r.pattern}</p>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Dialog open={showTagRuleForm} onOpenChange={setShowTagRuleForm}>
                <DialogContent>
                  <DialogTitle>{editingTagRule ? 'Edit Rule' : 'Add Rule'}</DialogTitle>
                  <DialogDescription className="sr-only">Configure auto-tagging rule</DialogDescription>
                  <div className="mt-4 space-y-4">
                    <div>
                      <label className="text-sm font-medium">Name</label>
                      <Input value={trName} onChange={e => setTrName(e.target.value)} placeholder="e.g. Payment errors" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Title pattern</label>
                      <Input value={trPattern} onChange={e => setTrPattern(e.target.value)} placeholder="e.g. Adyen|Stripe|payment" className="mt-1" />
                      <p className="mt-1 text-xs text-muted-foreground">Plain text = contains match. Supports regex.</p>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm font-medium">Tag key</label>
                        <Input value={trTagKey} onChange={e => setTrTagKey(e.target.value)} placeholder="e.g. team" className="mt-1" />
                      </div>
                      <div>
                        <label className="text-sm font-medium">Tag value</label>
                        <Input value={trTagValue} onChange={e => setTrTagValue(e.target.value)} placeholder="e.g. payment" className="mt-1" />
                      </div>
                    </div>
                    <div className="flex justify-end gap-2">
                      <Button variant="outline" onClick={() => setShowTagRuleForm(false)}>Cancel</Button>
                      <Button onClick={handleSaveTagRule} disabled={!trName || !trPattern || !trTagKey || !trTagValue}>{editingTagRule ? 'Save' : 'Add'}</Button>
                    </div>
                  </div>
                </DialogContent>
              </Dialog>

              <ConfirmDialog
                open={!!showDeleteTagRule}
                onOpenChange={open => { if (!open) setShowDeleteTagRule(null) }}
                title="Delete Rule"
                description="This tag rule will be permanently deleted."
                confirmLabel="Delete"
                variant="destructive"
                onConfirm={() => { if (showDeleteTagRule) handleDeleteTagRule(showDeleteTagRule) }}
              />
            </>
          )}

          {activeSection === 'ai' && aiProviderConfigured && (
            <>
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">AI Integration</p>
                <h2 className="text-xl font-semibold">AI Features</h2>
                <p className="text-sm text-muted-foreground">
                  Configure AI-powered features for this project. Provider: <span className="font-medium text-foreground">{aiProviderName || 'Unknown'}</span>
                </p>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Master Toggle</CardTitle>
                  <CardDescription>Enable or disable all AI features for this project.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={aiEnabled}
                      onChange={e => setAiEnabled(e.target.checked)}
                      className="h-4 w-4 rounded border-border accent-primary"
                    />
                    <span className="text-sm font-medium">Enable AI for this project</span>
                  </label>
                </CardContent>
              </Card>

              {aiEnabled && (
                <>
                  <Card>
                    <CardHeader>
                      <CardTitle className="text-base">Feature Toggles</CardTitle>
                      <CardDescription>Choose which AI features are active.</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <label className="flex items-center gap-3 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={aiTicketDescription}
                          onChange={e => setAiTicketDescription(e.target.checked)}
                          className="h-4 w-4 rounded border-border accent-primary"
                        />
                        <div>
                          <span className="text-sm font-medium">Ticket Description Generation</span>
                          <p className="text-xs text-muted-foreground">Generate structured descriptions when creating tickets from issues.</p>
                        </div>
                      </label>
                      <label className="flex items-center gap-3 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={aiMergeSuggestions}
                          onChange={e => setAiMergeSuggestions(e.target.checked)}
                          className="h-4 w-4 rounded border-border accent-primary"
                        />
                        <div>
                          <span className="text-sm font-medium">Merge Suggestions</span>
                          <p className="text-xs text-muted-foreground">Suggest duplicate issues that could be merged (runs every 5 minutes).</p>
                        </div>
                      </label>
                      <label className="flex items-center gap-3 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={aiRootCause}
                          onChange={e => setAiRootCause(e.target.checked)}
                          className="h-4 w-4 rounded border-border accent-primary"
                        />
                        <div>
                          <span className="text-sm font-medium">Root Cause Analysis</span>
                          <p className="text-xs text-muted-foreground">AI-generated root cause analysis for issues (Phase 2).</p>
                        </div>
                      </label>
                    </CardContent>
                  </Card>

                  {aiUsage && (
                    <Card>
                      <CardHeader>
                        <CardTitle className="text-base">Token Usage</CardTitle>
                        <CardDescription>AI token consumption for this project.</CardDescription>
                      </CardHeader>
                      <CardContent>
                        <div className="grid grid-cols-2 gap-4">
                          <div className="space-y-1">
                            <p className="text-xs text-muted-foreground">Today</p>
                            <p className="text-lg font-semibold">{aiUsage.today_tokens.toLocaleString()}</p>
                            <p className="text-xs text-muted-foreground">{aiUsage.today_calls} calls</p>
                          </div>
                          <div className="space-y-1">
                            <p className="text-xs text-muted-foreground">This Week</p>
                            <p className="text-lg font-semibold">{aiUsage.week_tokens.toLocaleString()}</p>
                            <p className="text-xs text-muted-foreground">{aiUsage.week_calls} calls</p>
                          </div>
                        </div>
                        {aiUsage.daily_budget > 0 && (
                          <div className="mt-4">
                            <div className="flex justify-between text-xs text-muted-foreground mb-1">
                              <span>Daily budget</span>
                              <span>{aiUsage.today_tokens.toLocaleString()} / {aiUsage.daily_budget.toLocaleString()}</span>
                            </div>
                            <div className="h-2 rounded-full bg-muted overflow-hidden">
                              <div
                                className={cn(
                                  'h-full rounded-full transition-all',
                                  aiUsage.today_tokens / aiUsage.daily_budget > 0.9 ? 'bg-red-500' :
                                  aiUsage.today_tokens / aiUsage.daily_budget > 0.7 ? 'bg-amber-500' : 'bg-primary'
                                )}
                                style={{ width: `${Math.min(100, (aiUsage.today_tokens / aiUsage.daily_budget) * 100)}%` }}
                              />
                            </div>
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  )}
                </>
              )}

              <div className="flex justify-end">
                <Button onClick={handleSaveAI} disabled={savingAI}>
                  {savingAI ? 'Saving...' : 'Save AI Settings'}
                </Button>
              </div>
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

              <div className="space-y-1 mt-8">
                <h2 className="text-xl font-semibold">Source Code Repository</h2>
                <p className="text-sm text-muted-foreground">
                  Link stack trace frames directly to your source code. Enables clickable links and suspect commit detection.
                </p>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Repository</CardTitle>
                  <CardDescription>Connect your source code repository for stack trace linking.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-5">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Provider</label>
                      <Select value={repoProvider} onChange={e => setRepoProvider(e.target.value)} className="mt-1">
                        <option value="">Not configured</option>
                        <option value="github">GitHub</option>
                        <option value="bitbucket">Bitbucket</option>
                      </Select>
                    </div>
                    <div>
                      <label className="text-sm font-medium">Default Branch</label>
                      <Input value={repoDefaultBranch} onChange={e => setRepoDefaultBranch(e.target.value)} placeholder="main" className="mt-1" />
                    </div>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">{repoProvider === 'bitbucket' ? 'Workspace' : 'Owner'}</label>
                      <Input value={repoOwner} onChange={e => setRepoOwner(e.target.value)} placeholder="e.g. myorg" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Repository Name</label>
                      <Input value={repoName} onChange={e => setRepoName(e.target.value)} placeholder="e.g. my-app" className="mt-1" />
                    </div>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Access Token</label>
                      <Input
                        type="password"
                        value={repoToken}
                        onChange={e => setRepoToken(e.target.value)}
                        placeholder={project?.repo_token_set ? '••••••••• (configured)' : 'Token with read access'}
                        className="mt-1"
                      />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Needs read-only access to repository contents.
                      </p>
                    </div>
                    <div>
                      <label className="text-sm font-medium">Path Strip Prefix</label>
                      <Input value={repoPathStrip} onChange={e => setRepoPathStrip(e.target.value)} placeholder="/var/www/app/" className="mt-1" />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Prefix to remove from runtime file paths to match repo paths.
                      </p>
                    </div>
                  </div>

                  <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs text-muted-foreground">
                      {project?.repo_token_set ? 'Repository token is configured.' : 'No repository configured yet.'}
                    </p>
                    <div className="flex flex-col-reverse gap-2 sm:flex-row">
                      <Button variant="outline" onClick={handleTestRepo} disabled={repoTesting || !repoProvider || !repoOwner || !repoName}>
                        {repoTesting ? 'Testing...' : 'Test Connection'}
                      </Button>
                      <Button onClick={handleSaveRepo} disabled={savingRepo}>
                        {savingRepo ? 'Saving...' : 'Save Repository Settings'}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <div className="space-y-1 mt-8">
                <h2 className="text-xl font-semibold">GitHub Issues</h2>
                <p className="text-sm text-muted-foreground">
                  Create GitHub issues from GoSnag issues, manually or automatically.
                </p>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">GitHub Connection</CardTitle>
                  <CardDescription>Repository and authentication details for GitHub issue creation.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-5">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Repository Owner</label>
                      <Input value={githubOwner} onChange={e => setGithubOwner(e.target.value)} placeholder="e.g. myorg" className="mt-1" />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Repository Name</label>
                      <Input value={githubRepo} onChange={e => setGithubRepo(e.target.value)} placeholder="e.g. my-app" className="mt-1" />
                    </div>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium">Personal Access Token</label>
                      <Input
                        type="password"
                        value={githubToken}
                        onChange={e => setGithubToken(e.target.value)}
                        placeholder={project?.github_token_set ? '••••••••• (configured)' : 'ghp_... or fine-grained token'}
                        className="mt-1"
                      />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Needs <code>issues: write</code> permission. Leave blank to keep the stored token.
                      </p>
                    </div>
                    <div>
                      <label className="text-sm font-medium">Labels</label>
                      <Input value={githubLabels} onChange={e => setGithubLabels(e.target.value)} placeholder="bug, gosnag" className="mt-1" />
                      <p className="mt-1 text-xs text-muted-foreground">Comma-separated labels to apply to created issues.</p>
                    </div>
                  </div>

                  <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs text-muted-foreground">
                      {project?.github_token_set ? 'A GitHub token is already stored for this project.' : 'No GitHub token stored yet.'}
                    </p>
                    <div className="flex flex-col-reverse gap-2 sm:flex-row">
                      <Button variant="outline" onClick={handleTestGithub} disabled={githubTesting || !canTestGithub}>
                        {githubTesting ? 'Testing...' : 'Test Connection'}
                      </Button>
                      <Button onClick={handleSaveGithub} disabled={savingGithub}>
                        {savingGithub ? 'Saving...' : 'Save GitHub Settings'}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-base">GitHub Auto-Creation Rules</CardTitle>
                    <CardDescription>Rules determine when GoSnag should create GitHub issues automatically.</CardDescription>
                  </div>
                  <Button size="sm" variant="outline" onClick={openAddGithubRule} disabled={!githubOwner}>
                    <Plus className="mr-1 h-4 w-4" /> Add Rule
                  </Button>
                </CardHeader>
                <CardContent>
                  {!githubOwner ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">Configure GitHub first.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">
                        Rules become available once the repository owner is set.
                      </p>
                    </div>
                  ) : githubRules.length === 0 ? (
                    <div className="py-8 text-center text-muted-foreground">
                      <p className="text-sm">No auto-creation rules yet.</p>
                      <p className="mt-1 text-xs text-muted-foreground/60">
                        Add a rule to automatically create GitHub issues when issues match conditions.
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {githubRules.map(r => (
                        <div key={r.id} className="rounded-md border p-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">{r.name}</span>
                              <button
                                onClick={() => handleToggleGithubRule(r)}
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
                                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditGithubRule(r)}>
                                    <Pencil className="h-3.5 w-3.5" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Edit rule</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowDeleteGithubRule(r.id)}>
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
              <div className="mt-2">
                <p className="text-xs font-medium text-muted-foreground mb-1.5">Example usage:</p>
                <pre className="bg-muted px-3 py-2 rounded text-xs font-mono break-all whitespace-pre-wrap text-muted-foreground">
{`curl -H "Authorization: Bearer ${newToken}" \\
  "${window.location.origin}/api/v1/projects/${projectId}/issues?limit=5"`}
                </pre>
              </div>
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

      {/* Delete Jira Rule Confirm */}
      <ConfirmDialog
        open={!!showDeleteRule}
        onOpenChange={open => { if (!open) setShowDeleteRule(null) }}
        title="Delete Rule"
        description="This auto-creation rule will be permanently deleted."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (showDeleteRule) handleDeleteRule(showDeleteRule) }}
      />

      {/* GitHub Rule Form Dialog */}
      <Dialog open={showGithubRuleForm} onOpenChange={setShowGithubRuleForm}>
        <DialogContent>
          <DialogTitle>{editingGithubRule ? 'Edit Rule' : 'Add Rule'}</DialogTitle>
          <DialogDescription className="sr-only">Configure GitHub auto-creation rule</DialogDescription>
          <div className="mt-4 space-y-4">
            <div>
              <label className="text-sm font-medium">Name</label>
              <Input value={ghRuleName} onChange={e => setGhRuleName(e.target.value)} placeholder="e.g. Critical errors" className="mt-1" />
            </div>
            <div>
              <label className="text-sm font-medium">Level filter</label>
              <p className="text-xs text-muted-foreground mb-1">Comma-separated. Empty = all levels.</p>
              <Input value={ghRuleLevelFilter} onChange={e => setGhRuleLevelFilter(e.target.value)} placeholder="e.g. fatal,error" className="mt-1" />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium">Min events</label>
                <Input type="number" value={ghRuleMinEvents} onChange={e => setGhRuleMinEvents(e.target.value)} placeholder="0" className="mt-1" />
              </div>
              <div>
                <label className="text-sm font-medium">Min users</label>
                <Input type="number" value={ghRuleMinUsers} onChange={e => setGhRuleMinUsers(e.target.value)} placeholder="0" className="mt-1" />
              </div>
            </div>
            <div>
              <label className="text-sm font-medium">Title pattern</label>
              <p className="text-xs text-muted-foreground mb-1">Regex or plain text. Empty = match all.</p>
              <Input value={ghRuleTitlePattern} onChange={e => setGhRuleTitlePattern(e.target.value)} placeholder="e.g. database|timeout" className="mt-1" />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowGithubRuleForm(false)}>Cancel</Button>
              <Button onClick={handleSaveGithubRule} disabled={!ghRuleName.trim()}>{editingGithubRule ? 'Save' : 'Add'}</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete GitHub Rule Confirm */}
      <ConfirmDialog
        open={!!showDeleteGithubRule}
        onOpenChange={open => { if (!open) setShowDeleteGithubRule(null) }}
        title="Delete Rule"
        description="This auto-creation rule will be permanently deleted."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (showDeleteGithubRule) handleDeleteGithubRule(showDeleteGithubRule) }}
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
        <DialogContent className="max-w-xl">
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
              <label className="text-sm font-medium">Conditions</label>
              <p className="text-xs text-muted-foreground mb-2">Define when this alert triggers. No conditions = always triggers on new/reopened issues.</p>
              <ConditionBuilder
                value={alertConditions}
                onChange={setAlertConditions}
                availableTypes={['level', 'platform', 'title', 'total_events', 'velocity_1h', 'velocity_24h', 'user_count']}
              />
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
