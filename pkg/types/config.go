package types

// ScanConfig representa la configuración para el escaneo
type ScanConfig struct {
	Recursive bool     `yaml:"recursive" json:"recursive"`
	Patterns  []string `yaml:"patterns" json:"patterns"`
	Timeout   int      `yaml:"timeout" json:"timeout"` // en segundos
}

// RegistryConfig representa la configuración de registros
type RegistryConfig struct {
	DockerHub DockerHubConfig `yaml:"dockerhub" json:"dockerhub"`
	GHCR      GHCRConfig      `yaml:"ghcr" json:"ghcr"`
	Timeout   int             `yaml:"timeout" json:"timeout"` // en segundos
}

// DockerHubConfig configuración para Docker Hub
type DockerHubConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Timeout int  `yaml:"timeout" json:"timeout"`
}

// GHCRConfig configuración para GitHub Container Registry
type GHCRConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Token   string `yaml:"token" json:"token" env:"GITHUB_TOKEN"`
	Timeout int    `yaml:"timeout" json:"timeout"`
}

// TelegramConfig configuración para notificaciones Telegram
type TelegramConfig struct {
	BotToken string `yaml:"bot_token" json:"bot_token" env:"TELEGRAM_BOT_TOKEN"`
	ChatID   string `yaml:"chat_id" json:"chat_id" env:"TELEGRAM_CHAT_ID"`
	Enabled  bool   `yaml:"enabled" json:"enabled" env:"TELEGRAM_ENABLED"`
	Template string `yaml:"template" json:"template"`
}

// Config representa la configuración completa de la aplicación
type Config struct {
	Telegram TelegramConfig `yaml:"telegram" json:"telegram"`
	Registry RegistryConfig `yaml:"registry" json:"registry"`
	Scan     ScanConfig     `yaml:"scan" json:"scan"`
}
