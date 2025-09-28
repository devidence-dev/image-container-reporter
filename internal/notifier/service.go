package notifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/docker-image-reporter/pkg/errors"
	"github.com/user/docker-image-reporter/pkg/types"
)

// NotificationService coordina el envío de notificaciones a múltiples clientes
type NotificationService struct {
	clients []types.NotificationClient
}

// NewNotificationService crea un nuevo servicio de notificaciones
func NewNotificationService(clients ...types.NotificationClient) *NotificationService {
	return &NotificationService{
		clients: clients,
	}
}

// AddClient agrega un cliente de notificación al servicio
func (s *NotificationService) AddClient(client types.NotificationClient) {
	s.clients = append(s.clients, client)
}

// NotifyScanResult envía notificaciones basadas en el resultado del escaneo
func (s *NotificationService) NotifyScanResult(ctx context.Context, result types.ScanResult, formatter types.ReportFormatter) error {
	if len(s.clients) == 0 {
		return nil // No hay clientes configurados, no es un error
	}

	// Solo enviar notificaciones si hay updates o errores
	if !result.HasUpdates() && !result.HasErrors() {
		return nil // Nada que notificar
	}

	// Formatear el mensaje usando el formatter proporcionado
	message, err := formatter.Format(result)
	if err != nil {
		return errors.Wrap("notification.NotifyScanResult", err)
	}

	// Enviar a todos los clientes
	var errs []string
	for _, client := range s.clients {
		if err := client.SendNotification(ctx, message); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", client.Name(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Newf("notification.NotifyScanResult", "failed to send notifications: %s", strings.Join(errs, "; "))
	}

	return nil
}

// NotifyCustomMessage envía un mensaje personalizado a todos los clientes
func (s *NotificationService) NotifyCustomMessage(ctx context.Context, message string) error {
	if len(s.clients) == 0 {
		return nil // No hay clientes configurados, no es un error
	}

	// Enviar a todos los clientes
	var errs []string
	for _, client := range s.clients {
		if err := client.SendNotification(ctx, message); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", client.Name(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Newf("notification.NotifyCustomMessage", "failed to send notifications: %s", strings.Join(errs, "; "))
	}

	return nil
}

// HasClients verifica si hay clientes de notificación configurados
func (s *NotificationService) HasClients() bool {
	return len(s.clients) > 0
}

// GetClientNames devuelve los nombres de todos los clientes configurados
func (s *NotificationService) GetClientNames() []string {
	names := make([]string, len(s.clients))
	for i, client := range s.clients {
		names[i] = client.Name()
	}
	return names
}
