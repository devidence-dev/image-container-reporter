package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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

	// Telegram tiene un límite de 4096 caracteres por mensaje
	const maxMessageLength = 4096

	// Si el mensaje es corto, enviarlo directamente
	if len(message) <= maxMessageLength {
		return t.sendSingleMessage(ctx, message)
	}

	// Dividir el mensaje en partes más pequeñas
	messages := t.splitMessage(message, maxMessageLength)

	// Enviar cada parte
	for i, msg := range messages {
		if i > 0 {
			// Pequeña pausa entre mensajes para evitar rate limits
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}

		if err := t.sendSingleMessage(ctx, msg); err != nil {
			return errors.Wrapf("telegram.SendNotification", err, "failed to send message part %d/%d", i+1, len(messages))
		}
	}

	return nil
}

// sendSingleMessage envía un mensaje individual a Telegram
func (t *TelegramClient) sendSingleMessage(ctx context.Context, message string) error {
	// Preparar la solicitud
	reqBody := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return errors.Wrap("telegram.sendSingleMessage", err)
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

	return errors.Wrapf("telegram.sendSingleMessage", lastErr, "failed after %d attempts", maxRetries)
}

// splitMessage divide un mensaje largo en partes más pequeñas
func (t *TelegramClient) splitMessage(message string, maxLength int) []string {
	if len(message) <= maxLength {
		return []string{message}
	}

	var messages []string
	runes := []rune(message) // Usar runes para manejar correctamente caracteres Unicode

	for len(runes) > 0 {
		if len(runes) <= maxLength {
			messages = append(messages, string(runes))
			break
		}

		// Encontrar un buen punto de corte (al final de una línea o cerca del límite)
		cutPoint := maxLength

		// Buscar hacia atrás para encontrar un salto de línea
		for i := maxLength - 1; i > maxLength/2 && i > 0; i-- {
			if runes[i] == '\n' {
				cutPoint = i + 1 // Incluir el salto de línea
				break
			}
		}

		// Si no se encontró un salto de línea, buscar un espacio
		if cutPoint == maxLength {
			for i := maxLength - 1; i > maxLength/2 && i > 0; i-- {
				if runes[i] == ' ' {
					cutPoint = i + 1 // Incluir el espacio
					break
				}
			}
		}

		// Cortar el mensaje
		part := string(runes[:cutPoint])
		messages = append(messages, part)
		runes = runes[cutPoint:]
	}

	return messages
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
	defer func() { _ = resp.Body.Close() }()

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

// sendMultipartRequest envía una solicitud HTTP multipart a la API de Telegram
func (t *TelegramClient) sendMultipartRequest(ctx context.Context, url string, body *bytes.Buffer, boundary string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return errors.Wrap("telegram.sendMultipartRequest", err)
	}

	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

	resp, err := t.client.Do(req)
	if err != nil {
		return errors.Wrap("telegram.sendMultipartRequest", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Leer la respuesta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap("telegram.sendMultipartRequest", err)
	}

	// Verificar el código de estado
	if resp.StatusCode != http.StatusOK {
		return errors.Newf("telegram.sendMultipartRequest", "telegram API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	// Parsear la respuesta JSON
	var telegramResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description,omitempty"`
	}

	if err := json.Unmarshal(respBody, &telegramResp); err != nil {
		return errors.Wrap("telegram.sendMultipartRequest", err)
	}

	if !telegramResp.OK {
		return errors.Newf("telegram.sendMultipartRequest", "telegram API error: %s", telegramResp.Description)
	}

	return nil
}

// Name devuelve el nombre del cliente de notificación
func (t *TelegramClient) Name() string {
	return "telegram"
}

// SendFile envía un archivo como documento a Telegram
func (t *TelegramClient) SendFile(ctx context.Context, filePath, fileName, caption string) error {
	if t.botToken == "" {
		return errors.New("telegram.SendFile", "bot token is required")
	}
	if t.chatID == "" {
		return errors.New("telegram.SendFile", "chat ID is required")
	}

	// Leer el archivo
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrap("telegram.SendFile", err)
	}

	// Crear multipart form data
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Agregar campos
	if err := w.WriteField("chat_id", t.chatID); err != nil {
		return errors.Wrap("telegram.SendFile", err)
	}
	if caption != "" {
		if err := w.WriteField("caption", caption); err != nil {
			return errors.Wrap("telegram.SendFile", err)
		}
		if err := w.WriteField("parse_mode", "HTML"); err != nil {
			return errors.Wrap("telegram.SendFile", err)
		}
	}

	// Agregar el archivo
	fw, err := w.CreateFormFile("document", fileName)
	if err != nil {
		return errors.Wrap("telegram.SendFile", err)
	}
	if _, err := fw.Write(fileData); err != nil {
		return errors.Wrap("telegram.SendFile", err)
	}

	w.Close()

	url := fmt.Sprintf(telegramBaseURL, t.botToken, "sendDocument")

	// Intentar enviar con reintentos
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := t.sendMultipartRequest(ctx, url, &b, w.Boundary())
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

	return errors.Wrapf("telegram.SendFile", lastErr, "failed after %d attempts", maxRetries)
}
