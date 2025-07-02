package regobrick

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test RegisterBuiltin2_ with all options to cover remaining lines
func TestRegisterBuiltin2_AllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a string, b int) error {
		if a == "error" || b < 0 {
			return nil // This will make the condition fail
		}
		return nil
	}

	funcName := "test_builtin2_error_all_opts"
	RegisterBuiltin2_[string, int](
		funcName,
		testBuiltin,
		WithCategories("validation", "error_check"),
		WithNondeterministic(),
	)

	policy := `package test
result if {
	` + funcName + `("valid", 5)
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result")
	}
}

// Test RegisterBuiltin3_ with all options
func TestRegisterBuiltin3_AllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c int) error {
		if a < 0 || b < 0 || c < 0 {
			return nil
		}
		return nil
	}

	funcName := "test_builtin3_error_all_opts"
	RegisterBuiltin3_[int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "validation"),
		WithNondeterministic(),
	)

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
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result")
	}
}

// Test RegisterBuiltin4_ with all options
func TestRegisterBuiltin4_AllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c, d int) error {
		if a+b+c+d < 0 {
			return nil
		}
		return nil
	}

	funcName := "test_builtin4_error_all_opts"
	RegisterBuiltin4_[int, int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "sum_validation"),
		WithNondeterministic(),
	)

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
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result")
	}
}

// Test RegisterBuiltin5_ with all options
func TestRegisterBuiltin5_AllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c, d, e int) error {
		if a+b+c+d+e < 0 {
			return nil
		}
		return nil
	}

	funcName := "test_builtin5_error_all_opts"
	RegisterBuiltin5_[int, int, int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "sum_validation", "five_args"),
		WithNondeterministic(),
	)

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
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Errorf("Expected true result")
	}
}

// Test RegisterBuiltin4 with all options (return value version)
func TestRegisterBuiltin4_ReturnAllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c, d int) (int, error) {
		return a + b + c + d, nil
	}

	funcName := "test_builtin4_return_all_opts"
	RegisterBuiltin4[int, int, int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "addition"),
		WithNondeterministic(),
	)

	policy := `package test
result = ` + funcName + `(10, 20, 30, 40)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	// Handle different numeric types
	switch result := rs[0].Expressions[0].Value.(type) {
	case int:
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	case int64:
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	case float64:
		if result != 100.0 {
			t.Errorf("Expected 100.0, got %f", result)
		}
	case json.Number:
		if result.String() != "100" {
			t.Errorf("Expected 100, got %s", result.String())
		}
	default:
		t.Errorf("Expected numeric result, got %T: %v", result, result)
	}
}

// Test RegisterBuiltin5 with all options (return value version)
func TestRegisterBuiltin5_ReturnAllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c, d, e int) (int, error) {
		return a + b + c + d + e, nil
	}

	funcName := "test_builtin5_return_all_opts"
	RegisterBuiltin5[int, int, int, int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "addition", "five_params"),
		WithNondeterministic(),
	)

	policy := `package test
result = ` + funcName + `(10, 20, 30, 40, 50)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	// Handle different numeric types
	switch result := rs[0].Expressions[0].Value.(type) {
	case int:
		if result != 150 {
			t.Errorf("Expected 150, got %d", result)
		}
	case int64:
		if result != 150 {
			t.Errorf("Expected 150, got %d", result)
		}
	case float64:
		if result != 150.0 {
			t.Errorf("Expected 150.0, got %f", result)
		}
	case json.Number:
		if result.String() != "150" {
			t.Errorf("Expected 150, got %s", result.String())
		}
	default:
		t.Errorf("Expected numeric result, got %T: %v", result, result)
	}
}
