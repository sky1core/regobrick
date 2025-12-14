package regobrick

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

func TestNumber_WithOperatorOverloads(t *testing.T) {
	ensureOperatorOverloadsRegistered()

	// Number 입력으로 Rego 연산이 정상 동작하는지 확인
	tests := []struct {
		name     string
		a, b     Number
		op       string
		expected string
	}{
		{"add", "1.1", "2.2", "+", "3.3"},
		{"sub", "5.5", "2.2", "-", "3.3"},
		{"mul", "2.5", "4", "*", "10"},
		{"div", "10", "4", "/", "2.5"},
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

func TestNumber_ExponentNotation_Error(t *testing.T) {
	ensureOperatorOverloadsRegistered()

	// OPA API로 전달된 json.Number는 정규화되지 않고 그대로 유지됨.
	// 따라서 Number("1e-8")는 연산 시 udecimal.Parse에서 실패.
	//
	// 동작은 OPA 설정에 따라 다름:
	// - 기본 모드: 결과 없음 (규칙 미충족)
	// - StrictBuiltinErrors: eval_builtin_error 발생

	module := `package test
result := input.a + input.b
`
	ctx := context.Background()
	input := map[string]any{
		"a": Number("1e-8"), // 지수표기 - udecimal 파싱 실패
		"b": Number("1"),
	}

	t.Run("default_mode", func(t *testing.T) {
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

		// 기본 모드: 연산 실패 시 결과 없음 (에러 아님)
		if len(rs) > 0 && len(rs[0].Expressions) > 0 {
			t.Error("expected no result due to exponent notation, but got result")
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

		_, err = query.Eval(ctx, rego.EvalInput(input))
		// Strict 모드: eval_builtin_error 발생
		if err == nil {
			t.Error("expected eval error due to exponent notation, but got none")
		} else if !strings.Contains(err.Error(), "eval_builtin_error") {
			t.Fatalf("expected eval_builtin_error, got: %v", err)
		}
	})
}

func TestNumber_LeadingZero_Error(t *testing.T) {
	// Leading zero ("01", "007" 등)는 JSON 스펙에서 invalid
	//
	// 레이어별 동작:
	// - udecimal.Parse("01") → 성공 (udecimal은 leading zero 허용)
	// - ast.InterfaceToValue → 성공 (OPA는 검증 안 함)
	// - encoding/json.Marshal → 실패 (Go JSON이 막음)
	//
	// 결론: rego.EvalInput 처리 중 Go의 json.Marshal에서 에러 발생
	// 참고: exponent notation("1e-8")은 JSON valid → OPA 통과 → udecimal.Parse 실패

	module := `package test
result := input.a + input.b
`
	ctx := context.Background()
	input := map[string]any{
		"a": Number("01"), // Go json.Marshal에서 실패
		"b": Number("1"),
	}

	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	// Go encoding/json이 leading zero를 거부
	_, err = query.Eval(ctx, rego.EvalInput(input))
	if err == nil {
		t.Error("expected json.Marshal error due to leading zero, but got none")
	}
}

func TestNumber_Comparison(t *testing.T) {
	ensureOperatorOverloadsRegistered()

	tests := []struct {
		name     string
		a, b     Number
		op       string
		expected bool
	}{
		{"gt_true", "3.3", "2.2", ">", true},
		{"gt_false", "2.2", "3.3", ">", false},
		{"eq_true", "3.3", "3.3", "==", true},
		{"eq_false", "3.3", "2.2", "==", false},
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
	ensureOperatorOverloadsRegistered()

	tests := []struct {
		name     string
		input    Number
		op       string
		expected string
	}{
		{"abs_neg", "-3.3", "abs(input.n)", "3.3"},
		{"round", "3.5", "round(input.n)", "4"},
		{"ceil", "3.1", "ceil(input.n)", "4"},
		{"floor", "3.9", "floor(input.n)", "3"},
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
	// 다양한 형태의 Number 표현이 동일하게 처리되는지 확인
	// udecimal.Parse가 정규화하므로 동등한 값은 동일하게 비교되어야 함

	tests := []struct {
		name string
		a, b Number
		op   string
		want bool
	}{
		// 정수 표현 차이
		{"int_vs_decimal", "1", "1.0", "==", true},
		{"int_vs_decimal_zeros", "1", "1.00", "==", true},
		{"decimal_zeros", "1.0", "1.00", "==", true},

		// 소수점 표현 차이 (JSON 숫자는 반드시 정수부 필요, ".5"는 invalid)
		{"trailing_zeros", "0.5", "0.50", "==", true},
		{"trailing_zeros2", "0.500", "0.5", "==", true},

		// 음수/양수 0
		{"zero_vs_neg_zero", "0", "-0", "==", true},
		{"zero_variations", "0.0", "0.00", "==", true},

		// 연산 결과 비교
		{"add_format", "1.0", "1", "+", true}, // 1.0 + 1 의 결과가 정상적으로 나오는지
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var module string
			if tt.op == "==" {
				module = `package test
result := input.a == input.b
`
			} else {
				// 연산 테스트: 결과가 나오면 성공
				module = `package test
result := input.a ` + tt.op + ` input.b
`
			}

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

			if tt.op == "==" {
				result := rs[0].Expressions[0].Value.(bool)
				if result != tt.want {
					t.Errorf("got %v, want %v", result, tt.want)
				}
			}
			// 연산의 경우 결과가 있으면 성공
		})
	}
}
