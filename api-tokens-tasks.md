# API Tokens — Public API Access (Project-scoped)

## Overview
Add Bearer token authentication so external systems can access GoSnag's API per project. Tokens are scoped to a single project with read-only or read-write permissions.

## Tasks

### Backend
- [x] Migration `000007_api_tokens` — create `api_tokens` table
- [x] SQL queries — CRUD (create, list by project, get by hash, delete, update last_used)
- [x] Run `make sqlc` to regenerate
- [x] Token auth middleware — check `Authorization: Bearer gsn_xxx`, validate project scope
- [x] Modify auth middleware chain — try Bearer token first, fall back to session cookie
- [x] Token handler — POST/GET/DELETE `/api/v1/projects/{id}/tokens`
- [x] Register routes in router.go
- [x] Write permission middleware for mutating endpoints

### Frontend
- [x] Token management section in ProjectSettings
- [x] Create token form (name, permission, optional expiry)
- [x] Show plain token once after creation (copy to clipboard)
- [x] List tokens (name, permission, created, last used, expiry, revoke button)

### Security
- [x] Store SHA-256 hash of token, never the plain token
- [x] Token prefix `gsn_` for easy identification
- [x] Read-only tokens can GET issues/events, read-write can also update status/assign
- [x] Token scoped to project — validated in middleware against URL param
