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
			return fmt.Errorf("docker daemon scan failed: %w", err)
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
	logger.Info("Notification check", "notify_flag", notify, "has_clients", notifySvc.HasClients(), "has_updates", result.HasUpdates(), "has_errors", result.HasErrors())
	if notify && notifySvc.HasClients() {
		// Para notificaciones, generar HTML y enviarlo como archivo adjunto
		htmlFormatter := reportSvc.htmlFormatter
		htmlContent, err := htmlFormatter.Format(result)
		if err != nil {
			logger.Error("Failed to format HTML report", "error", err)
		} else {
			// Crear archivo temporal
			tempFile, err := os.CreateTemp("", "docker-report-*.html")
			if err != nil {
				logger.Error("Failed to create temp file", "error", err)
			} else {
				defer os.Remove(tempFile.Name()) // Limpiar archivo temporal

				// Escribir contenido HTML
				if _, err := tempFile.WriteString(htmlContent); err != nil {
					logger.Error("Failed to write HTML to temp file", "error", err)
				} else {
					tempFile.Close()

					// Enviar archivo como adjunto
					caption := fmt.Sprintf("🐳 <b>Docker Image Updates Report</b>\n\n📊 <b>Summary:</b> %s\n📅 <b>Scanned:</b> %s",
						result.Summary(),
						result.ScanTimestamp.Format("2006-01-02 15:04:05"))

					if err := notifySvc.SendFile(ctx, tempFile.Name(), "docker-updates-report.html", caption); err != nil {
						logger.Error("Failed to send HTML report", "error", err)
					} else {
						logger.Info("HTML report sent successfully")
					}
				}
			}
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

	genericClient := registry.NewGenericRegistryClient(time.Duration(cfg.Registry.Timeout)*time.Second, cfg.Registry.GHCRToken)

	// Crear scanner
	scanSvc := scanner.NewService(composeParser, []types.RegistryClient{genericClient}, slog.Default())

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
	logger := slog.Default()
	logger.Info("Telegram config check", "enabled", cfg.Telegram.Enabled, "bot_token_set", cfg.Telegram.BotToken != "", "chat_id_set", cfg.Telegram.ChatID != "")
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		telegramClient := notifier.NewTelegramClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
		notifySvc.AddClient(telegramClient)
		logger.Info("Telegram client added to notification service")
	} else {
		logger.Warn("Telegram client not added due to missing configuration")
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

// scanDockerDaemon executes a scan using Docker daemon to inspect running containers
func scanDockerDaemon(ctx context.Context, dockerClient *docker.Client, cfg *types.Config, logger *slog.Logger) (types.ScanResult, error) {
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

	// Filter out images built locally that are not available in any public registry.
	var scannable []types.DockerImage
	for _, img := range images {
		if isLocalImage(img) {
			logger.Info("Skipping local image", "service", img.ServiceName, "image", img.String())
		} else {
			scannable = append(scannable, img)
		}
	}

	result, err := createScanService(cfg).ScanImages(ctx, scannable, "docker-daemon")
	if err != nil {
		return types.ScanResult{}, err
	}
	result.TotalServicesFound = len(images) // include skipped locals in total
	return *result, nil
}

// isLocalImage checks if an image appears to be built locally and not available in public registries
func isLocalImage(image types.DockerImage) bool {
	// Extract the actual image name from repository (remove library/ prefix if present)
	imageName := strings.TrimPrefix(image.Repository, "library/")

	// Known local image patterns (specific images that are definitely local builds)
	knownLocalImages := []string{
		"github-runner-github-runner",
		"gaganode-gaganode",
		"devidence-home-app",
		"automation-hub-automation-hub",
	}

	// Check exact matches for known local images
	for _, localImg := range knownLocalImages {
		if imageName == localImg {
			return true
		}
	}

	// Pattern-based detection
	// Images with repetitive names (name-name-name pattern)
	parts := strings.Split(imageName, "-")
	if len(parts) >= 2 {
		// Check if parts repeat (like github-runner-github-runner)
		firstPart := parts[0]
		for i := 1; i < len(parts); i++ {
			if parts[i] == firstPart {
				return true // Repetitive pattern detected
			}
		}
	}

	// Check for Docker Compose naming patterns
	if strings.Contains(imageName, "-") && strings.Contains(imageName, "_") {
		return true
	}

	// Check for common local image patterns
	if strings.Contains(imageName, "local") ||
		strings.Contains(imageName, "dev") ||
		strings.Contains(imageName, "build") ||
		strings.Contains(imageName, "custom") {
		return true
	}

	// Check if it's a hash-like name (built from commit hash)
	if len(imageName) >= 8 && len(imageName) <= 12 {
		// Check if it's mostly hexadecimal characters
		hexCount := 0
		for _, char := range imageName {
			if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
				hexCount++
			}
		}
		if float64(hexCount)/float64(len(imageName)) > 0.8 {
			return true // Likely a hash
		}
	}

	return false
}

// reportService es un helper para manejar los formateadores
type reportService struct {
	jsonFormatter *report.JSONFormatter
	htmlFormatter *report.HTMLFormatter
}
