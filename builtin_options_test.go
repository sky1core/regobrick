package regobrick

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test RegisterBuiltin2 with all options
func TestRegisterBuiltin2WithAllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a string, b int) (string, error) {
		result := ""
		for i := 0; i < b; i++ {
			result += a
		}
		return result, nil
	}

	funcName := "test_builtin2_all_options"
	RegisterBuiltin2[string, int, string](
		funcName,
		testBuiltin,
		WithCategories("test_category", "custom_category"),
		WithNondeterministic(),
	)

	policy := `package test
result = ` + funcName + `("hi", 3)`

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

	result, ok := rs[0].Expressions[0].Value.(string)
	if !ok {
		t.Errorf("Expected string result, got %T", rs[0].Expressions[0].Value)
		return
	}

	expected := "hihihi"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Check that categories were stored
	if cats, ok := customBuiltinCategories[funcName]; ok {
		if len(cats) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(cats))
		}
		if cats[0] != "test_category" || cats[1] != "custom_category" {
			t.Errorf("Expected [test_category, custom_category], got %v", cats)
		}
	} else {
		t.Errorf("Categories not stored for builtin %s", funcName)
	}
}

// Test RegisterBuiltin3 with all options
func TestRegisterBuiltin3WithAllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext, a, b, c int) (int, error) {
		return a + b + c, nil
	}

	funcName := "test_builtin3_all_options"
	RegisterBuiltin3[int, int, int, int](
		funcName,
		testBuiltin,
		WithCategories("math", "arithmetic"),
		WithNondeterministic(),
	)

	policy := `package test
result = ` + funcName + `(10, 20, 30)`

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
		if result != 60 {
			t.Errorf("Expected 60, got %d", result)
		}
	case int64:
		if result != 60 {
			t.Errorf("Expected 60, got %d", result)
		}
	case float64:
		if result != 60.0 {
			t.Errorf("Expected 60.0, got %f", result)
		}
	case json.Number:
		if result.String() != "60" {
			t.Errorf("Expected 60, got %s", result.String())
		}
	default:
		t.Errorf("Expected numeric result, got %T: %v", result, result)
	}

	// Check that categories were stored
	if cats, ok := customBuiltinCategories[funcName]; ok {
		if len(cats) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(cats))
		}
		if cats[0] != "math" || cats[1] != "arithmetic" {
			t.Errorf("Expected [math, arithmetic], got %v", cats)
		}
	} else {
		t.Errorf("Categories not stored for builtin %s", funcName)
	}
}

// Test RegisterBuiltin0 with all options
func TestRegisterBuiltin0WithAllOptions(t *testing.T) {
	testBuiltin := func(ctx rego.BuiltinContext) (string, error) {
		return "zero_args_result", nil
	}

	funcName := "test_builtin0_all_options"
	RegisterBuiltin0[string](
		funcName,
		testBuiltin,
		WithCategories("utility", "constants"),
		WithNondeterministic(),
	)

	policy := `package test
result = ` + funcName + `()`

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

	result, ok := rs[0].Expressions[0].Value.(string)
	if !ok {
		t.Errorf("Expected string result, got %T", rs[0].Expressions[0].Value)
		return
	}

	expected := "zero_args_result"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Check that categories were stored
	if cats, ok := customBuiltinCategories[funcName]; ok {
		if len(cats) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(cats))
		}
		if cats[0] != "utility" || cats[1] != "constants" {
			t.Errorf("Expected [utility, constants], got %v", cats)
		}
	} else {
		t.Errorf("Categories not stored for builtin %s", funcName)
	}
}

// Test error-only builtins with all options
func TestErrorOnlyBuiltinsWithAllOptions(t *testing.T) {
	// Test RegisterBuiltin0_ with all options
	testBuiltin0 := func(ctx rego.BuiltinContext) error {
		return nil
	}

	funcName0 := "test_error_builtin0_all_options"
	RegisterBuiltin0_(
		funcName0,
		testBuiltin0,
		WithCategories("validation", "checks"),
		WithNondeterministic(),
	)

	// Test RegisterBuiltin1_ with all options
	testBuiltin1 := func(ctx rego.BuiltinContext, input string) error {
		if input == "invalid" {
			return nil // This will cause the condition to fail
		}
		return nil
	}

	funcName1 := "test_error_builtin1_all_options"
	RegisterBuiltin1_[string](
		funcName1,
		testBuiltin1,
		WithCategories("validation", "string_checks"),
		WithNondeterministic(),
	)

	// Test the error-only builtins in conditions
	policy := `package test
result0 if {
	` + funcName0 + `()
}

result1 if {
	` + funcName1 + `("valid")
}`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test"),
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

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	
	if result["result0"] != true {
		t.Errorf("Expected result0=true, got %v", result["result0"])
	}
	
	if result["result1"] != true {
		t.Errorf("Expected result1=true, got %v", result["result1"])
	}

	// Check that categories were stored for both
	if cats, ok := customBuiltinCategories[funcName0]; ok {
		if len(cats) != 2 {
			t.Errorf("Expected 2 categories for %s, got %d", funcName0, len(cats))
		}
	} else {
		t.Errorf("Categories not stored for builtin %s", funcName0)
	}

	if cats, ok := customBuiltinCategories[funcName1]; ok {
		if len(cats) != 2 {
			t.Errorf("Expected 2 categories for %s, got %d", funcName1, len(cats))
		}
	} else {
		t.Errorf("Categories not stored for builtin %s", funcName1)
	}
}

// Test storeCustomCategories function edge cases
func TestStoreCustomCategoriesEdgeCases(t *testing.T) {
	// Test with empty categories
	storeCustomCategories("test_empty_categories", []string{})
	if cats, ok := customBuiltinCategories["test_empty_categories"]; !ok {
		t.Error("Empty categories should still be stored")
	} else if cats != nil && len(cats) != 0 {
		t.Errorf("Expected empty categories, got %v", cats)
	}

	// Test with nil categories
	storeCustomCategories("test_nil_categories", nil)
	if cats, ok := customBuiltinCategories["test_nil_categories"]; !ok {
		t.Error("Nil categories should still be stored")
	} else if cats != nil {
		t.Errorf("Expected nil categories, got %v", cats)
	}

	// Test with single category
	storeCustomCategories("test_single_category", []string{"single"})
	if cats, ok := customBuiltinCategories["test_single_category"]; !ok {
		t.Error("Single category should be stored")
	} else if len(cats) != 1 || cats[0] != "single" {
		t.Errorf("Expected [single], got %v", cats)
	}

	// Test overwriting existing categories
	storeCustomCategories("test_overwrite", []string{"original"})
	storeCustomCategories("test_overwrite", []string{"new1", "new2"})
	if cats, ok := customBuiltinCategories["test_overwrite"]; !ok {
		t.Error("Overwritten categories should be stored")
	} else if len(cats) != 2 || cats[0] != "new1" || cats[1] != "new2" {
		t.Errorf("Expected [new1, new2], got %v", cats)
	}
}
