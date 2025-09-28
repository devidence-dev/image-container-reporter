package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/pkg/types"
)

// Mock implementations for testing
type mockRegistryClient struct {
	name     string
	tags     []string
	err      error
	delay    time.Duration
}

func (m *mockRegistryClient) Name() string {
	return m.name
}

func (m *mockRegistryClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	if m.err != nil {
		return nil, m.err
	}
	
	return m.tags, nil
}

func (m *mockRegistryClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	return nil, errors.New("not implemented")
}

func TestService_ScanDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	tests := []struct {
		name           string
		setupMocks     func() (types.ComposeParser, []types.RegistryClient)
		config         Config
		expectedError  bool
		expectedUpdates int
		expectedUpToDate int
	}{
		{
			name: "successful scan with updates",
			setupMocks: func() (types.ComposeParser, []types.RegistryClient) {
				parser := compose.NewParser()
				
				registry := &mockRegistryClient{
					name: "docker.io",
					tags: []string{"1.22", "1.21", "1.20"},
				}
				
				return parser, []types.RegistryClient{registry}
			},
			config: DefaultConfig(),
			expectedError: false,
			expectedUpdates: 0, // Will depend on test data
			expectedUpToDate: 0,
		},
		{
			name: "registry timeout",
			setupMocks: func() (types.ComposeParser, []types.RegistryClient) {
				parser := compose.NewParser()
				
				registry := &mockRegistryClient{
					name:  "docker.io",
					delay: 2 * time.Second, // Longer than timeout
					tags:  []string{"1.22"},
				}
				
				return parser, []types.RegistryClient{registry}
			},
			config: Config{
				Recursive:       true,
				Patterns:        []string{"docker-compose.yml"},
				MaxConcurrency:  5,
				RegistryTimeout: 100 * time.Millisecond,
			},
			expectedError: false, // Errors are collected, not returned
		},
		{
			name: "registry error",
			setupMocks: func() (types.ComposeParser, []types.RegistryClient) {
				parser := compose.NewParser()
				
				registry := &mockRegistryClient{
					name: "docker.io",
					err:  errors.New("registry unavailable"),
				}
				
				return parser, []types.RegistryClient{registry}
			},
			config: DefaultConfig(),
			expectedError: false, // Errors are collected, not returned
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, registries := tt.setupMocks()
			service := NewService(parser, registries, logger)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := service.ScanDirectory(ctx, "testdata", tt.config)
			
			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if result == nil {
				t.Fatal("Expected result but got nil")
			}
			
			// Basic result validation
			if result.ProjectName == "" {
				t.Error("Expected project name to be set")
			}
			
			if result.ScanTimestamp.IsZero() {
				t.Error("Expected scan timestamp to be set")
			}
		})
	}
}

func TestService_checkImageForUpdates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	tests := []struct {
		name           string
		image          types.DockerImage
		registryTags   []string
		registryError  error
		expectUpdate   bool
		expectUpToDate bool
		expectError    bool
	}{
		{
			name: "update available",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "1.20",
			},
			registryTags:   []string{"1.22", "1.21", "1.20"},
			expectUpdate:   true,
			expectUpToDate: false,
			expectError:    false,
		},
		{
			name: "up to date",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "1.22",
			},
			registryTags:   []string{"1.22", "1.21", "1.20"},
			expectUpdate:   false,
			expectUpToDate: true,
			expectError:    false,
		},
		{
			name: "registry error",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "1.20",
			},
			registryError:  errors.New("registry unavailable"),
			expectUpdate:   false,
			expectUpToDate: false,
			expectError:    true,
		},
		{
			name: "no tags found",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "1.20",
			},
			registryTags:   []string{},
			expectUpdate:   false,
			expectUpToDate: false,
			expectError:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockRegistryClient{
				name: "docker.io",
				tags: tt.registryTags,
				err:  tt.registryError,
			}
			
			service := NewService(nil, []types.RegistryClient{registry}, logger)
			
			updatesChan := make(chan types.ImageUpdate, 1)
			upToDateChan := make(chan string, 1)
			errorsChan := make(chan string, 1)
			
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			
			service.checkImageForUpdates(ctx, "test-service:"+tt.image.String(), tt.image, 
				updatesChan, upToDateChan, errorsChan)
			
			close(updatesChan)
			close(upToDateChan)
			close(errorsChan)
			
			// Check results
			updates := make([]types.ImageUpdate, 0)
			upToDate := make([]string, 0)
			errors := make([]string, 0)
			
			for update := range updatesChan {
				updates = append(updates, update)
			}
			for service := range upToDateChan {
				upToDate = append(upToDate, service)
			}
			for err := range errorsChan {
				errors = append(errors, err)
			}
			
			if tt.expectUpdate && len(updates) == 0 {
				t.Error("Expected update but got none")
			}
			if !tt.expectUpdate && len(updates) > 0 {
				t.Errorf("Expected no update but got %d", len(updates))
			}
			
			if tt.expectUpToDate && len(upToDate) == 0 {
				t.Error("Expected up-to-date result but got none")
			}
			if !tt.expectUpToDate && len(upToDate) > 0 {
				t.Errorf("Expected no up-to-date result but got %d", len(upToDate))
			}
			
			if tt.expectError && len(errors) == 0 {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && len(errors) > 0 {
				t.Errorf("Expected no error but got %d: %v", len(errors), errors)
			}
		})
	}
}

func TestService_canHandleRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(nil, nil, logger)
	
	tests := []struct {
		name         string
		clientName   string
		registry     string
		expected     bool
	}{
		{
			name:       "docker.io client handles docker.io",
			clientName: "docker.io",
			registry:   "docker.io",
			expected:   true,
		},
		{
			name:       "docker.io client handles empty registry",
			clientName: "docker.io",
			registry:   "",
			expected:   true,
		},
		{
			name:       "ghcr client handles ghcr.io",
			clientName: "ghcr.io",
			registry:   "ghcr.io",
			expected:   true,
		},
		{
			name:       "docker.io client cannot handle ghcr.io",
			clientName: "docker.io",
			registry:   "ghcr.io",
			expected:   false,
		},
		{
			name:       "case insensitive matching",
			clientName: "Docker.io",
			registry:   "DOCKER.IO",
			expected:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockRegistryClient{name: tt.clientName}
			result := service.canHandleRegistry(client, tt.registry)
			
			if result != tt.expected {
				t.Errorf("canHandleRegistry(%s, %s) = %v, want %v", 
					tt.clientName, tt.registry, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if !config.Recursive {
		t.Error("Expected recursive to be true by default")
	}
	
	if len(config.Patterns) == 0 {
		t.Error("Expected patterns to be set by default")
	}
	
	if config.MaxConcurrency <= 0 {
		t.Error("Expected max concurrency to be positive")
	}
	
	if config.RegistryTimeout <= 0 {
		t.Error("Expected registry timeout to be positive")
	}
}

func TestService_ConcurrencyControl(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	// Create a slow registry client
	registry := &mockRegistryClient{
		name:  "docker.io",
		delay: 100 * time.Millisecond,
		tags:  []string{"latest"},
	}
	
	service := NewService(nil, []types.RegistryClient{registry}, logger)
	
	// Create multiple images to test concurrency
	images := make(map[string]types.DockerImage)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("service%d:nginx:latest", i)
		images[key] = types.DockerImage{
			Registry:   "docker.io",
			Repository: "nginx",
			Tag:        "latest",
		}
	}
	
	config := Config{
		MaxConcurrency:  5, // Limit concurrency
		RegistryTimeout: time.Second,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	start := time.Now()
	updates, upToDate, errors := service.checkForUpdates(ctx, images, config)
	duration := time.Since(start)
	
	// With 20 images, 100ms delay each, and max concurrency of 5,
	// it should take roughly 400ms (20/5 * 100ms)
	// Allow some margin for overhead
	if duration > 800*time.Millisecond {
		t.Errorf("Concurrency control not working properly. Expected ~400ms, got %v", duration)
	}
	
	totalResults := len(updates) + len(upToDate) + len(errors)
	if totalResults != 20 {
		t.Errorf("Expected 20 total results, got %d", totalResults)
	}
}

func TestService_ContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	// Create a slow registry client
	registry := &mockRegistryClient{
		name:  "docker.io",
		delay: time.Second, // Long delay
		tags:  []string{"latest"},
	}
	
	service := NewService(nil, []types.RegistryClient{registry}, logger)
	
	images := map[string]types.DockerImage{
		"service1:nginx:latest": {
			Registry:   "docker.io",
			Repository: "nginx",
			Tag:        "latest",
		},
	}
	
	config := DefaultConfig()
	
	// Create context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	updates, upToDate, errors := service.checkForUpdates(ctx, images, config)
	
	// Should have been cancelled
	totalResults := len(updates) + len(upToDate) + len(errors)
	if totalResults > 1 {
		t.Errorf("Expected cancellation to limit results, got %d total results", totalResults)
	}
}