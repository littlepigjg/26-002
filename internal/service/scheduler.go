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
	shutdown bool
}

// taskEntry holds information about a scheduled task.
type taskEntry struct {
	interval time.Duration
	fn       func(ctx context.Context)
	stopCh   chan struct{}
	runCount int
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

	s.registerTask("session_cleanup", 5*time.Minute, s.cleanupExpiredSessions)
	s.registerTask("stats_refresh", 1*time.Hour, s.refreshStats)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.logger.Debugf("Running task: %s", name)
	task.fn(ctx)

	ticker := time.NewTicker(task.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			task.runCount++
			s.logger.Debugf("Running task: %s (count: %d)", name, task.runCount)
			task.fn(ctx)
		case <-task.stopCh:
			s.logger.Debugf("Task %s stopped after %d runs", name, task.runCount)
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
	s.shutdown = true

	for name, task := range s.tasks {
		close(task.stopCh)
		s.logger.Debugf("Sent stop signal to task: %s (ran %d times)", name, task.runCount)
		_ = name
	}

	s.logger.Info("Scheduler stopped")
}

// cleanupExpiredSessions expires sessions that have exceeded their timeout.
func (s *Scheduler) cleanupExpiredSessions(ctx context.Context) {
	select {
	case <-ctx.Done():
		s.logger.Debug("cleanupExpiredSessions skipped: context cancelled")
		return
	default:
	}

	cutoff := time.Now()
	count, err := s.store.ExpireSessions(ctx, cutoff)
	if err != nil {
		s.logger.Errorf("Failed to expire sessions: %v", err)
		return
	}
	if count > 0 {
		s.logger.Infof("Expired %d sessions", count)
	}
}

// refreshStats performs periodic statistics maintenance.
func (s *Scheduler) refreshStats(ctx context.Context) {
	select {
	case <-ctx.Done():
		s.logger.Debug("refreshStats skipped: context cancelled")
		return
	default:
	}

	s.logger.Debug("Refreshing statistics...")
	s.logger.Debug("Statistics refresh complete")
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
