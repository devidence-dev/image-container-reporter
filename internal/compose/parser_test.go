package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestParser_CanParse(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		filename string
		expected bool
	}{
		{"docker-compose.yml", true},
		{"docker-compose.yaml", true},
		{"compose.yml", true},
		{"compose.yaml", true},
		{"docker-compose.prod.yml", true},
		{"docker-compose.dev.yaml", true},
		{"docker-compose.test.yml", true},
		{"Dockerfile", false},
		{"config.yaml", false},
		{"docker-compose.txt", false},
		{"compose.json", false},
		{"my-compose.yml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := parser.CanParse(tt.filename)
			if result != tt.expected {
				t.Errorf("CanParse(%s) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestParser_parseImageString(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name          string
		imageStr      string
		expectedImage types.DockerImage
		expectError   bool
	}{
		{
			name:     "simple image",
			imageStr: "nginx",
			expectedImage: types.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
			},
		},
		{
			name:     "image with tag",
			imageStr: "nginx:1.20",
			expectedImage: types.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.20",
			},
		},
		{
			name:     "user image",
			imageStr: "user/nginx:latest",
			expectedImage: types.DockerImage{
				Registry:   "docker.io",
				Repository: "user/nginx",
				Tag:        "latest",
			},
		},
		{
			name:     "ghcr image",
			imageStr: "ghcr.io/owner/repo:v1.0.0",
			expectedImage: types.DockerImage{
				Registry:   "ghcr.io",
				Repository: "owner/repo",
				Tag:        "v1.0.0",
			},
		},
		{
			name:     "private registry",
			imageStr: "registry.example.com/myapp:1.2.3",
			expectedImage: types.DockerImage{
				Registry:   "registry.example.com",
				Repository: "myapp",
				Tag:        "1.2.3",
			},
		},
		{
			name:     "registry with port",
			imageStr: "localhost:5000/myapp",
			expectedImage: types.DockerImage{
				Registry:   "localhost:5000",
				Repository: "myapp",
				Tag:        "latest",
			},
		},
		{
			name:     "image with digest",
			imageStr: "nginx@sha256:abc123",
			expectedImage: types.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Digest:     "sha256:abc123",
			},
		},
		{
			name:     "image with tag and digest",
			imageStr: "nginx:1.20@sha256:abc123",
			expectedImage: types.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.20",
				Digest:     "sha256:abc123",
			},
		},
		{
			name:        "empty image",
			imageStr:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseImageString(tt.imageStr)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Registry != tt.expectedImage.Registry {
				t.Errorf("Registry = %s, want %s", result.Registry, tt.expectedImage.Registry)
			}
			if result.Repository != tt.expectedImage.Repository {
				t.Errorf("Repository = %s, want %s", result.Repository, tt.expectedImage.Repository)
			}
			if result.Tag != tt.expectedImage.Tag {
				t.Errorf("Tag = %s, want %s", result.Tag, tt.expectedImage.Tag)
			}
			if result.Digest != tt.expectedImage.Digest {
				t.Errorf("Digest = %s, want %s", result.Digest, tt.expectedImage.Digest)
			}
		})
	}
}

func TestParser_ParseFile(t *testing.T) {
	parser := NewParser()

	// Crear archivo temporal de prueba
	tempDir := t.TempDir()
	composeFile := filepath.Join(tempDir, "docker-compose.yml")

	composeContent := `version: '3.8'
services:
  web:
    image: nginx:1.20
    ports:
      - "80:80"
  
  db:
    image: postgres:13
    environment:
      POSTGRES_PASSWORD: secret
  
  redis:
    image: redis:alpine
  
  app:
    image: ghcr.io/user/myapp:v1.0.0
    depends_on:
      - db
      - redis
  
  builder:
    build: .
    # No image, should be skipped
`

	err := os.WriteFile(composeFile, []byte(composeContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parsear el archivo
	images, err := parser.ParseFile(context.Background(), composeFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verificar que se encontraron las imágenes correctas
	expectedImages := []types.DockerImage{
		{
			Registry:    "docker.io",
			Repository:  "library/nginx",
			Tag:         "1.20",
			ServiceName: "web",
			ComposeFile: composeFile,
		},
		{
			Registry:    "docker.io",
			Repository:  "library/postgres",
			Tag:         "13",
			ServiceName: "db",
			ComposeFile: composeFile,
		},
		{
			Registry:    "docker.io",
			Repository:  "library/redis",
			Tag:         "alpine",
			ServiceName: "redis",
			ComposeFile: composeFile,
		},
		{
			Registry:    "ghcr.io",
			Repository:  "user/myapp",
			Tag:         "v1.0.0",
			ServiceName: "app",
			ComposeFile: composeFile,
		},
	}

	if len(images) != len(expectedImages) {
		t.Fatalf("Expected %d images, got %d", len(expectedImages), len(images))
	}

	// Verificar cada imagen
	for i, expected := range expectedImages {
		found := false
		for _, actual := range images {
			if actual.ServiceName == expected.ServiceName {
				found = true
				if actual.Registry != expected.Registry {
					t.Errorf("Image %d Registry = %s, want %s", i, actual.Registry, expected.Registry)
				}
				if actual.Repository != expected.Repository {
					t.Errorf("Image %d Repository = %s, want %s", i, actual.Repository, expected.Repository)
				}
				if actual.Tag != expected.Tag {
					t.Errorf("Image %d Tag = %s, want %s", i, actual.Tag, expected.Tag)
				}
				if actual.ServiceName != expected.ServiceName {
					t.Errorf("Image %d ServiceName = %s, want %s", i, actual.ServiceName, expected.ServiceName)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected service %s not found in results", expected.ServiceName)
		}
	}
}

func TestParser_ParseFile_InvalidYAML(t *testing.T) {
	parser := NewParser()

	// Crear archivo con YAML inválido
	tempDir := t.TempDir()
	composeFile := filepath.Join(tempDir, "invalid-compose.yml")

	invalidContent := `version: '3.8'
services:
  web:
    image: nginx
    ports:
      - "80:80"
    invalid_yaml: [unclosed bracket
`

	err := os.WriteFile(composeFile, []byte(invalidContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Intentar parsear el archivo inválido
	_, err = parser.ParseFile(context.Background(), composeFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, but got none")
	}
}

func TestParser_ParseFile_NonExistentFile(t *testing.T) {
	parser := NewParser()

	// Intentar parsear un archivo que no existe
	_, err := parser.ParseFile(context.Background(), "/nonexistent/file.yml")
	if err == nil {
		t.Error("Expected error for non-existent file, but got none")
	}
}
