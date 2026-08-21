package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/cli/exitcodes"
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
