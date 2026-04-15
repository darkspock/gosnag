package project

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// StatsCache caches the project list with stats using stale-while-revalidate.
// Get always returns instantly from cache. When dirty and minRefresh has elapsed,
// a single background goroutine rebuilds the cache with parallel queries.
type StatsCache struct {
	mu         sync.RWMutex
	data       []ProjectListItem
	hasData    bool
	buildAt    time.Time
	dirty      atomic.Bool
	rebuilding atomic.Bool
	minRefresh time.Duration
	queries    *db.Queries
}

func NewStatsCache(queries *db.Queries, minRefresh time.Duration) *StatsCache {
	return &StatsCache{
		queries:    queries,
		minRefresh: minRefresh,
	}
}

// Invalidate marks the cache as dirty. Next Get will trigger an async rebuild
// if minRefresh has elapsed, while still serving stale data instantly.
func (c *StatsCache) Invalidate() {
	c.dirty.Store(true)
}

// InvalidateSync forces a synchronous rebuild of the cache.
// Use after mutations (create/delete) where the next read must see fresh data.
func (c *StatsCache) InvalidateSync(ctx context.Context) {
	result, err := buildResult(ctx, c.queries)
	if err != nil {
		slog.Error("sync cache rebuild failed", "error", err)
		c.dirty.Store(true)
		return
	}
	c.mu.Lock()
	c.data = result
	c.hasData = true
	c.buildAt = time.Now()
	c.dirty.Store(false)
	c.mu.Unlock()
}

// Get returns the cached project list. First call builds synchronously.
// Subsequent calls always return cached data immediately; if dirty and
// minRefresh has elapsed, an async rebuild is triggered in the background.
func (c *StatsCache) Get(ctx context.Context) ([]ProjectListItem, error) {
	c.mu.RLock()
	hasData := c.hasData
	data := c.data
	buildAt := c.buildAt
	c.mu.RUnlock()

	if !hasData {
		return c.buildSync(ctx)
	}

	// Stale-while-revalidate: serve cached, rebuild in background if needed
	if c.dirty.Load() && time.Since(buildAt) >= c.minRefresh {
		if c.rebuilding.CompareAndSwap(false, true) {
			go func() {
				defer c.rebuilding.Store(false)
				bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := c.buildAsync(bgCtx); err != nil {
					slog.Error("stats cache rebuild failed", "error", err)
				}
			}()
		}
	}

	return data, nil
}

func (c *StatsCache) buildSync(ctx context.Context) ([]ProjectListItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hasData {
		return c.data, nil
	}
	result, err := buildResult(ctx, c.queries)
	if err != nil {
		return nil, err
	}
	c.data = result
	c.hasData = true
	c.buildAt = time.Now()
	c.dirty.Store(false)
	return result, nil
}

func (c *StatsCache) buildAsync(ctx context.Context) error {
	result, err := buildResult(ctx, c.queries)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.data = result
	c.hasData = true
	c.buildAt = time.Now()
	c.dirty.Store(false)
	c.mu.Unlock()
	return nil
}

func buildResult(ctx context.Context, queries *db.Queries) ([]ProjectListItem, error) {
	projects, err := queries.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var (
		wg          sync.WaitGroup
		stats       []db.GetProjectStatsRow
		trendRows   []db.GetProjectEventTrendRow
		releaseRows []db.GetProjectLatestReleaseRow
		weeklyRows  []db.GetProjectWeeklyErrorsRow
		groups      []db.ProjectGroup
	)

	wg.Add(5)
	go func() { defer wg.Done(); stats, _ = queries.GetProjectStats(ctx) }()
	go func() { defer wg.Done(); trendRows, _ = queries.GetProjectEventTrend(ctx) }()
	go func() { defer wg.Done(); releaseRows, _ = queries.GetProjectLatestRelease(ctx) }()
	go func() { defer wg.Done(); weeklyRows, _ = queries.GetProjectWeeklyErrors(ctx) }()
	go func() { defer wg.Done(); groups, _ = queries.ListProjectGroups(ctx) }()
	wg.Wait()

	groupMap := make(map[uuid.UUID]string, len(groups))
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

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
		sp := toSafeProject(p)
		if p.GroupID.Valid {
			sp.GroupName = groupMap[p.GroupID.UUID]
		}
		item := ProjectListItem{SafeProject: sp, Trend: make([]int32, 14)}
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

	return result, nil
}
