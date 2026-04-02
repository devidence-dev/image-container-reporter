# Plan: Limpieza Total del Proyecto

## Objetivo

Eliminar ~700 líneas de código manual (HTTP, rate limiting, JSON parsing, auth) reemplazándolo con
`go-containerregistry`, y corregir la duplicación estructural entre `cmd/scan.go` y
`internal/scanner/scanner.go`.

## Alcance del cambio

| Categoría | Acción | Archivos |
|---|---|---|
| Clientes de registry | **Eliminar** | `dockerhub.go`, `ghcr.go` |
| Tests de clientes eliminados | **Eliminar** | `dockerhub_test.go`, `ghcr_test.go` |
| Tests de regresión de versiones | **Mover** a `pkg/utils/` | `dockerhub_portainer_test.go`, `dockerhub_multi_test.go` |
| Configuración | **Simplificar** | `pkg/types/config.go` |
| GenericRegistryClient | **Extender** con soporte de token | `internal/registry/generic.go` |
| Scanner service | **Nuevo método** `ScanImages()` | `internal/scanner/scanner.go` |
| Comando scan | **Refactorizar** | `cmd/scan.go` |
| Tests del cliente genérico | **Nuevo archivo** | `internal/registry/generic_test.go` |

---

## Paso 1 — Extender GenericRegistryClient con soporte de token

**Archivo:** `internal/registry/generic.go`

**Motivación:** El GHCR token que hoy vive en `config.yaml` (env: `GITHUB_TOKEN`) debe seguir
siendo soportado para acceder a registries privados. En lugar de eliminarlo, lo pasamos al
`GenericRegistryClient` vía un keychain personalizado.

**Cambios:**

1. Agregar campo `keychain authn.Keychain` al struct `GenericRegistryClient`.
2. Crear función `NewGenericRegistryClient(timeout, token)` donde `token` es opcional (`""`).
3. Implementar `tokenKeychain` privado:
   - Si el registry es `ghcr.io` y hay token configurado → devuelve `authn.FromConfig` con ese token.
   - Para cualquier otro registry → delega a `authn.DefaultKeychain`.

```go
// Constructor nuevo:
func NewGenericRegistryClient(timeout time.Duration, token string) *GenericRegistryClient {
    kc := buildKeychain(token)
    return &GenericRegistryClient{timeout: timeout, keychain: kc}
}

func buildKeychain(ghcrToken string) authn.Keychain {
    if ghcrToken == "" {
        return authn.DefaultKeychain
    }
    return &tokenKeychain{ghcrToken: ghcrToken, fallback: authn.DefaultKeychain}
}

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
```

4. En `GetLatestTags()` usar `remote.WithAuthFromKeychain(g.keychain)` en lugar de `authn.DefaultKeychain` hardcodeado.

---

## Paso 2 — Simplificar pkg/types/config.go

**Archivo:** `pkg/types/config.go`

**Motivación:** Con un único cliente genérico ya no tiene sentido tener flags `Enabled` por registry
ni `Timeout` por cliente. Se mantiene solo lo esencial.

**Antes:**
```go
type RegistryConfig struct {
    DockerHub DockerHubConfig `yaml:"dockerhub" json:"dockerhub"`
    GHCR      GHCRConfig      `yaml:"ghcr" json:"ghcr"`
    Timeout   int             `yaml:"timeout" json:"timeout"`
}

type DockerHubConfig struct {
    Enabled bool `yaml:"enabled" json:"enabled"`
    Timeout int  `yaml:"timeout" json:"timeout"`
}

type GHCRConfig struct {
    Enabled bool   `yaml:"enabled" json:"enabled"`
    Token   string `yaml:"token" json:"token" env:"GITHUB_TOKEN"`
    Timeout int    `yaml:"timeout" json:"timeout"`
}
```

**Después:**
```go
type RegistryConfig struct {
    GHCRToken string `yaml:"ghcr_token" json:"ghcr_token" env:"GITHUB_TOKEN"`
    Timeout   int    `yaml:"timeout" json:"timeout"`
}
```

`DockerHubConfig` y `GHCRConfig` se **eliminan**.

---

## Paso 3 — Agregar scanner.Service.ScanImages()

**Archivo:** `internal/scanner/scanner.go`

**Motivación:** `scanDockerDaemon()` en `cmd/scan.go` duplica la lógica de filtrado de versiones
(`FilterPreReleases → FilterTagsBySuffix → FindBestUpdateTag → CompareVersions`) que ya existe en
`checkForUpdates()`. La solución es agregar un método público al servicio que acepte imágenes
directamente en lugar de descubrirlas desde compose files.

**Nuevo método:**
```go
// ScanImages checks a pre-supplied list of images for updates.
// It is the counterpart of ScanDirectory for non-compose sources (e.g. Docker daemon).
func (s *Service) ScanImages(ctx context.Context, images []types.DockerImage, projectName string) (types.ScanResult, error)
```

La implementación reutiliza `checkForUpdates()` directamente — los canales, goroutines y lógica
de filtrado son idénticos. Solo cambia el origen de las imágenes.

---

## Paso 4 — Refactorizar cmd/scan.go

**Archivo:** `cmd/scan.go`

### 4a. Simplificar createScanService()

**Antes:** crea DockerHubClient + GHCRClient + GenericRegistryClient.

**Después:** solo crea GenericRegistryClient con el token de config:
```go
func createScanService(cfg *types.Config) *scanner.Service {
    composeParser := compose.NewParser()
    genericClient := registry.NewGenericRegistryClient(
        time.Duration(cfg.Registry.Timeout)*time.Second,
        cfg.Registry.GHCRToken,
    )
    return scanner.NewService(composeParser, []types.RegistryClient{genericClient}, slog.Default())
}
```

### 4b. Simplificar scanDockerDaemon()

**Antes:** duplica creación de clientes + duplica lógica de filtrado (~120 líneas).

**Después:** delega completamente al servicio:
```go
func scanDockerDaemon(...) (types.ScanResult, error) {
    images, err := dockerClient.ScanRunningContainers(ctx)
    ...
    // Filtrar imágenes locales (lógica que es específica de Docker daemon)
    var scannable []types.DockerImage
    for _, img := range images {
        if !isLocalImage(img) {
            scannable = append(scannable, img)
        }
    }

    scanSvc := createScanService(cfg)
    return scanSvc.ScanImages(ctx, scannable, "docker-daemon")
}
```

### 4c. Eliminar canHandleRegistryForImage()

La función (líneas 529-541) es idéntica a `canHandleRegistry()` del scanner. Ya no se usa una
vez que `scanDockerDaemon()` delega al servicio.

---

## Paso 5 — Simplificar scanner.canHandleRegistry()

**Archivo:** `internal/scanner/scanner.go`

Con solo `GenericRegistryClient` en la lista (que devuelve `true` siempre), `canHandleRegistry()`
se convierte en letra muerta. Sin embargo, el método es parte del contrato interno del servicio
(permite registrar clientes especializados en el futuro). Se puede simplificar a:

```go
func (s *Service) canHandleRegistry(client types.RegistryClient, registry string) bool {
    clientName := strings.ToLower(client.Name())
    registryLower := strings.ToLower(registry)
    switch clientName {
    case "generic":
        return true
    default:
        return clientName == registryLower || (clientName == "docker.io" && registryLower == "")
    }
}
```

O mantener la versión completa si se anticipa re-agregar clientes especializados en el futuro.

---

## Paso 6 — Mover tests de regresión de versiones a pkg/utils/

**Archivos origen:** `dockerhub_portainer_test.go`, `dockerhub_multi_test.go`

**Motivación:** Estos tests no prueban el cliente Docker Hub en sí — prueban que la cadena
`FilterPreReleases → SortVersions → FindBestUpdateTag → CompareVersions` funciona correctamente
con conjuntos de tags reales de Portainer, Caddy, AdGuard, etc. Son tests de regresión valiosos
pero están en el lugar equivocado.

**Acción:** Crear `pkg/utils/version_regression_test.go` con los mismos casos de prueba pero
usando directamente los tags como slice, sin pasar por ningún cliente de registry. Los casos
de test se mantienen idénticos.

---

## Paso 7 — Eliminar archivos del registry manual

```
rm internal/registry/dockerhub.go
rm internal/registry/ghcr.go
rm internal/registry/dockerhub_test.go
rm internal/registry/ghcr_test.go
rm internal/registry/dockerhub_portainer_test.go  ← después de migrar en Paso 6
rm internal/registry/dockerhub_multi_test.go      ← después de migrar en Paso 6
```

---

## Paso 8 — Crear internal/registry/generic_test.go

Tests unitarios para las partes testables del cliente genérico:

- `TestGenericRegistryClient_Name` → devuelve "generic"
- `TestBuildRepoReference` → tabla de casos (docker.io/nginx, ghcr.io/user/app, quay.io/org/img)
- `TestIsValidGenericTag` → tabla de casos (vacío, SHA hex, temp/tmp, versiones válidas)
- `TestTokenKeychain_GHCRWithToken` → resolver ghcr.io devuelve auth configurado
- `TestTokenKeychain_OtherRegistryFallsBack` → resolver docker.io delega a DefaultKeychain

---

## Paso 9 — Verificar compilación y tests

```bash
go build ./...
go test ./...
```

---

## Resumen de archivos

### Eliminados
| Archivo | Líneas eliminadas |
|---|---|
| `internal/registry/dockerhub.go` | 277 |
| `internal/registry/ghcr.go` | 261 |
| `internal/registry/dockerhub_test.go` | 191 |
| `internal/registry/ghcr_test.go` | 310 |
| `internal/registry/dockerhub_portainer_test.go` | 266 |
| `internal/registry/dockerhub_multi_test.go` | 338 |
| **Total eliminado** | **~1.643 líneas** |

### Modificados
| Archivo | Cambio neto estimado |
|---|---|
| `internal/registry/generic.go` | +30 líneas (tokenKeychain) |
| `internal/scanner/scanner.go` | +25 líneas (ScanImages) |
| `cmd/scan.go` | −120 líneas (createScanService, scanDockerDaemon, canHandleRegistryForImage) |
| `pkg/types/config.go` | −12 líneas (structs eliminados) |

### Creados
| Archivo | Contenido |
|---|---|
| `internal/registry/generic_test.go` | Tests unitarios para generic.go |
| `pkg/utils/version_regression_test.go` | Tests de regresión migrados |

---

## Beneficios

- **−1.300 líneas netas** de código de producción
- **Sin límite de 100 tags** (DockerHub tenía `page_size=100` sin paginación real)
- **Auth unificada** via `~/.docker/config.json` + token opcional en config
- **Un solo lugar** para lógica de filtrado de versiones (scanner.Service)
- **OCI estándar** para cualquier registry sin código extra
