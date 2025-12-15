package regobrick

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// =============================================================================
// Number 단위 테스트 (Scan, Value, JSON)
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
	// json.Number는 빈 문자열을 0으로 출력 (Go 관례)
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
	// json.Number는 null을 빈 문자열로 처리.
	// 빈 문자열은 MarshalJSON에서 0으로 출력.
	var n Number
	err := json.Unmarshal([]byte("null"), &n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "" {
		t.Errorf("expected empty string, got %q", n.String())
	}
	// 빈 문자열 → 0으로 출력 (Go 관례)
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
	// 숫자 리터럴 (따옴표 없이)
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
	// JSON Unmarshal이 float64를 거치지 않고 정밀도를 보존하는지 확인
	type Record struct {
		Val Number `json:"val"`
	}
	// 38자리 - float64는 약 15~17자리만 표현 가능
	input := `{"val": 12345678901234567890123456789012345678.12}`
	var r Record
	err := json.Unmarshal([]byte(input), &r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 정밀도가 보존되어야 함
	expected := "12345678901234567890123456789012345678.12"
	if r.Val.String() != expected {
		t.Errorf("precision loss: got %s, want %s", r.Val.String(), expected)
	}
}

func TestNumber_Roundtrip_ScanValue(t *testing.T) {
	// Scan → Value 왕복 테스트
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
	// Marshal → Unmarshal 왕복 테스트
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
// Rego 통합 테스트 (기존)
// =============================================================================

func TestNumber_WithDecimalOperators(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Number 입력으로 Rego 연산이 정상 동작하는지 확인
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

func TestNumber_ExponentNotation_Error(t *testing.T) {
	ensureDecimalArithmeticEnabled()

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
	// 다양한 형태의 Number 표현이 동일하게 처리되는지 확인
	// udecimal.Parse가 정규화하므로 동등한 값은 동일하게 비교되어야 함

	// 비교 테스트
	comparisonTests := []struct {
		name     string
		a, b     Number
		wantEq   bool
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

	// 연산 테스트 (기대 결과 명시)
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
