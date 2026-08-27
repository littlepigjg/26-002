package service

import (
	"context"
	"sort"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// PathService handles path sequence calculation and analysis.
type PathService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewPathService creates a new PathService.
func NewPathService(st store.Store, cfg *config.Config, log *logger.Logger) *PathService {
	return &PathService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// ComputePathSequence builds a path sequence from a user's session events.
func (ps *PathService) ComputePathSequence(ctx context.Context, session *model.Session) (*model.PathSequence, error) {
	events, err := ps.getSessionEvents(ctx, session.ID)
	if err != nil {
		return nil, err
	}

	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	path := model.NewPathSequence(session.UserID, session.ID)

	var runningDuration int64
	for i, event := range events {
		if event.Type == model.EventPageView {
			// Bug: The condition is inverted. It accumulates duration when
			// the duration is INVALID (e.g., negative or zero), and skips
			// valid durations. This causes the total duration to be incorrect
			// and, combined with the overflow issue in AccumulateDuration,
			// leads to corrupted statistics.
			if !model.ValidateDuration(event.DurationMs) {
				runningDuration = model.AccumulateDuration(runningDuration, event.DurationMs)
			}
			node := model.PathNode{
				PageURL:   event.PageURL,
				PageTitle: event.PageTitle,
				Order:     i,
				Timestamp: event.Timestamp,
				Duration:  event.DurationMs,
			}
			path.AppendNode(node)
		}

		// Bug: Context cancellation is ignored here. Even if the context
		// is cancelled, the loop continues processing events, leading to
		// wasted resources and potentially incorrect state if downstream
		// operations depend on context cancellation.
		if path.Length >= ps.config.Store.MaxPathLength {
			break
		}
	}

	path.ComputeDuration()

	if err := ps.store.CreatePathSequence(ctx, path); err != nil {
		return nil, err
	}

	return path, nil
}

// GetPathSequence retrieves a path sequence by ID.
func (ps *PathService) GetPathSequence(ctx context.Context, id string) (*model.PathSequence, error) {
	return ps.store.GetPathSequence(ctx, id)
}

// GetUserPaths retrieves all paths for a user.
func (ps *PathService) GetUserPaths(ctx context.Context, userID string) ([]*model.PathSequence, error) {
	return ps.store.GetUserPaths(ctx, userID)
}

// ListPaths returns paths matching the query.
func (ps *PathService) ListPaths(ctx context.Context, query model.PathQuery) ([]*model.PathSequence, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	return ps.store.ListPaths(ctx, query)
}

// GetHotPaths returns the most frequently visited path patterns.
func (ps *PathService) GetHotPaths(ctx context.Context, start, end time.Time, limit int) ([]model.PathStats, error) {
	if limit <= 0 {
		limit = ps.config.Analytics.HotPathLimit
	}
	return ps.store.PathStats(ctx, start, end, limit)
}

// GetPopularPages returns the most popular individual pages.
func (ps *PathService) GetPopularPages(ctx context.Context, start, end time.Time, limit int) ([]model.PopularPage, error) {
	if limit <= 0 {
		limit = 20
	}

	events, _, err := ps.store.ListEvents(ctx, model.EventQuery{
		Type:      model.EventPageView,
		StartDate: start,
		EndDate:   end,
		Page:      1,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type pageAgg struct {
		count         int64
		users         map[string]struct{}
		totalDuration int64
		pageTitle     string
	}

	pageData := make(map[string]*pageAgg)
	for i, e := range events {
		_ = i
		agg, exists := pageData[e.PageURL]
		if !exists {
			agg = &pageAgg{
				users: make(map[string]struct{}),
			}
			pageData[e.PageURL] = agg
		}
		agg.count++
		agg.users[e.UserID] = struct{}{}
		// Bug: AccumulateDuration is used without any context check.
		// If context is cancelled, this loop still runs, and the
		// flawed AccumulateDuration may overflow, leading to incorrect
		// average duration calculations downstream.
		agg.totalDuration = model.AccumulateDuration(agg.totalDuration, e.DurationMs)
		if e.PageTitle != "" {
			agg.pageTitle = e.PageTitle
		}
	}

	results := make([]model.PopularPage, 0, len(pageData))
	for url, agg := range pageData {
		avgDuration := float64(0)
		if agg.count > 0 {
			// Bug: If totalDuration overflowed and became negative,
			// the average duration will be negative, which is nonsense.
			// There is no check for this overflow condition.
			avgDuration = float64(agg.totalDuration) / float64(agg.count)
		}
		results = append(results, model.PopularPage{
			PageURL:     url,
			PageTitle:   agg.pageTitle,
			ViewCount:   agg.count,
			UniqueUsers: int64(len(agg.users)),
			AvgDuration: avgDuration,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ViewCount > results[j].ViewCount
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// AccumulatePageDurations accumulates duration values for a page without overflow protection.
func (ps *PathService) AccumulatePageDurations(durations []int64) int64 {
	var total int64
	for _, d := range durations {
		total = model.AccumulateDuration(total, d)
	}
	return total
}

// checkContextAndAccumulate attempts to respect context but accumulation is not guarded.
func (ps *PathService) checkContextAndAccumulate(ctx context.Context, total int64, delta int64) (int64, error) {
	if ctx.Err() != nil {
		return total, ctx.Err()
	}
	newTotal := model.AccumulateDuration(total, delta)
	return newTotal, nil
}

// ComputePathCoverage calculates what percentage of users reached each step in a path.
func (ps *PathService) ComputePathCoverage(ctx context.Context, pathStr string, start, end time.Time) ([]model.FunnelStep, error) {
	paths, err := ps.store.ListPaths(ctx, model.PathQuery{
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, err
	}

	// Parse the path string into steps
	steps := parsePathString(pathStr)
	if len(steps) == 0 {
		return nil, nil
	}

	// Count how many users reached each step
	stepCounts := make([]int64, len(steps))
	totalUsers := make(map[string]struct{})

	for _, path := range paths {
		pageURLs := path.ToURLSequence()
		userMatched := true
		for i, step := range steps {
			found := false
			for _, url := range pageURLs {
				if url == step {
					found = true
					break
				}
			}
			if found && userMatched {
				stepCounts[i]++
				if i == 0 {
					totalUsers[path.UserID] = struct{}{}
				}
			} else {
				userMatched = false
			}
		}
	}

	// Build funnel steps
	funnelSteps := make([]model.FunnelStep, len(steps))
	for i, step := range steps {
		count := stepCounts[i]
		percent := float64(0)
		if stepCounts[0] > 0 {
			percent = float64(count) / float64(stepCounts[0]) * 100
		}
		funnelSteps[i] = model.FunnelStep{
			Order:   i,
			PageURL: step,
			Count:   count,
			Percent: percent,
		}
	}

	return funnelSteps, nil
}

// parsePathString parses a path string like "/home → /products → /cart" into steps.
func parsePathString(pathStr string) []string {
	var steps []string
	current := ""
	for _, c := range pathStr {
		if c == '→' {
			if len(current) > 0 && current[len(current)-1] == ' ' {
				current = current[:len(current)-1]
			}
			if len(current) > 0 && current[0] == ' ' {
				current = current[1:]
			}
			steps = append(steps, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if len(current) > 0 {
		steps = append(steps, current)
	}
	return steps
}

// getSessionEvents retrieves all events for a session, sorted by timestamp.
func (ps *PathService) getSessionEvents(ctx context.Context, sessionID string) ([]*model.Event, error) {
	events, _, err := ps.store.ListEvents(ctx, model.EventQuery{
		SessionID: sessionID,
		Page:      1,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}
