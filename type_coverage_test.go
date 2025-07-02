package regobrick

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test regoTypeOf function with all supported types
func TestRegoTypeOfAllTypes(t *testing.T) {
	// Test bool type
	boolBuiltin := func(ctx rego.BuiltinContext, input bool) (bool, error) {
		return !input, nil
	}
	RegisterBuiltin1[bool, bool]("test_bool_type", boolBuiltin)

	// Test int types
	intBuiltin := func(ctx rego.BuiltinContext, input int) (int, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[int, int]("test_int_type", intBuiltin)

	int8Builtin := func(ctx rego.BuiltinContext, input int8) (int8, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[int8, int8]("test_int8_type", int8Builtin)

	int16Builtin := func(ctx rego.BuiltinContext, input int16) (int16, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[int16, int16]("test_int16_type", int16Builtin)

	int32Builtin := func(ctx rego.BuiltinContext, input int32) (int32, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[int32, int32]("test_int32_type", int32Builtin)

	int64Builtin := func(ctx rego.BuiltinContext, input int64) (int64, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[int64, int64]("test_int64_type", int64Builtin)

	// Test uint types
	uintBuiltin := func(ctx rego.BuiltinContext, input uint) (uint, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[uint, uint]("test_uint_type", uintBuiltin)

	uint8Builtin := func(ctx rego.BuiltinContext, input uint8) (uint8, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[uint8, uint8]("test_uint8_type", uint8Builtin)

	uint16Builtin := func(ctx rego.BuiltinContext, input uint16) (uint16, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[uint16, uint16]("test_uint16_type", uint16Builtin)

	uint32Builtin := func(ctx rego.BuiltinContext, input uint32) (uint32, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[uint32, uint32]("test_uint32_type", uint32Builtin)

	uint64Builtin := func(ctx rego.BuiltinContext, input uint64) (uint64, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[uint64, uint64]("test_uint64_type", uint64Builtin)

	// Test float types
	float32Builtin := func(ctx rego.BuiltinContext, input float32) (float32, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[float32, float32]("test_float32_type", float32Builtin)

	float64Builtin := func(ctx rego.BuiltinContext, input float64) (float64, error) {
		return input * 2, nil
	}
	RegisterBuiltin1[float64, float64]("test_float64_type", float64Builtin)

	// Test string type
	stringBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return "processed: " + input, nil
	}
	RegisterBuiltin1[string, string]("test_string_type", stringBuiltin)

	// Test RegoDecimal type
	decimalBuiltin := func(ctx rego.BuiltinContext, input RegoDecimal) (RegoDecimal, error) {
		doubled := input.Decimal.Mul(NewRegoDecimalFromInt(2).Decimal)
		return NewRegoDecimal(doubled), nil
	}
	RegisterBuiltin1[RegoDecimal, RegoDecimal]("test_regodecimal_type", decimalBuiltin)

	// Test time.Time type - this tests the regoTypeOf function for time.Time
	// We need to create a builtin that actually uses time.Time as parameter type
	timeBuiltin := func(ctx rego.BuiltinContext, input time.Time) (string, error) {
		return input.Format("2006-01-02"), nil
	}
	RegisterBuiltin1[time.Time, string]("test_time_type", timeBuiltin)

	// Test time.Time type with current time (this will test the time.Time case in regoTypeOf)
	currentTimeBuiltin := func(ctx rego.BuiltinContext) (time.Time, error) {
		return time.Now(), nil
	}
	RegisterBuiltin0[time.Time]("test_current_time", currentTimeBuiltin)

	// Test custom struct type (should default to types.A)
	type CustomStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	structBuiltin := func(ctx rego.BuiltinContext, input CustomStruct) (string, error) {
		return input.Name, nil
	}
	RegisterBuiltin1[CustomStruct, string]("test_struct_type", structBuiltin)

	// Test slice type (should default to types.A)
	sliceBuiltin := func(ctx rego.BuiltinContext, input []string) (int, error) {
		return len(input), nil
	}
	RegisterBuiltin1[[]string, int]("test_slice_type", sliceBuiltin)

	// Test map type (should default to types.A)
	mapBuiltin := func(ctx rego.BuiltinContext, input map[string]interface{}) (int, error) {
		return len(input), nil
	}
	RegisterBuiltin1[map[string]interface{}, int]("test_map_type", mapBuiltin)

	// Now test some of these builtins to ensure they work
	t.Run("bool_builtin", func(t *testing.T) {
		policy := `package test
result = test_bool_type(true)`

		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", policy),
		).PrepareForEval(ctx)

		if err != nil {
			t.Errorf("Failed to prepare bool query: %v", err)
			return
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Errorf("Failed to evaluate bool query: %v", err)
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

		if result != false {
			t.Errorf("Expected false, got %v", result)
		}
	})

	t.Run("string_builtin", func(t *testing.T) {
		policy := `package test
result = test_string_type("hello")`

		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", policy),
		).PrepareForEval(ctx)

		if err != nil {
			t.Errorf("Failed to prepare string query: %v", err)
			return
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Errorf("Failed to evaluate string query: %v", err)
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

		expected := "processed: hello"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})

	t.Run("decimal_builtin", func(t *testing.T) {
		policy := `package test
result = test_regodecimal_type(input.value)`

		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", policy),
		).PrepareForEval(ctx)

		if err != nil {
			t.Errorf("Failed to prepare decimal query: %v", err)
			return
		}

		input := map[string]interface{}{
			"value": NewRegoDecimalFromInt(50),
		}

		rs, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			t.Errorf("Failed to evaluate decimal query: %v", err)
			return
		}

		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			t.Errorf("Expected 1 result, got %d", len(rs))
			return
		}

		// The result might be converted to a different numeric type by OPA
		t.Logf("Decimal result type: %T, value: %v", rs[0].Expressions[0].Value, rs[0].Expressions[0].Value)
	})

	t.Run("time_builtin", func(t *testing.T) {
		// Test the current time builtin which returns time.Time
		policy := `package test
result = test_current_time()`

		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", policy),
		).PrepareForEval(ctx)

		if err != nil {
			t.Errorf("Failed to prepare time query: %v", err)
			return
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Errorf("Failed to evaluate time query: %v", err)
			return
		}

		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			t.Errorf("Expected 1 result, got %d", len(rs))
			return
		}

		// time.Time will be converted to string by OPA
		result, ok := rs[0].Expressions[0].Value.(string)
		if !ok {
			t.Errorf("Expected string result (converted from time.Time), got %T", rs[0].Expressions[0].Value)
			return
		}

		// The result should be an RFC3339 formatted time string
		t.Logf("Time builtin result: %s", result)
		if len(result) < 10 { // Should be at least YYYY-MM-DD
			t.Errorf("Expected time string, got %s", result)
		}
	})

	t.Run("slice_builtin", func(t *testing.T) {
		policy := `package test
result = test_slice_type(["a", "b", "c"])`

		ctx := context.Background()
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", policy),
		).PrepareForEval(ctx)

		if err != nil {
			t.Errorf("Failed to prepare slice query: %v", err)
			return
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Errorf("Failed to evaluate slice query: %v", err)
			return
		}

		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			t.Errorf("Expected 1 result, got %d", len(rs))
			return
		}

		// Handle different numeric types
		switch result := rs[0].Expressions[0].Value.(type) {
		case int:
			if result != 3 {
				t.Errorf("Expected 3, got %d", result)
			}
		case int64:
			if result != 3 {
				t.Errorf("Expected 3, got %d", result)
			}
		case float64:
			if result != 3.0 {
				t.Errorf("Expected 3.0, got %f", result)
			}
		case json.Number:
			if result.String() != "3" {
				t.Errorf("Expected 3, got %s", result.String())
			}
		default:
			t.Errorf("Expected numeric result, got %T: %v", result, result)
		}
	})
}
