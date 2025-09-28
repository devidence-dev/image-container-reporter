package compose

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
	yaml "gopkg.in/yaml.v3"
)

// Parser implementa la interfaz ComposeParser para parsear archivos docker-compose
type Parser struct{}

// NewParser crea una nueva instancia del parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parsea un archivo docker-compose y extrae las imágenes Docker
func (p *Parser) ParseFile(ctx context.Context, filePath string) ([]types.DockerImage, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, errors.Wrapf("compose.ParseFile", err, "reading file %s", filePath)
	}

	// Load environment variables from .env file if it exists
	envVars := make(map[string]string)
	composeDir := filepath.Dir(filePath)
	envFile := filepath.Join(composeDir, ".env")
	if envData, err := os.ReadFile(envFile); err == nil { //nolint:gosec
		// Parse .env file content
		envVars = p.parseEnvFile(string(envData))
	}

	// Expand environment variables in the compose file content
	expandedData := p.expandEnvVars(string(data), envVars)

	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(expandedData), &compose); err != nil {
		return nil, errors.Wrapf("compose.ParseFile", err, "parsing YAML file %s", filePath)
	}

	var images []types.DockerImage
	for serviceName, service := range compose.Services {
		if service.Image == "" {
			// Skip services without image (they might use build instead)
			continue
		}

		image, err := p.parseImageString(service.Image)
		if err != nil {
			// Log warning but continue with other services
			continue
		}

		// Add service context to the image for better tracking
		image.ServiceName = serviceName
		image.ComposeFile = filePath

		images = append(images, image)
	}

	return images, nil
}

// CanParse determina si el parser puede manejar el archivo dado
func (p *Parser) CanParse(filePath string) bool {
	name := filepath.Base(filePath)

	// Patrones estándar de docker-compose
	patterns := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	// Verificar patrones exactos
	for _, pattern := range patterns {
		if name == pattern {
			return true
		}
	}

	// Verificar patrones con prefijos (docker-compose.prod.yml, etc.)
	if strings.HasPrefix(name, "docker-compose.") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
		return true
	}

	return false
}

// parseImageString parsea una string de imagen Docker en sus componentes
func (p *Parser) parseImageString(imageStr string) (types.DockerImage, error) {
	if imageStr == "" {
		return types.DockerImage{}, errors.New("compose.parseImageString", "empty image string")
	}

	// Remover espacios en blanco
	imageStr = strings.TrimSpace(imageStr)

	// Separar tag/digest
	var tag, digest string

	// Verificar si tiene digest (@sha256:...)
	if strings.Contains(imageStr, "@") {
		parts := strings.Split(imageStr, "@")
		if len(parts) != 2 {
			return types.DockerImage{}, errors.Newf("compose.parseImageString", "invalid image format with digest: %s", imageStr)
		}
		imageStr = parts[0]
		digest = parts[1]
	}

	// Separar tag (:tag) - necesitamos ser cuidadosos con registries que tienen puerto
	// Primero verificar si hay un / en la string - esto nos ayuda a distinguir registry:port/image de image:tag
	if strings.Contains(imageStr, "/") {
		// Hay un slash, buscar el último : después del último /
		lastSlashIndex := strings.LastIndex(imageStr, "/")
		afterSlash := imageStr[lastSlashIndex:]

		if strings.Contains(afterSlash, ":") {
			// Hay un : después del último /, probablemente es un tag
			colonIndex := strings.LastIndex(imageStr, ":")
			tag = imageStr[colonIndex+1:]
			imageStr = imageStr[:colonIndex]
		} else {
			tag = "latest"
		}
	} else {
		// No hay slash, parsing normal
		parts := strings.Split(imageStr, ":")
		switch len(parts) {
		case 1:
			tag = "latest"
		case 2:
			tag = parts[1]
			imageStr = parts[0]
		default:
			// Múltiples :, usar el último como tag
			lastColonIndex := strings.LastIndex(imageStr, ":")
			tag = imageStr[lastColonIndex+1:]
			imageStr = imageStr[:lastColonIndex]
		}
	}

	// Parsear registry y repository
	registry, repository := p.parseRegistryAndRepository(imageStr)

	return types.DockerImage{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
	}, nil
}

// parseRegistryAndRepository separa el registry del repository
func (p *Parser) parseRegistryAndRepository(imageStr string) (string, string) {
	parts := strings.Split(imageStr, "/")

	switch len(parts) {
	case 1:
		// Solo nombre de imagen (ej: "nginx")
		// Asumir Docker Hub con library/
		return "docker.io", "library/" + parts[0]

	case 2:
		// Puede ser:
		// - usuario/imagen en Docker Hub (ej: "user/nginx")
		// - registry/imagen (ej: "localhost:5000/nginx")

		// Si el primer parte contiene un punto, dos puntos, o localhost, es un registry
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost" {
			return parts[0], parts[1]
		}

		// Si no, es usuario/imagen en Docker Hub
		return "docker.io", imageStr

	default:
		// 3 o más partes: registry/namespace/imagen o registry/user/imagen
		// El primer parte es el registry
		registry := parts[0]
		repository := strings.Join(parts[1:], "/")

		return registry, repository
	}
}

// parseEnvFile parsea el contenido de un archivo .env y retorna un mapa de variables
func (p *Parser) parseEnvFile(content string) map[string]string {
	envVars := make(map[string]string)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parsear líneas como KEY=VALUE
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			// Remover comillas si existen
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			envVars[key] = value
		}
	}

	return envVars
}

// expandEnvVars expande variables de entorno en el contenido usando un mapa personalizado
func (p *Parser) expandEnvVars(content string, envVars map[string]string) string {
	// Patrón regex para encontrar variables como ${VAR} o ${VAR:-default}
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extraer el contenido dentro de ${}
		varContent := match[2 : len(match)-1] // Remover ${ y }

		// Verificar si tiene valor por defecto (VAR:-default)
		var parts []string
		if strings.Contains(varContent, ":-") {
			parts = strings.SplitN(varContent, ":-", 2)
		} else {
			parts = []string{varContent}
		}

		varName := parts[0]
		defaultValue := ""
		if len(parts) > 1 {
			defaultValue = parts[1]
		}

		// Buscar en el mapa de variables del .env
		if value, exists := envVars[varName]; exists {
			return value
		}

		// Si no existe en .env, usar valor por defecto o dejar la variable sin expandir
		if defaultValue != "" {
			return defaultValue
		}

		// Si no hay valor por defecto, devolver la variable original
		return match
	})
}

// ComposeFile representa la estructura de un archivo docker-compose
type ComposeFile struct {
	Version  string             `yaml:"version,omitempty"`
	Services map[string]Service `yaml:"services"`
}

// Service representa un servicio en docker-compose
type Service struct {
	Image       string            `yaml:"image,omitempty"`
	Build       interface{}       `yaml:"build,omitempty"` // Puede ser string o objeto
	Environment interface{}       `yaml:"environment,omitempty"`
	Ports       []interface{}     `yaml:"ports,omitempty"`
	Volumes     []interface{}     `yaml:"volumes,omitempty"`
	DependsOn   interface{}       `yaml:"depends_on,omitempty"`
	Networks    interface{}       `yaml:"networks,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}
