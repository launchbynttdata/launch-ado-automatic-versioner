package exitcodes

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: Success},
		{name: "semantic", err: NewSemanticError("bad title", errors.New("cause")), want: SemanticValidation},
		{name: "ado api", err: WrapADOAPI(errors.New("boom")), want: ADOAPIError},
		{name: "config", err: WrapConfig(errors.New("missing flag")), want: ConfigError},
		{name: "generic", err: errors.New("something else"), want: ConfigError},
		{name: "wrapped semantic", err: fmt.Errorf("outer: %w", NewSemanticError("bad", nil)), want: SemanticValidation},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CodeFor(tc.err); got != tc.want {
				t.Fatalf("CodeFor() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWrapHelpersNilSafe(t *testing.T) {
	t.Parallel()

	if WrapADOAPI(nil) != nil {
		t.Fatalf("expected WrapADOAPI(nil) to return nil")
	}
	if WrapConfig(nil) != nil {
		t.Fatalf("expected WrapConfig(nil) to return nil")
	}
}
