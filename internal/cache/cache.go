package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

// CacheEntry represents a cached registry response
type CacheEntry struct {
	Tags      []string
	ImageInfo *types.ImageInfo
	Timestamp time.Time
	TTL       time.Duration
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Since(e.Timestamp) > e.TTL
}

// CacheStats holds statistics about cache usage
type CacheStats struct {
	Hits     int64 `json:"hits"`
	Misses   int64 `json:"misses"`
	Evicted  int64 `json:"evicted"`
	Size     int64 `json:"size"`
}

// HitRate returns the cache hit rate as a percentage
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0.0
	}
	return float64(s.Hits) / float64(total) * 100.0
}

// RegistryCache provides in-memory caching for registry responses
type RegistryCache struct {
	cache       sync.Map
	defaultTTL  time.Duration
	stats       CacheStats
	cleanupTick *time.Ticker
	stopCleanup chan struct{}
}

// Config holds cache configuration
type Config struct {
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
}

// DefaultConfig returns sensible default cache configuration
func DefaultConfig() Config {
	return Config{
		DefaultTTL:      15 * time.Minute, // Cache for 15 minutes by default
		CleanupInterval: 5 * time.Minute,  // Clean up expired entries every 5 minutes
	}
}

// NewRegistryCache creates a new registry cache with the given configuration
func NewRegistryCache(config Config) *RegistryCache {
	cache := &RegistryCache{
		defaultTTL:  config.DefaultTTL,
		stopCleanup: make(chan struct{}),
	}
	
	// Start background cleanup goroutine
	if config.CleanupInterval > 0 {
		cache.cleanupTick = time.NewTicker(config.CleanupInterval)
		go cache.cleanupLoop()
	}
	
	return cache
}

// GetTags retrieves cached tags for an image
func (c *RegistryCache) GetTags(image types.DockerImage) ([]string, bool) {
	key := c.makeKey(image, "tags")
	
	if value, ok := c.cache.Load(key); ok {
		entry := value.(*CacheEntry)
		
		if !entry.IsExpired() {
			atomic.AddInt64(&c.stats.Hits, 1)
			return entry.Tags, true
		}
		
		// Entry expired, remove it
		c.cache.Delete(key)
		atomic.AddInt64(&c.stats.Evicted, 1)
		atomic.AddInt64(&c.stats.Size, -1)
	}
	
	atomic.AddInt64(&c.stats.Misses, 1)
	return nil, false
}

// SetTags caches tags for an image
func (c *RegistryCache) SetTags(image types.DockerImage, tags []string) {
	c.SetTagsWithTTL(image, tags, c.defaultTTL)
}

// SetTagsWithTTL caches tags for an image with custom TTL
func (c *RegistryCache) SetTagsWithTTL(image types.DockerImage, tags []string, ttl time.Duration) {
	key := c.makeKey(image, "tags")
	
	entry := &CacheEntry{
		Tags:      make([]string, len(tags)), // Create a copy to avoid external modifications
		Timestamp: time.Now(),
		TTL:       ttl,
	}
	copy(entry.Tags, tags)
	
	// Check if this is a new entry
	_, existed := c.cache.LoadOrStore(key, entry)
	if !existed {
		atomic.AddInt64(&c.stats.Size, 1)
	} else {
		// Update existing entry
		c.cache.Store(key, entry)
	}
}

// GetImageInfo retrieves cached image info
func (c *RegistryCache) GetImageInfo(image types.DockerImage) (*types.ImageInfo, bool) {
	key := c.makeKey(image, "info")
	
	if value, ok := c.cache.Load(key); ok {
		entry := value.(*CacheEntry)
		
		if !entry.IsExpired() {
			atomic.AddInt64(&c.stats.Hits, 1)
			return entry.ImageInfo, true
		}
		
		// Entry expired, remove it
		c.cache.Delete(key)
		atomic.AddInt64(&c.stats.Evicted, 1)
		atomic.AddInt64(&c.stats.Size, -1)
	}
	
	atomic.AddInt64(&c.stats.Misses, 1)
	return nil, false
}

// SetImageInfo caches image info
func (c *RegistryCache) SetImageInfo(image types.DockerImage, info *types.ImageInfo) {
	c.SetImageInfoWithTTL(image, info, c.defaultTTL)
}

// SetImageInfoWithTTL caches image info with custom TTL
func (c *RegistryCache) SetImageInfoWithTTL(image types.DockerImage, info *types.ImageInfo, ttl time.Duration) {
	key := c.makeKey(image, "info")
	
	entry := &CacheEntry{
		ImageInfo: info,
		Timestamp: time.Now(),
		TTL:       ttl,
	}
	
	// Check if this is a new entry
	_, existed := c.cache.LoadOrStore(key, entry)
	if !existed {
		atomic.AddInt64(&c.stats.Size, 1)
	} else {
		// Update existing entry
		c.cache.Store(key, entry)
	}
}

// Clear removes all entries from the cache
func (c *RegistryCache) Clear() {
	c.cache.Range(func(key, value interface{}) bool {
		c.cache.Delete(key)
		return true
	})
	
	atomic.StoreInt64(&c.stats.Size, 0)
	atomic.AddInt64(&c.stats.Evicted, atomic.LoadInt64(&c.stats.Size))
}

// Stats returns current cache statistics
func (c *RegistryCache) Stats() CacheStats {
	return CacheStats{
		Hits:    atomic.LoadInt64(&c.stats.Hits),
		Misses:  atomic.LoadInt64(&c.stats.Misses),
		Evicted: atomic.LoadInt64(&c.stats.Evicted),
		Size:    atomic.LoadInt64(&c.stats.Size),
	}
}

// Close stops the cache cleanup goroutine
func (c *RegistryCache) Close() {
	if c.cleanupTick != nil {
		c.cleanupTick.Stop()
		close(c.stopCleanup)
	}
}

// makeKey creates a cache key for an image and operation type
func (c *RegistryCache) makeKey(image types.DockerImage, operation string) string {
	// Create a unique key that includes registry, repository, tag, and operation
	return image.Registry + "/" + image.Repository + ":" + image.Tag + "#" + operation
}

// cleanupLoop runs in the background to remove expired entries
func (c *RegistryCache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTick.C:
			c.cleanupExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes all expired entries from the cache
func (c *RegistryCache) cleanupExpired() {
	var keysToDelete []interface{}
	
	// First pass: collect expired keys
	c.cache.Range(func(key, value interface{}) bool {
		entry := value.(*CacheEntry)
		if entry.IsExpired() {
			keysToDelete = append(keysToDelete, key)
		}
		return true
	})
	
	// Second pass: delete expired keys
	for _, key := range keysToDelete {
		c.cache.Delete(key)
		atomic.AddInt64(&c.stats.Evicted, 1)
		atomic.AddInt64(&c.stats.Size, -1)
	}
}

// CachedRegistryClient wraps a registry client with caching capabilities
type CachedRegistryClient struct {
	client types.RegistryClient
	cache  *RegistryCache
}

// NewCachedRegistryClient creates a new cached registry client
func NewCachedRegistryClient(client types.RegistryClient, cache *RegistryCache) *CachedRegistryClient {
	return &CachedRegistryClient{
		client: client,
		cache:  cache,
	}
}

// Name returns the name of the underlying registry client
func (c *CachedRegistryClient) Name() string {
	return c.client.Name()
}

// GetLatestTags gets tags with caching
func (c *CachedRegistryClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	// Try cache first
	if tags, found := c.cache.GetTags(image); found {
		return tags, nil
	}
	
	// Cache miss, fetch from registry
	tags, err := c.client.GetLatestTags(ctx, image)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	c.cache.SetTags(image, tags)
	
	return tags, nil
}

// GetImageInfo gets image info with caching
func (c *CachedRegistryClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	// Try cache first
	if info, found := c.cache.GetImageInfo(image); found {
		return info, nil
	}
	
	// Cache miss, fetch from registry
	info, err := c.client.GetImageInfo(ctx, image)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	c.cache.SetImageInfo(image, info)
	
	return info, nil
}