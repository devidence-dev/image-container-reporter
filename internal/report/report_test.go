package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestJSONFormatter_Format(t *testing.T) {
	formatter := JSONFormatter{}

	// Crear un ScanResult de ejemplo
	result := types.ScanResult{
		ProjectName:   "test-project",
		ScanTimestamp: time.Date(2025, 9, 28, 12, 0, 0, 0, time.UTC),
		UpdatesAvailable: []types.ImageUpdate{
			{
				ServiceName:      "web",
				ServiceDirectory: "/app",
				CurrentImage: types.DockerImage{
					Registry:   "docker.io",
					Repository: "nginx",
					Tag:        "1.20",
				},
				LatestImage: types.DockerImage{
					Registry:   "docker.io",
					Repository: "nginx",
					Tag:        "1.21",
				},
				UpdateType: types.UpdateTypeMinor,
				UpdatedAt:  time.Date(2025, 9, 28, 11, 0, 0, 0, time.UTC),
			},
		},
		UpToDateServices:   []string{"db", "cache"},
		Errors:             []string{"Failed to check registry"},
		TotalServicesFound: 3,
		FilesScanned:       []string{"docker-compose.yml"},
	}

	// Formatear el resultado
	output, err := formatter.Format(result)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verificar que es JSON válido
	var parsed types.ScanResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verificar que contiene los datos esperados
	if parsed.ProjectName != result.ProjectName {
		t.Errorf("Expected project name %s, got %s", result.ProjectName, parsed.ProjectName)
	}

	if len(parsed.UpdatesAvailable) != 1 {
		t.Errorf("Expected 1 update, got %d", len(parsed.UpdatesAvailable))
	}

	if parsed.TotalServicesFound != result.TotalServicesFound {
		t.Errorf("Expected %d services, got %d", result.TotalServicesFound, parsed.TotalServicesFound)
	}
}

func TestJSONFormatter_FormatName(t *testing.T) {
	formatter := JSONFormatter{}

	if name := formatter.FormatName(); name != "json" {
		t.Errorf("Expected format name 'json', got '%s'", name)
	}
}

func TestHTMLFormatter_Format(t *testing.T) {
	formatter := HTMLFormatter{}

	// Crear un ScanResult de ejemplo
	result := types.ScanResult{
		ProjectName:   "test-project",
		ScanTimestamp: time.Date(2025, 9, 28, 12, 0, 0, 0, time.UTC),
		UpdatesAvailable: []types.ImageUpdate{
			{
				ServiceName:      "web",
				ServiceDirectory: "/app",
				CurrentImage: types.DockerImage{
					Registry:   "docker.io",
					Repository: "nginx",
					Tag:        "1.20",
				},
				LatestImage: types.DockerImage{
					Registry:   "docker.io",
					Repository: "nginx",
					Tag:        "1.21",
				},
				UpdateType: types.UpdateTypeMinor,
				UpdatedAt:  time.Date(2025, 9, 28, 11, 0, 0, 0, time.UTC),
			},
		},
		UpToDateServices:   []string{"db"},
		Errors:             []string{"Connection timeout"},
		TotalServicesFound: 2,
		FilesScanned:       []string{"docker-compose.yml"},
	}

	// Formatear el resultado
	output, err := formatter.Format(result)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verificar que contiene elementos HTML esperados
	expectedElements := []string{
		"<!DOCTYPE html>",
		"<html>",
		"<head>",
		"<title>Docker Image Report</title>",
		"<style>",
		"<h1>Docker Image Scan Report</h1>",
		"test-project",
		"Available Updates",
		"<table>",
		"<th>Service</th>",
		"web",
		"nginx:1.20",
		"nginx:1.21",
		"minor",
		"Errors",
		"Connection timeout",
		"Files Scanned",
		"docker-compose.yml",
		"</body>",
		"</html>",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected HTML to contain '%s', but it doesn't", element)
		}
	}
}

func TestHTMLFormatter_FormatName(t *testing.T) {
	formatter := HTMLFormatter{}

	if name := formatter.FormatName(); name != "html" {
		t.Errorf("Expected format name 'html', got '%s'", name)
	}
}

func TestHTMLFormatter_Format_NoUpdates(t *testing.T) {
	formatter := HTMLFormatter{}

	// Crear un ScanResult sin updates
	result := types.ScanResult{
		ProjectName:        "test-project",
		ScanTimestamp:      time.Date(2025, 9, 28, 12, 0, 0, 0, time.UTC),
		UpdatesAvailable:   []types.ImageUpdate{},
		UpToDateServices:   []string{"web", "db"},
		Errors:             []string{},
		TotalServicesFound: 2,
		FilesScanned:       []string{"docker-compose.yml"},
	}

	// Formatear el resultado
	output, err := formatter.Format(result)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verificar que no contiene tabla de updates
	if strings.Contains(output, "<table>") {
		t.Error("Expected no table when there are no updates")
	}

	// Verificar que contiene mensaje de éxito
	if !strings.Contains(output, "All 2 services are up to date") {
		t.Error("Expected success message for up-to-date services")
	}
}
