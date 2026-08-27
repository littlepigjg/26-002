package service

import (
	"context"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/timeutil"
)

// QueryService provides complex query building and execution.
type QueryService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewQueryService creates a new QueryService.
func NewQueryService(st store.Store, cfg *config.Config, log *logger.Logger) *QueryService {
	return &QueryService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// QueryRequest represents a complex query request.
type QueryRequest struct {
	TimeRange  timeutil.TimeRange      `json:"time_range"`
	Dimensions []string                `json:"dimensions"`
	Metrics    []string                `json:"metrics"`
	Filters    []model.FilterCondition `json:"filters"`
	Limit      int                     `json:"limit"`
	Offset     int                     `json:"offset"`
}

// QueryResult contains the results of a query.
type QueryResult struct {
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	HasMore bool        `json:"has_more"`
}

// ExecuteQuery executes a complex analytical query.
func (qs *QueryService) ExecuteQuery(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 10000 {
		req.Limit = 10000
	}

	// Resolve time range
	var startDate, endDate time.Time
	if req.TimeRange != "" {
		tw, err := timeutil.ResolveTimeRange(req.TimeRange)
		if err != nil {
			return nil, err
		}
		startDate = tw.Start
		endDate = tw.End
	} else {
		endDate = time.Now()
		startDate = endDate.Add(-7 * 24 * time.Hour)
	}

	page := 1
	if req.Offset > 0 && req.Limit > 0 {
		page = (req.Offset / req.Limit) + 1
	}

	query := model.EventQuery{
		StartDate: startDate,
		EndDate:   endDate,
		PageSize:  req.Limit,
		Page:      page,
	}

	// Build store-level query filters from request filters
	for _, f := range req.Filters {
		if val, ok := f.Value.(string); ok {
			switch f.Dimension {
			case model.DimOS:
				query.OS = val
			case model.DimBrowser:
				query.Browser = val
			case model.DimCountry:
				query.Country = val
			case model.DimPage:
				query.PageURL = val
			case model.DimReferrer:
				query.Referrer = val
			case model.DimDeviceType:
				query.DeviceType = model.DeviceType(val)
			}
		}
	}

	events, total, err := qs.store.ListEvents(ctx, query)
	if err != nil {
		return nil, err
	}

	hasMore := (req.Offset+req.Limit) < total

	// Apply in-memory filters
	if len(req.Filters) > 0 {
		var filtered []*model.Event
		for _, e := range events {
			if queryMatchesFilters(e, req.Filters) {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	// If no events matched after filtering, try without store-level filters
	if len(events) == 0 && len(req.Filters) > 0 {
		retryQuery := model.EventQuery{
			StartDate: startDate,
			EndDate:   endDate,
			PageSize:  req.Limit,
			Page:      page,
		}
		retryEvents, _, retryErr := qs.store.ListEvents(ctx, retryQuery)
		if retryErr == nil {
			var filtered []*model.Event
			for _, e := range retryEvents {
				if queryMatchesFilters(e, req.Filters) {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}
	}

	return &QueryResult{
		Data:    events,
		Total:   int64(total),
		HasMore: hasMore,
	}, nil
}

// queryMatchesFilters checks if an event matches all filter conditions.
func queryMatchesFilters(event *model.Event, conditions []model.FilterCondition) bool {
	for _, cond := range conditions {
		if !queryMatchCondition(event, cond) {
			return false
		}
	}
	return true
}

// queryMatchCondition checks if a single condition matches.
func queryMatchCondition(event *model.Event, cond model.FilterCondition) bool {
	var value string
	switch cond.Dimension {
	case model.DimPage:
		value = event.PageURL
	case model.DimCountry:
		value = event.Country
	case model.DimDeviceType:
		value = string(event.DeviceType)
	case model.DimOS:
		value = event.OS
	case model.DimBrowser:
		value = event.Browser
	case model.DimUserType:
		value = "" // UserType is session-level, not event-level
	case model.DimReferrer:
		value = event.Referrer
	default:
		return false
	}

	filterVal, ok := cond.Value.(string)
	if !ok {
		return false
	}

	switch cond.Operator {
	case model.OpEqual:
		return value == filterVal
	case model.OpNotEqual:
		return value != filterVal
	case model.OpContains:
		return queryContainsStr(value, filterVal)
	case model.OpGreaterThan, model.OpLessThan:
		return false
	default:
		return false
	}
}

func queryContainsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TimeRangeQuery validates and processes time range queries.
func (qs *QueryService) TimeRangeQuery(ctx context.Context, rangeType string) (time.Time, time.Time, error) {
	tr := timeutil.TimeRange(rangeType)
	tw, err := timeutil.ResolveTimeRange(tr)
	if err != nil {
		// Fallback: custom ranges
		now := time.Now()
		switch rangeType {
		case "7d":
			return now.AddDate(0, 0, -7), now, nil
		case "30d":
			return now.AddDate(0, 0, -30), now, nil
		case "90d":
			return now.AddDate(0, 0, -90), now, nil
		default:
			return now.AddDate(0, 0, -7), now, nil
		}
	}
	return tw.Start, tw.End, nil
}
