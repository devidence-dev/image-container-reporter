# Plan: GenericRegistryClient con `go-containerregistry`

## Objetivo

Agregar soporte universal para cualquier registry OCI-compatible (Quay.io, registries
privados, etc.) y superar las limitaciones actuales:

- DockerHub: solo 100 tags via API propietaria (sin paginación)
- GHCR: usa GitHub Packages API en lugar del protocolo OCI estándar
- Cualquier otro registry: no tiene cliente → error "no registry client available"

## Estrategia

Agregar `GenericRegistryClient` usando `google/go-containerregistry` como **fallback
universal**. Los clientes DockerHub y GHCR existentes se mantienen intactos. El cliente
genérico actúa cuando ningún cliente específico puede manejar el registry.

Flujo de selección de cliente en el scanner:
1. `docker.io` → DockerHubClient (si habilitado en config)
2. `ghcr.io` → GHCRClient (si habilitado en config)
3. Cualquier otro registry → **GenericRegistryClient** (siempre habilitado)
4. Si DockerHub está deshabilitado y la imagen es `docker.io` → GenericRegistryClient

---

## Paso 1: Agregar la dependencia

```bash
go get github.com/google/go-containerregistry@latest
```

Esto actualiza `go.mod` y `go.sum` automáticamente.

---

## Paso 2: Crear `internal/registry/generic.go`

Crear el archivo con el siguiente contenido:

```go
package registry

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/remote"
	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// GenericRegistryClient implements RegistryClient for any OCI-compatible registry
// using the standard OCI Distribution Specification via google/go-containerregistry.
// It acts as a universal fallback for registries not handled by specialized clients.
type GenericRegistryClient struct {
	timeout time.Duration
}

// NewGenericRegistryClient creates a new generic OCI registry client.
func NewGenericRegistryClient(timeout time.Duration) *GenericRegistryClient {
	return &GenericRegistryClient{timeout: timeout}
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
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
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
```

---

## Paso 3: Actualizar `internal/scanner/scanner.go`

### Cambio en `canHandleRegistry`

Buscar el método `canHandleRegistry` (línea ~325) y agregar el caso `"generic"`:

```go
// canHandleRegistry checks if a registry client can handle the given registry
func (s *Service) canHandleRegistry(client types.RegistryClient, registry string) bool {
	clientName := strings.ToLower(client.Name())
	registryName := strings.ToLower(registry)

	switch clientName {
	case "docker.io", "dockerhub":
		return registryName == "docker.io" || registryName == ""
	case "ghcr.io", "ghcr":
		return registryName == "ghcr.io"
	case "generic":
		return true // handles any registry as universal fallback
	default:
		return clientName == registryName
	}
}
```

> **Nota**: Como el cliente genérico se agrega al **final** de la lista de registries, los
> clientes especializados (DockerHub, GHCR) siempre tienen prioridad cuando están habilitados.

---

## Paso 4: Actualizar `cmd/scan.go`

Hay **dos funciones** que construyen la lista de clientes de registry. Ambas necesitan el mismo cambio.

### 4a. Función `createScanService` (línea ~180)

**Antes:**
```go
func createScanService(cfg *types.Config) *scanner.Service {
	composeParser := compose.NewParser()

	var registryClients []types.RegistryClient

	if cfg.Registry.DockerHub.Enabled {
		dockerHubClient := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
		registryClients = append(registryClients, dockerHubClient)
	}

	if cfg.Registry.GHCR.Enabled {
		ghcrClient := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
		registryClients = append(registryClients, ghcrClient)
	}

	scanSvc := scanner.NewService(composeParser, registryClients, slog.Default())
	return scanSvc
}
```

**Después:** agregar el cliente genérico al final:
```go
func createScanService(cfg *types.Config) *scanner.Service {
	composeParser := compose.NewParser()

	var registryClients []types.RegistryClient

	if cfg.Registry.DockerHub.Enabled {
		dockerHubClient := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
		registryClients = append(registryClients, dockerHubClient)
	}

	if cfg.Registry.GHCR.Enabled {
		ghcrClient := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
		registryClients = append(registryClients, ghcrClient)
	}

	// Generic OCI client as universal fallback for any registry not handled above.
	// Also handles docker.io/ghcr.io when their dedicated clients are disabled.
	genericClient := registry.NewGenericRegistryClient(time.Duration(cfg.Registry.Timeout) * time.Second)
	registryClients = append(registryClients, genericClient)

	scanSvc := scanner.NewService(composeParser, registryClients, slog.Default())
	return scanSvc
}
```

### 4b. Función `scanDockerDaemon` (línea ~318-332)

**Antes:**
```go
var registryClients []types.RegistryClient

if cfg.Registry.DockerHub.Enabled {
    dockerHubClient := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
    registryClients = append(registryClients, dockerHubClient)
}

if cfg.Registry.GHCR.Enabled {
    ghcrClient := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
    registryClients = append(registryClients, ghcrClient)
}
```

**Después:** agregar el cliente genérico al final del mismo bloque:
```go
var registryClients []types.RegistryClient

if cfg.Registry.DockerHub.Enabled {
    dockerHubClient := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
    registryClients = append(registryClients, dockerHubClient)
}

if cfg.Registry.GHCR.Enabled {
    ghcrClient := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
    registryClients = append(registryClients, ghcrClient)
}

// Generic OCI client as universal fallback for any registry not handled above.
genericClient := registry.NewGenericRegistryClient(time.Duration(cfg.Registry.Timeout) * time.Second)
registryClients = append(registryClients, genericClient)
```

---

## Paso 5: Verificar que compila

```bash
go build ./...
go test ./...
```

---

## Resumen de archivos modificados

| Archivo | Tipo de cambio |
|---|---|
| `go.mod` + `go.sum` | Agregar `github.com/google/go-containerregistry` |
| `internal/registry/generic.go` | **Nuevo archivo** |
| `internal/scanner/scanner.go` | Agregar caso `"generic"` en `canHandleRegistry` |
| `cmd/scan.go` | Agregar `genericClient` en `createScanService` y `scanDockerDaemon` |

## Beneficios

- **Cualquier registry OCI** funciona automáticamente (Quay.io, Harbor, registries privados, etc.)
- **Todos los tags** disponibles via protocolo OCI estándar (sin límite de 100)
- **Auth automática** via `~/.docker/config.json` (Docker login credentials)
- **Sin código extra por registry** — un solo cliente para todos los casos no cubiertos
- **No rompe nada** — los clientes DockerHub y GHCR siguen con prioridad
