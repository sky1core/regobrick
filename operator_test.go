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
	operatorOnce.Do(UseDecimalArithmetic)
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
		{"round_3.5", "round(3.5)", "4"},    // 3.5 → 4
		{"round_2.5", "round(2.5)", "3"},    // 2.5 → 3
		{"round_1.5", "round(1.5)", "2"},    // 1.5 → 2
		{"round_0.5", "round(0.5)", "1"},    // 0.5 → 1
		{"round_4.5", "round(4.5)", "5"},    // 4.5 → 5
		{"round_3.4", "round(3.4)", "3"},    // 일반 반올림
		{"round_3.6", "round(3.6)", "4"},    // 일반 반올림
		{"round_neg", "round(-2.5)", "-3"},  // -2.5 → -3 (away from zero)
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
		// float64 대표 문제: 0.1 + 0.2 != 0.3
		{"float_classic", "0.1 + 0.2", "0.3"},

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
