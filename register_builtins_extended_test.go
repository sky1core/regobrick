package regobrick

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test WithDefaultDecimal option
func TestWithDefaultDecimalOption(t *testing.T) {
	decimalBuiltin := func(ctx rego.BuiltinContext, amount RegoDecimal) (string, error) {
		return "Processed: " + amount.String(), nil
	}

	funcName := "test_default_decimal_option"
	RegisterBuiltin1[RegoDecimal, string](
		funcName, 
		decimalBuiltin,
		WithDefaultDecimal(),
	)

	policy := `package test
result = ` + funcName + `(input.amount)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with WithDefaultDecimal: %v", err)
		return
	}

	amount := NewRegoDecimalFromInt(100)
	input := map[string]interface{}{
		"amount": amount,
	}

	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		t.Errorf("Failed to evaluate query with WithDefaultDecimal: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(string)
	if !ok {
		t.Errorf("Expected string result, got %T", rs[0].Expressions[0].Value)
		return
	}

	expected := "Processed: 100"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// Test RegisterBuiltin0_ (no args, error-only)
func TestRegisterBuiltin0_(t *testing.T) {
	noArgErrorBuiltin := func(ctx rego.BuiltinContext) error {
		return nil
	}

	funcName := "test_no_arg_error_builtin"
	RegisterBuiltin0_(funcName, noArgErrorBuiltin)

	policy := `package test
result if {
	` + funcName + `()
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin0_: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin0_: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from no-arg error-only builtin condition")
	}
}

// Test RegisterBuiltin1_ (error-only form)
func TestRegisterBuiltin1_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, input string) error {
		if input == "error" {
			return fmt.Errorf("test error")
		}
		return nil
	}

	funcName := "test_error_only_builtin"
	RegisterBuiltin1_[string](funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `("test")
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with error-only builtin: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with error-only builtin: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from error-only builtin condition")
	}
}

// Test RegisterBuiltin2_ (error-only form)
func TestRegisterBuiltin2_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, a string, b int) error {
		if a == "error" || b < 0 {
			return fmt.Errorf("test error")
		}
		return nil
	}

	funcName := "test_error_only_builtin_2"
	RegisterBuiltin2_[string, int](funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `("test", 5)
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin2_: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin2_: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from RegisterBuiltin2_ condition")
	}
}

// Test RegisterBuiltin3_ (error-only form)
func TestRegisterBuiltin3_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, a, b, c int) error {
		if a < 0 || b < 0 || c < 0 {
			return fmt.Errorf("negative values not allowed")
		}
		return nil
	}

	funcName := "test_error_only_builtin_3"
	RegisterBuiltin3_[int, int, int](funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `(1, 2, 3)
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin3_: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin3_: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from RegisterBuiltin3_ condition")
	}
}

// Test RegisterBuiltin4_ (error-only form)
func TestRegisterBuiltin4_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, a, b, c, d int) error {
		if a+b+c+d < 0 {
			return fmt.Errorf("sum is negative")
		}
		return nil
	}

	funcName := "test_error_only_builtin_4"
	RegisterBuiltin4_[int, int, int, int](funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `(1, 2, 3, 4)
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin4_: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin4_: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from RegisterBuiltin4_ condition")
	}
}

// Test RegisterBuiltin5_ (error-only form)
func TestRegisterBuiltin5_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, a, b, c, d, e int) error {
		if a+b+c+d+e < 0 {
			return fmt.Errorf("sum is negative")
		}
		return nil
	}

	funcName := "test_error_only_builtin_5"
	RegisterBuiltin5_[int, int, int, int, int](funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `(1, 2, 3, 4, 5)
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin5_: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin5_: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result from RegisterBuiltin5_ condition")
	}
}

// Test RegisterBuiltin0 (no args, with return value)
func TestRegisterBuiltin0(t *testing.T) {
	noArgBuiltin := func(ctx rego.BuiltinContext) (string, error) {
		return "constant_value", nil
	}

	funcName := "test_no_arg_return_builtin"
	RegisterBuiltin0[string](funcName, noArgBuiltin)

	policy := `package test
result = ` + funcName + `()`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin0: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin0: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(string)
	if !ok {
		t.Errorf("Expected string result, got %T", rs[0].Expressions[0].Value)
		return
	}

	expected := "constant_value"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
