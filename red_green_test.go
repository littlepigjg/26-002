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
	log := logger.New(os.Stdout, logger.ParseLevel("ERROR"), "test")
	memStore := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()

	t.Run("ExportAsync context propagation", func(t *testing.T) {
		exportSvc := service.NewExportService(memStore, cfg, log)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var capturedCtx context.Context
		exportSvc.SetContextSpy(func(c context.Context) {
			capturedCtx = c
		})

		query := model.EventQuery{Page: 1, PageSize: 10}
		format := model.ExportJSON
		_, err := exportSvc.ExportAsync(ctx, query, format)
		if err != nil {
			t.Fatalf("ExportAsync failed: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		cancel()

		if capturedCtx == nil {
			t.Fatal("context spy was not called - capturedCtx is nil")
		}

		if capturedCtx == context.Background() {
			fmt.Println("RED（红灯，缺陷未修复）：ExportAsync 使用 context.Background() 而非传入的 context，上下文取消信号无法传递")
			t.Error("RED: ExportAsync uses context.Background() instead of the passed context - cancellation cannot be propagated")
		} else {
			fmt.Println("GREEN（绿灯，缺陷已修复）：ExportAsync 正确使用传入的 context，上下文取消信号可以传递")
		}
	})

	t.Run("ProcessEventsBatch context propagation", func(t *testing.T) {
		exportSvc := service.NewExportService(memStore, cfg, log)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var capturedCtx context.Context
		exportSvc.SetContextSpy(func(c context.Context) {
			capturedCtx = c
		})

		events := make([]*model.Event, 5)
		for i := 0; i < 5; i++ {
			events[i] = model.NewEvent("user1", "session1", model.EventPageView, "/test")
		}

		_, err := exportSvc.ProcessEventsBatch(ctx, events)
		if err != nil {
			t.Fatalf("ProcessEventsBatch failed: %v", err)
		}

		cancel()

		if capturedCtx == nil {
			t.Fatal("context spy was not called - capturedCtx is nil")
		}

		if capturedCtx == context.Background() {
			fmt.Println("RED（红灯，缺陷未修复）：ProcessEventsBatch 使用 context.Background() 而非传入的 context，上下文取消信号无法传递")
			t.Error("RED: ProcessEventsBatch uses context.Background() instead of the passed context - cancellation cannot be propagated")
		} else {
			fmt.Println("GREEN（绿灯，缺陷已修复）：ProcessEventsBatch 正确使用传入的 context，上下文取消信号可以传递")
		}
	})

	t.Run("Scheduler task context propagation", func(t *testing.T) {
		sched := service.NewScheduler(memStore, log)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var capturedCtx context.Context
		sched.SetContextSpy(func(c context.Context) {
			capturedCtx = c
		})

		sched.SetRootContext(ctx)
		sched.Start()

		time.Sleep(200 * time.Millisecond)

		cancel()
		sched.Stop()

		if capturedCtx == nil {
			t.Fatal("context spy was not called - capturedCtx is nil")
		}

		if capturedCtx == context.Background() {
			fmt.Println("RED（红灯，缺陷未修复）：Scheduler 任务使用 context.Background() 而非 root context，上下文取消信号无法传递")
			t.Error("RED: Scheduler tasks use context.Background() instead of root context - cancellation cannot be propagated")
		} else {
			fmt.Println("GREEN（绿灯，缺陷已修复）：Scheduler 任务正确使用 root context，上下文取消信号可以传递")
		}
	})

	t.Run("ExportAsync cancellation actually stops job", func(t *testing.T) {
		exportSvc := service.NewExportService(memStore, cfg, log)

		// Add enough events to make export take time (20 batches × 10ms = 200ms)
		eventCount := 2000
		events := make([]*model.Event, eventCount)
		for i := 0; i < eventCount; i++ {
			e := model.NewEvent("user_cancel", "session_cancel", model.EventPageView, "/page")
			e.Timestamp = time.Now()
			events[i] = e
		}
		for _, e := range events {
			memStore.CreateEvent(context.Background(), e)
		}

		ctx, cancel := context.WithCancel(context.Background())

		query := model.EventQuery{Page: 1, PageSize: 50000}
		format := model.ExportJSON
		jobID, err := exportSvc.ExportAsync(ctx, query, format)
		if err != nil {
			t.Fatalf("ExportAsync failed: %v", err)
		}

		// Let the job start processing
		time.Sleep(50 * time.Millisecond)

		// Cancel the context - the job should respect this
		cancel()

		// Wait long enough for cancellation to propagate
		time.Sleep(300 * time.Millisecond)

		status, err := exportSvc.GetJobStatus(jobID)
		if err != nil {
			t.Fatalf("GetJobStatus failed: %v", err)
		}

		// If context is not propagated (defect), the job continues processing or completes
		// If context is properly propagated (fixed), the job should be canceled/failed
		if status.Status == model.ExportStatusProcessing {
			fmt.Println("RED（红灯，缺陷未修复）：取消 context 后导出任务仍在运行，context 取消信号未传递")
			t.Error("RED: Export job continues running after context cancellation - context not properly propagated")
		} else if status.Status == model.ExportStatusFailed && status.Canceled {
			fmt.Println("GREEN（绿灯，缺陷已修复）：取消 context 后导出任务已停止，context 取消信号正确传递")
		} else if status.Status == model.ExportStatusCompleted {
			fmt.Println("RED（红灯，缺陷未修复）：导出任务在取消 context 后仍然完成，context 取消信号未传递")
			t.Error("RED: Export job completed despite context cancellation - context not properly propagated")
		} else {
			fmt.Printf("UNEXPECTED: job status=%s canceled=%v error=%v\n", status.Status, status.Canceled, status.Error)
			t.Errorf("Unexpected job status after cancellation: status=%s canceled=%v", status.Status, status.Canceled)
		}
	})

	fmt.Println("\n=== 测试完成 ===")
}
