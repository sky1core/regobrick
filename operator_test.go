package regobrick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

var operatorOnce sync.Once

func ensureDecimalArithmeticEnabled() {
	operatorOnce.Do(func() { UseDecimalArithmetic() })
}

func evalModuleResult(t *testing.T, module string, input any, options ...func(*rego.Rego)) rego.ResultSet {
	t.Helper()

	rs, err := evalModule(t, module, input, options...)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return rs
}

func evalModule(t *testing.T, module string, input any, options ...func(*rego.Rego)) (rego.ResultSet, error) {
	t.Helper()

	ctx := context.Background()
	args := []func(*rego.Rego){
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	}
	if input != nil {
		args = append(args, rego.Input(input))
	}
	args = append(args, options...)

	query, err := rego.New(args...).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	return query.Eval(ctx)
}

func requireSingleExprValue(t *testing.T, rs rego.ResultSet) interface{} {
	t.Helper()
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected single result, got none")
	}
	return rs[0].Expressions[0].Value
}

func requireUndefinedResult(t *testing.T, rs rego.ResultSet) {
	t.Helper()
	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		t.Fatalf("expected undefined result, got %v", rs)
	}
}

func TestDecimalOperators_Arithmetic(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"plus", "1.1 + 2.2", "3.3"},
		{"minus", "5.5 - 2.2", "3.3"},
		{"multiply", "2.5 * 4", "10"},
		{"divide", "10 / 4", "2.5"},
		{"remainder", "10 % 3", "1"},
		{"remainder_float", "10.5 % 3", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_DivideByZero(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
result := 10 / 0
`
	ctx := context.Background()

	t.Run("default_mode", func(t *testing.T) {
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Fatalf("unexpected eval error: %v", err)
		}

		// Default mode: no result when the operation fails (not an error)
		if len(rs) > 0 && len(rs[0].Expressions) > 0 {
			t.Error("expected no result for divide by zero, but got result")
		}
	})

	t.Run("strict_builtin_errors", func(t *testing.T) {
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		_, err = query.Eval(ctx)
		// Strict mode: the error propagates upward
		if err == nil {
			t.Fatal("expected eval error for divide by zero, but got none")
		}

		// Step 1: check the typed error code (like standard OPA, divide by zero is BuiltinErr)
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Errorf("expected error code %s, got %s", topdown.BuiltinErr, topdownErr.Code)
		}
		// Step 2: check the message (same "div: divide by zero" as standard OPA)
		if !strings.Contains(err.Error(), "div: divide by zero") {
			t.Errorf("expected message to contain %q, got: %v", "div: divide by zero", err)
		}
	})
}

func TestDecimalOperators_ModuloByZero(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Default mode: modulo by zero returns undefined
	t.Run("default_mode", func(t *testing.T) {
		module := `package test
result := 10 % 0
`
		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Fatalf("unexpected error in default mode: %v", err)
		}

		// Default mode: undefined due to the error → len(rs)==0
		if len(rs) != 0 {
			t.Errorf("expected undefined (len(rs)==0) for modulo by zero, got %d results", len(rs))
		}
	})

	// Strict mode: modulo by zero raises an error
	t.Run("strict_mode", func(t *testing.T) {
		module := `package test
result := 10 % 0
`
		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		_, err = query.Eval(ctx)
		if err == nil {
			t.Fatal("expected eval error for modulo by zero, but got none")
		}

		// Step 1: check the typed error code (like standard OPA, modulo by zero is BuiltinErr)
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Errorf("expected error code %s, got %s", topdown.BuiltinErr, topdownErr.Code)
		}
		// Step 2: check the message (same "rem: modulo by zero" as standard OPA)
		if !strings.Contains(err.Error(), "rem: modulo by zero") {
			t.Errorf("expected message to contain %q, got: %v", "rem: modulo by zero", err)
		}
	})
}

func TestDecimalOperators_Comparison(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"gt_true", "3.3 > 2.2", true},
		{"gt_false", "2.2 > 3.3", false},
		{"gte_true", "3.3 >= 3.3", true},
		{"gte_false", "2.2 >= 3.3", false},
		{"lt_true", "2.2 < 3.3", true},
		{"lt_false", "3.3 < 2.2", false},
		{"lte_true", "3.3 <= 3.3", true},
		{"lte_false", "3.3 <= 2.2", false},
		{"eq_true", "3.3 == 3.3", true},
		{"eq_false", "3.3 == 2.2", false},
		{"neq_true", "3.3 != 2.2", true},
		{"neq_false", "3.3 != 3.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_Unary(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"abs_positive", "abs(3.3)", "3.3"},
		{"abs_negative", "abs(-3.3)", "3.3"},
		// round is half away from zero (same as OPA default behavior)
		{"round_3.5", "round(3.5)", "4"},   // 3.5 → 4
		{"round_2.5", "round(2.5)", "3"},   // 2.5 → 3
		{"round_1.5", "round(1.5)", "2"},   // 1.5 → 2
		{"round_0.5", "round(0.5)", "1"},   // 0.5 → 1
		{"round_4.5", "round(4.5)", "5"},   // 4.5 → 5
		{"round_3.4", "round(3.4)", "3"},   // ordinary rounding
		{"round_3.6", "round(3.6)", "4"},   // ordinary rounding
		{"round_neg", "round(-2.5)", "-3"}, // -2.5 → -3 (away from zero)
		{"ceil", "ceil(3.1)", "4"},
		{"floor", "floor(3.9)", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_Precision(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// basic precision check
		{"basic_add", "0.1 + 0.2", "0.3"},
		// big.Float precision limit: 1.1 + 2.2 != 3.3 (Standard OPA), udecimal is exact
		{"bigfloat_limit", "1.1 + 2.2", "3.3"},
		{"bigfloat_limit_eq", "0.3 - 0.1 == 0.2", "true"},

		// long decimal operations
		{"long_decimal_add", "0.123456789 + 0.000000001", "0.12345679"},
		{"long_decimal_sub", "1.123456789123456789 - 0.000000000000000001", "1.123456789123456788"},

		// large numbers (float64 has about 15-17 digits of precision)
		{"large_number_add", "9999999999999999 + 1", "10000000000000000"},
		{"large_number_mul", "123456789 * 987654321", "121932631112635269"},

		// financial calculation cases
		{"money_mul", "100.25 * 0.03", "3.0075"},
		{"money_div", "100 / 3", "33.3333333333333333333"}, // udecimal defaults to 19 decimal places

		// comparison operator precision
		{"compare_precision", "0.1 + 0.2 == 0.3", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			var result string
			switch v := rs[0].Expressions[0].Value.(type) {
			case json.Number:
				result = v.String()
			case bool:
				result = "false"
				if v {
					result = "true"
				}
			}

			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_SetMinus(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// verify that the minus operation works correctly on sets
	module := `package test
result := {1, 2, 3} - {2}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	// verify the set result
	result := rs[0].Expressions[0].Value.([]interface{})
	got := map[int]bool{}
	for _, v := range result {
		switch n := v.(type) {
		case json.Number:
			i, err := n.Int64()
			if err != nil {
				t.Fatalf("set minus: non-int element %q: %v", n.String(), err)
			}
			got[int(i)] = true
		case float64:
			got[int(n)] = true
		default:
			t.Fatalf("set minus: unexpected element type %T (%v)", v, v)
		}
	}
	if len(got) != 2 || !got[1] || !got[3] {
		t.Fatalf("set minus: got %v, want {1,3}", got)
	}
}

// TestDecimalOperators_Minus_TypeMismatch exercises the three type-mismatch
// branches of precisionMinus (operator.go): the operands reach runtime through
// input (typed Any), so they bypass OPA's compile-time type checker and hit the
// runtime type-error branches. Each case asserts the operand position and the
// expected type string reported in strict mode.
func TestDecimalOperators_Minus_TypeMismatch(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name    string
		module  string
		input   map[string]interface{}
		wantMsg string
	}{
		{
			// lhs is a number, rhs is a non-numeric string -> operand 2 must be number.
			name:    "number_minus_nonnumeric_string",
			module:  "package test\nimport rego.v1\nresult := 1 - input.b",
			input:   map[string]interface{}{"b": "a"},
			wantMsg: "operand 2 must be number",
		},
		{
			// lhs is a set, rhs is a number -> operand 2 must be set.
			name:    "set_minus_number",
			module:  "package test\nimport rego.v1\nresult := {1} - input.b",
			input:   map[string]interface{}{"b": 3},
			wantMsg: "operand 2 must be set",
		},
		{
			// lhs is neither number nor set -> operand 1 must be one of {number, set}.
			name:    "bool_minus_number",
			module:  "package test\nimport rego.v1\nresult := input.a - 1",
			input:   map[string]interface{}{"a": true},
			wantMsg: "operand 1 must be one of {number, set}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalModule(t, tt.module, tt.input, rego.StrictBuiltinErrors(true))
			if err == nil {
				t.Fatal("expected eval error for type mismatch, got nil")
			}
			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("expected topdown.Error, got %T: %v", err, err)
			}
			if topdownErr.Code != topdown.TypeErr {
				t.Fatalf("expected %s, got %s (%v)", topdown.TypeErr, topdownErr.Code, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("expected message to contain %q, got: %v", tt.wantMsg, err)
			}
		})
	}
}

// TestDecimalOperators_CoercionOff_ElementAndOperandErrors covers error paths in
// operator.go that trigger with string coercion disabled (the default): aggregate
// element type errors (elementToDecimal) and binary-operand type errors
// (operandToDecimal), including the default (non-number, non-string) branches.
func TestDecimalOperators_CoercionOff_ElementAndOperandErrors(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name    string
		module  string
		input   map[string]interface{}
		wantMsg string
	}{
		{
			// sum element is a non-numeric string; coercion off -> element type error.
			name:    "sum_nonnumeric_string_element",
			module:  "package test\nimport rego.v1\nresult := sum(input.arr)",
			input:   map[string]interface{}{"arr": []interface{}{"abc"}},
			wantMsg: "operand 1 must be array of number",
		},
		{
			// sum element is a bool -> elementToDecimal's default branch.
			name:    "sum_bool_element",
			module:  "package test\nimport rego.v1\nresult := sum(input.arr)",
			input:   map[string]interface{}{"arr": []interface{}{true}},
			wantMsg: "operand 1 must be array of number",
		},
		{
			// 1 + "abc": second operand is a non-numeric string, coercion off.
			name:    "plus_nonnumeric_string_operand",
			module:  "package test\nimport rego.v1\nresult := 1 + input.b",
			input:   map[string]interface{}{"b": "abc"},
			wantMsg: "operand 2 must be number",
		},
		{
			// 1 + true: second operand hits operandToDecimal's default branch.
			name:    "plus_bool_operand",
			module:  "package test\nimport rego.v1\nresult := 1 + input.b",
			input:   map[string]interface{}{"b": true},
			wantMsg: "operand 2 must be number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalModule(t, tt.module, tt.input, rego.StrictBuiltinErrors(true))
			if err == nil {
				t.Fatal("expected eval error, got nil")
			}
			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("expected topdown.Error, got %T: %v", err, err)
			}
			if topdownErr.Code != topdown.TypeErr {
				t.Fatalf("expected %s, got %s (%v)", topdown.TypeErr, topdownErr.Code, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("expected message to contain %q, got: %v", tt.wantMsg, err)
			}
		})
	}
}

// TestDecimalOperators_Max_NumericElementParseError covers the numeric-mode parse
// error branch of precisionMax: every element is numeric-like, but one is a Number
// whose expanded value exceeds udecimal precision, so elementToDecimal returns the
// raw parse error (BuiltinErr) rather than a type error.
func TestDecimalOperators_Max_NumericElementParseError(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := "package test\nresult := max([input.x, 1])"
	input := map[string]interface{}{"x": Number("1e-25")}

	t.Run("default_undefined", func(t *testing.T) {
		rs := evalModuleResult(t, module, input)
		requireUndefinedResult(t, rs)
	})

	t.Run("strict_error", func(t *testing.T) {
		_, err := evalModule(t, module, input, rego.StrictBuiltinErrors(true))
		if err == nil {
			t.Fatal("expected eval error for out-of-precision element, got nil")
		}
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Fatalf("expected %s, got %s (%v)", topdown.BuiltinErr, topdownErr.Code, err)
		}
	})
}

func TestDecimalOperators_EqualityFallbackForNonNumbers(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
result := {
  "s_eq": "a" == "a",
  "s_neq": "a" != "b",
  "o_eq": {"x": 1, "y": [1,2]} == {"y": [1,2], "x": 1},
  "o_neq": {"x": 1} != {"x": 2},
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	m, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", rs[0].Expressions[0].Value)
	}
	if m["s_eq"] != true || m["s_neq"] != true || m["o_eq"] != true || m["o_neq"] != true {
		t.Fatalf("unexpected equality results: %v", m)
	}
}

func TestDecimalOperators_Aggregates(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// sum tests
		{"sum_array", `sum([0.1, 0.2, 0.3])`, "0.6"},
		{"sum_precision", `sum([0.1, 0.2])`, "0.3"}, // float64 gives 0.30000000000000004
		{"sum_large", `sum([9999999999999999, 1])`, "10000000000000000"},
		{"sum_empty", `sum([])`, "0"},

		// product tests
		{"product_array", `product([2, 3, 4])`, "24"},
		{"product_precision", `product([0.1, 0.2, 0.3])`, "0.006"},
		{"product_empty", `product([])`, "1"},

		// max tests
		{"max_array", `max([1, 3, 2])`, "3"},
		{"max_precision", `max([0.1, 0.11, 0.09])`, "0.11"},
		{"max_large", `max([9999999999999999, 9999999999999998])`, "9999999999999999"},

		// min tests
		{"min_array", `min([3, 1, 2])`, "1"},
		{"min_precision", `min([0.1, 0.11, 0.09])`, "0.09"},
		{"min_large", `min([9999999999999999, 9999999999999998])`, "9999999999999998"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_Aggregates_Set(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"sum_set", `sum({0.1, 0.2, 0.3})`, "0.6"},
		{"product_set", `product({2, 3, 4})`, "24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_MaxMin_NonNumbers(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// verify that max/min also work on non-numeric values
	module := `package test
result := {
  "max_str": max(["b", "a", "c"]),
  "min_str": min(["b", "a", "c"]),
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	m, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", rs[0].Expressions[0].Value)
	}
	if m["max_str"] != "c" || m["min_str"] != "a" {
		t.Fatalf("unexpected results: %v", m)
	}
}

func TestDecimalOperators_MaxMin_Set(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"max_set_numbers", `max({1, 3, 2})`, "3"},
		{"min_set_numbers", `min({3, 1, 2})`, "1"},
		{"max_set_precision", `max({0.1, 0.11, 0.09})`, "0.11"},
		{"min_set_precision", `min({0.1, 0.11, 0.09})`, "0.09"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
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

func TestDecimalOperators_MaxMin_SetNonNumbers(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
result := {
  "max_str": max({"b", "a", "c"}),
  "min_str": min({"b", "a", "c"}),
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	m, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", rs[0].Expressions[0].Value)
	}
	if m["max_str"] != "c" || m["min_str"] != "a" {
		t.Fatalf("unexpected results: %v", m)
	}
}

func TestDecimalOperators_NoStringCoercion_Default(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Without WithStringCoercion(), a string operand is a type error
	module := `package test
import rego.v1
result := input.a + input.b`
	rs := evalModuleResult(t, module, map[string]interface{}{
		"a": "5.5",
		"b": 2.2,
	})
	requireUndefinedResult(t, rs)
}

func TestWithStringCoercion_PublicAPI(t *testing.T) {
	// Register the cleanup first so an intermediate t.Fatalf cannot leak the
	// coercion-ON global state into subsequent tests.
	t.Cleanup(func() {
		UseDecimalArithmetic()
	})

	module := `package test
import rego.v1
result := input.a + input.b`
	input := map[string]interface{}{
		"a": "5.5",
		"b": 2.2,
	}

	UseDecimalArithmetic()
	rs := evalModuleResult(t, module, input)
	requireUndefinedResult(t, rs)

	UseDecimalArithmetic(WithStringCoercion())
	rs = evalModuleResult(t, module, input)
	if got := requireSingleExprValue(t, rs).(json.Number).String(); got != "7.7" {
		t.Fatalf("got %s, want 7.7", got)
	}

	UseDecimalArithmetic()
	rs = evalModuleResult(t, module, input)
	requireUndefinedResult(t, rs)
}

func enableStringCoercion(t *testing.T) {
	t.Helper()
	UseDecimalArithmetic(WithStringCoercion())
	t.Cleanup(func() {
		UseDecimalArithmetic()
	})
}

func TestDecimalOperators_StringCoercion_Arithmetic(t *testing.T) {
	enableStringCoercion(t)

	// Test runtime automatic conversion of numbers arriving as strings in JSON input
	// Since the OPA compiler does not know the type of input values (any), runtime coercion applies
	module := `package test
import rego.v1
result_minus := input.a - input.b
result_plus := input.a + input.b
result_mul := input.a * input.b
result_div := input.a / input.b
result_rem := input.c % input.d
`
	tests := []struct {
		name     string
		input    map[string]interface{}
		query    string
		expected string
	}{
		{"string_minus_number", map[string]interface{}{"a": "5.5", "b": 2.2, "c": "10", "d": 3}, "data.test.result_minus", "3.3"},
		{"string_plus_number", map[string]interface{}{"a": "1.5", "b": 2, "c": "10", "d": 3}, "data.test.result_plus", "3.5"},
		{"string_mul_number", map[string]interface{}{"a": "100.25", "b": 0.03, "c": "10", "d": 3}, "data.test.result_mul", "3.0075"},
		{"string_div_number", map[string]interface{}{"a": "10", "b": 4, "c": "10", "d": 3}, "data.test.result_div", "2.5"},
		{"string_rem_number", map[string]interface{}{"a": "5.5", "b": 2.2, "c": "10", "d": 3}, "data.test.result_rem", "1"},
		{"both_strings_minus", map[string]interface{}{"a": "5.5", "b": "2.2", "c": "10", "d": "3"}, "data.test.result_minus", "3.3"},
		{"both_strings_mul", map[string]interface{}{"a": "5.5", "b": "2", "c": "10", "d": "3"}, "data.test.result_mul", "11"},
		{"number_minus_string", map[string]interface{}{"a": 1, "b": "0.3", "c": "10", "d": 3}, "data.test.result_minus", "0.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			query, err := rego.New(
				rego.Query(tt.query),
				rego.Module("test.rego", module),
				rego.Input(tt.input),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			result := requireSingleExprValue(t, rs).(json.Number).String()
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_Comparison(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name     string
		module   string
		expected bool
	}{
		{
			"string_gt_number",
			`package test
			import rego.v1
			qty := "0.73"
			result := qty > 0.5`,
			true,
		},
		{
			"string_lt_number",
			`package test
			import rego.v1
			qty := "0.73"
			result := qty < 1`,
			true,
		},
		{
			"string_gte",
			`package test
			import rego.v1
			result := "3.3" >= 3.3`,
			true,
		},
		{
			"string_lte",
			`package test
			import rego.v1
			result := "2.2" <= 3.3`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, nil)
			result := requireSingleExprValue(t, rs).(bool)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_Unary(t *testing.T) {
	enableStringCoercion(t)

	module := `package test
import rego.v1
result_abs := abs(input.v)
result_round := round(input.v)
result_ceil := ceil(input.v)
result_floor := floor(input.v)
`
	tests := []struct {
		name     string
		input    map[string]interface{}
		query    string
		expected string
	}{
		{"abs_string", map[string]interface{}{"v": "-3.3"}, "data.test.result_abs", "3.3"},
		{"round_string", map[string]interface{}{"v": "3.5"}, "data.test.result_round", "4"},
		{"ceil_string", map[string]interface{}{"v": "3.1"}, "data.test.result_ceil", "4"},
		{"floor_string", map[string]interface{}{"v": "3.9"}, "data.test.result_floor", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			query, err := rego.New(
				rego.Query(tt.query),
				rego.Module("test.rego", module),
				rego.Input(tt.input),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			result := requireSingleExprValue(t, rs).(json.Number).String()
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_InvalidString(t *testing.T) {
	enableStringCoercion(t)

	// non-numeric strings are still an error (reaching runtime via input)
	module := `package test
import rego.v1
result := input.a + input.b`
	rs := evalModuleResult(t, module, map[string]interface{}{
		"a": "abc",
		"b": 1,
	})
	requireUndefinedResult(t, rs)
}

func TestDecimalOperators_StringCoercion_InvalidString_StrictBuiltinErrors(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name   string
		module string
		input  map[string]interface{}
	}{
		{
			name: "arithmetic_invalid_string",
			module: `package test
import rego.v1
result := input.a + input.b`,
			input: map[string]interface{}{"a": "abc", "b": 1},
		},
		// Note: max/min no longer error on mixed numeric/non-numeric elements.
		// They fall back to the default ast.Compare ordering (see
		// TestDecimalOperators_StringCoercion_MaxMin_MixedFallback).
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalModule(t, tt.module, tt.input, rego.StrictBuiltinErrors(true))
			if err == nil {
				t.Fatal("expected eval error in strict builtin mode, got nil")
			}

			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("expected topdown.Error, got %T: %v", err, err)
			}
			if topdownErr.Code != topdown.TypeErr {
				t.Fatalf("expected %s, got %s (%v)", topdown.TypeErr, topdownErr.Code, err)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_InputParams(t *testing.T) {
	enableStringCoercion(t)

	// Real-world usage scenario: a number arriving as a string in JSON input
	module := `package test
import rego.v1
result := input.qty - input.pos`

	rs := evalModuleResult(t, module, map[string]interface{}{
		"qty": "0.73", // JSON string
		"pos": 0.5,    // JSON number
	})
	result := requireSingleExprValue(t, rs).(json.Number).String()
	if result != "0.23" {
		t.Errorf("got %s, want 0.23", result)
	}
}

func TestDecimalOperators_StringCoercion_MaxMin_NumericStrings(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name     string
		query    string
		input    map[string]interface{}
		expected interface{}
	}{
		{
			name:     "max_array_numeric_strings",
			query:    "data.test.result",
			input:    map[string]interface{}{"arr": []interface{}{"1", "10", "2"}},
			expected: "10",
		},
		{
			name:     "min_array_numeric_strings",
			query:    "data.test.result",
			input:    map[string]interface{}{"arr": []interface{}{"1", "10", "2"}},
			expected: "1",
		},
		{
			name:     "max_set_numeric_strings",
			query:    "data.test.result",
			input:    map[string]interface{}{"arr": []interface{}{"1", "10", "2"}},
			expected: "10",
		},
		{
			name:     "min_set_numeric_strings",
			query:    "data.test.result",
			input:    map[string]interface{}{"arr": []interface{}{"1", "10", "2"}},
			expected: "1",
		},
	}

	modules := map[string]string{
		"max_array_numeric_strings": `package test
import rego.v1
result := max(input.arr)`,
		"min_array_numeric_strings": `package test
import rego.v1
result := min(input.arr)`,
		"max_set_numeric_strings": `package test
import rego.v1
result := max({x | some x in input.arr})`,
		"min_set_numeric_strings": `package test
import rego.v1
result := min({x | some x in input.arr})`,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			query, err := rego.New(
				rego.Query(tt.query),
				rego.Module("test.rego", modules[tt.name]),
				rego.Input(tt.input),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

// TestDecimalOperators_StringCoercion_MaxMin_NonNumericFallback is a regression
// test for the fallback policy: with WithStringCoercion() enabled, collections
// whose elements are not all numeric-like must fall back to the default
// ast.Compare ordering rather than erroring or returning undefined. This keeps
// max/min behaving identically to standard OPA (and to coercion-off mode) for
// non-numeric inputs, while numeric-only collections still use precise numeric
// comparison.
func TestDecimalOperators_StringCoercion_MaxMin_NonNumericFallback(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name     string
		module   string
		input    map[string]interface{}
		expected interface{}
	}{
		{
			name: "max_all_non_numeric_strings",
			module: `package test
import rego.v1
result := max(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "banana",
		},
		{
			name: "min_all_non_numeric_strings",
			module: `package test
import rego.v1
result := min(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "apple",
		},
		{
			name: "max_set_non_numeric_strings",
			module: `package test
import rego.v1
result := max({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "banana",
		},
		{
			name: "min_set_non_numeric_strings",
			module: `package test
import rego.v1
result := min({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "apple",
		},
		{
			// Mixed types (number, numeric-string, bool): not all numeric-like →
			// fallback to type ordering (bool < number < string) → max is "2".
			name: "max_mixed_types_fallback",
			module: `package test
import rego.v1
result := max(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{1, "2", true}},
			expected: "2",
		},
		{
			// Same mixed collection: min is the boolean true (lowest in type order).
			name: "min_mixed_types_fallback",
			module: `package test
import rego.v1
result := min(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{1, "2", true}},
			expected: true,
		},
		{
			// Number + numeric-string: all numeric-like → numeric mode retained.
			// 2 > 1 by decimal value, so the "2" term is selected.
			name: "max_number_and_numeric_string_stays_numeric",
			module: `package test
import rego.v1
result := max(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{1, "2"}},
			expected: "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

// TestDecimalOperators_MaxMin_NonNumeric_CoercionOff confirms the fallback policy
// is identical whether or not WithStringCoercion() is enabled.
func TestDecimalOperators_MaxMin_NonNumeric_CoercionOff(t *testing.T) {
	UseDecimalArithmetic()

	tests := []struct {
		name     string
		module   string
		input    map[string]interface{}
		expected interface{}
	}{
		{
			name: "max_all_non_numeric_strings",
			module: `package test
import rego.v1
result := max(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "banana",
		},
		{
			name: "min_all_non_numeric_strings",
			module: `package test
import rego.v1
result := min(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"apple", "banana"}},
			expected: "apple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

// TestDecimalOperators_StringCoercion_MaxMin_MixedFallback verifies that when a
// collection mixes numeric-like and non-numeric elements, max/min fall back to
// the default ast.Compare ordering (lexicographic for strings) instead of
// erroring. This matches standard OPA and the coercion-off behavior.
func TestDecimalOperators_StringCoercion_MaxMin_MixedFallback(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name     string
		module   string
		input    map[string]interface{}
		expected interface{}
	}{
		{
			name: "max_array_mixed",
			module: `package test
import rego.v1
result := max(input.arr)`,
			// "0.1" < "abc" lexicographically → max is "abc"
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
			expected: "abc",
		},
		{
			name: "min_array_mixed",
			module: `package test
import rego.v1
result := min(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
			expected: "0.1",
		},
		{
			name: "max_set_mixed",
			module: `package test
import rego.v1
result := max({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
			expected: "abc",
		},
		{
			name: "min_set_mixed",
			module: `package test
import rego.v1
result := min({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
			expected: "0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_SumProduct(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name     string
		module   string
		input    map[string]interface{}
		expected string
	}{
		{
			name: "sum_array_numeric_strings",
			module: `package test
import rego.v1
result := sum(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "0.2", "0.3"}},
			expected: "0.6",
		},
		{
			name: "product_array_numeric_strings",
			module: `package test
import rego.v1
result := product(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"2", "3", "4"}},
			expected: "24",
		},
		{
			name: "sum_set_numeric_strings",
			module: `package test
import rego.v1
result := sum({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"0.1", "0.2", "0.3"}},
			expected: "0.6",
		},
		{
			name: "product_set_numeric_strings",
			module: `package test
import rego.v1
result := product({x | some x in input.arr})`,
			input:    map[string]interface{}{"arr": []interface{}{"2", "3", "4"}},
			expected: "24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			if got := requireSingleExprValue(t, rs).(json.Number).String(); got != tt.expected {
				t.Fatalf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_SumProduct_InvalidString_StrictBuiltinErrors(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name   string
		module string
		input  map[string]interface{}
	}{
		{
			name: "sum_array_invalid_string",
			module: `package test
import rego.v1
result := sum(input.arr)`,
			input: map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
		},
		{
			name: "product_array_invalid_string",
			module: `package test
import rego.v1
result := product(input.arr)`,
			input: map[string]interface{}{"arr": []interface{}{"2", "abc"}},
		},
		{
			name: "sum_set_invalid_string",
			module: `package test
import rego.v1
result := sum({x | some x in input.arr})`,
			input: map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
		},
		{
			name: "product_set_invalid_string",
			module: `package test
import rego.v1
result := product({x | some x in input.arr})`,
			input: map[string]interface{}{"arr": []interface{}{"2", "abc"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalModule(t, tt.module, tt.input, rego.StrictBuiltinErrors(true))
			if err == nil {
				t.Fatal("expected eval error in strict builtin mode, got nil")
			}

			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("expected topdown.Error, got %T: %v", err, err)
			}
			if topdownErr.Code != topdown.TypeErr {
				t.Fatalf("expected %s, got %s (%v)", topdown.TypeErr, topdownErr.Code, err)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_SumProduct_InvalidString_DefaultMode(t *testing.T) {
	enableStringCoercion(t)

	tests := []struct {
		name   string
		module string
		input  map[string]interface{}
	}{
		{
			name: "sum_array_invalid_string",
			module: `package test
import rego.v1
result := sum(input.arr)`,
			input: map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
		},
		{
			name: "product_array_invalid_string",
			module: `package test
import rego.v1
result := product(input.arr)`,
			input: map[string]interface{}{"arr": []interface{}{"2", "abc"}},
		},
		{
			name: "sum_set_invalid_string",
			module: `package test
import rego.v1
result := sum({x | some x in input.arr})`,
			input: map[string]interface{}{"arr": []interface{}{"0.1", "abc"}},
		},
		{
			name: "product_set_invalid_string",
			module: `package test
import rego.v1
result := product({x | some x in input.arr})`,
			input: map[string]interface{}{"arr": []interface{}{"2", "abc"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			requireUndefinedResult(t, rs)
		})
	}
}

func TestDecimalOperators_Comparison_NonNumericUndefined(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// In UseDecimalArithmetic the comparison operators are numeric-only, so
	// non-numeric comparisons should become undefined
	tests := []struct {
		name string
		expr string
	}{
		{"string_lt_string", `"a" < "b"`},
		{"string_gt_number", `"hello" > 123`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := `package test
import rego.v1
result if { ` + tt.expr + ` }`
			rs := evalModuleResult(t, module, nil)
			requireUndefinedResult(t, rs)
		})
	}
}

func TestDecimalOperators_StringCoercion_EqualityNotApplied(t *testing.T) {
	enableStringCoercion(t)

	// Even when stringCoercion is enabled, it is not applied to == and != (preserving OPA's type system)
	tests := []struct {
		name     string
		module   string
		expected bool
	}{
		{
			"string_eq_number_false",
			`package test
			import rego.v1
			result := "3.3" == 3.3`,
			false,
		},
		{
			"string_neq_number_true",
			`package test
			import rego.v1
			result := "3.3" != 3.3`,
			true,
		},
		{
			"string_eq_integer_false",
			`package test
			import rego.v1
			result := "1" == 1`,
			false,
		},
		{
			"string_neq_integer_true",
			`package test
			import rego.v1
			result := "1" != 1`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, nil)
			result := requireSingleExprValue(t, rs).(bool)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_StringCoercion_MaxMin_MixedTypes(t *testing.T) {
	enableStringCoercion(t)

	// Array mixing numeric strings and a non-numeric type (bool): since not all elements
	// are numeric-like, it does not enter numeric mode and falls back to the default comparison (ast.Compare) order.
	// By the type ordering (bool < number < string), max is "1" (a string) and min is true.
	// In both default and strict modes it must return a value without error.
	tests := []struct {
		name     string
		module   string
		input    map[string]interface{}
		expected interface{}
	}{
		{
			name: "max_string_and_bool",
			module: `package test
import rego.v1
result := max(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"1", true}},
			expected: "1",
		},
		{
			name: "min_string_and_bool",
			module: `package test
import rego.v1
result := min(input.arr)`,
			input:    map[string]interface{}{"arr": []interface{}{"1", true}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_default", func(t *testing.T) {
			rs := evalModuleResult(t, tt.module, tt.input)
			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})

		t.Run(tt.name+"_strict", func(t *testing.T) {
			rs, err := evalModule(t, tt.module, tt.input, rego.StrictBuiltinErrors(true))
			if err != nil {
				t.Fatalf("expected no error in strict mode (fallback comparison), got %v", err)
			}
			if got := requireSingleExprValue(t, rs); got != tt.expected {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestDecimalOperators_MaxMin_Empty(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// max/min on an empty array returns undefined
	tests := []struct {
		name string
		expr string
	}{
		{"max_empty_array", `max([])`},
		{"min_empty_array", `min([])`},
		{"max_empty_set", `max(set())`},
		{"min_empty_set", `min(set())`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			// max/min on an empty collection is undefined, so there should be no result
			if len(rs) != 0 {
				t.Errorf("expected undefined (len(rs)==0) for empty collection, got %d results: %v", len(rs), rs)
			}
		})
	}
}

// TestExpandExponent is a unit test for the exponent-notation expansion helper.
func TestExpandExponent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1e-8", "0.00000001"},                   // negative exponent
		{"1e8", "100000000"},                     // positive exponent
		{"1E3", "1000"},                          // uppercase E
		{"-2.5E+3", "-2500"},                     // sign + uppercase E + decimal mantissa
		{"1.5e0", "1.5"},                         // e0 (no shift)
		{"1e0", "1"},                             // e0 integer
		{"+1e2", "100"},                          // positive sign
		{"5e-1", "0.5"},                          // fractional result
		{"1.23e2", "123"},                        // decimal mantissa, positive exponent
		{"1.23e-2", "0.0123"},                    // decimal mantissa, negative exponent
		{"0.5e1", "05"},                          // leading zero (allowed by udecimal)
		{"1e-25", "0.0000000000000000000000001"}, // expansion beyond precision (only expansion is checked here)
		{"123.45", "123.45"},                     // no exponent → unchanged
		{"100", "100"},                           // integer unchanged
		{"", ""},                                 // empty string unchanged
		{"1eX", "1eX"},                           // invalid exponent part → unchanged (rejected by udecimal)
		// Size guard: for huge exponents whose expansion would exceed maxExpandedLen,
		// return the original unchanged instead of causing an allocation blowup via strings.Repeat
		// (udecimal.Parse rejects it as invalid format → same as the existing error behavior).
		{"1e2000000000", "1e2000000000"},   // huge positive exponent → not expanded
		{"1e-2000000000", "1e-2000000000"}, // huge negative exponent → not expanded
		{"1e65", "1e65"},                   // exceeds the exponent cap (maxExpandedLen) → not expanded
		{"1e100", "1e100"},                 // exceeds udecimal range → not expanded
		{"1e999999999999999999999999", "1e999999999999999999999999"}, // exponent exceeds int range → unchanged
		// Size guard 2 (pre-computing expansion length): the exponent (e10) is within the cap so it passes
		// guard 1, but expanding a 60-digit mantissa makes the result length exceed maxExpandedLen, so
		// it is not expanded and the original is returned unchanged (rejected by udecimal → same error behavior).
		{
			"111111111111111111111111111111111111111111111111111111111111e10",
			"111111111111111111111111111111111111111111111111111111111111e10",
		},
	}
	for _, tt := range tests {
		if got := expandExponent(tt.in); got != tt.want {
			t.Errorf("expandExponent(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDecimalOperators_ExponentNotation verifies that exponent-notation numbers behave
// the same as standard OPA in operations (regression prevention).
func TestDecimalOperators_ExponentNotation(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	t.Run("plus", func(t *testing.T) {
		// Standard OPA: 1e-8 + 1 == 1.00000001
		rs := evalModuleResult(t, "package test\nresult := 1e-8 + 1", nil)
		if got := requireSingleExprValue(t, rs); fmt.Sprintf("%v", got) != "1.00000001" {
			t.Errorf("expected 1.00000001, got %v", got)
		}
	})

	t.Run("max", func(t *testing.T) {
		rs := evalModuleResult(t, "package test\nresult := max([1e-8, 2])", nil)
		if got := requireSingleExprValue(t, rs); fmt.Sprintf("%v", got) != "2" {
			t.Errorf("expected 2, got %v", got)
		}
	})

	t.Run("equal", func(t *testing.T) {
		// Standard OPA: 1e-8 == 0.00000001 → true
		rs := evalModuleResult(t, "package test\nresult := 1e-8 == 0.00000001", nil)
		if got := requireSingleExprValue(t, rs); got != true {
			t.Errorf("expected true, got %v", got)
		}
	})

	t.Run("less_than", func(t *testing.T) {
		rs := evalModuleResult(t, "package test\nresult := 1e-8 < 1", nil)
		if got := requireSingleExprValue(t, rs); got != true {
			t.Errorf("expected true, got %v", got)
		}
	})

	t.Run("out_of_precision_default_undefined", func(t *testing.T) {
		// 1e-25 expands to 25 decimal places → exceeds precision (19 digits) → undefined
		rs := evalModuleResult(t, "package test\nresult := 1e-25 + 1", nil)
		requireUndefinedResult(t, rs)
	})

	t.Run("out_of_precision_strict_error", func(t *testing.T) {
		_, err := evalModule(t, "package test\nresult := 1e-25 + 1", nil, rego.StrictBuiltinErrors(true))
		if err == nil {
			t.Fatal("expected eval error for out-of-precision exponent, got none")
		}
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Errorf("expected %s, got %s", topdown.BuiltinErr, topdownErr.Code)
		}
	})

	// Huge exponents can arrive via the runtime input path, which does not go through the Rego parser.
	// Thanks to expandExponent's size guard, this must end immediately in the (udecimal invalid format)
	// error behavior without any gigabyte-scale string allocation.
	t.Run("huge_exponent_input_default_undefined", func(t *testing.T) {
		for _, v := range []Number{"1e2000000000", "1e-2000000000"} {
			rs := evalModuleResult(t, "package test\nresult := input.x + 1",
				map[string]interface{}{"x": v})
			requireUndefinedResult(t, rs)
		}
	})

	t.Run("huge_exponent_input_strict_error", func(t *testing.T) {
		for _, v := range []Number{"1e2000000000", "1e-2000000000"} {
			_, err := evalModule(t, "package test\nresult := input.x + 1",
				map[string]interface{}{"x": v}, rego.StrictBuiltinErrors(true))
			if err == nil {
				t.Fatalf("expected eval error for huge exponent %s, got none", v)
			}
			var topdownErr *topdown.Error
			if !errors.As(err, &topdownErr) {
				t.Fatalf("expected topdown.Error, got %T: %v", err, err)
			}
			if topdownErr.Code != topdown.BuiltinErr {
				t.Errorf("expected %s, got %s", topdown.BuiltinErr, topdownErr.Code)
			}
		}
	})
}

// TestDecimalOperators_StringCoercion_Exponent verifies that, with coercion ON, an
// exponent-notation string ("1e-8") is converted to a number and used in operations.
func TestDecimalOperators_StringCoercion_Exponent(t *testing.T) {
	enableStringCoercion(t)

	rs, err := evalModule(t, "package test\nimport rego.v1\nresult := input.s + 1",
		map[string]interface{}{"s": "1e-8"})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if got := requireSingleExprValue(t, rs); fmt.Sprintf("%v", got) != "1.00000001" {
		t.Errorf("expected 1.00000001, got %v", got)
	}
}
