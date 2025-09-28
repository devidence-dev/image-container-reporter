package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd crea el comando raíz de la aplicación
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-image-reporter",
		Short: "Docker Image Reporter - Scan and report Docker image updates",
		Long: `Docker Image Reporter is a tool to scan docker-compose files and report
available updates for Docker images from various registries.

It supports Docker Hub, GitHub Container Registry, and can send notifications
via Telegram when updates are found.`,
		Version: "0.1.0",
	}

	// Agregar subcomandos
	cmd.AddCommand(newScanCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newTestCmd())

	// Flags globales
	cmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	return cmd
}
