package model

import (
	"time"
)

// FunnelStep represents a step in a conversion funnel.
type FunnelStep struct {
	Order     int       `json:"order"`
	PageURL   string    `json:"page_url"`
	PageTitle string    `json:"page_title"`
	Count     int64     `json:"count"`
	Percent   float64   `json:"percent"`
}

// ConversionGoal defines a conversion goal with start and end pages.
type ConversionGoal struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	StartPage   string    `json:"start_page"`
	EndPage     string    `json:"end_page"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewConversionGoal creates a new ConversionGoal.
func NewConversionGoal(name, startPage, endPage string) *ConversionGoal {
	now := time.Now()
	return &ConversionGoal{
		ID:        generateGoalID(),
		Name:      name,
		StartPage: startPage,
		EndPage:   endPage,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ConversionResult represents the result of a conversion analysis.
type ConversionResult struct {
	GoalID          string    `json:"goal_id"`
	GoalName        string    `json:"goal_name"`
	StartPage       string    `json:"start_page"`
	EndPage         string    `json:"end_page"`
	TotalVisitors   int64     `json:"total_visitors"`
	ConvertedUsers  int64     `json:"converted_users"`
	ConversionRate  float64   `json:"conversion_rate"`
	DropOffCount    int64     `json:"drop_off_count"`
	DropOffRate     float64   `json:"drop_off_rate"`
	AvgTimeToConvert int64    `json:"avg_time_to_convert_ms"`
	Steps           []FunnelStep `json:"steps"`
	Period          TimeRange `json:"period"`
}

// ConversionQuery is the query parameters for conversion analysis.
type ConversionQuery struct {
	GoalID      string     `json:"goal_id"`
	StartPage   string     `json:"start_page"`
	EndPage     string     `json:"end_page"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	DeviceType  DeviceType `json:"device_type"`
	UserType    UserType   `json:"user_type"`
	MinDuration int64      `json:"min_duration_ms"`
}

// Validate checks if conversion query parameters are valid.
func (q *ConversionQuery) Validate() error {
	if q.StartPage == "" && q.GoalID == "" {
		return ErrMissingGoal
	}
	if q.StartDate.After(q.EndDate) && !q.EndDate.IsZero() {
		return ErrInvalidTimeRange
	}
	return nil
}

// TimeRange represents a time range for data analysis.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FunnelAnalysis represents a complete funnel analysis result.
type FunnelAnalysis struct {
	Goal             ConversionGoal `json:"goal"`
	TotalEntries     int64          `json:"total_entries"`
	StepConversions  []StepConversion `json:"step_conversions"`
	OverallRate      float64        `json:"overall_rate"`
	BiggestDropOff   int            `json:"biggest_drop_off_step"`
	Period           TimeRange      `json:"period"`
}

// StepConversion represents conversion metrics for a single funnel step.
type StepConversion struct {
	StepOrder     int     `json:"step_order"`
	PageURL       string  `json:"page_url"`
	EnterCount    int64   `json:"enter_count"`
	ExitCount     int64   `json:"exit_count"`
	ConversionRate float64 `json:"conversion_rate"`
	DropOffRate   float64 `json:"drop_off_rate"`
}

// ConversionTrend represents conversion rate over time.
type ConversionTrend struct {
	Date          string  `json:"date"`
	Visitors      int64   `json:"visitors"`
	Conversions   int64   `json:"conversions"`
	ConversionRate float64 `json:"conversion_rate"`
}
