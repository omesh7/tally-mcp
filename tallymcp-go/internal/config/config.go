// Package config provides application-level configuration with environment variable overrides.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configurable parameters for the TallyMCP server.
type Config struct {
	TallyHost string // Tally Prime HTTP server host (default: "localhost")
	TallyPort int    // Tally Prime HTTP server port (default: 9000)
	LogLevel  string // Logging verbosity: "debug", "info", "warn", "error" (default: "info")
}

// DefaultConfig returns the standard configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		TallyHost: "localhost",
		TallyPort: 9000,
		LogLevel:  "info",
	}
}

// LoadFromEnv creates a Config by reading environment variables, falling back to defaults.
//
// Supported environment variables:
//   - TALLY_HOST        → TallyHost
//   - TALLY_PORT        → TallyPort
//   - TALLYMCP_LOG_LEVEL → LogLevel
func LoadFromEnv() *Config {
	cfg := DefaultConfig()

	if host := os.Getenv("TALLY_HOST"); host != "" {
		cfg.TallyHost = host
	}
	if portStr := os.Getenv("TALLY_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
			cfg.TallyPort = p
		}
	}
	if level := os.Getenv("TALLYMCP_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}

	return cfg
}

// TallyURL returns the full base URL for the Tally Prime XML server.
func (c *Config) TallyURL() string {
	return fmt.Sprintf("http://%s:%d/", c.TallyHost, c.TallyPort)
}
