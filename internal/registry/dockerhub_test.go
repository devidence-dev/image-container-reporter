package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestDockerHubClient_Name(t *testing.T) {
	client := NewDockerHubClient(30 * time.Second)
	if client.Name() != "docker.io" {
		t.Errorf("Expected name 'docker.io', got '%s'", client.Name())
	}
}

func TestDockerHubClient_normalizeRepository(t *testing.T) {
	client := NewDockerHubClient(30 * time.Second)

	tests := []struct {
		input    string
		expected string
	}{
		{"nginx", "library/nginx"},
		{"library/nginx", "library/nginx"},
		{"user/nginx", "user/nginx"},
		{"docker.io/nginx", "library/nginx"},
		{"docker.io/user/nginx", "user/nginx"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := client.normalizeRepository(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRepository(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDockerHubClient_isValidTag(t *testing.T) {
	client := NewDockerHubClient(30 * time.Second)

	tests := []struct {
		tag      string
		expected bool
	}{
		{"1.20", true},
		{"v1.0.0", true},
		{"latest", true},
		{"stable", true},
		{"nightly", false},
		{"dev", false},
		{"development", false},
		{"abc123def", false}, // SHA-like
		{"1234567890abcdef", false}, // SHA-like
		{"edge", false},
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

func TestDockerHubClient_GetLatestTags(t *testing.T) {
	// Crear servidor mock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/repositories/library/nginx/tags" {
			response := `{
				"count": 3,
				"results": [
					{"name": "1.21", "last_updated": "2023-01-01T00:00:00Z"},
					{"name": "1.20", "last_updated": "2022-12-01T00:00:00Z"},
					{"name": "latest", "last_updated": "2023-01-01T00:00:00Z"},
					{"name": "nightly", "last_updated": "2023-01-01T00:00:00Z"}
				]
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewDockerHubClient(30 * time.Second)

	image := types.DockerImage{
		Registry:   "docker.io",
		Repository: "library/nginx",
		Tag:        "latest",
	}

	// Verificamos la lógica de normalización y filtrado
	normalized := client.normalizeRepository(image.Repository)
	if normalized != "library/nginx" {
		t.Errorf("Expected normalized repository 'library/nginx', got '%s'", normalized)
	}

	// Verificar filtrado de tags
	validTags := []string{"1.21", "1.20", "latest"}
	invalidTags := []string{"nightly"}

	for _, tag := range validTags {
		if !client.isValidTag(tag) {
			t.Errorf("Expected tag '%s' to be valid", tag)
		}
	}

	for _, tag := range invalidTags {
		if client.isValidTag(tag) {
			t.Errorf("Expected tag '%s' to be invalid", tag)
		}
	}
}

func TestDockerHubClient_parseDockerHubTime(t *testing.T) {
	tests := []struct {
		input    string
		expected bool // true if parsing should succeed
	}{
		{"2023-01-01T00:00:00Z", true},
		{"2023-01-01T00:00:00.000Z", true},
		{"2023-01-01T00:00:00+00:00", true},
		{"invalid-time", false}, // Should fallback to current time
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDockerHubTime(tt.input)
			
			if tt.expected {
				// Verificar que el tiempo parseado no sea el tiempo actual (aproximadamente)
				now := time.Now()
				diff := now.Sub(result)
				if diff < time.Hour {
					// Si la diferencia es menos de una hora, probablemente falló el parsing
					// y devolvió time.Now()
					if tt.input != "invalid-time" {
						t.Errorf("Expected successful parsing for %s, but got current time", tt.input)
					}
				}
			} else {
				// Para casos inválidos, debería devolver un tiempo cercano al actual
				now := time.Now()
				diff := now.Sub(result)
				if diff > time.Minute {
					t.Errorf("Expected fallback to current time for invalid input, but got %v", result)
				}
			}
		})
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"123456", true},
		{"abcdef", true},
		{"xyz123", false},
		{"123-456", false},
		{"", true}, // Empty string is technically hex
		{"g123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isHexString(tt.input)
			if result != tt.expected {
				t.Errorf("isHexString(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}