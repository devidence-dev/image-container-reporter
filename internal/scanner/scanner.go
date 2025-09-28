package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/pkg/types"
	"github.com/user/docker-image-reporter/pkg/utils"
)

// Service orchestrates the scanning of docker-compose files and checking for updates
type Service struct {
	parser     types.ComposeParser
	registries []types.RegistryClient
	logger     *slog.Logger
}

// Config holds configuration for scanning operations
type Config struct {
	Recursive       bool
	Patterns        []string
	MaxConcurrency  int
	RegistryTimeout time.Duration
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		Recursive:       true,
		Patterns:        []string{"docker-compose.yml", "docker-compose.*.yml", "compose.yml"},
		MaxConcurrency:  10,
		RegistryTimeout: 30 * time.Second,
	}
}

// NewService creates a new scanner service
func NewService(parser types.ComposeParser, registries []types.RegistryClient, logger *slog.Logger) *Service {
	return &Service{
		parser:     parser,
		registries: registries,
		logger:     logger,
	}
}

// ScanDirectory scans a directory for docker-compose files and checks for image updates
func (s *Service) ScanDirectory(ctx context.Context, path string, config Config) (*types.ScanResult, error) {
	s.logger.Info("Starting directory scan", "path", path, "recursive", config.Recursive)

	// Find compose files
	files, err := s.findComposeFiles(path, config)
	if err != nil {
		return nil, fmt.Errorf("finding compose files: %w", err)
	}

	if len(files) == 0 {
		s.logger.Warn("No compose files found", "path", path)
		return &types.ScanResult{
			ProjectName:      filepath.Base(path),
			ScanTimestamp:    time.Now(),
			UpdatesAvailable: []types.ImageUpdate{},
			UpToDateServices: []string{},
			Errors:           []string{"No compose files found"},
		}, nil
	}

	s.logger.Info("Found compose files", "count", len(files), "files", files)

	// Parse all compose files to extract images
	allImages, parseErrors := s.parseComposeFiles(ctx, files)

	// Check for updates concurrently
	updates, upToDate, checkErrors := s.checkForUpdates(ctx, allImages, config)

	// Combine all errors
	var allErrors []string
	allErrors = append(allErrors, parseErrors...)
	allErrors = append(allErrors, checkErrors...)

	result := &types.ScanResult{
		ProjectName:        filepath.Base(path),
		ScanTimestamp:      time.Now(),
		UpdatesAvailable:   updates,
		UpToDateServices:   upToDate,
		Errors:             allErrors,
		TotalServicesFound: len(allImages),
		FilesScanned:       files,
	}

	s.logger.Info("Scan completed",
		"updates_found", len(updates),
		"up_to_date", len(upToDate),
		"errors", len(allErrors))

	return result, nil
}

// findComposeFiles finds all compose files in the given path
func (s *Service) findComposeFiles(path string, config Config) ([]string, error) {
	scanner := compose.NewScanner()
	scanConfig := types.ScanConfig{
		Recursive: config.Recursive,
		Patterns:  config.Patterns,
	}
	return scanner.FindComposeFiles(context.Background(), path, scanConfig)
}

// parseComposeFiles parses all compose files and extracts images
func (s *Service) parseComposeFiles(ctx context.Context, files []string) (map[string]types.DockerImage, []string) {
	allImages := make(map[string]types.DockerImage)
	var errors []string

	for _, file := range files {
		s.logger.Debug("Parsing compose file", "file", file)

		images, err := s.parser.ParseFile(ctx, file)
		if err != nil {
			errMsg := fmt.Sprintf("parsing %s: %v", file, err)
			errors = append(errors, errMsg)
			s.logger.Error("Failed to parse compose file", "file", file, "error", err)
			continue
		}

		// Add images with service context - images already have ServiceName set
		for _, image := range images {
			key := fmt.Sprintf("%s:%s", image.ServiceName, image.String())
			allImages[key] = image
		}

		s.logger.Debug("Parsed compose file", "file", file, "images_found", len(images))
	}

	return allImages, errors
}

// checkForUpdates checks all images for available updates concurrently
func (s *Service) checkForUpdates(ctx context.Context, images map[string]types.DockerImage, config Config) ([]types.ImageUpdate, []string, []string) {
	if len(images) == 0 {
		return nil, nil, nil
	}

	// Create channels for results
	updatesChan := make(chan types.ImageUpdate, len(images))
	upToDateChan := make(chan string, len(images))
	errorsChan := make(chan string, len(images))

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, config.MaxConcurrency)

	var wg sync.WaitGroup

	// Process each image concurrently
	for serviceKey, image := range images {
		wg.Add(1)
		go func(key string, img types.DockerImage) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create context with timeout for this operation
			opCtx, cancel := context.WithTimeout(ctx, config.RegistryTimeout)
			defer cancel()

			s.checkImageForUpdates(opCtx, key, img, updatesChan, upToDateChan, errorsChan)
		}(serviceKey, image)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(updatesChan)
		close(upToDateChan)
		close(errorsChan)
	}()

	// Collect results
	var updates []types.ImageUpdate
	var upToDate []string
	var errors []string

	for updatesChan != nil || upToDateChan != nil || errorsChan != nil {
		select {
		case update, ok := <-updatesChan:
			if !ok {
				updatesChan = nil
			} else {
				updates = append(updates, update)
			}
		case service, ok := <-upToDateChan:
			if !ok {
				upToDateChan = nil
			} else {
				upToDate = append(upToDate, service)
			}
		case err, ok := <-errorsChan:
			if !ok {
				errorsChan = nil
			} else {
				errors = append(errors, err)
			}
		case <-ctx.Done():
			return updates, upToDate, append(errors, "scan cancelled: "+ctx.Err().Error())
		}
	}

	return updates, upToDate, errors
}

// checkImageForUpdates checks a single image for updates
func (s *Service) checkImageForUpdates(ctx context.Context, serviceKey string, image types.DockerImage, updatesChan chan<- types.ImageUpdate, upToDateChan chan<- string, errorsChan chan<- string) {
	serviceName := strings.Split(serviceKey, ":")[0]

	s.logger.Debug("Checking image for updates", "service", serviceName, "image", image.String())

	// Find appropriate registry client
	var client types.RegistryClient
	for _, reg := range s.registries {
		if s.canHandleRegistry(reg, image.Registry) {
			client = reg
			break
		}
	}

	if client == nil {
		errMsg := fmt.Sprintf("no registry client available for %s (registry: %s)", image.String(), image.Registry)
		errorsChan <- errMsg
		s.logger.Warn("No registry client available", "image", image.String(), "registry", image.Registry)
		return
	}

	// Get latest tags from registry
	tags, err := client.GetLatestTags(ctx, image)
	if err != nil {
		errMsg := fmt.Sprintf("getting tags for %s: %v", image.String(), err)
		errorsChan <- errMsg
		s.logger.Error("Failed to get tags", "image", image.String(), "error", err)
		return
	}

	if len(tags) == 0 {
		errMsg := fmt.Sprintf("no tags found for %s", image.String())
		errorsChan <- errMsg
		s.logger.Warn("No tags found", "image", image.String())
		return
	}

	// Filter and sort tags to find the latest stable version
	stableTags := utils.FilterPreReleases(tags)
	if len(stableTags) == 0 {
		s.logger.Debug("No stable tags found, using all tags", "image", image.String())
		stableTags = tags
	}

	sortedTags := utils.SortVersions(stableTags)
	latestTag := sortedTags[0]

	// Compare versions
	updateType := utils.CompareVersions(image.Tag, latestTag)

	if updateType == types.UpdateTypeNone {
		upToDateChan <- serviceName
		s.logger.Debug("Image is up to date", "service", serviceName, "image", image.String())
		return
	}

	// Create update record
	update := types.ImageUpdate{
		ServiceName:  serviceName,
		CurrentImage: image,
		LatestImage: types.DockerImage{
			Registry:   image.Registry,
			Repository: image.Repository,
			Tag:        latestTag,
		},
		UpdateType: updateType,
	}

	updatesChan <- update
	s.logger.Info("Update available",
		"service", serviceName,
		"current", image.Tag,
		"latest", latestTag,
		"type", updateType)
}

// canHandleRegistry checks if a registry client can handle the given registry
func (s *Service) canHandleRegistry(client types.RegistryClient, registry string) bool {
	clientName := strings.ToLower(client.Name())
	registryName := strings.ToLower(registry)

	switch clientName {
	case "docker.io", "dockerhub":
		return registryName == "docker.io" || registryName == ""
	case "ghcr.io", "ghcr":
		return registryName == "ghcr.io"
	default:
		return clientName == registryName
	}
}
