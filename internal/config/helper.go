package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// GetConfigPath devuelve la ruta del archivo de configuración
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap("config.GetConfigPath", err)
	}
	return filepath.Join(homeDir, DefaultConfigDir, DefaultConfigFile), nil
}

// EnsureConfigDir crea el directorio de configuración si no existe
func EnsureConfigDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap("config.EnsureConfigDir", err)
	}

	configDir := filepath.Join(homeDir, DefaultConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return errors.Wrapf("config.EnsureConfigDir", err, "creating directory %s", configDir)
	}

	return nil
}

// SetValue establece un valor en la configuración usando notación de puntos
func SetValue(cfg *types.Config, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return errors.Newf("config.SetValue", "invalid key format: %s", key)
	}

	switch parts[0] {
	case "telegram":
		return setTelegramValue(cfg, parts[1:], value)
	case "registry":
		return setRegistryValue(cfg, parts[1:], value)
	case "scan":
		return setScanValue(cfg, parts[1:], value)
	default:
		return errors.Newf("config.SetValue", "unknown config section: %s", parts[0])
	}
}

// GetValue obtiene un valor de la configuración usando notación de puntos
func GetValue(cfg *types.Config, key string) (string, error) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return "", errors.Newf("config.GetValue", "invalid key format: %s", key)
	}

	switch parts[0] {
	case "telegram":
		return getTelegramValue(cfg, parts[1:])
	case "registry":
		return getRegistryValue(cfg, parts[1:])
	case "scan":
		return getScanValue(cfg, parts[1:])
	default:
		return "", errors.Newf("config.GetValue", "unknown config section: %s", parts[0])
	}
}

func setTelegramValue(cfg *types.Config, parts []string, value string) error {
	if len(parts) == 0 {
		return errors.New("config.setTelegramValue", "missing telegram field")
	}

	switch parts[0] {
	case "bot_token":
		cfg.Telegram.BotToken = value
	case "chat_id":
		cfg.Telegram.ChatID = value
	case "enabled":
		enabled := strings.ToLower(value) == "true"
		cfg.Telegram.Enabled = enabled
	case "template":
		cfg.Telegram.Template = value
	default:
		return errors.Newf("config.setTelegramValue", "unknown telegram field: %s", parts[0])
	}

	return nil
}

func getTelegramValue(cfg *types.Config, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("config.getTelegramValue", "missing telegram field")
	}

	switch parts[0] {
	case "bot_token":
		return cfg.Telegram.BotToken, nil
	case "chat_id":
		return cfg.Telegram.ChatID, nil
	case "enabled":
		return fmt.Sprintf("%t", cfg.Telegram.Enabled), nil
	case "template":
		return cfg.Telegram.Template, nil
	default:
		return "", errors.Newf("config.getTelegramValue", "unknown telegram field: %s", parts[0])
	}
}

func setRegistryValue(cfg *types.Config, parts []string, value string) error {
	if len(parts) == 0 {
		return errors.New("config.setRegistryValue", "missing registry field")
	}

	switch parts[0] {
	case "timeout":
		// Parse timeout value
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return errors.Wrapf("config.setRegistryValue", err, "invalid timeout value: %s", value)
		}
		cfg.Registry.Timeout = timeout
	case "dockerhub":
		if len(parts) < 2 {
			return errors.New("config.setRegistryValue", "missing dockerhub field")
		}
		switch parts[1] {
		case "enabled":
			enabled := strings.ToLower(value) == "true"
			cfg.Registry.DockerHub.Enabled = enabled
		default:
			return errors.Newf("config.setRegistryValue", "unknown dockerhub field: %s", parts[1])
		}
	case "ghcr":
		if len(parts) < 2 {
			return errors.New("config.setRegistryValue", "missing ghcr field")
		}
		switch parts[1] {
		case "enabled":
			enabled := strings.ToLower(value) == "true"
			cfg.Registry.GHCR.Enabled = enabled
		case "token":
			cfg.Registry.GHCR.Token = value
		default:
			return errors.Newf("config.setRegistryValue", "unknown ghcr field: %s", parts[1])
		}
	default:
		return errors.Newf("config.setRegistryValue", "unknown registry field: %s", parts[0])
	}

	return nil
}

func getRegistryValue(cfg *types.Config, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("config.getRegistryValue", "missing registry field")
	}

	switch parts[0] {
	case "timeout":
		return fmt.Sprintf("%d", cfg.Registry.Timeout), nil
	case "dockerhub":
		if len(parts) < 2 {
			return "", errors.New("config.getRegistryValue", "missing dockerhub field")
		}
		switch parts[1] {
		case "enabled":
			return fmt.Sprintf("%t", cfg.Registry.DockerHub.Enabled), nil
		default:
			return "", errors.Newf("config.getRegistryValue", "unknown dockerhub field: %s", parts[1])
		}
	case "ghcr":
		if len(parts) < 2 {
			return "", errors.New("config.getRegistryValue", "missing ghcr field")
		}
		switch parts[1] {
		case "enabled":
			return fmt.Sprintf("%t", cfg.Registry.GHCR.Enabled), nil
		case "token":
			// No mostrar el token completo por seguridad
			if cfg.Registry.GHCR.Token == "" {
				return "", nil
			}
			return "[REDACTED]", nil
		default:
			return "", errors.Newf("config.getRegistryValue", "unknown ghcr field: %s", parts[1])
		}
	default:
		return "", errors.Newf("config.getRegistryValue", "unknown registry field: %s", parts[0])
	}
}

func setScanValue(cfg *types.Config, parts []string, value string) error {
	if len(parts) == 0 {
		return errors.New("config.setScanValue", "missing scan field")
	}

	switch parts[0] {
	case "recursive":
		enabled := strings.ToLower(value) == "true"
		cfg.Scan.Recursive = enabled
	case "patterns":
		// Split comma-separated patterns
		patterns := strings.Split(value, ",")
		for i, pattern := range patterns {
			patterns[i] = strings.TrimSpace(pattern)
		}
		cfg.Scan.Patterns = patterns
	case "timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return errors.Wrapf("config.setScanValue", err, "invalid timeout value: %s", value)
		}
		cfg.Scan.Timeout = timeout
	default:
		return errors.Newf("config.setScanValue", "unknown scan field: %s", parts[0])
	}

	return nil
}

func getScanValue(cfg *types.Config, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("config.getScanValue", "missing scan field")
	}

	switch parts[0] {
	case "recursive":
		return fmt.Sprintf("%t", cfg.Scan.Recursive), nil
	case "patterns":
		return strings.Join(cfg.Scan.Patterns, ", "), nil
	case "timeout":
		return fmt.Sprintf("%d", cfg.Scan.Timeout), nil
	default:
		return "", errors.Newf("config.getScanValue", "unknown scan field: %s", parts[0])
	}
}