package utils

import (
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestVersionRegression_RealWorldTags(t *testing.T) {
	tests := []struct {
		name         string
		current      string
		tags         []string
		expectUpdate bool
		expectedType types.UpdateType
	}{
		{
			name:    "Portainer alpine minor update",
			current: "2.32.0-alpine",
			tags: []string{
				"latest", "2.33.2", "2.33.2-alpine", "2.33.1", "2.33.1-alpine",
				"2.33.0", "2.33.0-alpine", "2.32.1", "2.32.1-alpine", "2.32.0", "2.32.0-alpine",
				"linux-amd64-2.33.2", "nightly", "dev-branch", "abc123def456",
			},
			expectUpdate: true,
			expectedType: types.UpdateTypeMinor,
		},
		{
			name:         "Cloudflared minor update",
			current:      "2025.8.1",
			tags:         []string{"latest", "2025.9.1", "2025.9.0", "2025.8.2", "2025.8.1", "2025.8.0"},
			expectUpdate: true,
			expectedType: types.UpdateTypeMinor,
		},
		{
			name:         "AdGuard patch update",
			current:      "v0.107.64",
			tags:         []string{"latest", "v0.107.66", "v0.107.65", "v0.107.64", "v0.107.63"},
			expectUpdate: true,
			expectedType: types.UpdateTypePatch,
		},
		{
			name:         "Same version without suffix",
			current:      "1.20.0-alpine",
			tags:         []string{"1.20.0", "1.20.0-alpine", "1.19.9-alpine", "latest"},
			expectUpdate: false,
			expectedType: types.UpdateTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stableTags := FilterPreReleases(tt.tags)
			if len(stableTags) == 0 {
				stableTags = tt.tags
			}

			suffixFiltered := FilterTagsBySuffix(stableTags, tt.current)
			tagsToUse := stableTags
			if len(suffixFiltered) > 0 {
				tagsToUse = suffixFiltered
			}

			latestTag := FindBestUpdateTag(tt.current, tagsToUse)
			if latestTag == "" {
				sorted := SortVersions(tagsToUse)
				if len(sorted) == 0 {
					t.Fatalf("no candidate tags available for %q", tt.current)
				}
				latestTag = sorted[0]
			}

			updateType := CompareVersions(tt.current, latestTag)
			hasUpdate := updateType != types.UpdateTypeNone

			if hasUpdate != tt.expectUpdate {
				t.Fatalf("expected update=%v, got update=%v (current=%s latest=%s type=%s)", tt.expectUpdate, hasUpdate, tt.current, latestTag, updateType)
			}

			if tt.expectUpdate && updateType != tt.expectedType {
				t.Fatalf("expected update type %s, got %s (current=%s latest=%s)", tt.expectedType, updateType, tt.current, latestTag)
			}
		})
	}
}
