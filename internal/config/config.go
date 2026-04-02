package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
	yaml "gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".icr"
	DefaultConfigFile = "config.yml"
)

// Load carga la configuración desde archivo y variables de entorno
func Load(configPath string) (*types.Config, error) {
	cfg := DefaultConfig()

	// Si no se especifica path, usar el directorio home del usuario
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap("config.Load", err)
		}
		configPath = filepath.Join(homeDir, DefaultConfigDir, DefaultConfigFile)
	}

	// Cargar desde archivo si existe
	if err := loadFromFile(cfg, configPath); err != nil {
		// Si el archivo no existe, no es un error - usar configuración por defecto
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf("config.Load", err, "loading config file %s", configPath)
		}
	}

	// Sobrescribir con variables de entorno
	loadFromEnv(cfg)

	// Validar configuración
	if err := validate(cfg); err != nil {
		return nil, errors.Wrap("config.Load", err)
	}

	return cfg, nil
}

// DefaultConfig devuelve la configuración por defecto
func DefaultConfig() *types.Config {
	return &types.Config{
		Telegram: types.TelegramConfig{
			Enabled:  false,
			Template: defaultTelegramTemplate(),
		},
		Registry: types.RegistryConfig{
			Timeout: 30,
		},
		Scan: types.ScanConfig{
			Recursive: true,
			Patterns: []string{
				"docker-compose.yml",
				"docker-compose.*.yml",
				"compose.yml",
			},
			Timeout: 300, // 5 minutos
		},
	}
}

// loadFromFile carga la configuración desde un archivo YAML
func loadFromFile(cfg *types.Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return errors.Wrapf("config.loadFromFile", err, "parsing YAML file %s", filePath)
	}

	return nil
}

// loadFromEnv carga configuración desde variables de entorno
func loadFromEnv(cfg *types.Config) {
	// Telegram configuration
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Telegram.BotToken = token
	}
	if chatID := os.Getenv("TELEGRAM_CHAT_ID"); chatID != "" {
		cfg.Telegram.ChatID = chatID
	}
	if enabled := os.Getenv("TELEGRAM_ENABLED"); enabled != "" {
		if val, err := strconv.ParseBool(enabled); err == nil {
			cfg.Telegram.Enabled = val
		}
	}

	// GitHub Container Registry token
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cfg.Registry.GHCRToken = token
	}

	// Registry timeout
	if timeout := os.Getenv("REGISTRY_TIMEOUT"); timeout != "" {
		if val, err := strconv.Atoi(timeout); err == nil && val > 0 {
			cfg.Registry.Timeout = val
		}
	}

	// Scan configuration
	if recursive := os.Getenv("SCAN_RECURSIVE"); recursive != "" {
		if val, err := strconv.ParseBool(recursive); err == nil {
			cfg.Scan.Recursive = val
		}
	}
	if patterns := os.Getenv("SCAN_PATTERNS"); patterns != "" {
		cfg.Scan.Patterns = strings.Split(patterns, ",")
		// Trim whitespace from patterns
		for i, pattern := range cfg.Scan.Patterns {
			cfg.Scan.Patterns[i] = strings.TrimSpace(pattern)
		}
	}
	if timeout := os.Getenv("SCAN_TIMEOUT"); timeout != "" {
		if val, err := strconv.Atoi(timeout); err == nil && val > 0 {
			cfg.Scan.Timeout = val
		}
	}
}
func validate(cfg *types.Config) error {
	// Validar configuración de Telegram si está habilitada
	if cfg.Telegram.Enabled {
		if cfg.Telegram.BotToken == "" {
			return errors.New("config.validate", "telegram bot token is required when telegram is enabled")
		}
		if cfg.Telegram.ChatID == "" {
			return errors.New("config.validate", "telegram chat ID is required when telegram is enabled")
		}
	}

	// Validar timeouts
	if cfg.Registry.Timeout <= 0 {
		return errors.New("config.validate", "registry timeout must be positive")
	}
	if cfg.Scan.Timeout <= 0 {
		return errors.New("config.validate", "scan timeout must be positive")
	}

	// Validar patrones de escaneo
	if len(cfg.Scan.Patterns) == 0 {
		return errors.New("config.validate", "at least one scan pattern is required")
	}

	return nil
}

// Save guarda la configuración en un archivo
func Save(cfg *types.Config, configPath string) error {
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errors.Wrap("config.Save", err)
		}
		configDir := filepath.Join(homeDir, DefaultConfigDir)
		if err := os.MkdirAll(configDir, 0750); err != nil {
			return errors.Wrapf("config.Save", err, "creating config directory %s", configDir)
		}
		configPath = filepath.Join(configDir, DefaultConfigFile)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap("config.Save", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return errors.Wrapf("config.Save", err, "writing config file %s", configPath)
	}

	return nil
}

// defaultTelegramTemplate devuelve el template por defecto para mensajes de Telegram
func defaultTelegramTemplate() string {
	return `🐳 <b>Docker Image Updates Available</b>

📊 <b>Summary:</b> {{.Summary}}
📅 <b>Scanned:</b> {{.ScanTimestamp.Format "2006-01-02 15:04:05"}}

{{range .UpdatesAvailable}}
🔄 <b>{{.ServiceName}}</b>
   Current: <code>{{.CurrentImage}}</code>
   Latest: <code>{{.LatestImage}}</code>
   Type: {{.UpdateType}}

{{end}}
{{if .Errors}}
⚠️ <b>Errors:</b>
{{range .Errors}}• {{.}}
{{end}}
{{end}}`
}
