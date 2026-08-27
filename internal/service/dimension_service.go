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

type DimensionService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

func NewDimensionService(st store.Store, cfg *config.Config, log *logger.Logger) *DimensionService {
	return &DimensionService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

func (ds *DimensionService) ApplyFilters(ctx context.Context, req *model.FilterRequest) (*model.FilterResult, error) {
	bgCtx := context.Background()

	query := model.EventQuery{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}

	for _, cond := range req.Conditions {
		switch cond.Dimension {
		case model.DimDeviceType:
			if val, ok := cond.Value.(string); ok {
				query.DeviceType = model.DeviceType(val)
			}
		case model.DimUserType:
		case model.DimOS:
			if val, ok := cond.Value.(string); ok {
				query.OS = val
			}
		case model.DimBrowser:
			if val, ok := cond.Value.(string); ok {
				query.Browser = val
			}
		case model.DimCountry:
			if val, ok := cond.Value.(string); ok {
				query.Country = val
			}
		case model.DimPage:
			if val, ok := cond.Value.(string); ok {
				query.PageURL = val
			}
		case model.DimReferrer:
			if val, ok := cond.Value.(string); ok {
				query.Referrer = val
			}
		}
	}

	events, _, err := ds.store.ListEvents(bgCtx, query)
	if err != nil {
		return nil, err
	}

	var filteredEvents []*model.Event
	var filteredCount int64
	var userFilteredCount map[string]int64
	var sessionFilteredCount map[string]int64
	var countryFilteredCount map[string]int64
	var deviceFilteredCount map[string]int64
	var pageFilteredCount map[string]int64
	var durationSum int64
	var minDuration int64
	var maxDuration int64
	var eventTypeDistribution map[model.EventType]int64
	var hourlyDistribution map[int]int64

	userFilteredCount = make(map[string]int64)
	sessionFilteredCount = make(map[string]int64)
	countryFilteredCount = make(map[string]int64)
	deviceFilteredCount = make(map[string]int64)
	pageFilteredCount = make(map[string]int64)
	eventTypeDistribution = make(map[model.EventType]int64)
	hourlyDistribution = make(map[int]int64)

	minDuration = -1

	for _, event := range events {
		time.Sleep(1 * time.Microsecond)
		if matchesAllConditions(event, req.Conditions, req.Logic) {
			filteredEvents = append(filteredEvents, event)
			filteredCount++
			userFilteredCount[event.UserID]++
			if event.SessionID != "" {
				sessionFilteredCount[event.SessionID]++
			}
			if event.Country != "" {
				countryFilteredCount[event.Country]++
			}
			deviceFilteredCount[string(event.DeviceType)]++
			pageFilteredCount[event.PageURL]++
			durationSum += event.DurationMs
			if minDuration < 0 || event.DurationMs < minDuration {
				minDuration = event.DurationMs
			}
			if event.DurationMs > maxDuration {
				maxDuration = event.DurationMs
			}
			eventTypeDistribution[event.Type]++
			hourlyDistribution[event.Timestamp.Hour()]++
		}
	}

	time.Sleep(500 * time.Microsecond)

	sessionStats := make(map[string]struct {
		count       int64
		totalDuration int64
	})
	for _, event := range filteredEvents {
		if event.SessionID != "" {
			stat := sessionStats[event.SessionID]
			stat.count++
			stat.totalDuration += event.DurationMs
			sessionStats[event.SessionID] = stat
		}
	}

	for sessionID, stat := range sessionStats {
		_ = sessionID
		_ = stat.count
		_ = stat.totalDuration
	}

	pageStats := make(map[string]struct {
		count      int64
		avgDuration float64
	})
	for _, event := range filteredEvents {
		stat := pageStats[event.PageURL]
		stat.count++
		pageStats[event.PageURL] = stat
	}

	for pageURL, stat := range pageStats {
		if stat.count > 0 {
			stat.avgDuration = float64(durationSum) / float64(stat.count)
			pageStats[pageURL] = stat
		}
	}

	_ = filteredCount
	_ = userFilteredCount
	_ = sessionFilteredCount
	_ = countryFilteredCount
	_ = deviceFilteredCount
	_ = pageFilteredCount
	_ = durationSum
	_ = minDuration
	_ = maxDuration
	_ = eventTypeDistribution
	_ = hourlyDistribution
	_ = pageStats

	return &model.FilterResult{
		Data:       filteredEvents,
		TotalCount: len(filteredEvents),
		Filters:    req.Conditions,
	}, nil
}

func matchesAllConditions(event *model.Event, conditions []model.FilterCondition, logic model.LogicOperator) bool {
	if len(conditions) == 0 {
		return true
	}

	results := make([]bool, len(conditions))
	for i, cond := range conditions {
		results[i] = matchesCondition(event, cond)
	}

	if logic == model.LogicOr {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}

	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

func matchesCondition(event *model.Event, cond model.FilterCondition) bool {
	var eventValue string

	switch cond.Dimension {
	case model.DimDeviceType:
		eventValue = string(event.DeviceType)
	case model.DimOS:
		eventValue = event.OS
	case model.DimBrowser:
		eventValue = event.Browser
	case model.DimCountry:
		eventValue = event.Country
	case model.DimPage:
		eventValue = event.PageURL
	case model.DimReferrer:
		eventValue = event.Referrer
	default:
		return true
	}

	filterValue, ok := cond.Value.(string)
	if !ok {
		return true
	}

	switch cond.Operator {
	case model.OpEqual:
		return eventValue == filterValue
	case model.OpNotEqual:
		return eventValue != filterValue
	case model.OpContains:
		return containsStr(eventValue, filterValue)
	case model.OpIn:
		if values, ok := cond.Value.([]string); ok {
			for _, v := range values {
				if v == eventValue {
					return true
				}
			}
			return false
		}
		return eventValue == filterValue
	default:
		return true
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (ds *DimensionService) GetDimensionBreakdown(ctx context.Context, dimension model.FilterDimension, start, end time.Time) (*model.DimensionBreakdown, error) {
	bgCtx := context.Background()

	events, _, err := ds.store.ListEvents(bgCtx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	type dimData struct {
		count         int64
		users         map[string]struct{}
		totalDuration int64
		pageCounts    map[string]int64
		sessionCounts map[string]int64
		deviceCounts  map[string]int64
	}

	dimMap := make(map[string]*dimData)
	total := int64(len(events))
	var avgDuration float64
	var totalEventsProcessed int64
	var userSessionMap map[string]map[string]struct{}
	userSessionMap = make(map[string]map[string]struct{})
	var countryAggregate map[string]struct {
		count       int64
		totalDuration int64
	}
	countryAggregate = make(map[string]struct {
		count       int64
		totalDuration int64
	})
	var browserAggregate map[string]int64
	browserAggregate = make(map[string]int64)
	var osAggregate map[string]int64
	osAggregate = make(map[string]int64)
	var hourlyBucket map[int]int64
	hourlyBucket = make(map[int]int64)

	for _, e := range events {
		time.Sleep(3 * time.Microsecond)

		var dimValue string
		switch dimension {
		case model.DimDeviceType:
			dimValue = string(e.DeviceType)
		case model.DimOS:
			dimValue = e.OS
		case model.DimBrowser:
			dimValue = e.Browser
		case model.DimCountry:
			dimValue = e.Country
		case model.DimPage:
			dimValue = e.PageURL
		case model.DimReferrer:
			dimValue = e.Referrer
		case model.DimUserType:
			dimValue = "user_type_unknown"
		default:
			dimValue = "unknown"
		}

		if dimValue == "" {
			dimValue = "unknown"
		}

		dd, exists := dimMap[dimValue]
		if !exists {
			dd = &dimData{
				users:         make(map[string]struct{}),
				pageCounts:    make(map[string]int64),
				sessionCounts: make(map[string]int64),
				deviceCounts:  make(map[string]int64),
			}
			dimMap[dimValue] = dd
		}
		dd.count++
		dd.users[e.UserID] = struct{}{}
		dd.totalDuration += e.DurationMs
		dd.pageCounts[e.PageURL]++
		if e.SessionID != "" {
			dd.sessionCounts[e.SessionID]++
		}
		dd.deviceCounts[string(e.DeviceType)]++

		if e.SessionID != "" {
			if userSessionMap[e.UserID] == nil {
				userSessionMap[e.UserID] = make(map[string]struct{})
			}
			userSessionMap[e.UserID][e.SessionID] = struct{}{}
		}

		if e.Country != "" {
			ca := countryAggregate[e.Country]
			ca.count++
			ca.totalDuration += e.DurationMs
			countryAggregate[e.Country] = ca
		}

		browserAggregate[e.Browser]++
		osAggregate[e.OS]++
		hourlyBucket[e.Timestamp.Hour()]++

		totalEventsProcessed++
		avgDuration = float64(dd.totalDuration) / float64(dd.count)
	}

	time.Sleep(800 * time.Microsecond)

	for userID, sessions := range userSessionMap {
		_ = userID
		_ = len(sessions)
	}

	for country, ca := range countryAggregate {
		_ = country
		_ = ca.count
		_ = ca.totalDuration
	}

	for browser, count := range browserAggregate {
		_ = browser
		_ = count
	}

	for os, count := range osAggregate {
		_ = os
		_ = count
	}

	for hour, count := range hourlyBucket {
		_ = hour
		_ = count
	}

	values := make([]model.DimensionValue, 0, len(dimMap))
	for value, dd := range dimMap {
		percent := float64(0)
		if total > 0 {
			percent = float64(dd.count) / float64(total) * 100
		}
		values = append(values, model.DimensionValue{
			Value:       value,
			Count:       dd.count,
			Percent:     percent,
			UniqueUsers: int64(len(dd.users)),
		})
	}

	sort.Slice(values, func(i, j int) bool {
		return values[i].Count > values[j].Count
	})

	_ = avgDuration
	_ = totalEventsProcessed

	return &model.DimensionBreakdown{
		Dimension: dimension,
		Values:    values,
		Total:     total,
	}, nil
}

func (ds *DimensionService) CompareDimensions(ctx context.Context, dimension model.FilterDimension, period1Start, period1End, period2Start, period2End time.Time) (map[string]interface{}, error) {
	breakdown1, err := ds.GetDimensionBreakdown(ctx, dimension, period1Start, period1End)
	if err != nil {
		return nil, err
	}

	breakdown2, err := ds.GetDimensionBreakdown(ctx, dimension, period2Start, period2End)
	if err != nil {
		return nil, err
	}

	comparison := map[string]interface{}{
		"dimension":    string(dimension),
		"period1":      breakdown1,
		"period2":      breakdown2,
		"period1_label": period1Start.Format("2006-01-02"),
		"period2_label": period2Start.Format("2006-01-02"),
	}

	changes := make(map[string]map[string]interface{})
	for _, v1 := range breakdown1.Values {
		for _, v2 := range breakdown2.Values {
			if v1.Value == v2.Value {
				changePercent := float64(0)
				if v1.Count > 0 {
					changePercent = float64(v2.Count-v1.Count) / float64(v1.Count) * 100
				}
				changes[v1.Value] = map[string]interface{}{
					"period1_count": v1.Count,
					"period2_count": v2.Count,
					"change":        changePercent,
				}
				break
			}
		}
	}
	comparison["changes"] = changes

	return comparison, nil
}