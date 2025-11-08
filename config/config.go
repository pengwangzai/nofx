package config

import (
	"encoding/json"
	"os"
	"strconv"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `json:"server"`
	Database DatabaseConfig `json:"database"`
	API     APIConfig     `json:"api"`
	Logging LoggingConfig `json:"logging"`
	Trading TradingConfig `json:"trading"`
	Security SecurityConfig `json:"security"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// APIConfig represents API configuration
type APIConfig struct {
	Timeout   int `json:"timeout"`
	RateLimit int `json:"rate_limit"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// TradingConfig represents trading configuration
type TradingConfig struct {
	DefaultLeverage int64   `json:"default_leverage"`
	MaxPositionSize float64 `json:"max_position_size"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionKeyPath string `json:"encryption_key_path"`
}

// Load loads configuration from file or environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("PORT", "8080"),
		},
		Logging: LoggingConfig{
			Level: getEnv("LOG_LEVEL", "info"),
			File:  getEnv("LOG_FILE", ""),
		},
		Security: SecurityConfig{
			EncryptionEnabled: getEnvBool("ENCRYPTION_ENABLED", false),
			EncryptionKeyPath: getEnv("ENCRYPTION_KEY_PATH", ""),
		},
	}

	// Try to load from config.json
	if _, err := os.Stat("config.json"); err == nil {
		file, err := os.Open("config.json")
		if err == nil {
			defer file.Close()
			if err := json.NewDecoder(file).Decode(cfg); err != nil {
				return nil, err
			}
		}
	}

	return cfg, nil
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}