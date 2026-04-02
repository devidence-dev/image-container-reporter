package registry

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// GenericRegistryClient implements RegistryClient for any OCI-compatible registry
// using the standard OCI Distribution Specification via google/go-containerregistry.
// It acts as a universal fallback for registries not handled by specialized clients.
type GenericRegistryClient struct {
	timeout  time.Duration
	keychain authn.Keychain
}

// NewGenericRegistryClient creates a new generic OCI registry client.
// ghcrToken is optional: when non-empty it is used as a Bearer token for ghcr.io,
// which allows access to private GHCR images. All other registries fall back to
// credentials from ~/.docker/config.json (authn.DefaultKeychain).
func NewGenericRegistryClient(timeout time.Duration, ghcrToken string) *GenericRegistryClient {
	return &GenericRegistryClient{
		timeout:  timeout,
		keychain: buildKeychain(ghcrToken),
	}
}

// buildKeychain returns a keychain that injects a Bearer token for ghcr.io when
// provided, and falls back to the Docker config keychain for everything else.
func buildKeychain(ghcrToken string) authn.Keychain {
	if ghcrToken == "" {
		return authn.DefaultKeychain
	}
	return &tokenKeychain{ghcrToken: ghcrToken, fallback: authn.DefaultKeychain}
}

// tokenKeychain provides ghcr.io authentication via a configured token and
// delegates all other registries to the Docker config keychain.
type tokenKeychain struct {
	ghcrToken string
	fallback  authn.Keychain
}

func (k *tokenKeychain) Resolve(res authn.Resource) (authn.Authenticator, error) {
	if strings.HasSuffix(res.RegistryStr(), "ghcr.io") {
		return authn.FromConfig(authn.AuthConfig{
			Username: "x-access-token",
			Password: k.ghcrToken,
		}), nil
	}
	return k.fallback.Resolve(res)
}

// Name returns "generic" to indicate this client handles any registry as a fallback.
func (g *GenericRegistryClient) Name() string {
	return "generic"
}

// GetLatestTags fetches all tags for the given image from any OCI-compatible registry.
// Authentication is resolved automatically from ~/.docker/config.json via the default keychain.
func (g *GenericRegistryClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	repoRef := buildRepoReference(image)

	repo, err := name.NewRepository(repoRef)
	if err != nil {
		return nil, errors.Wrapf("generic.GetLatestTags", err, "parsing repository %s", repoRef)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	tags, err := remote.List(repo,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(g.keychain),
	)
	if err != nil {
		return nil, errors.Wrapf("generic.GetLatestTags", err, "listing tags for %s", repoRef)
	}

	filtered := make([]string, 0, len(tags))
	for _, t := range tags {
		if isValidGenericTag(t) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		return nil, errors.Newf("generic.GetLatestTags", "no valid tags found for %s", repoRef)
	}

	return filtered, nil
}

// GetImageInfo returns basic image metadata. Tag listing is the primary use case.
func (g *GenericRegistryClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	tags, err := g.GetLatestTags(ctx, image)
	if err != nil {
		tags = []string{image.Tag}
	}
	return &types.ImageInfo{
		Tags:         tags,
		LastModified: time.Now(),
		Architecture: "amd64",
	}, nil
}

// buildRepoReference constructs the full repository reference understood by go-containerregistry.
// go-containerregistry handles docker.io and library/ prefixes natively.
func buildRepoReference(image types.DockerImage) string {
	repo := image.Repository
	// Strip known registry prefixes; go-containerregistry adds them back correctly.
	repo = strings.TrimPrefix(repo, "docker.io/")
	repo = strings.TrimPrefix(repo, "index.docker.io/")

	if image.Registry != "" &&
		image.Registry != "docker.io" &&
		image.Registry != "index.docker.io" {
		return image.Registry + "/" + repo
	}
	return repo
}

// isValidGenericTag returns true for tags that are useful for version comparison.
func isValidGenericTag(tag string) bool {
	if tag == "" {
		return false
	}
	tagLower := strings.ToLower(tag)
	if strings.Contains(tagLower, "temp") || strings.Contains(tagLower, "tmp") {
		return false
	}
	// Reject SHA digests used as tags (12-64 hex chars)
	if len(tag) >= 12 && len(tag) <= 64 && isHexString(tag) {
		return false
	}
	return true
}
