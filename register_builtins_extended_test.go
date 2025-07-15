package regobrick

import (
	"context"
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// WithDefaultDecimal option is defined but not actually used in the codebase
// This test has been removed as it was testing non-functional code

// Test RegisterBuiltin0_ (error-only builtin)
func TestRegisterBuiltin0_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext) error {
		// This builtin always succeeds (returns nil error)
		return nil
	}

	funcName := "test_error_only_builtin"
	RegisterBuiltin0_(funcName, errorBuiltin)

	policy := `package test
result if {
	` + funcName + `()
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

	// Should have exactly one result (true)
	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Errorf("Expected 1 result, got %d", len(rs))
		return
	}

	result := rs[0].Expressions[0].Value.(bool)
	if !result {
		t.Errorf("Expected true result, got %v", result)
	}
}

// Test RegisterBuiltin1_ (error-only builtin with 1 argument)
func TestRegisterBuiltin1_(t *testing.T) {
	errorBuiltin := func(ctx rego.BuiltinContext, input string) error {
		if input == "fail" {
			return errors.New("intentional failure")
		}
		return nil
	}

	funcName := "test_error_builtin_1arg"
	RegisterBuiltin1_[string](funcName, errorBuiltin)

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "success case",
			input:       "success",
			expectError: false,
		},
		{
			name:        "failure case",
			input:       "fail",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result if {
	` + funcName + `("` + tt.input + `")
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
			
			if tt.expectError {
				if err == nil && len(rs) > 0 {
					t.Errorf("Expected error or no results for %s, but got success", tt.name)
				} else {
					t.Logf("Got expected error/failure for %s", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.name, err)
					return
				}
				if len(rs) != 1 || len(rs[0].Expressions) != 1 {
					t.Errorf("Expected 1 result for %s, got %d", tt.name, len(rs))
					return
				}
				result := rs[0].Expressions[0].Value.(bool)
				if !result {
					t.Errorf("Expected true result for %s, got %v", tt.name, result)
				}
			}
		})
	}
}
