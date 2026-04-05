# Epic: Issue Tags (Manual + Auto Rules)

## Overview
Issues can have tags (key:value format like `team:payment`). Tags are assigned manually or automatically via regex rules. Issues are searchable/filterable by tags.

## Design

### Database
- `issue_tags` table: issue_id, key, value (composite unique)
- `tag_rules` table: project-scoped rules that auto-assign tags based on title regex

### Tag Format
- `key:value` — e.g. `team:payment`, `env:production`, `component:auth`
- Key and value are free text, no predefined list
- Multiple tags per issue allowed

### Manual Assignment
- Tag input on issue detail page (add/remove)
- Autocomplete from existing tags in the project

### Auto Rules
- Per project, regex on issue title → assign tag
- Default match is "contains" (plain text = contains, supports full regex)
- Example: pattern `Adyen|Stripe` → tag `team:payment`
- Evaluated on event ingestion (same as priority — goroutine)
- Only adds tag if not already present (idempotent)

### Search/Filter
- Search field in issue list supports `tag:team:payment` syntax
- Or a tag filter dropdown with existing tags

## Tasks

### Backend
- [ ] Migration `000013_tags`
- [ ] SQL queries: tag CRUD per issue, tag rules CRUD per project, search by tag
- [ ] sqlc regenerate
- [ ] Tag handler: manual add/remove on issues
- [ ] Tag rules handler: CRUD
- [ ] Auto-tag evaluator: hook into ingest pipeline
- [ ] List issues query: filter by tag
- [ ] Autocomplete endpoint: list distinct tags for a project

### Frontend
- [ ] Tag display on issue list (small badges)
- [ ] Tag input on issue detail (add/remove with autocomplete)
- [ ] Tag rules section in Project Settings
- [ ] Tag filter in issue list (search or dropdown)
