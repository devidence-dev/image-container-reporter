package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// Configuration keys and values constants
const (
	// Configuration section keys
	keyTelegram  = "telegram"
	keyRegistry  = "registry"
	keyScan      = "scan"
	keyEnabled   = "enabled"
	keyTimeout   = "timeout"
	keyBotToken  = "bot_token"
	keyChatID    = "chat_id"
	keyTemplate  = "template"
	keyDockerHub = "dockerhub"
	keyGHCR      = "ghcr"
	keyToken     = "token"
	keyRecursive = "recursive"
	keyPatterns  = "patterns"

	// Configuration values
	valueTrue = "true"
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
	if err := os.MkdirAll(configDir, 0750); err != nil {
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
	case keyTelegram:
		return setTelegramValue(cfg, parts[1:], value)
	case keyRegistry:
		return setRegistryValue(cfg, parts[1:], value)
	case keyScan:
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
	case keyTelegram:
		return getTelegramValue(cfg, parts[1:])
	case keyRegistry:
		return getRegistryValue(cfg, parts[1:])
	case keyScan:
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
	case keyBotToken:
		cfg.Telegram.BotToken = value
	case keyChatID:
		cfg.Telegram.ChatID = value
	case keyEnabled:
		enabled := strings.ToLower(value) == valueTrue
		cfg.Telegram.Enabled = enabled
	case keyTemplate:
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
	case keyBotToken:
		return cfg.Telegram.BotToken, nil
	case keyChatID:
		return cfg.Telegram.ChatID, nil
	case keyEnabled:
		return fmt.Sprintf("%t", cfg.Telegram.Enabled), nil
	case keyTemplate:
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
	case keyTimeout:
		// Parse timeout value
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return errors.Wrapf("config.setRegistryValue", err, "invalid timeout value: %s", value)
		}
		cfg.Registry.Timeout = timeout
	case keyDockerHub:
		if len(parts) < 2 {
			return errors.New("config.setRegistryValue", "missing dockerhub field")
		}
		switch parts[1] {
		case keyEnabled:
			enabled := strings.ToLower(value) == valueTrue
			cfg.Registry.DockerHub.Enabled = enabled
		default:
			return errors.Newf("config.setRegistryValue", "unknown dockerhub field: %s", parts[1])
		}
	case keyGHCR:
		if len(parts) < 2 {
			return errors.New("config.setRegistryValue", "missing ghcr field")
		}
		switch parts[1] {
		case keyEnabled:
			enabled := strings.ToLower(value) == valueTrue
			cfg.Registry.GHCR.Enabled = enabled
		case keyToken:
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
	case keyTimeout:
		return fmt.Sprintf("%d", cfg.Registry.Timeout), nil
	case keyDockerHub:
		if len(parts) < 2 {
			return "", errors.New("config.getRegistryValue", "missing dockerhub field")
		}
		switch parts[1] {
		case keyEnabled:
			return fmt.Sprintf("%t", cfg.Registry.DockerHub.Enabled), nil
		default:
			return "", errors.Newf("config.getRegistryValue", "unknown dockerhub field: %s", parts[1])
		}
	case keyGHCR:
		if len(parts) < 2 {
			return "", errors.New("config.getRegistryValue", "missing ghcr field")
		}
		switch parts[1] {
		case keyEnabled:
			return fmt.Sprintf("%t", cfg.Registry.GHCR.Enabled), nil
		case keyToken:
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
	case keyRecursive:
		enabled := strings.ToLower(value) == valueTrue
		cfg.Scan.Recursive = enabled
	case keyPatterns:
		// Split comma-separated patterns
		patterns := strings.Split(value, ",")
		for i, pattern := range patterns {
			patterns[i] = strings.TrimSpace(pattern)
		}
		cfg.Scan.Patterns = patterns
	case keyTimeout:
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
	case keyRecursive:
		return fmt.Sprintf("%t", cfg.Scan.Recursive), nil
	case keyPatterns:
		return strings.Join(cfg.Scan.Patterns, ", "), nil
	case keyTimeout:
		return fmt.Sprintf("%d", cfg.Scan.Timeout), nil
	default:
		return "", errors.Newf("config.getScanValue", "unknown scan field: %s", parts[0])
	}
}
