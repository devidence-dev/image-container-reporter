package utils

import (
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		newVersion     string
		expected       types.UpdateType
	}{
		// Semantic version tests
		{
			name:           "major update",
			currentVersion: "1.0.0",
			newVersion:     "2.0.0",
			expected:       types.UpdateTypeMajor,
		},
		{
			name:           "minor update",
			currentVersion: "1.0.0",
			newVersion:     "1.1.0",
			expected:       types.UpdateTypeMinor,
		},
		{
			name:           "patch update",
			currentVersion: "1.0.0",
			newVersion:     "1.0.1",
			expected:       types.UpdateTypePatch,
		},
		{
			name:           "no update - same version",
			currentVersion: "1.0.0",
			newVersion:     "1.0.0",
			expected:       types.UpdateTypeNone,
		},
		{
			name:           "no update - older version",
			currentVersion: "1.1.0",
			newVersion:     "1.0.0",
			expected:       types.UpdateTypeNone,
		},
		{
			name:           "version with v prefix",
			currentVersion: "v1.0.0",
			newVersion:     "v1.1.0",
			expected:       types.UpdateTypeMinor,
		},
		{
			name:           "mixed v prefix",
			currentVersion: "1.0.0",
			newVersion:     "v1.1.0",
			expected:       types.UpdateTypeMinor,
		},
		// Non-semantic version tests
		{
			name:           "string comparison - newer",
			currentVersion: "latest",
			newVersion:     "stable",
			expected:       types.UpdateTypeUnknown,
		},
		{
			name:           "string comparison - same",
			currentVersion: "latest",
			newVersion:     "latest",
			expected:       types.UpdateTypeNone,
		},
		{
			name:           "docker tag with suffix",
			currentVersion: "1.0.0-alpine",
			newVersion:     "1.1.0-alpine",
			expected:       types.UpdateTypeMinor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.currentVersion, tt.newVersion)
			if result != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %v, want %v",
					tt.currentVersion, tt.newVersion, result, tt.expected)
			}
		})
	}
}

func TestIsPreRelease(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{"stable version", "1.0.0", false},
		{"alpha version", "1.0.0-alpha", true},
		{"beta version", "1.0.0-beta.1", true},
		{"rc version", "1.0.0-rc.1", true},
		{"dev version", "1.0.0-dev", true},
		{"nightly version", "nightly", true},
		{"development version", "development", true},
		{"canary version", "canary-123", true},
		{"latest tag", "latest", false},
		{"stable tag", "stable", false},
		{"version with Alpha uppercase", "1.0.0-Alpha", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPreRelease(tt.version)
			if result != tt.expected {
				t.Errorf("IsPreRelease(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

func TestIsSemanticVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{"simple semver", "1.0.0", true},
		{"semver with v prefix", "v1.0.0", true},
		{"semver with pre-release", "1.0.0-alpha", true},
		{"semver with build metadata", "1.0.0+build.1", true},
		{"non-semantic tag", "latest", false},
		{"non-semantic tag", "stable", false},
		{"partial version", "1.0", false},
		{"single number", "1", false},
		{"docker tag with suffix", "1.0.0-alpine", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSemanticVersion(tt.version)
			if result != tt.expected {
				t.Errorf("IsSemanticVersion(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

func TestFilterPreReleases(t *testing.T) {
	input := []string{
		"1.0.0",
		"1.1.0-alpha",
		"1.1.0-beta.1",
		"1.1.0",
		"1.2.0-rc.1",
		"1.2.0",
		"nightly",
		"latest",
		"dev-branch",
	}

	expected := []string{
		"1.0.0",
		"1.1.0",
		"1.2.0",
		"latest",
	}

	result := FilterPreReleases(input)

	if len(result) != len(expected) {
		t.Errorf("FilterPreReleases() returned %d items, want %d", len(result), len(expected))
		return
	}

	for i, version := range expected {
		if result[i] != version {
			t.Errorf("FilterPreReleases()[%d] = %q, want %q", i, result[i], version)
		}
	}
}

func TestSortVersions(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "semantic versions",
			input:    []string{"1.0.0", "1.2.0", "1.1.0", "2.0.0"},
			expected: []string{"2.0.0", "1.2.0", "1.1.0", "1.0.0"},
		},
		{
			name:     "mixed versions",
			input:    []string{"1.0.0", "latest", "1.1.0", "stable"},
			expected: []string{"1.1.0", "1.0.0", "stable", "latest"},
		},
		{
			name:     "versions with v prefix",
			input:    []string{"v1.0.0", "v1.2.0", "v1.1.0"},
			expected: []string{"v1.2.0", "v1.1.0", "v1.0.0"},
		},
		{
			name:     "single version",
			input:    []string{"1.0.0"},
			expected: []string{"1.0.0"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SortVersions(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("SortVersions() returned %d items, want %d", len(result), len(tt.expected))
				return
			}

			for i, version := range tt.expected {
				if result[i] != version {
					t.Errorf("SortVersions()[%d] = %q, want %q", i, result[i], version)
				}
			}
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "semantic versions",
			input:    []string{"1.0.0", "1.2.0", "1.1.0", "2.0.0"},
			expected: "2.0.0",
		},
		{
			name:     "mixed versions",
			input:    []string{"1.0.0", "latest", "1.1.0"},
			expected: "1.1.0",
		},
		{
			name:     "single version",
			input:    []string{"1.0.0"},
			expected: "1.0.0",
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLatestVersion(tt.input)
			if result != tt.expected {
				t.Errorf("GetLatestVersion(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetLatestVersionExcludingPreReleases(t *testing.T) {
	input := []string{
		"1.0.0",
		"1.1.0-alpha",
		"1.1.0-beta.1",
		"1.1.0",
		"1.2.0-rc.1",
		"2.0.0-alpha",
	}

	expected := "1.1.0"
	result := GetLatestVersionExcludingPreReleases(input)

	if result != expected {
		t.Errorf("GetLatestVersionExcludingPreReleases() = %q, want %q", result, expected)
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"version with v prefix", "v1.0.0", "1.0.0"},
		{"version without prefix", "1.0.0", "1.0.0"},
		{"version with alpine suffix", "1.0.0-alpine", "1.0.0"},
		{"version with slim suffix", "1.0.0-slim", "1.0.0"},
		{"version with scratch suffix", "1.0.0-scratch", "1.0.0"},
		{"version with v prefix and suffix", "v1.0.0-alpine", "1.0.0"},
		{"plain tag", "latest", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultUpdateFilter(t *testing.T) {
	filter := DefaultUpdateFilter()

	if filter.IncludePreReleases {
		t.Error("DefaultUpdateFilter().IncludePreReleases should be false")
	}

	if filter.MinUpdateType != types.UpdateTypePatch {
		t.Errorf("DefaultUpdateFilter().MinUpdateType = %v, want %v",
			filter.MinUpdateType, types.UpdateTypePatch)
	}

	expectedPatterns := []string{"nightly", "snapshot", "temp", "tmp"}
	if len(filter.ExcludePatterns) != len(expectedPatterns) {
		t.Errorf("DefaultUpdateFilter().ExcludePatterns length = %d, want %d",
			len(filter.ExcludePatterns), len(expectedPatterns))
	}
}

func TestShouldIncludeUpdate(t *testing.T) {
	tests := []struct {
		name             string
		currentVersion   string
		candidateVersion string
		filter           UpdateFilter
		expected         bool
	}{
		{
			name:             "patch update allowed",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.1",
			filter:           DefaultUpdateFilter(),
			expected:         true,
		},
		{
			name:             "minor update allowed",
			currentVersion:   "1.0.0",
			candidateVersion: "1.1.0",
			filter:           DefaultUpdateFilter(),
			expected:         true,
		},
		{
			name:             "pre-release excluded by default",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.1-alpha",
			filter:           DefaultUpdateFilter(),
			expected:         false,
		},
		{
			name:             "pre-release included when allowed",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.1-alpha",
			filter:           UpdateFilter{IncludePreReleases: true, MinUpdateType: types.UpdateTypePatch},
			expected:         true,
		},
		{
			name:             "patch excluded when min is minor",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.1",
			filter:           UpdateFilter{MinUpdateType: types.UpdateTypeMinor},
			expected:         false,
		},
		{
			name:             "excluded pattern matched",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.1-nightly",
			filter:           DefaultUpdateFilter(),
			expected:         false,
		},
		{
			name:             "same version excluded",
			currentVersion:   "1.0.0",
			candidateVersion: "1.0.0",
			filter:           DefaultUpdateFilter(),
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldIncludeUpdate(tt.currentVersion, tt.candidateVersion, tt.filter)
			if result != tt.expected {
				t.Errorf("ShouldIncludeUpdate(%q, %q, %+v) = %v, want %v",
					tt.currentVersion, tt.candidateVersion, tt.filter, result, tt.expected)
			}
		})
	}
}

func TestFilterUpdates(t *testing.T) {
	currentVersion := "1.0.0"
	availableVersions := []string{
		"0.9.0",         // older
		"1.0.0",         // same
		"1.0.1",         // patch
		"1.0.2-alpha",   // patch pre-release
		"1.1.0",         // minor
		"1.1.0-beta",    // minor pre-release
		"2.0.0",         // major
		"1.0.1-nightly", // excluded pattern
	}

	tests := []struct {
		name     string
		filter   UpdateFilter
		expected []string
	}{
		{
			name:     "default filter",
			filter:   DefaultUpdateFilter(),
			expected: []string{"1.0.1", "1.1.0", "2.0.0"},
		},
		{
			name: "include pre-releases",
			filter: UpdateFilter{
				IncludePreReleases: true,
				MinUpdateType:      types.UpdateTypePatch,
				ExcludePatterns:    []string{"nightly"},
			},
			expected: []string{"1.0.1", "1.0.2-alpha", "1.1.0", "1.1.0-beta", "2.0.0"},
		},
		{
			name: "only major updates",
			filter: UpdateFilter{
				IncludePreReleases: false,
				MinUpdateType:      types.UpdateTypeMajor,
				ExcludePatterns:    []string{},
			},
			expected: []string{"2.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterUpdates(currentVersion, availableVersions, tt.filter)

			if len(result) != len(tt.expected) {
				t.Errorf("FilterUpdates() returned %d items, want %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			for i, version := range tt.expected {
				if result[i] != version {
					t.Errorf("FilterUpdates()[%d] = %q, want %q", i, result[i], version)
				}
			}
		})
	}
}

func TestGetSignificantUpdates(t *testing.T) {
	currentVersion := "1.0.0"
	availableVersions := []string{
		"1.0.1",       // patch - should be excluded
		"1.0.2-alpha", // patch pre-release - should be excluded
		"1.1.0",       // minor - should be included
		"1.1.0-beta",  // minor pre-release - should be excluded
		"2.0.0",       // major - should be included
	}

	expected := []string{"1.1.0", "2.0.0"}
	result := GetSignificantUpdates(currentVersion, availableVersions)

	if len(result) != len(expected) {
		t.Errorf("GetSignificantUpdates() returned %d items, want %d", len(result), len(expected))
		return
	}

	for i, version := range expected {
		if result[i] != version {
			t.Errorf("GetSignificantUpdates()[%d] = %q, want %q", i, result[i], version)
		}
	}
}

func TestGetAllStableUpdates(t *testing.T) {
	currentVersion := "1.0.0"
	availableVersions := []string{
		"1.0.1",         // patch - should be included
		"1.0.2-alpha",   // patch pre-release - should be excluded
		"1.1.0",         // minor - should be included
		"1.1.0-beta",    // minor pre-release - should be excluded
		"2.0.0",         // major - should be included
		"1.0.1-nightly", // excluded pattern - should be excluded
	}

	expected := []string{"1.0.1", "1.1.0", "2.0.0"}
	result := GetAllStableUpdates(currentVersion, availableVersions)

	if len(result) != len(expected) {
		t.Errorf("GetAllStableUpdates() returned %d items, want %d", len(result), len(expected))
		return
	}

	for i, version := range expected {
		if result[i] != version {
			t.Errorf("GetAllStableUpdates()[%d] = %q, want %q", i, result[i], version)
		}
	}
}

func TestClassifyVersionUpdate(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		newVersion     string
		expectedType   types.UpdateType
		expectedPre    bool
		expectedSig    bool
		expectedDesc   string
	}{
		{
			name:           "major update",
			currentVersion: "1.0.0",
			newVersion:     "2.0.0",
			expectedType:   types.UpdateTypeMajor,
			expectedPre:    false,
			expectedSig:    true,
			expectedDesc:   "Major update available",
		},
		{
			name:           "minor update",
			currentVersion: "1.0.0",
			newVersion:     "1.1.0",
			expectedType:   types.UpdateTypeMinor,
			expectedPre:    false,
			expectedSig:    true,
			expectedDesc:   "Minor update available",
		},
		{
			name:           "patch update",
			currentVersion: "1.0.0",
			newVersion:     "1.0.1",
			expectedType:   types.UpdateTypePatch,
			expectedPre:    false,
			expectedSig:    false,
			expectedDesc:   "Patch update available",
		},
		{
			name:           "pre-release update",
			currentVersion: "1.0.0",
			newVersion:     "1.1.0-alpha",
			expectedType:   types.UpdateTypeMinor,
			expectedPre:    true,
			expectedSig:    true,
			expectedDesc:   "Minor update available (pre-release)",
		},
		{
			name:           "no update",
			currentVersion: "1.0.0",
			newVersion:     "1.0.0",
			expectedType:   types.UpdateTypeNone,
			expectedPre:    false,
			expectedSig:    false,
			expectedDesc:   "No update available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyVersionUpdate(tt.currentVersion, tt.newVersion)

			if result.UpdateType != tt.expectedType {
				t.Errorf("UpdateType = %v, want %v", result.UpdateType, tt.expectedType)
			}

			if result.IsPreRelease != tt.expectedPre {
				t.Errorf("IsPreRelease = %v, want %v", result.IsPreRelease, tt.expectedPre)
			}

			if result.IsSignificant != tt.expectedSig {
				t.Errorf("IsSignificant = %v, want %v", result.IsSignificant, tt.expectedSig)
			}

			if result.Description != tt.expectedDesc {
				t.Errorf("Description = %q, want %q", result.Description, tt.expectedDesc)
			}
		})
	}
}
