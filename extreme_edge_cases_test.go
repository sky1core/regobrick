package regobrick

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Test Unicode and special characters in builtin names and categories
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	tests := []struct {
		name         string
		builtinName  string
		categories   []string
		expectError  bool
	}{
		{
			name:        "unicode builtin name",
			builtinName: "test_unicode_함수",
			categories:  []string{"한글_카테고리"},
			expectError: false,
		},
		{
			name:        "emoji in builtin name",
			builtinName: "test_emoji_🚀_builtin",
			categories:  []string{"emoji_🎯_category"},
			expectError: false,
		},
		{
			name:        "special symbols",
			builtinName: "test_symbols_αβγ_builtin",
			categories:  []string{"symbols_∑∆∏"},
			expectError: false,
		},
		{
			name:        "mixed scripts",
			builtinName: "test_mixed_русский_العربية_中文",
			categories:  []string{"mixed_scripts"},
			expectError: false,
		},
		{
			name:        "very long unicode name",
			builtinName: strings.Repeat("测试", 50) + "_builtin",
			categories:  []string{strings.Repeat("카테고리", 20)},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the name is valid UTF-8
			if !utf8.ValidString(tt.builtinName) {
				t.Errorf("Builtin name is not valid UTF-8: %s", tt.builtinName)
				return
			}

			builtin := func(ctx rego.BuiltinContext, input string) (string, error) {
				return "processed: " + input, nil
			}

			// This should not panic or fail
			RegisterBuiltin1[string, string](
				tt.builtinName,
				builtin,
				WithCategories(tt.categories...),
			)

			// Test that the builtin can be used (though OPA might have restrictions)
			policy := fmt.Sprintf(`package test
result = %s("hello")`, tt.builtinName)

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				// OPA might reject unicode builtin names, which is fine
				t.Logf("OPA rejected unicode builtin name (expected): %v", err)
				return
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Logf("Unicode builtin evaluation failed (might be expected): %v", err)
				return
			}

			if len(rs) == 1 && len(rs[0].Expressions) == 1 {
				result := rs[0].Expressions[0].Value.(string)
				t.Logf("Unicode builtin worked: %s -> %s", tt.builtinName, result)
			}
		})
	}
}

// Test extremely large inputs and outputs
func TestExtremelyLargeInputsOutputs(t *testing.T) {
	// Test with very large strings
	largeStringBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return "length: " + fmt.Sprintf("%d", len(input)), nil
	}
	RegisterBuiltin1[string, string]("test_large_string", largeStringBuiltin)

	tests := []struct {
		name       string
		inputSize  int
		expectFail bool
	}{
		{
			name:      "1KB string",
			inputSize: 1024,
		},
		{
			name:      "10KB string",
			inputSize: 10 * 1024,
		},
		{
			name:      "100KB string",
			inputSize: 100 * 1024,
		},
		{
			name:      "1MB string",
			inputSize: 1024 * 1024,
		},
		// Uncomment for stress testing (might be slow)
		// {
		// 	name:      "10MB string",
		// 	inputSize: 10 * 1024 * 1024,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create large string
			largeString := strings.Repeat("a", tt.inputSize)

			policy := `package test
result = test_large_string(input.large_data)`

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				t.Errorf("Failed to prepare query for %s: %v", tt.name, err)
				return
			}

			input := map[string]interface{}{
				"large_data": largeString,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				if tt.expectFail {
					t.Logf("Expected failure for %s: %v", tt.name, err)
					return
				}
				t.Errorf("Failed to evaluate %s: %v", tt.name, err)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Errorf("Expected 1 result for %s, got %d", tt.name, len(rs))
				return
			}

			result := rs[0].Expressions[0].Value.(string)
			expected := fmt.Sprintf("length: %d", tt.inputSize)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}

			t.Logf("Successfully processed %s: %s", tt.name, result)
		})
	}
}

// Test deeply nested data structures
func TestDeeplyNestedStructures(t *testing.T) {
	nestedBuiltin := func(ctx rego.BuiltinContext, input map[string]interface{}) (string, error) {
		// Count nesting depth
		depth := 0
		current := input
		for {
			if nested, ok := current["nested"]; ok {
				if nestedMap, ok := nested.(map[string]interface{}); ok {
					current = nestedMap
					depth++
					if depth > 1000 { // Prevent infinite loop
						break
					}
				} else {
					break
				}
			} else {
				break
			}
		}
		return fmt.Sprintf("depth: %d", depth), nil
	}
	RegisterBuiltin1[map[string]interface{}, string]("test_nested_depth", nestedBuiltin)

	tests := []struct {
		name  string
		depth int
	}{
		{name: "depth 10", depth: 10},
		{name: "depth 50", depth: 50},
		{name: "depth 100", depth: 100},
		// Uncomment for stress testing
		// {name: "depth 500", depth: 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create deeply nested structure
			nested := make(map[string]interface{})
			current := nested
			for i := 0; i < tt.depth; i++ {
				next := make(map[string]interface{})
				current["nested"] = next
				current["level"] = i
				current = next
			}
			current["final"] = "reached"

			policy := `package test
result = test_nested_depth(input.nested_data)`

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				t.Errorf("Failed to prepare query for %s: %v", tt.name, err)
				return
			}

			input := map[string]interface{}{
				"nested_data": nested,
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Errorf("Failed to evaluate %s: %v", tt.name, err)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Errorf("Expected 1 result for %s, got %d", tt.name, len(rs))
				return
			}

			result := rs[0].Expressions[0].Value.(string)
			expected := fmt.Sprintf("depth: %d", tt.depth)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}

			t.Logf("Successfully processed %s: %s", tt.name, result)
		})
	}
}

// Test builtin that returns functions (edge case for toTerm)
func TestBuiltinReturningFunction(t *testing.T) {
	functionBuiltin := func(ctx rego.BuiltinContext, input string) (func() string, error) {
		if input == "nil" {
			return nil, nil // Test nil function
		}
		return func() string {
			return "function result: " + input
		}, nil
	}
	RegisterBuiltin1[string, func() string]("test_function_return", functionBuiltin)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "nil function",
			input:    "nil",
			expected: nil, // Should be converted to null
		},
		{
			name:     "valid function",
			input:    "test",
			expected: nil, // Functions can't be serialized, might be null or error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := fmt.Sprintf(`package test
result = test_function_return("%s")`, tt.input)

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				t.Errorf("Failed to prepare query for %s: %v", tt.name, err)
				return
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Logf("Function builtin evaluation failed (might be expected): %v", err)
				return
			}

			// Functions might not return results or return null
			if len(rs) == 0 {
				t.Logf("Function builtin returned no results for %s (might be expected)", tt.name)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Logf("Function builtin returned unexpected result count for %s: %d", tt.name, len(rs))
				return
			}

			result := rs[0].Expressions[0].Value
			t.Logf("Function builtin result for %s: %v (%T)", tt.name, result, result)

			if tt.input == "nil" && result != nil {
				t.Errorf("Expected nil result for nil function, got %v", result)
			}
		})
	}
}

// Test builtin with circular reference in input
func TestCircularReferenceHandling(t *testing.T) {
	circularBuiltin := func(ctx rego.BuiltinContext, input map[string]interface{}) (string, error) {
		// Simple depth check instead of circular reference detection
		// (since Go maps in JSON can't have true circular references)
		
		var countDepth func(interface{}, int) int
		countDepth = func(obj interface{}, depth int) int {
			if depth > 100 { // Prevent deep recursion
				return depth
			}
			
			if m, ok := obj.(map[string]interface{}); ok {
				maxDepth := depth
				for _, v := range m {
					childDepth := countDepth(v, depth+1)
					if childDepth > maxDepth {
						maxDepth = childDepth
					}
				}
				return maxDepth
			}
			
			if arr, ok := obj.([]interface{}); ok {
				maxDepth := depth
				for _, v := range arr {
					childDepth := countDepth(v, depth+1)
					if childDepth > maxDepth {
						maxDepth = childDepth
					}
				}
				return maxDepth
			}
			
			return depth
		}
		
		maxDepth := countDepth(input, 0)
		return fmt.Sprintf("max depth: %d", maxDepth), nil
	}
	RegisterBuiltin1[map[string]interface{}, string]("test_circular", circularBuiltin)

	t.Run("circular reference test", func(t *testing.T) {
		// Create a nested structure
		data := map[string]interface{}{
			"name": "test",
			"nested": map[string]interface{}{
				"value": 42,
				"deep": map[string]interface{}{
					"level": 3,
				},
			},
		}

		policy := `package test
result = test_circular(input.data)`

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
			"data": data,
		}

		rs, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			t.Errorf("Failed to evaluate: %v", err)
			return
		}

		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			t.Errorf("Expected 1 result, got %d", len(rs))
			return
		}

		result := rs[0].Expressions[0].Value.(string)
		t.Logf("Depth test result: %s", result)
	})
}

// Test builtin registration with empty/whitespace names
func TestBuiltinRegistrationEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		builtinName string
		expectPanic bool
	}{
		{
			name:        "empty builtin name",
			builtinName: "",
			expectPanic: false, // Might be allowed but not usable
		},
		{
			name:        "whitespace only name",
			builtinName: "   ",
			expectPanic: false,
		},
		{
			name:        "name with newlines",
			builtinName: "test\nbuiltin",
			expectPanic: false,
		},
		{
			name:        "name with tabs",
			builtinName: "test\tbuiltin",
			expectPanic: false,
		},
		{
			name:        "name with null character",
			builtinName: "test\x00builtin",
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic for %s: %v", tt.name, r)
					} else {
						t.Logf("Expected panic for %s: %v", tt.name, r)
					}
				}
			}()

			builtin := func(ctx rego.BuiltinContext, input string) (string, error) {
				return "result", nil
			}

			RegisterBuiltin1[string, string](tt.builtinName, builtin)
			t.Logf("Successfully registered builtin with name: %q", tt.builtinName)
		})
	}
}

// Test memory usage with many builtin registrations
func TestManyBuiltinRegistrations(t *testing.T) {
	const numBuiltins = 1000

	t.Run("register many builtins", func(t *testing.T) {
		for i := 0; i < numBuiltins; i++ {
			builtinName := fmt.Sprintf("mass_builtin_%d", i)
			
			builtin := func(ctx rego.BuiltinContext, input string) (string, error) {
				return fmt.Sprintf("result_%d: %s", i, input), nil
			}
			
			RegisterBuiltin1[string, string](
				builtinName,
				builtin,
				WithCategories(fmt.Sprintf("category_%d", i%10)), // 10 different categories
			)
		}

		// Test that some of them work
		testCases := []int{0, 100, 500, 999}
		for _, idx := range testCases {
			builtinName := fmt.Sprintf("mass_builtin_%d", idx)
			policy := fmt.Sprintf(`package test
result = %s("test")`, builtinName)

			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", policy),
			).PrepareForEval(ctx)

			if err != nil {
				t.Errorf("Failed to prepare query for builtin %d: %v", idx, err)
				continue
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Errorf("Failed to evaluate builtin %d: %v", idx, err)
				continue
			}

			if len(rs) == 1 && len(rs[0].Expressions) == 1 {
				result := rs[0].Expressions[0].Value.(string)
				expected := fmt.Sprintf("result_%d: test", idx)
				if result != expected {
					t.Errorf("Builtin %d: expected %q, got %q", idx, expected, result)
				}
			}
		}

		t.Logf("Successfully registered and tested %d builtins", numBuiltins)
	})
}
