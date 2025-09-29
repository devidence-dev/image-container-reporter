package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"

	dockerTypes "github.com/user/docker-image-reporter/pkg/types"
)

// Client wraps Docker daemon client functionality
type Client struct {
	client *client.Client
	logger *slog.Logger
}

// NewClient creates a new Docker daemon client
func NewClient(logger *slog.Logger) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	return &Client{
		client: cli,
		logger: logger,
	}, nil
}

// Close closes the Docker client connection
func (d *Client) Close() error {
	return d.client.Close()
}

// ScanRunningContainers scans all running containers and extracts their images
func (d *Client) ScanRunningContainers(ctx context.Context) ([]dockerTypes.DockerImage, error) {
	d.logger.Info("Scanning running containers via Docker daemon")

	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	if len(containers) == 0 {
		d.logger.Warn("No running containers found")
		return []dockerTypes.DockerImage{}, nil
	}

	d.logger.Info("Found running containers", "count", len(containers))

	var images []dockerTypes.DockerImage
	for _, cont := range containers {
		image, err := d.extractImageFromContainer(ctx, cont)
		if err != nil {
			d.logger.Error("Failed to extract image from container",
				"container_id", cont.ID[:12],
				"container_name", d.getContainerName(cont),
				"error", err)
			continue
		}

		images = append(images, image)
	}

	d.logger.Info("Extracted images from running containers", "count", len(images))
	return images, nil
}

// extractImageFromContainer extracts Docker image information from a container
func (d *Client) extractImageFromContainer(ctx context.Context, cont container.Summary) (dockerTypes.DockerImage, error) {
	// Get detailed container information
	inspect, err := d.client.ContainerInspect(ctx, cont.ID)
	if err != nil {
		return dockerTypes.DockerImage{}, fmt.Errorf("inspecting container %s: %w", cont.ID[:12], err)
	}

	// Parse the image string
	imageStr := inspect.Config.Image
	image, err := d.parseImageString(imageStr)
	if err != nil {
		return dockerTypes.DockerImage{}, fmt.Errorf("parsing image string %s: %w", imageStr, err)
	}

	// Extract service name from labels or container name
	serviceName := d.extractServiceName(cont, inspect.Config.Labels)
	image.ServiceName = serviceName

	// Add container context
	image.ContainerID = cont.ID[:12]
	image.ContainerName = d.getContainerName(cont)

	d.logger.Debug("Extracted image from container",
		"container", image.ContainerName,
		"service", serviceName,
		"image", image.String())

	return image, nil
}

// extractServiceName extracts service name from container labels or name
func (d *Client) extractServiceName(cont container.Summary, labels map[string]string) string {
	// Try compose service label first
	if serviceName, ok := labels["com.docker.compose.service"]; ok {
		return serviceName
	}

	// Try project + service labels
	if project, ok := labels["com.docker.compose.project"]; ok {
		if service, ok := labels["com.docker.compose.service"]; ok {
			return fmt.Sprintf("%s_%s", project, service)
		}
	}

	// Fall back to container name (remove leading slash and suffix)
	name := d.getContainerName(cont)
	// Remove common suffixes like _1, _2, etc.
	if idx := strings.LastIndex(name, "_"); idx > 0 {
		if suffix := name[idx+1:]; len(suffix) <= 2 {
			// Check if suffix is numeric
			if _, err := fmt.Sscanf(suffix, "%d", new(int)); err == nil {
				name = name[:idx]
			}
		}
	}

	return name
}

// getContainerName returns the first container name without leading slash
func (d *Client) getContainerName(cont container.Summary) string {
	if len(cont.Names) > 0 {
		return strings.TrimPrefix(cont.Names[0], "/")
	}
	return cont.ID[:12]
}

// parseImageString parses Docker image string into components
func (d *Client) parseImageString(imageStr string) (dockerTypes.DockerImage, error) {
	if imageStr == "" {
		return dockerTypes.DockerImage{}, fmt.Errorf("empty image string")
	}

	// Remove whitespace
	imageStr = strings.TrimSpace(imageStr)

	// Handle digest (@sha256:...)
	var tag, digest string
	if strings.Contains(imageStr, "@") {
		parts := strings.Split(imageStr, "@")
		if len(parts) != 2 {
			return dockerTypes.DockerImage{}, fmt.Errorf("invalid image format with digest: %s", imageStr)
		}
		imageStr = parts[0]
		digest = parts[1]
	}

	// Handle tag (:tag)
	if strings.Contains(imageStr, "/") {
		// Has slash, look for : after last /
		lastSlashIndex := strings.LastIndex(imageStr, "/")
		afterSlash := imageStr[lastSlashIndex:]

		if strings.Contains(afterSlash, ":") {
			colonIndex := strings.LastIndex(imageStr, ":")
			tag = imageStr[colonIndex+1:]
			imageStr = imageStr[:colonIndex]
		} else {
			tag = "latest"
		}
	} else {
		// No slash, normal parsing
		parts := strings.Split(imageStr, ":")
		switch len(parts) {
		case 1:
			tag = "latest"
		case 2:
			tag = parts[1]
			imageStr = parts[0]
		default:
			// Multiple :, use last as tag
			lastColonIndex := strings.LastIndex(imageStr, ":")
			tag = imageStr[lastColonIndex+1:]
			imageStr = imageStr[:lastColonIndex]
		}
	}

	// Parse registry and repository
	registry, repository := d.parseRegistryAndRepository(imageStr)

	return dockerTypes.DockerImage{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
	}, nil
}

// parseRegistryAndRepository separates registry from repository
func (d *Client) parseRegistryAndRepository(imageStr string) (string, string) {
	parts := strings.Split(imageStr, "/")

	switch len(parts) {
	case 1:
		// Just image name (e.g., "nginx")
		// Assume Docker Hub with library/
		return "docker.io", "library/" + parts[0]

	case 2:
		// Can be:
		// - user/image on Docker Hub (e.g., "user/nginx")
		// - registry/image (e.g., "localhost:5000/nginx")

		// If first part contains dot, colon, or is localhost, it's a registry
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost" {
			return parts[0], parts[1]
		}

		// Otherwise, it's user/image on Docker Hub
		return "docker.io", imageStr

	default:
		// 3+ parts: registry/namespace/image or registry/user/image
		registry := parts[0]
		repository := strings.Join(parts[1:], "/")
		return registry, repository
	}
}

// Ping tests connection to Docker daemon
func (d *Client) Ping(ctx context.Context) error {
	_, err := d.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pinging docker daemon: %w", err)
	}
	return nil
}

// GetDockerInfo returns Docker daemon information
func (d *Client) GetDockerInfo(ctx context.Context) (*system.Info, error) {
	info, err := d.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting docker info: %w", err)
	}
	return &info, nil
}
