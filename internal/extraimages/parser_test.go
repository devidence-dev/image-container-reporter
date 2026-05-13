package extraimages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestParse(t *testing.T) {
	t.Run("valid file with service names", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - service: "my-app"
    image: "ghcr.io/user/app:v1.0.0"
  - service: "nginx-proxy"
    image: "nginx:1.25-alpine"
  - service: "postgres"
    image: "postgres:15"
`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(imgs) != 3 {
			t.Fatalf("expected 3 images, got %d", len(imgs))
		}

		assertImage(t, imgs[0], types.DockerImage{
			Registry:    "ghcr.io",
			Repository:  "user/app",
			Tag:         "v1.0.0",
			ServiceName: "my-app",
		})
		assertImage(t, imgs[1], types.DockerImage{
			Registry:    "docker.io",
			Repository:  "library/nginx",
			Tag:         "1.25-alpine",
			ServiceName: "nginx-proxy",
		})
		assertImage(t, imgs[2], types.DockerImage{
			Registry:    "docker.io",
			Repository:  "library/postgres",
			Tag:         "15",
			ServiceName: "postgres",
		})
	})

	t.Run("service name derived from image when omitted", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - image: "ghcr.io/user/myapp:v2.0.0"
  - image: "redis:7"
`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(imgs) != 2 {
			t.Fatalf("expected 2 images, got %d", len(imgs))
		}
		if imgs[0].ServiceName != "myapp" {
			t.Errorf("ServiceName = %q, want %q", imgs[0].ServiceName, "myapp")
		}
		if imgs[1].ServiceName != "redis" {
			t.Errorf("ServiceName = %q, want %q", imgs[1].ServiceName, "redis")
		}
	})

	t.Run("entries with empty image field are skipped", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - service: "valid"
    image: "nginx:latest"
  - service: "no-image"
    image: ""
`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(imgs) != 1 {
			t.Fatalf("expected 1 image, got %d", len(imgs))
		}
		if imgs[0].ServiceName != "valid" {
			t.Errorf("ServiceName = %q, want %q", imgs[0].ServiceName, "valid")
		}
	})

	t.Run("empty images list returns empty slice without error", func(t *testing.T) {
		f := writeTempFile(t, `images: []`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(imgs) != 0 {
			t.Errorf("expected 0 images, got %d", len(imgs))
		}
	})

	t.Run("file does not exist returns error", func(t *testing.T) {
		_, err := Parse("/nonexistent/path/extra-images.yml")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		f := writeTempFile(t, `images: [unclosed bracket`)
		_, err := Parse(f)
		if err == nil {
			t.Fatal("expected error for invalid YAML, got nil")
		}
	})

	t.Run("invalid image string returns error", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - service: "bad"
    image: ""
  - service: "also-bad"
    image: "   "
`)
		// Both entries have blank images and should be skipped, not error.
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// whitespace-only image string: ParseImageString trims it, resulting in empty → skipped
		if len(imgs) != 0 {
			t.Errorf("expected 0 images (all blank), got %d", len(imgs))
		}
	})

	t.Run("docker hub image without registry", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - service: "web"
    image: "nginx:1.25"
`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertImage(t, imgs[0], types.DockerImage{
			Registry:    "docker.io",
			Repository:  "library/nginx",
			Tag:         "1.25",
			ServiceName: "web",
		})
	})

	t.Run("private registry with port", func(t *testing.T) {
		f := writeTempFile(t, `
images:
  - service: "internal"
    image: "registry.example.com:5000/myapp:3.1"
`)
		imgs, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if imgs[0].Registry != "registry.example.com:5000" {
			t.Errorf("Registry = %q, want %q", imgs[0].Registry, "registry.example.com:5000")
		}
		if imgs[0].Tag != "3.1" {
			t.Errorf("Tag = %q, want %q", imgs[0].Tag, "3.1")
		}
	})
}

func TestDeriveServiceName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"nginx:latest", "nginx"},
		{"nginx", "nginx"},
		{"ghcr.io/user/myapp:v1.0.0", "myapp"},
		{"ghcr.io/user/myapp", "myapp"},
		{"registry.example.com/team/service:2.0", "service"},
		{"redis@sha256:abc123", "redis"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := deriveServiceName(tc.input)
			if got != tc.want {
				t.Errorf("deriveServiceName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func assertImage(t *testing.T, got, want types.DockerImage) {
	t.Helper()
	if got.Registry != want.Registry {
		t.Errorf("Registry = %q, want %q", got.Registry, want.Registry)
	}
	if got.Repository != want.Repository {
		t.Errorf("Repository = %q, want %q", got.Repository, want.Repository)
	}
	if got.Tag != want.Tag {
		t.Errorf("Tag = %q, want %q", got.Tag, want.Tag)
	}
	if got.ServiceName != want.ServiceName {
		t.Errorf("ServiceName = %q, want %q", got.ServiceName, want.ServiceName)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "extra-images.yml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return f
}
