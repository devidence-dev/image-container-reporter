package docker

import (
	"log/slog"
	"testing"

	"github.com/docker/docker/api/types/container"
	dockerTypes "github.com/user/docker-image-reporter/pkg/types"
)

func TestParseImageString(t *testing.T) {
	logger := slog.Default()
	client := &Client{logger: logger}

	tests := []struct {
		name     string
		imageStr string
		want     dockerTypes.DockerImage
		wantErr  bool
	}{
		{
			name:     "simple image with tag",
			imageStr: "nginx:1.20",
			want: dockerTypes.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.20",
			},
		},
		{
			name:     "image without tag (latest implied)",
			imageStr: "nginx",
			want: dockerTypes.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
			},
		},
		{
			name:     "user repository with tag",
			imageStr: "user/app:v1.0.0",
			want: dockerTypes.DockerImage{
				Registry:   "docker.io",
				Repository: "user/app",
				Tag:        "v1.0.0",
			},
		},
		{
			name:     "private registry with port",
			imageStr: "registry.example.com:5000/myapp:1.0",
			want: dockerTypes.DockerImage{
				Registry:   "registry.example.com:5000",
				Repository: "myapp",
				Tag:        "1.0",
			},
		},
		{
			name:     "ghcr image",
			imageStr: "ghcr.io/user/app:latest",
			want: dockerTypes.DockerImage{
				Registry:   "ghcr.io",
				Repository: "user/app",
				Tag:        "latest",
			},
		},
		{
			name:     "image with digest",
			imageStr: "nginx@sha256:abc123",
			want: dockerTypes.DockerImage{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Digest:     "sha256:abc123",
			},
		},
		{
			name:     "empty string",
			imageStr: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.parseImageString(tt.imageStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseImageString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseImageString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRegistryAndRepository(t *testing.T) {
	logger := slog.Default()
	client := &Client{logger: logger}

	tests := []struct {
		name           string
		imageStr       string
		wantRegistry   string
		wantRepository string
	}{
		{
			name:           "simple image name",
			imageStr:       "nginx",
			wantRegistry:   "docker.io",
			wantRepository: "library/nginx",
		},
		{
			name:           "user image",
			imageStr:       "user/app",
			wantRegistry:   "docker.io",
			wantRepository: "user/app",
		},
		{
			name:           "registry with domain",
			imageStr:       "registry.example.com/app",
			wantRegistry:   "registry.example.com",
			wantRepository: "app",
		},
		{
			name:           "registry with port",
			imageStr:       "localhost:5000/app",
			wantRegistry:   "localhost:5000",
			wantRepository: "app",
		},
		{
			name:           "ghcr registry",
			imageStr:       "ghcr.io/user/app",
			wantRegistry:   "ghcr.io",
			wantRepository: "user/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, repository := client.parseRegistryAndRepository(tt.imageStr)
			if registry != tt.wantRegistry {
				t.Errorf("parseRegistryAndRepository() registry = %v, want %v", registry, tt.wantRegistry)
			}
			if repository != tt.wantRepository {
				t.Errorf("parseRegistryAndRepository() repository = %v, want %v", repository, tt.wantRepository)
			}
		})
	}
}

func TestExtractServiceName(t *testing.T) {
	logger := slog.Default()
	client := &Client{logger: logger}

	tests := []struct {
		name   string
		cont   container.Summary
		labels map[string]string
		want   string
	}{
		{
			name: "compose service label",
			cont: container.Summary{Names: []string{"/myapp_web_1"}},
			labels: map[string]string{
				"com.docker.compose.service": "web",
			},
			want: "web",
		},
		{
			name: "compose project and service labels",
			cont: container.Summary{Names: []string{"/myapp_web_1"}},
			labels: map[string]string{
				"com.docker.compose.project": "myapp",
				"com.docker.compose.service": "web",
			},
			want: "web",
		},
		{
			name:   "no compose labels, use container name",
			cont:   container.Summary{Names: []string{"/nginx_container"}},
			labels: map[string]string{},
			want:   "nginx_container",
		},
		{
			name:   "container name with suffix",
			cont:   container.Summary{Names: []string{"/myapp_web_1"}},
			labels: map[string]string{},
			want:   "myapp_web",
		},
		{
			name:   "container name with multi-digit suffix",
			cont:   container.Summary{Names: []string{"/myapp_web_10"}},
			labels: map[string]string{},
			want:   "myapp_web", // Strip two-digit numeric suffixes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.extractServiceName(tt.cont, tt.labels)
			if got != tt.want {
				t.Errorf("extractServiceName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test getContainerName functionality
func TestGetContainerName(t *testing.T) {
	logger := slog.Default()
	client := &Client{logger: logger}

	tests := []struct {
		name string
		cont container.Summary
		want string
	}{
		{
			name: "normal container name",
			cont: container.Summary{Names: []string{"/nginx_container"}},
			want: "nginx_container",
		},
		{
			name: "multiple names, use first",
			cont: container.Summary{Names: []string{"/name1", "/name2"}},
			want: "name1",
		},
		{
			name: "no names, use container ID",
			cont: container.Summary{Names: []string{}, ID: "abc123def456"},
			want: "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.getContainerName(tt.cont)
			if got != tt.want {
				t.Errorf("getContainerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
