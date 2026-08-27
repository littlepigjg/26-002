package service

import (
	"context"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

func setup(t *testing.T) (*URLService, *store.URLStore, *store.AccessLogStore) {
	t.Helper()
	cfg := config.Default()
	ustore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	if err := ustore.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	lstore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore: %v", err)
	}
	if err := lstore.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	svc, err := NewURLService(cfg, ustore)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}
	return svc, ustore, lstore
}

func TestCreateVisitsStartsAtZero(t *testing.T) {
	svc, ustore, _ := setup(t)

	created, err := svc.Create(context.Background(), &model.CreateReq{
		RawURL:    "https://example.com/page1",
		MaxVisits: 50,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Visits != 0 {
		t.Fatalf("returned Visits = %d, want 0 (MaxVisits is a limit, not the count)", created.Visits)
	}

	// The stored record must also read 0, not 50.
	got, err := ustore.Get(created.Code)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Visits != 0 {
		t.Fatalf("stored Visits = %d, want 0", got.Visits)
	}
}

func TestGetReturnsCopyNotLiveReference(t *testing.T) {
	svc, ustore, _ := setup(t)

	created, err := svc.Create(context.Background(), &model.CreateReq{
		RawURL:   "https://example.com/page1",
		MaxVisits: 50,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	a, err := ustore.Get(created.Code)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	b, err := ustore.Get(created.Code)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a == b {
		t.Fatalf("Get returned the same pointer; callers can mutate the live record")
	}
	// Mutating one must not affect the other or the stored record.
	a.Visits = 999
	b2, _ := ustore.Get(created.Code)
	if b.Visits == 999 || b2.Visits == 999 {
		t.Fatalf("aliasing detected: mutation leaked into another Get or the store")
	}
}

func TestSaveOverwriteSemantics(t *testing.T) {
	_, ustore, _ := setup(t)

	orig := &model.ShortURL{Code: "abc", RawURL: "https://example.com/orig"}
	if err := ustore.Save(orig, false); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	// overwrite=false on an existing code must fail with ErrCodeExists.
	dup := &model.ShortURL{Code: "abc", RawURL: "https://example.com/dup"}
	if err := ustore.Save(dup, false); err != model.ErrCodeExists {
		t.Fatalf("Save overwrite=false: got %v, want ErrCodeExists", err)
	}

	// overwrite=true on an existing code must succeed and replace the value.
	repl := &model.ShortURL{Code: "abc", RawURL: "https://example.com/repl", Visits: 7}
	if err := ustore.Save(repl, true); err != nil {
		t.Fatalf("Save overwrite=true: %v", err)
	}
	got, err := ustore.Get("abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RawURL != "https://example.com/repl" {
		t.Fatalf("stored value not replaced: got %q", got.RawURL)
	}
	// Mutating the caller's pointer after overwrite=true must not touch the store.
	repl.Visits = 123
	got2, _ := ustore.Get("abc")
	if got2.Visits == 123 {
		t.Fatalf("store aliased caller pointer after overwrite")
	}
}

func TestLoadRespectsCancelledContext(t *testing.T) {
	cfg := config.Default()
	ustore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ustore.Load(ctx); err != context.Canceled {
		t.Fatalf("Load with cancelled ctx: got %v, want context.Canceled", err)
	}
}

func TestRedirectIncrementsVisitsFromZero(t *testing.T) {
	svc, ustore, lstore := setup(t)

	created, err := svc.Create(context.Background(), &model.CreateReq{
		RawURL:    "https://example.com/page1",
		MaxVisits: 50,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Visits != 0 {
		t.Fatalf("precondition: created Visits = %d, want 0", created.Visits)
	}

	rs, err := NewRedirectService(ustore, lstore)
	if err != nil {
		t.Fatalf("NewRedirectService: %v", err)
	}

	res, err := rs.HandleRedirect(context.Background(), &RedirectRequest{
		Code:      created.Code,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("HandleRedirect: %v", err)
	}
	if res.Status != 302 || res.RawURL != "https://example.com/page1" {
		t.Fatalf("unexpected redirect result: %+v", res)
	}

	got, _ := ustore.Get(created.Code)
	if got.Visits != 1 {
		t.Fatalf("after one redirect, stored Visits = %d, want 1", got.Visits)
	}

	// A second redirect goes to 2, not to an off-by-one of the MaxVisits.
	rs.HandleRedirect(context.Background(), &RedirectRequest{Code: created.Code, Timestamp: time.Now()})
	got, _ = ustore.Get(created.Code)
	if got.Visits != 2 {
		t.Fatalf("after two redirects, stored Visits = %d, want 2", got.Visits)
	}
}
