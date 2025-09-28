package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

// Mock registry client for testing
type mockRegistryClient struct {
	name      string
	tags      []string
	imageInfo *types.ImageInfo
	err       error
	callCount int
}

func (m *mockRegistryClient) Name() string {
	return m.name
}

func (m *mockRegistryClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockRegistryClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.imageInfo, nil
}

func TestRegistryCache_GetSetTags(t *testing.T) {
	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	tags := []string{"latest", "1.21", "1.20"}

	// Test cache miss
	if cachedTags, found := cache.GetTags(image); found {
		t.Errorf("Expected cache miss, but got tags: %v", cachedTags)
	}

	// Set tags in cache
	cache.SetTags(image, tags)

	// Test cache hit
	cachedTags, found := cache.GetTags(image)
	if !found {
		t.Error("Expected cache hit, but got miss")
	}

	if len(cachedTags) != len(tags) {
		t.Errorf("Expected %d tags, got %d", len(tags), len(cachedTags))
	}

	for i, tag := range tags {
		if cachedTags[i] != tag {
			t.Errorf("Expected tag %s at index %d, got %s", tag, i, cachedTags[i])
		}
	}
}

func TestRegistryCache_TTLExpiration(t *testing.T) {
	config := Config{
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: 0, // Disable automatic cleanup for this test
	}
	cache := NewRegistryCache(config)
	defer cache.Close()

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	tags := []string{"latest", "1.21"}

	// Set tags with short TTL
	cache.SetTagsWithTTL(image, tags, 50*time.Millisecond)

	// Should be available immediately
	if _, found := cache.GetTags(image); !found {
		t.Error("Expected cache hit immediately after setting")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should be expired now
	if _, found := cache.GetTags(image); found {
		t.Error("Expected cache miss after TTL expiration")
	}
}

func TestRegistryCache_Stats(t *testing.T) {
	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	// Initial stats should be zero
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Size != 0 {
		t.Errorf("Expected zero stats initially, got: %+v", stats)
	}

	// Cache miss should increment misses
	cache.GetTags(image)
	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Set and get should increment hits and size
	cache.SetTags(image, []string{"latest"})
	cache.GetTags(image)

	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}

	// Hit rate should be 50% (1 hit, 1 miss)
	expectedHitRate := 50.0
	if stats.HitRate() != expectedHitRate {
		t.Errorf("Expected hit rate %.1f%%, got %.1f%%", expectedHitRate, stats.HitRate())
	}
}

func TestRegistryCache_Clear(t *testing.T) {
	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	image1 := types.DockerImage{Registry: "docker.io", Repository: "nginx", Tag: "latest"}
	image2 := types.DockerImage{Registry: "docker.io", Repository: "redis", Tag: "latest"}

	// Add some entries
	cache.SetTags(image1, []string{"latest"})
	cache.SetTags(image2, []string{"latest"})

	stats := cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2 before clear, got %d", stats.Size)
	}

	// Clear cache
	cache.Clear()

	// Check that cache is empty
	if _, found := cache.GetTags(image1); found {
		t.Error("Expected cache miss after clear")
	}

	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", stats.Size)
	}
}

func TestRegistryCache_CleanupExpired(t *testing.T) {
	config := Config{
		DefaultTTL:      100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}
	cache := NewRegistryCache(config)
	defer cache.Close()

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	// Set entry with short TTL
	cache.SetTagsWithTTL(image, []string{"latest"}, 50*time.Millisecond)

	// Verify it's there
	if _, found := cache.GetTags(image); !found {
		t.Error("Expected cache hit immediately after setting")
	}

	// Wait for cleanup to run (should happen after 50ms + some buffer)
	time.Sleep(150 * time.Millisecond)

	// Entry should be cleaned up
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", stats.Size)
	}
	if stats.Evicted == 0 {
		t.Error("Expected at least 1 eviction after cleanup")
	}
}

func TestCachedRegistryClient_GetLatestTags(t *testing.T) {
	mockClient := &mockRegistryClient{
		name: "docker.io",
		tags: []string{"latest", "1.21", "1.20"},
	}

	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	cachedClient := NewCachedRegistryClient(mockClient, cache)

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	ctx := context.Background()

	// First call should hit the registry
	tags1, err := cachedClient.GetLatestTags(ctx, image)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if mockClient.callCount != 1 {
		t.Errorf("Expected 1 registry call, got %d", mockClient.callCount)
	}

	// Second call should hit the cache
	tags2, err := cachedClient.GetLatestTags(ctx, image)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if mockClient.callCount != 1 {
		t.Errorf("Expected still 1 registry call (cached), got %d", mockClient.callCount)
	}

	// Results should be identical
	if len(tags1) != len(tags2) {
		t.Errorf("Tag lengths differ: %d vs %d", len(tags1), len(tags2))
	}

	for i, tag := range tags1 {
		if tags2[i] != tag {
			t.Errorf("Tag mismatch at index %d: %s vs %s", i, tag, tags2[i])
		}
	}

	// Verify cache stats
	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.Misses)
	}
}

func TestCachedRegistryClient_ErrorHandling(t *testing.T) {
	mockClient := &mockRegistryClient{
		name: "docker.io",
		err:  errors.New("registry unavailable"),
	}

	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	cachedClient := NewCachedRegistryClient(mockClient, cache)

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	ctx := context.Background()

	// Error should be propagated
	_, err := cachedClient.GetLatestTags(ctx, image)
	if err == nil {
		t.Error("Expected error to be propagated")
	}

	if err.Error() != "registry unavailable" {
		t.Errorf("Expected 'registry unavailable', got '%s'", err.Error())
	}

	// Error should not be cached
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected cache size 0 after error, got %d", stats.Size)
	}
}

func TestRegistryCache_ConcurrentAccess(t *testing.T) {
	cache := NewRegistryCache(DefaultConfig())
	defer cache.Close()

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	tags := []string{"latest", "1.21", "1.20"}

	// Set initial value
	cache.SetTags(image, tags)

	// Run concurrent reads and writes
	done := make(chan bool, 20)

	// Start 10 readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.GetTags(image)
			}
			done <- true
		}()
	}

	// Start 10 writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				cache.SetTags(image, append(tags, "writer-"+string(rune(id))))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// Cache should still be functional
	if _, found := cache.GetTags(image); !found {
		t.Error("Cache should still be functional after concurrent access")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DefaultTTL <= 0 {
		t.Error("Default TTL should be positive")
	}

	if config.CleanupInterval <= 0 {
		t.Error("Cleanup interval should be positive")
	}

	if config.DefaultTTL <= config.CleanupInterval {
		t.Error("Default TTL should be longer than cleanup interval")
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	// Fresh entry
	entry := &CacheEntry{
		Timestamp: time.Now(),
		TTL:       time.Minute,
	}

	if entry.IsExpired() {
		t.Error("Fresh entry should not be expired")
	}

	// Expired entry
	entry.Timestamp = time.Now().Add(-2 * time.Minute)

	if !entry.IsExpired() {
		t.Error("Old entry should be expired")
	}
}

func TestCacheStats_HitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{"no requests", 0, 0, 0.0},
		{"all hits", 10, 0, 100.0},
		{"all misses", 0, 10, 0.0},
		{"50% hit rate", 5, 5, 50.0},
		{"75% hit rate", 75, 25, 75.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CacheStats{
				Hits:   tt.hits,
				Misses: tt.misses,
			}

			hitRate := stats.HitRate()
			if hitRate != tt.expected {
				t.Errorf("Expected hit rate %.1f%%, got %.1f%%", tt.expected, hitRate)
			}
		})
	}
}
