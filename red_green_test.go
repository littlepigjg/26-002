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

const totalEvents = 200

func TestRedGreen(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "")
	s := store.NewMemoryStore(log)
	defer s.Close()

	baseTime := time.Now()
	for i := 0; i < totalEvents; i++ {
		ev := model.NewEvent("user-001", "ses-001", model.EventPageView, fmt.Sprintf("/page-%d", i%10))
		ev.ID = fmt.Sprintf("evt-%04d", i)
		ev.Timestamp = baseTime.Add(-time.Duration(totalEvents-i) * time.Second)
		if err := s.CreateEvent(context.Background(), ev); err != nil {
			t.Fatalf("failed to create event: %v", err)
		}
	}

	t.Run("PageSize_zero_uses_default_pagination", func(t *testing.T) {
		q := model.EventQuery{
			UserID:   "user-001",
			PageSize: 0,
		}
		if err := q.Validate(); err != nil {
			t.Fatalf("validate error: %v", err)
		}

		events, total, err := s.ListEvents(context.Background(), q)
		if err != nil {
			t.Fatalf("ListEvents error: %v", err)
		}

		if len(events) == totalEvents && total == totalEvents {
			fmt.Printf("RED（红灯，缺陷未修复）: PageSize=0 触发全表扫描，返回了全部 %d 条事件\n", total)
			t.FailNow()
		}

		if len(events) > model.DefaultPageSize {
			fmt.Printf("RED（红灯，缺陷未修复）: PageSize=0 返回了 %d 条，超过默认分页大小 %d\n", len(events), model.DefaultPageSize)
			t.FailNow()
		}

		fmt.Printf("GREEN（绿灯，缺陷已修复）: PageSize=0 正确限制了返回数量，仅返回 %d 条（共 %d 条）\n", len(events), total)
	})

	t.Run("Cancelled_context_stops_full_scan", func(t *testing.T) {
		q := model.EventQuery{
			UserID:   "user-001",
			PageSize: 0,
		}
		q.Validate()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		start := time.Now()
		events, _, err := s.ListEvents(ctx, q)
		elapsed := time.Since(start)

		if err == nil && len(events) == totalEvents {
			fmt.Printf("RED（红灯，缺陷未修复）: Context 已取消但仍完成了全表扫描，返回 %d 条，用时 %v\n", len(events), elapsed)
			t.FailNow()
		}

		if err == nil {
			fmt.Printf("RED（红灯，缺陷未修复）: Context 已取消但未返回错误，返回 %d 条，用时 %v\n", len(events), elapsed)
			t.FailNow()
		}

		fmt.Printf("GREEN（绿灯，缺陷已修复）: Context 取消正确中断了查询，返回错误 %v，用时 %v\n", err, elapsed)
	})

	t.Run("Cancelled_context_stops_normal_query", func(t *testing.T) {
		q := model.EventQuery{
			UserID:   "user-001",
			PageSize: 25,
			Page:     1,
		}
		q.Validate()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		events, total, err := s.ListEvents(ctx, q)

		if err == nil {
			fmt.Printf("RED（红灯，缺陷未修复）: Context 已取消但仍完成了分页查询，返回 %d 条（共 %d 条），未返回错误\n", len(events), total)
			t.FailNow()
		}

		fmt.Printf("GREEN（绿灯，缺陷已修复）: Context 取消正确中断了分页查询，返回错误 %v\n", err)
	})

	t.Run("Validate_sets_default_for_zero_pageSize", func(t *testing.T) {
		q := model.EventQuery{
			UserID:   "user-001",
			PageSize: 0,
		}
		if err := q.Validate(); err != nil {
			t.Fatalf("validate error: %v", err)
		}

		if q.PageSize != model.DefaultPageSize {
			fmt.Printf("RED（红灯，缺陷未修复）: PageSize=0 经过 Validate 后应为 %d，实际为 %d\n", model.DefaultPageSize, q.PageSize)
			t.FailNow()
		}

		fmt.Printf("GREEN（绿灯，缺陷已修复）: PageSize=0 正确设置为默认分页大小 %d\n", q.PageSize)
	})
}
