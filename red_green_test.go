package ubaas

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.DefaultConfig()
	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	if err := us.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}

	req := &model.CreateReq{RawURL: "https://example.com/test", CustomCode: "test12345"}
	su, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := us.Get(su.Code)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RawURL != su.RawURL {
		t.Fatalf("RawURL mismatch: want %q, got %q", su.RawURL, got.RawURL)
	}

	mc := model.NewMetricsCollector()

	var wg sync.WaitGroup
	const numWriters = 200
	const numReads = 100
	const opsPerGoroutine = 500

	writerReady := make(chan struct{}, numWriters)
	readerReady := make(chan struct{}, numReads)
	start := make(chan struct{})

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writerReady <- struct{}{}
			<-start
			for j := 0; j < opsPerGoroutine; j++ {
				mc.RecordRequest("/api/test", "GET")
				mc.CompleteRequest(true)
			}
		}()
	}

	for i := 0; i < numReads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			readerReady <- struct{}{}
			<-start
			for j := 0; j < opsPerGoroutine; j++ {
				mc.Snapshot()
			}
		}()
	}

	for i := 0; i < numWriters; i++ {
		<-writerReady
	}
	for i := 0; i < numReads; i++ {
		<-readerReady
	}

	close(start)
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	snap := mc.Snapshot()
	expectedTotal := int64(numWriters * opsPerGoroutine)
	if snap.TotalRequests != expectedTotal {
		t.Errorf("TotalRequests mismatch: want %d, got %d", expectedTotal, snap.TotalRequests)
	}
	if snap.ActiveRequests != 0 {
		t.Errorf("ActiveRequests should be 0 after all completed, got %d", snap.ActiveRequests)
	}

	t.Log("RED (红灯，缺陷未修复) - 指标收集器 Snapshot 在高并发下存在数据竞争")
}
