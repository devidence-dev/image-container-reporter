package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/user/docker-image-reporter/pkg/types"
)

func TestNewTestCmd(t *testing.T) {
	cmd := newTestCmd()

	if cmd.Use != "test" {
		t.Errorf("Expected command use to be 'test', got '%s'", cmd.Use)
	}

	if cmd.Short != "Test connectivity to services" {
		t.Errorf("Expected command short to be 'Test connectivity to services', got '%s'", cmd.Short)
	}

	// Check flags exist
	telegramFlag := cmd.Flags().Lookup("telegram")
	if telegramFlag == nil {
		t.Error("Expected --telegram flag to exist")
	}

	registriesFlag := cmd.Flags().Lookup("registries")
	if registriesFlag == nil {
		t.Error("Expected --registries flag to exist")
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("Expected --all flag to exist")
	}
}

func TestRunTest_NoFlags(t *testing.T) {
	cmd := newTestCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := runTest(cmd, []string{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	output := buf.String()
	expectedParts := []string{
		"Use --telegram, --registries, or --all flags",
		"--telegram",
		"--registries",
		"--all",
	}

	for _, part := range expectedParts {
		if !bytes.Contains([]byte(output), []byte(part)) {
			t.Errorf("Expected output to contain '%s', but it didn't", part)
		}
	}
}

func TestTestTelegram_Disabled(t *testing.T) {
	cfg := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled: false,
		},
	}

	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := testTelegram(cmd, cfg)

	if err != nil {
		t.Errorf("Expected no error for disabled telegram, got %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Telegram is disabled in configuration")) {
		t.Error("Expected output to mention telegram is disabled")
	}
}

func TestTestTelegram_MissingToken(t *testing.T) {
	cfg := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled: true,
			ChatID:  "123456",
		},
	}

	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := testTelegram(cmd, cfg)

	if err == nil {
		t.Error("Expected error for missing bot token")
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Telegram bot token is not configured")) {
		t.Error("Expected output to mention missing bot token")
	}
}

func TestTestTelegram_MissingChatID(t *testing.T) {
	cfg := &types.Config{
		Telegram: types.TelegramConfig{
			Enabled:  true,
			BotToken: "test_token",
		},
	}

	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := testTelegram(cmd, cfg)

	if err == nil {
		t.Error("Expected error for missing chat ID")
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Telegram chat ID is not configured")) {
		t.Error("Expected output to mention missing chat ID")
	}
}

func TestTestRegistries_NoRegistriesEnabled(t *testing.T) {
	cfg := &types.Config{
		Registry: types.RegistryConfig{
			DockerHub: types.DockerHubConfig{Enabled: false},
			GHCR:      types.GHCRConfig{Enabled: false},
		},
	}

	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := testRegistries(cmd, cfg)

	if err != nil {
		t.Errorf("Expected no error for no registries enabled, got %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("No registries are enabled in configuration")) {
		t.Error("Expected output to mention no registries enabled")
	}
}
