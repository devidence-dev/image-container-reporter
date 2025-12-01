package report

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/user/docker-image-reporter/pkg/types"
)

//go:embed assets/report_template.html assets/styles.css
var reportAssets embed.FS

// capitalizeFirst capitaliza la primera letra de una cadena
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// HTMLFormatter implementa ReportFormatter para generar reportes en formato HTML
type HTMLFormatter struct{}

// UpdateDistributionItem representa un ítem de distribución de actualizaciones
type UpdateDistributionItem struct {
	Type  string
	Count int
	Color string
}

// UpdateItem representa un ítem de actualización para el template
type UpdateItem struct {
	ServiceName  string
	CurrentImage string
	LatestImage  string
	UpdateType   string
	BadgeClass   string
}

// templateData estructura los datos para el template
type templateData struct {
	Styles             template.HTML
	ProjectName        string
	ScanTimestamp      string
	TotalServices      int
	UpdatesCount       int
	UpToDateCount      int
	HasUpdates         bool
	UpdateDistribution []UpdateDistributionItem
	Updates            []UpdateItem
	Errors             []string
}

// Format convierte un ScanResult en un string HTML formateado
func (f HTMLFormatter) Format(result types.ScanResult) (string, error) {
	// Cargar CSS
	cssBytes, err := reportAssets.ReadFile("assets/styles.css")
	if err != nil {
		return "", fmt.Errorf("error loading styles: %w", err)
	}

	// Cargar template
	tmplBytes, err := reportAssets.ReadFile("assets/report_template.html")
	if err != nil {
		return "", fmt.Errorf("error loading template: %w", err)
	}

	tmpl, err := template.New("report").Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Calcular distribución de actualizaciones
	distribution := make(map[string]int)
	colorMap := map[string]string{
		"patch":   "#3fb950",
		"minor":   "#d29922",
		"major":   "#f85149",
		"unknown": "#8b949e",
	}

	for _, update := range result.UpdatesAvailable {
		updateType := strings.ToLower(update.UpdateType.String())
		distribution[updateType]++
	}

	// Preparar items de distribución para el template
	var distributionItems []UpdateDistributionItem
	for _, updateType := range []string{"patch", "minor", "major", "unknown"} {
		if count, ok := distribution[updateType]; ok && count > 0 {
			distributionItems = append(distributionItems, UpdateDistributionItem{
				Type:  capitalizeFirst(updateType),
				Count: count,
				Color: colorMap[updateType],
			})
		}
	}

	// Preparar items de actualización
	var updateItems []UpdateItem
	for _, update := range result.UpdatesAvailable {
		badgeClass := "badge-unknown"
		switch strings.ToLower(update.UpdateType.String()) {
		case "patch":
			badgeClass = "badge-patch"
		case "minor":
			badgeClass = "badge-minor"
		case "major":
			badgeClass = "badge-major"
		}

		updateItems = append(updateItems, UpdateItem{
			ServiceName:  html.EscapeString(update.ServiceName),
			CurrentImage: html.EscapeString(update.CurrentImage.String()),
			LatestImage:  html.EscapeString(update.LatestImage.String()),
			UpdateType:   html.EscapeString(update.UpdateType.String()),
			BadgeClass:   badgeClass,
		})
	}

	// Preparar datos del template
	// #nosec G203 - cssBytes proviene de archivos embebidos internos, no de entrada del usuario
	data := templateData{
		Styles:             template.HTML(cssBytes),
		ProjectName:        html.EscapeString(result.ProjectName),
		ScanTimestamp:      result.ScanTimestamp.Format("Jan 2, 2006 15:04 MST"),
		TotalServices:      result.TotalServicesFound,
		UpdatesCount:       len(result.UpdatesAvailable),
		UpToDateCount:      len(result.UpToDateServices),
		HasUpdates:         result.HasUpdates(),
		UpdateDistribution: distributionItems,
		Updates:            updateItems,
		Errors:             result.Errors,
	}

	// Renderizar template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

// FormatName devuelve el nombre del formato
func (f HTMLFormatter) FormatName() string {
	return "html"
}
