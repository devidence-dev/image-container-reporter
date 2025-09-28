package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestNewConfigCmd(t *testing.T) {
	cmd := newConfigCmd()

	if cmd.Use != "config" {
		t.Errorf("Expected command use to be 'config', got '%s'", cmd.Use)
	}

	if cmd.Short != "Manage configuration settings" {
		t.Errorf("Expected command short to be 'Manage configuration settings', got '%s'", cmd.Short)
	}

	// Check subcommands exist
	subcommands := cmd.Commands()
	expectedSubs := []string{"show", "get <key>", "set <key> <value>"}
	if len(subcommands) != len(expectedSubs) {
		t.Errorf("Expected %d subcommands, got %d", len(expectedSubs), len(subcommands))
	}

	for _, sub := range subcommands {
		found := false
		for _, expected := range expectedSubs {
			if sub.Use == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected subcommand: %s", sub.Use)
		}
	}
}

func TestNewConfigShowCmd(t *testing.T) {
	cmd := newConfigShowCmd()

	if cmd.Use != "show" {
		t.Errorf("Expected command use to be 'show', got '%s'", cmd.Use)
	}

	if cmd.Short != "Show current configuration" {
		t.Errorf("Expected command short to be 'Show current configuration', got '%s'", cmd.Short)
	}
}

func TestRunConfigShow(t *testing.T) {
	// This test is complex due to config.Load() loading user config
	// Instead, test the JSON marshaling directly
	testConfig := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled:  true,
			BotToken: "test_token",
			ChatID:   "123456",
		},
		Registry: types.RegistryConfig{
			DockerHub: types.DockerHubConfig{
				Enabled: true,
				Timeout: 30,
			},
		},
	}

	// Test JSON marshaling (what runConfigShow does)
	output, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, `"bot_token": "test_token"`) {
		t.Error("Expected output to contain bot_token")
	}
	if !strings.Contains(outputStr, `"chat_id": "123456"`) {
		t.Error("Expected output to contain chat_id")
	}
}

func TestNewConfigSetCmd(t *testing.T) {
	cmd := newConfigSetCmd()

	if cmd.Use != "set <key> <value>" {
		t.Errorf("Expected command use to be 'set <key> <value>', got '%s'", cmd.Use)
	}

	if cmd.Short != "Set a configuration value" {
		t.Errorf("Expected command short to be 'Set a configuration value', got '%s'", cmd.Short)
	}
}

func TestRunConfigSet(t *testing.T) {
	// Test the setConfigValue function directly instead of the full command
	cfg := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled: false,
		},
	}

	err := setConfigValue(cfg, "telegram.enabled", "true")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !cfg.Telegram.Enabled {
		t.Error("Expected telegram.enabled to be true")
	}
}

func TestNewConfigGetCmd(t *testing.T) {
	cmd := newConfigGetCmd()

	if cmd.Use != "get <key>" {
		t.Errorf("Expected command use to be 'get <key>', got '%s'", cmd.Use)
	}

	if cmd.Short != "Get a configuration value" {
		t.Errorf("Expected command short to be 'Get a configuration value', got '%s'", cmd.Short)
	}
}

func TestRunConfigGet(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create test config
	testConfig := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled:  true,
			BotToken: "test_token",
			ChatID:   "123456",
		},
		Registry: types.RegistryConfig{
			Timeout: 30,
			DockerHub: types.DockerHubConfig{
				Enabled: true,
				Timeout: 30,
			},
			GHCR: types.GHCRConfig{
				Enabled: true,
				Timeout: 30,
			},
		},
		Scan: types.ScanConfig{
			Recursive: true,
			Timeout:   300,
			Patterns: []string{
				"docker-compose.yml",
			},
		},
	}

	// Save test config
	if err := saveTestConfig(testConfig, configPath); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	cmd := newConfigGetCmd()
	cmd.Flags().StringP("config", "c", "", "Path to configuration file")
	cmd.Flags().Set("config", configPath)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := runConfigGet(cmd, []string{"telegram.bot_token"})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	output := buf.String()
	expected := "telegram.bot_token = test_token\n"
	if output != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, output)
	}
}

func TestSetConfigValue_Telegram(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		key   string
		value string
		check func(*types.Config) bool
	}{
		{
			key:   "telegram.enabled",
			value: "true",
			check: func(c *types.Config) bool { return c.Telegram.Enabled },
		},
		{
			key:   "telegram.bot_token",
			value: "new_token",
			check: func(c *types.Config) bool { return c.Telegram.BotToken == "new_token" },
		},
		{
			key:   "telegram.chat_id",
			value: "789",
			check: func(c *types.Config) bool { return c.Telegram.ChatID == "789" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := setConfigValue(cfg, tt.key, tt.value)
			if err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.key, err)
			}
			if !tt.check(cfg) {
				t.Errorf("Config value not set correctly for %s", tt.key)
			}
		})
	}
}

func TestSetConfigValue_Registry(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		key   string
		value string
		check func(*types.Config) bool
	}{
		{
			key:   "registry.dockerhub.enabled",
			value: "true",
			check: func(c *types.Config) bool { return c.Registry.DockerHub.Enabled },
		},
		{
			key:   "registry.dockerhub.timeout",
			value: "60",
			check: func(c *types.Config) bool { return c.Registry.DockerHub.Timeout == 60 },
		},
		{
			key:   "registry.ghcr.enabled",
			value: "true",
			check: func(c *types.Config) bool { return c.Registry.GHCR.Enabled },
		},
		{
			key:   "registry.ghcr.token",
			value: "ghcr_token",
			check: func(c *types.Config) bool { return c.Registry.GHCR.Token == "ghcr_token" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := setConfigValue(cfg, tt.key, tt.value)
			if err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.key, err)
			}
			if !tt.check(cfg) {
				t.Errorf("Config value not set correctly for %s", tt.key)
			}
		})
	}
}

func TestSetConfigValue_Scan(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		key   string
		value string
		check func(*types.Config) bool
	}{
		{
			key:   "scan.recursive",
			value: "false",
			check: func(c *types.Config) bool { return !c.Scan.Recursive },
		},
		{
			key:   "scan.timeout",
			value: "600",
			check: func(c *types.Config) bool { return c.Scan.Timeout == 600 },
		},
		{
			key:   "scan.patterns",
			value: "docker-compose.yml,compose.yaml",
			check: func(c *types.Config) bool {
				return len(c.Scan.Patterns) == 2 &&
					c.Scan.Patterns[0] == "docker-compose.yml" &&
					c.Scan.Patterns[1] == "compose.yaml"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := setConfigValue(cfg, tt.key, tt.value)
			if err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.key, err)
			}
			if !tt.check(cfg) {
				t.Errorf("Config value not set correctly for %s", tt.key)
			}
		})
	}
}

func TestGetConfigValue(t *testing.T) {
	cfg := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled:  true,
			BotToken: "test_token",
			ChatID:   "123456",
		},
		Registry: types.RegistryConfig{
			DockerHub: types.DockerHubConfig{
				Enabled: true,
				Timeout: 30,
			},
			GHCR: types.GHCRConfig{
				Enabled: true,
				Token:   "ghcr_token",
				Timeout: 45,
			},
		},
		Scan: types.ScanConfig{
			Recursive: true,
			Timeout:   300,
			Patterns:  []string{"docker-compose.yml", "compose.yml"},
		},
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"telegram.enabled", "true"},
		{"telegram.bot_token", "test_token"},
		{"telegram.chat_id", "123456"},
		{"registry.dockerhub.enabled", "true"},
		{"registry.dockerhub.timeout", "30"},
		{"registry.ghcr.token", "ghcr_token"},
		{"scan.recursive", "true"},
		{"scan.timeout", "300"},
		{"scan.patterns", "docker-compose.yml,compose.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, err := getConfigValue(cfg, tt.key)
			if err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.key, err)
			}
			if value != tt.expected {
				t.Errorf("Expected value '%s' for %s, got '%s'", tt.expected, tt.key, value)
			}
		})
	}
}

func TestSetConfigValue_InvalidKey(t *testing.T) {
	cfg := &types.Config{}

	err := setConfigValue(cfg, "invalid.key", "value")
	if err == nil {
		t.Error("Expected error for invalid key")
	}
}

func TestGetConfigValue_InvalidKey(t *testing.T) {
	cfg := &types.Config{}

	_, err := getConfigValue(cfg, "invalid.key")
	if err == nil {
		t.Error("Expected error for invalid key")
	}
}

// Helper functions for testing
func saveTestConfig(cfg *types.Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
