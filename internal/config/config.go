package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Default sleep configuration
	DefaultWakeAt   string
	DefaultSleepAt  string
	DefaultTimezone string

	// Namespace controller configuration
	NamespaceDelaySeconds int
	ExcludedNamespaces    []string

	// Global policy defaults
	Weekend        string
	ExcludeWeekend bool
}

// Default configuration constants
// TODO: export consts to file.
const (
	DefaultWakeAt                = "08:00"
	DefaultSleepAt               = "20:00"
	DefaultTimezone              = "UTC"
	DefaultNamespaceDelaySeconds = 20
	DefaultWeekend               = ""
	DefaultExcludeWeekend        = false
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
		DefaultWakeAt:         getEnvStrOrDefault("SLEEPOD_DEFAULT_WAKE_AT", DefaultWakeAt),
		DefaultSleepAt:        getEnvStrOrDefault("SLEEPOD_DEFAULT_SLEEP_AT", DefaultSleepAt),
		DefaultTimezone:       getEnvStrOrDefault("SLEEPOD_DEFAULT_TIMEZONE", DefaultTimezone),
		ExcludedNamespaces:    getEnvStringSliceOrDefault("SLEEPOD_EXCLUDED_NAMESPACES", DefaultExcludedNamespaces),
		NamespaceDelaySeconds: getEnvIntOrDefault("SLEEPOD_NAMESPACE_DELAY_SECONDS", DefaultNamespaceDelaySeconds),
		Weekend:               getEnvStrOrDefault("SLEEPOD_WEEKEND", DefaultWeekend),
		ExcludeWeekend:        getEnvBoolOrDefault("SLEEPOD_EXCLUDE_WEEKEND", DefaultExcludeWeekend),
	}
	namespace := os.Getenv("SLEEPOD_NAMESPACE")
	if namespace != "" {
		config.ExcludedNamespaces = append(config.ExcludedNamespaces, namespace)
	}

	return config
}

// IsNamespaceExcludedFromConfig checks if a namespace is in the exclusion list
func (c *Config) IsNamespaceExcludedFromConfig(namespace string) bool {
	for _, excluded := range c.ExcludedNamespaces {
		if excluded == namespace {
			return true
		}
	}
	return false
}

// GetNamespaceDelay returns the delay as a time.Duration
func (c *Config) GetNamespaceDelay() time.Duration {
	return time.Duration(c.NamespaceDelaySeconds) * time.Second
}

// Helper functions for str environment variable parsing
func getEnvStrOrDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper functions for int environment variable parsing
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvStringSliceOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
