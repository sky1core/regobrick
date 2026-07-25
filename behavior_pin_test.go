package regobrick

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
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
	RegisterBuiltin1[any, bool]("pin_any_accepted", func(rego.BuiltinContext, any) (bool, error) {
		return true, nil
	})
	RegisterBuiltin1[[]int64, bool]("pin_int_slice_accepted", func(rego.BuiltinContext, []int64) (bool, error) {
		return true, nil
	})
	RegisterBuiltin1[string, string]("pin_string_echo", func(_ rego.BuiltinContext, s string) (string, error) {
		return s, nil
	})
	RegisterBuiltin1[bool, bool]("pin_bool_echo", func(_ rego.BuiltinContext, b bool) (bool, error) {
		return b, nil
	})
	RegisterBuiltin1[map[string]any, map[string]any]("pin_map_echo", func(_ rego.BuiltinContext, m map[string]any) (map[string]any, error) {
		return m, nil
	})
	RegisterBuiltin1[int, int]("pin_int_echo", func(_ rego.BuiltinContext, n int) (int, error) {
		return n, nil
	})
	RegisterBuiltin1[int64, int64]("pin_int64_echo", func(_ rego.BuiltinContext, n int64) (int64, error) {
		return n, nil
	})
	RegisterBuiltin1[json.Number, json.Number]("pin_json_number_echo", func(_ rego.BuiltinContext, n json.Number) (json.Number, error) {
		return n, nil
	})
	RegisterBuiltin1[Number, Number]("pin_number_echo", func(_ rego.BuiltinContext, n Number) (Number, error) {
		return n, nil
	})
	RegisterBuiltin1[uint64, uint64]("pin_uint64_echo", func(_ rego.BuiltinContext, n uint64) (uint64, error) {
		return n, nil
	})
	// Reports the received uint64 as a string, which crosses the return boundary
	// as a Rego string and so cannot be distorted by numeric conversion.
	RegisterBuiltin1[uint64, string]("pin_uint64_as_string", func(_ rego.BuiltinContext, n uint64) (string, error) {
		return strconv.FormatUint(n, 10), nil
	})
	RegisterBuiltin1[uint64, uint]("pin_uint64_as_uint", func(_ rego.BuiltinContext, n uint64) (uint, error) {
		return uint(n), nil
	})
	RegisterBuiltin1[int8, int8]("pin_int8_echo", func(_ rego.BuiltinContext, n int8) (int8, error) {
		return n, nil
	})
	RegisterBuiltin1[int16, int16]("pin_int16_echo", func(_ rego.BuiltinContext, n int16) (int16, error) {
		return n, nil
	})
	RegisterBuiltin1[int32, int32]("pin_int32_echo", func(_ rego.BuiltinContext, n int32) (int32, error) {
		return n, nil
	})
	RegisterBuiltin1[uint8, uint8]("pin_uint8_echo", func(_ rego.BuiltinContext, n uint8) (uint8, error) {
		return n, nil
	})
	RegisterBuiltin1[uint16, uint16]("pin_uint16_echo", func(_ rego.BuiltinContext, n uint16) (uint16, error) {
		return n, nil
	})
	RegisterBuiltin1[uint32, uint32]("pin_uint32_echo", func(_ rego.BuiltinContext, n uint32) (uint32, error) {
		return n, nil
	})
	RegisterBuiltin0[float32]("pin_float32_max", func(rego.BuiltinContext) (float32, error) {
		return math.MaxFloat32, nil
	})
	// float64(9007199254740993) is 2^53+1, which float64 cannot represent; the
	// constant conversion already rounds it down to 2^53.
	RegisterBuiltin0[float64]("pin_float_beyond_int53", func(rego.BuiltinContext) (float64, error) {
		return float64(9007199254740993), nil
	})
}

// requirePinBuiltinError asserts that err is the eval_builtin_error a custom
// builtin raises under StrictBuiltinErrors(true), carrying wantMsg.
func requirePinBuiltinError(t *testing.T, err error, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected eval error containing %q, got none", wantMsg)
	}
	var topdownErr *topdown.Error
	if !errors.As(err, &topdownErr) {
		t.Fatalf("expected topdown.Error, got %T: %v", err, err)
	}
	if topdownErr.Code != topdown.BuiltinErr {
		t.Fatalf("expected error code %s, got %s (%v)", topdown.BuiltinErr, topdownErr.Code, err)
	}
	if !strings.Contains(err.Error(), wantMsg) {
		t.Fatalf("expected message to contain %q, got: %v", wantMsg, err)
	}
}

// Pin: with decimal arithmetic, 0.1 + 0.2 evaluates to exactly 0.3, and it is
// that normalized result which composite (array/object) equality goes on to
// compare, so these compare equal.
//
// What carries this pin is the arithmetic, not the equality. A composite operand
// is not an ast.Number, so precisionEqual (operator.go) hands the pair to OPA's
// own structural comparison rather than comparing element-wise as decimals:
// dropping the == override leaves both cases below true, while changing the +
// override breaks them.
//
// These operands do NOT distinguish decimal from standard OPA, and the pin does
// not claim they do: standard OPA also returns exactly 0.3 for 0.1 + 0.2 and also
// answers true for both cases below. The operands that diverge are 1.1 + 2.2 and
// 3.3, where standard OPA's big.Float residue (3.3000000000000000002, pinned by
// TestStandardArithmeticIsNotOverridden in internal/opaerrparity) makes
// [1.1 + 2.2] == [3.3] false while decimal arithmetic keeps it true.
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

// Pin: scalar != follows == at the precision limit. A number beyond udecimal's
// 19-dp precision fails to parse, so the comparison is undefined rather than
// true. Standard OPA would return true (the values genuinely differ), which is
// also what makes this the case that notices != losing its decimal override.
func TestPin_ScalarInequalityBeyondPrecisionIsUndefined(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
import rego.v1
result := input.a != 0.3`
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

// Pin: magnitude is NOT capped at the u128 fast-path bound. Plain-notation
// values beyond 2^128 parse and compute exactly via udecimal's big.Int
// fallback; only the 19-decimal-place scale is a hard limit. Exponent notation
// is additionally subject to the 64-character expansion budget: 1e30 works,
// 1e100 is undefined.
func TestPin_LargeMagnitudeNumbers(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// 2^128 + 1: beyond the u128 coefficient fast path, still exact.
	module := `package test
import rego.v1
result := input.a + 1`
	rs := evalModuleResult(t, module, map[string]any{
		"a": Number("340282366920938463463374607431768211456"),
	})
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != "340282366920938463463374607431768211457" {
		t.Fatalf("2^128 + 1: got %s", got)
	}

	// 1e30 expands within the 64-character budget and computes exactly.
	module = `package test
import rego.v1
result := 1e30 + 1`
	rs = evalModuleResult(t, module, nil)
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != "1000000000000000000000000000001" {
		t.Fatalf("1e30 + 1: got %s", got)
	}

	// Expansion-budget boundary: 1e61 (62 digits) still parses, 1e62 is the
	// first magnitude to exceed the budget.
	module = `package test
import rego.v1
result := input.a + 1`
	rs = evalModuleResult(t, module, map[string]any{"a": Number("1e61")})
	want := "1" + strings.Repeat("0", 60) + "1"
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != want {
		t.Fatalf("1e61 + 1: got %s, want %s", got, want)
	}
	rs = evalModuleResult(t, module, map[string]any{"a": Number("1e62")})
	requireUndefinedResult(t, rs)

	// 1e100 exceeds the expansion budget: undefined instead of a huge result.
	rs = evalModuleResult(t, module, map[string]any{"a": Number("1e100")})
	requireUndefinedResult(t, rs)

	// Plain-notation input length boundary: udecimal caps number strings at 200
	// characters. 200 digits parse (big.Int fallback, exact), 201 are undefined.
	d200 := strings.Repeat("9", 200)
	rs = evalModuleResult(t, module, map[string]any{"a": Number(d200)})
	want = "1" + strings.Repeat("0", 200) // (10^200 - 1) + 1 = 10^200
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != want {
		t.Fatalf("200-digit + 1: got %s, want %s", got, want)
	}
	rs = evalModuleResult(t, module, map[string]any{"a": Number(strings.Repeat("9", 201))})
	requireUndefinedResult(t, rs)
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

// The remaining pins in this file cover the custom builtin conversion boundary.
//
// The argument path converts by serializing each argument to its Rego text
// (Value.String()) and decoding that text as JSON. Two Rego values have a Rego
// text that is not JSON — sets and strings holding control characters — so they
// cannot reach a builtin at all, whatever Go type the parameter declares. That
// is a limit of the conversion route, not a decision about what builtins should
// accept.
//
// These are pinned so the limit stays visible and documented (see README,
// "Writing Custom Builtins" → Known limitations). If a conversion path that
// walks the AST directly is ever adopted, these tests are expected to fail;
// that is the moment to update them deliberately, together with the README
// contract, rather than a reason to weaken them now.
//
// The asserted messages ("... looking for beginning of object key string",
// "... in string escape code", "json: cannot unmarshal ...") belong to Go's
// encoding/json, not to regobrick or OPA, so a Go toolchain upgrade can reword
// them while the contract stands (encoding/json v2 is in progress). The same
// coupling to a foreign error string is what internal/opaerrparity exists to
// watch on the OPA side. On failure, separate the two cases before touching an
// expectation: if the argument still fails to convert and only the wording moved,
// update the literal and say so; if such an argument now converts, the
// documented contract changed and the README has to change with it.

// Pin: a Rego set cannot be passed to a custom builtin. Its Rego text
// (`{1, 2, 3}`) is not valid JSON, so the argument fails to convert — default
// mode: no result; StrictBuiltinErrors(true): eval_builtin_error.
func TestPin_SetArgumentIsUnconvertible(t *testing.T) {
	// The JSON scanner reads `{` as the start of an object and then rejects the
	// first element where an object key would have to be.
	const wantMsg = "invalid character '1' looking for beginning of object key string"

	setModules := map[string]string{
		// Declared parameter type is irrelevant: the failure happens while
		// decoding the serialized argument, before any Go type is involved.
		"any_param": `package test
import rego.v1
result := pin_any_accepted({1, 2, 3})`,
		"slice_param": `package test
import rego.v1
result := pin_int_slice_accepted({1, 2, 3})`,
		// A set nested inside an otherwise convertible argument fails the same
		// way: the whole argument is serialized as one text.
		"nested_in_object": `package test
import rego.v1
result := pin_any_accepted({"a": {1, 2}})`,
		// Not a syntactic property of set literals either — a set built by a
		// comprehension at runtime fails identically.
		"set_comprehension": `package test
import rego.v1
result := pin_any_accepted({x | x := [1, 2][_]})`,
	}

	for name, module := range setModules {
		t.Run(name, func(t *testing.T) {
			requireUndefinedResult(t, evalModuleResult(t, module, nil))

			_, err := evalModule(t, module, nil, rego.StrictBuiltinErrors(true))
			requirePinBuiltinError(t, err, wantMsg)
		})
	}

	// Contrast: the same parameters take arrays and objects. Only sets are
	// excluded, so passing a set as an array is the available workaround — the
	// []int64 parameter that rejected {1, 2, 3} accepts [1, 2, 3].
	accepted := map[string]string{
		"pin_any_accepted":       `[1, 2, 3]`,
		"pin_int_slice_accepted": `[1, 2, 3]`,
	}
	for name, arg := range accepted {
		module := "package test\nimport rego.v1\nresult := " + name + "(" + arg + ")"
		if got := requireSingleExprValue(t, evalModuleResult(t, module, nil)); got != true {
			t.Fatalf("%s(%s): got %v, want true", name, arg, got)
		}
	}
	objectArg := `package test
import rego.v1
result := pin_any_accepted({"a": 1})`
	if got := requireSingleExprValue(t, evalModuleResult(t, objectArg, nil)); got != true {
		t.Fatalf("object argument: got %v, want true", got)
	}

	// The workaround documented in the README: rebuild the set as an array.
	workaround := `package test
import rego.v1
s := {1, 2, 3}
result := pin_int_slice_accepted([x | x := s[_]])`
	if got := requireSingleExprValue(t, evalModuleResult(t, workaround, nil)); got != true {
		t.Fatalf("set-to-array workaround: got %v, want true", got)
	}
}

// Pin: a string argument holding a control character cannot reach a custom
// builtin. Rego text for strings is Go-quoted (`"\x01"`, `"\a"`), and those
// escapes are not valid JSON — default mode: no result;
// StrictBuiltinErrors(true): eval_builtin_error.
func TestPin_ControlCharacterStringArgumentIsUnconvertible(t *testing.T) {
	cases := []struct {
		name    string
		module  string
		input   any
		wantMsg string
	}{
		{
			// U+0001 is quoted as \x01; JSON has no \x escape.
			name: "u0001_literal",
			module: `package test
import rego.v1
result := pin_string_echo("\u0001")`,
			wantMsg: "invalid character 'x' in string escape code",
		},
		{
			// Reaching the builtin from input is no different.
			name: "u0001_from_input",
			module: `package test
import rego.v1
result := pin_string_echo(input.s)`,
			input:   map[string]any{"s": "\u0001"},
			wantMsg: "invalid character 'x' in string escape code",
		},
		{
			// U+0007 is quoted as \a, an escape Go has and JSON does not.
			name: "bell_literal",
			module: `package test
import rego.v1
result := pin_string_echo("\u0007")`,
			wantMsg: "invalid character 'a' in string escape code",
		},
		{
			// Nested inside a composite argument, same failure.
			name: "nested_in_object",
			module: `package test
import rego.v1
result := pin_any_accepted({"a": "\u0001"})`,
			wantMsg: "invalid character 'x' in string escape code",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireUndefinedResult(t, evalModuleResult(t, tc.module, tc.input))

			_, err := evalModule(t, tc.module, tc.input, rego.StrictBuiltinErrors(true))
			requirePinBuiltinError(t, err, tc.wantMsg)
		})
	}

	// Contrast: escapes JSON shares with Go (\n) and printable non-ASCII text
	// cross the boundary unchanged. The limitation is control characters, not
	// escaping in general.
	for _, tc := range []struct{ module, want string }{
		{`package test
import rego.v1
result := pin_string_echo("a\nb")`, "a\nb"},
		{`package test
import rego.v1
result := pin_string_echo("café")`, "café"},
	} {
		if got := requireSingleExprValue(t, evalModuleResult(t, tc.module, nil)); got != tc.want {
			t.Fatalf("string round-trip: got %q, want %q", got, tc.want)
		}
	}
}

// Pin: a number with a fractional part is rejected by an integer parameter
// rather than truncated — default mode: no result; StrictBuiltinErrors(true):
// eval_builtin_error. OPA's compile-time type check does not catch this: both
// 1 and 1.5 are types.N.
func TestPin_FractionalNumberIntoIntArgumentFails(t *testing.T) {
	module := `package test
import rego.v1
result := pin_int_echo(1.5)`

	requireUndefinedResult(t, evalModuleResult(t, module, nil))

	_, err := evalModule(t, module, nil, rego.StrictBuiltinErrors(true))
	requirePinBuiltinError(t, err, "json: cannot unmarshal number 1.5 into Go value of type int")
}

// Pin: a float64 return is formatted with strconv.FormatFloat(f, 'g', -1, 64),
// so large magnitudes come back in exponent notation rather than as plain
// digits. The value is also already rounded to float64 precision before it is
// formatted (2^53+1 arrives as 2^53), which is a property of float64 itself,
// not of the conversion.
func TestPin_Float64ReturnUsesExponentNotation(t *testing.T) {
	module := `package test
import rego.v1
result := pin_float_beyond_int53()`
	got, ok := requireSingleExprValue(t, evalModuleResult(t, module, nil)).(json.Number)
	if !ok {
		t.Fatalf("expected json.Number result")
	}
	if got.String() != "9.007199254740992e+15" {
		t.Fatalf("float64 return: got %s, want 9.007199254740992e+15", got.String())
	}
}

// Pin: a float32 return does NOT use exponent notation, however large it is —
// the exponent form is float64's alone. The two types take different routes:
// ast.InterfaceToValue has a `case float64` that formats with
// strconv.FormatFloat(f, 'g', -1, 64), while float32 has no case there and falls
// through to util.RoundTrip, which formats with 'f' (OPA v1.11.0,
// v1/util/json.go — `case float32: strconv.FormatFloat(float64(v), 'f', -1, 32)`).
//
// Pinned because the README states the two notations per row, and nothing but
// this test would notice if a float32 started serializing like a float64.
func TestPin_Float32ReturnUsesPlainNotation(t *testing.T) {
	module := `package test
import rego.v1
result := pin_float32_max()`
	got, ok := requireSingleExprValue(t, evalModuleResult(t, module, nil)).(json.Number)
	if !ok {
		t.Fatalf("expected json.Number result")
	}
	const want = "340282350000000000000000000000000000000"
	if got.String() != want {
		t.Fatalf("float32 return: got %s, want %s", got.String(), want)
	}
}

// Pin: a uint64 RETURN above math.MaxInt64 comes back as a negative number,
// while the same value passed as an ARGUMENT arrives intact.
//
// The distortion is upstream and unconditional: ast.InterfaceToValue routes
// uint64 through ast.InternedValueOr, whose own switch narrows it before the
// exact supplier can run (OPA v1.11.0, ast/interning.go:154-155 —
// `case uint64: return internedIntNumberValue(int(value))`), so the two's
// complement bit pattern is reinterpreted as a signed int. regobrick's return
// path (internal/builtin.toTerm) hands the value over unchanged and has no say
// in it. Notably, uint takes a different route — InterfaceToValue has no `case
// uint`, so it falls through to the JSON round-trip and stays exact — which is
// why the README carve-out names uint64 specifically.
//
// This freezes upstream behavior, not an intended contract. An OPA release that
// stopped narrowing would return the exact value and fail this pin; that is the
// signal to update the pin and drop the limitation from the README, not to
// weaken the pin now.
func TestPin_Uint64ReturnWrapsAboveMaxInt64(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skipf("pinned values assume a 64-bit int/uint, got %d-bit", strconv.IntSize)
	}

	cases := []struct {
		name string
		in   string
		// wantUint64Return is the uint64 value after it crossed back out.
		wantUint64Return string
		// wantStringReturn is the same value reported as a string, which shows
		// what the builtin actually received.
		wantStringReturn string
		// wantUintReturn is the same value returned as uint instead of uint64.
		wantUintReturn string
	}{
		{
			name:             "zero",
			in:               "0",
			wantUint64Return: "0",
			wantStringReturn: "0",
			wantUintReturn:   "0",
		},
		{
			name:             "max_int64_is_still_exact",
			in:               "9223372036854775807",
			wantUint64Return: "9223372036854775807",
			wantStringReturn: "9223372036854775807",
			wantUintReturn:   "9223372036854775807",
		},
		{
			name:             "max_int64_plus_one_wraps_to_min_int64",
			in:               "9223372036854775808",
			wantUint64Return: "-9223372036854775808",
			wantStringReturn: "9223372036854775808",
			wantUintReturn:   "9223372036854775808",
		},
		{
			name:             "max_uint64_wraps_to_minus_one",
			in:               "18446744073709551615",
			wantUint64Return: "-1",
			wantStringReturn: "18446744073709551615",
			wantUintReturn:   "18446744073709551615",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]any{"n": Number(tc.in)}

			numeric := `package test
import rego.v1
result := pin_uint64_echo(input.n)`
			got, ok := requireSingleExprValue(t, evalModuleResult(t, numeric, input)).(json.Number)
			if !ok {
				t.Fatalf("expected json.Number result")
			}
			if got.String() != tc.wantUint64Return {
				t.Fatalf("uint64 return for %s: got %s, want %s", tc.in, got.String(), tc.wantUint64Return)
			}

			asString := `package test
import rego.v1
result := pin_uint64_as_string(input.n)`
			if got := requireSingleExprValue(t, evalModuleResult(t, asString, input)); got != tc.wantStringReturn {
				t.Fatalf("uint64 argument for %s: builtin received %v, want %s", tc.in, got, tc.wantStringReturn)
			}

			asUint := `package test
import rego.v1
result := pin_uint64_as_uint(input.n)`
			gotUint, ok := requireSingleExprValue(t, evalModuleResult(t, asUint, input)).(json.Number)
			if !ok {
				t.Fatalf("expected json.Number result")
			}
			if gotUint.String() != tc.wantUintReturn {
				t.Fatalf("uint return for %s: got %s, want %s", tc.in, gotUint.String(), tc.wantUintReturn)
			}
		})
	}
}

// Pin: the sized integer types carry their range boundaries exactly, in both
// directions. Each case sends the boundary in as an argument and reads back what
// the builtin returned, so a distortion on either leg shows up here.
//
// This exists because "integers are exact" turned out not to be a property of
// the integer types but of ast.InterfaceToValue's per-type conversion table:
// uint64 alone is narrowed to a signed int there and wraps
// (TestPin_Uint64ReturnWrapsAboveMaxInt64, which also carries the uint contrast
// axis — uint is deliberately not repeated here). An OPA release that reworks
// that table can silently move any other type into the same failure mode, and
// only these boundaries would notice.
func TestPin_SizedIntegerBoundariesAreExact(t *testing.T) {
	cases := []struct {
		builtin string
		values  []string
	}{
		{"pin_int8_echo", []string{"-128", "127"}},
		{"pin_int16_echo", []string{"-32768", "32767"}},
		{"pin_int32_echo", []string{"-2147483648", "2147483647"}},
		{"pin_uint8_echo", []string{"0", "255"}},
		{"pin_uint16_echo", []string{"0", "65535"}},
		{"pin_uint32_echo", []string{"0", "4294967295"}},
		{"pin_int64_echo", []string{"-9223372036854775808", "9223372036854775807"}},
	}

	for _, tc := range cases {
		for _, want := range tc.values {
			t.Run(tc.builtin+"/"+want, func(t *testing.T) {
				module := "package test\nimport rego.v1\nresult := " + tc.builtin + "(input.n)"
				input := map[string]any{"n": Number(want)}
				got, ok := requireSingleExprValue(t, evalModuleResult(t, module, input)).(json.Number)
				if !ok {
					t.Fatalf("expected json.Number result")
				}
				if got.String() != want {
					t.Fatalf("%s round-trip: got %s, want %s", tc.builtin, got.String(), want)
				}
			})
		}
	}
}

// Pin: the two type-mapping rows no other test exercises on the argument side —
// a bool parameter and a map[string]T parameter. The remaining rows are pinned
// by TestPin_StructRoundTrip, TestPin_TimeRoundTrip,
// TestPin_NilSliceNullVsEmptySliceArray, TestPin_SetArgumentIsUnconvertible
// (array and string arguments), TestPin_IntegerAndNumberArgumentsAreExact and
// TestPin_Float64ReturnUsesExponentNotation.
func TestPin_BoolAndMapArgumentRoundTrip(t *testing.T) {
	boolArg := `package test
import rego.v1
result := pin_bool_echo(true)`
	if got := requireSingleExprValue(t, evalModuleResult(t, boolArg, nil)); got != true {
		t.Fatalf("bool round-trip: got %v (%T), want true", got, got)
	}

	mapArg := `package test
import rego.v1
result := pin_map_echo({"a": 1, "b": "x"})`
	want := map[string]any{"a": json.Number("1"), "b": "x"}
	if got := requireSingleExprValue(t, evalModuleResult(t, mapArg, nil)); !reflect.DeepEqual(got, want) {
		t.Fatalf("map round-trip: got %#v, want %#v", got, want)
	}
}

// Pin: integer and decimal-string parameter types carry their digits exactly.
// This is the counterpart to the float64 pin above and backs the README
// recommendation to declare json.Number / regobrick.Number when exact decimal
// digits matter: neither the argument nor the return goes through float64.
func TestPin_IntegerAndNumberArgumentsAreExact(t *testing.T) {
	cases := []struct {
		name   string
		module string
		input  any
		want   string
	}{
		{
			// 2^62 + 1: beyond float64's exact-integer range.
			name: "int64_beyond_float64_exact_range",
			module: `package test
import rego.v1
result := pin_int64_echo(input.n)`,
			input: map[string]any{"n": Number("4611686018427387905")},
			want:  "4611686018427387905",
		},
		{
			name: "json_number_19_decimal_places",
			module: `package test
import rego.v1
result := pin_json_number_echo(input.n)`,
			input: map[string]any{"n": Number("0.1234567890123456789")},
			want:  "0.1234567890123456789",
		},
		{
			name: "regobrick_number_19_decimal_places",
			module: `package test
import rego.v1
result := pin_number_echo(input.n)`,
			input: map[string]any{"n": Number("0.1234567890123456789")},
			want:  "0.1234567890123456789",
		},
		{
			name: "regobrick_number_beyond_float64_exact_range",
			module: `package test
import rego.v1
result := pin_number_echo(input.n)`,
			input: map[string]any{"n": Number("4611686018427387905")},
			want:  "4611686018427387905",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := requireSingleExprValue(t, evalModuleResult(t, tc.module, tc.input)).(json.Number)
			if !ok {
				t.Fatalf("expected json.Number result")
			}
			if got.String() != tc.want {
				t.Fatalf("round-trip: got %s, want %s", got.String(), tc.want)
			}
		})
	}
}
