package utils

import (
	"regexp"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/user/docker-image-reporter/pkg/types"
)

var (
	// Pre-release patterns to filter out
	preReleasePatterns = []string{
		"alpha", "beta", "rc", "dev", "devel", "development",
		"nightly", "snapshot", "test", "experimental", "canary",
	}

	// Regex to detect if a version looks semantic
	semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
)

// CompareVersions compares two version strings and returns the update type
// Returns types.UpdateTypeNone if newVersion is not newer than currentVersion
func CompareVersions(currentVersion, newVersion string) types.UpdateType {
	// Try semantic version comparison first
	if updateType := compareSemantic(currentVersion, newVersion); updateType != types.UpdateTypeUnknown {
		return updateType
	}

	// Fall back to string comparison
	return compareString(currentVersion, newVersion)
}

// compareSemantic attempts to parse versions as semantic versions and compare them
func compareSemantic(currentVersion, newVersion string) types.UpdateType {
	currentSemver, err1 := semver.NewVersion(NormalizeVersion(currentVersion))
	newSemver, err2 := semver.NewVersion(NormalizeVersion(newVersion))

	// If either version can't be parsed as semantic, return unknown
	if err1 != nil || err2 != nil {
		return types.UpdateTypeUnknown
	}

	// Compare versions
	comparison := newSemver.Compare(currentSemver)
	if comparison <= 0 {
		return types.UpdateTypeNone
	}

	// Determine update type based on version differences
	if newSemver.Major() > currentSemver.Major() {
		return types.UpdateTypeMajor
	}

	if newSemver.Minor() > currentSemver.Minor() {
		return types.UpdateTypeMinor
	}

	if newSemver.Patch() > currentSemver.Patch() {
		return types.UpdateTypePatch
	}

	// Pre-release or metadata changes
	return types.UpdateTypePatch
}

// compareString performs simple string comparison as fallback
func compareString(currentVersion, newVersion string) types.UpdateType {
	if currentVersion == newVersion {
		return types.UpdateTypeNone
	}

	// Simple lexicographic comparison
	if newVersion > currentVersion {
		return types.UpdateTypeUnknown
	}

	return types.UpdateTypeNone
}

// NormalizeVersion removes common prefixes and suffixes to help with parsing
func NormalizeVersion(version string) string {
	// Remove 'v' prefix if present
	normalized := strings.TrimPrefix(version, "v")

	// Remove common suffixes that might interfere with parsing
	suffixes := []string{
		"-alpine", "-slim", "-scratch", "-ubuntu", "-debian", 
		"-bullseye", "-buster", "-focal", "-jammy",
		"-musl", "-glibc",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(normalized, suffix) {
			normalized = strings.TrimSuffix(normalized, suffix)
			break
		}
	}

	return normalized
}

// IsPreRelease checks if a version string contains pre-release indicators
func IsPreRelease(version string) bool {
	lowerVersion := strings.ToLower(version)

	// Special cases that should not be considered pre-release
	if lowerVersion == "latest" || lowerVersion == "stable" {
		return false
	}

	for _, pattern := range preReleasePatterns {
		// Use word boundaries or specific patterns to avoid false positives
		if strings.Contains(lowerVersion, pattern) {
			// Additional check to avoid false positives like "latest" containing "test"
			if pattern == "test" && lowerVersion == "latest" {
				continue
			}
			return true
		}
	}

	return false
}

// IsSemanticVersion checks if a version string looks like semantic versioning
func IsSemanticVersion(version string) bool {
	return semverRegex.MatchString(NormalizeVersion(version))
}

// FilterPreReleases filters out pre-release versions from a slice of tags
func FilterPreReleases(tags []string) []string {
	var filtered []string
	for _, tag := range tags {
		if !IsPreRelease(tag) {
			filtered = append(filtered, tag)
		}
	}
	return filtered
}

// SortVersions sorts a slice of version strings in descending order (newest first)
func SortVersions(versions []string) []string {
	if len(versions) <= 1 {
		return versions
	}

	// Separate semantic and non-semantic versions
	var semantic, nonSemantic []string

	for _, version := range versions {
		if IsSemanticVersion(version) {
			semantic = append(semantic, version)
		} else {
			nonSemantic = append(nonSemantic, version)
		}
	}

	// Sort semantic versions using semver
	sortedSemantic := sortSemanticVersions(semantic)

	// Sort non-semantic versions lexicographically (reversed)
	sortedNonSemantic := sortStringVersions(nonSemantic)

	// Combine results (semantic first, then non-semantic)
	result := make([]string, 0, len(versions))
	result = append(result, sortedSemantic...)
	result = append(result, sortedNonSemantic...)

	return result
}

// sortSemanticVersions sorts semantic versions in descending order
func sortSemanticVersions(versions []string) []string {
	if len(versions) <= 1 {
		return versions
	}

	// Convert to semver objects for sorting
	type versionPair struct {
		original string
		semver   *semver.Version
	}

	var pairs []versionPair
	for _, version := range versions {
		if sv, err := semver.NewVersion(NormalizeVersion(version)); err == nil {
			pairs = append(pairs, versionPair{original: version, semver: sv})
		}
	}

	// Sort in descending order (newest first)
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].semver.LessThan(pairs[j].semver) {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Extract original version strings
	result := make([]string, len(pairs))
	for i, pair := range pairs {
		result[i] = pair.original
	}

	return result
}

// sortStringVersions sorts non-semantic versions lexicographically in descending order
func sortStringVersions(versions []string) []string {
	if len(versions) <= 1 {
		return versions
	}

	// Simple bubble sort in descending order
	sorted := make([]string, len(versions))
	copy(sorted, versions)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] < sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// GetLatestVersion returns the latest version from a slice of version strings
func GetLatestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	sorted := SortVersions(versions)
	return sorted[0]
}

// GetLatestVersionExcludingPreReleases returns the latest stable version
func GetLatestVersionExcludingPreReleases(versions []string) string {
	filtered := FilterPreReleases(versions)
	return GetLatestVersion(filtered)
}

// UpdateFilter represents filtering preferences for updates
type UpdateFilter struct {
	// IncludePreReleases determines if pre-release versions should be included
	IncludePreReleases bool
	// MinUpdateType specifies the minimum update type to include
	MinUpdateType types.UpdateType
	// ExcludePatterns contains patterns to exclude from updates
	ExcludePatterns []string
}

// DefaultUpdateFilter returns a sensible default filter configuration
func DefaultUpdateFilter() UpdateFilter {
	return UpdateFilter{
		IncludePreReleases: false,
		MinUpdateType:      types.UpdateTypePatch,
		ExcludePatterns:    []string{"nightly", "snapshot", "temp", "tmp"},
	}
}

// FilterUpdates filters a list of available versions based on the current version and filter preferences
func FilterUpdates(currentVersion string, availableVersions []string, filter UpdateFilter) []string {
	var filtered []string

	for _, version := range availableVersions {
		if ShouldIncludeUpdate(currentVersion, version, filter) {
			filtered = append(filtered, version)
		}
	}

	return filtered
}

// ShouldIncludeUpdate determines if a version should be included based on filter criteria
func ShouldIncludeUpdate(currentVersion, candidateVersion string, filter UpdateFilter) bool {
	// Skip if it's a pre-release and we don't want them
	if !filter.IncludePreReleases && IsPreRelease(candidateVersion) {
		return false
	}

	// Skip if it matches any exclude pattern
	if matchesExcludePatterns(candidateVersion, filter.ExcludePatterns) {
		return false
	}

	// Check if the update type meets the minimum requirement
	updateType := CompareVersions(currentVersion, candidateVersion)

	// Skip if no update or update type is below minimum
	if updateType == types.UpdateTypeNone {
		return false
	}

	return isUpdateTypeAcceptable(updateType, filter.MinUpdateType)
}

// matchesExcludePatterns checks if a version matches any of the exclude patterns
func matchesExcludePatterns(version string, patterns []string) bool {
	lowerVersion := strings.ToLower(version)

	for _, pattern := range patterns {
		if strings.Contains(lowerVersion, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// isUpdateTypeAcceptable checks if an update type meets the minimum requirement
func isUpdateTypeAcceptable(updateType, minUpdateType types.UpdateType) bool {
	// Define update type hierarchy (higher values = more significant updates)
	hierarchy := map[types.UpdateType]int{
		types.UpdateTypeNone:    0,
		types.UpdateTypePatch:   1,
		types.UpdateTypeMinor:   2,
		types.UpdateTypeMajor:   3,
		types.UpdateTypeUnknown: 1, // Treat unknown as patch level
	}

	updateLevel, exists1 := hierarchy[updateType]
	minLevel, exists2 := hierarchy[minUpdateType]

	// If either type is not in hierarchy, be conservative and allow it
	if !exists1 || !exists2 {
		return true
	}

	return updateLevel >= minLevel
}

// GetSignificantUpdates returns only updates that are considered significant
// (major or minor updates by default)
func GetSignificantUpdates(currentVersion string, availableVersions []string) []string {
	filter := UpdateFilter{
		IncludePreReleases: false,
		MinUpdateType:      types.UpdateTypeMinor,
		ExcludePatterns:    DefaultUpdateFilter().ExcludePatterns,
	}

	return FilterUpdates(currentVersion, availableVersions, filter)
}

// GetAllStableUpdates returns all stable updates (excluding pre-releases)
func GetAllStableUpdates(currentVersion string, availableVersions []string) []string {
	filter := UpdateFilter{
		IncludePreReleases: false,
		MinUpdateType:      types.UpdateTypePatch,
		ExcludePatterns:    DefaultUpdateFilter().ExcludePatterns,
	}

	return FilterUpdates(currentVersion, availableVersions, filter)
}

// ClassifyUpdate provides detailed information about an update
type UpdateClassification struct {
	UpdateType    types.UpdateType
	IsPreRelease  bool
	IsSignificant bool
	Description   string
}

// ClassifyVersionUpdate analyzes the relationship between current and new version
func ClassifyVersionUpdate(currentVersion, newVersion string) UpdateClassification {
	updateType := CompareVersions(currentVersion, newVersion)
	isPreRelease := IsPreRelease(newVersion)
	isSignificant := updateType == types.UpdateTypeMajor || updateType == types.UpdateTypeMinor

	var description string
	switch updateType {
	case types.UpdateTypeNone:
		description = "No update available"
	case types.UpdateTypePatch:
		description = "Patch update available"
	case types.UpdateTypeMinor:
		description = "Minor update available"
	case types.UpdateTypeMajor:
		description = "Major update available"
	case types.UpdateTypeUnknown:
		description = "Update available (version format unknown)"
	}

	if isPreRelease {
		description += " (pre-release)"
	}

	return UpdateClassification{
		UpdateType:    updateType,
		IsPreRelease:  isPreRelease,
		IsSignificant: isSignificant,
		Description:   description,
	}
}
