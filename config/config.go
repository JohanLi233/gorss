package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Feed represents an RSS feed configuration
type Feed struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

// Config represents the application configuration
type Config struct {
	Feeds  []Feed       `mapstructure:"feeds"`
	Ollama OllamaConfig `mapstructure:"ollama"`
}

// OllamaConfig represents configuration for the Ollama LLM integration
type OllamaConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	URL         string `mapstructure:"url"`
	Model       string `mapstructure:"model"`
	MaxArticles int    `mapstructure:"max_articles"`
	Timeout     int    `mapstructure:"timeout"`
}

// LoadConfig loads the configuration from the default location
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "gorss")
	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config file exists, if not create a default one
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		defaultConfig := `feeds:
		- name: "gorss"
		url: "https://github.com/JohanLi233/gorss/releases"
		ollama:
		enabled: true
		url: "http://localhost:11434"
		model: "qwen3:32b "
		max_articles: 100
		timeout: 30
		`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
		fmt.Printf("Created default config file at %s\n", configPath)
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}
