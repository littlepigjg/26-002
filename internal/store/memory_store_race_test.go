package store

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

type testDiscard struct{}

func (testDiscard) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = testDiscard{}

// TestMemoryStoreConcurrentCreateEvents exercises the data race that crashed
// the production load test: concurrent writers calling CreateEvent while
// readers call ListEvents / RawSnapshot. Before the fix the store accessed
// eventsByUser / eventsBySession outside the lock, causing
// "fatal error: concurrent map read and map write" and lost updates that left
// the per-user index inconsistent with the events map. The race detector is
// the primary guard; the consistency assertions catch the drift without -race.
//
// Run with: go test -race -run TestMemoryStoreConcurrentCreateEvents ./internal/store
func TestMemoryStoreConcurrentCreateEvents(t *testing.T) {
	log := logger.New(testDiscard{}, logger.LevelError, "")
	s := NewMemoryStore(log)
	defer s.Close()

	const writers = 50
	const perWriter = 20
	const readers = 10
	const numUsers = 5

	var wg sync.WaitGroup
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				event := &model.Event{
					ID:         fmt.Sprintf("evt-%d-%d", gid, i),
					UserID:     fmt.Sprintf("user-%d", gid%numUsers),
					Type:       model.EventPageView,
					PageURL:    "/page",
					DeviceType: model.DeviceDesktop,
				}
				if err := s.CreateEvent(context.Background(), event); err != nil {
					t.Errorf("CreateEvent failed: %v", err)
				}
			}
		}(g)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_, _, _ = s.ListEvents(context.Background(), model.EventQuery{Page: 1, PageSize: 50})
				_ = s.RawSnapshot()
				_ = s.RawUserIndexSnapshot()
			}
		}()
	}
	wg.Wait()

	snap := s.RawSnapshot()
	if len(snap) != writers*perWriter {
		t.Fatalf("expected %d events, got %d", writers*perWriter, len(snap))
	}

	// Index total must equal events map size; no lost or duplicated updates.
	uid := s.RawUserIndexSnapshot()
	total := 0
	for _, ids := range uid {
		total += len(ids)
	}
	if total != len(snap) {
		t.Fatalf("index total %d != events map size %d", total, len(snap))
	}

	// Every user must have exactly the expected number of events (even split
	// since each writer contributes one event to gid%numUsers).
	expected := writers / numUsers * perWriter
	for u := 0; u < numUsers; u++ {
		got := len(uid[fmt.Sprintf("user-%d", u)])
		if got != expected {
			t.Fatalf("user-%d: expected %d events, got %d", u, expected, got)
		}
	}
}
