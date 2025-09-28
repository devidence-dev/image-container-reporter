package types

import (
	"fmt"
	"time"
)

// ScanResult representa el resultado completo de un escaneo
type ScanResult struct {
	ProjectName        string        `json:"project_name"`
	ScanTimestamp      time.Time     `json:"scan_timestamp"`
	UpdatesAvailable   []ImageUpdate `json:"updates_available"`
	UpToDateServices   []string      `json:"up_to_date_services"`
	Errors             []string      `json:"errors"`
	TotalServicesFound int           `json:"total_services_found"`
	FilesScanned       []string      `json:"files_scanned"`
}

// HasUpdates indica si hay actualizaciones disponibles
func (r ScanResult) HasUpdates() bool {
	return len(r.UpdatesAvailable) > 0
}

// HasErrors indica si hubo errores durante el escaneo
func (r ScanResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// Summary devuelve un resumen del resultado del escaneo
func (r ScanResult) Summary() string {
	if r.HasUpdates() {
		return fmt.Sprintf("%d updates available, %d services up to date", 
			len(r.UpdatesAvailable), len(r.UpToDateServices))
	}
	return fmt.Sprintf("All %d services are up to date", len(r.UpToDateServices))
}

// ImageInfo contiene información detallada de una imagen desde el registro
type ImageInfo struct {
	Tags         []string  `json:"tags"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size,omitempty"`
	Architecture string    `json:"architecture,omitempty"`
}