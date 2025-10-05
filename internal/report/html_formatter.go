package report

import (
	"fmt"
	"html"
	"strings"

	"github.com/user/docker-image-reporter/pkg/types"
)

// HTMLFormatter implementa ReportFormatter para generar reportes en formato HTML
type HTMLFormatter struct{}

// Format convierte un ScanResult en un string HTML formateado
func (f HTMLFormatter) Format(result types.ScanResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docker Image Scan Report - Devidence</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css" rel="stylesheet">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.1/font/bootstrap-icons.css">
    <style>
        :root {
            --bg-primary: #0d1117;
            --bg-secondary: #161b22;
            --bg-tertiary: #21262d;
            --border-color: #30363d;
            --text-primary: #c9d1d9;
            --text-secondary: #8b949e;
            --accent-green: #238636;
            --accent-yellow: #d29922;
            --accent-red: #da3633;
            --accent-blue: #58a6ff;
        }

        body {
            background: var(--bg-primary);
            color: var(--text-primary);
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif;
        }

        .navbar-dark {
            background: var(--bg-secondary);
            border-bottom: 1px solid var(--border-color);
            padding: 1rem 0;
        }

        .container-fluid {
            max-width: 1400px;
        }

        .sidebar {
            background: var(--bg-secondary);
            border-right: 1px solid var(--border-color);
            min-height: calc(100vh - 80px);
            padding: 1.5rem;
        }

        .metric-box {
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
            transition: all 0.2s;
        }

        .metric-box:hover {
            border-color: var(--accent-blue);
        }

        .metric-value {
            font-size: 2rem;
            font-weight: 600;
            line-height: 1;
            margin-bottom: 0.5rem;
        }

        .metric-label {
            color: var(--text-secondary);
            font-size: 0.85rem;
        }

        .content-area {
            padding: 1.5rem;
        }

        .status-badge {
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 0.75rem 1rem;
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 1.5rem;
        }

        .status-badge.warning {
            border-color: var(--accent-yellow);
            background: rgba(210, 153, 34, 0.1);
        }

        .status-badge.success {
            border-color: var(--accent-green);
            background: rgba(35, 134, 54, 0.1);
        }

        .table-devops {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            overflow: hidden;
        }

        .table-devops .table {
            color: var(--text-primary);
            background: transparent;
        }

        .table-devops thead {
            background: var(--bg-tertiary);
            border-bottom: 1px solid var(--border-color);
        }

        .table-devops th {
            color: var(--text-secondary);
            font-weight: 600;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            padding: 1rem;
            border: none;
            background: var(--bg-tertiary);
        }

        .table-devops td {
            color: var(--text-primary);
            padding: 1rem;
            border-top: 1px solid var(--border-color);
            vertical-align: middle;
            background: transparent;
        }

        .table-devops tbody tr {
            background: var(--bg-secondary);
        }

        .table-devops tbody tr:hover {
            background: var(--bg-tertiary) !important;
        }

        .service-name {
            font-weight: 600;
            color: var(--accent-blue);
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .image-tag {
            font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Courier New', monospace;
            font-size: 0.85rem;
            background: var(--bg-primary);
            padding: 0.4rem 0.6rem;
            border-radius: 4px;
            color: var(--text-secondary);
            display: inline-block;
            word-break: break-all;
        }

        .badge-type {
            padding: 0.4rem 0.75rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            display: inline-block;
        }

        .badge-patch {
            background: rgba(35, 134, 54, 0.2);
            color: #3fb950;
            border: 1px solid rgba(35, 134, 54, 0.3);
        }

        .badge-minor {
            background: rgba(210, 153, 34, 0.2);
            color: #d29922;
            border: 1px solid rgba(210, 153, 34, 0.3);
        }

        .badge-major {
            background: rgba(218, 54, 51, 0.2);
            color: #f85149;
            border: 1px solid rgba(218, 54, 51, 0.3);
        }

        .badge-unknown {
            background: rgba(139, 148, 158, 0.2);
            color: #8b949e;
            border: 1px solid rgba(139, 148, 158, 0.3);
        }

        .section-header {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 1rem;
            padding-bottom: 0.75rem;
            border-bottom: 1px solid var(--border-color);
        }

        .section-header h5 {
            margin: 0;
            font-size: 1.1rem;
        }

        .theme-toggle {
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            padding: 0.4rem 0.8rem;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }

        .theme-toggle:hover {
            background: var(--bg-primary);
        }

        @media (max-width: 1200px) {
            .image-tag {
                font-size: 0.75rem;
                padding: 0.3rem 0.5rem;
            }
        }
    </style>
</head>
<body>
    <nav class="navbar navbar-dark">
        <div class="container-fluid">
            <span class="navbar-brand mb-0 h1">
                <i class="bi bi-stack me-2"></i>Docker Image Scanner
            </span>
            <button class="theme-toggle">
                <i class="bi bi-moon-fill"></i>
            </button>
        </div>
    </nav>

    <div class="container-fluid">
        <div class="row">
            <div class="col-md-3 sidebar">
                <div class="mb-4">
                    <h6 class="text-uppercase" style="color: var(--text-secondary); font-size: 0.75rem; letter-spacing: 1px;">Project Info</h6>
                    <p class="mb-1"><strong>` + html.EscapeString(result.ProjectName) + `</strong></p>
                    <p class="mb-0" style="color: var(--text-secondary); font-size: 0.85rem;">
                        <i class="bi bi-clock me-1"></i>` + result.ScanTimestamp.Format("Jan 2, 2006 15:04 MST") + `
                    </p>
                </div>

                <div class="metric-box">
                    <div class="metric-value" style="color: var(--accent-blue);">` + fmt.Sprintf("%d", result.TotalServicesFound) + `</div>
                    <div class="metric-label">Total Services</div>
                </div>

                <div class="metric-box">
                    <div class="metric-value" style="color: var(--accent-yellow);">` + fmt.Sprintf("%d", len(result.UpdatesAvailable)) + `</div>
                    <div class="metric-label">Pending Updates</div>
                </div>

                <div class="metric-box">
                    <div class="metric-value" style="color: var(--accent-green);">` + fmt.Sprintf("%d", len(result.UpToDateServices)) + `</div>
                    <div class="metric-label">Up to Date</div>
                </div>`)

	// Calculate update distribution
	patchCount := 0
	minorCount := 0
	majorCount := 0
	unknownCount := 0
	for _, update := range result.UpdatesAvailable {
		switch strings.ToLower(update.UpdateType.String()) {
		case "patch":
			patchCount++
		case "minor":
			minorCount++
		case "major":
			majorCount++
		default:
			unknownCount++
		}
	}

	sb.WriteString(`
                <div class="mt-4 pt-3" style="border-top: 1px solid var(--border-color);">
                    <h6 class="text-uppercase mb-3" style="color: var(--text-secondary); font-size: 0.75rem;">Update Distribution</h6>`)

	if patchCount > 0 {
		sb.WriteString(`
                    <div class="d-flex justify-content-between mb-2">
                        <span style="color: var(--text-secondary); font-size: 0.85rem;">Patch</span>
                        <span style="color: #3fb950;">` + fmt.Sprintf("%d", patchCount) + `</span>
                    </div>`)
	}
	if minorCount > 0 {
		sb.WriteString(`
                    <div class="d-flex justify-content-between mb-2">
                        <span style="color: var(--text-secondary); font-size: 0.85rem;">Minor</span>
                        <span style="color: #d29922;">` + fmt.Sprintf("%d", minorCount) + `</span>
                    </div>`)
	}
	if majorCount > 0 {
		sb.WriteString(`
                    <div class="d-flex justify-content-between mb-2">
                        <span style="color: var(--text-secondary); font-size: 0.85rem;">Major</span>
                        <span style="color: #f85149;">` + fmt.Sprintf("%d", majorCount) + `</span>
                    </div>`)
	}
	if unknownCount > 0 {
		sb.WriteString(`
                    <div class="d-flex justify-content-between">
                        <span style="color: var(--text-secondary); font-size: 0.85rem;">Unknown</span>
                        <span style="color: var(--text-secondary);">` + fmt.Sprintf("%d", unknownCount) + `</span>
                    </div>`)
	}

	sb.WriteString(`
                </div>
            </div>

            <div class="col-md-9 content-area">`)

	// Status badge
	if result.HasUpdates() {
		sb.WriteString(`
                <div class="status-badge warning">
                    <i class="bi bi-exclamation-triangle-fill" style="color: var(--accent-yellow);"></i>
                    <span><strong>` + fmt.Sprintf("%d", len(result.UpdatesAvailable)) + ` updates available</strong> across your services</span>
                </div>`)
	} else {
		sb.WriteString(`
                <div class="status-badge success">
                    <i class="bi bi-check-circle-fill" style="color: var(--accent-green);"></i>
                    <span><strong>All services are up to date</strong></span>
                </div>`)
	}

	// Updates table
	if len(result.UpdatesAvailable) > 0 {
		sb.WriteString(`
                <div class="section-header">
                    <i class="bi bi-arrow-repeat" style="color: var(--accent-blue);"></i>
                    <h5>Available Updates</h5>
                </div>

                <div class="table-devops">
                    <table class="table mb-0">
                        <thead>
                            <tr>
                                <th style="width: 15%;">Service</th>
                                <th style="width: 30%;">Current Image</th>
                                <th style="width: 30%;">Latest Image</th>
                                <th style="width: 10%;">Type</th>
                            </tr>
                        </thead>
                        <tbody>`)

		for _, update := range result.UpdatesAvailable {
			updateTypeClass := "badge-unknown"
			switch strings.ToLower(update.UpdateType.String()) {
			case "patch":
				updateTypeClass = "badge-patch"
			case "minor":
				updateTypeClass = "badge-minor"
			case "major":
				updateTypeClass = "badge-major"
			}

			sb.WriteString(`
                            <tr>
                                <td>
                                    <div class="service-name">
                                        <i class="bi bi-box"></i>
                                        ` + html.EscapeString(update.ServiceName) + `
                                    </div>
                                </td>
                                <td>
                                    <code class="image-tag">` + html.EscapeString(update.CurrentImage.String()) + `</code>
                                </td>
                                <td>
                                    <code class="image-tag">` + html.EscapeString(update.LatestImage.String()) + `</code>
                                </td>
                                <td>
                                    <span class="badge-type ` + updateTypeClass + `">` + html.EscapeString(update.UpdateType.String()) + `</span>
                                </td>
                            </tr>`)
		}

		sb.WriteString(`
                        </tbody>
                    </table>
                </div>`)
	}

	// Errors
	if len(result.Errors) > 0 {
		sb.WriteString(`
                <div class="section-header mt-4">
                    <i class="bi bi-exclamation-triangle" style="color: var(--accent-red);"></i>
                    <h5>Errors</h5>
                </div>
                <div style="background: var(--bg-secondary); border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem;">
                    <ul style="margin-bottom: 0; color: var(--accent-red);">`)
		for _, err := range result.Errors {
			sb.WriteString("<li>" + html.EscapeString(err) + "</li>")
		}
		sb.WriteString(`
                    </ul>
                </div>`)
	}

	sb.WriteString(`
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>`)

	return sb.String(), nil
}

// FormatName devuelve el nombre del formato
func (f HTMLFormatter) FormatName() string {
	return "html"
}
