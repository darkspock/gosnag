const BASE = '/api/v1'

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
    throw new Error(body.error || `Request failed: ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// Auth
export const api = {
  me: () => request<User>('/me'),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  getAuthConfig: () => request<{ google_client_id: string }>('/auth/config'),
  googleLogin: (credential: string) =>
    request<User>('/auth/google/token', { method: 'POST', body: JSON.stringify({ credential }) }),

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
  updateProject: (id: string, data: { name: string; slug: string; default_cooldown_minutes?: number; warning_as_error?: boolean; jira_base_url?: string; jira_email?: string; jira_api_token?: string; jira_project_key?: string; jira_issue_type?: string; group_id?: string | null }) =>
    request<Project>(`/projects/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProject: (id: string) =>
    request<void>(`/projects/${id}`, { method: 'DELETE' }),

  // Issues
  listIssues: (projectId: string, params?: { status?: string; level?: string; limit?: number; offset?: number; today?: boolean; assigned_to?: string; assigned_any?: boolean; search?: string; tag?: string }) => {
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
    return request<IssueListResponse>(`/projects/${projectId}/issues?${q}`)
  },
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
  name: string
  slug: string
  default_cooldown_minutes: number
  warning_as_error: boolean
  jira_base_url: string
  jira_email: string
  jira_api_token_set: boolean
  jira_project_key: string
  jira_issue_type: string
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
  priority: number
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
