package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Resolver provides helper functions for applying env > CLI > default precedence.
type Resolver struct {
	logger *zap.Logger
}

// NewResolver creates a Resolver with the provided logger.
func NewResolver(logger *zap.Logger) Resolver {
	return Resolver{logger: logger}
}

func (r Resolver) logConflict(setting, envVal, cliVal string) {
	if r.logger == nil {
		return
	}
	r.logger.Warn(
		"config: conflict for "+setting,
		zap.String("env", envVal),
		zap.String("cli", cliVal),
		zap.String("decision", "using env value"),
	)
}

func (r Resolver) pick(setting string, envVal string, envSet bool, cliVal string, cliSet bool, defaultVal string) string {
	if envSet && cliSet && envVal != cliVal {
		r.logConflict(setting, envVal, cliVal)
	}
	if envSet {
		return envVal
	}
	if cliSet {
		return cliVal
	}
	return defaultVal
}

// String resolves a string setting using the precedence rules.
func (r Resolver) String(setting, envKey, cliVal string, cliSet bool, defaultVal string) string {
	envVal, envSet := os.LookupEnv(envKey)
	return r.pick(setting, strings.TrimSpace(envVal), envSet, cliVal, cliSet, defaultVal)
}

// Bool resolves a boolean setting.
func (r Resolver) Bool(setting, envKey string, cliVal bool, cliSet bool, defaultVal bool) (bool, error) {
	envVal, envSet := os.LookupEnv(envKey)
	if !envSet {
		if cliSet {
			return cliVal, nil
		}
		return defaultVal, nil
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(envVal))
	if err != nil {
		return false, fmt.Errorf("config %s: invalid boolean %q: %w", setting, envVal, err)
	}

	if cliSet && parsed != cliVal {
		r.logConflict(setting, envVal, strconv.FormatBool(cliVal))
	}

	return parsed, nil
}

// Int resolves an integer setting.
func (r Resolver) Int(setting, envKey string, cliVal int, cliSet bool, defaultVal int) (int, error) {
	envVal, envSet := os.LookupEnv(envKey)
	if !envSet {
		if cliSet {
			return cliVal, nil
		}
		return defaultVal, nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(envVal))
	if err != nil {
		return 0, fmt.Errorf("config %s: invalid integer %q: %w", setting, envVal, err)
	}

	if cliSet && parsed != cliVal {
		r.logConflict(setting, envVal, strconv.Itoa(cliVal))
	}

	return parsed, nil
}

// StringSlice resolves a slice of strings. Env values are comma-separated.
func (r Resolver) StringSlice(setting, envKey string, cliVal []string, cliSet bool, defaultVal []string) []string {
	envVal, envSet := os.LookupEnv(envKey)
	if envSet {
		parts := splitAndClean(envVal)
		if cliSet && !equalSlices(parts, cliVal) {
			r.logConflict(setting, envVal, strings.Join(cliVal, ","))
		}
		return parts
	}

	if cliSet {
		return sanitizeStrings(cliVal)
	}

	return sanitizeStrings(defaultVal)
}

func splitAndClean(value string) []string {
	if value == "" {
		return nil
	}
	raw := strings.Split(value, ",")
	return sanitizeStrings(raw)
}

func sanitizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clean := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
