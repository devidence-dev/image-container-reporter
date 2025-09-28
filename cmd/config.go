package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/user/docker-image-reporter/internal/config"
	"github.com/user/docker-image-reporter/pkg/types"
)

// newConfigCmd crea el comando config
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
		Long:  `Manage configuration settings for registries, notifications, and scanning options.`,
	}

	// Subcomandos
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())

	return cmd
}

// newConfigShowCmd crea el subcomando config show
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  `Display the current configuration settings.`,
		RunE:  runConfigShow,
	}
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Mostrar configuración en formato JSON
	output, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format configuration: %w", err)
	}

	cmd.Println(string(output))
	return nil
}

// newConfigSetCmd crea el subcomando config set
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  `Set a configuration value. Use 'config show' to see available keys.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	configPath, _ := cmd.Flags().GetString("config")

	// Cargar configuración existente
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Actualizar el valor según la clave
	if err := setConfigValue(cfg, key, value); err != nil {
		return fmt.Errorf("failed to set configuration value: %w", err)
	}

	// Guardar configuración
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	cmd.Printf("Configuration updated: %s = %s\n", key, value)
	return nil
}

// newConfigGetCmd crea el subcomando config get
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long:  `Get the value of a configuration key.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Obtener el valor según la clave
	value, err := getConfigValue(cfg, key)
	if err != nil {
		return fmt.Errorf("failed to get configuration value: %w", err)
	}

	cmd.Printf("%s = %s\n", key, value)
	return nil
}

// setConfigValue establece un valor en la configuración según la clave
func setConfigValue(cfg *types.Config, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid key format, expected 'section.key'")
	}

	section := strings.ToLower(parts[0])
	subkey := strings.ToLower(parts[1])

	switch section {
	case "telegram":
		return setTelegramConfig(cfg, subkey, value)
	case "registry":
		return setRegistryConfig(cfg, parts[1:], value)
	case "scan":
		return setScanConfig(cfg, subkey, value)
	default:
		return fmt.Errorf("unknown configuration section: %s", section)
	}
}

// getConfigValue obtiene un valor de la configuración según la clave
func getConfigValue(cfg *types.Config, key string) (string, error) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid key format, expected 'section.key'")
	}

	section := strings.ToLower(parts[0])
	subkey := strings.ToLower(parts[1])

	switch section {
	case "telegram":
		return getTelegramConfig(cfg, subkey)
	case "registry":
		return getRegistryConfig(cfg, parts[1:])
	case "scan":
		return getScanConfig(cfg, subkey)
	default:
		return "", fmt.Errorf("unknown configuration section: %s", section)
	}
}

// Funciones auxiliares para Telegram
func setTelegramConfig(cfg *types.Config, key, value string) error {
	switch key {
	case "enabled":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		cfg.Telegram.Enabled = val
	case "bot_token":
		cfg.Telegram.BotToken = value
	case "chat_id":
		cfg.Telegram.ChatID = value
	case "template":
		cfg.Telegram.Template = value
	default:
		return fmt.Errorf("unknown telegram key: %s", key)
	}
	return nil
}

func getTelegramConfig(cfg *types.Config, key string) (string, error) {
	switch key {
	case "enabled":
		return strconv.FormatBool(cfg.Telegram.Enabled), nil
	case "bot_token":
		return cfg.Telegram.BotToken, nil
	case "chat_id":
		return cfg.Telegram.ChatID, nil
	case "template":
		return cfg.Telegram.Template, nil
	default:
		return "", fmt.Errorf("unknown telegram key: %s", key)
	}
}

// Funciones auxiliares para Registry
func setRegistryConfig(cfg *types.Config, keys []string, value string) error {
	if len(keys) < 2 {
		return fmt.Errorf("registry key must be in format 'registry.subkey' or 'registry.provider.key'")
	}

	provider := strings.ToLower(keys[0])
	subkey := strings.ToLower(keys[1])

	switch provider {
	case "dockerhub":
		return setDockerHubConfig(cfg, subkey, value)
	case "ghcr":
		return setGHCRConfig(cfg, subkey, value)
	case "timeout":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid timeout value: %s", value)
		}
		cfg.Registry.Timeout = val
		cfg.Registry.DockerHub.Timeout = val
		cfg.Registry.GHCR.Timeout = val
	default:
		return fmt.Errorf("unknown registry provider: %s", provider)
	}
	return nil
}

func getRegistryConfig(cfg *types.Config, keys []string) (string, error) {
	if len(keys) < 2 {
		return "", fmt.Errorf("registry key must be in format 'registry.subkey' or 'registry.provider.key'")
	}

	provider := strings.ToLower(keys[0])
	subkey := strings.ToLower(keys[1])

	switch provider {
	case "dockerhub":
		return getDockerHubConfig(cfg, subkey)
	case "ghcr":
		return getGHCRConfig(cfg, subkey)
	case "timeout":
		return strconv.Itoa(cfg.Registry.Timeout), nil
	default:
		return "", fmt.Errorf("unknown registry provider: %s", provider)
	}
}

func setDockerHubConfig(cfg *types.Config, key, value string) error {
	switch key {
	case "enabled":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		cfg.Registry.DockerHub.Enabled = val
	case "timeout":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid timeout value: %s", value)
		}
		cfg.Registry.DockerHub.Timeout = val
	default:
		return fmt.Errorf("unknown dockerhub key: %s", key)
	}
	return nil
}

func getDockerHubConfig(cfg *types.Config, key string) (string, error) {
	switch key {
	case "enabled":
		return strconv.FormatBool(cfg.Registry.DockerHub.Enabled), nil
	case "timeout":
		return strconv.Itoa(cfg.Registry.DockerHub.Timeout), nil
	default:
		return "", fmt.Errorf("unknown dockerhub key: %s", key)
	}
}

func setGHCRConfig(cfg *types.Config, key, value string) error {
	switch key {
	case "enabled":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		cfg.Registry.GHCR.Enabled = val
	case "token":
		cfg.Registry.GHCR.Token = value
	case "timeout":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid timeout value: %s", value)
		}
		cfg.Registry.GHCR.Timeout = val
	default:
		return fmt.Errorf("unknown ghcr key: %s", key)
	}
	return nil
}

func getGHCRConfig(cfg *types.Config, key string) (string, error) {
	switch key {
	case "enabled":
		return strconv.FormatBool(cfg.Registry.GHCR.Enabled), nil
	case "token":
		return cfg.Registry.GHCR.Token, nil
	case "timeout":
		return strconv.Itoa(cfg.Registry.GHCR.Timeout), nil
	default:
		return "", fmt.Errorf("unknown ghcr key: %s", key)
	}
}

// Funciones auxiliares para Scan
func setScanConfig(cfg *types.Config, key, value string) error {
	switch key {
	case "recursive":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		cfg.Scan.Recursive = val
	case "timeout":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid timeout value: %s", value)
		}
		cfg.Scan.Timeout = val
	case "patterns":
		cfg.Scan.Patterns = strings.Split(value, ",")
		// Trim whitespace
		for i, pattern := range cfg.Scan.Patterns {
			cfg.Scan.Patterns[i] = strings.TrimSpace(pattern)
		}
	default:
		return fmt.Errorf("unknown scan key: %s", key)
	}
	return nil
}

func getScanConfig(cfg *types.Config, key string) (string, error) {
	switch key {
	case "recursive":
		return strconv.FormatBool(cfg.Scan.Recursive), nil
	case "timeout":
		return strconv.Itoa(cfg.Scan.Timeout), nil
	case "patterns":
		return strings.Join(cfg.Scan.Patterns, ","), nil
	default:
		return "", fmt.Errorf("unknown scan key: %s", key)
	}
}
