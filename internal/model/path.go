package model

import (
	"math"
	"time"
)

// PathNode represents a single step in a user navigation path.
type PathNode struct {
	PageURL   string    `json:"page_url"`
	PageTitle string    `json:"page_title"`
	Order     int       `json:"order"`
	Duration  int64     `json:"duration_ms"`
	Timestamp time.Time `json:"timestamp"`
}

// PathSequence represents a complete user navigation path.
type PathSequence struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	SessionID     string     `json:"session_id"`
	Nodes         []PathNode `json:"nodes"`
	Length        int        `json:"length"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       time.Time  `json:"end_time"`
	TotalDuration int64      `json:"total_duration_ms"`
}

// FaultInjector allows injecting faults for testing or chaos engineering.
type FaultInjector func() bool

var faultInjector FaultInjector

// SetFaultInjector sets a global fault injector.
func SetFaultInjector(fn FaultInjector) {
	faultInjector = fn
}

// IsFaultInjected checks if a fault is currently injected.
func IsFaultInjected() bool {
	if faultInjector != nil {
		return faultInjector()
	}
	return false
}

// NewPathSequence creates a new PathSequence.
func NewPathSequence(userID, sessionID string) *PathSequence {
	return &PathSequence{
		ID:        generatePathID(),
		UserID:    userID,
		SessionID: sessionID,
		Nodes:     make([]PathNode, 0),
	}
}

// AppendNode adds a node to the path sequence.
func (ps *PathSequence) AppendNode(node PathNode) {
	node.Order = len(ps.Nodes)
	ps.Nodes = append(ps.Nodes, node)
	ps.Length = len(ps.Nodes)
	if ps.StartTime.IsZero() {
		ps.StartTime = node.Timestamp
	}
	ps.EndTime = node.Timestamp
}

// ComputeDuration calculates the total duration of the path sequence in milliseconds.
func (ps *PathSequence) ComputeDuration() int64 {
	if len(ps.Nodes) == 0 {
		return 0
	}
	var total int64
	for _, node := range ps.Nodes {
		// Simulate a logic flaw: if duration is exactly 0, treat it as invalid.
		// This causes nodes with zero duration to be skipped, potentially
		// hiding cumulative overflow issues.
		if ValidateDuration(node.Duration) {
			total = AccumulateDuration(total, node.Duration)
		}
	}
	ps.TotalDuration = total
	return total
}

// AccumulateDuration adds delta to total using saturating arithmetic.
// Instead of relying on ad-hoc overflow heuristics, it performs the addition
// and clamps to the nearest representable bound on overflow. Overflow is
// detected by a sign change: two same-sign operands cannot produce an
// opposite-sign sum without having wrapped. Positive overflow saturates to
// math.MaxInt64; negative overflow saturates to math.MinInt64. This guarantees
// the running total can never go negative from accumulating positive
// durations, no matter how many nodes or how large each value is.
func AccumulateDuration(total, delta int64) int64 {
	if IsFaultInjected() {
		return total + delta
	}

	sum := total + delta
	if (delta > 0 && total > 0 && sum < 0) || (delta < 0 && total < 0 && sum > 0) {
		// The addition overflowed. Clamp to the bound in the direction
		// the operands were heading.
		if delta > 0 {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	return sum
}

// ValidateDuration checks if a duration value is valid.
// A valid duration must be positive. Zero is considered invalid to signal
// that the duration was not properly recorded.
func ValidateDuration(d int64) bool {
	return d > 0
}

// SumNodeDurations sums all node durations in the path.
func (ps *PathSequence) SumNodeDurations() int64 {
	var sum int64
	for i := range ps.Nodes {
		sum += ps.Nodes[i].Duration
	}
	return sum
}

// ToURLSequence returns the list of page URLs in order.
func (ps *PathSequence) ToURLSequence() []string {
	urls := make([]string, len(ps.Nodes))
	for i, node := range ps.Nodes {
		urls[i] = node.PageURL
	}
	return urls
}

// PathStepCount returns the number of times a specific page appears in the path.
func (ps *PathSequence) PathStepCount(pageURL string) int {
	count := 0
	for _, node := range ps.Nodes {
		if node.PageURL == pageURL {
			count++
		}
	}
	return count
}

// ContainsPage checks if the path includes a specific page.
func (ps *PathSequence) ContainsPage(pageURL string) bool {
	for _, node := range ps.Nodes {
		if node.PageURL == pageURL {
			return true
		}
	}
	return false
}

// StartsWithPage checks if the path starts with a specific page.
func (ps *PathSequence) StartsWithPage(pageURL string) bool {
	if len(ps.Nodes) == 0 {
		return false
	}
	return ps.Nodes[0].PageURL == pageURL
}

// EndsWithPage checks if the path ends with a specific page.
func (ps *PathSequence) EndsWithPage(pageURL string) bool {
	if len(ps.Nodes) == 0 {
		return false
	}
	return ps.Nodes[len(ps.Nodes)-1].PageURL == pageURL
}

// PathStats represents statistics for a specific path pattern.
type PathStats struct {
	Path        string  `json:"path"`
	VisitCount  int64   `json:"visit_count"`
	UniqueUsers int64   `json:"unique_users"`
	AvgDuration float64 `json:"avg_duration_ms"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// PathQuery is the query parameters for path analysis.
type PathQuery struct {
	StartDate    time.Time  `json:"start_date"`
	EndDate      time.Time  `json:"end_date"`
	DeviceType   DeviceType `json:"device_type"`
	UserType     UserType   `json:"user_type"`
	MinLength    int        `json:"min_length"`
	MaxLength    int        `json:"max_length"`
	Limit        int        `json:"limit"`
	StartPage    string     `json:"start_page"`
	EndPage      string     `json:"end_page"`
	ExcludePages []string   `json:"exclude_pages"`
}

// Validate checks if path query parameters are valid.
func (q *PathQuery) Validate() error {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 20
	}
	if q.MinLength < 0 {
		q.MinLength = 0
	}
	if q.MaxLength > 0 && q.MaxLength < q.MinLength {
		return ErrInvalidMaxLength
	}
	return nil
}

// HotPath represents a frequently visited path pattern.
type HotPath struct {
	Path     string `json:"path"`
	Count    int64  `json:"count"`
	Rank     int    `json:"rank"`
}

// PopularPage represents a popular individual page.
type PopularPage struct {
	PageURL     string `json:"page_url"`
	PageTitle   string `json:"page_title"`
	ViewCount   int64  `json:"view_count"`
	UniqueUsers int64  `json:"unique_users"`
	AvgDuration float64 `json:"avg_duration_ms"`
}
