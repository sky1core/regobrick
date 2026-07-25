// Package opaerrparity pins the error strings standard OPA produces for
// division and modulo by zero.
//
// operator.go reproduces those strings verbatim (errors.New("divide by zero"),
// errors.New("modulo by zero")) so that a policy hitting the error sees the same
// message whether or not UseDecimalArithmetic is in effect. Nothing enforces
// that parity: regobrick's own tests assert only the regobrick side, so if an
// OPA release reworded its arithmetic errors, the two would drift apart
// silently and only the copied literals would be left behind. This test pins the
// standard OPA side, making the drift a build failure.
//
// It exists as its own package for test-binary isolation. The assertions are
// only meaningful in a process where regobrick has NOT overridden the
// arithmetic builtins, and topdown's builtin registry is process-global with no
// way to unregister — so regobrick's own test binary, which calls
// UseDecimalArithmetic, cannot host them. This package deliberately does not
// import regobrick at all, and TestStandardArithmeticIsNotOverridden verifies
// the resulting binary really is running stock OPA rather than assuming it.
package opaerrparity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// evalExpr evaluates a single expression under StrictBuiltinErrors(true), which
// is the mode that surfaces builtin errors to the caller instead of leaving the
// rule undefined.
func evalExpr(t *testing.T, expr string) (rego.ResultSet, error) {
	t.Helper()

	module := "package test\nimport rego.v1\nresult := " + expr + "\n"
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
		rego.StrictBuiltinErrors(true),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error for %q: %v", expr, err)
	}
	return query.Eval(ctx)
}

// TestStandardArithmeticIsNotOverridden establishes the precondition the parity
// assertions rest on: this binary evaluates with OPA's stock float arithmetic.
// Without it, a future import of regobrick (directly, or through a helper
// package that calls UseDecimalArithmetic in init) would silently turn the
// parity test into a test of regobrick's own copied strings, which would pass
// while proving nothing.
func TestStandardArithmeticIsNotOverridden(t *testing.T) {
	const (
		expr             = "1.1 + 2.2"
		wantStandardOPA  = "3.3000000000000000002" // big.Float, OPA v1.11.0
		decimalIfOverran = "3.3"                   // what regobrick's decimal path returns
	)

	rs, err := evalExpr(t, expr)
	if err != nil {
		t.Fatalf("eval %q: %v", expr, err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatalf("eval %q: undefined result", expr)
	}

	got := rs[0].Expressions[0].Value.(json.Number).String()
	if got == decimalIfOverran {
		t.Fatalf("%s evaluated to %s, which is regobrick's decimal result: this test binary has "+
			"UseDecimalArithmetic applied, so the error-string assertions in this package would be "+
			"checking regobrick's own copies rather than standard OPA. Keep this package free of any "+
			"import that registers the decimal builtins.", expr, got)
	}
	if got != wantStandardOPA {
		t.Errorf("%s evaluated to %s, want %s. Neither standard OPA's known value nor regobrick's: "+
			"OPA's numeric representation changed, so re-check what this package is actually pinning.",
			expr, got, wantStandardOPA)
	}
}

func TestStandardOPAArithmeticErrorStrings(t *testing.T) {
	tests := []struct {
		name string
		expr string
		// wantMessage is topdown.Error.Message: the builtin name prefix is added
		// by topdown, and the remainder is the literal operator.go duplicates.
		wantMessage string
		// wantCopiedLiteral is the exact string operator.go passes to errors.New.
		wantCopiedLiteral string
	}{
		{
			name:              "divide by zero",
			expr:              "1 / 0",
			wantMessage:       "div: divide by zero",
			wantCopiedLiteral: "divide by zero",
		},
		{
			name:              "modulo by zero",
			expr:              "10 % 0",
			wantMessage:       "rem: modulo by zero",
			wantCopiedLiteral: "modulo by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalExpr(t, tt.expr)
			if err == nil {
				t.Fatalf("standard OPA no longer raises an error for %s under StrictBuiltinErrors. "+
					"regobrick returns one for the same expression, so the two now disagree on whether "+
					"this is an error at all — a bigger divergence than a reworded message.", tt.expr)
			}

			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("standard OPA returned %T for %s, not *topdown.Error: %v", err, tt.expr, err)
			}
			if topdownErr.Code != topdown.BuiltinErr {
				t.Errorf("standard OPA now reports code %s for %s, want %s. regobrick returns a plain "+
					"error so that topdown classifies it as %s; a different code means the two paths no "+
					"longer produce the same kind of failure.",
					topdownErr.Code, tt.expr, topdown.BuiltinErr, topdown.BuiltinErr)
			}

			if topdownErr.Message != tt.wantMessage {
				t.Errorf("standard OPA's message for %s is now %q, pinned at %q.\n"+
					"operator.go hard-codes %q to keep regobrick's message identical to standard OPA's, "+
					"and that copy cannot notice a rewording on OPA's side. Update the literal in "+
					"operator.go and the expectation here together, or accept the divergence explicitly.",
					tt.expr, topdownErr.Message, tt.wantMessage, tt.wantCopiedLiteral)
			}
		})
	}
}
