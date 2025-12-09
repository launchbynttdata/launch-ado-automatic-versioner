package config

import (
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestResolver_Secret_RedactsLogs(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	resolver := NewResolver(logger)

	envKey := "TEST_SECRET_ENV"
	err := os.Setenv(envKey, "env-secret")
	if err != nil {
		t.Fatalf("failed to set env var: %v", err)
	}
	defer func() {
		err := os.Unsetenv(envKey)
		if err != nil {
			t.Fatalf("failed to unset env var: %v", err)
		}
	}()

	val := resolver.Secret("test-secret", envKey, "cli-secret", true, "default")

	if val != "env-secret" {
		t.Errorf("expected env value 'env-secret', got %q", val)
	}

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	if entry.Message != "config: conflict for test-secret" {
		t.Errorf("unexpected log message: %q", entry.Message)
	}

	fields := entry.ContextMap()
	if fields["env"] != "***" {
		t.Errorf("expected env field to be redacted, got %q", fields["env"])
	}
	if fields["cli"] != "***" {
		t.Errorf("expected cli field to be redacted, got %q", fields["cli"])
	}
}

func TestResolver_String_DoesNotRedactLogs(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	resolver := NewResolver(logger)

	envKey := "TEST_STRING_ENV"
	err := os.Setenv(envKey, "env-value")
	if err != nil {
		t.Fatalf("failed to set env var: %v", err)
	}
	defer func() {
		err := os.Unsetenv(envKey)
		if err != nil {
			t.Fatalf("failed to unset env var: %v", err)
		}
	}()

	val := resolver.String("test-string", envKey, "cli-value", true, "default")

	if val != "env-value" {
		t.Errorf("expected env value 'env-value', got %q", val)
	}

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	fields := logs.All()[0].ContextMap()
	if fields["env"] != "env-value" {
		t.Errorf("expected env field to be 'env-value', got %q", fields["env"])
	}
	if fields["cli"] != "cli-value" {
		t.Errorf("expected cli field to be 'cli-value', got %q", fields["cli"])
	}
}
