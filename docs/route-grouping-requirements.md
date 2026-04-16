# Route Grouping — Requirements Document

---

## 1. Overview

Add framework-aware route grouping to GoSnag so issues caused by dynamic URLs can be grouped by a canonical route template instead of raw request paths.

This is especially important for applications where the runtime URL includes path parameters, such as CodeIgniter routes like:

- `/coverApp/Reserv/getCalendar/4/2026`
- `/api/v4/internal/payment/id/12345`

Without canonicalization, the same logical endpoint can create multiple issues purely because path parameters differ.

### 1.1 Goals

- Group issues by canonical route instead of raw parameterized path when appropriate
- Support framework-specific route analysis starting with CodeIgniter
- Allow projects to define their framework explicitly
- Provide a project-level UI to inspect, import, edit, enable, and disable route grouping rules
- Prefer deterministic routing sources from code over heuristics or AI
- Preserve the error title while using canonical route data for culprit and grouping

### 1.2 Non-Goals

- Replacing stacktrace-based grouping for exception-driven issues
- Full static analysis of every supported framework in the first iteration
- AI-only route discovery without deterministic fallbacks
- Automatically rewriting historical issues in the first release
- Generating framework route definitions back into source repositories

---

## 2. Problem Statement

### 2.1 Current Limitation

GoSnag can extract a request path from the event, but it currently treats the path too literally. Query strings are removed, but dynamic path segments are not normalized into route templates.

Example:

- Raw URL: `GET /coverApp/Reserv/getCalendar/4/2026`
- Desired route template: `GET /coverApp/Reserv/getCalendar/:restaurant_id/:year`

### 2.2 Desired Outcome

For route-centric issues, the system should:

- keep the issue title as the error/problem title, e.g. `Error: [ExcessiveQueries]`
- use the canonical route template as the culprit, e.g. `GET /coverApp/Reserv/getCalendar/:restaurant_id/:year`
- optionally use the canonical route template as part of the fingerprint when route grouping is enabled

---

## 3. Project Configuration

### 3.1 Framework Field

**REQ-ROUTE-001**: Each project MUST have a `framework` field.

**REQ-ROUTE-002**: Supported initial values MUST include:

- `generic`
- `codeigniter`

**REQ-ROUTE-003**: The system SHOULD be designed to support future values such as:

- `laravel`
- `django`
- `rails`
- `express`
- `spring`

**REQ-ROUTE-004**: `framework` MUST default to `generic`.

### 3.2 Route Grouping Mode

**REQ-ROUTE-010**: Each project MUST support route grouping independently of stacktrace grouping.

**REQ-ROUTE-011**: The project MUST allow enabling or disabling canonical route grouping for route-centric issues.

**REQ-ROUTE-012**: Route grouping MUST NOT overwrite or degrade exception/stacktrace grouping when the exception fingerprint is already the best signal.

---

## 4. Route Rule Model

### 4.1 Route Grouping Rule

**REQ-ROUTE-020**: The system MUST support a project-level collection of route grouping rules.

**REQ-ROUTE-021**: Each rule MUST support at least:

- HTTP method or method wildcard
- route template or match pattern
- canonical route template
- target/controller string if known
- source of the rule
- confidence score
- enabled/disabled state

**REQ-ROUTE-022**: Each rule SHOULD also support metadata such as:

- source file path
- framework
- notes
- imported timestamp

### 4.2 Rule Sources

**REQ-ROUTE-030**: A route grouping rule MUST store how it was discovered. Initial source values SHOULD include:

- `source_code`
- `framework_convention`
- `observed_issue`
- `ai_suggestion`
- `manual`

**REQ-ROUTE-031**: Rules imported from source code MUST be treated as higher-confidence than issue-derived or AI-derived rules.

---

## 5. Project Settings UI

### 5.1 Issues Configuration

**REQ-ROUTE-040**: Project Settings → Issues MUST include a routing section when route grouping is supported.

**REQ-ROUTE-041**: The routing section MUST include:

- project framework selector
- route grouping enable/disable control
- route grouping rules table
- import button

### 5.2 Route Rules Table

**REQ-ROUTE-042**: The rules table MUST display at least:

- method
- canonical route template
- target/controller
- source
- confidence
- enabled state

**REQ-ROUTE-043**: Users MUST be able to:

- enable or disable a rule
- edit a rule
- delete a rule
- create a manual rule

**REQ-ROUTE-044**: The UI SHOULD allow filtering by source and confidence.

### 5.3 Import Flow

**REQ-ROUTE-045**: The UI MUST provide an `Import Rules` action.

**REQ-ROUTE-046**: The import action SHOULD show available import sources for the project, such as:

- source code
- existing issues
- framework conventions
- AI suggestions

**REQ-ROUTE-047**: Imported rules MUST be reviewable after import, not silently activated without visibility.

---

## 6. Import Sources and Priority

### 6.1 Source Code Import

**REQ-ROUTE-050**: Source code import MUST be the primary route discovery mechanism when source code is available.

**REQ-ROUTE-051**: For `codeigniter`, the importer MUST support:

- `application/config/routes.php`
- additional route files included from that root file
- route definitions expressed as `$route[...] = ...`
- route definitions with HTTP verb subkeys such as `['get']` or `['post']`

**REQ-ROUTE-052**: The importer SHOULD preserve CodeIgniter placeholders such as:

- `(:any)`
- `(:num)`
- explicit regex route segments

**REQ-ROUTE-053**: The importer MUST capture the controller target string when available.

### 6.2 Framework Convention Import

**REQ-ROUTE-060**: If the framework supports implicit routing conventions, the system MAY generate additional fallback rules from framework conventions.

**REQ-ROUTE-061**: Convention-derived rules MUST be marked separately from explicit route definitions.

### 6.3 Existing Issues Import

**REQ-ROUTE-070**: The system SHOULD support discovering candidate route rules from existing issues.

**REQ-ROUTE-071**: Issue-derived rules MUST be heuristics only and MUST have lower default confidence than source-derived rules.

**REQ-ROUTE-072**: Issue-derived import SHOULD analyze:

- observed request URLs
- request methods
- stacktrace controller paths when available
- repeated path shapes across many issues

### 6.4 AI Suggestions

**REQ-ROUTE-080**: AI MAY be used to suggest route normalization candidates when deterministic parsing is not enough.

**REQ-ROUTE-081**: AI MUST NOT be the primary or only source of truth when source code is available.

**REQ-ROUTE-082**: AI-generated rules MUST be marked as `ai_suggestion` and SHOULD require explicit review.

---

## 7. Route Canonicalization Behavior

### 7.1 Canonical Route Output

**REQ-ROUTE-090**: When a request URL matches a route grouping rule, the system MUST produce a canonical route template.

**REQ-ROUTE-091**: The canonical route template SHOULD preserve stable path structure while normalizing variable path segments.

**REQ-ROUTE-092**: Canonical route templates MAY use placeholders such as:

- `:any`
- `:num`
- `:uuid`
- `:date`
- `:token`
- project- or framework-specific parameter names when known

### 7.2 Title, Culprit, Fingerprint

**REQ-ROUTE-100**: Route grouping MUST NOT replace the issue title with the route template.

**REQ-ROUTE-101**: The issue title MUST remain the error/problem title.

**REQ-ROUTE-102**: When canonical route grouping applies, the culprit SHOULD become the canonical route template, optionally prefixed with method.

**REQ-ROUTE-103**: The fingerprint MAY incorporate the canonical route template for route-centric issues when the project has route grouping enabled.

**REQ-ROUTE-104**: Exception-driven issues with strong stacktrace-based grouping SHOULD continue using exception grouping unless the product explicitly defines a route-first override mode.

---

## 8. CodeIgniter-Specific Requirements

### 8.1 Explicit Routes

**REQ-ROUTE-CI-001**: The CodeIgniter analyzer MUST parse the root routes file and any route files included from it.

**REQ-ROUTE-CI-002**: The analyzer MUST recognize:

- simple route definitions
- HTTP verb-specific route definitions
- placeholders like `(:any)` and `(:num)`
- custom regex segments inside route keys

### 8.2 Implicit Routes

**REQ-ROUTE-CI-010**: The analyzer SHOULD support CodeIgniter’s conventional controller/method/params shape as a fallback when no explicit route matches.

**REQ-ROUTE-CI-011**: Convention-based candidates MUST be marked as lower confidence than explicit routes.

### 8.3 Controller-Derived Heuristics

**REQ-ROUTE-CI-020**: The analyzer MAY use stacktrace frames under `application/controllers/` to infer likely controller ownership for issue-derived candidate rules.

**REQ-ROUTE-CI-021**: Controller-derived heuristics MUST NOT override explicit route definitions from source code.

---

## 9. Source Code Integration

### 9.1 Repository Connectivity

**REQ-ROUTE-110**: If a project has source code connection configured, the system SHOULD be able to fetch or inspect the configured repository to import route rules.

**REQ-ROUTE-111**: Route import from source code SHOULD work both:

- from local/available source when present in the environment
- from connected source code provider when repository access is configured

### 9.2 Import Constraints

**REQ-ROUTE-112**: Importers MUST avoid executing application code. Parsing MUST be static or structure-aware.

**REQ-ROUTE-113**: If source code parsing fails, the system MUST fail gracefully and allow fallback import sources.

---

## 10. Data Quality and Confidence

### 10.1 Confidence

**REQ-ROUTE-120**: Each imported rule SHOULD have a confidence score.

**REQ-ROUTE-121**: Suggested default confidence ordering:

- explicit route from source code: highest
- framework convention: high
- issue-derived candidate: medium
- AI suggestion: low to medium
- manual: user-defined

### 10.2 Conflicts

**REQ-ROUTE-122**: If multiple rules match the same observed URL, the system MUST resolve conflicts deterministically.

**REQ-ROUTE-123**: Resolution SHOULD prioritize:

1. enabled rules over disabled rules
2. explicit source code rules over convention rules
3. convention rules over issue-derived rules
4. higher confidence over lower confidence
5. more specific route patterns over broader ones

---

## 11. API Requirements

**REQ-ROUTE-130**: The backend MUST expose APIs to:

- read project route grouping config
- list route grouping rules
- create manual rules
- update rules
- delete rules
- trigger rule import

**REQ-ROUTE-131**: Import execution MAY be synchronous for small projects and asynchronous for larger repositories.

**REQ-ROUTE-132**: If import is asynchronous, the API SHOULD expose import status and last import result.

---

## 12. Observability

**REQ-ROUTE-140**: The system SHOULD log route grouping decisions for debugging when debug logging is enabled.

**REQ-ROUTE-141**: Logs SHOULD include:

- raw URL
- normalized URL
- matched rule ID
- rule source
- confidence
- final culprit

**REQ-ROUTE-142**: The system SHOULD expose enough data in issue detail views to explain how a route was normalized.

---

## 13. Rollout Plan

### 13.1 Phase 1

- add `framework` to project
- add route grouping rules model
- add UI table and import trigger
- implement CodeIgniter source-code importer
- support canonical route templates for culprit

### 13.2 Phase 2

- add issue-derived candidate discovery
- add repository-backed import from connected source code
- add confidence-based review workflow

### 13.3 Phase 3

- add AI-assisted suggestions
- add more framework analyzers
- add historical re-grouping tools if needed

---

## 14. Key Product Decisions

- Source code is the preferred truth when available
- AI is optional and advisory, not foundational
- Canonical route grouping must be framework-aware
- Titles must stay as error/problem titles
- Route templates should influence culprit and grouping, not erase the semantic identity of the error
