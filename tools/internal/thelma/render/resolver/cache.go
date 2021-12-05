package resolver

import (
	"strings"
	"sync"
)

// Separator use when joining elements of a ChartRelease into a string cache key
const defaultCacheKeySeparator = " " // URLs can't have whitespace, so this won't show up
// in Helm repo and chart names, or in chart version strings

// Synchronized cache is a concurrent-safe cache for ResolvedCharts.
// It guarantees that the chart resolver function will only be called once for a given cache key.
type syncCache interface {
	get(chartRelease ChartRelease, resolver resolverFn) (ResolvedChart, error)
}

// Maps chart releases to cache keys.
// This is useful because the local chart resolver, for example, ignores chart versions
// when resolving charts.
type keyMapper func(chartRelease ChartRelease) string

// Function the cache should use for resolving a chart release.
type resolverFn func(chartRelease ChartRelease) (ResolvedChart, error)

// Default key mapper factors all fields of ChartRelease into a unique key
func defaultCacheKeyMapper(chartRelease ChartRelease) string {
	return strings.Join(
		[]string{
			chartRelease.Repo,
			chartRelease.Name,
			chartRelease.Version,
		},
		defaultCacheKeySeparator,
	)
}

type syncCacheImpl struct {
	globalMutex sync.RWMutex
	keyMapper   keyMapper
	cache       map[string]*entry
}

// Returns a new syncCache instance
func newSyncCache() syncCache {
	return newSyncCacheWithMapper(defaultCacheKeyMapper)
}

func newSyncCacheWithMapper(keyMapper func(ChartRelease) string) syncCache {
	return &syncCacheImpl{
		globalMutex: sync.RWMutex{},
		keyMapper:   keyMapper,
		cache:       make(map[string]*entry),
	}
}

func (c *syncCacheImpl) get(chartRelease ChartRelease, resolver resolverFn) (ResolvedChart, error) {
	key := c.keyMapper(chartRelease)
	_entry := c.getEntry(key)

	return _entry.getValue(chartRelease, resolver)
}

func (c *syncCacheImpl) getEntry(key string) *entry {
	// get a read lock so we can safely read from the map
	c.globalMutex.RLock()
	_entry, exists := c.cache[key]
	c.globalMutex.RUnlock()

	if exists {
		return _entry
	}

	// entry does not exist in map, so set a write lock and create one
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	_entry = &entry{}
	c.cache[key] = _entry

	return _entry
}

// Represents an entry in the cache
type entry struct {
	initialized   bool
	mutex         sync.RWMutex
	resolvedChart ResolvedChart
	err           error
}

// Returns the cached value for a given entry
func (e *entry) getValue(chartRelease ChartRelease, resolver resolverFn) (ResolvedChart, error) {
	// Obtain a read lock
	// if we've been initialized already, return the cached values
	e.mutex.RLock()
	if e.initialized {
		defer e.mutex.RUnlock()
		return e.resolvedChart, e.err
	}

	// Otherwise: release the read lock & obtain a write lock instead
	e.mutex.RUnlock()
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// generate the values
	rc, err := resolver(chartRelease)

	// save them & mark this entry as initialized
	e.resolvedChart = rc
	e.err = err
	e.initialized = true

	// return result
	return e.resolvedChart, e.err
}
