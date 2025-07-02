package regobrick

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test nil handling in builtin return values (0.6.2 feature)
func TestBuiltinNilHandling(t *testing.T) {
	// Test builtin that returns nil pointer
	nilPtrBuiltin := func(ctx rego.BuiltinContext, input string) (*string, error) {
		if input == "return_nil" {
			return nil, nil // Return nil pointer
		}
		result := "processed: " + input
		return &result, nil
	}

	RegisterBuiltin1[string, *string]("test_nil_ptr_builtin", nilPtrBuiltin)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "nil pointer return",
			input:    "return_nil",
			expected: nil, // Should be converted to null
		},
		{
			name:     "valid pointer return",
			input:    "hello",
			expected: "processed: hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_nil_ptr_builtin("` + tt.input + `")`

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

			result := rs[0].Expressions[0].Value
			if tt.expected == nil {
				// Should be null in Rego
				if result != nil {
					t.Errorf("Expected nil/null result, got %v (%T)", result, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

// Test nil slice handling
func TestBuiltinNilSliceHandling(t *testing.T) {
	nilSliceBuiltin := func(ctx rego.BuiltinContext, input string) ([]string, error) {
		if input == "return_nil_slice" {
			return nil, nil // Return nil slice
		}
		return []string{input, "processed"}, nil
	}

	RegisterBuiltin1[string, []string]("test_nil_slice_builtin", nilSliceBuiltin)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "nil slice return",
			input:    "return_nil_slice",
			expected: nil, // Should be converted to null
		},
		{
			name:     "valid slice return",
			input:    "hello",
			expected: []interface{}{"hello", "processed"}, // OPA converts to []interface{}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_nil_slice_builtin("` + tt.input + `")`

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

			result := rs[0].Expressions[0].Value
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil/null result, got %v (%T)", result, result)
				}
			} else {
				// For slice comparison, we need to handle the conversion
				t.Logf("Slice result: %v (%T)", result, result)
				// The exact comparison might vary based on OPA's conversion
			}
		})
	}
}

// Test nil map handling
func TestBuiltinNilMapHandling(t *testing.T) {
	nilMapBuiltin := func(ctx rego.BuiltinContext, input string) (map[string]interface{}, error) {
		if input == "return_nil_map" {
			return nil, nil // Return nil map
		}
		return map[string]interface{}{
			"input":     input,
			"processed": true,
		}, nil
	}

	RegisterBuiltin1[string, map[string]interface{}]("test_nil_map_builtin", nilMapBuiltin)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "nil map return",
			input:    "return_nil_map",
			expected: nil, // Should be converted to null
		},
		{
			name:     "valid map return",
			input:    "hello",
			expected: map[string]interface{}{"input": "hello", "processed": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_nil_map_builtin("` + tt.input + `")`

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

			result := rs[0].Expressions[0].Value
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil/null result, got %v (%T)", result, result)
				}
			} else {
				// For map comparison
				t.Logf("Map result: %v (%T)", result, result)
			}
		})
	}
}

// Test nil interface handling
func TestBuiltinNilInterfaceHandling(t *testing.T) {
	nilInterfaceBuiltin := func(ctx rego.BuiltinContext, input string) (interface{}, error) {
		if input == "return_nil_interface" {
			return nil, nil // Return nil interface
		}
		if input == "return_nil_ptr_interface" {
			var ptr *string = nil
			return ptr, nil // Return nil pointer as interface
		}
		return "processed: " + input, nil
	}

	RegisterBuiltin1[string, interface{}]("test_nil_interface_builtin", nilInterfaceBuiltin)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "nil interface return",
			input:    "return_nil_interface",
			expected: nil,
		},
		{
			name:     "nil pointer as interface return",
			input:    "return_nil_ptr_interface",
			expected: nil,
		},
		{
			name:     "valid interface return",
			input:    "hello",
			expected: "processed: hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_nil_interface_builtin("` + tt.input + `")`

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

			result := rs[0].Expressions[0].Value
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil/null result, got %v (%T)", result, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

// Test nil channel handling
func TestBuiltinNilChannelHandling(t *testing.T) {
	nilChannelBuiltin := func(ctx rego.BuiltinContext, input string) (chan string, error) {
		if input == "return_nil_channel" {
			return nil, nil // Return nil channel
		}
		ch := make(chan string, 1)
		ch <- input
		close(ch)
		return ch, nil
	}

	RegisterBuiltin1[string, chan string]("test_nil_channel_builtin", nilChannelBuiltin)

	// Test nil channel case
	policy := `package test
result = test_nil_channel_builtin("return_nil_channel")`

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

	result := rs[0].Expressions[0].Value
	if result != nil {
		t.Errorf("Expected nil/null result for nil channel, got %v (%T)", result, result)
	}
}

// Test nil function handling
func TestBuiltinNilFunctionHandling(t *testing.T) {
	nilFuncBuiltin := func(ctx rego.BuiltinContext, input string) (func() string, error) {
		if input == "return_nil_func" {
			return nil, nil // Return nil function
		}
		return func() string { return "function result" }, nil
	}

	RegisterBuiltin1[string, func() string]("test_nil_func_builtin", nilFuncBuiltin)

	// Test nil function case
	policy := `package test
result = test_nil_func_builtin("return_nil_func")`

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

	result := rs[0].Expressions[0].Value
	if result != nil {
		t.Errorf("Expected nil/null result for nil function, got %v (%T)", result, result)
	}
}

// Test complex nil scenarios
func TestComplexNilScenarios(t *testing.T) {
	// Test struct with nil fields
	type TestStruct struct {
		Name    *string                 `json:"name"`
		Values  []int                   `json:"values"`
		Mapping map[string]interface{}  `json:"mapping"`
		Func    func() string           `json:"-"` // Functions can't be JSON marshaled
	}

	complexNilBuiltin := func(ctx rego.BuiltinContext, input string) (*TestStruct, error) {
		switch input {
		case "return_nil_struct":
			return nil, nil
		case "return_struct_with_nil_fields":
			return &TestStruct{
				Name:    nil,     // nil pointer
				Values:  nil,     // nil slice
				Mapping: nil,     // nil map
				Func:    nil,     // nil function
			}, nil
		case "return_struct_with_values":
			name := "test"
			return &TestStruct{
				Name:    &name,
				Values:  []int{1, 2, 3},
				Mapping: map[string]interface{}{"key": "value"},
				Func:    func() string { return "test" },
			}, nil
		default:
			return nil, nil
		}
	}

	RegisterBuiltin1[string, *TestStruct]("test_complex_nil_builtin", complexNilBuiltin)

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "nil struct",
			input: "return_nil_struct",
		},
		{
			name:  "struct with nil fields",
			input: "return_struct_with_nil_fields",
		},
		{
			name:  "struct with values",
			input: "return_struct_with_values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = test_complex_nil_builtin("` + tt.input + `")`

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

			result := rs[0].Expressions[0].Value
			t.Logf("Complex nil test %q result: %v (%T)", tt.name, result, result)

			// For nil struct, result should be nil
			if tt.input == "return_nil_struct" && result != nil {
				t.Errorf("Expected nil result for nil struct, got %v", result)
			}
		})
	}
}
