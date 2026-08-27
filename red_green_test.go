package ubaas_test

import (
	"context"
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
	log := logger.New(os.Stdout, logger.LevelError, "")
	memStore := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()
	svc := service.NewEventService(memStore, cfg, log)

	svc.SetPanicGuard(func() bool { return true })

	req := &model.EventCreateRequest{
		UserID: "test-user",
		Type:   model.EventPageView,
		PageURL: "/home",
	}

	ctx := context.Background()
	event, err := svc.CreateEvent(ctx, req)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	retrieved, err := memStore.GetEvent(ctx, event.ID)
	if err != nil {
		t.Logf("RED (红灯，缺陷未修复): 事件在后台goroutine panic中丢失，GetEvent返回错误: %v", err)
		t.FailNow()
	}

	if retrieved == nil {
		t.Logf("RED (红灯，缺陷未修复): 检索到的事件为nil，数据丢失")
		t.FailNow()
	}

	if retrieved.ID != event.ID {
		t.Logf("RED (红灯，缺陷未修复): 事件ID不匹配，期望%s，实际%s", event.ID, retrieved.ID)
		t.FailNow()
	}

	t.Logf("GREEN (绿灯，缺陷已修复): 事件成功存储并可检索，ID=%s", retrieved.ID)
}
