//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
)

var tokenCountingAddress = envOrDefault("TOKEN_COUNTING_ADDRESS", "token-counting:50051")

func envOrDefault(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
