package registry

import (
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/user/docker-image-reporter/pkg/types"
)

func TestGenericRegistryClient_Name(t *testing.T) {
	client := NewGenericRegistryClient(30*time.Second, "")
	if got := client.Name(); got != "generic" {
		t.Fatalf("Name() = %q, want %q", got, "generic")
	}
}

func TestBuildRepoReference(t *testing.T) {
	tests := []struct {
		name     string
		image    types.DockerImage
		expected string
	}{
		{
			name: "docker io official image",
			image: types.DockerImage{
				Registry:   "docker.io",
				Repository: "docker.io/nginx",
			},
			expected: "nginx",
		},
		{
			name: "ghcr image",
			image: types.DockerImage{
				Registry:   "ghcr.io",
				Repository: "user/app",
			},
			expected: "ghcr.io/user/app",
		},
		{
			name: "quay image",
			image: types.DockerImage{
				Registry:   "quay.io",
				Repository: "org/img",
			},
			expected: "quay.io/org/img",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildRepoReference(tt.image); got != tt.expected {
				t.Fatalf("buildRepoReference() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsValidGenericTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected bool
	}{
		{name: "empty", tag: "", expected: false},
		{name: "sha like hex", tag: "abc123def456", expected: false},
		{name: "temporary tag", tag: "tmp-build", expected: false},
		{name: "temp tag", tag: "my-temp-tag", expected: false},
		{name: "valid semver", tag: "1.2.3", expected: true},
		{name: "valid latest", tag: "latest", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidGenericTag(tt.tag); got != tt.expected {
				t.Fatalf("isValidGenericTag(%q) = %v, want %v", tt.tag, got, tt.expected)
			}
		})
	}
}

func TestTokenKeychain_GHCRWithToken(t *testing.T) {
	kc := &tokenKeychain{
		ghcrToken: "secret-token",
		fallback:  &testKeychain{},
	}

	auth, err := kc.Resolve(testResource{registry: "ghcr.io"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	cfg, err := auth.Authorization()
	if err != nil {
		t.Fatalf("Authorization() error = %v", err)
	}

	if cfg.Username != "x-access-token" {
		t.Fatalf("username = %q, want %q", cfg.Username, "x-access-token")
	}
	if cfg.Password != "secret-token" {
		t.Fatalf("password = %q, want %q", cfg.Password, "secret-token")
	}
}

func TestTokenKeychain_OtherRegistryFallsBack(t *testing.T) {
	fallback := &testKeychain{auth: authn.FromConfig(authn.AuthConfig{Username: "u", Password: "p"})}
	kc := &tokenKeychain{
		ghcrToken: "secret-token",
		fallback:  fallback,
	}

	auth, err := kc.Resolve(testResource{registry: "docker.io"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !fallback.called {
		t.Fatal("expected fallback keychain to be used")
	}

	cfg, err := auth.Authorization()
	if err != nil {
		t.Fatalf("Authorization() error = %v", err)
	}
	if cfg.Username != "u" || cfg.Password != "p" {
		t.Fatalf("fallback credentials = %q/%q, want u/p", cfg.Username, cfg.Password)
	}
}

type testResource struct {
	registry string
}

func (r testResource) String() string {
	return r.registry
}

func (r testResource) RegistryStr() string {
	return r.registry
}

type testKeychain struct {
	called bool
	auth   authn.Authenticator
	err    error
}

func (k *testKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	k.called = true
	if k.auth != nil || k.err != nil {
		return k.auth, k.err
	}
	return authn.Anonymous, nil
}
