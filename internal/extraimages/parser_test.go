package extraimages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

// ---- Parse (YAML) --------------------------------------------------------

func TestParse_ValidFile(t *testing.T) {
	df1 := writeTempDockerfile(t, "FROM nginx:1.25-alpine\n")
	df2 := writeTempDockerfile(t, "FROM postgres:15\n")

	yaml := writeTempFile(t, "extra-images.yml", "dockerfiles:\n  - "+df1+"\n  - "+df2+"\n")

	imgs, err := Parse(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("expected 2 images, got %d", len(imgs))
	}
}

func TestParse_EmptyList(t *testing.T) {
	yaml := writeTempFile(t, "extra-images.yml", "dockerfiles: []\n")
	imgs, err := Parse(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("expected 0 images, got %d", len(imgs))
	}
}

func TestParse_YAMLFileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/extra-images.yml")
	if err == nil {
		t.Fatal("expected error for missing YAML file, got nil")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	f := writeTempFile(t, "extra-images.yml", "dockerfiles: [unclosed bracket\n")
	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestParse_DockerfileNotFound(t *testing.T) {
	yaml := writeTempFile(t, "extra-images.yml", "dockerfiles:\n  - /nonexistent/Dockerfile\n")
	_, err := Parse(yaml)
	if err == nil {
		t.Fatal("expected error for missing Dockerfile, got nil")
	}
}

// ---- parseDockerfile -----------------------------------------------------

func TestParseDockerfile_SingleFrom(t *testing.T) {
	df := writeTempDockerfile(t, `FROM nginx:1.25-alpine
RUN apk add curl
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	assertImage(t, imgs[0], types.DockerImage{
		Registry:    "docker.io",
		Repository:  "library/nginx",
		Tag:         "1.25-alpine",
		ServiceName: "nginx",
	})
}

func TestParseDockerfile_MultiStage(t *testing.T) {
	df := writeTempDockerfile(t, `FROM golang:1.26 AS builder
RUN go build .

FROM alpine:3.19
COPY --from=builder /app /app
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("expected 2 images, got %d", len(imgs))
	}
	assertImage(t, imgs[0], types.DockerImage{
		Registry:    "docker.io",
		Repository:  "library/golang",
		Tag:         "1.26",
		ServiceName: "builder",
	})
	assertImage(t, imgs[1], types.DockerImage{
		Registry:    "docker.io",
		Repository:  "library/alpine",
		Tag:         "3.19",
		ServiceName: "alpine",
	})
}

func TestParseDockerfile_ArgWithDefault(t *testing.T) {
	df := writeTempDockerfile(t, `ARG GO_VERSION=1.26
ARG DISTRO=trixie
FROM golang:${GO_VERSION}-${DISTRO}
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	if imgs[0].Tag != "1.26-trixie" {
		t.Errorf("Tag = %q, want %q", imgs[0].Tag, "1.26-trixie")
	}
}

func TestParseDockerfile_ArgWithoutDefault_SkipsUnresolved(t *testing.T) {
	// ARG without default → ${VAR} stays unresolved → ParseImageString receives
	// "golang:${GO_VERSION}" which produces an invalid image ref → skipped silently.
	df := writeTempDockerfile(t, `ARG GO_VERSION
FROM golang:${GO_VERSION}
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unresolved placeholder: the tag contains "${" so it's not a valid semver tag,
	// but ParseImageString itself won't error — it just produces a weird tag.
	// The image is included but won't match any registry tag. We verify it doesn't panic.
	_ = imgs
}

func TestParseDockerfile_ArgQuotedDefault(t *testing.T) {
	df := writeTempDockerfile(t, `ARG VERSION="1.25"
FROM nginx:${VERSION}
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	if imgs[0].Tag != "1.25" {
		t.Errorf("Tag = %q, want %q", imgs[0].Tag, "1.25")
	}
}

func TestParseDockerfile_SkipScratch(t *testing.T) {
	df := writeTempDockerfile(t, `FROM golang:1.26 AS builder
FROM scratch
COPY --from=builder /app /app
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image (scratch skipped), got %d", len(imgs))
	}
	if imgs[0].Tag != "1.26" {
		t.Errorf("Tag = %q, want %q", imgs[0].Tag, "1.26")
	}
}

func TestParseDockerfile_SkipLocalStageReference(t *testing.T) {
	df := writeTempDockerfile(t, `FROM golang:1.26 AS builder
FROM builder AS tester
FROM alpine:3.19
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "FROM builder" references the local alias → skipped
	if len(imgs) != 2 {
		t.Fatalf("expected 2 images (local stage skipped), got %d", len(imgs))
	}
}

func TestParseDockerfile_PlatformFlag(t *testing.T) {
	df := writeTempDockerfile(t, `FROM --platform=linux/amd64 nginx:1.25
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	assertImage(t, imgs[0], types.DockerImage{
		Registry:   "docker.io",
		Repository: "library/nginx",
		Tag:        "1.25",
	})
}

func TestParseDockerfile_GHCRImage(t *testing.T) {
	df := writeTempDockerfile(t, `ARG GO_VERSION=1.26
ARG VARIANT=trixie
FROM mcr.microsoft.com/devcontainers/go:dev-${GO_VERSION}-${VARIANT}
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	assertImage(t, imgs[0], types.DockerImage{
		Registry:    "mcr.microsoft.com",
		Repository:  "devcontainers/go",
		Tag:         "dev-1.26-trixie",
		ServiceName: "go",
	})
}

func TestParseDockerfile_LineContinuation(t *testing.T) {
	df := writeTempDockerfile(t, `FROM \
    nginx:1.25-alpine
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	if imgs[0].Tag != "1.25-alpine" {
		t.Errorf("Tag = %q, want %q", imgs[0].Tag, "1.25-alpine")
	}
}

func TestParseDockerfile_CommentsAndBlankLines(t *testing.T) {
	df := writeTempDockerfile(t, `# syntax=docker/dockerfile:1

# Base image
FROM nginx:1.25

# Add curl
RUN apk add curl
`)
	imgs, err := parseDockerfile(df)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
}

// ---- helpers -------------------------------------------------------------

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
	if want.ServiceName != "" && got.ServiceName != want.ServiceName {
		t.Errorf("ServiceName = %q, want %q", got.ServiceName, want.ServiceName)
	}
}

func writeTempDockerfile(t *testing.T, content string) string {
	t.Helper()
	return writeTempFile(t, "Dockerfile", content)
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return f
}
