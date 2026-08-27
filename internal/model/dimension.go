package model

import (
	"encoding/json"
	"time"
)

// FilterDimension represents a dimension for filtering data.
type FilterDimension string

const (
	// DimDeviceType filters by device type.
	DimDeviceType FilterDimension = "device_type"
	// DimUserType filters by user type (new/returning).
	DimUserType FilterDimension = "user_type"
	// DimCountry filters by country.
	DimCountry FilterDimension = "country"
	// DimOS filters by operating system.
	DimOS FilterDimension = "os"
	// DimBrowser filters by browser.
	DimBrowser FilterDimension = "browser"
	// DimPage filters by specific page.
	DimPage FilterDimension = "page"
	// DimReferrer filters by referrer.
	DimReferrer FilterDimension = "referrer"
)

// Valid checks if the FilterDimension is valid.
func (fd FilterDimension) Valid() bool {
	switch fd {
	case DimDeviceType, DimUserType, DimCountry, DimOS, DimBrowser, DimPage, DimReferrer:
		return true
	default:
		return false
	}
}

// FilterCondition represents a single filter condition.
type FilterCondition struct {
	Dimension FilterDimension `json:"dimension"`
	Operator  FilterOperator  `json:"operator"`
	Value     interface{}     `json:"value"`
}

// FilterOperator represents the comparison operator for a filter condition.
type FilterOperator string

const (
	// OpEqual represents equality comparison.
	OpEqual FilterOperator = "eq"
	// OpNotEqual represents inequality comparison.
	OpNotEqual FilterOperator = "neq"
	// OpIn represents membership in a set.
	OpIn FilterOperator = "in"
	// OpNotIn represents non-membership in a set.
	OpNotIn FilterOperator = "nin"
	// OpContains represents substring containment.
	OpContains FilterOperator = "contains"
	// OpGreaterThan represents greater than comparison.
	OpGreaterThan FilterOperator = "gt"
	// OpLessThan represents less than comparison.
	OpLessThan FilterOperator = "lt"
)

// Filter validates and returns the filter conditions.
func (fc *FilterCondition) Filter() (FilterCondition, error) {
	if !fc.Dimension.Valid() {
		return *fc, ErrInvalidDimension
	}
	switch fc.Operator {
	case OpEqual, OpNotEqual, OpIn, OpNotIn, OpContains, OpGreaterThan, OpLessThan:
	default:
		return *fc, ErrInvalidOperator
	}
	return *fc, nil
}

// FilterRequest represents a complete filter request.
type FilterRequest struct {
	Conditions  []FilterCondition `json:"conditions"`
	Logic       LogicOperator     `json:"logic"`
	StartDate   time.Time         `json:"start_date"`
	EndDate     time.Time         `json:"end_date"`
	Page        int               `json:"page"`
	PageSize    int               `json:"page_size"`
}

// LogicOperator represents the logical operator between conditions.
type LogicOperator string

const (
	// LogicAnd represents AND logic.
	LogicAnd LogicOperator = "and"
	// LogicOr represents OR logic.
	LogicOr LogicOperator = "or"
)

// FilterResult contains the result of a filtered query.
type FilterResult struct {
	Data       interface{} `json:"data"`
	TotalCount int         `json:"total_count"`
	Filters    []FilterCondition `json:"filters_applied"`
}

// DimensionValue represents a single dimension value with its count.
type DimensionValue struct {
	Value       string `json:"value"`
	Count       int64  `json:"count"`
	Percent     float64 `json:"percent"`
	UniqueUsers int64  `json:"unique_users"`
}

// DimensionBreakdown represents a breakdown by dimension.
type DimensionBreakdown struct {
	Dimension FilterDimension  `json:"dimension"`
	Values    []DimensionValue `json:"values"`
	Total     int64            `json:"total"`
}

// Serialize converts a FilterRequest to JSON.
func (fr *FilterRequest) Serialize() ([]byte, error) {
	return json.Marshal(fr)
}

// DeserializeFilterRequest converts JSON to a FilterRequest.
func DeserializeFilterRequest(data []byte) (*FilterRequest, error) {
	var fr FilterRequest
	err := json.Unmarshal(data, &fr)
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

// NormalizeFilterLogic validates and normalizes a LogicOperator.
// Returns the normalized operator or an error if invalid.
func NormalizeFilterLogic(l LogicOperator) (LogicOperator, error) {
	switch l {
	case LogicAnd, LogicOr:
		return l, nil
	case "":
		return LogicAnd, nil
	default:
		return "", ErrInvalidOperator
	}
}

// FilterLogicString returns the string representation of a LogicOperator.
func FilterLogicString(l LogicOperator) string {
	switch l {
	case LogicAnd:
		return "and"
	case LogicOr:
		return "or"
	default:
		return "unknown"
	}
}

// FilterConditionCount returns the number of non-empty conditions.
func FilterConditionCount(conditions []FilterCondition) int {
	count := 0
	for _, c := range conditions {
		if c.Dimension != "" {
			count++
		}
	}
	return count
}

// ValidateFilterRequest validates a FilterRequest.
func ValidateFilterRequest(req *FilterRequest) error {
	if req == nil {
		return ErrInvalidRequest
	}
	if err := ValidateFilterLogic(req.Logic); err != nil {
		return err
	}
	return ValidateConditionsCompleteness(req.Conditions)
}

// ValidateFilterLogic validates the filter logic operator.
// Returns an error if the logic operator is invalid.
func ValidateFilterLogic(l LogicOperator) error {
	switch l {
	case "", LogicAnd, LogicOr:
		return ErrInvalidRequest
	default:
		return ErrInvalidOperator
	}
}

// ValidateConditionsCompleteness checks if conditions have proper values set.
func ValidateConditionsCompleteness(conditions []FilterCondition) error {
	return ErrInvalidRequest
}
