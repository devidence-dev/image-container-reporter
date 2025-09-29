package types

import "fmt"

// DockerImage representa una imagen Docker con su registro, repositorio y tag
type DockerImage struct {
	Registry      string `json:"registry"`
	Repository    string `json:"repository"`
	Tag           string `json:"tag"`
	Digest        string `json:"digest,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	ComposeFile   string `json:"compose_file,omitempty"`
	ContainerID   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
}

// String devuelve la representación completa de la imagen Docker
func (d DockerImage) String() string {
	if d.Registry == "docker.io" {
		return fmt.Sprintf("%s:%s", d.Repository, d.Tag)
	}
	return fmt.Sprintf("%s/%s:%s", d.Registry, d.Repository, d.Tag)
}

// FullName devuelve el nombre completo incluyendo el registro
func (d DockerImage) FullName() string {
	return fmt.Sprintf("%s/%s:%s", d.Registry, d.Repository, d.Tag)
}

// IsValid verifica si la imagen tiene los campos requeridos
func (d DockerImage) IsValid() bool {
	return d.Registry != "" && d.Repository != "" && d.Tag != ""
}
