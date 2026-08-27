package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/service"
	"github.com/ubaas/ubaas/store"
)

// TestRedGreen exercises the shurl public API and decides whether the
// codebase is in a RED (bug present) or GREEN (bug fixed) state.  The
// test output explicitly prints RED/GREEN so a human or CI can tell
// at a glance whether the code is healthy.
func TestRedGreen(t *testing.T) {
	var (
		failures int
		lastErr  error
		lastDesc string
	)

	check := func(desc string, ok bool, err error) {
		if !ok {
			failures++
			lastDesc = desc
			lastErr = err
		}
	}

	// -----------------------------------------------------------------
	// Case 1: Save/Create errors must preserve the diagnostic reason so
	// that callers can tell *why* storage failed (e.g. which path,
	// which operation) rather than just getting a generic "fail" string.
	// -----------------------------------------------------------------
	t.Run("error-propagation-store-save", func(t *testing.T) {
		dir := t.TempDir()
		badPath := filepath.Join(dir, "no-such-dir", "data.json")

		cfg := config.Default()
		cfg.Storage.URLFilePath(badPath)

		s, err := store.NewURLStore(cfg)
		check("NewURLStore returns a store", s != nil, nil)
		// NewURLStore surfaces a diagnostic error containing the path
		// so callers know which file could not be opened.
		check("NewURLStore error contains path diagnostic",
			err != nil && strings.Contains(err.Error(), "no-such-dir"), err)

		if s == nil {
			return
		}

		su := &model.ShortURL{
			Code:      "abc1",
			RawURL:    "https://example.com/a",
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    true,
			Disabled:  false,
		}
		if err := su.Validate(); err != nil {
			t.Fatalf("precondition: ShortURL must validate, got: %v", err)
		}

		svc, err := service.NewURLService(cfg, s)
		check("NewURLService returns a service", svc != nil, err)

		err = s.Save(su, false)
		check("Save returns an error", err != nil, err)
		// Save must tell the caller *why* storage failed.  The requested
		// path is the most reliable diagnostic detail.
		check("Save error preserves path diagnostic",
			err != nil && strings.Contains(err.Error(), "no-such-dir"), err)

		req := &model.CreateReq{
			RawURL:     "https://example.com/a",
			CustomCode: "abc1",
			MaxVisits:  0,
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("precondition: CreateReq must validate, got: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err = svc.Create(ctx, req)
		check("Service.Create returns an error on bad store", err != nil, err)
		// The critical assertion for the whole test: the Create error
		// must still carry the original diagnostic keyword ("no-such-dir")
		// so the caller can act on it.
		check("Service.Create error preserves path diagnostic",
			err != nil && strings.Contains(err.Error(), "no-such-dir"), err)
	})

	// -----------------------------------------------------------------
	// Case 2: validation errors from Create must keep their specific
	// reason so callers can distinguish between "missing url" and
	// "bad scheme" instead of getting one opaque "invalid" message.
	// -----------------------------------------------------------------
	t.Run("error-propagation-validation", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "good.json")

		cfg := config.Default()
		cfg.Storage.URLFilePath(path)

		s, err := store.NewURLStore(cfg)
		if err != nil {
			t.Fatalf("precondition: store must be openable in this case, got: %v", err)
		}
		if err := s.Load(context.Background()); err != nil {
			t.Fatalf("precondition: Load must succeed, got: %v", err)
		}
		defer s.Close()

		svc, err := service.NewURLService(cfg, s)
		if err != nil {
			t.Fatalf("precondition: NewURLService must succeed, got: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// raw_url is empty -> specific ErrRawURLRequired
		_, err = svc.Create(ctx, &model.CreateReq{RawURL: "", CustomCode: "abcd", MaxVisits: 0})
		check("empty raw_url returns error", err != nil, err)
		check("empty raw_url preserves specific 'raw_url' diagnostic",
			err != nil && strings.Contains(err.Error(), "raw_url"), err)

		// bad scheme -> specific ErrInvalidURL
		_, err = svc.Create(ctx, &model.CreateReq{RawURL: "ftp://x.com", CustomCode: "abcd", MaxVisits: 0})
		check("bad scheme returns error", err != nil, err)
		check("bad scheme preserves specific 'http' diagnostic",
			err != nil && strings.Contains(err.Error(), "http"), err)

		// negative max_visits -> specific ErrMaxVisitsNegative
		_, err = svc.Create(ctx, &model.CreateReq{RawURL: "https://x.com", CustomCode: "abcd", MaxVisits: -1})
		check("negative max_visits returns error", err != nil, err)
		check("negative max_visits preserves specific 'max_visits' diagnostic",
			err != nil && strings.Contains(err.Error(), "max_visits"), err)
	})

	// -----------------------------------------------------------------
	// Case 3: Get not-found must propagate the store's specific
	// "not found" diagnostic through the redirect layer instead of
	// being flattened into a generic "redirect failed" string.
	// -----------------------------------------------------------------
	t.Run("error-propagation-redirect-not-found", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "good.json")
		logPath := filepath.Join(dir, "access.log")

		cfg := config.Default()
		cfg.Storage.URLFilePath(path)
		cfg.Storage.LogFilePath(logPath)

		s, err := store.NewURLStore(cfg)
		if err != nil {
			t.Fatalf("precondition: url store must open, got: %v", err)
		}
		defer s.Close()

		l, err := store.NewAccessLogStore(cfg)
		if err != nil {
			t.Fatalf("precondition: log store must open, got: %v", err)
		}
		defer l.Close()

		rs, err := service.NewRedirectService(s, l)
		if err != nil {
			t.Fatalf("precondition: NewRedirectService must succeed, got: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		_, err = rs.HandleRedirect(ctx, &service.RedirectRequest{Code: "DOESNOTEXIST", Timestamp: time.Now()})
		check("HandleRedirect returns error for missing code", err != nil, err)
		// Diagnostic keyword that should survive if the chain is intact.
		check("HandleRedirect error preserves 'code not found' diagnostic",
			err != nil && strings.Contains(err.Error(), "code not found"),
			err)
	})

	// -----------------------------------------------------------------
	// Summary
	// -----------------------------------------------------------------
	if failures == 0 {
		t.Log("GREEN（绿灯，缺陷已修复）")
	} else {
		t.Logf("RED（红灯，缺陷未修复） — %s: %v", lastDesc, lastErr)
		t.FailNow()
	}
}

// TestRedGreenShort exists as a stable anchor for verify_cmd.
func TestRedGreenShort(t *testing.T) {
	TestRedGreen(t)
}

// Keep the os import referenced for the generated helper style.
var _ = os.TempDir
