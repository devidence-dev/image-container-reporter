package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/user/docker-image-reporter/internal/compose"
	"github.com/user/docker-image-reporter/internal/config"
	"github.com/user/docker-image-reporter/internal/docker"
	"github.com/user/docker-image-reporter/internal/notifier"
	"github.com/user/docker-image-reporter/internal/registry"
	"github.com/user/docker-image-reporter/internal/report"
	"github.com/user/docker-image-reporter/internal/scanner"
	"github.com/user/docker-image-reporter/pkg/types"
	"github.com/user/docker-image-reporter/pkg/utils"
)

// Output format constants
const (
	formatHTML = "html"
	formatJSON = "json"
)

// newScanCmd crea el comando scan
func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan docker-compose files or running containers for image updates",
		Long: `Scan docker-compose files in the specified path (or current directory)
or running Docker containers for image updates. Reports available updates from configured registries.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runScan,
	}

	cmd.Flags().BoolP("notify", "n", false, "Send notifications for found updates")
	cmd.Flags().StringP("output", "o", "console", "Output format (console, json, html)")
	cmd.Flags().String("output-file", "", "Write output to file instead of stdout")
	cmd.Flags().Bool("docker-daemon", false, "Scan running containers via Docker daemon instead of compose files")
	cmd.Flags().Bool("fail-on-updates", false, "Exit with non-zero code if updates are found")

	return cmd
}

func runScan(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

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
	useDockerDaemon, _ := cmd.Flags().GetBool("docker-daemon")
	failOnUpdates, _ := cmd.Flags().GetBool("fail-on-updates")

	ctx := cmd.Context()

	var result types.ScanResult

	if useDockerDaemon {
		logger.Info("Starting Docker daemon scan")

		// Crear cliente Docker
		dockerClient, err := docker.NewClient(logger)
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Probar conexión
		if err := dockerClient.Ping(ctx); err != nil {
			return fmt.Errorf("failed to connect to Docker daemon: %w", err)
		}

		// Escanear contenedores en ejecución
		result, err = scanDockerDaemon(ctx, dockerClient, cfg, logger)
		if err != nil {
			return fmt.Errorf("Docker daemon scan failed: %w", err)
		}
	} else {
		logger.Info("Starting compose files scan")

		// Determinar el path a escanear
		scanPath := "."
		if len(args) > 0 {
			scanPath = args[0]
		}

		// Verificar que el path existe
		if _, err := os.Stat(scanPath); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", scanPath)
		}

		logger.Info("Starting scan", "path", scanPath)

		// Crear servicios
		scanSvc := createScanService(cfg)

		// Ejecutar el escaneo
		scanConfig := scanner.DefaultConfig()
		scanResultPtr, err := scanSvc.ScanDirectory(ctx, scanPath, scanConfig)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		result = *scanResultPtr
	}

	// Crear servicios comunes
	reportSvc := createReportService()
	notifySvc := createNotificationService(cfg)

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

	// Fallar si hay actualizaciones y se solicitó
	if failOnUpdates && len(result.UpdatesAvailable) > 0 {
		return fmt.Errorf("found %d image updates", len(result.UpdatesAvailable))
	}

	return nil
}

func createScanService(cfg *types.Config) *scanner.Service {
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

	return scanSvc
}

func createReportService() *reportService {
	jsonFormatter := &report.JSONFormatter{}
	htmlFormatter := &report.HTMLFormatter{}

	return &reportService{
		jsonFormatter: jsonFormatter,
		htmlFormatter: htmlFormatter,
	}
}

func createNotificationService(cfg *types.Config) *notifier.NotificationService {
	notifySvc := notifier.NewNotificationService()

	// Agregar cliente de Telegram si está configurado
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		telegramClient := notifier.NewTelegramClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
		notifySvc.AddClient(telegramClient)
	}

	return notifySvc
}

func outputResult(cmd *cobra.Command, result types.ScanResult, format, outputFile string, reportSvc *reportService) error {
	var formatter types.ReportFormatter
	var ext string

	switch strings.ToLower(format) {
	case formatJSON:
		formatter = reportSvc.jsonFormatter
		ext = ".json"
	case formatHTML:
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

		if err := os.WriteFile(outputFile, []byte(output), 0600); err != nil {
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
	case formatHTML:
		return reportSvc.htmlFormatter
	default:
		return reportSvc.jsonFormatter
	}
}

// scanDockerDaemon executes a scan using Docker daemon to inspect running containers
func scanDockerDaemon(ctx context.Context, dockerClient *docker.Client, cfg *types.Config, logger *slog.Logger) (types.ScanResult, error) {
	// Obtener imágenes de contenedores en ejecución
	images, err := dockerClient.ScanRunningContainers(ctx)
	if err != nil {
		return types.ScanResult{}, fmt.Errorf("scanning running containers: %w", err)
	}

	if len(images) == 0 {
		logger.Warn("No running containers found")
		return types.ScanResult{
			ProjectName:      "docker-daemon",
			ScanTimestamp:    time.Now(),
			UpdatesAvailable: []types.ImageUpdate{},
			UpToDateServices: []string{},
			Errors:           []string{"No running containers found"},
		}, nil
	}

	// Crear servicios para verificar actualizaciones
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

	// Verificar actualizaciones para cada imagen
	var updates []types.ImageUpdate
	var upToDate []string
	var scanErrors []string

	for _, image := range images {
		logger.Debug("Checking image for updates", "service", image.ServiceName, "image", image.String())

		// Buscar cliente de registro apropiado
		var client types.RegistryClient
		for _, reg := range registryClients {
			if canHandleRegistryForImage(reg, image.Registry) {
				client = reg
				break
			}
		}

		if client == nil {
			errMsg := fmt.Sprintf("no registry client available for %s (registry: %s)", image.String(), image.Registry)
			scanErrors = append(scanErrors, errMsg)
			logger.Warn("No registry client available", "image", image.String(), "registry", image.Registry)
			continue
		}

		// Obtener tags más recientes del registro
		tags, err := client.GetLatestTags(ctx, image)
		if err != nil {
			errMsg := fmt.Sprintf("getting tags for %s: %v", image.String(), err)
			scanErrors = append(scanErrors, errMsg)
			logger.Error("Failed to get tags", "image", image.String(), "error", err)
			continue
		}

		if len(tags) == 0 {
			errMsg := fmt.Sprintf("no tags found for %s", image.String())
			scanErrors = append(scanErrors, errMsg)
			logger.Warn("No tags found", "image", image.String())
			continue
		}

		// Filtrar y ordenar tags para encontrar la versión estable más reciente
		stableTags := utils.FilterPreReleases(tags)
		if len(stableTags) == 0 {
			logger.Debug("No stable tags found, using all tags", "image", image.String())
			stableTags = tags
		}

		sortedTags := utils.SortVersions(stableTags)
		latestTag := sortedTags[0]

		// Comparar versiones
		updateType := utils.CompareVersions(image.Tag, latestTag)

		if updateType == types.UpdateTypeNone {
			upToDate = append(upToDate, image.ServiceName)
			logger.Debug("Image is up to date", "service", image.ServiceName, "image", image.String())
			continue
		}

		// Crear registro de actualización
		update := types.ImageUpdate{
			ServiceName:  image.ServiceName,
			CurrentImage: image,
			LatestImage: types.DockerImage{
				Registry:   image.Registry,
				Repository: image.Repository,
				Tag:        latestTag,
			},
			UpdateType: updateType,
		}

		updates = append(updates, update)
		logger.Info("Update available",
			"service", image.ServiceName,
			"current", image.Tag,
			"latest", latestTag,
			"type", updateType)
	}

	result := types.ScanResult{
		ProjectName:        "docker-daemon",
		ScanTimestamp:      time.Now(),
		UpdatesAvailable:   updates,
		UpToDateServices:   upToDate,
		Errors:             scanErrors,
		TotalServicesFound: len(images),
		FilesScanned:       []string{}, // No files scanned in daemon mode
	}

	return result, nil
}

// canHandleRegistryForImage checks if a registry client can handle the given registry
func canHandleRegistryForImage(client types.RegistryClient, registry string) bool {
	clientName := strings.ToLower(client.Name())
	registryName := strings.ToLower(registry)

	switch clientName {
	case "docker.io", "dockerhub":
		return registryName == "docker.io" || registryName == ""
	case "ghcr.io", "ghcr":
		return registryName == "ghcr.io"
	default:
		return clientName == registryName
	}
}

// reportService es un helper para manejar los formateadores
type reportService struct {
	jsonFormatter *report.JSONFormatter
	htmlFormatter *report.HTMLFormatter
}
