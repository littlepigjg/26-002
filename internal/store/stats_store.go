package store

import (
	"context"
	"time"

	"github.com/ubaas/ubaas/internal/model"
)

// StatsStore provides aggregated statistics storage and querying.
type StatsStore struct {
	store *MemoryStore
}

// NewStatsStore creates a new StatsStore.
func NewStatsStore(st *MemoryStore) *StatsStore {
	return &StatsStore{store: st}
}

// GetEventCountsByType returns counts for each event type in a time range.
func (ss *StatsStore) GetEventCountsByType(ctx context.Context, start, end time.Time) (map[model.EventType]int64, error) {
	eventTypes := []model.EventType{
		model.EventPageView,
		model.EventClick,
		model.EventDuration,
		model.EventConversion,
		model.EventCustom,
	}

	counts := make(map[model.EventType]int64)
	for _, et := range eventTypes {
		count, err := ss.store.EventCountByType(ctx, et, start, end)
		if err != nil {
			continue
		}
		counts[et] = count
	}
	return counts, nil
}

// GetDeviceTypeCounts returns event counts broken down by device type.
func (ss *StatsStore) GetDeviceTypeCounts(ctx context.Context, start, end time.Time) (map[model.DeviceType]int64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	counts := make(map[model.DeviceType]int64)
	for _, e := range events {
		dt := e.DeviceType
		if dt == "" {
			dt = model.DeviceOther
		}
		counts[dt]++
	}
	return counts, nil
}

// GetCountryCounts returns event counts broken down by country.
func (ss *StatsStore) GetCountryCounts(ctx context.Context, start, end time.Time) (map[string]int64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, e := range events {
		country := e.Country
		if country == "" {
			country = "unknown"
		}
		counts[country]++
	}
	return counts, nil
}

// GetHourlyCounts returns event counts broken down by hour of the day.
func (ss *StatsStore) GetHourlyCounts(ctx context.Context, start, end time.Time) (map[int]int64, error) {
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

// GetPageDurationStats returns average duration for each page.
func (ss *StatsStore) GetPageDurationStats(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	events, _, err := ss.store.ListEvents(ctx, model.EventQuery{
		Type:      model.EventDuration,
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type agg struct {
		total int64
		count int64
	}
	pageData := make(map[string]*agg)
	for _, e := range events {
		a, ok := pageData[e.PageURL]
		if !ok {
			a = &agg{}
			pageData[e.PageURL] = a
		}
		a.total += e.DurationMs
		a.count++
	}

	result := make(map[string]float64)
	for url, a := range pageData {
		if a.count > 0 {
			result[url] = float64(a.total) / float64(a.count)
		}
	}
	return result, nil
}
