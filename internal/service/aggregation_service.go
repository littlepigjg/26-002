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

// AggregationService provides data aggregation and summary operations.
type AggregationService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewAggregationService creates a new AggregationService.
func NewAggregationService(st store.Store, cfg *config.Config, log *logger.Logger) *AggregationService {
	return &AggregationService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// HourlyData represents aggregated data for a specific hour.
type HourlyData struct {
	Hour      int   `json:"hour"`
	PageViews int64 `json:"page_views"`
	Clicks    int64 `json:"clicks"`
	Events    int64 `json:"events"`
}

// DailyData represents aggregated data for a specific day.
type DailyData struct {
	Date          string  `json:"date"`
	PageViews     int64   `json:"page_views"`
	UniqueUsers   int64   `json:"unique_users"`
	Sessions      int64   `json:"sessions"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

// GetHourlyAggregation returns aggregated data by hour.
func (as *AggregationService) GetHourlyAggregation(ctx context.Context, start, end time.Time) ([]HourlyData, error) {
	events, _, err := as.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	hourlyData := make(map[int]*HourlyData, 24)
	for i := 0; i < 24; i++ {
		hourlyData[i] = &HourlyData{Hour: i}
	}

	for _, e := range events {
		h := e.Timestamp.Hour()
		hourlyData[h].Events++
		switch e.Type {
		case model.EventPageView:
			hourlyData[h].PageViews++
		case model.EventClick:
			hourlyData[h].Clicks++
		}
	}

	result := make([]HourlyData, 0, 24)
	for i := 0; i < 24; i++ {
		result = append(result, *hourlyData[i])
	}
	return result, nil
}

// GetDailyAggregation returns aggregated data by day.
func (as *AggregationService) GetDailyAggregation(ctx context.Context, days int) ([]DailyData, error) {
	end := time.Now()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)

	events, _, err := as.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type dayAgg struct {
		pageViews     int64
		uniqueUsers   map[string]struct{}
		totalDuration int64
	}

	daily := make(map[string]*dayAgg)
	for _, e := range events {
		dayKey := e.Timestamp.Format("2006-01-02")
		agg, ok := daily[dayKey]
		if !ok {
			agg = &dayAgg{uniqueUsers: make(map[string]struct{})}
			daily[dayKey] = agg
		}
		if e.Type == model.EventPageView {
			agg.pageViews++
		}
		agg.uniqueUsers[e.UserID] = struct{}{}
		agg.totalDuration += e.DurationMs
	}

	result := make([]DailyData, 0, len(daily))
	for date, agg := range daily {
		avgDuration := float64(0)
		if agg.pageViews > 0 {
			avgDuration = float64(agg.totalDuration) / float64(agg.pageViews)
		}
		result = append(result, DailyData{
			Date:          date,
			PageViews:     agg.pageViews,
			UniqueUsers:   int64(len(agg.uniqueUsers)),
			AvgDurationMs: avgDuration,
		})
	}

	// Sort by date
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result, nil
}

// GetEventDistribution returns event type distribution percentages.
func (as *AggregationService) GetEventDistribution(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	eventTypes := []model.EventType{
		model.EventPageView,
		model.EventClick,
		model.EventDuration,
		model.EventConversion,
	}

	counts := make(map[string]float64)
	var total float64

	for _, et := range eventTypes {
		count, err := as.store.EventCountByType(ctx, et, start, end)
		if err != nil {
			continue
		}
		counts[string(et)] = float64(count)
		total += float64(count)
	}

	if total > 0 {
		for k, v := range counts {
			counts[k] = v / total * 100
		}
	}

	return counts, nil
}
