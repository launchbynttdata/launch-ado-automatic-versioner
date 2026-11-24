package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/cli/exitcodes"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/config"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/prtitle"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/pipeline"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/services/prlabel"
)

func TestMapPRLabelErrorExitCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		result prlabel.Result
		want   int
		msgHas string
	}{
		{
			name: "invalid conventional commit exits 3",
			err:  fmt.Errorf("%w: parse failed", prtitle.ErrInvalidConventionalCommit),
			result: prlabel.Result{
				PRTitle: "WIP stuff",
			},
			want:   exitcodes.SemanticValidation,
			msgHas: `invalid format in "WIP stuff"`,
		},
		{
			name:   "ado api exits 2",
			err:    fmt.Errorf("%w: listing pr labels: boom", prlabel.ErrADOAPI),
			want:   exitcodes.ADOAPIError,
			msgHas: "ado api",
		},
		{
			name:   "empty branch exits 1",
			err:    prlabel.ErrEmptyBranch,
			want:   exitcodes.ConfigError,
			msgHas: "empty branch",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mapped := mapPRLabelError(tc.err, tc.result)
			if got := exitcodes.CodeFor(mapped); got != tc.want {
				t.Fatalf("CodeFor(mapPRLabelError()) = %d, want %d (err=%v)", got, tc.want, mapped)
			}
			if tc.msgHas != "" && !strings.Contains(mapped.Error(), tc.msgHas) {
				t.Fatalf("mapped error %q does not contain %q", mapped.Error(), tc.msgHas)
			}
		})
	}
}

func TestEmitPRLabelFallbackWarningExactLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := emitPRLabelFallbackWarning(&buf); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	want := pipeline.FallbackWarningPrefix + pipeline.FallbackWarningMessage
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMapPRLabelErrorNil(t *testing.T) {
	t.Parallel()
	if err := mapPRLabelError(nil, prlabel.Result{}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRootCommandRejectsTokenFlag(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"pr-label", "--token", "secret-canary"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unknown flag error for --token, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag: --token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("reads token from stdin when --token-stdin is set", func(t *testing.T) {
		t.Setenv(envToken, "")
		_ = os.Unsetenv(envToken)

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flag := bindBoolFlag(fs, "token-stdin", "token-stdin", "", "", false, "")
		if err := fs.Parse([]string{"--token-stdin"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}

		resolver := config.NewResolver(zap.NewNop())
		got, err := resolveToken(resolver, strings.NewReader("  stdin-secret-token\n"), flag)
		if err != nil {
			t.Fatalf("resolveToken: %v", err)
		}
		if got != "stdin-secret-token" {
			t.Fatalf("got %q, want %q", got, "stdin-secret-token")
		}
	})

	t.Run("env overrides stdin and redacts conflict log", func(t *testing.T) {
		t.Setenv(envToken, "env-secret-token")

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flag := bindBoolFlag(fs, "token-stdin", "token-stdin", "", "", false, "")
		if err := fs.Parse([]string{"--token-stdin"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}

		core, logs := observer.New(zap.WarnLevel)
		resolver := config.NewResolver(zap.New(core))
		got, err := resolveToken(resolver, strings.NewReader("stdin-secret-token\n"), flag)
		if err != nil {
			t.Fatalf("resolveToken: %v", err)
		}
		if got != "env-secret-token" {
			t.Fatalf("got %q, want %q", got, "env-secret-token")
		}
		if logs.Len() != 1 {
			t.Fatalf("expected 1 conflict log entry, got %d", logs.Len())
		}
		fields := logs.All()[0].ContextMap()
		if fields["env"] != "***" || fields["cli"] != "***" {
			t.Fatalf("expected redacted conflict log fields, got %v", fields)
		}
	})

	t.Run("fails when neither env nor stdin provides token", func(t *testing.T) {
		t.Setenv(envToken, "")
		_ = os.Unsetenv(envToken)

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flag := bindBoolFlag(fs, "token-stdin", "token-stdin", "", "", false, "")

		resolver := config.NewResolver(zap.NewNop())
		_, err := resolveToken(resolver, strings.NewReader(""), flag)
		if err == nil {
			t.Fatal("expected error when token is missing, got nil")
		}
		if !strings.Contains(err.Error(), "--token-stdin") {
			t.Fatalf("expected error to mention --token-stdin, got %v", err)
		}
	})
}
