package extraimages

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/pkg/types"
)

type file struct {
	Dockerfiles []string `yaml:"dockerfiles"`
}

// argVarRegex matches ${VAR} and $VAR substitution patterns.
var argVarRegex = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// Parse reads a YAML file listing Dockerfiles, extracts their FROM base images
// (resolving ARG defaults and skipping local stage references), and returns
// them as a DockerImage slice ready for update checking.
//
// Expected format:
//
//	dockerfiles:
//	  - /path/to/Dockerfile
//	  - /other/path/Dockerfile
func Parse(filePath string) ([]types.DockerImage, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading extra images file %s: %w", filePath, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing extra images file %s: %w", filePath, err)
	}

	var images []types.DockerImage
	for _, p := range f.Dockerfiles {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		imgs, err := parseDockerfile(p)
		if err != nil {
			return nil, err
		}
		images = append(images, imgs...)
	}

	return images, nil
}

// parseDockerfile extracts base images from FROM instructions in a Dockerfile.
// It resolves ARG defaults defined before the first FROM, handles multi-stage
// builds, and skips scratch and local stage references.
func parseDockerfile(filePath string) ([]types.DockerImage, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading Dockerfile %s: %w", filePath, err)
	}

	lines := joinContinuationLines(strings.Split(string(data), "\n"))

	globalArgs := make(map[string]string) // ARG defaults before the first FROM
	stageAliases := make(map[string]bool) // AS aliases from previous stages
	firstFromSeen := false

	parser := compose.NewParser()
	var images []types.DockerImage

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)

		// Collect ARG defaults before the first FROM; they are the only ones
		// that can be referenced in FROM image references.
		if !firstFromSeen && strings.HasPrefix(upper, "ARG ") {
			if name, value, ok := parseARG(line[4:]); ok {
				globalArgs[name] = value
			}
			continue
		}

		if !strings.HasPrefix(upper, "FROM ") {
			continue
		}

		firstFromSeen = true
		imageRef, alias := parseFROM(line[5:])
		imageRef = resolveArgs(imageRef, globalArgs)

		if alias != "" {
			stageAliases[strings.ToLower(alias)] = true
		}

		// "scratch" is a virtual empty base with no registry.
		if strings.ToLower(imageRef) == "scratch" {
			continue
		}

		// Reference to a previous stage alias, not a registry image.
		if stageAliases[strings.ToLower(imageRef)] {
			continue
		}

		img, err := parser.ParseImageString(imageRef)
		if err != nil {
			continue // unresolvable placeholder — skip silently
		}

		if alias != "" {
			img.ServiceName = alias
		} else {
			img.ServiceName = deriveServiceName(imageRef)
		}
		img.ComposeFile = filePath

		images = append(images, img)
	}

	return images, nil
}

// joinContinuationLines merges lines ending with \ with the following line,
// matching standard Dockerfile line-continuation behaviour.
func joinContinuationLines(lines []string) []string {
	var result []string
	var buf strings.Builder

	for _, line := range lines {
		if trimmed, ok := strings.CutSuffix(line, "\\"); ok {
			buf.WriteString(trimmed)
			buf.WriteByte(' ')
		} else {
			buf.WriteString(line)
			result = append(result, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		result = append(result, buf.String())
	}

	return result
}

// parseARG parses the body of an ARG instruction (the part after "ARG ").
// Returns name, default value, and true when a default is present.
func parseARG(body string) (name, value string, hasDefault bool) {
	body = strings.TrimSpace(body)
	rawName, rest, ok := strings.Cut(body, "=")
	if !ok {
		return body, "", false
	}
	name = strings.TrimSpace(rawName)
	value = strings.Trim(strings.TrimSpace(rest), `"'`)
	return name, value, true
}

// parseFROM parses the body of a FROM instruction (the part after "FROM ").
// Handles --flag arguments (e.g. --platform) and the optional AS alias.
// Returns the image reference and alias (empty string when absent).
func parseFROM(body string) (imageRef, alias string) {
	fields := strings.Fields(strings.TrimSpace(body))

	var remaining []string
	for _, f := range fields {
		if !strings.HasPrefix(f, "--") {
			remaining = append(remaining, f)
		}
	}

	if len(remaining) == 0 {
		return "", ""
	}

	imageRef = remaining[0]
	if len(remaining) >= 3 && strings.ToUpper(remaining[1]) == "AS" {
		alias = remaining[2]
	}

	return imageRef, alias
}

// resolveArgs substitutes ${VAR} and $VAR occurrences using the provided map.
// Unresolved references are left as-is.
func resolveArgs(s string, args map[string]string) string {
	return argVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		var name string
		if strings.HasPrefix(match, "${") {
			name = match[2 : len(match)-1]
		} else {
			name = match[1:]
		}
		if val, ok := args[name]; ok {
			return val
		}
		return match
	})
}

// deriveServiceName extracts a human-readable name from an image reference
// when no AS alias is available.
func deriveServiceName(imageRef string) string {
	s := imageRef
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "@"); idx >= 0 {
		s = s[:idx]
	}
	if s == "" {
		return imageRef
	}
	return s
}
