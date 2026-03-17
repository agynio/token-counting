package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultGRPCAddress = ":50051"
	defaultLogLevel    = "info"
)

// Config captures runtime configuration derived from the environment.
type Config struct {
	GRPCAddress string
	LogLevel    string
}

// Load reads configuration from environment variables, applying defaults when
// values are not provided. Returns an error when supplied values are invalid.
func Load() (Config, error) {
	var cfg Config

	cfg.GRPCAddress = readEnv("GRPC_ADDRESS", defaultGRPCAddress)

	logLevel, err := parseLogLevel(readEnv("LOG_LEVEL", defaultLogLevel))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = logLevel

	return cfg, nil
}

func readEnv(key, def string) string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}
	return def
}

func parseLogLevel(level string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(level))
	if value == "" {
		value = defaultLogLevel
	}

	switch value {
	case "info":
		return "info", nil
	case "debug":
		return "debug", nil
	case "warn", "warning":
		return "warn", nil
	case "error":
		return "error", nil
	default:
		return "", fmt.Errorf("invalid LOG_LEVEL: %q", level)
	}
}
