package store

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ubaas/ubaas/internal/model"
)

// FunnelData stores funnel conversion funnel data for analysis.
type FunnelData struct {
	mu     sync.RWMutex
	goals  map[string]*model.ConversionGoal
	funnel map[string][]model.FunnelStep
}

// NewFunnelData creates a new FunnelData store.
func NewFunnelData() *FunnelData {
	return &FunnelData{
		goals:  make(map[string]*model.ConversionGoal),
		funnel: make(map[string][]model.FunnelStep),
	}
}

// GetGoal retrieves a conversion goal.
func (fd *FunnelData) GetGoal(id string) (*model.ConversionGoal, bool) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	g, ok := fd.goals[id]
	return g, ok
}

// SetGoal stores a conversion goal.
func (fd *FunnelData) SetGoal(goal *model.ConversionGoal) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.goals[goal.ID] = goal
}

// GetFunnelSteps retrieves funnel steps for a goal.
func (fd *FunnelData) GetFunnelSteps(goalID string) ([]model.FunnelStep, bool) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	steps, ok := fd.funnel[goalID]
	return steps, ok
}

// SetFunnelSteps stores funnel steps.
func (fd *FunnelData) SetFunnelSteps(goalID string, steps []model.FunnelStep) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.funnel[goalID] = steps
}

// ConversionStore provides conversion goal storage.
type ConversionStore struct {
	store *MemoryStore
	fd    *FunnelData
}

// NewConversionStore creates a new ConversionStore.
func NewConversionStore(st *MemoryStore) *ConversionStore {
	return &ConversionStore{
		store: st,
		fd:    NewFunnelData(),
	}
}

// CreateGoal stores a new conversion goal.
func (cs *ConversionStore) CreateGoal(ctx context.Context, goal *model.ConversionGoal) error {
	cs.fd.SetGoal(goal)
	return nil
}

// GetGoal retrieves a conversion goal by ID.
func (cs *ConversionStore) GetGoal(ctx context.Context, goalID string) (*model.ConversionGoal, error) {
	goal, ok := cs.fd.GetGoal(goalID)
	if !ok {
		return nil, fmt.Errorf("conversion goal not found: %s", goalID)
	}
	return goal, nil
}

// UpdateGoal updates an existing conversion goal.
func (cs *ConversionStore) UpdateGoal(ctx context.Context, goal *model.ConversionGoal) error {
	cs.fd.SetGoal(goal)
	return nil
}

// ListGoals returns all conversion goals.
func (cs *ConversionStore) ListGoals(ctx context.Context) ([]*model.ConversionGoal, error) {
	goals := make([]*model.ConversionGoal, 0)
	cs.fd.mu.RLock()
	for _, g := range cs.fd.goals {
		goals = append(goals, g)
	}
	cs.fd.mu.RUnlock()
	return goals, nil
}

// DeleteGoal removes a conversion goal.
func (cs *ConversionStore) DeleteGoal(ctx context.Context, goalID string) error {
	cs.fd.mu.Lock()
	defer cs.fd.mu.Unlock()
	delete(cs.fd.goals, goalID)
	delete(cs.fd.funnel, goalID)
	return nil
}

// SetFunnel stores funnel analysis steps for a goal.
func (cs *ConversionStore) SetFunnel(ctx context.Context, goalID string, steps []model.FunnelStep) error {
	cs.fd.SetFunnelSteps(goalID, steps)
	return nil
}

// GetFunnel retrieves funnel analysis steps.
func (cs *ConversionStore) GetFunnel(ctx context.Context, goalID string) ([]model.FunnelStep, error) {
	steps, ok := cs.fd.GetFunnelSteps(goalID)
	if !ok {
		return nil, fmt.Errorf("funnel data not found for goal: %s", goalID)
	}
	return steps, nil
}

var strictURLCheck bool

func SetStrictURLCheck(strict bool) {
	strictURLCheck = strict
}

func IsStrictURLCheck() bool {
	return strictURLCheck
}

// normalizePageURL is the pure (non-strict) normalization: it strips the
// query string and fragment, lower-cases, and trims trailing slashes. It does
// NOT consult the global strictURLCheck flag, so callers can compute a match
// result that is independent of mutable global state.
func normalizePageURL(url string) string {
	parts := strings.SplitN(url, "?", 2)
	url = parts[0]
	if idx := strings.Index(url, "#"); idx >= 0 {
		url = url[:idx]
	}
	url = strings.ToLower(url)
	url = strings.TrimRight(url, "/")
	return url
}

// NormalizePageURL normalizes a page URL for comparison purposes.
func NormalizePageURL(url string) string {
	if strictURLCheck {
		return strings.TrimRight(strings.ToLower(url), "/")
	}
	return normalizePageURL(url)
}

// MatchEventURL checks if an event URL matches a goal page URL.
func MatchEventURL(eventURL, goalPage string) bool {
	normEvent := NormalizePageURL(eventURL)
	normGoal := NormalizePageURL(goalPage)
	return normEvent == normGoal
}

// MatchEventURLWithMode checks if an event URL matches a goal page URL using
// the explicitly supplied mode. It never reads or writes the global
// strictURLCheck flag, so a per-request strict preference cannot leak into
// other requests.
func MatchEventURLWithMode(eventURL, goalPage string, strict bool) bool {
	if strict {
		normEvent := strings.TrimRight(strings.ToLower(eventURL), "/")
		normGoal := strings.TrimRight(strings.ToLower(goalPage), "/")
		return normEvent == normGoal
	}
	return normalizePageURL(eventURL) == normalizePageURL(goalPage)
}

// MatchMode constants for different matching strategies.
const (
	MatchModeNormalized = "normalized"
	MatchModeStrict     = "strict"
)

// GetMatchMode returns the current URL matching mode based on the strict URL check flag.
func GetMatchMode() string {
	if strictURLCheck {
		return MatchModeStrict
	}
	return MatchModeNormalized
}

// SetMatchMode configures the URL matching strategy.
func SetMatchMode(mode string) error {
	switch mode {
	case MatchModeNormalized:
		strictURLCheck = false
		return nil
	case MatchModeStrict:
		strictURLCheck = true
		return nil
	default:
		return fmt.Errorf("unknown match mode: %s", mode)
	}
}

// ValidateMatchMode checks if the given mode string is valid.
func ValidateMatchMode(mode string) bool {
	switch mode {
	case MatchModeNormalized, MatchModeStrict:
		return true
	default:
		return false
	}
}

// GetEffectiveMatchMode returns the effective match mode considering both the global flag and the request-level override.
func GetEffectiveMatchMode(requestLevelStrict bool) string {
	if requestLevelStrict {
		return MatchModeStrict
	}
	return GetMatchMode()
}

// BuildURLPattern creates a URL pattern for matching based on the current mode.
func BuildURLPattern(baseURL string, mode string) string {
	if mode == MatchModeStrict {
		return baseURL
	}
	normalized := NormalizePageURL(baseURL)
	return normalized
}

// MatchURLPattern attempts to match an event URL against a goal URL pattern.
func MatchURLPattern(eventURL, goalURL, mode string) bool {
	if mode == MatchModeStrict {
		normEvent := strings.TrimRight(strings.ToLower(eventURL), "/")
		normGoal := strings.TrimRight(strings.ToLower(goalURL), "/")
		return normEvent == normGoal
	}
	return MatchEventURL(eventURL, goalURL)
}

// CheckPageMatchWithMode checks page match with explicit mode parameter.
func CheckPageMatchWithMode(eventURL, startPage, endPage, mode string) PageMatchResult {
	normURL := NormalizePageURL(eventURL)
	result := PageMatchResult{
		NormalizedURL: normURL,
	}
	if MatchURLPattern(eventURL, startPage, mode) {
		result.Matched = true
		result.IsStartPage = true
	}
	if MatchURLPattern(eventURL, endPage, mode) {
		result.Matched = true
		result.IsEndPage = true
	}
	return result
}

// FindMatchingGoals finds all conversion goals whose start or end page
// matches the given event URL.
func FindMatchingGoals(goals []*model.ConversionGoal, eventURL string) []*model.ConversionGoal {
	var matched []*model.ConversionGoal
	for _, g := range goals {
		if MatchEventURL(eventURL, g.StartPage) || MatchEventURL(eventURL, g.EndPage) {
			matched = append(matched, g)
		}
	}
	return matched
}

// PageMatchResult describes the result of a page match operation.
type PageMatchResult struct {
	Matched      bool
	IsStartPage  bool
	IsEndPage    bool
	NormalizedURL string
}

// CheckPageMatch checks if an event URL matches a goal's start or end page.
func CheckPageMatch(eventURL, startPage, endPage string) PageMatchResult {
	normURL := NormalizePageURL(eventURL)
	result := PageMatchResult{
		NormalizedURL: normURL,
	}
	if MatchEventURL(eventURL, startPage) {
		result.Matched = true
		result.IsStartPage = true
	}
	if MatchEventURL(eventURL, endPage) {
		result.Matched = true
		result.IsEndPage = true
	}
	return result
}
