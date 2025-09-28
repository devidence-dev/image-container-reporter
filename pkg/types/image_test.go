package types

import "testing"

func TestDockerImage_String(t *testing.T) {
	tests := []struct {
		name     string
		image    DockerImage
		expected string
	}{
		{
			name: "docker hub image",
			image: DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			expected: "nginx:latest",
		},
		{
			name: "ghcr image",
			image: DockerImage{
				Registry:   "ghcr.io",
				Repository: "owner/repo",
				Tag:        "v1.0.0",
			},
			expected: "ghcr.io/owner/repo:v1.0.0",
		},
		{
			name: "private registry",
			image: DockerImage{
				Registry:   "registry.example.com",
				Repository: "myapp",
				Tag:        "1.2.3",
			},
			expected: "registry.example.com/myapp:1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.image.String()
			if result != tt.expected {
				t.Errorf("DockerImage.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDockerImage_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		image    DockerImage
		expected bool
	}{
		{
			name: "valid image",
			image: DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			expected: true,
		},
		{
			name: "missing registry",
			image: DockerImage{
				Repository: "nginx",
				Tag:        "latest",
			},
			expected: false,
		},
		{
			name: "missing repository",
			image: DockerImage{
				Registry: "docker.io",
				Tag:      "latest",
			},
			expected: false,
		},
		{
			name: "missing tag",
			image: DockerImage{
				Registry:   "docker.io",
				Repository: "nginx",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.image.IsValid()
			if result != tt.expected {
				t.Errorf("DockerImage.IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}
