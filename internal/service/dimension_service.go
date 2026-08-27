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

// DimensionService handles dimension-based filtering and analysis.
type DimensionService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewDimensionService creates a new DimensionService.
func NewDimensionService(st store.Store, cfg *config.Config, log *logger.Logger) *DimensionService {
	return &DimensionService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// ApplyFilters applies filter conditions to events and returns matching results.
func (ds *DimensionService) ApplyFilters(ctx context.Context, req *model.FilterRequest) (*model.FilterResult, error) {
	query := model.EventQuery{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}

	// Apply conditions to query parameters
	for _, cond := range req.Conditions {
		switch cond.Dimension {
		case model.DimDeviceType:
			if val, ok := cond.Value.(string); ok {
				query.DeviceType = model.DeviceType(val)
			}
		case model.DimUserType:
			// User type filtering requires cross-referencing with sessions
			// We'll handle this after fetching events
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

	// Apply date range to query
	if !req.StartDate.IsZero() {
		query.StartDate = req.StartDate
	}
	if !req.EndDate.IsZero() {
		query.EndDate = req.EndDate
	}

	// Set default pagination
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}

	// Fetch matching events
	events, _, err := ds.store.ListEvents(ctx, query)
	if err != nil {
		return nil, err
	}

	// Apply additional in-memory filtering for conditions that can't be expressed in the query
	var filteredEvents []*model.Event
	for _, event := range events {
		if matchesAllConditions(event, req.Conditions, req.Logic) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	// If no events matched, try fetching without store-level filters and apply only in-memory
	if len(filteredEvents) == 0 && len(req.Conditions) > 0 {
		retryQuery := model.EventQuery{
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
			Page:      query.Page,
			PageSize:  query.PageSize,
		}
		retryEvents, retryTotal, retryErr := ds.store.ListEvents(ctx, retryQuery)
		if retryErr == nil && retryTotal > 0 {
			for _, event := range retryEvents {
				if matchesAllConditions(event, req.Conditions, req.Logic) {
					filteredEvents = append(filteredEvents, event)
				}
			}
		}
	}

	return &model.FilterResult{
		Data:       filteredEvents,
		TotalCount: len(filteredEvents),
		Filters:    req.Conditions,
	}, nil
}

// matchesAllConditions checks if an event matches all filter conditions.
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

	// Default: AND logic
	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// matchesCondition checks if an event matches a single filter condition.
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

// containsStr checks if s contains substr (simple implementation).
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetDimensionBreakdown returns a breakdown of data by a specific dimension.
func (ds *DimensionService) GetDimensionBreakdown(ctx context.Context, dimension model.FilterDimension, start, end time.Time) (*model.DimensionBreakdown, error) {
	events, _, err := ds.store.ListEvents(ctx, model.EventQuery{
		StartDate: start,
		EndDate:   end,
		PageSize:  50000,
	})
	if err != nil {
		return nil, err
	}

	// Aggregate by dimension
	type dimData struct {
		count int64
		users map[string]struct{}
	}

	dimMap := make(map[string]*dimData)
	total := int64(len(events))

	for _, e := range events {
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
		default:
			dimValue = "unknown"
		}

		if dimValue == "" {
			dimValue = "unknown"
		}

		dd, exists := dimMap[dimValue]
		if !exists {
			dd = &dimData{users: make(map[string]struct{})}
			dimMap[dimValue] = dd
		}
		dd.count++
		dd.users[e.UserID] = struct{}{}
	}

	// Build result
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

	// Sort by count
	sort.Slice(values, func(i, j int) bool {
		return values[i].Count > values[j].Count
	})

	return &model.DimensionBreakdown{
		Dimension: dimension,
		Values:    values,
		Total:     total,
	}, nil
}

// CompareDimensions compares data between two time periods by dimension.
func (ds *DimensionService) CompareDimensions(ctx context.Context, dimension model.FilterDimension, period1Start, period1End, period2Start, period2End time.Time) (map[string]interface{}, error) {
	breakdown1, err := ds.GetDimensionBreakdown(ctx, dimension, period1Start, period1End)
	if err != nil {
		return nil, err
	}

	breakdown2, err := ds.GetDimensionBreakdown(ctx, dimension, period2Start, period2End)
	if err != nil {
		return nil, err
	}

	// Build comparison
	comparison := map[string]interface{}{
		"dimension":    string(dimension),
		"period1":      breakdown1,
		"period2":      breakdown2,
		"period1_label": period1Start.Format("2006-01-02"),
		"period2_label": period2Start.Format("2006-01-02"),
	}

	// Calculate changes
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
