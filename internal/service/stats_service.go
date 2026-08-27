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

// StatsService provides aggregated statistics and metrics.
type StatsService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewStatsService creates a new StatsService.
func NewStatsService(st store.Store, cfg *config.Config, log *logger.Logger) *StatsService {
	return &StatsService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// GetOverallStats returns overall application statistics.
func (ss *StatsService) GetOverallStats(ctx context.Context) (map[string]interface{}, error) {
	activeSessions, err := ss.store.ActiveSessionCount(ctx)
	if err != nil {
		return nil, err
	}

	totalEvents, err := ss.store.EventCountByType(ctx, "", time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}

	uniqueUsers, err := ss.store.UniqueUsersCount(ctx, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}

	// Get today's stats
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
	todayEnd := todayStart.Add(24 * time.Hour)

	todayEvents, err := ss.store.EventCountByType(ctx, "", todayStart, todayEnd)
	if err != nil {
		return nil, err
	}

	todayUsers, err := ss.store.UniqueUsersCount(ctx, todayStart, todayEnd)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"active_sessions":  activeSessions,
		"total_events":     totalEvents,
		"total_users":      uniqueUsers,
		"today_events":     todayEvents,
		"today_users":      todayUsers,
		"server_time":      time.Now().Format(time.RFC3339),
	}, nil
}

// GetEventBreakdown returns event counts by type for a time range.
func (ss *StatsService) GetEventBreakdown(ctx context.Context, start, end time.Time) (map[string]int64, error) {
	eventTypes := []model.EventType{
		model.EventPageView,
		model.EventClick,
		model.EventDuration,
		model.EventConversion,
		model.EventCustom,
	}

	breakdown := make(map[string]int64)
	for _, et := range eventTypes {
		count, err := ss.store.EventCountByType(ctx, et, start, end)
		if err != nil {
			continue
		}
		breakdown[string(et)] = count
	}

	return breakdown, nil
}

// GetPageStats returns statistics for individual pages.
func (ss *StatsService) GetPageStats(ctx context.Context, start, end time.Time, limit int) ([]model.PopularPage, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		Type:      model.EventPageView,
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	// Aggregate by page
	type pageData struct {
		count       int64
		users       map[string]struct{}
		totalDuration int64
		pageTitle   string
	}

	pages := make(map[string]*pageData)
	for _, e := range events {
		pd, exists := pages[e.PageURL]
		if !exists {
			pd = &pageData{users: make(map[string]struct{})}
			pages[e.PageURL] = pd
		}
		pd.count++
		pd.users[e.UserID] = struct{}{}
		pd.totalDuration += e.DurationMs
		if e.PageTitle != "" {
			pd.pageTitle = e.PageTitle
		}
	}

	results := make([]model.PopularPage, 0, len(pages))
	for url, pd := range pages {
		avgDuration := float64(0)
		if pd.count > 0 {
			avgDuration = float64(pd.totalDuration) / float64(pd.count)
		}
		results = append(results, model.PopularPage{
			PageURL:     url,
			PageTitle:   pd.pageTitle,
			ViewCount:   pd.count,
			UniqueUsers: int64(len(pd.users)),
			AvgDuration: avgDuration,
		})
	}

	// Sort by view count
	sort.Slice(results, func(i, j int) bool {
		return results[i].ViewCount > results[j].ViewCount
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetAverageDuration returns average stay duration by page.
func (ss *StatsService) GetAverageDuration(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		Type:      model.EventDuration,
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type durationAgg struct {
		total   int64
		count   int64
	}

	durations := make(map[string]*durationAgg)
	for _, e := range events {
		agg, exists := durations[e.PageURL]
		if !exists {
			agg = &durationAgg{}
			durations[e.PageURL] = agg
		}
		agg.total += e.DurationMs
		agg.count++
	}

	result := make(map[string]float64)
	for url, agg := range durations {
		if agg.count > 0 {
			result[url] = float64(agg.total) / float64(agg.count)
		}
	}

	return result, nil
}

// GetDeviceBreakdown returns event breakdown by device type.
func (ss *StatsService) GetDeviceBreakdown(ctx context.Context, start, end time.Time) (map[string]map[string]int64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	breakdown := make(map[string]map[string]int64)
	for _, e := range events {
		device := string(e.DeviceType)
		if device == "" {
			device = string(model.DeviceOther)
		}
		if breakdown[device] == nil {
			breakdown[device] = make(map[string]int64)
		}
		breakdown[device][string(e.Type)]++
	}

	return breakdown, nil
}

// GetHourlyDistribution returns event counts per hour of the day.
func (ss *StatsService) GetHourlyDistribution(ctx context.Context, start, end time.Time) (map[int]int64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		Type:      model.EventPageView,
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	hourly := make(map[int]int64, 24)
	for i := 0; i < 24; i++ {
		hourly[i] = 0
	}

	for _, e := range events {
		hourly[e.Timestamp.Hour()]++
	}

	return hourly, nil
}

// GetCountryBreakdown returns event breakdown by country.
func (ss *StatsService) GetCountryBreakdown(ctx context.Context, start, end time.Time) ([]model.DimensionValue, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type countryData struct {
		count int64
		users map[string]struct{}
	}

	countries := make(map[string]*countryData)
	total := int64(len(events))

	for _, e := range events {
		country := e.Country
		if country == "" {
			country = "unknown"
		}
		cd, exists := countries[country]
		if !exists {
			cd = &countryData{users: make(map[string]struct{})}
			countries[country] = cd
		}
		cd.count++
		cd.users[e.UserID] = struct{}{}
	}

	results := make([]model.DimensionValue, 0, len(countries))
	for country, cd := range countries {
		percent := float64(0)
		if total > 0 {
			percent = float64(cd.count) / float64(total) * 100
		}
		results = append(results, model.DimensionValue{
			Value:       country,
			Count:       cd.count,
			Percent:     percent,
			UniqueUsers: int64(len(cd.users)),
		})
	}

	// Sort by count
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	return results, nil
}
