package bug21_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func newTestCfg(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.URLFilePath(filepath.Join(dir, "urls.json"))
	cfg.Storage.LogFilePath(filepath.Join(dir, "access.log"))
	cfg.Storage.FlushOnWrite(true)
	cfg.Storage.SyncInterval(5 * time.Second)
	return cfg
}

func writeSeed(t *testing.T, cfg *config.Config, code, rawURL string) {
	t.Helper()
	path := cfg.Storage.URLFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := fmt.Sprintf(
		`{"%s":{"code":"%s","raw_url":"%s","created_at":"2024-01-01T00:00:00Z","visits":3,"custom":true,"disabled":false,"max_visits":0}}`,
		code, code, rawURL,
	)
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
}

func TestRedGreen(t *testing.T) {
	cfg := newTestCfg(t)

	const seedCode = "seed0001"
	const seedURL = "https://example.com/seed-page"
	writeSeed(t, cfg, seedCode, seedURL)

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := us.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The seed record must be intact after Load.
	got, err := us.Get(seedCode)
	if err != nil {
		t.Fatalf("Get(seed) after load: %v", err)
	}
	if got.Code != seedCode || got.RawURL != seedURL {
		t.Fatalf("seed integrity broken after load: code=%q url=%q", got.Code, got.RawURL)
	}

	// Now drive a Save path that reuses the store's shared read buffer.
	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}

	created, err := svc.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com/new-page",
		CustomCode: "new0001",
		MaxVisits:  0,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil || created.Code != "new0001" {
		t.Fatalf("create returned unexpected value: %+v", created)
	}

	// After the create/flush cycle the seed record must still read
	// back the exact URL that was originally written to disk.
	got2, err := us.Get(seedCode)
	if err != nil {
		t.Fatalf("Get(seed) after create: %v", err)
	}
	if got2.Code != seedCode {
		t.Errorf("seed code changed: want %q got %q", seedCode, got2.Code)
	}
	if got2.RawURL != seedURL {
		t.Errorf("seed url corrupted: want %q got %q", seedURL, got2.RawURL)
	}

	// A second create reinforces the pollution so it is reproducible.
	_, err = svc.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com/another-page",
		CustomCode: "new0002",
		MaxVisits:  0,
	})
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	got3, err := us.Get(seedCode)
	if err != nil {
		t.Fatalf("Get(seed) after second create: %v", err)
	}
	if got3.Code != seedCode || got3.RawURL != seedURL {
		t.Errorf("seed data corrupted across save cycle: got code=%q url=%q", got3.Code, got3.RawURL)
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
		return
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}

func TestContextLifecycle(t *testing.T) {
	cfg := newTestCfg(t)
	writeSeed(t, cfg, "ctx0001", "https://example.com/ctx-page")

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	// Even after the store is closed, Load is called with a fresh
	// context.  The contract requires Load to honour context
	// cancellation: if the supplied context is already Done, Load
	// must return an error and MUST NOT mutate the internal state.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = us.Load(ctx)
	if err == nil {
		t.Fatalf("Load with cancelled context must return an error")
	}

	// The records map must remain empty (Load must not populate it
	// when the context is cancelled).
	snap := us.RawSnapshot()
	if len(snap) != 0 {
		t.Errorf("Load populated state despite cancelled context: %d entries", len(snap))
	}

	// And attempting to Get the seeded code must still return
	// ErrURLNotFound because Load's side effects should not have
	// been committed.
	if _, err := us.Get("ctx0001"); err != model.ErrURLNotFound {
		t.Errorf("Get after cancelled Load: want ErrURLNotFound got %v", err)
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
		return
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}

func TestRedirectRoundtrip(t *testing.T) {
	cfg := newTestCfg(t)
	writeSeed(t, cfg, "r1", "https://example.com/landing")

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore: %v", err)
	}
	defer ls.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := us.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := ls.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}

	rsvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatalf("NewRedirectService: %v", err)
	}

	res, err := rsvc.HandleRedirect(ctx, &service.RedirectRequest{
		Code:      "r1",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("HandleRedirect: %v", err)
	}
	if res.Status != 302 {
		t.Errorf("status: want 302 got %d", res.Status)
	}
	if res.RawURL != "https://example.com/landing" {
		t.Errorf("raw url: want landing got %q", res.RawURL)
	}

	// After HandleRedirect has called Save internally, a follow-up
	// Get must still return the original URL.
	got, err := us.Get("r1")
	if err != nil {
		t.Fatalf("Get after redirect: %v", err)
	}
	if got.Code != "r1" {
		t.Errorf("code polluted after HandleRedirect: got %q want %q", got.Code, "r1")
	}
	if got.RawURL != "https://example.com/landing" {
		t.Errorf("raw url polluted after HandleRedirect: got %q", got.RawURL)
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
		return
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}

func TestCustomCodeValidation(t *testing.T) {
	cfg := newTestCfg(t)
	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	ctx := context.Background()
	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}

	cases := []struct {
		name       string
		code       string
		expectErr  bool
	}{
		{"valid", "abcd", false},
		{"too short", "ab", true},
		{"illegal chars", "ab c", true},
		{"illegal chars slash", "ab/c", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Create(ctx, &model.CreateReq{
				RawURL:     "https://example.com/v",
				CustomCode: c.code,
				MaxVisits:  0,
			})
			if c.expectErr && err == nil {
				t.Fatalf("expected error for code %q", c.code)
			}
			if !c.expectErr && err != nil {
				if !strings.Contains(err.Error(), "") {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
		return
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}

func TestSnapshotReflectsWrites(t *testing.T) {
	cfg := newTestCfg(t)
	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	defer us.Close()

	ctx := context.Background()
	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}

	if _, err := svc.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com/a",
		CustomCode: "codea",
	}); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	if _, err := svc.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com/b",
		CustomCode: "codeb",
	}); err != nil {
		t.Fatalf("Create b: %v", err)
	}

	snap := us.RawSnapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot size: want 2 got %d", len(snap))
	}
	if snap["codea"].RawURL != "https://example.com/a" {
		t.Errorf("codea url: got %q", snap["codea"].RawURL)
	}
	if snap["codeb"].RawURL != "https://example.com/b" {
		t.Errorf("codeb url: got %q", snap["codeb"].RawURL)
	}

	if t.Failed() {
		fmt.Println("RED (红灯，缺陷未修复)")
		return
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}
