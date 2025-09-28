package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
	"golang.org/x/time/rate"
)

// GHCRClient implementa RegistryClient para GitHub Container Registry
type GHCRClient struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	token       string
	baseURL     string
}

// NewGHCRClient crea un nuevo cliente para GitHub Container Registry
func NewGHCRClient(token string, timeout time.Duration) *GHCRClient {
	return &GHCRClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		// GitHub API permite 5000 requests por hora para usuarios autenticados
		// Usamos un rate limit conservador de 60 requests por minuto
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 60),
		token:       token,
		baseURL:     "https://api.github.com",
	}
}

// Name devuelve el nombre del registro
func (g *GHCRClient) Name() string {
	return "ghcr.io"
}

// GetLatestTags obtiene las etiquetas más recientes de una imagen
func (g *GHCRClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	// Aplicar rate limiting
	if err := g.rateLimiter.Wait(ctx); err != nil {
		return nil, errors.Wrap("ghcr.GetLatestTags", err)
	}

	owner, packageName := g.parseRepository(image.Repository)
	if owner == "" || packageName == "" {
		return nil, errors.Newf("ghcr.GetLatestTags", "invalid repository format: %s", image.Repository)
	}

	// Usar GitHub Packages API para obtener versiones
	url := fmt.Sprintf("%s/user/packages/container/%s/versions", g.baseURL, packageName)
	if owner != "" {
		url = fmt.Sprintf("%s/orgs/%s/packages/container/%s/versions", g.baseURL, owner, packageName)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf("ghcr.GetLatestTags", err, "creating request for %s", image.Repository)
	}

	// Configurar headers de autenticación
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "docker-image-reporter/1.0")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf("ghcr.GetLatestTags", err, "making request to %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.Newf("ghcr.GetLatestTags", "package %s not found or not accessible", image.Repository)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("ghcr.GetLatestTags", "unauthorized - check GitHub token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("ghcr.GetLatestTags", "unexpected status %d for %s", resp.StatusCode, image.Repository)
	}

	var versions []GitHubPackageVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, errors.Wrapf("ghcr.GetLatestTags", err, "decoding response for %s", image.Repository)
	}

	tags := make([]string, 0, len(versions))
	for _, version := range versions {
		for _, tag := range version.Metadata.Container.Tags {
			if g.isValidTag(tag) {
				tags = append(tags, tag)
			}
		}
	}

	if len(tags) == 0 {
		return nil, errors.Newf("ghcr.GetLatestTags", "no valid tags found for %s", image.Repository)
	}

	return tags, nil
}

// GetImageInfo obtiene información detallada de una imagen
func (g *GHCRClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	// Aplicar rate limiting
	if err := g.rateLimiter.Wait(ctx); err != nil {
		return nil, errors.Wrap("ghcr.GetImageInfo", err)
	}

	owner, packageName := g.parseRepository(image.Repository)
	if owner == "" || packageName == "" {
		return nil, errors.Newf("ghcr.GetImageInfo", "invalid repository format: %s", image.Repository)
	}

	// Obtener información del paquete
	url := fmt.Sprintf("%s/user/packages/container/%s", g.baseURL, packageName)
	if owner != "" {
		url = fmt.Sprintf("%s/orgs/%s/packages/container/%s", g.baseURL, owner, packageName)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf("ghcr.GetImageInfo", err, "creating request for %s", image.Repository)
	}

	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "docker-image-reporter/1.0")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf("ghcr.GetImageInfo", err, "making request to %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.Newf("ghcr.GetImageInfo", "package %s not found or not accessible", image.Repository)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("ghcr.GetImageInfo", "unexpected status %d for %s", resp.StatusCode, image.Repository)
	}

	var packageInfo GitHubPackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&packageInfo); err != nil {
		return nil, errors.Wrapf("ghcr.GetImageInfo", err, "decoding package response for %s", image.Repository)
	}

	// Obtener tags
	tags, err := g.GetLatestTags(ctx, image)
	if err != nil {
		// Si no podemos obtener tags, usar el tag actual
		tags = []string{image.Tag}
	}

	return &types.ImageInfo{
		Tags:         tags,
		LastModified: parseGitHubTime(packageInfo.UpdatedAt),
		Size:         0, // GitHub Packages API no proporciona tamaño fácilmente
		Architecture: "amd64", // Asumir amd64 por defecto
	}, nil
}

// parseRepository parsea el repositorio GHCR en owner y package name
func (g *GHCRClient) parseRepository(repository string) (string, string) {
	// Remover ghcr.io/ si está presente
	repository = strings.TrimPrefix(repository, "ghcr.io/")
	
	parts := strings.Split(repository, "/")
	if len(parts) < 2 {
		return "", ""
	}

	// Para GHCR, el formato es owner/package-name
	owner := parts[0]
	packageName := strings.Join(parts[1:], "/")

	return owner, packageName
}

// isValidTag determina si un tag es válido para consideración
func (g *GHCRClient) isValidTag(tag string) bool {
	// Similar a Docker Hub pero más permisivo para repositorios privados
	if tag == "" {
		return false
	}

	// Filtrar tags que parecen commits SHA
	if len(tag) >= 7 && len(tag) <= 40 && isHexString(tag) {
		return false
	}

	// Filtrar algunos tags de desarrollo comunes
	tagLower := strings.ToLower(tag)
	if strings.Contains(tagLower, "temp") || strings.Contains(tagLower, "tmp") {
		return false
	}

	return true
}

// parseGitHubTime parsea el formato de tiempo de GitHub
func parseGitHubTime(timeStr string) time.Time {
	// GitHub usa formato ISO 8601
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	return time.Now()
}

// GitHubPackageVersion representa una versión de paquete en GitHub
type GitHubPackageVersion struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Metadata  struct {
		PackageType string `json:"package_type"`
		Container   struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

// GitHubPackageInfo representa información de un paquete en GitHub
type GitHubPackageInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PackageType string `json:"package_type"`
	Owner       struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
	VersionCount int    `json:"version_count"`
	Visibility   string `json:"visibility"`
	URL          string `json:"url"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}