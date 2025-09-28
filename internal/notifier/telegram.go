package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/user/docker-image-reporter/pkg/errors"
)

const (
	telegramBaseURL = "https://api.telegram.org/bot%s/%s"
	maxRetries      = 3
	retryDelay      = 2 * time.Second
)

// TelegramClient implementa NotificationClient para enviar notificaciones via Telegram
type TelegramClient struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramClient crea un nuevo cliente de Telegram
func NewTelegramClient(botToken, chatID string) *TelegramClient {
	return &TelegramClient{
		botToken: botToken,
		chatID:   chatID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendNotification envía una notificación a Telegram con reintentos
func (t *TelegramClient) SendNotification(ctx context.Context, message string) error {
	if t.botToken == "" {
		return errors.New("telegram.SendNotification", "bot token is required")
	}
	if t.chatID == "" {
		return errors.New("telegram.SendNotification", "chat ID is required")
	}

	// Preparar la solicitud
	reqBody := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return errors.Wrap("telegram.SendNotification", err)
	}

	url := fmt.Sprintf(telegramBaseURL, t.botToken, "sendMessage")

	// Intentar enviar con reintentos
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := t.sendRequest(ctx, url, jsonData)
		if err == nil {
			return nil // Éxito
		}

		lastErr = err

		// Si no es el último intento, esperar antes de reintentar
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				// Continuar con el siguiente intento
			}
		}
	}

	return errors.Wrapf("telegram.SendNotification", lastErr, "failed after %d attempts", maxRetries)
}

// sendRequest envía una solicitud HTTP a la API de Telegram
func (t *TelegramClient) sendRequest(ctx context.Context, url string, jsonData []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap("telegram.sendRequest", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return errors.Wrap("telegram.sendRequest", err)
	}
	defer resp.Body.Close()

	// Leer la respuesta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap("telegram.sendRequest", err)
	}

	// Verificar el código de estado
	if resp.StatusCode != http.StatusOK {
		return errors.Newf("telegram.sendRequest", "telegram API error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parsear la respuesta JSON
	var telegramResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description,omitempty"`
	}

	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return errors.Wrap("telegram.sendRequest", err)
	}

	if !telegramResp.OK {
		return errors.Newf("telegram.sendRequest", "telegram API error: %s", telegramResp.Description)
	}

	return nil
}

// Name devuelve el nombre del cliente de notificación
func (t *TelegramClient) Name() string {
	return "telegram"
}
