package regobrick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// =============================================================================
// Number unit tests (Scan, Value, JSON)
// =============================================================================

func TestNumber_Scan_String(t *testing.T) {
	var n Number
	err := n.Scan("123.456789012345678901234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "123.456789012345678901234567890" {
		t.Errorf("got %s, want 123.456789012345678901234567890", n.String())
	}
}

func TestNumber_Scan_Bytes(t *testing.T) {
	var n Number
	err := n.Scan([]byte("987.654321098765432109876543210"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "987.654321098765432109876543210" {
		t.Errorf("got %s, want 987.654321098765432109876543210", n.String())
	}
}

func TestNumber_Scan_EmptyBytes(t *testing.T) {
	var n Number
	err := n.Scan([]byte{})
	if err == nil {
		t.Fatal("expected error for empty bytes, got nil")
	}
}

func TestNumber_Scan_Nil(t *testing.T) {
	var n Number
	err := n.Scan(nil)
	if err == nil {
		t.Fatal("expected error for NULL, got nil")
	}
}

func TestNumber_Scan_Float64(t *testing.T) {
	var n Number
	err := n.Scan(float64(123.456))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "123.456" {
		t.Errorf("got %s, want 123.456", n.String())
	}
}

func TestNumber_Scan_Float64_NaN(t *testing.T) {
	var n Number
	err := n.Scan(math.NaN())
	if err == nil {
		t.Fatal("expected error for NaN, got nil")
	}
	if !strings.Contains(err.Error(), "NaN") {
		t.Fatalf("expected NaN error, got %v", err)
	}
	if n.String() != "" {
		t.Errorf("Number should not be mutated on NaN, got %q", n.String())
	}
}

func TestNumber_Scan_Float64_PosInf(t *testing.T) {
	var n Number
	err := n.Scan(math.Inf(1))
	if err == nil {
		t.Fatal("expected error for +Inf, got nil")
	}
	if !strings.Contains(err.Error(), "Inf") {
		t.Fatalf("expected Inf error, got %v", err)
	}
	if n.String() != "" {
		t.Errorf("Number should not be mutated on +Inf, got %q", n.String())
	}
}

func TestNumber_Scan_Float64_NegInf(t *testing.T) {
	var n Number
	err := n.Scan(math.Inf(-1))
	if err == nil {
		t.Fatal("expected error for -Inf, got nil")
	}
	if !strings.Contains(err.Error(), "Inf") {
		t.Fatalf("expected Inf error, got %v", err)
	}
	if n.String() != "" {
		t.Errorf("Number should not be mutated on -Inf, got %q", n.String())
	}
}

func TestNumber_Scan_Int64(t *testing.T) {
	var n Number
	err := n.Scan(int64(12345))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "12345" {
		t.Errorf("got %s, want 12345", n.String())
	}
}

func TestNumber_Scan_UnsupportedType(t *testing.T) {
	var n Number
	err := n.Scan(struct{}{})
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}

func TestNumber_Value(t *testing.T) {
	n := Number("123.456789012345678901234567890")
	val, err := n.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if s != "123.456789012345678901234567890" {
		t.Errorf("got %s, want 123.456789012345678901234567890", s)
	}
}

func TestNumber_Scan_EmptyString(t *testing.T) {
	var n Number
	err := n.Scan("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestNumber_Value_EmptyString(t *testing.T) {
	var n Number
	_, err := n.Value()
	if err == nil {
		t.Fatal("expected error for empty value, got nil")
	}
}

func TestNumber_MarshalJSON_EmptyString(t *testing.T) {
	// json.Number outputs an empty string as 0 (Go convention)
	var n Number
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(b) != "0" {
		t.Errorf("expected 0, got %s", b)
	}
}

func TestNumber_UnmarshalJSON_Null(t *testing.T) {
	// json.Number treats null as an empty string.
	// An empty string is output as 0 by MarshalJSON.
	var n Number
	err := json.Unmarshal([]byte("null"), &n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "" {
		t.Errorf("expected empty string, got %q", n.String())
	}
	// empty string -> output as 0 (Go convention)
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(b) != "0" {
		t.Errorf("expected 0, got %s", b)
	}
}

func TestNumber_MarshalJSON(t *testing.T) {
	n := Number("123.456")
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// numeric literal (without quotes)
	if string(data) != "123.456" {
		t.Errorf("got %s, want 123.456", string(data))
	}
}

func TestNumber_MarshalJSON_InStruct(t *testing.T) {
	type Record struct {
		Price Number `json:"price"`
	}
	r := Record{Price: Number("999.888")}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"price":999.888}` {
		t.Errorf("got %s, want {\"price\":999.888}", string(data))
	}
}

func TestNumber_UnmarshalJSON(t *testing.T) {
	var n Number
	err := json.Unmarshal([]byte("456.789"), &n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "456.789" {
		t.Errorf("got %s, want 456.789", n.String())
	}
}

func TestNumber_UnmarshalJSON_InvalidToken(t *testing.T) {
	var n Number
	err := json.Unmarshal([]byte(`"not-a-number"`), &n)
	if err == nil {
		t.Fatal("expected error for invalid numeric token, got nil")
	}
}

func TestNumber_UnmarshalJSON_InStruct(t *testing.T) {
	type Record struct {
		Price Number `json:"price"`
	}
	var r Record
	err := json.Unmarshal([]byte(`{"price": 111.222}`), &r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Price.String() != "111.222" {
		t.Errorf("got %s, want 111.222", r.Price.String())
	}
}

func TestNumber_UnmarshalJSON_Precision(t *testing.T) {
	// Verify that JSON Unmarshal preserves precision without going through float64
	type Record struct {
		Val Number `json:"val"`
	}
	// 38 digits - float64 can only represent about 15 to 17 digits
	input := `{"val": 12345678901234567890123456789012345678.12}`
	var r Record
	err := json.Unmarshal([]byte(input), &r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// precision must be preserved
	expected := "12345678901234567890123456789012345678.12"
	if r.Val.String() != expected {
		t.Errorf("precision loss: got %s, want %s", r.Val.String(), expected)
	}
}

func TestNumber_Roundtrip_ScanValue(t *testing.T) {
	// Scan -> Value roundtrip test
	precision := "123456789012345678901234567890.12345678901234567890"
	var n Number
	if err := n.Scan(precision); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	val, err := n.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if val != precision {
		t.Errorf("roundtrip failed: got %s, want %s", val, precision)
	}
}

func TestNumber_Roundtrip_JSON(t *testing.T) {
	// Marshal -> Unmarshal roundtrip test
	type Record struct {
		Price Number `json:"price"`
	}
	orig := Record{Price: Number("999.888777666555444333")}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Record
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if orig.Price.String() != parsed.Price.String() {
		t.Errorf("roundtrip failed: got %s, want %s", parsed.Price.String(), orig.Price.String())
	}
}

// =============================================================================
// Rego integration tests (existing)
// =============================================================================

func TestNumber_WithDecimalOperators(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Verify that Rego operations work correctly with Number input
	tests := []struct {
		name     string
		a, b     Number
		op       string
		expected string
	}{
		{"add", Number("1.1"), Number("2.2"), "+", "3.3"},
		{"sub", Number("5.5"), Number("2.2"), "-", "3.3"},
		{"mul", Number("2.5"), Number("4"), "*", "10"},
		{"div", Number("10"), Number("4"), "/", "2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
result := input.a ` + tt.op + ` input.b
`
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			input := map[string]any{
				"a": tt.a,
				"b": tt.b,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result := rs[0].Expressions[0].Value.(json.Number).String()
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestNumber_ExponentNotation(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Exponent notation (e.g. "1e-8") is expanded into plain decimal notation
	// before udecimal parsing, so it works the same as standard OPA (1e-8 + 1 == 1.00000001).
	// However, if the expanded result exceeds udecimal's precision (19 digits after the decimal point), it still fails.

	module := `package test
result := input.a + input.b
`
	ctx := context.Background()

	t.Run("in_precision_succeeds", func(t *testing.T) {
		input := map[string]any{
			"a": Number("1e-8"), // 8 decimal places -> within precision
			"b": Number("1"),
		}
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		rs, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			t.Fatalf("unexpected eval error: %v", err)
		}
		if len(rs) == 0 || len(rs[0].Expressions) == 0 {
			t.Fatal("expected result for exponent notation, got none")
		}
		if got := fmt.Sprintf("%v", rs[0].Expressions[0].Value); got != "1.00000001" {
			t.Errorf("expected 1.00000001, got %s", got)
		}
	})

	t.Run("out_of_precision_default_mode", func(t *testing.T) {
		// 1e-25 expands to 25 decimal places, exceeding udecimal's precision (19 digits) -> fails
		input := map[string]any{
			"a": Number("1e-25"),
			"b": Number("1"),
		}
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		rs, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			t.Fatalf("unexpected eval error: %v", err)
		}
		if len(rs) > 0 && len(rs[0].Expressions) > 0 {
			t.Error("expected no result for out-of-precision exponent, but got result")
		}
	})

	t.Run("out_of_precision_strict", func(t *testing.T) {
		input := map[string]any{
			"a": Number("1e-25"),
			"b": Number("1"),
		}
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		_, err = query.Eval(ctx, rego.EvalInput(input))
		if err == nil {
			t.Fatal("expected eval error for out-of-precision exponent, but got none")
		}
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Errorf("expected error code %s, got %s", topdown.BuiltinErr, topdownErr.Code)
		}
	})
}

func TestNumber_LeadingZero_Error(t *testing.T) {
	// Leading zeros ("01", "007", etc.) are invalid per the JSON spec.
	//
	// Behavior by layer:
	// - udecimal.Parse("01") -> succeeds (udecimal allows leading zeros)
	// - ast.InterfaceToValue -> succeeds (OPA does not validate)
	// - encoding/json.Marshal -> fails (Go JSON blocks it)
	//
	// Conclusion: the error occurs in Go's json.Marshal during rego.EvalInput processing
	// Note: exponent notation ("1e-8") is valid JSON -> passes OPA -> parses correctly after decimal expansion

	module := `package test
result := input.a + input.b
`
	ctx := context.Background()
	input := map[string]any{
		"a": Number("01"), // fails in Go json.Marshal
		"b": Number("1"),
	}

	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	// Go encoding/json rejects the leading zero while marshaling the input.
	// The failure must originate from the json.Marshal layer (not udecimal or
	// OPA), so the error message identifies the invalid numeric literal "01".
	_, err = query.Eval(ctx, rego.EvalInput(input))
	if err == nil {
		t.Fatal("expected json.Marshal error due to leading zero, but got none")
	}
	if !strings.Contains(err.Error(), "invalid number literal") {
		t.Errorf("expected a json.Marshal invalid-number-literal error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "01") {
		t.Errorf("expected error to mention the offending literal %q, got: %v", "01", err)
	}
}

func TestNumber_Comparison(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		a, b     Number
		op       string
		expected bool
	}{
		{"gt_true", Number("3.3"), Number("2.2"), ">", true},
		{"gt_false", Number("2.2"), Number("3.3"), ">", false},
		{"eq_true", Number("3.3"), Number("3.3"), "==", true},
		{"eq_false", Number("3.3"), Number("2.2"), "==", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
result := input.a ` + tt.op + ` input.b
`
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			input := map[string]any{
				"a": tt.a,
				"b": tt.b,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result := rs[0].Expressions[0].Value.(bool)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNumber_UnaryOperations(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		input    Number
		op       string
		expected string
	}{
		{"abs_neg", Number("-3.3"), "abs(input.n)", "3.3"},
		{"round", Number("3.5"), "round(input.n)", "4"},
		{"ceil", Number("3.1"), "ceil(input.n)", "4"},
		{"floor", Number("3.9"), "floor(input.n)", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
result := ` + tt.op + `
`
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			input := map[string]any{
				"n": tt.input,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result := rs[0].Expressions[0].Value.(json.Number).String()
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestNumber_FormatVariations(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Verify that different Number representations are handled consistently.
	// udecimal.Parse normalizes values, so equivalent values must compare equal.

	// Comparison tests
	comparisonTests := []struct {
		name   string
		a, b   Number
		wantEq bool
	}{
		{"int_vs_decimal", Number("1"), Number("1.0"), true},
		{"int_vs_decimal_zeros", Number("1"), Number("1.00"), true},
		{"decimal_zeros", Number("1.0"), Number("1.00"), true},
		{"trailing_zeros", Number("0.5"), Number("0.50"), true},
		{"trailing_zeros2", Number("0.500"), Number("0.5"), true},
		{"zero_vs_neg_zero", Number("0"), Number("-0"), true},
		{"zero_variations", Number("0.0"), Number("0.00"), true},
	}

	for _, tt := range comparisonTests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
result := input.a == input.b
`
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			input := map[string]any{"a": tt.a, "b": tt.b}
			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result := rs[0].Expressions[0].Value.(bool)
			if result != tt.wantEq {
				t.Errorf("got %v, want %v", result, tt.wantEq)
			}
		})
	}

	// Arithmetic tests (with explicit expected results)
	arithmeticTests := []struct {
		name       string
		a, b       Number
		op         string
		wantResult string
	}{
		{"add_format", Number("1.0"), Number("1"), "+", "2"},
		{"add_decimals", Number("1.5"), Number("2.5"), "+", "4"},
		{"sub_format", Number("3.0"), Number("1"), "-", "2"},
	}

	for _, tt := range arithmeticTests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
result := input.a ` + tt.op + ` input.b
`
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			input := map[string]any{"a": tt.a, "b": tt.b}
			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result := rs[0].Expressions[0].Value.(json.Number)
			if result.String() != tt.wantResult {
				t.Errorf("got %s, want %s", result.String(), tt.wantResult)
			}
		})
	}
}
