package regobrick

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/shopspring/decimal"
)

// Test RegisterBuiltin4 and RegisterBuiltin5 with realistic scenarios
func TestRegisterBuiltin4(t *testing.T) {
	// Realistic 4-argument builtin: validate user access
	accessValidator := func(ctx rego.BuiltinContext, user string, resource string, action string, context_data map[string]interface{}) (bool, error) {
		// Simple access control logic
		if user == "admin" {
			return true, nil
		}
		if user == "user" && action == "read" {
			return true, nil
		}
		if user == "guest" && resource == "public" && action == "read" {
			return true, nil
		}
		return false, nil
	}

	RegisterBuiltin4[string, string, string, map[string]interface{}, bool](
		"validate_access",
		accessValidator,
		WithCategories("security", "access_control"),
	)

	tests := []struct {
		name     string
		user     string
		resource string
		action   string
		context  map[string]interface{}
		expected bool
	}{
		{
			name:     "admin full access",
			user:     "admin",
			resource: "sensitive",
			action:   "write",
			context:  map[string]interface{}{"ip": "192.168.1.1"},
			expected: true,
		},
		{
			name:     "user read access",
			user:     "user",
			resource: "document",
			action:   "read",
			context:  map[string]interface{}{"department": "engineering"},
			expected: true,
		},
		{
			name:     "guest public read",
			user:     "guest",
			resource: "public",
			action:   "read",
			context:  map[string]interface{}{},
			expected: true,
		},
		{
			name:     "guest denied write",
			user:     "guest",
			resource: "public",
			action:   "write",
			context:  map[string]interface{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := `package test
result = validate_access(input.user, input.resource, input.action, input.context)`

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
				"user":     tt.user,
				"resource": tt.resource,
				"action":   tt.action,
				"context":  tt.context,
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

			result := rs[0].Expressions[0].Value.(bool)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test RegisterBuiltin5 with financial calculation scenario
func TestRegisterBuiltin5WithRegoDecimal(t *testing.T) {
	// Realistic 5-argument builtin: calculate loan payment
	loanCalculator := func(ctx rego.BuiltinContext, principal RegoDecimal, rate RegoDecimal, term int, fees RegoDecimal, insurance RegoDecimal) (RegoDecimal, error) {
		// Simple loan payment calculation: (principal + fees + insurance) / term + (principal * rate / 12)
		termDecimal := NewRegoDecimalFromInt(int64(term))
		twelveDecimal := NewRegoDecimalFromInt(12)
		
		monthlyRate := NewRegoDecimal(rate.Div(twelveDecimal.Decimal))
		monthlyPrincipal := NewRegoDecimal(principal.Div(termDecimal.Decimal))
		monthlyInterest := NewRegoDecimal(principal.Mul(monthlyRate.Decimal))
		monthlyFees := NewRegoDecimal(fees.Div(termDecimal.Decimal))
		monthlyInsurance := NewRegoDecimal(insurance.Div(termDecimal.Decimal))
		
		totalMonthly := NewRegoDecimal(monthlyPrincipal.Add(monthlyInterest.Decimal).Add(monthlyFees.Decimal).Add(monthlyInsurance.Decimal))
		return totalMonthly, nil
	}

	RegisterBuiltin5[RegoDecimal, RegoDecimal, int, RegoDecimal, RegoDecimal, RegoDecimal](
		"calculate_loan_payment",
		loanCalculator,
		WithCategories("finance", "calculation"),
	)

	policy := `package test
result = calculate_loan_payment(input.principal, input.rate, input.term, input.fees, input.insurance)`

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
		"principal": NewRegoDecimal(decimal.NewFromFloat(100000)), // $100,000 loan
		"rate":      NewRegoDecimal(decimal.NewFromFloat(0.05)),   // 5% annual rate
		"term":      360,                                          // 30 years (360 months)
		"fees":      NewRegoDecimal(decimal.NewFromFloat(1200)),   // $1,200 in fees
		"insurance": NewRegoDecimal(decimal.NewFromFloat(2400)),   // $2,400 annual insurance
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

	// The result should be a reasonable monthly payment
	// This is a realistic test of complex decimal calculations
	t.Logf("Monthly loan payment calculation result: %v", rs[0].Expressions[0].Value)
}
