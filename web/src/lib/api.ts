const BASE = '/api/v1'

export class ApiError extends Error {
  status: number
  body: Record<string, unknown>
  _isApiError = true
  constructor(status: number, body: Record<string, unknown>) {
    super(body.error as string || body.message as string || `Request failed: ${status}`)
    this.status = status
    this.body = body
  }
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError || (typeof e === 'object' && e !== null && '_isApiError' in e)
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })
  if (res.status === 401) {
    if (window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// Auth
export const api = {
  me: () => request<User>('/me'),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  getAuthConfig: () => request<{ auth_mode: string; google_client_id: string }>('/auth/config'),
  googleLogin: (credential: string) =>
    request<User>('/auth/google/token', { method: 'POST', body: JSON.stringify({ credential }) }),
  localLogin: (email: string) =>
    request<User>('/auth/local/login', { method: 'POST', body: JSON.stringify({ email }) }),

  // Groups
  listGroups: () => request<ProjectGroup[]>('/groups'),
  createGroup: (name: string) => request<ProjectGroup>('/groups', { method: 'POST', body: JSON.stringify({ name }) }),
  updateGroup: (id: string, name: string) => request<ProjectGroup>(`/groups/${id}`, { method: 'PUT', body: JSON.stringify({ name }) }),
  deleteGroup: (id: string) => request<void>(`/groups/${id}`, { method: 'DELETE' }),

  // Favorites
  listFavorites: () => request<string[]>('/favorites'),
  addFavorite: (projectId: string) => request<void>(`/projects/${projectId}/favorite`, { method: 'PUT' }),
  removeFavorite: (projectId: string) => request<void>(`/projects/${projectId}/favorite`, { method: 'DELETE' }),

  // Projects
  listProjects: () => request<Project[]>('/projects'),
  getProject: (id: string) => request<ProjectWithDSN>(`/projects/${id}`),
  createProject: (data: { name: string; slug?: string; default_cooldown_minutes?: number }) =>
    request<ProjectWithDSN>('/projects', { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (id: string, data: Record<string, unknown>) =>
    request<Project>(`/projects/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  reorderProjects: (items: { id: string; position: number }[]) =>
    request<void>('/projects/reorder', { method: 'PUT', body: JSON.stringify(items) }),
  deleteProject: (id: string) =>
    request<void>(`/projects/${id}`, { method: 'DELETE' }),

  // Issues
  listIssues: (projectId: string, params?: { status?: string; level?: string; limit?: number; offset?: number; today?: boolean; assigned_to?: string; assigned_any?: boolean; search?: string; tag?: string; release?: string }) => {
    const q = new URLSearchParams()
    if (params?.status) q.set('status', params.status)
    if (params?.level) q.set('level', params.level)
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    if (params?.today) q.set('today', 'true')
    if (params?.assigned_to) q.set('assigned_to', params.assigned_to)
    if (params?.assigned_any) q.set('assigned_any', 'true')
    if (params?.search) q.set('search', params.search)
    if (params?.tag) q.set('tag', params.tag)
    if (params?.release) q.set('release', params.release)
    return request<IssueListResponse>(`/projects/${projectId}/issues?${q}`)
  },
  listIssueReleases: (projectId: string) =>
    request<string[]>(`/projects/${projectId}/issues/releases`),
  getIssueCounts: (projectId: string, params?: { level?: string }) => {
    const q = new URLSearchParams()
    if (params?.level) q.set('level', params.level)
    const qs = q.toString()
    return request<IssueCounts>(`/projects/${projectId}/issues/counts${qs ? '?' + qs : ''}`)
  },
  deleteIssues: (projectId: string, ids: string[]) =>
    request<{ deleted: number }>(`/projects/${projectId}/issues`, { method: 'DELETE', body: JSON.stringify({ ids }) }),
  mergeIssues: (projectId: string, primaryId: string, issueIds: string[]) =>
    request<Issue>(`/projects/${projectId}/issues/merge`, { method: 'POST', body: JSON.stringify({ primary_id: primaryId, issue_ids: issueIds }) }),
  getIssue: (projectId: string, issueId: string) =>
    request<Issue>(`/projects/${projectId}/issues/${issueId}`),
  updateIssueStatus: (projectId: string, issueId: string, data: { status: string; cooldown_minutes?: number; resolved_in_release?: string; snooze_minutes?: number; snooze_event_threshold?: number }) =>
    request<Issue>(`/projects/${projectId}/issues/${issueId}`, { method: 'PUT', body: JSON.stringify(data) }),
  assignIssue: (projectId: string, issueId: string, userId: string | null) =>
    request<Issue>(`/projects/${projectId}/issues/${issueId}/assign`, { method: 'PUT', body: JSON.stringify({ assigned_to: userId }) }),
  followIssue: (projectId: string, issueId: string) =>
    request<{ followed: boolean }>(`/projects/${projectId}/issues/${issueId}/follow`, { method: 'POST' }),
  unfollowIssue: (projectId: string, issueId: string) =>
    request<{ followed: boolean }>(`/projects/${projectId}/issues/${issueId}/follow`, { method: 'DELETE' }),

  // Comments
  listComments: (projectId: string, issueId: string) =>
    request<IssueComment[]>(`/projects/${projectId}/issues/${issueId}/comments`),
  createComment: (projectId: string, issueId: string, body: string) =>
    request<IssueComment>(`/projects/${projectId}/issues/${issueId}/comments`, { method: 'POST', body: JSON.stringify({ body }) }),
  updateComment: (projectId: string, issueId: string, commentId: string, body: string) =>
    request<IssueComment>(`/projects/${projectId}/issues/${issueId}/comments/${commentId}`, { method: 'PUT', body: JSON.stringify({ body }) }),
  deleteComment: (projectId: string, issueId: string, commentId: string) =>
    request<void>(`/projects/${projectId}/issues/${issueId}/comments/${commentId}`, { method: 'DELETE' }),

  listEvents: (projectId: string, issueId: string, params?: { limit?: number; offset?: number }) => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    return request<EventListResponse>(`/projects/${projectId}/issues/${issueId}/events?${q}`)
  },

  // Users
  listUsers: () => request<User[]>('/users'),
  inviteUser: (email: string, role: string) =>
    request<User>('/users/invite', { method: 'POST', body: JSON.stringify({ email, role }) }),
  updateUserRole: (userId: string, role: string) =>
    request<User>(`/users/${userId}`, { method: 'PUT', body: JSON.stringify({ role }) }),
  updateUserStatus: (userId: string, status: string) =>
    request<User>(`/users/${userId}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),

  // API Tokens
  listTokens: (projectId: string) => request<APIToken[]>(`/projects/${projectId}/tokens`),
  createToken: (projectId: string, data: { name: string; permission: string; expires_in?: number }) =>
    request<APIToken & { token: string }>(`/projects/${projectId}/tokens`, { method: 'POST', body: JSON.stringify(data) }),
  deleteToken: (projectId: string, tokenId: string) =>
    request<void>(`/projects/${projectId}/tokens/${tokenId}`, { method: 'DELETE' }),

  // Priority Rules
  listPriorityRules: (projectId: string) => request<PriorityRule[]>(`/projects/${projectId}/priority-rules`),
  createPriorityRule: (projectId: string, data: PriorityRuleData) =>
    request<PriorityRule>(`/projects/${projectId}/priority-rules`, { method: 'POST', body: JSON.stringify(data) }),
  updatePriorityRule: (projectId: string, ruleId: string, data: PriorityRuleData) =>
    request<PriorityRule>(`/projects/${projectId}/priority-rules/${ruleId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePriorityRule: (projectId: string, ruleId: string) =>
    request<void>(`/projects/${projectId}/priority-rules/${ruleId}`, { method: 'DELETE' }),
  recalcPriority: (projectId: string) =>
    request<{ recalculated: number }>(`/projects/${projectId}/priority-rules/recalc`, { method: 'POST' }),

  // Tags
  listIssueTags: (projectId: string, issueId: string) => request<IssueTag[]>(`/projects/${projectId}/issues/${issueId}/tags`),
  addIssueTag: (projectId: string, issueId: string, key: string, value: string) =>
    request<void>(`/projects/${projectId}/issues/${issueId}/tags`, { method: 'POST', body: JSON.stringify({ key, value }) }),
  removeIssueTag: (projectId: string, issueId: string, key: string, value: string) =>
    request<void>(`/projects/${projectId}/issues/${issueId}/tags`, { method: 'DELETE', body: JSON.stringify({ key, value }) }),
  listDistinctTags: (projectId: string) => request<{ key: string; value: string }[]>(`/projects/${projectId}/tags`),
  listTagRules: (projectId: string) => request<TagRule[]>(`/projects/${projectId}/tag-rules`),
  createTagRule: (projectId: string, data: { name: string; pattern: string; tag_key: string; tag_value: string; enabled: boolean }) =>
    request<TagRule>(`/projects/${projectId}/tag-rules`, { method: 'POST', body: JSON.stringify(data) }),
  updateTagRule: (projectId: string, ruleId: string, data: { name: string; pattern: string; tag_key: string; tag_value: string; enabled: boolean }) =>
    request<TagRule>(`/projects/${projectId}/tag-rules/${ruleId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTagRule: (projectId: string, ruleId: string) =>
    request<void>(`/projects/${projectId}/tag-rules/${ruleId}`, { method: 'DELETE' }),

  // Jira
  testJiraConnection: (projectId: string) =>
    request<{ ok: boolean; error?: string }>(`/projects/${projectId}/jira/test`, { method: 'POST' }),
  createJiraTicket: (projectId: string, issueId: string) =>
    request<{ key: string; url: string }>(`/projects/${projectId}/issues/${issueId}/jira`, { method: 'POST' }),
  listJiraRules: (projectId: string) => request<JiraRule[]>(`/projects/${projectId}/jira/rules`),
  createJiraRule: (projectId: string, data: { name: string; enabled: boolean; level_filter: string; min_events: number; min_users: number; title_pattern: string }) =>
    request<JiraRule>(`/projects/${projectId}/jira/rules`, { method: 'POST', body: JSON.stringify(data) }),
  updateJiraRule: (projectId: string, ruleId: string, data: { name: string; enabled: boolean; level_filter: string; min_events: number; min_users: number; title_pattern: string }) =>
    request<JiraRule>(`/projects/${projectId}/jira/rules/${ruleId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteJiraRule: (projectId: string, ruleId: string) =>
    request<void>(`/projects/${projectId}/jira/rules/${ruleId}`, { method: 'DELETE' }),

  // GitHub
  testGithubConnection: (projectId: string) =>
    request<{ ok: boolean; error?: string }>(`/projects/${projectId}/github/test`, { method: 'POST' }),
  createGithubIssue: (projectId: string, issueId: string) =>
    request<{ number: number; url: string }>(`/projects/${projectId}/issues/${issueId}/github`, { method: 'POST' }),
  listGithubRules: (projectId: string) => request<GithubRule[]>(`/projects/${projectId}/github/rules`),
  createGithubRule: (projectId: string, data: { name: string; enabled: boolean; level_filter: string; min_events: number; min_users: number; title_pattern: string }) =>
    request<GithubRule>(`/projects/${projectId}/github/rules`, { method: 'POST', body: JSON.stringify(data) }),
  updateGithubRule: (projectId: string, ruleId: string, data: { name: string; enabled: boolean; level_filter: string; min_events: number; min_users: number; title_pattern: string }) =>
    request<GithubRule>(`/projects/${projectId}/github/rules/${ruleId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteGithubRule: (projectId: string, ruleId: string) =>
    request<void>(`/projects/${projectId}/github/rules/${ruleId}`, { method: 'DELETE' }),

  // Tickets
  createTicket: (projectId: string, issueId: string, data?: { priority?: number }) =>
    request<Ticket>(`/projects/${projectId}/issues/${issueId}/ticket`, { method: 'POST', body: JSON.stringify(data || {}) }),
  getTicketByIssue: (projectId: string, issueId: string) =>
    request<{ ticket: Ticket | null }>(`/projects/${projectId}/issues/${issueId}/ticket`),
  getTicket: (projectId: string, ticketId: string) =>
    request<Ticket>(`/projects/${projectId}/tickets/${ticketId}`),
  updateTicket: (projectId: string, ticketId: string, data: Partial<TicketUpdate>) =>
    request<Ticket>(`/projects/${projectId}/tickets/${ticketId}`, { method: 'PUT', body: JSON.stringify(data) }),
  getTicketTransitions: (projectId: string, ticketId: string) =>
    request<{ current: string; transitions: string[] }>(`/projects/${projectId}/tickets/${ticketId}/transitions`),
  listTickets: (projectId: string, params?: { status?: string; limit?: number; offset?: number }) => {
    const q = new URLSearchParams()
    if (params?.status) q.set('status', params.status)
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    return request<TicketListResponse>(`/projects/${projectId}/tickets?${q}`)
  },
  getTicketCounts: (projectId: string) =>
    request<Record<string, number>>(`/projects/${projectId}/tickets/counts`),

  // Activities
  listActivities: (projectId: string, issueId: string, params?: { limit?: number; offset?: number }) => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    return request<ActivityListResponse>(`/projects/${projectId}/issues/${issueId}/activities?${q}`)
  },

  // Uploads
  uploadImage: async (file: File): Promise<string> => {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch('/api/v1/upload', {
      method: 'POST',
      body: form,
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || 'Upload failed')
    }
    const data = await res.json()
    return data.url
  },

  // Alerts
  listAlerts: (projectId: string) => request<AlertConfig[]>(`/projects/${projectId}/alerts`),
  createAlert: (projectId: string, data: { alert_type: string; config: object; enabled: boolean; level_filter?: string; title_pattern?: string; min_events?: number; min_velocity_1h?: number; exclude_pattern?: string; conditions?: object }) =>
    request<AlertConfig>(`/projects/${projectId}/alerts`, { method: 'POST', body: JSON.stringify(data) }),
  updateAlert: (projectId: string, alertId: string, data: { config: object; enabled: boolean; level_filter?: string; title_pattern?: string; min_events?: number; min_velocity_1h?: number; exclude_pattern?: string; conditions?: object }) =>
    request<AlertConfig>(`/projects/${projectId}/alerts/${alertId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAlert: (projectId: string, alertId: string) =>
    request<void>(`/projects/${projectId}/alerts/${alertId}`, { method: 'DELETE' }),

  // Global tokens
  listGlobalTokens: () => request<any[]>('/tokens'),
  createGlobalToken: (data: { name: string; permission: string; expires_in?: number }) =>
    request<any>('/tokens', { method: 'POST', body: JSON.stringify(data) }),
  deleteGlobalToken: (tokenId: string) =>
    request<void>(`/tokens/${tokenId}`, { method: 'DELETE' }),
}

// Types
export interface User {
  id: string
  email: string
  name: string
  role: string
  status: string
  avatar_url: string
  created_at: string
}

export interface ProjectGroup {
  id: string
  name: string
  position: number
  created_at: string
}

export interface Project {
  id: string
  numeric_id: number
  name: string
  slug: string
  default_cooldown_minutes: number
  warning_as_error: boolean
  max_events_per_issue: number
  icon: string
  color: string
  position: number
  jira_base_url: string
  jira_email: string
  jira_api_token_set: boolean
  jira_project_key: string
  jira_issue_type: string
  github_token_set: boolean
  github_owner: string
  github_repo: string
  github_labels: string
  workflow_mode: string
  issue_display_mode: string
  group_id: string | null
  created_at: string
  total_issues?: number
  open_issues?: number
  latest_event?: string
  trend?: number[]
  latest_release?: string
  errors_this_week?: number
  errors_last_week?: number
}

export interface ProjectWithDSN extends Project {
  dsn: string
  legacy_dsn: string
}

export interface Issue {
  id: string
  project_id: string
  title: string
  fingerprint: string
  status: string
  level: string
  platform: string
  first_seen: string
  last_seen: string
  event_count: number
  assigned_to: string | null
  resolved_at: string | null
  cooldown_until: string | null
  resolved_in_release: string | null
  snooze_until: string | null
  snooze_event_threshold: number | null
  snooze_events_at_start: number
  jira_ticket_key: string | null
  jira_ticket_url: string | null
  github_issue_number: number | null
  github_issue_url: string | null
  priority: number
  culprit: string
  first_release: string
  followed?: boolean
  followers?: { id: string; name: string; email: string }[]
  tags?: IssueTag[]
  user_count?: number
  trend?: number[]
}

export interface IssueListResponse {
  issues: Issue[]
  total: number
  limit: number
  offset: number
}

export interface Event {
  id: string
  issue_id: string
  project_id: string
  event_id: string
  timestamp: string
  platform: string
  level: string
  message: string
  release: string
  environment: string
  server_name: string
  data: Record<string, unknown>
}

export interface EventListResponse {
  events: Event[]
  total: number
  limit: number
  offset: number
}

export interface IssueCounts {
  total: number
  by_status: Record<string, number>
  today: number
  assigned_to_me: number
  assigned_any: number
}

export interface APIToken {
  id: string
  project_id: string
  name: string
  permission: string
  token?: string
  last_used_at: string | null
  expires_at: string | null
  created_at: string
}

export interface IssueTag {
  key: string
  value: string
}

export interface IssueComment {
  id: string
  issue_id: string
  user_id: string
  user_name: string
  user_email: string
  user_avatar: string
  body: string
  created_at: string
  updated_at: string
}

export interface TagRule {
  id: string
  project_id: string
  name: string
  pattern: string
  tag_key: string
  tag_value: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface PriorityRule {
  id: string
  project_id: string
  name: string
  rule_type: string
  pattern: string
  operator: string
  threshold: number
  points: number
  enabled: boolean
  position: number
  created_at: string
  updated_at: string
}

export type PriorityRuleData = {
  name: string
  rule_type: string
  pattern: string
  operator: string
  threshold: number
  points: number
  enabled: boolean
}

export interface JiraRule {
  id: string
  project_id: string
  name: string
  enabled: boolean
  level_filter: string
  min_events: number
  min_users: number
  title_pattern: string
  created_at: string
  updated_at: string
}

export interface GithubRule {
  id: string
  project_id: string
  name: string
  enabled: boolean
  level_filter: string
  min_events: number
  min_users: number
  title_pattern: string
  created_at: string
  updated_at: string
}

export interface Ticket {
  id: string
  issue_id: string
  project_id: string
  status: string
  assigned_to: string | null
  created_by: string
  priority: number
  due_date: string | null
  resolution_type: string | null
  resolution_notes: string | null
  fix_reference: string | null
  title: string
  description: string
  escalated_system: string | null
  escalated_key: string | null
  escalated_url: string | null
  created_at: string
  updated_at: string
}

export interface TicketUpdate {
  status?: string
  assigned_to?: string
  priority?: number
  due_date?: string
  resolution_type?: string
  resolution_notes?: string
  fix_reference?: string
  title?: string
  description?: string
  escalated_system?: string
  escalated_key?: string
  escalated_url?: string
  force?: boolean
}

export interface TicketWithIssue extends Ticket {
  issue_title: string
  issue_level: string
  issue_event_count: number
  issue_first_seen: string
  issue_last_seen: string
  assignee_name: string | null
  assignee_email: string | null
  assignee_avatar: string | null
}

export interface TicketListResponse {
  tickets: TicketWithIssue[]
  total: number
}

export interface Activity {
  id: string
  issue_id: string
  ticket_id: string | null
  user_id: string | null
  user_name: string | null
  user_email: string | null
  user_avatar: string | null
  action: string
  old_value: string | null
  new_value: string | null
  metadata: Record<string, unknown> | null
  created_at: string
}

export interface ActivityListResponse {
  activities: Activity[]
  total: number
}

export interface AlertConfig {
  id: string
  project_id: string
  alert_type: string
  config: Record<string, unknown>
  enabled: boolean
  level_filter: string
  title_pattern: string
  min_events: number
  min_velocity_1h: number
  exclude_pattern: string
  conditions: object | null
  created_at: string
}
