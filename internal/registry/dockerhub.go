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

// DockerHubClient implementa RegistryClient para Docker Hub
type DockerHubClient struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	baseURL     string
}

// NewDockerHubClient crea un nuevo cliente para Docker Hub
func NewDockerHubClient(timeout time.Duration) *DockerHubClient {
	return &DockerHubClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		// Docker Hub permite ~100 requests por 6 horas para usuarios anónimos
		// Usamos un rate limit conservador de 10 requests por minuto
		rateLimiter: rate.NewLimiter(rate.Every(6*time.Second), 10),
		baseURL:     "https://registry.hub.docker.com/v2",
	}
}

// Name devuelve el nombre del registro
func (d *DockerHubClient) Name() string {
	return "docker.io"
}

// GetLatestTags obtiene las etiquetas más recientes de una imagen
func (d *DockerHubClient) GetLatestTags(ctx context.Context, image types.DockerImage) ([]string, error) {
	// Aplicar rate limiting
	if err := d.rateLimiter.Wait(ctx); err != nil {
		return nil, errors.Wrap("dockerhub.GetLatestTags", err)
	}

	// Normalizar el nombre del repositorio para Docker Hub
	repository := d.normalizeRepository(image.Repository)

	// Usar la API pública de Docker Hub para obtener tags
	// Incrementamos page_size y usamos ordering por nombre para obtener versiones más recientes
	url := fmt.Sprintf("%s/repositories/%s/tags?page_size=100", d.baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf("dockerhub.GetLatestTags", err, "creating request for %s", repository)
	}

	req.Header.Set("User-Agent", "docker-image-reporter/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf("dockerhub.GetLatestTags", err, "making request to %s", url)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.Newf("dockerhub.GetLatestTags", "repository %s not found", repository)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("dockerhub.GetLatestTags", "unexpected status %d for %s", resp.StatusCode, repository)
	}

	var response DockerHubTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, errors.Wrapf("dockerhub.GetLatestTags", err, "decoding response for %s", repository)
	}

	tags := make([]string, 0, len(response.Results))
	for _, result := range response.Results {
		// Filtrar tags que no son útiles
		if d.isValidTag(result.Name) {
			tags = append(tags, result.Name)
		}
	}

	if len(tags) == 0 {
		return nil, errors.Newf("dockerhub.GetLatestTags", "no valid tags found for %s", repository)
	}

	return tags, nil
}

// GetImageInfo obtiene información detallada de una imagen
func (d *DockerHubClient) GetImageInfo(ctx context.Context, image types.DockerImage) (*types.ImageInfo, error) {
	// Aplicar rate limiting
	if err := d.rateLimiter.Wait(ctx); err != nil {
		return nil, errors.Wrap("dockerhub.GetImageInfo", err)
	}

	repository := d.normalizeRepository(image.Repository)

	// Obtener información del repositorio
	url := fmt.Sprintf("%s/repositories/%s", d.baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf("dockerhub.GetImageInfo", err, "creating request for %s", repository)
	}

	req.Header.Set("User-Agent", "docker-image-reporter/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf("dockerhub.GetImageInfo", err, "making request to %s", url)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.Newf("dockerhub.GetImageInfo", "repository %s not found", repository)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("dockerhub.GetImageInfo", "unexpected status %d for %s", resp.StatusCode, repository)
	}

	var repoInfo DockerHubRepositoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, errors.Wrapf("dockerhub.GetImageInfo", err, "decoding repository response for %s", repository)
	}

	// Obtener tags para información adicional
	tags, err := d.GetLatestTags(ctx, image)
	if err != nil {
		// Si no podemos obtener tags, devolver información básica
		tags = []string{image.Tag}
	}

	return &types.ImageInfo{
		Tags:         tags,
		LastModified: parseDockerHubTime(repoInfo.LastUpdated),
		Size:         0,       // Docker Hub API pública no proporciona tamaño fácilmente
		Architecture: "amd64", // Asumir amd64 por defecto
	}, nil
}

// normalizeRepository normaliza el nombre del repositorio para Docker Hub
func (d *DockerHubClient) normalizeRepository(repository string) string {
	// Remover docker.io/ si está presente
	repository = strings.TrimPrefix(repository, "docker.io/")

	// Si no tiene namespace, agregar library/
	if !strings.Contains(repository, "/") {
		return "library/" + repository
	}

	return repository
}

// isValidTag determina si un tag es válido para consideración
func (d *DockerHubClient) isValidTag(tag string) bool {
	tagLower := strings.ToLower(tag)

	// Siempre permitir latest
	if tagLower == "latest" {
		return true
	}

	// Filtrar tags de desarrollo específicos
	developmentTags := []string{
		"nightly", "snapshot", "dev-", "devel-", "development-",
		"unstable-", "canary-", "alpha", "beta", "rc-", "test-",
	}

	for _, invalid := range developmentTags {
		if strings.Contains(tagLower, invalid) {
			return false
		}
	}

	// Filtrar tags de arquitectura específica
	archTags := []string{"linux-", "windows-", "arm64-", "amd64-", "ppc64le-", "s390x-"}
	for _, arch := range archTags {
		if strings.HasPrefix(tagLower, arch) {
			return false
		}
	}

	// Filtrar tags que parecen commits SHA (12+ caracteres, mostly hex)
	if len(tag) >= 12 && len(tag) <= 40 && isHexString(tag) {
		return false
	}

	// Filtrar tags temporales
	if strings.Contains(tagLower, "temp") || strings.Contains(tagLower, "tmp") {
		return false
	}

	// Permitir versiones semánticas y otros tags razonables
	// Al menos debe tener algún dígito o ser un tag común
	if strings.ContainsAny(tag, "0123456789") || tagLower == "stable" || tagLower == "lts" || tagLower == "development" {
		return true
	}

	// Para otros tags sin números, ser más conservador
	// Solo permitir si es corto y parece un nombre de versión
	if len(tag) <= 10 && !strings.ContainsAny(tag, "_-+") {
		return true
	}

	return false
}

// isHexString verifica si una string es hexadecimal
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) { //nolint:staticcheck
			return false
		}
	}
	return true
}

// parseDockerHubTime parsea el formato de tiempo de Docker Hub
func parseDockerHubTime(timeStr string) time.Time {
	// Docker Hub usa formato ISO 8601
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	// Si no podemos parsear, devolver tiempo actual
	return time.Now()
}

// DockerHubTagsResponse representa la respuesta de la API de tags de Docker Hub
type DockerHubTagsResponse struct {
	Count    int                `json:"count"`
	Next     *string            `json:"next"`
	Previous *string            `json:"previous"`
	Results  []DockerHubTagInfo `json:"results"`
}

// DockerHubTagInfo representa información de un tag en Docker Hub
type DockerHubTagInfo struct {
	Name        string `json:"name"`
	FullSize    int64  `json:"full_size"`
	LastUpdated string `json:"last_updated"`
	LastPushed  string `json:"last_pushed"`
	Images      []struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		Size         int64  `json:"size"`
	} `json:"images"`
}

// DockerHubRepositoryResponse representa la respuesta de información del repositorio
type DockerHubRepositoryResponse struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
	LastUpdated string `json:"last_updated"`
	PullCount   int64  `json:"pull_count"`
	StarCount   int    `json:"star_count"`
}
