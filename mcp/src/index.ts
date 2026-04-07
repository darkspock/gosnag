import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const GOSNAG_URL = process.env.GOSNAG_URL || "";
const GOSNAG_TOKEN = process.env.GOSNAG_TOKEN || "";

async function api(path: string, options?: RequestInit) {
  const url = `${GOSNAG_URL}/api/v1${path}`;
  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${GOSNAG_TOKEN}`,
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`GoSnag API error ${res.status}: ${text}`);
  }
  if (res.status === 204) return null;
  return res.json();
}

const server = new McpServer({
  name: "gosnag",
  version: "1.0.0",
});

// --- Projects ---

server.tool("list_projects", "List all projects with issue counts and trends", {}, async () => {
  const data = await api("/projects");
  return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
});

server.tool(
  "get_project",
  "Get project details including DSN",
  { project_id: z.string().describe("Project UUID") },
  async ({ project_id }) => {
    const data = await api(`/projects/${project_id}`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "create_project",
  "Create a new project",
  {
    name: z.string().describe("Project name"),
    slug: z.string().describe("URL-friendly slug"),
  },
  async ({ name, slug }) => {
    const data = await api("/projects", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "update_project",
  "Update project settings",
  {
    project_id: z.string().describe("Project UUID"),
    name: z.string().optional().describe("New name"),
    slug: z.string().optional().describe("New slug"),
    default_cooldown_minutes: z.number().optional().describe("Default cooldown in minutes"),
    warning_as_error: z.boolean().optional().describe("Treat warnings as errors"),
  },
  async ({ project_id, ...body }) => {
    const data = await api(`/projects/${project_id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "delete_project",
  "Delete a project and all its data",
  { project_id: z.string().describe("Project UUID") },
  async ({ project_id }) => {
    await api(`/projects/${project_id}`, { method: "DELETE" });
    return { content: [{ type: "text", text: "Project deleted" }] };
  }
);

// --- Issues ---

server.tool(
  "list_issues",
  "List issues for a project with optional filters",
  {
    project_id: z.string().describe("Project UUID"),
    status: z.string().optional().describe("Filter by status: open, resolved, snoozed, ignored, reopened"),
    level: z.string().optional().describe("Filter by level: errors, warning, info_only"),
    limit: z.number().optional().describe("Max results (default 25)"),
    search: z.string().optional().describe("Search in title"),
  },
  async ({ project_id, ...params }) => {
    const q = new URLSearchParams();
    if (params.status) q.set("status", params.status);
    if (params.level) q.set("level", params.level);
    if (params.limit) q.set("limit", String(params.limit));
    if (params.search) q.set("search", params.search);
    const data = await api(`/projects/${project_id}/issues?${q}`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "get_issue",
  "Get issue details",
  {
    project_id: z.string().describe("Project UUID"),
    issue_id: z.string().describe("Issue UUID"),
  },
  async ({ project_id, issue_id }) => {
    const data = await api(`/projects/${project_id}/issues/${issue_id}`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "update_issue_status",
  "Update issue status (resolve, reopen, snooze, ignore)",
  {
    project_id: z.string().describe("Project UUID"),
    issue_id: z.string().describe("Issue UUID"),
    status: z.string().describe("New status: open, resolved, snoozed, ignored"),
    cooldown_minutes: z.number().optional().describe("Cooldown minutes when resolving"),
  },
  async ({ project_id, issue_id, ...body }) => {
    const data = await api(`/projects/${project_id}/issues/${issue_id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "get_issue_events",
  "List recent events for an issue",
  {
    project_id: z.string().describe("Project UUID"),
    issue_id: z.string().describe("Issue UUID"),
    limit: z.number().optional().describe("Max events (default 10)"),
  },
  async ({ project_id, issue_id, limit }) => {
    const q = limit ? `?limit=${limit}` : "";
    const data = await api(`/projects/${project_id}/issues/${issue_id}/events${q}`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "get_issue_counts",
  "Get issue counts by status for a project",
  {
    project_id: z.string().describe("Project UUID"),
    level: z.string().optional().describe("Filter by level"),
  },
  async ({ project_id, level }) => {
    const q = level ? `?level=${level}` : "";
    const data = await api(`/projects/${project_id}/issues/counts${q}`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

// --- Alerts ---

server.tool(
  "list_alerts",
  "List alert configurations for a project",
  { project_id: z.string().describe("Project UUID") },
  async ({ project_id }) => {
    const data = await api(`/projects/${project_id}/alerts`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "create_alert",
  "Create an alert (email or slack)",
  {
    project_id: z.string().describe("Project UUID"),
    alert_type: z.enum(["email", "slack"]).describe("Alert type"),
    config: z.object({}).passthrough().describe("Alert config: {recipients:[...]} for email, {webhook_url:...} for slack"),
    conditions: z.object({}).passthrough().optional().describe("Condition group: {operator:'and',conditions:[...]}"),
  },
  async ({ project_id, ...body }) => {
    const data = await api(`/projects/${project_id}/alerts`, {
      method: "POST",
      body: JSON.stringify({ ...body, enabled: true }),
    });
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

// --- Tags ---

server.tool(
  "list_issue_tags",
  "List tags on an issue",
  {
    project_id: z.string().describe("Project UUID"),
    issue_id: z.string().describe("Issue UUID"),
  },
  async ({ project_id, issue_id }) => {
    const data = await api(`/projects/${project_id}/issues/${issue_id}/tags`);
    return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
  }
);

server.tool(
  "add_issue_tag",
  "Add a tag to an issue",
  {
    project_id: z.string().describe("Project UUID"),
    issue_id: z.string().describe("Issue UUID"),
    key: z.string().describe("Tag key"),
    value: z.string().describe("Tag value"),
  },
  async ({ project_id, issue_id, key, value }) => {
    await api(`/projects/${project_id}/issues/${issue_id}/tags`, {
      method: "POST",
      body: JSON.stringify({ key, value }),
    });
    return { content: [{ type: "text", text: `Tag ${key}:${value} added` }] };
  }
);

// --- Users ---

server.tool("list_users", "List all users", {}, async () => {
  const data = await api("/users");
  return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
});

// Start server
async function main() {
  if (!GOSNAG_URL) {
    console.error("GOSNAG_URL environment variable is required (e.g. https://sentry.cover-aws.com)");
    process.exit(1);
  }
  if (!GOSNAG_TOKEN) {
    console.error("GOSNAG_TOKEN environment variable is required (personal access token)");
    process.exit(1);
  }
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch(console.error);
