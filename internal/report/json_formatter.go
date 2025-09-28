package report

import (
	"encoding/json"

	"github.com/user/docker-image-reporter/pkg/types"
)

// JSONFormatter implementa ReportFormatter para generar reportes en formato JSON
type JSONFormatter struct{}

// Format convierte un ScanResult en un string JSON formateado
func (f JSONFormatter) Format(result types.ScanResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatName devuelve el nombre del formato
func (f JSONFormatter) FormatName() string {
	return "json"
}
