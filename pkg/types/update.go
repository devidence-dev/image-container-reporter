package types

import "time"

// UpdateType representa el tipo de actualización disponible
type UpdateType string

const (
	UpdateTypeMajor   UpdateType = "major"
	UpdateTypeMinor   UpdateType = "minor"
	UpdateTypePatch   UpdateType = "patch"
	UpdateTypeUnknown UpdateType = "unknown"
	UpdateTypeNone    UpdateType = "none"
)

// String devuelve la representación string del tipo de actualización
func (u UpdateType) String() string {
	return string(u)
}

// ImageUpdate representa una actualización disponible para una imagen Docker
type ImageUpdate struct {
	ServiceName      string      `json:"service_name"`
	ServiceDirectory string      `json:"service_directory"`
	CurrentImage     DockerImage `json:"current_image"`
	LatestImage      DockerImage `json:"latest_image"`
	UpdateType       UpdateType  `json:"update_type"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// IsSignificant determina si la actualización es significativa (major o minor)
func (u ImageUpdate) IsSignificant() bool {
	return u.UpdateType == UpdateTypeMajor || u.UpdateType == UpdateTypeMinor
}