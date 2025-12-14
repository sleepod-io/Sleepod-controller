package config

import (
	"os"
	"strings"
)

type Config struct {
	// Default sleep configuration
	DefaultWakeAt   string
	DefaultSleepAt  string
	DefaultTimezone string

	// Namespace controller configuration
	// Namespaces to ignore (e.g. kube-system)
	ExcludedNamespaces []string
}

// Default configuration constants
const (
	DefaultWakeAt   = "08:00"
	DefaultSleepAt  = "20:00"
	DefaultTimezone = "UTC"
)

var DefaultExcludedNamespaces = []string{
	"kube-system",
	"kube-public",
	"kube-node-lease",
	"local-path-storage", // Common in Kind/Minikube
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		DefaultWakeAt:      getEnvOrDefault("SLEEPOD_DEFAULT_WAKE_AT", DefaultWakeAt),
		DefaultSleepAt:     getEnvOrDefault("SLEEPOD_DEFAULT_SLEEP_AT", DefaultSleepAt),
		DefaultTimezone:    getEnvOrDefault("SLEEPOD_DEFAULT_TIMEZONE", DefaultTimezone),
		ExcludedNamespaces: getEnvStringSliceOrDefault("SLEEPOD_EXCLUDED_NAMESPACES", DefaultExcludedNamespaces),
	}

	return config
}

// IsNamespaceExcluded checks if a namespace is in the exclusion list
func (c *Config) IsNamespaceExcluded(namespace string) bool {
	for _, excluded := range c.ExcludedNamespaces {
		if excluded == namespace {
			return true
		}
	}
	return false
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvStringSliceOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
