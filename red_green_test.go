package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/pkg/cache"
	"github.com/ubaas/ubaas/service"
	"github.com/ubaas/ubaas/store"
)

func TestRedGreen(t *testing.T) {
	hasBug := false

	t.Run("LRUCache eviction does not consider TTL expiration", func(t *testing.T) {
		c := cache.NewLRUCache(3)

		c.Set("A", "valid-long-ttl", 1*time.Hour)
		c.Set("B", "expired-short-ttl", 1*time.Nanosecond)
		c.Set("C", "expired-short-ttl", 1*time.Nanosecond)

		time.Sleep(5 * time.Millisecond)

		c.Set("D", "new-entry", 1*time.Hour)

		keys := c.Keys()
		keySet := make(map[string]bool)
		for _, k := range keys {
			keySet[k] = true
		}

		if !keySet["A"] {
			hasBug = true
			t.Error("LRUCache evicted valid entry 'A' while expired entries 'B' and 'C' should have been removed first")
		}
		if !keySet["D"] {
			t.Error("New entry 'D' should be present after eviction")
		}
	})

	t.Run("Cache eviction does not consider TTL expiration", func(t *testing.T) {
		c := cache.NewWithMaxSize(1*time.Hour, 3)

		c.SetWithTTL("A", "valid-long-ttl", 1*time.Hour)
		c.SetWithTTL("B", "expired-short-ttl", 1*time.Nanosecond)
		c.SetWithTTL("C", "expired-short-ttl", 1*time.Nanosecond)

		time.Sleep(5 * time.Millisecond)

		c.SetWithTTL("D", "new-entry", 1*time.Hour)

		keys := c.Keys()
		keySet := make(map[string]bool)
		for _, k := range keys {
			keySet[k] = true
		}

		if !keySet["A"] {
			hasBug = true
			t.Error("Cache evicted valid entry 'A' while expired entries 'B' and 'C' should have been removed first")
		}
		if !keySet["D"] {
			t.Error("New entry 'D' should be present after eviction")
		}
	})

	t.Run("URLStore Save eviction respects TTL via LRUCache", func(t *testing.T) {
		cfg := config.Default()
		cfg.Storage.CacheMaxSize(3)
		cfg.Storage.FlushOnWrite(false)

		s, err := store.NewURLStore(cfg)
		if err != nil {
			t.Fatalf("NewURLStore failed: %v", err)
		}
		defer s.Close()

		u1 := &model.ShortURL{
			Code:      "aaa1",
			RawURL:    "https://example.com/valid",
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    true,
			Disabled:  false,
			MaxVisits: 0,
		}

		u2 := &model.ShortURL{
			Code:      "bbb2",
			RawURL:    "https://example.com/expired1",
			CreatedAt: time.Now().Add(-25 * time.Hour),
			Visits:    100,
			Custom:    true,
			Disabled:  false,
			MaxVisits: 100,
		}

		u3 := &model.ShortURL{
			Code:      "ccc3",
			RawURL:    "https://example.com/expired2",
			CreatedAt: time.Now().Add(-25 * time.Hour),
			Visits:    50,
			Custom:    true,
			Disabled:  false,
			MaxVisits: 50,
		}

		if err := s.Save(u1, false); err != nil {
			t.Fatalf("Save u1 failed: %v", err)
		}
		if err := s.Save(u2, false); err != nil {
			t.Fatalf("Save u2 failed: %v", err)
		}
		if err := s.Save(u3, false); err != nil {
			t.Fatalf("Save u3 failed: %v", err)
		}

		time.Sleep(5 * time.Millisecond)

		u4 := &model.ShortURL{
			Code:      "ddd4",
			RawURL:    "https://example.com/new",
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    true,
			Disabled:  false,
			MaxVisits: 0,
		}

		if err := s.Save(u4, false); err != nil {
			t.Fatalf("Save u4 failed: %v", err)
		}

		cacheKeys := s.CacheKeys()
		keySet := make(map[string]bool)
		for _, k := range cacheKeys {
			keySet[k] = true
		}

		if !keySet["aaa1"] {
			hasBug = true
			t.Error("URLStore evicted valid entry 'aaa1' from cache while expired entries should have been removed first")
		}
		if !keySet["ddd4"] {
			t.Error("New entry 'ddd4' should be present in cache")
		}
	})

	t.Run("URLService Create does not lose valid entries on cache overflow", func(t *testing.T) {
		cfg := config.Default()
		cfg.Storage.CacheMaxSize(2)
		cfg.Storage.FlushOnWrite(false)

		s, err := store.NewURLStore(cfg)
		if err != nil {
			t.Fatalf("NewURLStore failed: %v", err)
		}
		defer s.Close()

		svc, err := service.NewURLService(cfg, s)
		if err != nil {
			t.Fatalf("NewURLService failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req1 := &model.CreateReq{
			RawURL:     "https://example.com/valid-page",
			CustomCode: "aaaa1",
			MaxVisits:  0,
		}

		req2 := &model.CreateReq{
			RawURL:     "https://example.com/expired-page",
			CustomCode: "bbbb2",
			MaxVisits:  1,
		}

		_, err = svc.Create(ctx, req1)
		if err != nil {
			t.Fatalf("Create req1 failed: %v", err)
		}

		_, err = svc.Create(ctx, req2)
		if err != nil {
			t.Fatalf("Create req2 failed: %v", err)
		}

		if err := s.IncrementVisitsWithGuard("bbbb2"); err != nil {
			t.Fatalf("IncrementVisits failed: %v", err)
		}

		time.Sleep(5 * time.Millisecond)

		req3 := &model.CreateReq{
			RawURL:     "https://example.com/new-page",
			CustomCode: "cccc3",
			MaxVisits:  0,
		}

		_, err = svc.Create(ctx, req3)
		if err != nil {
			t.Fatalf("Create req3 failed: %v", err)
		}

		cacheKeys := s.CacheKeys()
		keySet := make(map[string]bool)
		for _, k := range cacheKeys {
			keySet[k] = true
		}

		if !keySet["aaaa1"] {
			hasBug = true
			t.Error("URLService Create evicted valid entry 'aaaa1' from cache while expired entries should have been removed first")
		}
	})

	t.Run("Cache SetWithEviction does not clean expired entries first", func(t *testing.T) {
		c := cache.NewLRUCache(3)

		c.SetWithEviction("A", "valid", 1*time.Hour)
		c.SetWithEviction("B", "expired", 1*time.Nanosecond)
		c.SetWithEviction("C", "expired", 1*time.Nanosecond)

		time.Sleep(5 * time.Millisecond)

		c.SetWithEviction("D", "new", 1*time.Hour)

		keys := c.Keys()
		keySet := make(map[string]bool)
		for _, k := range keys {
			keySet[k] = true
		}

		if !keySet["A"] {
			hasBug = true
			t.Error("SetWithEviction evicted valid entry 'A' before expired entries")
		}
	})

	if hasBug {
		fmt.Println("RED (红灯，缺陷未修复)")
	} else {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
	}
}
