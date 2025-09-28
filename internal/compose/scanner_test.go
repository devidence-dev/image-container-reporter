package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestScanner_FindComposeFiles(t *testing.T) {
	scanner := NewScanner()

	// Crear estructura de directorios de prueba
	tempDir := t.TempDir()

	// Crear archivos de prueba
	files := map[string]string{
		"docker-compose.yml":      "version: '3'\nservices:\n  web:\n    image: nginx",
		"docker-compose.prod.yml": "version: '3'\nservices:\n  web:\n    image: nginx:prod",
		"compose.yml":             "version: '3'\nservices:\n  app:\n    image: app:latest",
		"Dockerfile":              "FROM nginx",
		"config.yaml":             "key: value",
		"subdir/docker-compose.yml": "version: '3'\nservices:\n  db:\n    image: postgres",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	tests := []struct {
		name      string
		config    types.ScanConfig
		expected  []string
	}{
		{
			name: "recursive scan",
			config: types.ScanConfig{
				Recursive: true,
				Patterns:  []string{"docker-compose.yml", "docker-compose.*.yml", "compose.yml"},
			},
			expected: []string{
				"docker-compose.yml",
				"docker-compose.prod.yml", 
				"compose.yml",
				"subdir/docker-compose.yml",
			},
		},
		{
			name: "non-recursive scan",
			config: types.ScanConfig{
				Recursive: false,
				Patterns:  []string{"docker-compose.yml", "docker-compose.*.yml", "compose.yml"},
			},
			expected: []string{
				"docker-compose.yml",
				"docker-compose.prod.yml",
				"compose.yml",
			},
		},
		{
			name: "specific pattern",
			config: types.ScanConfig{
				Recursive: true,
				Patterns:  []string{"docker-compose.prod.yml"},
			},
			expected: []string{
				"docker-compose.prod.yml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := scanner.FindComposeFiles(context.Background(), tempDir, tt.config)
			if err != nil {
				t.Fatalf("FindComposeFiles failed: %v", err)
			}

			// Convertir paths absolutos a relativos para comparación
			var relativeFiles []string
			for _, file := range files {
				rel, err := filepath.Rel(tempDir, file)
				if err != nil {
					t.Fatalf("Failed to get relative path: %v", err)
				}
				relativeFiles = append(relativeFiles, rel)
			}

			if len(relativeFiles) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d: %v", len(tt.expected), len(relativeFiles), relativeFiles)
				return
			}

			// Verificar que todos los archivos esperados están presentes
			for _, expected := range tt.expected {
				found := false
				for _, actual := range relativeFiles {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %s not found in results: %v", expected, relativeFiles)
				}
			}
		})
	}
}

func TestScanner_ScanDirectory(t *testing.T) {
	scanner := NewScanner()

	// Crear estructura de directorios de prueba
	tempDir := t.TempDir()

	// Crear archivos de prueba
	composeContent := `version: '3.8'
services:
  web:
    image: nginx:1.20
  db:
    image: postgres:13
  redis:
    image: redis:alpine
`

	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to create compose file: %v", err)
	}

	config := types.ScanConfig{
		Recursive: false,
		Patterns:  []string{"docker-compose.yml"},
	}

	images, scannedFiles, err := scanner.ScanDirectory(context.Background(), tempDir, config)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Verificar archivos escaneados
	if len(scannedFiles) != 1 {
		t.Errorf("Expected 1 scanned file, got %d", len(scannedFiles))
	}

	// Verificar imágenes encontradas
	expectedImages := 3 // web, db, redis
	if len(images) != expectedImages {
		t.Errorf("Expected %d images, got %d", expectedImages, len(images))
	}

	// Verificar que las imágenes tienen la información correcta
	serviceNames := make(map[string]bool)
	for _, image := range images {
		serviceNames[image.ServiceName] = true
		if image.ComposeFile != composeFile {
			t.Errorf("Expected ComposeFile %s, got %s", composeFile, image.ComposeFile)
		}
	}

	expectedServices := []string{"web", "db", "redis"}
	for _, service := range expectedServices {
		if !serviceNames[service] {
			t.Errorf("Expected service %s not found", service)
		}
	}
}

func TestScanner_matchesPatterns(t *testing.T) {
	scanner := NewScanner()

	tests := []struct {
		name     string
		filePath string
		patterns []string
		expected bool
	}{
		{
			name:     "exact match",
			filePath: "/path/to/docker-compose.yml",
			patterns: []string{"docker-compose.yml"},
			expected: true,
		},
		{
			name:     "glob pattern match",
			filePath: "/path/to/docker-compose.prod.yml",
			patterns: []string{"docker-compose.*.yml"},
			expected: true,
		},
		{
			name:     "multiple patterns - first matches",
			filePath: "/path/to/compose.yml",
			patterns: []string{"compose.yml", "docker-compose.yml"},
			expected: true,
		},
		{
			name:     "multiple patterns - second matches",
			filePath: "/path/to/docker-compose.yml",
			patterns: []string{"compose.yml", "docker-compose.yml"},
			expected: true,
		},
		{
			name:     "no match",
			filePath: "/path/to/Dockerfile",
			patterns: []string{"docker-compose.yml", "compose.yml"},
			expected: false,
		},
		{
			name:     "empty patterns - should match all",
			filePath: "/path/to/any-file.txt",
			patterns: []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.matchesPatterns(tt.filePath, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesPatterns(%s, %v) = %v, want %v", 
					tt.filePath, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestScanner_shouldSkipDirectory(t *testing.T) {
	scanner := NewScanner()

	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{"git directory", ".git", true},
		{"node_modules", "node_modules", true},
		{"vscode", ".vscode", true},
		{"build directory", "build", true},
		{"normal directory", "src", false},
		{"app directory", "app", false},
		{"hidden but allowed", ".config", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.shouldSkipDirectory(tt.dirName)
			if result != tt.expected {
				t.Errorf("shouldSkipDirectory(%s) = %v, want %v", 
					tt.dirName, result, tt.expected)
			}
		})
	}
}

func TestScanner_GetImagesByService(t *testing.T) {
	scanner := NewScanner()

	images := []types.DockerImage{
		{ServiceName: "web", Repository: "nginx", Tag: "latest"},
		{ServiceName: "db", Repository: "postgres", Tag: "13"},
		{ServiceName: "web", Repository: "nginx", Tag: "alpine"},
		{ServiceName: "", Repository: "redis", Tag: "latest"}, // Sin nombre de servicio
	}

	result := scanner.GetImagesByService(images)

	// Verificar que web tiene 2 imágenes
	if len(result["web"]) != 2 {
		t.Errorf("Expected 2 images for web service, got %d", len(result["web"]))
	}

	// Verificar que db tiene 1 imagen
	if len(result["db"]) != 1 {
		t.Errorf("Expected 1 image for db service, got %d", len(result["db"]))
	}

	// Verificar que unknown tiene 1 imagen (la sin nombre)
	if len(result["unknown"]) != 1 {
		t.Errorf("Expected 1 image for unknown service, got %d", len(result["unknown"]))
	}
}

func TestScanner_GetImagesByRegistry(t *testing.T) {
	scanner := NewScanner()

	images := []types.DockerImage{
		{Registry: "docker.io", Repository: "nginx", Tag: "latest"},
		{Registry: "ghcr.io", Repository: "user/app", Tag: "v1.0.0"},
		{Registry: "docker.io", Repository: "postgres", Tag: "13"},
		{Registry: "", Repository: "redis", Tag: "latest"}, // Sin registry
	}

	result := scanner.GetImagesByRegistry(images)

	// Verificar que docker.io tiene 2 imágenes
	if len(result["docker.io"]) != 2 {
		t.Errorf("Expected 2 images for docker.io registry, got %d", len(result["docker.io"]))
	}

	// Verificar que ghcr.io tiene 1 imagen
	if len(result["ghcr.io"]) != 1 {
		t.Errorf("Expected 1 image for ghcr.io registry, got %d", len(result["ghcr.io"]))
	}

	// Verificar que unknown tiene 1 imagen (la sin registry)
	if len(result["unknown"]) != 1 {
		t.Errorf("Expected 1 image for unknown registry, got %d", len(result["unknown"]))
	}
}