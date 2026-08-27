package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/store"
)

type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if s == nil {
		return nil, errors.New("url store is nil")
	}
	return &URLService{cfg: cfg, store: s}, nil
}

func (u *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, errors.New("nil create request")
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid create request: %s", err.Error())
	}
	code := req.CustomCode
	if code == "" {
		c, err := model.GenerateCode()
		if err != nil {
			return nil, fmt.Errorf("generate code failed: %s", err.Error())
		}
		code = c
	}
	su := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
		MaxVisits: req.MaxVisits,
	}
	if err := su.Validate(); err != nil {
		return nil, fmt.Errorf("invalid short url: %s", err.Error())
	}
	if err := u.store.Save(su, false); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "storage unavailable") {
			return nil, errors.New("create failed: storage unavailable")
		}
		return nil, fmt.Errorf("create failed for code %s: %s", code, msg)
	}
	return su, nil
}

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL string
	Status int
}

type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, errors.New("url store is nil")
	}
	if ls == nil {
		return nil, errors.New("log store is nil")
	}
	return &RedirectService{urlStore: us, logStore: ls}, nil
}

func (r *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil || strings.TrimSpace(req.Code) == "" {
		return nil, errors.New("empty redirect code")
	}
	su, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("redirect failed: %s", err.Error())
	}
	_ = r.logStore.Append(model.AccessLog{Code: req.Code, Timestamp: req.Timestamp})
	if su.Disabled {
		return nil, fmt.Errorf("code %s is disabled", req.Code)
	}
	return &RedirectResult{RawURL: su.RawURL, Status: 302}, nil
}