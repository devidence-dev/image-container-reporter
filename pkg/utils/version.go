package utils

import (
	"fmt"
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
		"pre", "preview", "unstable",
	}

	// Regex to detect if a version looks semantic
	semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

	// Regex helpers to allow padding numeric Docker tags like "18.1" or "19"
	twoPartSemverRegex = regexp.MustCompile(`^v?\d+\.\d+$`)
	onePartSemverRegex = regexp.MustCompile(`^v?\d+$`)
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
	currentSemver, err1 := parseFlexibleSemver(currentVersion)
	newSemver, err2 := parseFlexibleSemver(newVersion)

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

	// Remove common suffixes (including numeric variants like -alpine3.18)
	if suffix := ExtractVersionSuffix(normalized); suffix != "" {
		// Remove the suffix plus any trailing digits/dots/hyphens
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(suffix) + `[0-9\.\-]*$`)
		normalized = re.ReplaceAllString(normalized, "")
	}

	return normalized
}

// parseFlexibleSemver parses Docker tags that may omit patch or minor parts by padding them.
// Examples: "18.1" -> "18.1.0", "19" -> "19.0.0".
func parseFlexibleSemver(version string) (*semver.Version, error) {
	normalized := NormalizeVersion(version)

	if sv, err := semver.NewVersion(normalized); err == nil {
		return sv, nil
	}

	if twoPartSemverRegex.MatchString(normalized) {
		return semver.NewVersion(normalized + ".0")
	}

	if onePartSemverRegex.MatchString(normalized) {
		return semver.NewVersion(normalized + ".0.0")
	}

	return nil, fmt.Errorf("version is not semantic: %s", version)
}

// IsPreRelease checks if a version string contains pre-release indicators
func IsPreRelease(version string) bool {
	lowerVersion := strings.ToLower(version)

	// Special cases that should not be considered pre-release
	if lowerVersion == "latest" || lowerVersion == "stable" {
		return false
	}

	// Check for semantic versioning pre-release patterns (e.g., -alpha, -beta.1, -rc.2)
	if strings.Contains(lowerVersion, "-alpha") ||
		strings.Contains(lowerVersion, "-beta") ||
		strings.Contains(lowerVersion, "-rc") ||
		strings.Contains(lowerVersion, "-dev") ||
		strings.Contains(lowerVersion, "-devel") ||
		strings.Contains(lowerVersion, "-development") ||
		strings.Contains(lowerVersion, "-pre") ||
		strings.Contains(lowerVersion, "-preview") ||
		strings.Contains(lowerVersion, "-unstable") {
		return true
	}

	// Check for single letter pre-release indicators (e.g., -a.1, -b.2)
	if regexp.MustCompile(`-[a-zA-Z]\.`).MatchString(lowerVersion) {
		return true
	}

	// Check for other pre-release patterns
	for _, pattern := range preReleasePatterns {
		// Use word boundaries or specific patterns to avoid false positives
		if strings.Contains(lowerVersion, pattern) {
			// Additional check to avoid false positives like "latest" containing "test"
			if pattern == "test" && lowerVersion == "latest" {
				continue
			}
			// For single character patterns, be more strict
			if len(pattern) == 1 {
				if regexp.MustCompile(`-[a-zA-Z]\d*`).MatchString(lowerVersion) {
					return true
				}
			} else {
				return true
			}
		}
	}

	return false
}

// IsSemanticVersion checks if a version string looks like semantic versioning.
// Accepts full semver (major.minor.patch) and also Docker-style two-part or single-number tags
// (e.g., "18.1", "19") so they are treated as numeric and not as names like "trixie".
func IsSemanticVersion(version string) bool {
	n := NormalizeVersion(version)
	return semverRegex.MatchString(n) || twoPartSemverRegex.MatchString(n) || onePartSemverRegex.MatchString(n)
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

	// Convert to semver objects for sorting (using flexible parsing)
	type versionPair struct {
		original string
		semver   *semver.Version
	}

	var pairs []versionPair
	for _, version := range versions {
		if sv, err := parseFlexibleSemver(version); err == nil {
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

// ExtractVersionSuffix extracts the suffix from a version tag (e.g., "-alpine" from "2.10.0-alpine")
func ExtractVersionSuffix(version string) string {
	// Common Docker image suffixes
	suffixes := []string{
		"-alpine", "-slim", "-scratch", "-ubuntu", "-debian",
		"-bullseye", "-buster", "-focal", "-jammy",
		"-musl", "-glibc", "-bookworm", "-noble",
	}

	lowerVersion := strings.ToLower(version)
	// Prefer the longest matching base suffix (avoid accidental short matches)
	var bestMatch string
	for _, suffix := range suffixes {
		// Match suffix optionally followed by digits/dots/hyphens, e.g. -alpine3.18
		pattern := `(?i)` + regexp.QuoteMeta(suffix) + `[0-9\.\-]*$`
		if matched, _ := regexp.MatchString(pattern, lowerVersion); matched {
			if len(suffix) > len(bestMatch) {
				bestMatch = suffix
			}
		}
	}

	return bestMatch
}

// FilterTagsBySuffix filters tags to only include those with the same suffix as the current version
func FilterTagsBySuffix(tags []string, currentVersion string) []string {
	suffix := ExtractVersionSuffix(currentVersion)
	if suffix == "" {
		// No suffix in current version, return all tags
		return tags
	}

	var filtered []string
	// Build a pattern that matches the base suffix plus optional numeric variant suffix
	pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(suffix) + `[0-9\.\-]*$`)
	for _, tag := range tags {
		if pattern.MatchString(strings.ToLower(tag)) {
			filtered = append(filtered, tag)
		}
	}

	// If no tags match the suffix, return empty slice to indicate no compatible updates
	if len(filtered) == 0 {
		return []string{}
	}

	return filtered
}

// FindBestUpdateTag returns the best candidate tag to use as the latest update for the given currentVersion.
// It finds the highest semantic version greater than the current one (after normalization). If multiple
// original tags map to that semantic version (e.g., with and without suffix variants), it prefers a tag
// that matches the current suffix. If none match, it returns a generic tag from that version group.
func FindBestUpdateTag(currentVersion string, tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	// Build mapping from normalized semver string to original tags
	type group struct {
		sem  *semver.Version
		tags []string
	}

	groups := make(map[string]*group)

	for _, t := range tags {
		sv, err := parseFlexibleSemver(t)
		if err != nil {
			// skip non-semver tags
			continue
		}
		key := sv.String()
		g, ok := groups[key]
		if !ok {
			groups[key] = &group{sem: sv, tags: []string{t}}
		} else {
			g.tags = append(g.tags, t)
		}
	}

	// Parse current version
	currSv, err := parseFlexibleSemver(currentVersion)
	if err != nil {
		// If current is not semver, fallback to simple sort
		sorted := SortVersions(tags)
		if len(sorted) > 0 {
			return sorted[0]
		}
		return ""
	}

	// Find highest semver greater than current
	var best *group
	for _, g := range groups {
		if g.sem.Compare(currSv) <= 0 {
			continue
		}
		if best == nil || best.sem.LessThan(g.sem) {
			best = g
		}
	}

	if best == nil {
		return ""
	}

	// Prefer tag matching current suffix
	suffix := ExtractVersionSuffix(currentVersion)
	if suffix != "" {
		pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(suffix) + `[0-9\.\-]*$`)
		for _, t := range best.tags {
			if pattern.MatchString(strings.ToLower(t)) {
				return t
			}
		}
	}

	// If none match suffix, try to find a tag without suffix (generic)
	for _, t := range best.tags {
		if ExtractVersionSuffix(t) == "" {
			return t
		}
	}

	// Otherwise return the first available tag in the group
	return best.tags[0]
}
