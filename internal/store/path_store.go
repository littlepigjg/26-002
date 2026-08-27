package store

import (
	"context"
	"time"

	"github.com/ubaas/ubaas/internal/model"
)

// CreatePathSequence stores a new path sequence.
func (s *MemoryStore) CreatePathSequence(ctx context.Context, path *model.PathSequence) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	s.paths[path.ID] = path
	s.pathsByUser[path.UserID] = append(s.pathsByUser[path.UserID], path)
	return nil
}

// GetPathSequence retrieves a path sequence by ID.
func (s *MemoryStore) GetPathSequence(ctx context.Context, id string) (*model.PathSequence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, ok := s.paths[id]
	if !ok {
		return nil, model.ErrPathNotFound
	}
	return path, nil
}

// GetUserPaths retrieves all paths for a user.
func (s *MemoryStore) GetUserPaths(ctx context.Context, userID string) ([]*model.PathSequence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userPaths := s.pathsByUser[userID]
	if len(userPaths) == 0 {
		return []*model.PathSequence{}, nil
	}
	return userPaths, nil
}

// ListPaths returns path sequences matching the query.
func (s *MemoryStore) ListPaths(ctx context.Context, query model.PathQuery) ([]*model.PathSequence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isOpen {
		return nil, model.ErrStoreClosed
	}

	var results []*model.PathSequence
	for _, path := range s.paths {
		// Filter by date range
		if !query.StartDate.IsZero() && path.StartTime.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && path.StartTime.After(query.EndDate) {
			continue
		}

		// Filter by path length
		if query.MinLength > 0 && path.Length < query.MinLength {
			continue
		}
		if query.MaxLength > 0 && path.Length > query.MaxLength {
			continue
		}

		// Filter by start page
		if query.StartPage != "" && !path.StartsWithPage(query.StartPage) {
			continue
		}

		// Filter by end page
		if query.EndPage != "" && !path.EndsWithPage(query.EndPage) {
			continue
		}

		// Filter by device type and user type (using the first event's data)
		// These are approximate filters based on associated events
		if query.DeviceType != "" {
			hasMatchingEvent := false
			for _, event := range s.eventsByUser[path.UserID] {
				if event.DeviceType == query.DeviceType &&
					event.Timestamp.After(path.StartTime.Add(-time.Hour)) &&
					event.Timestamp.Before(path.EndTime.Add(time.Hour)) {
					hasMatchingEvent = true
					break
				}
			}
			if !hasMatchingEvent {
				continue
			}
		}

		results = append(results, path)

		// Apply limit
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

// PathStats returns aggregated path statistics for hot paths.
func (s *MemoryStore) PathStats(ctx context.Context, start, end time.Time, limit int) ([]model.PathStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Aggregate path counts by URL sequence pattern
	pathCounts := make(map[string]*struct {
		count       int64
		users       map[string]struct{}
		totalDuration int64
		firstSeen   time.Time
		lastSeen    time.Time
	})

	for _, path := range s.paths {
		if !start.IsZero() && path.StartTime.Before(start) {
			continue
		}
		if !end.IsZero() && path.StartTime.After(end) {
			continue
		}

		// Build path string
		pathStr := ""
		for i, node := range path.Nodes {
			if i > 0 {
				pathStr += " → "
			}
			pathStr += node.PageURL
		}

		stats, exists := pathCounts[pathStr]
		if !exists {
			stats = &struct {
				count       int64
				users       map[string]struct{}
				totalDuration int64
				firstSeen   time.Time
				lastSeen    time.Time
			}{
				users:     make(map[string]struct{}),
				firstSeen: path.StartTime,
				lastSeen:  path.EndTime,
			}
			pathCounts[pathStr] = stats
		}

		stats.count++
		stats.users[path.UserID] = struct{}{}
		stats.totalDuration += path.ComputeDuration()
		if path.StartTime.Before(stats.firstSeen) {
			stats.firstSeen = path.StartTime
		}
		if path.EndTime.After(stats.lastSeen) {
			stats.lastSeen = path.EndTime
		}
	}

	// Convert to result slice
	results := make([]model.PathStats, 0, len(pathCounts))
	for pathStr, stats := range pathCounts {
		avgDuration := float64(0)
		if stats.count > 0 {
			avgDuration = float64(stats.totalDuration) / float64(stats.count)
		}
		results = append(results, model.PathStats{
			Path:        pathStr,
			VisitCount:  stats.count,
			UniqueUsers: int64(len(stats.users)),
			AvgDuration: avgDuration,
			FirstSeen:   stats.firstSeen,
			LastSeen:    stats.lastSeen,
		})
	}

	// Sort by visit count descending and apply limit
	sortPathStatsByCount(results)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// sortPathStatsByCount sorts path stats by visit count descending (simple insertion sort).
func sortPathStatsByCount(stats []model.PathStats) {
	for i := 1; i < len(stats); i++ {
		key := stats[i]
		j := i - 1
		for j >= 0 && stats[j].VisitCount < key.VisitCount {
			stats[j+1] = stats[j]
			j--
		}
		stats[j+1] = key
	}
}
