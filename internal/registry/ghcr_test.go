package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestGHCRClient_Name(t *testing.T) {
	client := NewGHCRClient("test-token", 30*time.Second)
	if client.Name() != "ghcr.io" {
		t.Errorf("Expected name 'ghcr.io', got '%s'", client.Name())
	}
}

func TestGHCRClient_parseRepository(t *testing.T) {
	client := NewGHCRClient("test-token", 30*time.Second)

	tests := []struct {
		input         string
		expectedOwner string
		expectedPkg   string
	}{
		{"user/myapp", "user", "myapp"},
		{"ghcr.io/user/myapp", "user", "myapp"},
		{"org/namespace/app", "org", "namespace/app"},
		{"ghcr.io/org/namespace/app", "org", "namespace/app"},
		{"invalid", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, pkg := client.parseRepository(tt.input)
			if owner != tt.expectedOwner {
				t.Errorf("parseRepository(%s) owner = %s, want %s", tt.input, owner, tt.expectedOwner)
			}
			if pkg != tt.expectedPkg {
				t.Errorf("parseRepository(%s) package = %s, want %s", tt.input, pkg, tt.expectedPkg)
			}
		})
	}
}

func TestGHCRClient_isValidTag(t *testing.T) {
	client := NewGHCRClient("test-token", 30*time.Second)

	tests := []struct {
		tag      string
		expected bool
	}{
		{"v1.0.0", true},
		{"1.20", true},
		{"latest", true},
		{"main", true},
		{"", false},
		{"abc123def456", false}, // SHA-like
		{"1234567890abcdef", false}, // SHA-like
		{"temp-build", false},
		{"tmp-123", false},
		{"feature-branch", true},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			result := client.isValidTag(tt.tag)
			if result != tt.expected {
				t.Errorf("isValidTag(%s) = %v, want %v", tt.tag, result, tt.expected)
			}
		})
	}
}

func TestGHCRClient_GetLatestTags_MockServer(t *testing.T) {
	// Crear servidor mock para GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verificar autenticación
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/orgs/user/packages/container/myapp/versions" {
			response := `[
				{
					"id": 1,
					"name": "v1.0.0",
					"created_at": "2023-01-01T00:00:00Z",
					"updated_at": "2023-01-01T00:00:00Z",
					"metadata": {
						"package_type": "container",
						"container": {
							"tags": ["v1.0.0", "latest"]
						}
					}
				},
				{
					"id": 2,
					"name": "v1.1.0",
					"created_at": "2023-02-01T00:00:00Z",
					"updated_at": "2023-02-01T00:00:00Z",
					"metadata": {
						"package_type": "container",
						"container": {
							"tags": ["v1.1.0", "temp-build"]
						}
					}
				}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewGHCRClient("test-token", 30*time.Second)
	// Cambiar la URL base para usar el servidor mock
	client.baseURL = server.URL

	image := types.DockerImage{
		Registry:   "ghcr.io",
		Repository: "user/myapp",
		Tag:        "latest",
	}

	ctx := context.Background()
	tags, err := client.GetLatestTags(ctx, image)

	if err != nil {
		t.Fatalf("GetLatestTags failed: %v", err)
	}

	// Verificar que se obtuvieron tags válidos
	expectedValidTags := []string{"v1.0.0", "latest", "v1.1.0"}
	expectedInvalidTags := []string{"temp-build"}

	foundValid := make(map[string]bool)
	for _, tag := range tags {
		foundValid[tag] = true
		// Verificar que no hay tags inválidos
		for _, invalid := range expectedInvalidTags {
			if tag == invalid {
				t.Errorf("Found invalid tag in results: %s", tag)
			}
		}
	}

	// Verificar que se encontraron los tags válidos esperados
	for _, expected := range expectedValidTags {
		if client.isValidTag(expected) && !foundValid[expected] {
			t.Errorf("Expected valid tag not found: %s", expected)
		}
	}
}

func TestGHCRClient_GetImageInfo_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/orgs/user/packages/container/myapp" {
			response := `{
				"id": 123,
				"name": "myapp",
				"package_type": "container",
				"owner": {
					"login": "user",
					"type": "User"
				},
				"version_count": 5,
				"visibility": "public",
				"created_at": "2023-01-01T00:00:00Z",
				"updated_at": "2023-02-01T00:00:00Z"
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}

		// Mock para versions endpoint (llamado por GetLatestTags)
		if r.URL.Path == "/orgs/user/packages/container/myapp/versions" {
			response := `[
				{
					"id": 1,
					"name": "v1.0.0",
					"metadata": {
						"package_type": "container",
						"container": {
							"tags": ["v1.0.0"]
						}
					}
				}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewGHCRClient("test-token", 30*time.Second)
	client.baseURL = server.URL

	image := types.DockerImage{
		Registry:   "ghcr.io",
		Repository: "user/myapp",
		Tag:        "latest",
	}

	ctx := context.Background()
	info, err := client.GetImageInfo(ctx, image)

	if err != nil {
		t.Fatalf("GetImageInfo failed: %v", err)
	}

	if info == nil {
		t.Fatal("Expected ImageInfo, got nil")
	}

	// Verificar que se parseó correctamente el tiempo
	expectedTime := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	if !info.LastModified.Equal(expectedTime) {
		t.Errorf("Expected LastModified %v, got %v", expectedTime, info.LastModified)
	}

	// Verificar que hay tags
	if len(info.Tags) == 0 {
		t.Error("Expected tags in ImageInfo")
	}
}

func TestGHCRClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewGHCRClient("invalid-token", 30*time.Second)
	client.baseURL = server.URL

	image := types.DockerImage{
		Registry:   "ghcr.io",
		Repository: "user/myapp",
		Tag:        "latest",
	}

	ctx := context.Background()
	_, err := client.GetLatestTags(ctx, image)

	if err == nil {
		t.Error("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error, got: %v", err)
	}
}

func TestParseGitHubTime(t *testing.T) {
	tests := []struct {
		input    string
		expected bool // true if parsing should succeed
	}{
		{"2023-01-01T00:00:00Z", true},
		{"2023-01-01T00:00:00.123Z", true},
		{"2023-01-01T00:00:00+00:00", true},
		{"invalid-time", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseGitHubTime(tt.input)
			
			if tt.expected {
				// Verificar que el tiempo parseado no sea el tiempo actual
				now := time.Now()
				diff := now.Sub(result)
				if diff < time.Hour && tt.input != "invalid-time" {
					// Si la diferencia es menos de una hora, probablemente falló
					t.Errorf("Expected successful parsing for %s", tt.input)
				}
			} else {
				// Para casos inválidos, debería devolver tiempo actual
				now := time.Now()
				diff := now.Sub(result)
				if diff > time.Minute {
					t.Errorf("Expected fallback to current time for invalid input")
				}
			}
		})
	}
}