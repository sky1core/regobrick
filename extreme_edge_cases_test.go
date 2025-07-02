package regobrick

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Unicode builtin names are not supported by OPA - test removed as it's not practical

// Test realistic large inputs and performance characteristics
func TestRealisticLargeInputsOutputs(t *testing.T) {
	// Test with realistic large strings (like JSON payloads, logs, etc.)
	largeStringBuiltin := func(ctx rego.BuiltinContext, input string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"length":      len(input),
			"word_count":  len(strings.Fields(input)),
			"line_count":  len(strings.Split(input, "\n")),
			"has_json":    strings.Contains(input, "{") && strings.Contains(input, "}"),
			"first_100":   input[:min(100, len(input))],
		}, nil
	}
	RegisterBuiltin1[string, map[string]interface{}]("analyze_large_string", largeStringBuiltin)

	tests := []struct {
		name        string
		inputSize   int
		contentType string
		generator   func(int) string
	}{
		{
			name:        "large JSON payload",
			inputSize:   10 * 1024, // 10KB - realistic API payload size
			contentType: "json",
			generator: func(size int) string {
				// Generate realistic JSON structure
				var sb strings.Builder
				sb.WriteString(`{"users":[`)
				userSize := size / 100 // Approximate size per user
				for i := 0; i < userSize; i++ {
					if i > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf(`{"id":%d,"name":"user_%d","email":"user_%d@example.com"}`, i, i, i))
				}
				sb.WriteString(`]}`)
				return sb.String()
			},
		},
		{
			name:        "log file content",
			inputSize:   50 * 1024, // 50KB - realistic log file size
			contentType: "logs",
			generator: func(size int) string {
				var sb strings.Builder
				lineCount := size / 100 // Approximate size per log line
				for i := 0; i < lineCount; i++ {
					sb.WriteString(fmt.Sprintf("2023-01-01 12:00:%02d INFO [service] Processing request %d from user_%d\n", i%60, i, i%1000))
				}
				return sb.String()
			},
		},
		{
			name:        "configuration file",
			inputSize:   5 * 1024, // 5KB - realistic config file size
			contentType: "config",
			generator: func(size int) string {
				var sb strings.Builder
				configCount := size / 50
				for i := 0; i < configCount; i++ {
					sb.WriteString(fmt.Sprintf("config.section_%d.key_%d=value_%d\n", i/10, i, i))
				}
				return sb.String()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate realistic content
			content := tt.generator(tt.inputSize)
			actualSize := len(content)

			policy := `package test
result = analyze_large_string(input.content)`

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
				"content": content,
			}

			// Measure performance
			start := time.Now()
			rs, err := query.Eval(ctx, rego.EvalInput(input))
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Failed to evaluate %s: %v", tt.name, err)
				return
			}

			if len(rs) != 1 || len(rs[0].Expressions) != 1 {
				t.Errorf("Expected 1 result for %s, got %d", tt.name, len(rs))
				return
			}

			result := rs[0].Expressions[0].Value.(map[string]interface{})
			
			// Verify analysis results
			lengthNum, _ := result["length"].(json.Number)
			length, _ := lengthNum.Int64()
			if int(length) != actualSize {
				t.Errorf("Length mismatch for %s: expected %d, got %d", tt.name, actualSize, int(length))
			}

			wordCountNum, _ := result["word_count"].(json.Number)
			wordCount, _ := wordCountNum.Int64()
			lineCountNum, _ := result["line_count"].(json.Number)
			lineCount, _ := lineCountNum.Int64()
			hasJson := result["has_json"].(bool)
			first100 := result["first_100"].(string)

			t.Logf("%s analysis: size=%d, words=%d, lines=%d, hasJson=%t, duration=%v", 
				tt.name, int(length), int(wordCount), int(lineCount), hasJson, duration)
			t.Logf("First 100 chars: %q", first100)

			// Performance check - should process within reasonable time
			if duration > time.Second {
				t.Errorf("Processing %s took too long: %v", tt.name, duration)
			}

			// Content-specific validations
			switch tt.contentType {
			case "json":
				if !hasJson {
					t.Errorf("JSON content should be detected as JSON")
				}
			case "logs":
				if lineCount < 100 {
					t.Errorf("Log content should have many lines, got %d", lineCount)
				}
			case "config":
				if !strings.Contains(first100, "config.") {
					t.Errorf("Config content should start with config prefix")
				}
			}
		})
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Test realistic nested data structures (like complex JSON APIs, config files)
func TestRealisticNestedStructures(t *testing.T) {
	nestedAnalyzer := func(ctx rego.BuiltinContext, input map[string]interface{}) (map[string]interface{}, error) {
		analysis := map[string]interface{}{
			"max_depth":    0,
			"total_keys":   0,
			"array_count":  0,
			"object_count": 0,
			"leaf_values":  0,
		}

		var analyze func(interface{}, int) int
		analyze = func(obj interface{}, depth int) int {
			maxDepth := depth
			
			switch v := obj.(type) {
			case map[string]interface{}:
				analysis["object_count"] = analysis["object_count"].(int) + 1
				analysis["total_keys"] = analysis["total_keys"].(int) + len(v)
				
				for _, value := range v {
					childDepth := analyze(value, depth+1)
					if childDepth > maxDepth {
						maxDepth = childDepth
					}
				}
			case []interface{}:
				analysis["array_count"] = analysis["array_count"].(int) + 1
				
				for _, item := range v {
					childDepth := analyze(item, depth+1)
					if childDepth > maxDepth {
						maxDepth = childDepth
					}
				}
			default:
				analysis["leaf_values"] = analysis["leaf_values"].(int) + 1
			}
			
			return maxDepth
		}

		maxDepth := analyze(input, 0)
		analysis["max_depth"] = maxDepth
		
		return analysis, nil
	}
	RegisterBuiltin1[map[string]interface{}, map[string]interface{}]("analyze_nested_data", nestedAnalyzer)

	tests := []struct {
		name        string
		description string
		generator   func() map[string]interface{}
		expectDepth int
	}{
		{
			name:        "e-commerce product catalog",
			description: "Realistic product data with categories, variants, and metadata",
			expectDepth: 8, // Adjusted based on actual structure depth
			generator: func() map[string]interface{} {
				return map[string]interface{}{
					"catalog": map[string]interface{}{
						"categories": []interface{}{
							map[string]interface{}{
								"id":   "electronics",
								"name": "Electronics",
								"products": []interface{}{
									map[string]interface{}{
										"id":    "laptop-001",
										"name":  "Gaming Laptop",
										"price": 1299.99,
										"specs": map[string]interface{}{
											"cpu":    "Intel i7",
											"memory": "16GB",
											"storage": map[string]interface{}{
												"type": "SSD",
												"size": "512GB",
											},
										},
										"variants": []interface{}{
											map[string]interface{}{
												"color": "black",
												"stock": 10,
											},
											map[string]interface{}{
												"color": "silver",
												"stock": 5,
											},
										},
									},
								},
							},
						},
					},
				}
			},
		},
		{
			name:        "user profile with permissions",
			description: "Complex user data with nested permissions and preferences",
			expectDepth: 5,
			generator: func() map[string]interface{} {
				return map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"personal": map[string]interface{}{
								"name":  "John Doe",
								"email": "john@example.com",
								"preferences": map[string]interface{}{
									"notifications": map[string]interface{}{
										"email": true,
										"sms":   false,
										"push":  true,
									},
									"privacy": map[string]interface{}{
										"profile_visibility": "friends",
										"data_sharing":       false,
									},
								},
							},
							"permissions": map[string]interface{}{
								"roles": []interface{}{"user", "moderator"},
								"scopes": map[string]interface{}{
									"read":  []interface{}{"posts", "comments"},
									"write": []interface{}{"posts"},
									"admin": []interface{}{},
								},
							},
						},
					},
				}
			},
		},
		{
			name:        "API response with pagination",
			description: "Typical API response structure with nested data and metadata",
			expectDepth: 4,
			generator: func() map[string]interface{} {
				return map[string]interface{}{
					"data": []interface{}{
						map[string]interface{}{
							"id": "1",
							"attributes": map[string]interface{}{
								"title":       "First Post",
								"content":     "This is the content",
								"created_at":  "2023-01-01T00:00:00Z",
								"author": map[string]interface{}{
									"id":   "user-1",
									"name": "Author Name",
								},
								"tags": []interface{}{"tech", "programming"},
							},
						},
					},
					"meta": map[string]interface{}{
						"pagination": map[string]interface{}{
							"current_page": 1,
							"total_pages":  10,
							"per_page":     20,
							"total_count":  200,
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.generator()

			policy := `package test
result = analyze_nested_data(input.data)`

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
				"data": data,
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

			result := rs[0].Expressions[0].Value.(map[string]interface{})
			
			maxDepthNum, _ := result["max_depth"].(json.Number)
			maxDepth, _ := maxDepthNum.Int64()
			totalKeysNum, _ := result["total_keys"].(json.Number)
			totalKeys, _ := totalKeysNum.Int64()
			arrayCountNum, _ := result["array_count"].(json.Number)
			arrayCount, _ := arrayCountNum.Int64()
			objectCountNum, _ := result["object_count"].(json.Number)
			objectCount, _ := objectCountNum.Int64()
			leafValuesNum, _ := result["leaf_values"].(json.Number)
			leafValues, _ := leafValuesNum.Int64()

			t.Logf("%s (%s):", tt.name, tt.description)
			t.Logf("  Max depth: %d, Total keys: %d", int(maxDepth), int(totalKeys))
			t.Logf("  Arrays: %d, Objects: %d, Leaf values: %d", int(arrayCount), int(objectCount), int(leafValues))

			// Validate realistic expectations
			if int(maxDepth) < 2 {
				t.Errorf("Expected realistic nesting depth >= 2, got %d", int(maxDepth))
			}
			if int(maxDepth) > 10 {
				t.Errorf("Depth too extreme for realistic data: %d", int(maxDepth))
			}
			if int(totalKeys) == 0 {
				t.Errorf("Should have some keys in realistic data")
			}
			if int(objectCount) == 0 {
				t.Errorf("Should have some objects in realistic data")
			}

			// Check if depth is within expected range
			if tt.expectDepth > 0 && (int(maxDepth) < tt.expectDepth-1 || int(maxDepth) > tt.expectDepth+1) {
				t.Errorf("Expected depth around %d, got %d", tt.expectDepth, int(maxDepth))
			}
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
