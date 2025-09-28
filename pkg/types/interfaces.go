package types

import "context"

// RegistryClient define la interfaz para clientes de registros Docker
type RegistryClient interface {
	// GetLatestTags obtiene las etiquetas más recientes de una imagen
	GetLatestTags(ctx context.Context, image DockerImage) ([]string, error)

	// GetImageInfo obtiene información detallada de una imagen
	GetImageInfo(ctx context.Context, image DockerImage) (*ImageInfo, error)

	// Name devuelve el nombre del registro
	Name() string
}

// ComposeParser define la interfaz para parsear archivos docker-compose
type ComposeParser interface {
	// ParseFile parsea un archivo docker-compose y extrae las imágenes
	ParseFile(ctx context.Context, filePath string) ([]DockerImage, error)

	// CanParse determina si el parser puede manejar el archivo dado
	CanParse(filePath string) bool
}

// NotificationClient define la interfaz para clientes de notificación
type NotificationClient interface {
	// SendNotification envía una notificación con el mensaje dado
	SendNotification(ctx context.Context, message string) error

	// Name devuelve el nombre del cliente de notificación
	Name() string
}

// ReportFormatter define la interfaz para formatear reportes
type ReportFormatter interface {
	// Format convierte un ScanResult en un string formateado
	Format(result ScanResult) (string, error)

	// FormatName devuelve el nombre del formato
	FormatName() string
}

// VersionComparator define la interfaz para comparar versiones
type VersionComparator interface {
	// Compare compara dos versiones y devuelve el tipo de actualización
	Compare(current, latest string) UpdateType

	// IsNewer determina si la versión latest es más nueva que current
	IsNewer(current, latest string) bool
}
