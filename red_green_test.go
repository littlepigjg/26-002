package ubaas

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Session.TimeoutMinutes = 30
	log := logger.New(os.Stdout, logger.LevelError, "")
	st := store.NewMemoryStore(log)
	ss := service.NewSessionService(st, cfg, log)
	ctx := context.Background()

	normalEvent := model.NewEvent("user-001", "", model.EventPageView, "/home")
	session1, err := ss.BuildSession(ctx, normalEvent)
	if err != nil {
		t.Fatalf("BuildSession failed: %v", err)
	}

	if !session1.IsActive() {
		t.Fatal("initial session should be active")
	}

	time.Sleep(50 * time.Millisecond)

	skewedEvent := model.NewEvent("user-001", "", model.EventPageView, "/about")
	skewedEvent.Timestamp = normalEvent.Timestamp.Add(-45 * time.Minute)

	session2, err := ss.BuildSession(ctx, skewedEvent)
	if err != nil {
		t.Fatalf("BuildSession with clock-skewed event failed: %v", err)
	}

	if session2.EndTime.Before(time.Now()) {
		t.Errorf("EndTime %v is before now %v, session was corrupted by clock skew",
			session2.EndTime, time.Now())
	}

	if session2.EndTime.Before(session2.StartTime) {
		t.Errorf("EndTime %v is before StartTime %v, internal state is inconsistent",
			session2.EndTime, session2.StartTime)
	}

	if !session2.IsActive() {
		t.Error("session should remain active after processing clock-skewed event")
	}

	if session2.ID != session1.ID {
		t.Errorf("session ID changed from %s to %s, expected same session",
			session1.ID, session2.ID)
	}

	if session2.ComputeDuration() < 0 {
		t.Errorf("ComputeDuration returned negative value %d, duration calculation corrupted",
			session2.ComputeDuration())
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
	} else {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
	}
}