package extraimages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/pkg/types"
)

type file struct {
	Images []entry `yaml:"images"`
}

type entry struct {
	Service string `yaml:"service"`
	Image   string `yaml:"image"`
}

// Parse reads a YAML file listing additional images to scan and returns them as DockerImage slice.
// Expected format:
//
//	images:
//	  - service: "my-app"
//	    image: "ghcr.io/user/app:v1.0.0"
func Parse(filePath string) ([]types.DockerImage, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading extra images file %s: %w", filePath, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing extra images file %s: %w", filePath, err)
	}

	parser := compose.NewParser()
	images := make([]types.DockerImage, 0, len(f.Images))
	for _, e := range f.Images {
		e.Image = strings.TrimSpace(e.Image)
		if e.Image == "" {
			continue
		}
		img, err := parser.ParseImageString(e.Image)
		if err != nil {
			return nil, fmt.Errorf("invalid image %q for service %q: %w", e.Image, e.Service, err)
		}
		if e.Service != "" {
			img.ServiceName = e.Service
		} else {
			img.ServiceName = deriveServiceName(e.Image)
		}
		images = append(images, img)
	}

	return images, nil
}

// deriveServiceName extracts a human-readable name from an image reference when no service name is given.
func deriveServiceName(imageRef string) string {
	// Strip registry prefix and tag/digest, keep the repository base name
	s := imageRef
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, filepath.Ext(s))
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
