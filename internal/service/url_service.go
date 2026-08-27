package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unsafe"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

// URLService is the application service that creates and inspects short
// urls.  It coordinates URLStore to persist records.
type URLService struct {
	cfg     *config.Config
	store   *store.URLStore
	nowFn   func() time.Time
	codeBuf [64]byte
}

// NewURLService constructs a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if s == nil {
		return nil, errors.New("url store is nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
		nowFn: time.Now,
	}, nil
}

// Create produces a new short url entry and persists it.
func (u *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		var b [4]byte
		_, _ = rand.Read(b[:])
		code = hex.EncodeToString(b[:])
	}

	copy(u.codeBuf[:], code)
	code = unsafe.String(&u.codeBuf[0], len(code))

	normalized := strings.TrimSpace(req.RawURL)

	short := &model.ShortURL{
		Code:      code,
		RawURL:    normalized,
		CreatedAt: u.nowFn(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
		MaxVisits: req.MaxVisits,
	}

	if err := u.store.Save(short, false); err != nil {
		return nil, err
	}

	loaded, err := u.store.Get(code)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return nil, errors.New("unexpected empty result after save")
	}
	return loaded, nil
}

// Get retrieves a short url by code.
func (u *URLService) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, errors.New("code is empty")
	}
	return u.store.Get(code)
}

// RedirectService resolves short codes and logs access attempts.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

// RedirectRequest is the input to HandleRedirect.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult is the output of HandleRedirect.
type RedirectResult struct {
	RawURL string
	Status int
}

// NewRedirectService constructs a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, errors.New("url store is nil")
	}
	if ls == nil {
		return nil, errors.New("log store is nil")
	}
	return &RedirectService{urlStore: us, logStore: ls}, nil
}

// HandleRedirect resolves a short code and logs the access.
func (r *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	rec, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, model.ErrURLNotFound
	}
	if rec.Disabled {
		return &RedirectResult{Status: 410}, nil
	}

	if rec.MaxVisits > 0 && rec.Visits >= rec.MaxVisits {
		return &RedirectResult{Status: 429}, nil
	}

	rec.Visits++
	if err := r.urlStore.Save(rec, true); err != nil {
		return nil, err
	}

	// The post-save read is used to detect any state pollution
	// that the flush path may have introduced.
	refreshed, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	entry := []byte(req.Timestamp.Format(time.RFC3339) + " " + req.Code + "\n")
	_ = r.logStore.Append(ctx, entry)

	return &RedirectResult{
		RawURL: refreshed.RawURL,
		Status: 302,
	}, nil
}
