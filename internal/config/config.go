package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the trading system
type Config struct {
	Database    DatabaseConfig
	Server      ServerConfig
	MT5         MT5Config
	Risk        RiskConfig
	Logging     LoggingConfig
	Environment string
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL             string
	MaxConnections  int
	ConnMaxLifetime int // in minutes
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port          string
	WebhookSecret string
}

// MT5Config holds MetaTrader 5 connection settings
type MT5Config struct {
	Endpoint       string
	TimeoutSeconds int
	RetryAttempts  int
	RetryDelayMs   int
}

// RiskConfig holds risk management parameters
type RiskConfig struct {
	MaxDailyLoss     float64
	MaxPositionSize  float64
	MaxOpenPositions int
	EnableRiskChecks bool
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string
	Format string // json or text
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	// Fix for PostgreSQL prepared statement issue - add default_query_exec_mode=simple_protocol
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL != "" && !strings.Contains(databaseURL, "default_query_exec_mode=simple_protocol") {
		separator := "?"
		if strings.Contains(databaseURL, "?") {
			separator = "&"
		}
		databaseURL = databaseURL + separator + "default_query_exec_mode=simple_protocol"
	}

	config := &Config{
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxConnections:  getEnvInt("DB_MAX_CONNECTIONS", 5),    // Reduced from 10 for stability
			ConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 30), // Reduced from 60 for better connection cycling
		},
		Server: ServerConfig{
			Port:          getEnv("SERVER_PORT", "8081"),
			WebhookSecret: getEnv("WEBHOOK_SECRET", ""),
		},
		MT5: MT5Config{
			Endpoint:       getEnv("MT5_ENDPOINT", "http://localhost:8080"),
			TimeoutSeconds: getEnvInt("MT5_TIMEOUT_SECONDS", 5),
			RetryAttempts:  getEnvInt("MT5_RETRY_ATTEMPTS", 3),
			RetryDelayMs:   getEnvInt("MT5_RETRY_DELAY_MS", 1000),
		},
		Risk: RiskConfig{
			MaxDailyLoss:     getEnvFloat("RISK_MAX_DAILY_LOSS", 1000.0),
			MaxPositionSize:  getEnvFloat("RISK_MAX_POSITION_SIZE", 0.1),
			MaxOpenPositions: getEnvInt("RISK_MAX_OPEN_POSITIONS", 3),
			EnableRiskChecks: getEnvBool("RISK_ENABLE_CHECKS", true),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Environment: getEnv("ENVIRONMENT", "development"),
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.Server.WebhookSecret == "" {
		return fmt.Errorf("WEBHOOK_SECRET is required")
	}

	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, c.Logging.Level) {
		return fmt.Errorf("invalid log level: %s, must be one of %v", c.Logging.Level, validLogLevels)
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, c.Logging.Format) {
		return fmt.Errorf("invalid log format: %s, must be one of %v", c.Logging.Format, validLogFormats)
	}

	if c.Risk.MaxDailyLoss <= 0 {
		return fmt.Errorf("RISK_MAX_DAILY_LOSS must be positive")
	}

	if c.Risk.MaxPositionSize <= 0 || c.Risk.MaxPositionSize > 10 {
		return fmt.Errorf("RISK_MAX_POSITION_SIZE must be between 0 and 10")
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
