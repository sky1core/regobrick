package regobrick

import (
	"context"
	"encoding/json"
	"fmt"
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

// Test basic type validation - simplified version
func TestBasicTypeValidation(t *testing.T) {
	// Test builtin that expects int
	intBuiltin := func(ctx rego.BuiltinContext, input int) (string, error) {
		return fmt.Sprintf("Got int: %d", input), nil
	}
	RegisterBuiltin1[int, string]("test_int_only", intBuiltin)

	// Test builtin that expects string
	stringBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return fmt.Sprintf("Got string: %s", input), nil
	}
	RegisterBuiltin1[string, string]("test_string_only", stringBuiltin)

	tests := []struct {
		name        string
		policy      string
		expectError bool
	}{
		{
			name:        "valid int",
			policy:      `result = test_int_only(42)`,
			expectError: false,
		},
		{
			name:        "valid string",
			policy:      `result = test_string_only("hello")`,
			expectError: false,
		},
		{
			name:        "string to int builtin",
			policy:      `result = test_int_only("not_a_number")`,
			expectError: true,
		},
		{
			name:        "int to string builtin",
			policy:      `result = test_string_only(123)`,
			expectError: true,
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

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				} else {
					t.Logf("Got expected error for %s: %v", tt.name, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
				return
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Errorf("Failed to evaluate %s: %v", tt.name, err)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Errorf("Expected 1 result for %s, got %d", tt.name, len(rs))
				return
			}

			result := rs[0].Expressions[0].Value
			t.Logf("Basic type validation %s result: %v", tt.name, result)
		})
	}
}
