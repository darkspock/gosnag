package project

import (
	"context"
	"sync"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// StatsCache caches the project list with stats. Invalidated on event ingestion.
type StatsCache struct {
	mu      sync.RWMutex
	data    []ProjectListItem
	valid   bool
	buildAt time.Time
	maxAge  time.Duration
	queries *db.Queries
}

func NewStatsCache(queries *db.Queries, maxAge time.Duration) *StatsCache {
	return &StatsCache{
		queries: queries,
		maxAge:  maxAge,
	}
}

// Invalidate marks the cache as stale. Next Get will recompute.
func (c *StatsCache) Invalidate() {
	c.mu.Lock()
	c.valid = false
	c.mu.Unlock()
}

// Get returns the cached project list, recomputing if stale or expired.
func (c *StatsCache) Get(ctx context.Context) ([]ProjectListItem, error) {
	c.mu.RLock()
	if c.valid && time.Since(c.buildAt) < c.maxAge {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	return c.rebuild(ctx)
}

func (c *StatsCache) rebuild(ctx context.Context) ([]ProjectListItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have rebuilt while we waited for the lock
	if c.valid && time.Since(c.buildAt) < c.maxAge {
		return c.data, nil
	}

	projects, err := c.queries.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	// Run the 4 expensive queries in parallel
	var (
		wg         sync.WaitGroup
		stats      []db.GetProjectStatsRow
		trendRows  []db.GetProjectEventTrendRow
		releaseRows []db.GetProjectLatestReleaseRow
		weeklyRows []db.GetProjectWeeklyErrorsRow
	)

	wg.Add(4)
	go func() { defer wg.Done(); stats, _ = c.queries.GetProjectStats(ctx) }()
	go func() { defer wg.Done(); trendRows, _ = c.queries.GetProjectEventTrend(ctx) }()
	go func() { defer wg.Done(); releaseRows, _ = c.queries.GetProjectLatestRelease(ctx) }()
	go func() { defer wg.Done(); weeklyRows, _ = c.queries.GetProjectWeeklyErrors(ctx) }()
	wg.Wait()

	// Build maps
	statsMap := make(map[uuid.UUID]db.GetProjectStatsRow, len(stats))
	for _, s := range stats {
		statsMap[s.ProjectID] = s
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	trendMap := make(map[uuid.UUID][]int32)
	for _, tr := range trendRows {
		daysAgo := int(now.Sub(tr.Bucket.UTC().Truncate(24*time.Hour)).Hours() / 24)
		if daysAgo < 0 || daysAgo >= 14 {
			continue
		}
		if trendMap[tr.ProjectID] == nil {
			trendMap[tr.ProjectID] = make([]int32, 14)
		}
		trendMap[tr.ProjectID][13-daysAgo] = tr.Count
	}

	releaseMap := make(map[uuid.UUID]string, len(releaseRows))
	for _, r := range releaseRows {
		releaseMap[r.ProjectID] = r.Release
	}

	weeklyMap := make(map[uuid.UUID]db.GetProjectWeeklyErrorsRow, len(weeklyRows))
	for _, w := range weeklyRows {
		weeklyMap[w.ProjectID] = w
	}

	result := make([]ProjectListItem, len(projects))
	for i, p := range projects {
		item := ProjectListItem{SafeProject: toSafeProject(p), Trend: make([]int32, 14)}
		if s, ok := statsMap[p.ID]; ok {
			item.TotalIssues = s.TotalIssues
			item.OpenIssues = s.OpenIssues
			if t, ok := s.LatestEvent.(time.Time); ok {
				item.LatestEvent = t.Format(time.RFC3339)
			}
		}
		if t, ok := trendMap[p.ID]; ok {
			item.Trend = t
		}
		item.LatestRelease = releaseMap[p.ID]
		if w, ok := weeklyMap[p.ID]; ok {
			item.ErrorsThisWeek = w.ThisWeek
			item.ErrorsLastWeek = w.LastWeek
		}
		result[i] = item
	}

	c.data = result
	c.valid = true
	c.buildAt = time.Now()

	return result, nil
}
