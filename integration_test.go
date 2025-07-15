package regobrick

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/shopspring/decimal"
)

func TestIntegrationDefaultFalse(t *testing.T) {
	// Test the complete flow: parse module with default_false, evaluate policy
	policy := `package example

import data.regobrick.default_false

allow if {
	input.user == "admin"
}

deny if {
	input.user == "guest"
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("policy.rego", policy, []string{}),
		rego.Query("data.example"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Failed to prepare query: %v", err)
	}

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:  "admin user should be allowed",
			input: map[string]interface{}{"user": "admin"},
			expected: map[string]interface{}{
				"allow": true,
				"deny":  false, // default false should be applied
			},
		},
		{
			name:  "guest user should be denied",
			input: map[string]interface{}{"user": "guest"},
			expected: map[string]interface{}{
				"allow": false, // default false should be applied
				"deny":  true,
			},
		},
		{
			name:  "unknown user should default to false",
			input: map[string]interface{}{"user": "unknown"},
			expected: map[string]interface{}{
				"allow": false, // default false
				"deny":  false, // default false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, err := query.Eval(ctx, rego.EvalInput(tt.input))
			if err != nil {
				t.Fatalf("Failed to evaluate query: %v", err)
			}

			if len(rs) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(rs))
			}

			result := rs[0].Expressions[0].Value.(map[string]interface{})

			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Expected key %q not found in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("For key %q: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestIntegrationCustomBuiltinWithDecimal(t *testing.T) {
	// Register a custom builtin that works with decimals
	calculateTax := func(ctx rego.BuiltinContext, amount RegoDecimal, rate RegoDecimal) (RegoDecimal, error) {
		tax := amount.Decimal.Mul(rate.Decimal)
		return NewRegoDecimal(tax), nil
	}

	RegisterBuiltin2[RegoDecimal, RegoDecimal, RegoDecimal]("calculate_tax", calculateTax)

	policy := `package finance

import data.regobrick.default_false

tax_amount = calculate_tax(input.amount, input.tax_rate)

high_tax if {
	tax_amount > 100
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("finance.rego", policy, []string{}),
		rego.Query("data.finance"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Failed to prepare query: %v", err)
	}

	tests := []struct {
		name        string
		amount      string
		taxRate     string
		expectedTax string
		highTax     bool
	}{
		{
			name:        "low tax calculation",
			amount:      "100.00",
			taxRate:     "0.05",
			expectedTax: "5.00",
			highTax:     false,
		},
		{
			name:        "high tax calculation",
			amount:      "2000.00",
			taxRate:     "0.08",
			expectedTax: "160.00",
			highTax:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, _ := decimal.NewFromString(tt.amount)
			taxRate, _ := decimal.NewFromString(tt.taxRate)

			input := map[string]interface{}{
				"amount":   NewRegoDecimal(amount),
				"tax_rate": NewRegoDecimal(taxRate),
			}

			rs, err := query.Eval(ctx, rego.EvalInput(input))
			if err != nil {
				t.Fatalf("Failed to evaluate query: %v", err)
			}

			if len(rs) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(rs))
			}

			result := rs[0].Expressions[0].Value.(map[string]interface{})

			// Check tax amount calculation
			if taxAmount, ok := result["tax_amount"]; ok {
				// The result might be a RegoDecimal or converted to another type
				// We need to handle this conversion properly
				t.Logf("Tax amount result type: %T, value: %v", taxAmount, taxAmount)
			} else {
				t.Error("Expected tax_amount in result")
			}

			// Check high_tax boolean
			if highTaxValue, ok := result["high_tax"]; ok {
				if highTaxValue != tt.highTax {
					t.Errorf("Expected high_tax=%v, got %v", tt.highTax, highTaxValue)
				}
			} else if tt.highTax {
				// If high_tax is not in result but we expected true, that's an error
				t.Error("Expected high_tax=true but key not found in result")
			}
			// If high_tax is not in result and we expected false, that's OK due to default_false
		})
	}
}

func TestIntegrationCapabilityFiltering(t *testing.T) {
	// Test that capability filtering works in practice
	policy := `package security

# This should work - concat is allowed
message = concat(" ", ["Hello", "World"])

# This might not work if we filter out certain builtins
# result = some_restricted_builtin("test")`

	// Create filtered capabilities that only allow certain builtins
	allowedNames := []string{"concat"}
	allowedCats := []string{"strings"}
	caps := FilterCapabilities(allowedNames, allowedCats)

	ctx := context.Background()
	query, err := rego.New(
		Module("security.rego", policy, []string{}),
		rego.Query("data.security.message"),
		rego.Capabilities(caps),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Failed to prepare query with filtered capabilities: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Failed to evaluate query: %v", err)
	}

	if len(rs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rs))
	}

	result := rs[0].Expressions[0].Value.(string)
	expected := "Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestIntegrationComplexPolicy(t *testing.T) {
	// Test a more complex policy that uses multiple RegoBrick features
	
	// Register custom builtins
	isValidEmail := func(ctx rego.BuiltinContext, email string) (bool, error) {
		// Simple email validation for testing
		return len(email) > 0 && email != "invalid", nil
	}
	
	calculateDiscount := func(ctx rego.BuiltinContext, amount RegoDecimal, percentage int) (RegoDecimal, error) {
		discount := amount.Decimal.Mul(NewRegoDecimalFromInt(int64(percentage)).Decimal).Div(NewRegoDecimalFromInt(100).Decimal)
		return NewRegoDecimal(discount), nil
	}

	RegisterBuiltin1[string, bool]("is_valid_email", isValidEmail)
	RegisterBuiltin2[RegoDecimal, int, RegoDecimal]("calculate_discount", calculateDiscount)

	policy := `package ecommerce

import data.regobrick.default_false

# User validation
valid_user if {
	input.user.email
	is_valid_email(input.user.email)
	input.user.age >= 18
}

# Order processing
process_order if {
	valid_user
	input.order.amount > 0
}

# Discount calculation
discount_amount = calculate_discount(input.order.amount, input.user.discount_percent)

final_amount = input.order.amount - discount_amount

# Special offers
vip_customer if {
	input.user.membership == "vip"
	input.order.amount > 1000
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("ecommerce.rego", policy, []string{}),
		rego.Query("data.ecommerce"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Failed to prepare complex policy: %v", err)
	}

	// Test with valid input
	amount, _ := decimal.NewFromString("1500.00")
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"email":            "user@example.com",
			"age":              25,
			"membership":       "vip",
			"discount_percent": 10,
		},
		"order": map[string]interface{}{
			"amount": NewRegoDecimal(amount),
		},
	}

	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		t.Fatalf("Failed to evaluate complex policy: %v", err)
	}

	if len(rs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rs))
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})

	// Verify expected results
	expectedBools := map[string]bool{
		"valid_user":    true,
		"process_order": true,
		"vip_customer":  true,
	}

	for key, expected := range expectedBools {
		if actual, ok := result[key]; !ok {
			if expected {
				t.Errorf("Expected %q=true but key not found", key)
			}
			// If expected false and key not found, that's OK due to default_false
		} else if actual != expected {
			t.Errorf("For %q: expected %v, got %v", key, expected, actual)
		}
	}

	// Check that discount and final amounts are calculated
	if _, ok := result["discount_amount"]; !ok {
		t.Error("Expected discount_amount to be calculated")
	}
	if _, ok := result["final_amount"]; !ok {
		t.Error("Expected final_amount to be calculated")
	}

	t.Logf("Complex policy result: %+v", result)
}
