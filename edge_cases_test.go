package regobrick

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/shopspring/decimal"
)

// Test edge cases for ParseModule
func TestParseModuleAdvancedEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		source      string
		imports     []string
		expectError bool
		checkFunc   func(*testing.T, interface{})
	}{
		{
			name:     "module with function rules (should not get default)",
			filename: "functions.rego",
			source: `package functions

import data.regobrick.default_false

# This is a function rule - should NOT get default
allow(user) if {
	user == "admin"
}

# This is an if-rule - should get default
deny if {
	input.user == "guest"
}`,
			imports:     []string{},
			expectError: false,
		},
		{
			name:     "module with existing default rules",
			filename: "existing_defaults.rego",
			source: `package existing

import data.regobrick.default_false

# Existing default - should not be duplicated
default allow = false

allow if {
	input.user == "admin"
}

# This should get a new default
deny if {
	input.user == "guest"
}`,
			imports:     []string{},
			expectError: false,
		},
		{
			name:     "module with complex rule heads",
			filename: "complex.rego",
			source: `package complex

import data.regobrick.default_false

# Rule with key "if"
allow if {
	input.user == "admin"
}

# Boolean rule without explicit key
valid_user if {
	input.age >= 18
}

# Rule with partial set
permissions contains "read" if {
	input.role == "viewer"
}`,
			imports:     []string{},
			expectError: false,
		},
		{
			name:     "empty import path in imports list",
			filename: "empty_import.rego",
			source: `package test

rule1 if { true }`,
			imports:     []string{"", "data.valid.import", ""},
			expectError: false,
		},
		{
			name:     "import path with single segment",
			filename: "single_segment.rego",
			source: `package test

rule1 if { true }`,
			imports:     []string{"data"},
			expectError: false,
		},
		{
			name:     "import path with many segments",
			filename: "many_segments.rego",
			source: `package test

rule1 if { true }`,
			imports:     []string{"data.company.department.team.project.module"},
			expectError: false,
		},
		{
			name:        "completely malformed rego",
			filename:    "malformed.rego",
			source:      `this is not rego at all { } [ invalid syntax`,
			imports:     []string{},
			expectError: true,
		},
		{
			name:        "rego with syntax error",
			filename:    "syntax_error.rego",
			source:      `package test\n\nrule1 if {\n\tinput.value ==\n}`, // missing value after ==
			imports:     []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ParseModule(tt.filename, tt.source, tt.imports)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseModule() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseModule() unexpected error = %v", err)
				return
			}

			if module == nil {
				t.Errorf("ParseModule() returned nil module")
				return
			}

			// Verify the module structure
			if module.Package == nil {
				t.Errorf("ParseModule() module has no package")
				return
			}

			t.Logf("Module %s has %d rules, %d imports", tt.filename, len(module.Rules), len(module.Imports))

			// Check specific behaviors for default_false
			if strings.Contains(tt.source, "import data.regobrick.default_false") {
				// Count default rules
				defaultCount := 0
				ifRuleCount := 0
				functionRuleCount := 0

				for _, rule := range module.Rules {
					if rule.Default {
						defaultCount++
					}
					if len(rule.Head.Args) > 0 {
						functionRuleCount++
					} else {
						// Check if it's an if-rule or boolean rule
						isIfRule := false
						if rule.Head.Key != nil {
							if s, ok := rule.Head.Key.Value.(ast.String); ok && string(s) == "if" {
								isIfRule = true
							}
						} else {
							// Boolean rule
							isIfRule = true
						}
						if isIfRule {
							ifRuleCount++
						}
					}
				}

				t.Logf("Found %d default rules, %d if-rules, %d function rules", defaultCount, ifRuleCount, functionRuleCount)

				// Function rules should not get defaults
				if strings.Contains(tt.source, "allow(user)") {
					// This test has a function rule, make sure it didn't get a default
					hasAllowDefault := false
					for _, rule := range module.Rules {
						if rule.Default && rule.Head.Ref() != nil && strings.Contains(rule.Head.Ref().String(), "allow") {
							hasAllowDefault = true
							break
						}
					}
					if hasAllowDefault {
						t.Error("Function rule should not get a default rule")
					}
				}
			}
		})
	}
}

// Test RegoDecimal edge cases
func TestRegoDecimalEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		decimal  RegoDecimal
		expected string
	}{
		{
			name:     "very large number",
			decimal:  NewRegoDecimalFromInt(9223372036854775807), // max int64
			expected: "9223372036854775807",
		},
		{
			name:     "very small negative number",
			decimal:  NewRegoDecimalFromInt(-9223372036854775808), // min int64
			expected: "-9223372036854775808",
		},
		{
			name:     "zero with many decimal places",
			decimal:  func() RegoDecimal { d, _ := decimal.NewFromString("0.000000000000000000"); return NewRegoDecimal(d) }(),
			expected: "0",
		},
		{
			name:     "number with trailing zeros",
			decimal:  func() RegoDecimal { d, _ := decimal.NewFromString("123.4500000"); return NewRegoDecimal(d) }(),
			expected: "123.45", // Should remove trailing zeros
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.decimal)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			jsonStr := string(jsonBytes)
			if jsonStr != tt.expected {
				t.Errorf("JSON marshaling = %v, want %v", jsonStr, tt.expected)
			}

			// Verify it's a numeric literal, not a string
			if len(jsonStr) > 0 && (jsonStr[0] == '"' || jsonStr[len(jsonStr)-1] == '"') {
				t.Errorf("JSON output should be numeric literal, got string: %v", jsonStr)
			}
		})
	}
}

// Test builtin registration with extreme cases
func TestBuiltinRegistrationExtremeCases(t *testing.T) {
	// Test builtin with very long name
	longNameBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return "processed: " + input, nil
	}

	veryLongName := "test_builtin_with_extremely_long_name_that_goes_on_and_on_and_on"
	RegisterBuiltin1[string, string](veryLongName, longNameBuiltin)

	// Test builtin with special characters in categories
	specialCatBuiltin := func(ctx rego.BuiltinContext, input int) (int, error) {
		return input * 2, nil
	}

	RegisterBuiltin1[int, int](
		"test_special_categories",
		specialCatBuiltin,
		WithCategories("category-with-dashes", "category_with_underscores", "category.with.dots"),
	)

	// Test builtin with empty string category
	RegisterBuiltin1[string, string](
		"test_empty_category",
		func(ctx rego.BuiltinContext, input string) (string, error) { return input, nil },
		WithCategories(""),
	)

	// Test builtin with many categories
	manyCategories := make([]string, 100)
	for i := 0; i < 100; i++ {
		manyCategories[i] = fmt.Sprintf("category_%d", i)
	}

	RegisterBuiltin1[string, string](
		"test_many_categories",
		func(ctx rego.BuiltinContext, input string) (string, error) { return input, nil },
		WithCategories(manyCategories...),
	)

	// Verify the categories were stored correctly
	if cats, ok := customBuiltinCategories["test_many_categories"]; ok {
		if len(cats) != 100 {
			t.Errorf("Expected 100 categories, got %d", len(cats))
		}
	} else {
		t.Error("Categories not stored for test_many_categories")
	}

	// Test that the long name builtin works
	policy := `package test
result = ` + veryLongName + `("hello")`

	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare query with long name builtin: %v", err)
		return
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate query with long name builtin: %v", err)
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
}

// Test FilterCapabilities with extreme cases
func TestFilterCapabilitiesExtremeCases(t *testing.T) {
	tests := []struct {
		name         string
		allowedNames []string
		allowedCats  []string
		description  string
	}{
		{
			name:         "very long lists",
			allowedNames: make([]string, 1000), // 1000 empty strings
			allowedCats:  make([]string, 1000), // 1000 empty strings
			description:  "should handle very long lists",
		},
		{
			name:         "duplicate names",
			allowedNames: []string{"concat", "concat", "count", "count", "concat"},
			allowedCats:  []string{},
			description:  "should handle duplicate names",
		},
		{
			name:         "duplicate categories",
			allowedNames: []string{},
			allowedCats:  []string{"strings", "strings", "numbers", "strings"},
			description:  "should handle duplicate categories",
		},
		{
			name:         "names with special characters",
			allowedNames: []string{"test-builtin", "test_builtin", "test.builtin", "test builtin"},
			allowedCats:  []string{},
			description:  "should handle names with special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := FilterCapabilities(tt.allowedNames, tt.allowedCats)
			
			if caps == nil {
				t.Errorf("FilterCapabilities() returned nil")
				return
			}

			if caps.Builtins == nil {
				t.Errorf("FilterCapabilities() returned capabilities with nil Builtins")
				return
			}

			t.Logf("%s: Found %d builtins", tt.description, len(caps.Builtins))

			// Verify no nil builtins
			for i, builtin := range caps.Builtins {
				if builtin == nil {
					t.Errorf("Builtin at index %d is nil", i)
				}
			}
		})
	}
}

// Test error conditions in builtin execution
func TestBuiltinExecutionErrors(t *testing.T) {
	// Test builtin that returns error in certain conditions
	conditionalErrorBuiltin := func(ctx rego.BuiltinContext, input string) error {
		if input == "trigger_error" {
			return fmt.Errorf("intentional error: %s", input)
		}
		return nil // Success
	}

	RegisterBuiltin1_[string]("test_conditional_error_builtin", conditionalErrorBuiltin)

	// Test with input that triggers error - use in condition
	policy := `package test
result if {
	test_conditional_error_builtin("trigger_error")
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

	// This should not return a result (condition fails due to error)
	rs, err := query.Eval(ctx)
	if err != nil {
		t.Logf("Got error during evaluation (may be expected): %v", err)
	}

	// Should have no results or empty results
	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		if result, ok := rs[0].Expressions[0].Value.(bool); ok && result {
			t.Error("Expected condition to fail due to error, but it succeeded")
		}
	}

	// Test with input that doesn't trigger error
	policy2 := `package test
result if {
	test_conditional_error_builtin("ok")
}`

	query2, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", policy2),
	).PrepareForEval(ctx)

	if err != nil {
		t.Errorf("Failed to prepare second query: %v", err)
		return
	}

	rs2, err := query2.Eval(ctx)
	if err != nil {
		t.Errorf("Unexpected error in second evaluation: %v", err)
		return
	}

	// This should succeed
	if len(rs2) != 1 || len(rs2[0].Expressions) != 1 {
		t.Errorf("Expected 1 result for successful case, got %d", len(rs2))
		return
	}

	result, ok := rs2[0].Expressions[0].Value.(bool)
	if !ok {
		t.Errorf("Expected bool result, got %T", rs2[0].Expressions[0].Value)
		return
	}

	if !result {
		t.Error("Expected successful condition to return true")
	}
}

// Test complex integration scenarios
func TestComplexIntegrationScenarios(t *testing.T) {
	// Register multiple builtins with overlapping categories
	RegisterBuiltin1[string, string]("test_string_proc1", 
		func(ctx rego.BuiltinContext, s string) (string, error) { return "proc1:" + s, nil },
		WithCategories("string_processing", "custom"))
	
	RegisterBuiltin1[string, string]("test_string_proc2", 
		func(ctx rego.BuiltinContext, s string) (string, error) { return "proc2:" + s, nil },
		WithCategories("string_processing", "advanced"))
	
	RegisterBuiltin1[int, int]("test_math_proc", 
		func(ctx rego.BuiltinContext, n int) (int, error) { return n * 3, nil },
		WithCategories("math", "custom"))

	// Test policy that uses multiple custom builtins
	policy := `package complex

import data.regobrick.default_false

result1 = test_string_proc1("hello")
result2 = test_string_proc2("world") 
result3 = test_math_proc(42)

combined if {
	result1 == "proc1:hello"
	result2 == "proc2:world"
	result3 == 126
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("complex.rego", policy, []string{"data.utils.helpers"}),
		rego.Query("data.complex"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Failed to prepare complex query: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Failed to evaluate complex query: %v", err)
	}

	if len(rs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rs))
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	
	// Check all results
	expectedResults := map[string]interface{}{
		"result1":  "proc1:hello",
		"result2":  "proc2:world", 
		"result3":  json.Number("126"), // OPA returns numbers as json.Number
		"combined": true,
	}

	for key, expected := range expectedResults {
		if actual, ok := result[key]; !ok {
			t.Errorf("Expected key %q not found in result", key)
		} else if actual != expected {
			t.Errorf("For key %q: expected %v (%T), got %v (%T)", key, expected, expected, actual, actual)
		}
	}

	// Test capability filtering with these custom builtins
	caps := FilterCapabilities([]string{}, []string{"string_processing"})
	
	foundProc1 := false
	foundProc2 := false
	foundMathProc := false
	
	for _, builtin := range caps.Builtins {
		switch builtin.Name {
		case "test_string_proc1":
			foundProc1 = true
		case "test_string_proc2":
			foundProc2 = true
		case "test_math_proc":
			foundMathProc = true
		}
	}
	
	if !foundProc1 || !foundProc2 {
		t.Error("String processing builtins should be included in filtered capabilities")
	}
	
	if foundMathProc {
		t.Error("Math builtin should not be included when filtering by string_processing category")
	}
}
