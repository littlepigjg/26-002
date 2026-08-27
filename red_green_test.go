package main

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
	log := logger.New(os.Stdout, logger.LevelInfo, "")
	ms := store.NewMemoryStore(log)
	us := store.NewUserStore(ms)
	ss := service.NewSessionService(ms, cfg, log)

	ctx := context.Background()
	userID := "test-user-001"

	t.Run("test-user-type-consistency", func(t *testing.T) {
		hasMismatch := false
		var err error

		now := time.Now()
		event1 := model.NewEvent(userID, "", model.EventPageView, "/home")
		event1.Timestamp = now

		session1, err := ss.BuildSession(ctx, event1)
		if err != nil {
			t.Fatalf("BuildSession failed: %v", err)
		}

		_, err = us.UpdateUser(ctx, event1)
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		for i := 0; i < 4; i++ {
			event := model.NewEvent(userID, session1.ID, model.EventPageView, fmt.Sprintf("/page%d", i+1))
			event.Timestamp = now.Add(time.Duration(i+1) * time.Minute)
			_, err := ss.BuildSession(ctx, event)
			if err != nil {
				t.Fatalf("BuildSession failed: %v", err)
			}
			_, err = us.UpdateUser(ctx, event)
			if err != nil {
				t.Fatalf("UpdateUser failed: %v", err)
			}
		}

		sessions, err := ms.GetUserSessions(ctx, userID, true)
		if err != nil || len(sessions) == 0 {
			t.Fatalf("Failed to get sessions: %v", err)
		}
		currentSession := sessions[0]

		ud, err := us.GetUser(ctx, userID)
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}

		t.Logf("After 5 events - Session UserType: %s, UserDimension UserType: %s", currentSession.UserType, ud.UserType)

		if ud.UserType != model.UserReturning {
			t.Errorf("UserDimension should be UserReturning after 5 events, got %s", ud.UserType)
			hasMismatch = true
		}

		if currentSession.UserType != model.UserNew {
			t.Errorf("Session should be UserNew (AddEvent doesn't update UserType), got %s", currentSession.UserType)
			hasMismatch = true
		}

		if ud.UserType == model.UserReturning && currentSession.UserType == model.UserNew {
			hasMismatch = true
			t.Errorf("BUG: UserType mismatch - UserDimension is Returning but Session is New")
		}

		eventNew := model.NewEvent(userID, "", model.EventPageView, "/dashboard")
		eventNew.Timestamp = now.Add(2 * time.Hour)

		session2, err := ss.BuildSession(ctx, eventNew)
		if err != nil {
			t.Fatalf("BuildSession failed: %v", err)
		}

		ud2, err := us.UpdateUser(ctx, eventNew)
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		t.Logf("After timeout - Session2 UserType: %s, UserDimension UserType: %s", session2.UserType, ud2.UserType)

		if session2.UserType != model.UserReturning {
			t.Errorf("New session after timeout should be UserReturning, got %s", session2.UserType)
			hasMismatch = true
		}

		if ud2.UserType != model.UserReturning {
			t.Errorf("UserDimension should be UserReturning, got %s", ud2.UserType)
			hasMismatch = true
		}

		allSessions, _ := ms.GetUserSessions(ctx, userID, true)
		for _, s := range allSessions {
			if s.UserType != ud2.UserType {
				hasMismatch = true
				t.Errorf("Final check: Session %s has UserType %s, UserDimension has %s",
					s.ID, s.UserType, ud2.UserType)
			}
		}

		if hasMismatch {
			fmt.Println("RED (红灯，缺陷未修复)：用户类型在 Session 和 UserDimension 之间不一致")
		} else {
			fmt.Println("GREEN (绿灯，缺陷已修复)：用户类型在 Session 和 UserDimension 之间一致")
		}
	})
}
