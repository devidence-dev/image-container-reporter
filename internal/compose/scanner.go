package compose

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// Scanner maneja el escaneo de directorios en busca de archivos docker-compose
type Scanner struct {
	parser *Parser
}

// NewScanner crea una nueva instancia del scanner
func NewScanner() *Scanner {
	return &Scanner{
		parser: NewParser(),
	}
}

// ScanDirectory escanea un directorio en busca de archivos docker-compose
func (s *Scanner) ScanDirectory(ctx context.Context, rootPath string, config types.ScanConfig) ([]types.DockerImage, []string, error) {
	var allImages []types.DockerImage
	var scannedFiles []string

	err := s.walkDirectory(ctx, rootPath, config, func(filePath string) error {
		// Verificar si el contexto fue cancelado
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Verificar si el parser puede manejar este archivo
		if !s.parser.CanParse(filePath) {
			return nil
		}

		// Verificar si coincide con los patrones configurados
		if !s.matchesPatterns(filePath, config.Patterns) {
			return nil
		}

		// Parsear el archivo
		images, err := s.parser.ParseFile(ctx, filePath)
		if err != nil {
			// Log error but continue with other files
			return errors.Wrapf("compose.ScanDirectory", err, "parsing file %s", filePath)
		}

		allImages = append(allImages, images...)
		scannedFiles = append(scannedFiles, filePath)

		return nil
	})

	if err != nil {
		return nil, nil, errors.Wrap("compose.ScanDirectory", err)
	}

	return allImages, scannedFiles, nil
}

// FindComposeFiles encuentra todos los archivos docker-compose en un directorio
func (s *Scanner) FindComposeFiles(ctx context.Context, rootPath string, config types.ScanConfig) ([]string, error) {
	var files []string

	err := s.walkDirectory(ctx, rootPath, config, func(filePath string) error {
		// Verificar si el contexto fue cancelado
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Verificar si el parser puede manejar este archivo
		if !s.parser.CanParse(filePath) {
			return nil
		}

		// Verificar si coincide con los patrones configurados
		if !s.matchesPatterns(filePath, config.Patterns) {
			return nil
		}

		files = append(files, filePath)
		return nil
	})

	if err != nil {
		return nil, errors.Wrap("compose.FindComposeFiles", err)
	}

	return files, nil
}

// walkDirectory camina por el directorio aplicando la función a cada archivo
func (s *Scanner) walkDirectory(ctx context.Context, rootPath string, config types.ScanConfig, fn func(string) error) error {
	if config.Recursive {
		return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Skip directories that can't be accessed
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip directories
			if d.IsDir() {
				// Skip hidden directories and common ignore patterns
				if s.shouldSkipDirectory(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			return fn(path)
		})
	} else {
		// Solo escanear el directorio raíz
		entries, err := filepath.Glob(filepath.Join(rootPath, "*"))
		if err != nil {
			return errors.Wrapf("compose.walkDirectory", err, "globbing directory %s", rootPath)
		}

		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Verificar si es un archivo
			info, err := filepath.Abs(entry)
			if err != nil {
				continue
			}

			stat, err := filepath.Glob(info)
			if err != nil || len(stat) == 0 {
				continue
			}

			if err := fn(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

// matchesPatterns verifica si un archivo coincide con los patrones configurados
func (s *Scanner) matchesPatterns(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // Si no hay patrones, aceptar todos
	}

	fileName := filepath.Base(filePath)

	for _, pattern := range patterns {
		// Soporte para patrones glob simples
		matched, err := filepath.Match(pattern, fileName)
		if err != nil {
			// Si el patrón es inválido, hacer comparación exacta
			if fileName == pattern {
				return true
			}
			continue
		}

		if matched {
			return true
		}
	}

	return false
}

// shouldSkipDirectory determina si un directorio debe ser omitido
func (s *Scanner) shouldSkipDirectory(dirName string) bool {
	skipDirs := []string{
		".git",
		".svn",
		".hg",
		"node_modules",
		".vscode",
		".idea",
		"__pycache__",
		".pytest_cache",
		"vendor",
		"target",
		"build",
		"dist",
		".next",
		".nuxt",
		"coverage",
		".coverage",
		"tmp",
		"temp",
		".tmp",
		".temp",
	}

	// Skip hidden directories
	if strings.HasPrefix(dirName, ".") {
		for _, skip := range skipDirs {
			if dirName == skip {
				return true
			}
		}
	}

	// Skip common build/cache directories
	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}

	return false
}

// GetImagesByService agrupa las imágenes por nombre de servicio
func (s *Scanner) GetImagesByService(images []types.DockerImage) map[string][]types.DockerImage {
	serviceMap := make(map[string][]types.DockerImage)

	for _, image := range images {
		serviceName := image.ServiceName
		if serviceName == "" {
			serviceName = "unknown"
		}

		serviceMap[serviceName] = append(serviceMap[serviceName], image)
	}

	return serviceMap
}

// GetImagesByRegistry agrupa las imágenes por registro
func (s *Scanner) GetImagesByRegistry(images []types.DockerImage) map[string][]types.DockerImage {
	registryMap := make(map[string][]types.DockerImage)

	for _, image := range images {
		registry := image.Registry
		if registry == "" {
			registry = "unknown"
		}

		registryMap[registry] = append(registryMap[registry], image)
	}

	return registryMap
}
