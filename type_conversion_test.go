package regobrick

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/shopspring/decimal"
)

// Test type conversion edge cases
func TestTypeConversionEdgeCases(t *testing.T) {
	// Test various numeric type conversions
	numericBuiltin := func(ctx rego.BuiltinContext, input interface{}) (string, error) {
		return fmt.Sprintf("Type: %T, Value: %v", input, input), nil
	}

	RegisterBuiltin1[interface{}, string]("test_numeric_conversion", numericBuiltin)

	tests := []struct {
		name   string
		policy string
	}{
		{
			name:   "int conversion",
			policy: `result = test_numeric_conversion(42)`,
		},
		{
			name:   "float conversion",
			policy: `result = test_numeric_conversion(42.5)`,
		},
		{
			name:   "large number conversion",
			policy: `result = test_numeric_conversion(9223372036854775807)`,
		},
		{
			name:   "negative number conversion",
			policy: `result = test_numeric_conversion(-123)`,
		},
		{
			name:   "zero conversion",
			policy: `result = test_numeric_conversion(0)`,
		},
		{
			name:   "boolean true conversion",
			policy: `result = test_numeric_conversion(true)`,
		},
		{
			name:   "boolean false conversion",
			policy: `result = test_numeric_conversion(false)`,
		},
		{
			name:   "string conversion",
			policy: `result = test_numeric_conversion("hello")`,
		},
		{
			name:   "empty string conversion",
			policy: `result = test_numeric_conversion("")`,
		},
		{
			name:   "array conversion",
			policy: `result = test_numeric_conversion([1, 2, 3])`,
		},
		{
			name:   "empty array conversion",
			policy: `result = test_numeric_conversion([])`,
		},
		{
			name:   "object conversion",
			policy: `result = test_numeric_conversion({"key": "value"})`,
		},
		{
			name:   "empty object conversion",
			policy: `result = test_numeric_conversion({})`,
		},
		{
			name:   "null conversion",
			policy: `result = test_numeric_conversion(null)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPolicy := `package test
` + tt.policy

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", fullPolicy),
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

			result := rs[0].Expressions[0].Value
			t.Logf("Type conversion test %q result: %v", tt.name, result)
		})
	}
}

// Test RegoDecimal with various input types
func TestRegoDecimalTypeConversion(t *testing.T) {
	decimalBuiltin := func(ctx rego.BuiltinContext, input RegoDecimal) (map[string]interface{}, error) {
		return map[string]interface{}{
			"string_value":    input.String(),
			"is_zero":         input.IsZero(),
			"is_positive":     input.IsPositive(),
			"is_negative":     input.IsNegative(),
			"coefficient":     input.Coefficient().String(),
			"exponent":        input.Exponent(),
		}, nil
	}

	RegisterBuiltin1[RegoDecimal, map[string]interface{}]("test_decimal_conversion", decimalBuiltin)

	tests := []struct {
		name     string
		input    RegoDecimal
		expected map[string]interface{}
	}{
		{
			name:  "positive decimal",
			input: NewRegoDecimal(decimal.NewFromFloat(123.456)),
			expected: map[string]interface{}{
				"string_value": "123.456",
				"is_zero":      false,
				"is_positive":  true,
				"is_negative":  false,
			},
		},
		{
			name:  "negative decimal",
			input: NewRegoDecimal(decimal.NewFromFloat(-78.9)),
			expected: map[string]interface{}{
				"string_value": "-78.9",
				"is_zero":      false,
				"is_positive":  false,
				"is_negative":  true,
			},
		},
		{
			name:  "zero decimal",
			input: NewRegoDecimal(decimal.Zero),
			expected: map[string]interface{}{
				"string_value": "0",
				"is_zero":      true,
				"is_positive":  false,
				"is_negative":  false,
			},
		},
		{
			name:  "very large decimal",
			input: NewRegoDecimalFromInt(9223372036854775807),
			expected: map[string]interface{}{
				"string_value": "9223372036854775807",
				"is_zero":      false,
				"is_positive":  true,
				"is_negative":  false,
			},
		},
		{
			name:  "very small decimal",
			input: NewRegoDecimal(decimal.NewFromFloat(0.000000001)),
			expected: map[string]interface{}{
				"is_zero":     false,
				"is_positive": true,
				"is_negative": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_decimal_conversion(input.value)`

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				t.Errorf("Failed to prepare query: %v", err)
				return
			}

			input := map[string]interface{}{
				"value": tt.input,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Errorf("Failed to evaluate query: %v", err)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Errorf("Expected 1 result, got %d", len(rs))
				return
			}

			result := rs[0].Expressions[0].Value.(map[string]interface{})
			
			// Check expected values
			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Expected key %q not found in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("For key %q: expected %v, got %v", key, expectedValue, actualValue)
				}
			}

			t.Logf("Decimal conversion test %q result: %v", tt.name, result)
		})
	}
}

// Test time.Time conversion edge cases
func TestTimeTypeConversion(t *testing.T) {
	timeBuiltin := func(ctx rego.BuiltinContext, input time.Time) (map[string]interface{}, error) {
		return map[string]interface{}{
			"unix_timestamp": input.Unix(),
			"year":           input.Year(),
			"month":          int(input.Month()),
			"day":            input.Day(),
			"hour":           input.Hour(),
			"minute":         input.Minute(),
			"second":         input.Second(),
			"weekday":        input.Weekday().String(),
			"is_zero":        input.IsZero(),
			"rfc3339":        input.Format(time.RFC3339),
		}, nil
	}

	RegisterBuiltin1[time.Time, map[string]interface{}]("test_time_conversion", timeBuiltin)

	// Test with current time
	policy := `package test
result = test_time_conversion(input.time_value)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	input := map[string]interface{}{
		"time_value": testTime,
	}

	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	
	// Verify some expected values
	expectedValues := map[string]interface{}{
		"year":    json.Number("2023"),
		"month":   json.Number("12"),
		"day":     json.Number("25"),
		"hour":    json.Number("15"),
		"minute":  json.Number("30"),
		"second":  json.Number("45"),
		"weekday": "Monday",
		"is_zero": false,
	}

	for key, expectedValue := range expectedValues {
		if actualValue, ok := result[key]; !ok {
			t.Errorf("Expected key %q not found in result", key)
		} else if actualValue != expectedValue {
			t.Errorf("For key %q: expected %v (%T), got %v (%T)", key, expectedValue, expectedValue, actualValue, actualValue)
		}
	}

	t.Logf("Time conversion result: %v", result)
}

// Test complex nested type conversions
func TestComplexTypeConversions(t *testing.T) {
	type NestedStruct struct {
		ID       int                    `json:"id"`
		Name     string                 `json:"name"`
		Values   []float64              `json:"values"`
		Metadata map[string]interface{} `json:"metadata"`
		Created  time.Time              `json:"created"`
		Amount   RegoDecimal            `json:"amount"`
	}

	complexBuiltin := func(ctx rego.BuiltinContext, input NestedStruct) (map[string]interface{}, error) {
		return map[string]interface{}{
			"id_type":       fmt.Sprintf("%T", input.ID),
			"name_length":   len(input.Name),
			"values_count":  len(input.Values),
			"metadata_keys": len(input.Metadata),
			"created_year":  input.Created.Year(),
			"amount_string": input.Amount.String(),
		}, nil
	}

	RegisterBuiltin1[NestedStruct, map[string]interface{}]("test_complex_conversion", complexBuiltin)

	policy := `package test
result = test_complex_conversion(input.complex_data)`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query: %v", err)
		return
	}

	complexData := NestedStruct{
		ID:     123,
		Name:   "test_struct",
		Values: []float64{1.1, 2.2, 3.3},
		Metadata: map[string]interface{}{
			"version": "1.0",
			"active":  true,
		},
		Created: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Amount:  NewRegoDecimal(decimal.NewFromFloat(999.99)),
	}

	input := map[string]interface{}{
		"complex_data": complexData,
	}

	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		t.Errorf("Failed to evaluate query: %v", err)
		return
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	
	expectedValues := map[string]interface{}{
		"name_length":   json.Number("11"), // "test_struct" length
		"values_count":  json.Number("3"),
		"metadata_keys": json.Number("2"),
		"created_year":  json.Number("2023"),
		"amount_string": "999.99",
	}

	for key, expectedValue := range expectedValues {
		if actualValue, ok := result[key]; !ok {
			t.Errorf("Expected key %q not found in result", key)
		} else if actualValue != expectedValue {
			t.Errorf("For key %q: expected %v (%T), got %v (%T)", key, expectedValue, expectedValue, actualValue, actualValue)
		}
	}

	t.Logf("Complex type conversion result: %v", result)
}

// Test error conditions in type conversion
func TestTypeConversionErrors(t *testing.T) {
	// Test builtin that expects specific type but gets wrong type
	strictTypeBuiltin := func(ctx rego.BuiltinContext, input int) (string, error) {
		return fmt.Sprintf("Number: %d", input), nil
	}

	RegisterBuiltin1[int, string]("test_strict_type", strictTypeBuiltin)

	// This should work with integer
	policy1 := `package test
result = test_strict_type(42)`

	ctx := context.Background()
	query1, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy1),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query1: %v", err)
		return
	}

	rs1, err := query1.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query1: %v", err)
		return
	}

	if len(rs1) != 1 || len(rs1[0].Expressions) != 1 {
		t.Errorf("Expected 1 result for valid case, got %d", len(rs1))
		return
	}

	result1 := rs1[0].Expressions[0].Value.(string)
	expected1 := "Number: 42"
	if result1 != expected1 {
		t.Errorf("Expected %q, got %q", expected1, result1)
	}

	// Test with string input (should fail at compile time due to type mismatch)
	policy2 := `package test
result = test_strict_type("not_a_number")`

	_, err = rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy2),
	).PrepareForEval(ctx)

	// This should fail during preparation due to type mismatch
	if err == nil {
		t.Error("Expected error for type mismatch during preparation, but got none")
	} else {
		t.Logf("Got expected type error during preparation: %v", err)
		// Verify it's a type error
		if !strings.Contains(err.Error(), "rego_type_error") {
			t.Errorf("Expected rego_type_error, got: %v", err)
		}
	}

	// Test with float input (might be converted to int by OPA)
	policy3 := `package test
result = test_strict_type(42.0)`

	query3, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy3),
	).PrepareForEval(ctx)

	if err != nil {
		t.Logf("Float to int conversion failed at preparation: %v", err)
		// This might be expected depending on OPA's type system
	} else {
		rs3, err := query3.Eval(ctx)
		if err != nil {
			t.Logf("Float to int conversion failed at evaluation: %v", err)
		} else {
			t.Logf("Float to int conversion succeeded: %v", rs3)
		}
	}
}
