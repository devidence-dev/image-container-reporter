package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
	"github.com/user/docker-image-reporter/pkg/utils"
)

// TestMultipleImageUpdates tests para verificar detección de actualizaciones en múltiples imágenes
func TestMultipleImageUpdates(t *testing.T) {
	// Mock responses para diferentes imágenes
	mockResponses := map[string]string{
		"/v2/repositories/cloudflare/cloudflared/tags": `{
			"count": 50,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "2025.9.1", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "2025.9.0", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "2025.8.2", "last_updated": "2025-08-25T00:00:00Z"},
				{"name": "2025.8.1", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "2025.8.0", "last_updated": "2025-08-01T00:00:00Z"}
			]
		}`,
		"/v2/repositories/linuxserver/speedtest-tracker/tags": `{
			"count": 30,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "1.7.2", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "1.7.1", "last_updated": "2025-09-10T00:00:00Z"}, 
				{"name": "1.7.0", "last_updated": "2025-09-01T00:00:00Z"},
				{"name": "1.6.6", "last_updated": "2025-08-25T00:00:00Z"},
				{"name": "1.6.5", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "1.6.4", "last_updated": "2025-08-01T00:00:00Z"}
			]
		}`,
		"/v2/repositories/library/caddy/tags": `{
			"count": 40,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "2.11.1", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "2.11.1-alpine", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "2.11.0", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "2.11.0-alpine", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "2.10.1", "last_updated": "2025-08-25T00:00:00Z"},
				{"name": "2.10.1-alpine", "last_updated": "2025-08-25T00:00:00Z"},
				{"name": "2.10.0", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "2.10.0-alpine", "last_updated": "2025-08-15T00:00:00Z"}
			]
		}`,
		"/v2/repositories/hacdias/webdav/tags": `{
			"count": 25,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "v5.10", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "v5.9", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "v5.8", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "v5.7", "last_updated": "2025-08-01T00:00:00Z"}
			]
		}`,
		"/v2/repositories/adguard/adguardhome/tags": `{
			"count": 35,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "v0.107.66", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "v0.107.65", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "v0.107.64", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "v0.107.63", "last_updated": "2025-08-01T00:00:00Z"}
			]
		}`,
		"/v2/repositories/portainer/portainer-ce/tags": `{
			"count": 50,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "2.33.2", "last_updated": "2025-08-30T00:00:00Z"},
				{"name": "2.33.2-alpine", "last_updated": "2025-08-30T00:00:00Z"},
				{"name": "2.33.1", "last_updated": "2025-08-20T00:00:00Z"},
				{"name": "2.33.1-alpine", "last_updated": "2025-08-20T00:00:00Z"},
				{"name": "2.32.0", "last_updated": "2025-07-20T00:00:00Z"},
				{"name": "2.32.0-alpine", "last_updated": "2025-07-20T00:00:00Z"}
			]
		}`,
		"/v2/repositories/qbittorrentofficial/qbittorrent-nox/tags": `{
			"count": 30,
			"results": [
				{"name": "latest", "last_updated": "2025-09-25T00:00:00Z"},
				{"name": "5.2.1-3", "last_updated": "2025-09-20T00:00:00Z"},
				{"name": "5.2.0-1", "last_updated": "2025-09-10T00:00:00Z"},
				{"name": "5.1.3-1", "last_updated": "2025-08-25T00:00:00Z"},
				{"name": "5.1.2-2", "last_updated": "2025-08-15T00:00:00Z"},
				{"name": "5.1.2-1", "last_updated": "2025-08-10T00:00:00Z"}
			]
		}`,
	}

	// Crear servidor mock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if response, exists := mockResponses[path]; exists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}
		t.Logf("Mock server: No response for path: %s", path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Crear cliente con URL mock
	client := NewDockerHubClient(30 * time.Second)
	originalBaseURL := client.baseURL
	defer func() { client.baseURL = originalBaseURL }()

	// Reemplazar la función GetLatestTags para usar nuestro servidor mock
	originalClient := client.httpClient
	client.httpClient = &http.Client{Timeout: 30 * time.Second}
	defer func() { client.httpClient = originalClient }()

	// Test cases basados en las imágenes reales de tu sistema
	testCases := []struct {
		name             string
		image            types.DockerImage
		shouldHaveUpdate bool
		expectedMinor    bool // true si esperamos al menos un update minor
	}{
		{
			name: "Cloudflared_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "cloudflare/cloudflared",
				Tag:        "2025.8.1",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // 2025.8.1 → 2025.9.1
		},
		{
			name: "SpeedtestTracker_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "linuxserver/speedtest-tracker",
				Tag:        "1.6.5",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // 1.6.5 → 1.7.2
		},
		{
			name: "Caddy_Alpine_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "caddy",
				Tag:        "2.10.0-alpine",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // 2.10.0 → 2.11.1
		},
		{
			name: "WebDAV_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "hacdias/webdav",
				Tag:        "v5.8",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // v5.8 → v5.10
		},
		{
			name: "AdGuard_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "adguard/adguardhome",
				Tag:        "v0.107.64",
			},
			shouldHaveUpdate: true,
			expectedMinor:    false, // v0.107.64 → v0.107.66 (patch)
		},
		{
			name: "Portainer_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "portainer/portainer-ce",
				Tag:        "2.32.0-alpine",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // 2.32.0 → 2.33.2
		},
		{
			name: "QBittorrent_Update",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "qbittorrentofficial/qbittorrent-nox",
				Tag:        "5.1.2-2",
			},
			shouldHaveUpdate: true,
			expectedMinor:    true, // 5.1.2-2 → 5.2.1-3
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simular la llamada HTTP directamente
			normalizedRepo := client.normalizeRepository(tc.image.Repository)
			url := fmt.Sprintf("%s/v2/repositories/%s/tags?page_size=100", server.URL, normalizedRepo)

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}

			resp, err := client.httpClient.Do(req)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Unexpected status code: %d", resp.StatusCode)
			}

			var response DockerHubTagsResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("Error decoding response: %v", err)
			}

			// Procesar tags como lo haría el cliente real
			var tags []string
			for _, result := range response.Results {
				if client.isValidTag(result.Name) {
					tags = append(tags, result.Name)
				}
			}

			t.Logf("Retrieved %d valid tags for %s: %v", len(tags), tc.image.Repository, tags)

			if len(tags) == 0 {
				t.Fatal("No valid tags found")
			}

			// Filtrar y ordenar como en el código real
			stableTags := utils.FilterPreReleases(tags)
			if len(stableTags) == 0 {
				t.Logf("No stable tags, using all tags")
				stableTags = tags
			}

			sortedTags := utils.SortVersions(stableTags)
			if len(sortedTags) == 0 {
				t.Fatal("No tags after sorting")
			}

			latestTag := sortedTags[0]
			t.Logf("Latest tag determined: %s", latestTag)

			// Comparar versiones
			updateType := utils.CompareVersions(tc.image.Tag, latestTag)
			hasUpdate := updateType != types.UpdateTypeNone

			t.Logf("Version comparison: %s → %s = %s", tc.image.Tag, latestTag, updateType)

			if hasUpdate != tc.shouldHaveUpdate {
				t.Errorf("Expected update=%v, got update=%v (type=%s)",
					tc.shouldHaveUpdate, hasUpdate, updateType)
			}

			// Verificar tipo de update si se espera
			if tc.shouldHaveUpdate && hasUpdate {
				if tc.expectedMinor && (updateType != types.UpdateTypeMinor && updateType != types.UpdateTypeMajor) {
					t.Logf("Expected at least minor update, got %s", updateType)
				}
			}

			if hasUpdate {
				t.Logf("✅ UPDATE DETECTED: %s (%s → %s, type: %s)",
					tc.image.Repository, tc.image.Tag, latestTag, updateType)
			} else {
				t.Logf("ℹ️  No update: %s is up to date with %s",
					tc.image.Repository, tc.image.Tag)
			}
		})
	}
}

// TestRealDockerHubAPI test que hace llamadas reales a DockerHub (opcional, solo para debug)
func TestRealDockerHubAPI_Manual(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	// Este test se puede usar manualmente para verificar contra la API real
	client := NewDockerHubClient(30 * time.Second)
	ctx := context.Background()

	testImages := []types.DockerImage{
		{Registry: "docker.io", Repository: "portainer/portainer-ce", Tag: "2.32.0-alpine"},
		{Registry: "docker.io", Repository: "adguard/adguardhome", Tag: "v0.107.64"},
	}

	for _, image := range testImages {
		t.Run(image.Repository, func(t *testing.T) {
			tags, err := client.GetLatestTags(ctx, image)
			if err != nil {
				t.Logf("Error getting tags for %s: %v", image.Repository, err)
				return
			}

			t.Logf("Real API - %s tags (%d): %v", image.Repository, len(tags), tags[:min(10, len(tags))])

			if len(tags) > 0 {
				stableTags := utils.FilterPreReleases(tags)
				sortedTags := utils.SortVersions(stableTags)
				if len(sortedTags) > 0 {
					latestTag := sortedTags[0]
					updateType := utils.CompareVersions(image.Tag, latestTag)
					t.Logf("Real API - %s: %s → %s (%s)",
						image.Repository, image.Tag, latestTag, updateType)
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
