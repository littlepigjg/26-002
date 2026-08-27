package ubaas

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_ = us.Load(context.Background())
	_ = ls.Open(context.Background())

	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatal(err)
	}
	redirSvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatal(err)
	}

	initialCount := int64(30)
	for i := int64(0); i < initialCount; i++ {
		code := fmt.Sprintf("init-%d", i)
		_, err := svc.Create(context.Background(), &model.CreateReq{
			RawURL:     "https://example.com/" + code,
			CustomCode: code,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup

	concurrentCreates := int64(20 * 50)
	for g := 0; g < 20; g++ {
		goroutineID := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				code := fmt.Sprintf("cg-%d-%d", goroutineID, i)
				_, _ = svc.Create(context.Background(), &model.CreateReq{
					RawURL:     "https://example.com/new-" + code,
					CustomCode: code,
				})
			}
		}()
	}

	concurrentVisits := int64(16 * 50)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				codeIdx := int64(i) % initialCount
				_, _ = redirSvc.HandleRedirect(context.Background(), &service.RedirectRequest{
					Code:      fmt.Sprintf("init-%d", codeIdx),
					Timestamp: time.Now(),
				})
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(4 * time.Second):
	}

	actualTotal := us.GetTotalVisits()
	expectedTotal := initialCount + concurrentCreates + concurrentVisits

	actualCreated := svc.GetCreatedCount()
	expectedCreated := initialCount + concurrentCreates

	us.Close()
	ls.Close()

	redTotal := actualTotal != expectedTotal
	redCreated := actualCreated != expectedCreated

	if redTotal {
		t.Logf("RED（红灯，缺陷未修复）: 期望访问总数=%d, 实际=%d, 丢失=%d",
			expectedTotal, actualTotal, expectedTotal-actualTotal)
	}

	if redCreated {
		t.Logf("RED（红灯，缺陷未修复）: 期望创建总数=%d, 实际=%d, 丢失=%d",
			expectedCreated, actualCreated, expectedCreated-actualCreated)
	}

	if redTotal || redCreated {
		t.FailNow()
	}

	t.Logf("GREEN（绿灯，缺陷已修复）: 期望访问总数=%d, 实际=%d; 期望创建总数=%d, 实际=%d",
		expectedTotal, actualTotal, expectedCreated, actualCreated)
}
