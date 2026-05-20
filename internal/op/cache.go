package op

import (
	"context"
	"fmt"
	"time"
)

// CacheInitFunc is a function that initializes a sub-package's in-memory cache.
type CacheInitFunc func(context.Context) error

// CacheSaveFunc is a function that persists a sub-package's in-memory cache.
type CacheSaveFunc func(context.Context) error

var cacheInitFuncs []CacheInitFunc
var cacheSaveFuncs []CacheSaveFunc

// RegisterCacheInit registers a cache initialization function.
// Functions are called in registration order during InitCache().
func RegisterCacheInit(fn CacheInitFunc) {
	cacheInitFuncs = append(cacheInitFuncs, fn)
}

// RegisterCacheSave registers a cache save function.
// Functions are called in registration order during SaveCache().
func RegisterCacheSave(fn CacheSaveFunc) {
	cacheSaveFuncs = append(cacheSaveFuncs, fn)
}

// InitCache initializes all registered sub-package caches.
// The order of registration (set in init() below) follows dependency order:
// setting → channelGroup → channel → group → apikey → llm → stats
func InitCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, fn := range cacheInitFuncs {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// SaveCache persists all registered sub-package caches.
func SaveCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, fn := range cacheSaveFuncs {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// init registers cache init and save functions in explicit dependency order.
// This avoids init() file-order non-determinism by centralizing all registrations.
func init() {
	// ── Cache init order: setting → channelGroup → channel → group → apikey → llm → stats ──
	RegisterCacheInit(func(ctx context.Context) error {
		if err := settingRefreshCache(ctx); err != nil {
			return fmt.Errorf("setting refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := channelGroupRefreshCache(ctx); err != nil {
			return fmt.Errorf("channel group refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := channelRefreshCache(ctx); err != nil {
			return fmt.Errorf("channel refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := groupRefreshCache(ctx); err != nil {
			return fmt.Errorf("group refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := apiKeyRefreshCache(ctx); err != nil {
			return fmt.Errorf("api key refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := llmRefreshCache(ctx); err != nil {
			return fmt.Errorf("llm refresh cache error: %v", err)
		}
		return nil
	})
	RegisterCacheInit(func(ctx context.Context) error {
		if err := statsRefreshCache(ctx); err != nil {
			return fmt.Errorf("stats refresh cache error: %v", err)
		}
		return nil
	})

	// ── Cache save order ──
	RegisterCacheSave(func(ctx context.Context) error {
		if err := StatsSaveDB(ctx); err != nil {
			return err
		}
		return nil
	})
	RegisterCacheSave(func(ctx context.Context) error {
		if err := ChannelKeySaveDB(ctx); err != nil {
			return err
		}
		return nil
	})
	RegisterCacheSave(func(ctx context.Context) error {
		if err := RelayLogSaveDBTask(ctx); err != nil {
			return err
		}
		return nil
	})
}
