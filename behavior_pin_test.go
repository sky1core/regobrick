package regobrick

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
)

// These tests pin the CURRENT observed behavior of edge cases so that future
// changes surface as explicit test failures instead of silent drift. A pinned
// behavior is not automatically the desired behavior; each test states what is
// being frozen.

type pinPayload struct {
	X int    `json:"x"`
	Y string `json:"y"`
}

func init() {
	RegisterBuiltin0[[]string]("pin_nil_slice", func(rego.BuiltinContext) ([]string, error) {
		return nil, nil
	})
	RegisterBuiltin0[[]string]("pin_empty_slice", func(rego.BuiltinContext) ([]string, error) {
		return []string{}, nil
	})
	RegisterBuiltin0[time.Time]("pin_time_ret", func(rego.BuiltinContext) (time.Time, error) {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), nil
	})
	RegisterBuiltin1[time.Time, string]("pin_time_echo", func(_ rego.BuiltinContext, ts time.Time) (string, error) {
		return ts.UTC().Format(time.RFC3339), nil
	})
	RegisterBuiltin1[pinPayload, pinPayload]("pin_struct_echo", func(_ rego.BuiltinContext, p pinPayload) (pinPayload, error) {
		return p, nil
	})
}

// Pin: with decimal arithmetic, composite (array/object) equality benefits from
// arithmetic normalization: 0.1 + 0.2 evaluates to exactly 0.3, so the composite
// values compare equal. Standard OPA would yield false here (big.Float residue).
func TestPin_CompositeEqualityAfterDecimalArithmetic(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
import rego.v1
result := [0.1 + 0.2] == [0.3]`
	rs := evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs); got != true {
		t.Fatalf("array equality: got %v, want true", got)
	}

	module = `package test
import rego.v1
result := {"a": 0.1 + 0.2} == {"a": 0.3}`
	rs = evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs); got != true {
		t.Fatalf("object equality: got %v, want true", got)
	}
}

// Pin: scalar == on a number beyond udecimal's 19-dp precision fails to parse,
// so the comparison is undefined (default mode) instead of false. This is the
// documented precision-limit behavior: fail loudly rather than approximate.
// Standard OPA would return false (the values genuinely differ).
func TestPin_ScalarEqualityBeyondPrecisionIsUndefined(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
import rego.v1
result := input.a == 0.3`
	input := map[string]any{"a": Number("0.30000000000000000000000001")}

	rs := evalModuleResult(t, module, input)
	requireUndefinedResult(t, rs)
}

// Pin: on numeric ties, max/min return the FIRST encountered element's
// representation ("1.0" vs "1" are numerically equal but textually distinct).
func TestPin_MaxMinTieKeepsFirstRepresentation(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
import rego.v1
result := max([1.0, 1])`
	rs := evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != "1.0" {
		t.Fatalf("max tie: got %s, want 1.0", got)
	}

	module = `package test
import rego.v1
result := min([1, 1.00])`
	rs = evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != "1" {
		t.Fatalf("min tie: got %s, want 1", got)
	}
}

// Pin: a builtin returning a nil slice yields null, while an empty slice yields
// []. Callers relying on regobrick's adapter must be aware of the asymmetry.
func TestPin_NilSliceNullVsEmptySliceArray(t *testing.T) {
	module := `package test
import rego.v1
result := [pin_nil_slice(), pin_empty_slice()]`
	rs := evalModuleResult(t, module, nil)
	got := requireSingleExprValue(t, rs).([]any)
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %v", got)
	}
	if got[0] != nil {
		t.Fatalf("nil slice: got %v, want null", got[0])
	}
	if arr, ok := got[1].([]any); !ok || len(arr) != 0 {
		t.Fatalf("empty slice: got %v, want []", got[1])
	}
}

// Pin: time.Time crosses the builtin boundary as an RFC3339 string in both
// directions (declared as types.S; conversion via JSON round-trip).
func TestPin_TimeRoundTrip(t *testing.T) {
	module := `package test
import rego.v1
result := pin_time_ret()`
	rs := evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs); got != "2026-01-02T03:04:05Z" {
		t.Fatalf("time return: got %v (%T), want RFC3339 string", got, got)
	}

	module = `package test
import rego.v1
result := pin_time_echo(input.ts)`
	rs = evalModuleResult(t, module, map[string]any{"ts": "2026-01-02T03:04:05Z"})
	if got := requireSingleExprValue(t, rs); got != "2026-01-02T03:04:05Z" {
		t.Fatalf("time arg: got %v (%T), want RFC3339 string", got, got)
	}
}

// Pin: struct arguments/returns cross the boundary as objects keyed by their
// JSON tags (declared as types.A; conversion via JSON round-trip).
func TestPin_StructRoundTrip(t *testing.T) {
	module := `package test
import rego.v1
result := pin_struct_echo({"x": 1, "y": "a"})`
	rs := evalModuleResult(t, module, nil)
	want := map[string]any{"x": json.Number("1"), "y": "a"}
	if got := requireSingleExprValue(t, rs); !reflect.DeepEqual(got, want) {
		t.Fatalf("struct round-trip: got %#v, want %#v", got, want)
	}
}
