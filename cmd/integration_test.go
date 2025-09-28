package cmd_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/internal/registry"
	"github.com/user/docker-image-reporter/internal/scanner"
	"github.com/user/docker-image-reporter/pkg/types"
)

func TestScanWorkflowIntegration(t *testing.T) {
	// Test the complete scanning workflow with mocked components
	// This tests the integration between scanner, config, and report components

	// Create a test config
	cfg := &types.Config{
		Registry: types.RegistryConfig{
			DockerHub: types.DockerHubConfig{
				Enabled: true,
				Timeout: 30,
			},
		},
		Scan: types.ScanConfig{
			Recursive: true,
			Patterns:  []string{"docker-compose.yml"},
			Timeout:   300,
		},
	}

	// Test that we can create scanner service
	scanSvc := createScanServiceForTest(cfg)

	// Test scanning testdata directory
	testDataPath := "../testdata"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := scanSvc.ScanDirectory(ctx, testDataPath, scanner.DefaultConfig())

	// Scan may fail due to network, but should not crash
	if err != nil {
		t.Logf("Scan failed (expected for integration test): %v", err)
		if result == nil {
			t.Error("Expected scan result even on failure")
		}
	} else {
		// If scan succeeded, check basic results
		if len(result.FilesScanned) == 0 {
			t.Error("Expected to scan some files")
		}
		if result.TotalServicesFound == 0 {
			t.Error("Expected to find some services")
		}
		// Check that it found the expected files
		foundCompose := false
		foundProd := false
		for _, file := range result.FilesScanned {
			if strings.Contains(file, "docker-compose.yml") {
				foundCompose = true
			}
			if strings.Contains(file, "docker-compose.prod.yml") {
				foundProd = true
			}
		}
		if !foundCompose {
			t.Error("Expected to find docker-compose.yml")
		}
		if !foundProd {
			t.Error("Expected to find docker-compose.prod.yml")
		}
	}
}

func TestComposeParserIntegration(t *testing.T) {
	// Test that the compose parser can read the test files
	parser := compose.NewParser()
	ctx := context.Background()

	// Test parsing docker-compose.yml
	images1, err := parser.ParseFile(ctx, "../testdata/docker-compose.yml")
	if err != nil {
		t.Fatalf("Failed to parse docker-compose.yml: %v", err)
	}

	if len(images1) == 0 {
		t.Error("Expected to find images in docker-compose.yml")
	}

	// Check for expected services
	expectedServices := []string{"web", "api", "db", "redis", "monitoring"}
	for _, expected := range expectedServices {
		found := false
		for _, image := range images1 {
			if image.ServiceName == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find service %s in docker-compose.yml", expected)
		}
	}

	// Test parsing docker-compose.prod.yml
	images2, err := parser.ParseFile(ctx, "../testdata/docker-compose.prod.yml")
	if err != nil {
		t.Fatalf("Failed to parse docker-compose.prod.yml: %v", err)
	}

	if len(images2) == 0 {
		t.Error("Expected to find images in docker-compose.prod.yml")
	}

	// Check for expected services in prod file
	expectedProdServices := []string{"web", "api", "cache"}
	for _, expected := range expectedProdServices {
		found := false
		for _, image := range images2 {
			if image.ServiceName == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find service %s in docker-compose.prod.yml", expected)
		}
	}
}

// Helper function to create scan service for testing
func createScanServiceForTest(cfg *types.Config) *scanner.Service {
	// This duplicates the logic from cmd/scan.go but for testing
	composeParser := compose.NewParser()

	var registryClients []types.RegistryClient

	// Docker Hub
	if cfg.Registry.DockerHub.Enabled {
		client := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
		registryClients = append(registryClients, client)
	}

	// GHCR
	if cfg.Registry.GHCR.Enabled {
		client := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
		registryClients = append(registryClients, client)
	}

	return scanner.NewService(composeParser, registryClients, slog.Default())
}
