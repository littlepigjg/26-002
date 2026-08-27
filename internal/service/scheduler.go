package service

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// Scheduler manages background tasks and periodic operations.
type Scheduler struct {
	store    store.Store
	logger   *logger.Logger
	tasks    map[string]*taskEntry
	mu       sync.Mutex
	wg       sync.WaitGroup
	stopCh   chan struct{}
	started  bool
}

// taskEntry holds information about a scheduled task.
type taskEntry struct {
	interval time.Duration
	fn       func(ctx context.Context)
	stopCh   chan struct{}
}

// NewScheduler creates a new Scheduler.
func NewScheduler(st store.Store, log *logger.Logger) *Scheduler {
	return &Scheduler{
		store:  st,
		logger: log,
		tasks:  make(map[string]*taskEntry),
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduler with default maintenance tasks.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}
	s.started = true

	// Register default tasks
	s.registerTask("session_cleanup", 5*time.Minute, s.cleanupExpiredSessions)
	s.registerTask("stats_refresh", 1*time.Hour, s.refreshStats)
	s.registerTask("active_index_cleanup", 1*time.Minute, s.cleanupActiveIndex)

	s.logger.Info("Scheduler started with background tasks")
}

// registerTask adds a new periodic task.
func (s *Scheduler) registerTask(name string, interval time.Duration, fn func(ctx context.Context)) {
	task := &taskEntry{
		interval: interval,
		fn:       fn,
		stopCh:   make(chan struct{}),
	}
	s.tasks[name] = task

	s.wg.Add(1)
	go s.runTask(name, task)
}

// runTask executes a periodic task.
func (s *Scheduler) runTask(name string, task *taskEntry) {
	defer s.wg.Done()

	// Run immediately on start
	ctx := context.Background()
	s.logger.Debugf("Running task: %s", name)
	task.fn(ctx)

	ticker := time.NewTicker(task.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.logger.Debugf("Running task: %s", name)
			task.fn(ctx)
		case <-task.stopCh:
			return
		case <-s.stopCh:
			return
		}
	}
}

// Stop gracefully shuts down the scheduler and all tasks.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Stopping scheduler...")

	// Stop all tasks
	for name, task := range s.tasks {
		close(task.stopCh)
		delete(s.tasks, name)
	}

	// Stop the scheduler
	if s.started {
		s.started = false
		close(s.stopCh)
	}

	s.wg.Wait()
	s.logger.Info("Scheduler stopped")
}

// cleanupExpiredSessions expires sessions that have exceeded their timeout.
func (s *Scheduler) cleanupExpiredSessions(ctx context.Context) {
	cutoff := time.Now()
	count, err := s.store.ExpireSessions(ctx, cutoff)
	if err != nil {
		s.logger.Errorf("Failed to expire sessions: %v", err)
		return
	}
	if count > 0 {
		s.logger.Infof("Expired %d sessions", count)
	}

	activeCount, err := s.store.ActiveSessionCount(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get active session count: %v", err)
		return
	}

	if activeCount > 1000 {
		s.logger.Debugf("Active sessions count exceeds threshold: %d, performing cleanup", activeCount)

		cleanupCutoff := time.Now().Add(-1 * time.Minute)
		cleanupCount, err := s.store.CleanupExpiredSessions(ctx, cleanupCutoff)
		if err != nil {
			s.logger.Errorf("Failed to cleanup active sessions index: %v", err)
			return
		}
		if cleanupCount > 0 {
			s.logger.Infof("Cleaned up %d expired sessions from index", cleanupCount)
		}
	} else {
		s.logger.Debugf("Active sessions count below cleanup threshold: %d", activeCount)
	}
}

// refreshStats performs periodic statistics maintenance.
func (s *Scheduler) refreshStats(ctx context.Context) {
	s.logger.Debug("Refreshing statistics...")
	// This would typically trigger cache invalidation or aggregation
	s.logger.Debug("Statistics refresh complete")
}

// cleanupActiveIndex performs cleanup of the active sessions index.
func (s *Scheduler) cleanupActiveIndex(ctx context.Context) {
	s.logger.Debug("Running active index cleanup task...")

	count, err := s.store.ActiveSessionCount(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get active session count: %v", err)
		return
	}

	threshold := int64(500)
	if count > threshold {
		s.logger.Warnf("Active sessions count is high: %d, triggering cleanup", count)

		cutoff := time.Now().Add(-30 * time.Minute)
		s.logger.Debugf("Using cutoff time: %v", cutoff)

		cleanupCount, err := s.store.CleanupExpiredSessions(ctx, cutoff)
		if err != nil {
			s.logger.Errorf("Failed to cleanup active sessions index: %v", err)
			return
		}

		s.logger.Infof("Cleaned up %d expired sessions from active index", cleanupCount)

		remainingCount, err := s.store.ActiveSessionCount(ctx)
		if err != nil {
			s.logger.Errorf("Failed to get remaining active session count: %v", err)
			return
		}

		s.logger.Debugf("Remaining active sessions after cleanup: %d", remainingCount)

		if remainingCount > threshold {
			s.logger.Warnf("Active sessions still above threshold after cleanup: %d", remainingCount)
		}
	} else {
		s.logger.Debugf("Active sessions count below threshold: %d < %d", count, threshold)
	}
}

// GetTaskNames returns the names of all registered tasks.
func (s *Scheduler) GetTaskNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.tasks))
	for name := range s.tasks {
		names = append(names, name)
	}
	return names
}
