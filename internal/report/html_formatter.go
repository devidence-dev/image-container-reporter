package report

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/user/docker-image-reporter/pkg/types"
)

// HTMLFormatter implementa ReportFormatter para generar reportes en formato HTML
type HTMLFormatter struct{}

// Format convierte un ScanResult en un string HTML formateado
func (f HTMLFormatter) Format(result types.ScanResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Docker Image Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .summary { background-color: #e7f3ff; padding: 10px; border-radius: 5px; margin-bottom: 20px; }
        .error { color: red; }
        .success { color: green; }
    </style>
</head>
<body>`)

	sb.WriteString("<h1>Docker Image Scan Report</h1>")
	sb.WriteString(fmt.Sprintf("<p><strong>Project:</strong> %s</p>", html.EscapeString(result.ProjectName)))
	sb.WriteString(fmt.Sprintf("<p><strong>Scan Time:</strong> %s</p>", result.ScanTimestamp.Format(time.RFC3339)))

	// Summary
	sb.WriteString(`<div class="summary">`)
	if result.HasUpdates() {
		sb.WriteString(fmt.Sprintf(`<p class="error">%d updates available, %d services up to date</p>`, len(result.UpdatesAvailable), len(result.UpToDateServices)))
	} else {
		sb.WriteString(fmt.Sprintf(`<p class="success">All %d services are up to date</p>`, len(result.UpToDateServices)))
	}
	sb.WriteString(fmt.Sprintf("<p><strong>Total services found:</strong> %d</p>", result.TotalServicesFound))
	sb.WriteString("</div>")

	// Updates table
	if len(result.UpdatesAvailable) > 0 {
		sb.WriteString("<h2>Available Updates</h2>")
		sb.WriteString(`<table>
        <tr>
            <th>Service</th>
            <th>Directory</th>
            <th>Current Image</th>
            <th>Latest Image</th>
            <th>Update Type</th>
        </tr>`)

		for _, update := range result.UpdatesAvailable {
			sb.WriteString("<tr>")
			sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(update.ServiceName)))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(update.ServiceDirectory)))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(update.CurrentImage.String())))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(update.LatestImage.String())))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(update.UpdateType.String())))
			sb.WriteString("</tr>")
		}
		sb.WriteString("</table>")
	}

	// Errors
	if len(result.Errors) > 0 {
		sb.WriteString("<h2>Errors</h2><ul>")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("<li class=\"error\">%s</li>", html.EscapeString(err)))
		}
		sb.WriteString("</ul>")
	}

	// Files scanned
	if len(result.FilesScanned) > 0 {
		sb.WriteString("<h2>Files Scanned</h2><ul>")
		for _, file := range result.FilesScanned {
			sb.WriteString(fmt.Sprintf("<li>%s</li>", html.EscapeString(file)))
		}
		sb.WriteString("</ul>")
	}

	sb.WriteString("</body></html>")

	return sb.String(), nil
}

// FormatName devuelve el nombre del formato
func (f HTMLFormatter) FormatName() string {
	return "html"
}
