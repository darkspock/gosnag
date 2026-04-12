# Epic: Source Code Integration

## Product Thesis

**Error context without code context is half the story.** When a developer sees a stack trace in GoSnag, they should be able to click through to the exact file and line in their repository, see who last modified that code, and identify which commit likely introduced the bug — without leaving GoSnag or manually searching through Git history.

GoSnag already captures detailed stack traces with filenames, line numbers, and function names. But today these are static text. Linking them to live source code transforms GoSnag from "here's what broke" into "here's what broke, here's the code, here's who changed it, and here's when."

## Terminology

- **Source link**: A clickable URL from a stack frame to the file+line in GitHub/Bitbucket
- **Suspect commit**: A recent commit that modified files appearing in the stack trace — likely the commit that introduced the bug
- **Release commit**: The Git SHA or tag associated with a release version string (e.g., `1.2.3` → `abc123`)
- **Path mapping**: Translation from runtime file paths (e.g., `/var/www/app/handlers/users.py`) to repository paths (e.g., `src/handlers/users.py`)

## What Already Exists

| Feature | Current State |
|---------|--------------|
| Stack traces | Full frame-by-frame display (filename, function, line, column, context) |
| Release field | `first_release` on issues, `release` on events — text string, no Git link |
| Repository config | None — no repo URL stored anywhere |
| Code browsing | None |
| Git API access | GitHub token exists per project (for issue creation), Bitbucket not yet |

---

## Block 1: Repository Configuration

**Priority: Highest. Foundation for everything else.**

### 1.1 Data Model

Add repository configuration to projects:

```sql
ALTER TABLE projects
    ADD COLUMN repo_provider TEXT NOT NULL DEFAULT '',    -- 'github', 'bitbucket', ''
    ADD COLUMN repo_owner TEXT NOT NULL DEFAULT '',       -- org or user
    ADD COLUMN repo_name TEXT NOT NULL DEFAULT '',        -- repository name
    ADD COLUMN repo_default_branch TEXT NOT NULL DEFAULT 'main',
    ADD COLUMN repo_token TEXT NOT NULL DEFAULT '',       -- PAT for API access (read)
    ADD COLUMN repo_path_strip TEXT NOT NULL DEFAULT '';  -- prefix to strip from runtime paths
```

**Note:** `repo_token` is separate from `github_token` (used for issue creation). A project might use GitHub for issues but Bitbucket for code, or need different token scopes. If both are GitHub and the same token works, the UI can offer "Use same token as GitHub Issues."

**Path mapping example:**
- Runtime path: `/var/www/app/handlers/users.py`
- `repo_path_strip`: `/var/www/app/`
- Resulting repo path: `handlers/users.py`
- URL: `https://github.com/org/repo/blob/main/handlers/users.py#L42`

### 1.2 Provider Abstraction

Create `internal/sourcecode/` package with a provider interface:

```go
type Provider interface {
    // FileURL returns a link to a specific file and line in the repository
    FileURL(path string, line int, commitOrBranch string) string
    
    // GetCommitsForFiles returns recent commits that touched any of the given files
    GetCommitsForFiles(ctx context.Context, files []string, since time.Time) ([]Commit, error)
    
    // GetBlame returns blame info for a file at a specific line range
    GetBlame(ctx context.Context, path string, startLine, endLine int) ([]BlameLine, error)
    
    // GetCommit returns details for a specific commit SHA
    GetCommit(ctx context.Context, sha string) (*Commit, error)
    
    // ResolveRef resolves a tag or branch name to a commit SHA
    ResolveRef(ctx context.Context, ref string) (string, error)
    
    // TestConnection verifies the token and repo are accessible
    TestConnection(ctx context.Context) error
}

type Commit struct {
    SHA       string
    Message   string
    Author    string
    Email     string
    Timestamp time.Time
    URL       string
    Files     []string  // files modified in this commit
}

type BlameLine struct {
    Line      int
    CommitSHA string
    Author    string
    Email     string
    Timestamp time.Time
}
```

### 1.3 GitHub Provider

Implements the `Provider` interface using the GitHub REST API:

- `GET /repos/{owner}/{repo}/commits?path={file}&since={date}` — commits for file
- `GET /repos/{owner}/{repo}/git/blobs` + blame via GraphQL — blame info
- `GET /repos/{owner}/{repo}/git/ref/tags/{tag}` — resolve release tag to SHA
- Rate limit handling with `X-RateLimit-Remaining` header checks
- File URLs: `https://github.com/{owner}/{repo}/blob/{branch}/{path}#L{line}`

### 1.4 Bitbucket Provider

Implements the same interface using the Bitbucket Cloud REST API:

- `GET /2.0/repositories/{workspace}/{slug}/commits?path={file}` — commits for file  
- `GET /2.0/repositories/{workspace}/{slug}/src/{commit}/{path}` — file content
- `GET /2.0/repositories/{workspace}/{slug}/refs/tags/{name}` — resolve tag
- Auth: App passwords or OAuth tokens
- File URLs: `https://bitbucket.org/{workspace}/{slug}/src/{branch}/{path}#lines-{line}`

### 1.5 Settings UI

In Project Settings → Integrations, add a "Source Code" section:

- **Provider**: GitHub / Bitbucket dropdown
- **Owner/Workspace**: org or user name
- **Repository**: repo name
- **Default branch**: defaults to `main`
- **Token**: personal access token (needs `repo:read` / `repository:read`)
- **Path strip prefix**: runtime path prefix to remove (e.g., `/var/www/app/`)
- **Test Connection** button: verifies token + repo access
- Option: "Use same credentials as GitHub Issues integration" checkbox

### 1.6 API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/projects/{id}/repo/test` | Test repository connection |
| `GET` | `/projects/{id}/repo/file-url` | Get URL for a file+line (used by frontend to build links) |

---

## Block 2: Source Links in Stack Traces

**Priority: High. Immediate value with minimal backend work.**

### 2.1 Stack Frame Links

Transform every stack frame from static text into a clickable link:

**Before:**
```
  app/handlers/users.py:42 in handle_request
  app/middleware/auth.py:15 in authenticate
  flask/app.py:1234 in dispatch_request
```

**After:**
```
  app/handlers/users.py:42 in handle_request        → [View in GitHub]
  app/middleware/auth.py:15 in authenticate           → [View in GitHub]
  flask/app.py:1234 in dispatch_request              → (no link — external dependency)
```

**Logic:**
1. For each frame, apply path stripping: `/var/www/app/handlers/users.py` → `handlers/users.py`
2. Check if the resulting path looks like it belongs to the repo (not a framework/library path)
3. Build the URL: `https://github.com/{owner}/{repo}/blob/{branch}/{path}#L{line}`
4. If the event has a `release` and that release resolves to a commit SHA, use the SHA instead of branch (pins to exact code version)

**Filtering out library frames:**
- Common patterns to skip: `node_modules/`, `vendor/`, `site-packages/`, `lib/python`, `.gem/`, `/usr/lib/`
- Configurable per-project: "Library path patterns" (optional, sensible defaults)

### 2.2 Frontend Implementation

In the stack trace viewer (`IssueDetail.tsx` and event detail):
- Each frame that has a repo path gets a small link icon
- Click opens the file in a new tab at the correct line
- Hover shows the full URL
- Frames without a matching repo path stay as plain text

### 2.3 Release-Pinned Links

When an event has a `release` value:
1. Try to resolve it as a Git tag or branch: `v1.2.3` → commit `abc123`
2. Build links using the commit SHA instead of the branch: `/blob/abc123/path#L42`
3. This ensures the code shown matches exactly what was running when the error occurred

Cache the release → SHA mapping to avoid repeated API calls.

---

## Block 3: Suspect Commits

**Priority: Medium. High value but requires more API calls.**

### 3.1 Concept

When a new issue is detected, GoSnag identifies which recent commits most likely introduced the bug by checking which commits modified files that appear in the stack trace.

**Algorithm:**
1. Extract unique file paths from the stack trace (after path stripping)
2. For each file, query the Git provider for commits in the last 7 days (or since the previous release)
3. Rank commits by: number of matching files touched + recency
4. Display the top 3 suspect commits on the issue detail page

### 3.2 Data Model

```sql
CREATE TABLE suspect_commits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    commit_sha TEXT NOT NULL,
    commit_message TEXT NOT NULL,
    commit_author TEXT NOT NULL,
    commit_email TEXT NOT NULL,
    commit_url TEXT NOT NULL,
    committed_at TIMESTAMPTZ NOT NULL,
    matching_files TEXT[] NOT NULL,     -- files from stacktrace that this commit touched
    score INT NOT NULL DEFAULT 0,       -- ranking score
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_suspect_commits_issue ON suspect_commits(issue_id);
```

### 3.3 Background Worker

A background worker runs when a new issue is created (or on-demand):

1. Get the latest event's stack trace
2. Extract file paths, apply path stripping
3. Query the provider for recent commits touching those files
4. Score and store the top suspect commits
5. Record an activity: `suspect_commits_found`

**Rate limiting:** Only run for new issues (not every event). Cache file→commit mappings for 1 hour to reduce API calls.

### 3.4 UI

On the issue detail page (and ticket detail page), show a "Suspect Commits" section:

```
Suspect Commits
━━━━━━━━━━━━━━━
🔴 abc1234  "Fix user authentication flow"          Juan  2h ago  
             Touched: auth.py, middleware.py (2 matching files)  [View commit]

🟡 def5678  "Update database connection pooling"    Maria  1d ago
             Touched: db.py (1 matching file)                   [View commit]
```

Each commit links to the provider (GitHub/Bitbucket commit URL).

### 3.5 Blame Integration

For the top frame in the stack trace, fetch Git blame for the specific line range:

- Show inline on the stack frame: "Last modified by Juan in abc1234, 3 days ago"
- This gives instant attribution without needing to check multiple commits

---

## Block 4: Release Tracking

**Priority: Medium. Builds on the release field that already exists.**

### 4.1 Release → Commit Mapping

When an event arrives with a `release` value:
1. Check if we've already resolved this release to a commit SHA
2. If not, query the provider: try as a Git tag first, then as a branch
3. Store the mapping

```sql
CREATE TABLE release_commits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    release_version TEXT NOT NULL,
    commit_sha TEXT NOT NULL,
    commit_url TEXT,
    committed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, release_version)
);
```

### 4.2 Release Diff

When an issue has a `first_release`, show what changed in that release:

- Link to the diff between the previous release and this one: `https://github.com/{owner}/{repo}/compare/{prev_tag}...{current_tag}`
- List of files changed in the release
- Cross-reference with stack trace files to highlight suspicious changes

### 4.3 Deploy Tracking

Optional webhook endpoint that CI/CD can call after a deploy:

```
POST /api/v1/projects/{id}/deploys
{
  "version": "1.2.3",
  "commit": "abc123",
  "environment": "production",
  "url": "https://github.com/org/repo/releases/tag/v1.2.3"
}
```

This records when a release was deployed, enabling:
- "This issue first appeared 15 minutes after deploy of v1.2.3"
- Timeline correlation between deploys and error spikes

### 4.4 UI

On the issue detail page, a "Release" section:

```
Release
━━━━━━━
First seen in: v1.2.3 (abc1234)          [View release] [View diff from v1.2.2]
Deployed: 2024-01-15 14:30 UTC (2h ago)
Files changed in this release: 23         [3 match stack trace]
```

---

## Implementation Order

```
Block 1: Repository Config ──────┐
  1.1 Data model                  │
  1.2 Provider interface          ├──> Block 2: Source Links ──> Block 3: Suspect Commits
  1.3 GitHub provider             │      2.1 Frame links           3.1 Algorithm
  1.4 Bitbucket provider          │      2.2 Frontend              3.2 Data model
  1.5 Settings UI                 │      2.3 Release-pinned        3.3 Background worker
  1.6 API                         │                                3.4 UI
                                  │                                3.5 Blame
                                  │
                                  └──> Block 4: Release Tracking
                                         4.1 Release → commit mapping
                                         4.2 Release diff
                                         4.3 Deploy webhook
                                         4.4 UI
```

## MVP

1. **Repository config** (Block 1) — provider, credentials, path mapping
2. **Source links in stack traces** (Block 2) — clickable links to GitHub/Bitbucket
3. **Release-pinned links** (Block 2.3) — links point to exact release commit

**Deferred:** Suspect commits, blame, release diff, deploy tracking.

The MVP gives immediate value: every stack frame becomes a link to the code. No Git API calls needed for the basic case — just URL construction from the repo config.

---

## API Rate Limiting Considerations

GitHub API: 5,000 requests/hour with authenticated token. Bitbucket: 1,000 requests/hour.

**Strategies:**
- Source links (Block 2): **Zero API calls** — built from URL pattern, no API needed
- Release resolution (Block 2.3): **1 call per unique release** — cached permanently
- Suspect commits (Block 3): **1-5 calls per new issue** — only runs on first event
- Blame (Block 3.5): **1 call per frame** — only for top frame, cached

At typical volumes (< 100 new issues/day), this stays well within rate limits.

## Security

- Repository tokens are stored like Jira/GitHub issue tokens: never returned in API responses, `_set` boolean flag instead
- Tokens need **read-only** access (no write operations on the repository)
- Path stripping prevents leaking internal server paths in URLs
- Library frame filtering prevents linking to wrong repositories

## Out of Scope

- Inline code display (showing source code within GoSnag)
- PR integration (auto-commenting on PRs that introduce errors)
- Code search across repositories
- Multi-repo per project (one project = one repo)
- Self-hosted Git servers (GitLab, Gitea) — future consideration
