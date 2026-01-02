package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DBConfig holds PostgreSQL connection configuration
type DBConfig struct {
	ConnectionURL string // Full connection string (e.g., postgresql://user:pass@host:port/db)
	MaxConns      int32
	MinConns      int32
}

// LoadDBConfig loads database configuration from environment variables
func LoadDBConfig() DBConfig {
	// Prioritize DATABASE_URL (standard for managed PostgreSQL services)
	connectionURL := os.Getenv("DATABASE_URL")

	// Fallback: build from individual components for backwards compatibility
	if connectionURL == "" && getEnv("DB_HOST", "") != "" {
		connectionURL = fmt.Sprintf(
			"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			getEnv("DB_USER", "libble"),
			getEnv("DB_PASSWORD", ""),
			getEnv("DB_HOST", ""),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_NAME", "libble"),
			getEnv("DB_SSLMODE", "require"),
		)
	}

	return DBConfig{
		ConnectionURL: connectionURL,
		MaxConns:      int32(getEnvInt("DB_MAX_CONNS", 25)),
		MinConns:      int32(getEnvInt("DB_MIN_CONNS", 5)),
	}
}

// IsConfigured returns true if a connection URL is available
func (c DBConfig) IsConfigured() bool {
	return c.ConnectionURL != ""
}

// getEnv retrieves an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable with a default fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// parseDuration parses a duration string with a default fallback
func parseDuration(value string) time.Duration {
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	return 1 * time.Hour // Default fallback
}
