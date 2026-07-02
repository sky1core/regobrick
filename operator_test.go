package regobrick

import (
	"context"
	"encoding/json"
	"errors"
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

		// 기본 모드: 연산 실패 시 결과 없음 (에러 아님)
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
		// Strict 모드: 에러가 상위로 전파됨
		if err == nil {
			t.Fatal("expected eval error for divide by zero, but got none")
		}

		// 1차: typed error code 확인 (divide by zero는 TypeErr)
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.TypeErr {
			t.Errorf("expected error code %s, got %s", topdown.TypeErr, topdownErr.Code)
		}
		// 2차: 메시지 substring 확인 (보조)
		if !strings.Contains(err.Error(), "divide by zero") {
			t.Logf("warning: error message changed: %v", err)
		}
	})
}

func TestDecimalOperators_ModuloByZero(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// 기본 모드: modulo by zero는 undefined 반환
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

		// 기본 모드: 에러로 인해 undefined → len(rs)==0
		if len(rs) != 0 {
			t.Errorf("expected undefined (len(rs)==0) for modulo by zero, got %d results", len(rs))
		}
	})

	// Strict 모드: modulo by zero는 에러 발생
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

		// 1차: typed error code 확인 (modulo by zero는 TypeErr)
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.TypeErr {
			t.Errorf("expected error code %s, got %s", topdown.TypeErr, topdownErr.Code)
		}
		// 2차: 메시지 substring 확인 (보조)
		if !strings.Contains(err.Error(), "modulo by zero") {
			t.Logf("warning: error message changed: %v", err)
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
		// round는 half away from zero (OPA 기본 동작과 동일)
		{"round_3.5", "round(3.5)", "4"},   // 3.5 → 4
		{"round_2.5", "round(2.5)", "3"},   // 2.5 → 3
		{"round_1.5", "round(1.5)", "2"},   // 1.5 → 2
		{"round_0.5", "round(0.5)", "1"},   // 0.5 → 1
		{"round_4.5", "round(4.5)", "5"},   // 4.5 → 5
		{"round_3.4", "round(3.4)", "3"},   // 일반 반올림
		{"round_3.6", "round(3.6)", "4"},   // 일반 반올림
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
		// 기본 정밀도 확인
		{"basic_add", "0.1 + 0.2", "0.3"},
		// big.Float 정밀도 한계: 1.1 + 2.2 != 3.3 (Standard OPA), udecimal은 정확
		{"bigfloat_limit", "1.1 + 2.2", "3.3"},
		{"bigfloat_limit_eq", "0.3 - 0.1 == 0.2", "true"},

		// 긴 소수점 연산
		{"long_decimal_add", "0.123456789 + 0.000000001", "0.12345679"},
		{"long_decimal_sub", "1.123456789123456789 - 0.000000000000000001", "1.123456789123456788"},

		// 큰 숫자 (float64는 약 15-17자리 정밀도)
		{"large_number_add", "9999999999999999 + 1", "10000000000000000"},
		{"large_number_mul", "123456789 * 987654321", "121932631112635269"},

		// 금융 계산 케이스
		{"money_mul", "100.25 * 0.03", "3.0075"},
		{"money_div", "100 / 3", "33.3333333333333333333"}, // udecimal 기본 19자리 소수점

		// 비교 연산 정밀도
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

	// set에 대한 minus 연산이 정상 작동하는지 확인
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

	// set 결과 확인
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
		// sum 테스트
		{"sum_array", `sum([0.1, 0.2, 0.3])`, "0.6"},
		{"sum_precision", `sum([0.1, 0.2])`, "0.3"}, // float64는 0.30000000000000004
		{"sum_large", `sum([9999999999999999, 1])`, "10000000000000000"},
		{"sum_empty", `sum([])`, "0"},

		// product 테스트
		{"product_array", `product([2, 3, 4])`, "24"},
		{"product_precision", `product([0.1, 0.2, 0.3])`, "0.006"},
		{"product_empty", `product([])`, "1"},

		// max 테스트
		{"max_array", `max([1, 3, 2])`, "3"},
		{"max_precision", `max([0.1, 0.11, 0.09])`, "0.11"},
		{"max_large", `max([9999999999999999, 9999999999999998])`, "9999999999999999"},

		// min 테스트
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

	// max/min이 숫자가 아닌 경우에도 작동하는지 확인
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

	// WithStringCoercion()을 사용하지 않으면, 문자열 피연산자는 타입 에러
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

	// JSON input에서 문자열로 온 숫자에 대한 런타임 자동 변환 테스트
	// OPA 컴파일러는 input 값의 타입을 모르므로 (any), 런타임 coercion이 동작함
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

	// 숫자가 아닌 문자열은 여전히 에러 (input을 통해 런타임에 도달)
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

	// 실제 사용 시나리오: JSON input에서 문자열로 온 숫자
	module := `package test
import rego.v1
result := input.qty - input.pos`

	rs := evalModuleResult(t, module, map[string]interface{}{
		"qty": "0.73", // JSON 문자열
		"pos": 0.5,    // JSON 숫자
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

	// UseDecimalArithmetic에서 비교 연산자는 numeric-only이므로
	// 비숫자 비교는 undefined가 되어야 함
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

	// stringCoercion이 활성화되어도 ==, !=에는 적용되지 않음 (OPA 타입 시스템 유지)
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

	// 숫자 문자열과 비숫자 타입(bool)이 혼합된 배열: 모든 요소가 numeric-like가
	// 아니므로 숫자 모드로 가지 않고 기본 비교(ast.Compare) 순서로 폴백한다.
	// 타입 순서(bool < number < string)에 따라 max는 "1"(문자열), min은 true.
	// default/strict 어느 모드에서도 에러 없이 값을 반환해야 한다.
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

	// 빈 배열에 대한 max/min은 undefined 반환
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

			// 빈 컬렉션에 대한 max/min은 undefined이므로 결과가 없어야 함
			if len(rs) != 0 {
				t.Errorf("expected undefined (len(rs)==0) for empty collection, got %d results: %v", len(rs), rs)
			}
		})
	}
}
