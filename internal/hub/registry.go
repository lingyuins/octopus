package hub

import (
	"fmt"
	"sync"

	"github.com/lingyuins/octopus/internal/model"
)

var (
	mu       sync.RWMutex
	adapters = make(map[string]SiteAdapter)
)

// Register associates a site type string with an adapter implementation.
// Call from init() in each adapter sub-package.
func Register(siteType string, a SiteAdapter) {
	mu.Lock()
	defer mu.Unlock()
	adapters[siteType] = a
}

// Get returns the adapter for the given site type.
// Falls back to the "new-api" (common) adapter when no specific one is registered.
func Get(siteType string) (SiteAdapter, error) {
	mu.RLock()
	defer mu.RUnlock()

	if a, ok := adapters[siteType]; ok {
		return a, nil
	}
	if a, ok := adapters[model.SiteTypeNewAPI]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("no adapter registered for site type %q and no fallback available", siteType)
}

// MustGet is like Get but panics on error (for init-time wiring).
func MustGet(siteType string) SiteAdapter {
	a, err := Get(siteType)
	if err != nil {
		panic(err)
	}
	return a
}
