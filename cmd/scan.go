package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/internal/config"
	"github.com/user/docker-image-reporter/internal/notifier"
	"github.com/user/docker-image-reporter/internal/registry"
	"github.com/user/docker-image-reporter/internal/report"
	"github.com/user/docker-image-reporter/internal/scanner"
	"github.com/user/docker-image-reporter/pkg/types"
)

// newScanCmd crea el comando scan
func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan docker-compose files for image updates",
		Long: `Scan docker-compose files in the specified path (or current directory)
for Docker image updates. Reports available updates from configured registries.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runScan,
	}

	cmd.Flags().BoolP("notify", "n", false, "Send notifications for found updates")
	cmd.Flags().StringP("output", "o", "console", "Output format (console, json, html)")
	cmd.Flags().String("output-file", "", "Write output to file instead of stdout")

	return cmd
}

func runScan(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	// Determinar el path a escanear
	scanPath := "."
	if len(args) > 0 {
		scanPath = args[0]
	}

	// Verificar que el path existe
	if _, err := os.Stat(scanPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", scanPath)
	}

	// Obtener configuración
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Obtener flags
	notify, _ := cmd.Flags().GetBool("notify")
	outputFormat, _ := cmd.Flags().GetString("output")
	outputFile, _ := cmd.Flags().GetString("output-file")

	logger.Info("Starting scan",
		"path", scanPath,
		"notify", notify,
		"output", outputFormat)

	// Crear servicios
	scanSvc, err := createScanService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create scan service: %w", err)
	}

	reportSvc, err := createReportService()
	if err != nil {
		return fmt.Errorf("failed to create report service: %w", err)
	}

	notifySvc, err := createNotificationService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create notification service: %w", err)
	}

	// Ejecutar el escaneo
	ctx := cmd.Context()
	scanConfig := scanner.DefaultConfig()
	scanResult, err := scanSvc.ScanDirectory(ctx, scanPath, scanConfig)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Convertir a valor para las funciones que esperan ScanResult
	result := *scanResult

	// Mostrar resultados según el formato solicitado
	if err := outputResult(cmd, result, outputFormat, outputFile, reportSvc); err != nil {
		return fmt.Errorf("failed to output result: %w", err)
	}

	// Enviar notificaciones si está habilitado
	if notify && notifySvc.HasClients() {
		if err := notifySvc.NotifyScanResult(ctx, result, getFormatter(outputFormat, reportSvc)); err != nil {
			logger.Error("Failed to send notifications", "error", err)
			// No retornamos error aquí, el scan fue exitoso
		} else {
			logger.Info("Notifications sent successfully")
		}
	} else if notify && !notifySvc.HasClients() {
		logger.Warn("Notification requested but no clients configured")
	}

	logger.Info("Scan completed",
		"files_scanned", len(result.FilesScanned),
		"services_found", result.TotalServicesFound,
		"updates_available", len(result.UpdatesAvailable))

	return nil
}

func createScanService(cfg *types.Config) (*scanner.Service, error) {
	// Crear parser de compose
	composeParser := compose.NewParser()

	// Crear clientes de registro
	var registryClients []types.RegistryClient

	// Docker Hub
	if cfg.Registry.DockerHub.Enabled {
		dockerHubClient := registry.NewDockerHubClient(time.Duration(cfg.Registry.DockerHub.Timeout) * time.Second)
		registryClients = append(registryClients, dockerHubClient)
	}

	// GitHub Container Registry
	if cfg.Registry.GHCR.Enabled {
		ghcrClient := registry.NewGHCRClient(cfg.Registry.GHCR.Token, time.Duration(cfg.Registry.GHCR.Timeout)*time.Second)
		registryClients = append(registryClients, ghcrClient)
	}

	// Crear scanner
	scanSvc := scanner.NewService(composeParser, registryClients, slog.Default())

	return scanSvc, nil
}

func createReportService() (*reportService, error) {
	jsonFormatter := &report.JSONFormatter{}
	htmlFormatter := &report.HTMLFormatter{}

	return &reportService{
		jsonFormatter: jsonFormatter,
		htmlFormatter: htmlFormatter,
	}, nil
}

func createNotificationService(cfg *types.Config) (*notifier.NotificationService, error) {
	notifySvc := notifier.NewNotificationService()

	// Agregar cliente de Telegram si está configurado
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		telegramClient := notifier.NewTelegramClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
		notifySvc.AddClient(telegramClient)
	}

	return notifySvc, nil
}

func outputResult(cmd *cobra.Command, result types.ScanResult, format, outputFile string, reportSvc *reportService) error {
	var formatter types.ReportFormatter
	var ext string

	switch strings.ToLower(format) {
	case "json":
		formatter = reportSvc.jsonFormatter
		ext = ".json"
	case "html":
		formatter = reportSvc.htmlFormatter
		ext = ".html"
	default:
		// Formato console - mostrar resumen
		return outputConsole(cmd, result)
	}

	output, err := formatter.Format(result)
	if err != nil {
		return err
	}

	if outputFile != "" {
		// Asegurar que tenga la extensión correcta
		if !strings.HasSuffix(outputFile, ext) {
			outputFile += ext
		}

		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		cmd.Printf("Results written to %s\n", outputFile)
	} else {
		cmd.Println(output)
	}

	return nil
}

func outputConsole(cmd *cobra.Command, result types.ScanResult) error {
	cmd.Printf("Scan Results for: %s\n", result.ProjectName)
	cmd.Printf("Timestamp: %s\n", result.ScanTimestamp.Format("2006-01-02 15:04:05"))
	cmd.Printf("Files scanned: %d\n", len(result.FilesScanned))
	cmd.Printf("Total services found: %d\n", result.TotalServicesFound)
	cmd.Printf("Services up to date: %d\n", len(result.UpToDateServices))

	if len(result.UpdatesAvailable) > 0 {
		cmd.Printf("\nAvailable Updates (%d):\n", len(result.UpdatesAvailable))
		for _, update := range result.UpdatesAvailable {
			cmd.Printf("  %s (%s -> %s) [%s]\n",
				update.ServiceName,
				update.CurrentImage.Tag,
				update.LatestImage.Tag,
				update.UpdateType)
		}
	}

	if len(result.Errors) > 0 {
		cmd.Printf("\nErrors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			cmd.Printf("  - %s\n", err)
		}
	}

	return nil
}

func getFormatter(format string, reportSvc *reportService) types.ReportFormatter {
	switch strings.ToLower(format) {
	case "html":
		return reportSvc.htmlFormatter
	default:
		return reportSvc.jsonFormatter
	}
}

// reportService es un helper para manejar los formateadores
type reportService struct {
	jsonFormatter *report.JSONFormatter
	htmlFormatter *report.HTMLFormatter
}
