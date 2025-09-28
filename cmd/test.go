package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/user/docker-image-reporter/internal/config"
	"github.com/user/docker-image-reporter/internal/notifier"
	"github.com/user/docker-image-reporter/internal/registry"
	"github.com/user/docker-image-reporter/pkg/types"
)

// newTestCmd crea el comando test
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test connectivity to services",
		Long: `Test connectivity to configured services including Telegram bot,
Docker registries, and other external services.`,
		RunE: runTest,
	}

	cmd.Flags().Bool("telegram", false, "Test Telegram bot connectivity")
	cmd.Flags().Bool("registries", false, "Test registry connectivity")
	cmd.Flags().Bool("all", false, "Test all services")

	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	telegram, _ := cmd.Flags().GetBool("telegram")
	registries, _ := cmd.Flags().GetBool("registries")
	all, _ := cmd.Flags().GetBool("all")

	if all || telegram {
		if err := testTelegram(cmd, cfg); err != nil {
			logger.Error("Telegram test failed", "error", err)
		}
	}

	if all || registries {
		if err := testRegistries(cmd, cfg); err != nil {
			logger.Error("Registry test failed", "error", err)
		}
	}

	if !telegram && !registries && !all {
		cmd.Println("Use --telegram, --registries, or --all flags to specify what to test")
		cmd.Println("\nAvailable test options:")
		cmd.Println("  --telegram    Test Telegram bot connectivity")
		cmd.Println("  --registries  Test registry connectivity")
		cmd.Println("  --all         Test all services")
	}

	return nil
}

func testTelegram(cmd *cobra.Command, cfg *types.Config) error {
	cmd.Println("🔄 Testing Telegram connectivity...")

	if !cfg.Telegram.Enabled {
		cmd.Println("⚠️  Telegram is disabled in configuration")
		return nil
	}

	if cfg.Telegram.BotToken == "" {
		cmd.Println("❌ Telegram bot token is not configured")
		return fmt.Errorf("telegram bot token is required")
	}

	if cfg.Telegram.ChatID == "" {
		cmd.Println("❌ Telegram chat ID is not configured")
		return fmt.Errorf("telegram chat ID is required")
	}

	// Crear cliente de Telegram
	client := notifier.NewTelegramClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

	// Crear un contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Intentar enviar un mensaje de prueba
	testMessage := fmt.Sprintf("🧪 *Docker Image Reporter Test*\n\nTest message sent at %s\n\n✅ Bot connectivity successful!",
		time.Now().Format("2006-01-02 15:04:05"))

	err := client.SendNotification(ctx, testMessage)
	if err != nil {
		cmd.Printf("❌ Telegram test failed: %v\n", err)
		cmd.Println("💡 Make sure your bot token and chat ID are correct")
		cmd.Println("💡 You can get a bot token from @BotFather on Telegram")
		return err
	}

	cmd.Println("✅ Telegram bot connectivity successful")
	cmd.Println("📨 Test message sent to configured chat")
	return nil
}

func testRegistries(cmd *cobra.Command, cfg *types.Config) error {
	cmd.Println("🔄 Testing registry connectivity...")

	var clients []types.RegistryClient
	var clientNames []string

	// Docker Hub
	if cfg.Registry.DockerHub.Enabled {
		client := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
		clients = append(clients, client)
		clientNames = append(clientNames, "Docker Hub")
	}

	// GitHub Container Registry
	if cfg.Registry.GHCR.Enabled {
		client := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
		clients = append(clients, client)
		clientNames = append(clientNames, "GitHub Container Registry")
	}

	if len(clients) == 0 {
		cmd.Println("⚠️  No registries are enabled in configuration")
		return nil
	}

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Probar cada registro
	for i, client := range clients {
		cmd.Printf("🔍 Testing %s...\n", clientNames[i])

		// Intentar obtener tags de una imagen de prueba
		testImage := types.DockerImage{
			Registry:   "docker.io",
			Repository: "alpine",
			Tag:        "latest",
		}

		if clientNames[i] == "GitHub Container Registry" {
			testImage = types.DockerImage{
				Registry:   "ghcr.io",
				Repository: "octocat/hello-world",
				Tag:        "latest",
			}
		}

		tags, err := client.GetLatestTags(ctx, testImage)
		if err != nil {
			cmd.Printf("❌ %s test failed: %v\n", clientNames[i], err)
			if clientNames[i] == "GitHub Container Registry" && cfg.Registry.GHCR.Token == "" {
				cmd.Println("💡 GitHub Container Registry requires a personal access token")
				cmd.Println("💡 Set GITHUB_TOKEN environment variable or use 'config set registry.ghcr.token <token>'")
			}
		} else {
			cmd.Printf("✅ %s connectivity successful\n", clientNames[i])
			cmd.Printf("📦 Found %d tags for test image\n", len(tags))
		}
	}

	return nil
}
