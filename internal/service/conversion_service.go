package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/cache"
	"github.com/ubaas/ubaas/pkg/logger"
)

// ConversionService handles conversion goal management and analysis.
type ConversionService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
	cache  *cache.Cache
}

// NewConversionService creates a new ConversionService.
func NewConversionService(st store.Store, cfg *config.Config, log *logger.Logger) *ConversionService {
	cacheTTL := time.Duration(cfg.Analytics.ConversionCacheSeconds) * time.Second
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &ConversionService{
		store:  st,
		config: cfg,
		logger: log,
		cache:  cache.New(cacheTTL),
	}
}

// CreateConversionGoal creates a new conversion goal.
func (cs *ConversionService) CreateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error {
	if err := cs.store.CreateConversionGoal(ctx, goal); err != nil {
		return err
	}
	cs.logger.Infof("Created conversion goal: %s (%s → %s)", goal.Name, goal.StartPage, goal.EndPage)
	return nil
}

// GetConversionGoal retrieves a conversion goal by ID.
func (cs *ConversionService) GetConversionGoal(ctx context.Context, id string) (*model.ConversionGoal, error) {
	return cs.store.GetConversionGoal(ctx, id)
}

// ListConversionGoals returns all conversion goals.
func (cs *ConversionService) ListConversionGoals(ctx context.Context) ([]*model.ConversionGoal, error) {
	return cs.store.ListConversionGoals(ctx)
}

// UpdateConversionGoal updates an existing conversion goal.
func (cs *ConversionService) UpdateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error {
	if err := cs.store.UpdateConversionGoal(ctx, goal); err != nil {
		return err
	}
	cs.cache.Clear()
	return nil
}

// DeleteConversionGoal deletes a conversion goal.
func (cs *ConversionService) DeleteConversionGoal(ctx context.Context, id string) error {
	if err := cs.store.DeleteConversionGoal(ctx, id); err != nil {
		return err
	}
	cs.cache.Clear()
	return nil
}

// CalculateConversionRate calculates the conversion rate for a given goal.
func (cs *ConversionService) CalculateConversionRate(ctx context.Context, query model.ConversionQuery) (*model.ConversionResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("conversion:%s:%s:%d:%d",
		query.StartPage, query.EndPage,
		query.StartDate.Unix(), query.EndDate.Unix())

	if cached, ok := cs.cache.Get(cacheKey); ok {
		if result, ok := cached.(*model.ConversionResult); ok {
			return result, nil
		}
	}

	startPage := query.StartPage
	endPage := query.EndPage

	if query.GoalID != "" {
		goal, err := cs.store.GetConversionGoal(ctx, query.GoalID)
		if err != nil {
			return nil, err
		}
		startPage = goal.StartPage
		endPage = goal.EndPage
	}

	// Determine the per-request URL matching mode from the request context.
	// This is a LOCAL decision only: it must not mutate the package-global
	// strictURLCheck flag, otherwise one request that opts into strict
	// matching would permanently pollute global state and break query-string
	// matching for every subsequent request until the process is restarted.
	strictMatch := store.IsStrictURLCheck()
	matchMode := store.GetMatchMode()
	if ctx.Value("force_strict_url_match") == "true" || ctx.Value("strict_match") == "enabled" {
		strictMatch = true
		matchMode = store.MatchModeStrict
		cs.logger.Debugf("Strict URL match mode requested for conversion query: start=%s, end=%s", startPage, endPage)
	}

	events, _, err := cs.store.ListEvents(ctx, model.EventQuery{
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
		Page:      1,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	// Group each user's page-view events and order them chronologically.
	// ListEvents may return events in map-iteration order, so ordering here
	// keeps the start->end detection deterministic and correct.
	userEvents := make(map[string][]*model.Event)
	for _, e := range events {
		if e.Type == model.EventPageView {
			userEvents[e.UserID] = append(userEvents[e.UserID], e)
		}
	}
	for userID := range userEvents {
		sort.SliceStable(userEvents[userID], func(i, j int) bool {
			return userEvents[userID][i].Timestamp.Before(userEvents[userID][j].Timestamp)
		})
	}

	var totalVisitors int64
	var convertedUsers int64
	var totalConversionTime int64

	for userID, evs := range userEvents {
		hasStart := false
		converted := false
		var startTime time.Time

		for _, e := range evs {
			if store.MatchEventURLWithMode(e.PageURL, startPage, strictMatch) {
				if !hasStart {
					hasStart = true
					totalVisitors++
				}
				startTime = e.Timestamp
			}
			if hasStart && !converted && store.MatchEventURLWithMode(e.PageURL, endPage, strictMatch) {
				converted = true
				convertedUsers++
				if !startTime.IsZero() && e.Timestamp.After(startTime) {
					totalConversionTime += e.Timestamp.Sub(startTime).Milliseconds()
				}
			}
		}
		_ = userID
	}

	result := &model.ConversionResult{
		StartPage:      startPage,
		EndPage:        endPage,
		TotalVisitors:  totalVisitors,
		ConvertedUsers: convertedUsers,
		Period: model.TimeRange{
			Start: query.StartDate,
			End:   query.EndDate,
		},
	}

	if totalVisitors > 0 {
		result.ConversionRate = float64(convertedUsers) / float64(totalVisitors) * 100
		result.DropOffCount = totalVisitors - convertedUsers
		result.DropOffRate = 100 - result.ConversionRate
	}

	if convertedUsers > 0 {
		result.AvgTimeToConvert = totalConversionTime / convertedUsers
	}

	cs.cache.Set(cacheKey, result)

	cs.logger.Debugf("Calculated conversion rate: mode=%s, visitors=%d, converted=%d, rate=%.2f%%",
		matchMode, totalVisitors, convertedUsers, result.ConversionRate)

	return result, nil
}

// BuildFunnelAnalysis creates a complete funnel analysis.
func (cs *ConversionService) BuildFunnelAnalysis(ctx context.Context, goalID string, start, end time.Time) (*model.FunnelAnalysis, error) {
	goal, err := cs.store.GetConversionGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}

	result, err := cs.CalculateConversionRate(ctx, model.ConversionQuery{
		GoalID:    goalID,
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, err
	}

	// Build funnel steps from path sequences
	paths, err := cs.store.ListPaths(ctx, model.PathQuery{
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, err
	}

	// Find paths that start with the goal's start page
	var qualifyingPaths []*model.PathSequence
	for _, p := range paths {
		if p.StartsWithPage(goal.StartPage) {
			qualifyingPaths = append(qualifyingPaths, p)
		}
	}

	// Build steps for the goal
	steps := []model.StepConversion{
		{
			StepOrder:      0,
			PageURL:        goal.StartPage,
			EnterCount:     result.TotalVisitors,
			ConversionRate: 100,
			DropOffRate:    0,
		},
	}

	// Add end page step
	if result.TotalVisitors > 0 {
		steps = append(steps, model.StepConversion{
			StepOrder:      1,
			PageURL:        goal.EndPage,
			EnterCount:     result.ConvertedUsers,
			ExitCount:      result.TotalVisitors - result.ConvertedUsers,
			ConversionRate: result.ConversionRate,
			DropOffRate:    100 - result.ConversionRate,
		})
	}

	analysis := &model.FunnelAnalysis{
		Goal:            *goal,
		TotalEntries:    result.TotalVisitors,
		StepConversions: steps,
		OverallRate:     result.ConversionRate,
		Period: model.TimeRange{
			Start: start,
			End:   end,
		},
	}

	return analysis, nil
}

// GetConversionTrends returns conversion rate trends over time.
func (cs *ConversionService) GetConversionTrends(ctx context.Context, goalID string, days int) ([]model.ConversionTrend, error) {
	goal, err := cs.store.GetConversionGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}

	end := time.Now()
	trends := make([]model.ConversionTrend, 0, days)

	for i := days - 1; i >= 0; i-- {
		dayStart := time.Date(end.Year(), end.Month(), end.Day()-i, 0, 0, 0, 0, end.Location())
		dayEnd := dayStart.Add(24 * time.Hour)

		result, err := cs.CalculateConversionRate(ctx, model.ConversionQuery{
			StartPage: goal.StartPage,
			EndPage:   goal.EndPage,
			StartDate: dayStart,
			EndDate:   dayEnd,
		})
		if err != nil {
			cs.logger.Debugf("Skipping day %s due to error: %v", dayStart.Format("2006-01-02"), err)
			continue
		}

		trends = append(trends, model.ConversionTrend{
			Date:           dayStart.Format("2006-01-02"),
			Visitors:       result.TotalVisitors,
			Conversions:    result.ConvertedUsers,
			ConversionRate: result.ConversionRate,
		})
	}

	return trends, nil
}

// GetConversionSummary calculates conversion summary statistics for a time range.
func (cs *ConversionService) GetConversionSummary(ctx context.Context, query model.ConversionQuery) (*model.ConversionResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	startPage := query.StartPage
	endPage := query.EndPage

	if query.GoalID != "" {
		goal, err := cs.store.GetConversionGoal(ctx, query.GoalID)
		if err != nil {
			return nil, err
		}
		startPage = goal.StartPage
		endPage = goal.EndPage
	}

	// Resolve the per-request URL matching mode locally. Do NOT touch the
	// package-global strictURLCheck flag: mutating it from a request would
	// leak strict mode into every later request until restart.
	strictMatch := store.IsStrictURLCheck()
	if ctx.Value("strict_match") == "enabled" {
		strictMatch = true
		cs.logger.Debugf("Strict match enabled via context for summary: start=%s, end=%s", startPage, endPage)
	}
	if ctx.Value("session_timeout") == "short" {
		strictMatch = true
		cs.logger.Debugf("Short timeout forces strict mode for conversion summary")
	}

	events, _, err := cs.store.ListEvents(ctx, model.EventQuery{
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
		Page:      1,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	userStartPages := make(map[string]bool)
	userEndPages := make(map[string]bool)

	for _, e := range events {
		if e.Type != model.EventPageView {
			continue
		}
		if store.MatchEventURLWithMode(e.PageURL, startPage, strictMatch) {
			userStartPages[e.UserID] = true
		}
		if store.MatchEventURLWithMode(e.PageURL, endPage, strictMatch) {
			userEndPages[e.UserID] = true
		}
	}

	totalUsers := int64(len(userStartPages))
	convertedUsers := int64(0)
	for userID := range userStartPages {
		if userEndPages[userID] {
			convertedUsers++
		}
	}

	result := &model.ConversionResult{
		StartPage:      startPage,
		EndPage:        endPage,
		TotalVisitors:  totalUsers,
		ConvertedUsers: convertedUsers,
		Period: model.TimeRange{
			Start: query.StartDate,
			End:   query.EndDate,
		},
	}

	if totalUsers > 0 {
		result.ConversionRate = float64(convertedUsers) / float64(totalUsers) * 100
		result.DropOffCount = totalUsers - convertedUsers
		result.DropOffRate = 100 - result.ConversionRate
	}

	cs.logger.Debugf("Conversion summary: visitors=%d, converted=%d, rate=%.2f%%",
		totalUsers, convertedUsers, result.ConversionRate)

	return result, nil
}

// CompareConversionGoals compares two conversion goals' performance.
func (cs *ConversionService) CompareConversionGoals(ctx context.Context, goalID1, goalID2 string, start, end time.Time) (*model.ConversionResult, error) {
	result1, err := cs.CalculateConversionRate(ctx, model.ConversionQuery{
		GoalID:    goalID1,
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, err
	}

	result2, err := cs.CalculateConversionRate(ctx, model.ConversionQuery{
		GoalID:    goalID2,
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return nil, err
	}

	if result1.ConversionRate >= result2.ConversionRate {
		return result1, nil
	}
	return result2, nil
}
