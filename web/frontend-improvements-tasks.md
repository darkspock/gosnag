# GoSnag Frontend Improvements - Task List

Based on comprehensive design review. All items verified against current codebase.

## High Priority

### 1. Toast/Notification System
- [ ] Create `web/src/components/ui/toast.tsx` component (animated, auto-dismiss, variants: success/error/info)
- [ ] Create toast context/provider (`web/src/lib/use-toast.ts`)
- [ ] Add `<Toaster />` to Layout.tsx
- [ ] Replace all `window.alert()` calls with toast (ProjectSettings.tsx:132)
- [ ] Add success toasts after: save project, create/edit/delete alert, invite user, resolve/snooze/ignore issue, assign issue, copy DSN

### 2. Confirmation Dialog Component
- [ ] Create `web/src/components/ui/confirm-dialog.tsx` (reusable, styled to match dark theme)
- [ ] Replace `window.confirm()` in ProjectSettings.tsx:64 (delete project)
- [ ] Replace `window.confirm()` in ProjectSettings.tsx:150 (delete alert)
- [ ] Replace `window.confirm()` in IssueList.tsx:406 (bulk delete issues)

### 3. Mobile IssueList Filters
- [ ] Create a sheet/drawer component or use Radix Dialog as mobile sheet
- [ ] Add filter button visible only on mobile (`md:hidden`)
- [ ] Render sidebar content inside the mobile sheet
- [ ] Ensure filter selection closes the sheet and applies filters

### 4. Consistent Form Components
- [ ] Replace raw `<input>` in UserManagement.tsx:73-77 with `<Input>` component
- [ ] Replace raw `<select>` in IssueList.tsx:309-316 with `<Select>` component

## Medium Priority

### 5. Event Pagination in IssueDetail
- [ ] Add offset/total state for events
- [ ] Show pagination controls (Previous/Next) below event list
- [ ] Show event count summary ("1-50 of 342")
- [ ] Fetch next page on demand

### 6. Skeleton Loaders
- [ ] Create `web/src/components/ui/skeleton.tsx` component
- [ ] Add skeleton for IssueList (rows with animated placeholders)
- [ ] Add skeleton for IssueDetail (header + stat cards + event list)
- [ ] Add skeleton for Projects page (card grid)
- [ ] Make all loading states consistent (replace "Loading..." text)

### 7. Empty States
- [ ] IssueList: Add icon, message, and context ("No errors found" vs "No issues match this filter")
- [ ] ProjectSettings alerts: Suggest adding first alert
- [ ] IssueDetail events: Handle empty event list gracefully

### 8. Issue Search (Full Stack)
- [ ] Backend: Add `search` param to `ListIssuesByProject` SQL query (ILIKE on title)
- [ ] Backend: Add `search` param to issue handler List endpoint
- [ ] Regenerate sqlc
- [ ] Frontend API: Add `search` param to `listIssues`
- [ ] Frontend: Add search input above issue list
- [ ] Frontend: Debounced search with URL param sync

## Lower Priority

### 9. Tooltips for Icon Buttons
- [ ] Install `@radix-ui/react-tooltip`
- [ ] Create `web/src/components/ui/tooltip.tsx`
- [ ] Add tooltips to: Copy DSN, Delete project, Edit/Delete alert buttons, Disable/Enable user, Bulk delete issues, Logout

### 10. Breadcrumb Consistency
- [ ] Create a shared `Breadcrumb` component
- [ ] Add breadcrumbs to UserManagement page
- [ ] Use chevron separators instead of `/`

### 11. Enriched Project Cards
- [ ] Backend: Add issue counts to project list endpoint (or separate endpoint)
- [ ] Frontend: Show total issues, open issues, last event time on each project card
- [ ] Add a small colored indicator for project health

### 12. Keyboard Shortcuts
- [ ] Create `useKeyboardShortcut` hook
- [ ] IssueList: `j/k` navigate, `Enter` open issue
- [ ] IssueDetail: `r` resolve, `i` ignore, `s` snooze, `Esc` go back
- [ ] Global: `/` focus search, `?` show shortcut help

### 13. Loading State Consistency
- [ ] Ensure all pages use the spinner component instead of text
- [ ] UserManagement.tsx:61 - replace "Loading..." text with spinner
- [ ] ProjectSettings.tsx:163 - replace "Loading..." text with spinner
