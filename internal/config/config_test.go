package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verificar valores por defecto
	if cfg.Telegram.Enabled {
		t.Error("Expected Telegram to be disabled by default")
	}

	if !cfg.Registry.DockerHub.Enabled {
		t.Error("Expected DockerHub to be enabled by default")
	}

	if !cfg.Registry.GHCR.Enabled {
		t.Error("Expected GHCR to be enabled by default")
	}

	if !cfg.Scan.Recursive {
		t.Error("Expected recursive scan to be enabled by default")
	}

	if len(cfg.Scan.Patterns) == 0 {
		t.Error("Expected default scan patterns")
	}

	expectedPatterns := []string{
		"docker-compose.yml",
		"docker-compose.*.yml",
		"compose.yml",
	}

	if len(cfg.Scan.Patterns) != len(expectedPatterns) {
		t.Errorf("Expected %d patterns, got %d", len(expectedPatterns), len(cfg.Scan.Patterns))
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Guardar variables de entorno originales
	originalVars := map[string]string{
		"TELEGRAM_BOT_TOKEN": os.Getenv("TELEGRAM_BOT_TOKEN"),
		"TELEGRAM_CHAT_ID":   os.Getenv("TELEGRAM_CHAT_ID"),
		"TELEGRAM_ENABLED":   os.Getenv("TELEGRAM_ENABLED"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
	}

	// Limpiar al final del test
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key) //nolint:errcheck
			} else {
				os.Setenv(key, value) //nolint:errcheck,gosec
			}
		}
	}()

	// Configurar variables de entorno de prueba
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-bot-token") //nolint:errcheck,gosec
	os.Setenv("TELEGRAM_CHAT_ID", "test-chat-id")     //nolint:errcheck,gosec
	os.Setenv("TELEGRAM_ENABLED", "true")             //nolint:errcheck,gosec
	os.Setenv("GITHUB_TOKEN", "test-github-token")    //nolint:errcheck,gosec

	cfg := DefaultConfig()
	loadFromEnv(cfg)

	// Verificar que las variables se cargaron correctamente
	if cfg.Telegram.BotToken != "test-bot-token" {
		t.Errorf("Expected bot token 'test-bot-token', got '%s'", cfg.Telegram.BotToken)
	}

	if cfg.Telegram.ChatID != "test-chat-id" {
		t.Errorf("Expected chat ID 'test-chat-id', got '%s'", cfg.Telegram.ChatID)
	}

	if !cfg.Telegram.Enabled {
		t.Error("Expected Telegram to be enabled")
	}

	if cfg.Registry.GHCR.Token != "test-github-token" {
		t.Errorf("Expected GitHub token 'test-github-token', got '%s'", cfg.Registry.GHCR.Token)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *types.Config
		expectErr bool
	}{
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			expectErr: false,
		},
		{
			name: "telegram enabled without bot token",
			config: &types.Config{
				Telegram: types.TelegramConfig{
					Enabled: true,
					ChatID:  "test-chat",
				},
				Registry: types.RegistryConfig{Timeout: 30},
				Scan:     types.ScanConfig{Timeout: 300, Patterns: []string{"*.yml"}},
			},
			expectErr: true,
		},
		{
			name: "telegram enabled without chat ID",
			config: &types.Config{
				Telegram: types.TelegramConfig{
					Enabled:  true,
					BotToken: "test-token",
				},
				Registry: types.RegistryConfig{Timeout: 30},
				Scan:     types.ScanConfig{Timeout: 300, Patterns: []string{"*.yml"}},
			},
			expectErr: true,
		},
		{
			name: "invalid registry timeout",
			config: &types.Config{
				Telegram: types.TelegramConfig{Enabled: false},
				Registry: types.RegistryConfig{Timeout: 0},
				Scan:     types.ScanConfig{Timeout: 300, Patterns: []string{"*.yml"}},
			},
			expectErr: true,
		},
		{
			name: "no scan patterns",
			config: &types.Config{
				Telegram: types.TelegramConfig{Enabled: false},
				Registry: types.RegistryConfig{Timeout: 30},
				Scan:     types.ScanConfig{Timeout: 300, Patterns: []string{}},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.config)
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Crear directorio temporal
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	// Crear configuración de prueba
	originalConfig := &types.Config{
		Telegram: types.TelegramConfig{
			BotToken: "test-token",
			ChatID:   "test-chat",
			Enabled:  true,
		},
		Registry: types.RegistryConfig{
			DockerHub: types.DockerHubConfig{
				Enabled: true,
				Timeout: 45,
			},
			GHCR: types.GHCRConfig{
				Enabled: true,
				Token:   "github-token",
				Timeout: 45,
			},
			Timeout: 45,
		},
		Scan: types.ScanConfig{
			Recursive: false,
			Patterns:  []string{"custom-compose.yml"},
			Timeout:   600,
		},
	}

	// Guardar configuración
	err := Save(originalConfig, configPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Cargar configuración
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verificar que los valores se guardaron y cargaron correctamente
	if loadedConfig.Telegram.BotToken != originalConfig.Telegram.BotToken {
		t.Errorf("Bot token mismatch: expected %s, got %s",
			originalConfig.Telegram.BotToken, loadedConfig.Telegram.BotToken)
	}

	if loadedConfig.Telegram.Enabled != originalConfig.Telegram.Enabled {
		t.Errorf("Telegram enabled mismatch: expected %v, got %v",
			originalConfig.Telegram.Enabled, loadedConfig.Telegram.Enabled)
	}

	if loadedConfig.Registry.Timeout != originalConfig.Registry.Timeout {
		t.Errorf("Registry timeout mismatch: expected %d, got %d",
			originalConfig.Registry.Timeout, loadedConfig.Registry.Timeout)
	}

	if loadedConfig.Scan.Recursive != originalConfig.Scan.Recursive {
		t.Errorf("Scan recursive mismatch: expected %v, got %v",
			originalConfig.Scan.Recursive, loadedConfig.Scan.Recursive)
	}
}
