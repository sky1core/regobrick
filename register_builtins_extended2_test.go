package regobrick

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test RegisterBuiltin4 (4 args, with return value)
func TestRegisterBuiltin4(t *testing.T) {
	fourArgBuiltin := func(ctx rego.BuiltinContext, a, b, c, d int) (int, error) {
		return a + b + c + d, nil
	}

	funcName := "test_four_arg_builtin"
	RegisterBuiltin4[int, int, int, int, int](funcName, fourArgBuiltin)

	policy := `package test
result = ` + funcName + `(1, 2, 3, 4)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin4: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin4: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	// Handle different numeric types that OPA might return
	switch result := rs[0].Expressions[0].Value.(type) {
	case int:
		if result != 10 {
			t.Errorf("Expected 10, got %d", result)
		}
	case int64:
		if result != 10 {
			t.Errorf("Expected 10, got %d", result)
		}
	case float64:
		if result != 10.0 {
			t.Errorf("Expected 10.0, got %f", result)
		}
	case json.Number:
		if result.String() != "10" {
			t.Errorf("Expected 10, got %s", result.String())
		}
	default:
		t.Errorf("Expected numeric result, got %T: %v", result, result)
	}
}

// Test RegisterBuiltin5 (5 args, with return value)
func TestRegisterBuiltin5(t *testing.T) {
	fiveArgBuiltin := func(ctx rego.BuiltinContext, a, b, c, d, e int) (int, error) {
		return a + b + c + d + e, nil
	}

	funcName := "test_five_arg_builtin"
	RegisterBuiltin5[int, int, int, int, int, int](funcName, fiveArgBuiltin)

	policy := `package test
result = ` + funcName + `(1, 2, 3, 4, 5)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegisterBuiltin5: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with RegisterBuiltin5: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result with 1 expression, got %d results", len(rs))
		return
	}

	// Handle different numeric types that OPA might return
	switch result := rs[0].Expressions[0].Value.(type) {
	case int:
		if result != 15 {
			t.Errorf("Expected 15, got %d", result)
		}
	case int64:
		if result != 15 {
			t.Errorf("Expected 15, got %d", result)
		}
	case float64:
		if result != 15.0 {
			t.Errorf("Expected 15.0, got %f", result)
		}
	case json.Number:
		if result.String() != "15" {
			t.Errorf("Expected 15, got %s", result.String())
		}
	default:
		t.Errorf("Expected numeric result, got %T: %v", result, result)
	}
}

// Test RegisterBuiltin4 with different types
func TestRegisterBuiltin4WithMixedTypes(t *testing.T) {
	mixedBuiltin := func(ctx rego.BuiltinContext, name string, age int, active bool, score float64) (string, error) {
		status := "inactive"
		if active {
			status = "active"
		}
		return name + " is " + status, nil
	}

	funcName := "test_mixed_builtin"
	RegisterBuiltin4[string, int, bool, float64, string](funcName, mixedBuiltin)

	policy := `package test
result = ` + funcName + `("Alice", 25, true, 95.5)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with mixed types: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with mixed types: %v", err)
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

	expected := "Alice is active"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// Test RegisterBuiltin5 with RegoDecimal
func TestRegisterBuiltin5WithRegoDecimal(t *testing.T) {
	decimalBuiltin := func(ctx rego.BuiltinContext, a, b, c RegoDecimal, multiplier int, name string) (string, error) {
		sum := a.Decimal.Add(b.Decimal).Add(c.Decimal)
		total := sum.Mul(NewRegoDecimalFromInt(int64(multiplier)).Decimal)
		return name + ": " + total.String(), nil
	}

	funcName := "test_decimal_builtin_5"
	RegisterBuiltin5[RegoDecimal, RegoDecimal, RegoDecimal, int, string, string](funcName, decimalBuiltin)

	policy := `package test
result = ` + funcName + `(input.a, input.b, input.c, input.multiplier, input.name)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with RegoDecimal: %v", err)
		return
	}

	input := map[string]interface{}{
		"a":          NewRegoDecimalFromInt(10),
		"b":          NewRegoDecimalFromInt(20),
		"c":          NewRegoDecimalFromInt(30),
		"multiplier": 2,
		"name":       "Total",
	}

	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		t.Errorf("Failed to evaluate query with RegoDecimal: %v", err)
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

	expected := "Total: 120"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
