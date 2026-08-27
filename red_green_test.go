package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "test")
	memStore := store.NewMemoryStore(log)
	defer memStore.Close()

	timeout := 1 * time.Hour
	session := model.NewSession("user1", model.DeviceDesktop, timeout)
	memStore.CreateSession(context.Background(), session)

	stored, err := memStore.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}

	memStore.ExpireSessions(context.Background(), time.Now().Add(2*time.Hour))

	update := *stored
	event := model.NewEvent("user1", "", model.EventPageView, "/newpage")
	event.Timestamp = time.Now()
	update.AddEvent(event, timeout)

	err = memStore.UpdateSession(context.Background(), &update)

	if err == nil {
		fmt.Println("RED (红灯，缺陷存在)")
		t.FailNow()
	} else {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
	}
}
