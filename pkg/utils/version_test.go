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
		{
			name:           "two-part version compared to higher major",
			currentVersion: "18.1",
			newVersion:     "19",
			expected:       types.UpdateTypeMajor,
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
		{"version with b suffix", "v0.108.0-b.76", true},
		{"version with a suffix", "1.0.0-a.1", true},
		{"version with pre suffix", "1.0.0-pre", true},
		{"version with preview suffix", "1.0.0-preview", true},
		{"version with unstable suffix", "1.0.0-unstable", true},
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
		{"two-part version", "1.0", true},
		{"single number", "1", true},
		{"docker tag with suffix", "1.0.0-alpine", true},
		{"two-part postgres style", "18.1", true},
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
		"v0.108.0-b.76",
		"1.0.0-a.1",
		"1.0.0-pre",
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
			name:     "two-part numeric sorted over names",
			input:    []string{"trixie", "19", "18.1", "latest"},
			expected: []string{"19", "18.1", "trixie", "latest"},
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

func TestFindBestUpdateTagPrefersSemverOverCodenames(t *testing.T) {
	current := "18.1"
	tags := []string{"trixie", "bookworm", "18.1", "18.2", "19", "latest"}

	best := FindBestUpdateTag(current, tags)

	if best != "19" {
		t.Fatalf("expected best tag to be 19, got %s", best)
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
			result := NormalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
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

func TestExtractVersionSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"alpine suffix", "2.10.0-alpine", "-alpine"},
		{"slim suffix", "1.0.0-slim", "-slim"},
		{"debian suffix", "1.0.0-debian", "-debian"},
		{"ubuntu suffix", "1.0.0-ubuntu", "-ubuntu"},
		{"bullseye suffix", "1.0.0-bullseye", "-bullseye"},
		{"no suffix", "2.10.0", ""},
		{"latest tag", "latest", ""},
		{"case insensitive", "2.10.0-ALPINE", "-alpine"},
		{"unknown suffix", "2.10.0-custom", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVersionSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractVersionSuffix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFilterTagsBySuffix(t *testing.T) {
	tests := []struct {
		name           string
		tags           []string
		currentVersion string
		expected       []string
	}{
		{
			name:           "alpine suffix match",
			tags:           []string{"2.10.0", "2.10.0-alpine", "2.10.1", "2.10.1-alpine", "latest"},
			currentVersion: "2.10.0-alpine",
			expected:       []string{"2.10.0-alpine", "2.10.1-alpine"},
		},
		{
			name:           "slim suffix match",
			tags:           []string{"1.0.0", "1.0.0-slim", "1.1.0", "1.1.0-slim"},
			currentVersion: "1.0.0-slim",
			expected:       []string{"1.0.0-slim", "1.1.0-slim"},
		},
		{
			name:           "no suffix in current version",
			tags:           []string{"2.10.0", "2.10.0-alpine", "2.10.1"},
			currentVersion: "2.10.0",
			expected:       []string{"2.10.0", "2.10.0-alpine", "2.10.1"},
		},
		{
			name:           "no matching suffix tags",
			tags:           []string{"2.10.0", "2.10.1", "latest"},
			currentVersion: "2.10.0-alpine",
			expected:       []string{},
		},
		{
			name:           "empty tags",
			tags:           []string{},
			currentVersion: "2.10.0-alpine",
			expected:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterTagsBySuffix(tt.tags, tt.currentVersion)

			if len(result) != len(tt.expected) {
				t.Errorf("FilterTagsBySuffix() returned %d items, want %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			for i, tag := range tt.expected {
				if result[i] != tag {
					t.Errorf("FilterTagsBySuffix()[%d] = %q, want %q", i, result[i], tag)
				}
			}
		})
	}
}

// ─── Regression tests for reported false positives ───────────────────────────

// TestFalsePositive_IssueTagAsVersion covers the case:
// current: dullage/flatnotes:v5.5.4 → should NOT suggest "28-synology-port-issue"
// "28-synology-port-issue" was wrongly parsed as semver 28.0.0 > 5.5.4
func TestFalsePositive_IssueTagAsVersion(t *testing.T) {
	// Simulate the tags available for dullage/flatnotes
	tags := []string{
		"v5.5.0", "v5.5.1", "v5.5.2", "v5.5.3", "v5.5.4",
		"28-synology-port-issue", // should be completely ignored
		"latest",
	}

	best := FindBestUpdateTag("v5.5.4", tags)

	if best != "" {
		t.Errorf("FindBestUpdateTag: expected no update (empty), got %q — issue tag was wrongly treated as newer version", best)
	}
}

// TestFalsePositive_SambaVariantTag covers the case:
// current: ghcr.io/servercontainers/samba:a3.23.3-s4.22.6-r0
// → should NOT suggest "smbd-wsdd2-a3.23.3-s4.22.8-r0"
// Both tags are non-semver custom tags; "smbd-wsdd2-" prefix is a different variant
func TestFalsePositive_SambaVariantTag(t *testing.T) {
	tags := []string{
		"a3.23.3-s4.22.6-r0",
		"a3.23.3-s4.22.7-r0",
		"a3.23.3-s4.22.8-r0",
		"smbd-wsdd2-a3.23.3-s4.22.8-r0", // different variant, should NOT be suggested
		"smbd-wsdd2-a3.23.3-s4.22.6-r0",
		"latest",
	}

	currentTag := "a3.23.3-s4.22.6-r0"
	best := FindBestUpdateTag(currentTag, tags)

	if best == "smbd-wsdd2-a3.23.3-s4.22.8-r0" {
		t.Errorf("FindBestUpdateTag: got %q — variant tag with different prefix should not be suggested", best)
	}
}

// TestFalsePositive_QBittorrentLibtorrentSuffix covers the case:
// current: qbittorrentofficial/qbittorrent-nox:5.1.4-2
// → should NOT suggest "5.1.4-lt2-2" (lt2 = libtorrent2 variant, not a newer version)
func TestFalsePositive_QBittorrentLibtorrentSuffix(t *testing.T) {
	tags := []string{
		"5.1.0-1", "5.1.0-lt2-1",
		"5.1.2-1", "5.1.2-lt2-1",
		"5.1.4-1", "5.1.4-lt2-1",
		"5.1.4-2", "5.1.4-lt2-2", // lt2 is a different build variant
		"latest",
	}

	currentTag := "5.1.4-2"
	best := FindBestUpdateTag(currentTag, tags)

	if best == "5.1.4-lt2-2" {
		t.Errorf("FindBestUpdateTag: got %q — libtorrent variant tag should not be suggested as latest for non-lt2 current tag", best)
	}
	if best != "" {
		t.Errorf("FindBestUpdateTag: got %q — expected no update since 5.1.4-2 is the latest non-lt2 tag", best)
	}
}

// TestFalsePositive_DateBasedTagNotSuggestedForSemver covers the case:
// current: dullage/flatnotes:28-synology-port-issue
// → should NOT suggest kopia/kopia:20260224.0.42919 (different image/format entirely)
// This test validates that date-based tags are not mixed with semver tags
func TestFalsePositive_DateBasedTagNotSuggestedForSemver(t *testing.T) {
	// When current is a semver-ish image and registry returns date-based tags
	tags := []string{
		"v5.5.4",
		"v5.5.3",
		"20260224.0.42919",   // date-based build tag — should be excluded
		"20260101.0.11111",   // date-based build tag — should be excluded
		"28-synology-port-issue", // issue tag — should be excluded
	}

	currentTag := "v5.5.3"
	best := FindBestUpdateTag(currentTag, tags)

	if best == "20260224.0.42919" {
		t.Errorf("FindBestUpdateTag: got %q — date-based tag should not be suggested for a semver image", best)
	}
	if best != "v5.5.4" {
		t.Errorf("FindBestUpdateTag: expected %q, got %q", "v5.5.4", best)
	}
}

// TestIsSemanticVersion_FalsePositiveCases verifies that problematic tags are NOT
// considered semantic versions.
func TestIsSemanticVersion_FalsePositiveCases(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		// These MUST be false — they were causing false positives
		{"28-synology-port-issue", false},          // number + long text = issue/branch tag
		{"smbd-wsdd2-a3.23.3-s4.22.8-r0", false},  // word prefix = variant tag
		{"smbd-wsdd2-a3.23.3-s4.22.6-r0", false},  // word prefix = variant tag
		{"5-branch-name", false},                   // number + word = branch tag

		// These MUST be true — they are valid semver
		{"v5.5.4", true},
		{"5.1.4-2", true},      // build revision suffix (numeric only)
		{"5.1.4", true},
		{"1.2.3", true},
		{"19", true},
		{"18.1", true},
		{"5.1.4-1", true},      // build number suffix

		// These stay false — they were already correctly false
		{"latest", false},
		{"stable", false},
		{"a3.23.3-s4.22.6-r0", false}, // starts with letter 'a', not 'v'
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := IsSemanticVersion(tt.version)
			if result != tt.expected {
				t.Errorf("IsSemanticVersion(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

// TestIsDateBasedTag verifies date-based tag detection
func TestIsDateBasedTag(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"20260224.0.42919", true},
		{"20231015", true},
		{"20231015.1.0", true},
		{"v5.5.4", false},
		{"5.1.4-2", false},
		{"latest", false},
		{"28-synology-port-issue", false}, // 28 alone doesn't match YYYYMMDD
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := IsDateBasedTag(tt.version)
			if result != tt.expected {
				t.Errorf("IsDateBasedTag(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

// TestClassifyTagFamily verifies tag family classification
func TestClassifyTagFamily(t *testing.T) {
	tests := []struct {
		version  string
		expected TagPatternFamily
	}{
		{"v5.5.4", TagFamilySemver},
		{"5.1.4-2", TagFamilySemver},
		{"1.2.3", TagFamilySemver},
		{"19", TagFamilySemver},
		{"18.1", TagFamilySemver},
		{"20260224.0.42919", TagFamilyDateBased},
		{"20231015", TagFamilyDateBased},
		{"28-synology-port-issue", TagFamilyCustom},
		{"smbd-wsdd2-a3.23.3", TagFamilyCustom},
		{"a3.23.3-s4.22.6-r0", TagFamilyCustom},
		{"latest", TagFamilyCustom},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := ClassifyTagFamily(tt.version)
			if result != tt.expected {
				t.Errorf("ClassifyTagFamily(%q) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

// TestFilterTagsByFamily verifies that only same-family tags are returned
func TestFilterTagsByFamily(t *testing.T) {
	tests := []struct {
		name             string
		currentVersion   string
		tags             []string
		expectedCount    int
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:           "semver current - filters out issue and date tags",
			currentVersion: "v5.5.4",
			tags: []string{
				"v5.5.4", "v5.5.5", "v6.0.0",
				"28-synology-port-issue",    // custom — filtered out
				"20260224.0.42919",          // date-based — filtered out
				"latest",                    // custom — filtered out
			},
			expectedCount:    3,
			shouldContain:    []string{"v5.5.4", "v5.5.5", "v6.0.0"},
			shouldNotContain: []string{"28-synology-port-issue", "20260224.0.42919", "latest"},
		},
		{
			name:           "custom current - returns empty",
			currentVersion: "28-synology-port-issue",
			tags:           []string{"v5.5.4", "28-synology-port-issue", "smbd-wsdd2-a3.23.3"},
			expectedCount:  0, // custom tags can't be reliably compared
		},
		{
			name:           "date-based current - only date-based returned",
			currentVersion: "20231015.0.1",
			tags: []string{
				"v5.5.4",           // semver — filtered out
				"20231015.0.1",     // date — kept
				"20260224.0.42919", // date — kept
				"latest",           // custom — filtered out
			},
			expectedCount:    2,
			shouldContain:    []string{"20231015.0.1", "20260224.0.42919"},
			shouldNotContain: []string{"v5.5.4", "latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterTagsByFamily(tt.tags, tt.currentVersion)

			if len(result) != tt.expectedCount {
				t.Errorf("FilterTagsByFamily() returned %d items, want %d. Got: %v", len(result), tt.expectedCount, result)
			}

			resultSet := make(map[string]bool, len(result))
			for _, r := range result {
				resultSet[r] = true
			}

			for _, should := range tt.shouldContain {
				if !resultSet[should] {
					t.Errorf("FilterTagsByFamily() should contain %q but doesn't. Got: %v", should, result)
				}
			}
			for _, shouldNot := range tt.shouldNotContain {
				if resultSet[shouldNot] {
					t.Errorf("FilterTagsByFamily() should NOT contain %q but does. Got: %v", shouldNot, result)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Regression: exact tags seen in the live report on 2026-03-01
// ---------------------------------------------------------------------------

// kopia/kopia:0.22.3 must NOT suggest 20260224.0.42919 (date-based vs semver)
func TestLiveReport_KopiaSemverVsDateBased(t *testing.T) {
	tags := []string{
		"0.18", "0.19", "0.20", "0.21", "0.22", "0.22.3",
		"20260224.0.42919",
		"latest",
	}
	best := FindBestUpdateTag("0.22.3", tags)
	if best == "20260224.0.42919" {
		t.Errorf("got %q — date-based tag must not be suggested for semver current tag", best)
	}
	if best != "" {
		t.Errorf("expected no update, got %q", best)
	}
}

// samba a3.23.3-s4.22.6-r0 must NOT suggest smbd-wsdd2-a3.23.3-s4.22.8-r0 (different prefix variant)
func TestLiveReport_SambaVariantPrefix(t *testing.T) {
	tags := []string{
		"a3.23.3-s4.22.6-r0",
		"a3.23.3-s4.22.7-r0",
		"a3.23.3-s4.22.8-r0",
		"smbd-wsdd2-a3.23.3-s4.22.6-r0",
		"smbd-wsdd2-a3.23.3-s4.22.8-r0",
		"latest",
	}
	best := FindBestUpdateTag("a3.23.3-s4.22.6-r0", tags)
	if best == "smbd-wsdd2-a3.23.3-s4.22.8-r0" {
		t.Errorf("got %q — smbd-wsdd2 variant must not be suggested for non-variant current tag", best)
	}
}

// qbittorrent 5.1.4-2 must NOT suggest 5.1.4-lt2-2 (different named build variant)
func TestLiveReport_QbittorrentLt2Variant(t *testing.T) {
	tags := []string{
		"5.1.0-1", "5.1.0-lt2-1",
		"5.1.2-1", "5.1.2-lt2-1",
		"5.1.4-1", "5.1.4-lt2-1",
		"5.1.4-2", "5.1.4-lt2-2",
		"latest",
	}
	best := FindBestUpdateTag("5.1.4-2", tags)
	if best == "5.1.4-lt2-2" {
		t.Errorf("got %q — lt2 variant must not be suggested for non-lt2 current tag", best)
	}
	if best != "" {
		t.Errorf("expected no update, got %q", best)
	}
}
