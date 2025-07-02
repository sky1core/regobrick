package regobrick

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/shopspring/decimal"
)

func BenchmarkParseModule(b *testing.B) {
	source := `package benchmark

import data.regobrick.default_false

rule1 if {
	input.value == "test"
}

rule2 if {
	input.number > 100
}

rule3 if {
	input.array[0] == "first"
}`

	imports := []string{"data.company.utils", "data.shared.helpers"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseModule("benchmark.rego", source, imports)
		if err != nil {
			b.Fatalf("ParseModule failed: %v", err)
		}
	}
}

func BenchmarkParseModuleLarge(b *testing.B) {
	// Generate a large policy with many rules
	var sb strings.Builder
	sb.WriteString("package large_benchmark\n\n")
	sb.WriteString("import data.regobrick.default_false\n\n")
	
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf(`rule_%d if {
	input.field_%d == "value_%d"
}

`, i, i, i))
	}

	source := sb.String()
	imports := []string{"data.company.utils"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseModule("large_benchmark.rego", source, imports)
		if err != nil {
			b.Fatalf("ParseModule failed: %v", err)
		}
	}
}

func BenchmarkRegoDecimalCreation(b *testing.B) {
	b.Run("NewRegoDecimal", func(b *testing.B) {
		dec := decimal.NewFromFloat(123.456)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewRegoDecimal(dec)
		}
	})

	b.Run("NewRegoDecimalFromInt", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewRegoDecimalFromInt(int64(i))
		}
	})
}

func BenchmarkRegoDecimalArithmetic(b *testing.B) {
	rd1 := NewRegoDecimal(decimal.NewFromFloat(123.456))
	rd2 := NewRegoDecimal(decimal.NewFromFloat(78.901))

	b.Run("Add", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rd1.Decimal.Add(rd2.Decimal)
		}
	})

	b.Run("Sub", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rd1.Decimal.Sub(rd2.Decimal)
		}
	})

	b.Run("Mul", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rd1.Decimal.Mul(rd2.Decimal)
		}
	})

	b.Run("Div", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rd1.Decimal.Div(rd2.Decimal)
		}
	})
}

func BenchmarkFilterCapabilities(b *testing.B) {
	allowedNames := []string{"concat", "count", "sum", "max", "min"}
	allowedCats := []string{"strings", "numbers", "arrays"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FilterCapabilities(allowedNames, allowedCats)
	}
}

func BenchmarkBuiltinRegistration(b *testing.B) {
	testFunc := func(ctx rego.BuiltinContext, input string) (bool, error) {
		return input == "test", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		funcName := fmt.Sprintf("bench_builtin_%d", i)
		RegisterBuiltin1[string, bool](funcName, testFunc)
	}
}

func BenchmarkPolicyEvaluation(b *testing.B) {
	// Benchmark the complete flow: parse + evaluate
	policy := `package benchmark

import data.regobrick.default_false

allow if {
	input.user == "admin"
	input.action == "read"
}

deny if {
	input.user == "guest"
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("benchmark.rego", policy, []string{}),
		rego.Query("data.benchmark"),
	).PrepareForEval(ctx)

	if err != nil {
		b.Fatalf("Failed to prepare query: %v", err)
	}

	input := map[string]interface{}{
		"user":   "admin",
		"action": "read",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("Eval failed: %v", err)
		}
	}
}

func BenchmarkCustomBuiltinEvaluation(b *testing.B) {
	// Register a custom builtin for benchmarking
	testBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return strings.ToUpper(input), nil
	}

	RegisterBuiltin1[string, string]("bench_upper", testBuiltin)

	policy := `package benchmark
result = bench_upper(input.text)`

	ctx := context.Background()
	query, err := rego.New(
		Module("benchmark.rego", policy, []string{}),
		rego.Query("data.benchmark.result"),
	).PrepareForEval(ctx)

	if err != nil {
		b.Fatalf("Failed to prepare query: %v", err)
	}

	input := map[string]interface{}{
		"text": "hello world",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("Eval failed: %v", err)
		}
	}
}

func BenchmarkDecimalBuiltinEvaluation(b *testing.B) {
	// Register a decimal builtin for benchmarking
	addTax := func(ctx rego.BuiltinContext, amount RegoDecimal, rate RegoDecimal) (RegoDecimal, error) {
		tax := amount.Decimal.Mul(rate.Decimal)
		return NewRegoDecimal(amount.Decimal.Add(tax)), nil
	}

	RegisterBuiltin2[RegoDecimal, RegoDecimal, RegoDecimal]("bench_add_tax", addTax)

	policy := `package benchmark
total = bench_add_tax(input.amount, input.tax_rate)`

	ctx := context.Background()
	query, err := rego.New(
		Module("benchmark.rego", policy, []string{}),
		rego.Query("data.benchmark.total"),
	).PrepareForEval(ctx)

	if err != nil {
		b.Fatalf("Failed to prepare query: %v", err)
	}

	amount := NewRegoDecimal(decimal.NewFromFloat(100.00))
	taxRate := NewRegoDecimal(decimal.NewFromFloat(0.08))

	input := map[string]interface{}{
		"amount":   amount,
		"tax_rate": taxRate,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("Eval failed: %v", err)
		}
	}
}

func BenchmarkComplexPolicyEvaluation(b *testing.B) {
	// Benchmark a complex policy with multiple rules and custom builtins
	isValidUser := func(ctx rego.BuiltinContext, userID string) (bool, error) {
		return len(userID) > 0 && userID != "invalid", nil
	}

	RegisterBuiltin1[string, bool]("bench_is_valid_user", isValidUser)

	policy := `package complex_benchmark

import data.regobrick.default_false

valid_user if {
	bench_is_valid_user(input.user_id)
	input.user_age >= 18
}

can_access_resource if {
	valid_user
	input.resource in input.allowed_resources
}

audit_required if {
	can_access_resource
	input.resource_type == "sensitive"
}

final_decision if {
	can_access_resource
	not audit_required
}

final_decision if {
	can_access_resource
	audit_required
	input.audit_approved == true
}`

	ctx := context.Background()
	query, err := rego.New(
		Module("complex_benchmark.rego", policy, []string{}),
		rego.Query("data.complex_benchmark"),
	).PrepareForEval(ctx)

	if err != nil {
		b.Fatalf("Failed to prepare complex query: %v", err)
	}

	input := map[string]interface{}{
		"user_id":           "user123",
		"user_age":          25,
		"resource":          "document1",
		"allowed_resources": []string{"document1", "document2"},
		"resource_type":     "sensitive",
		"audit_approved":    true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("Complex eval failed: %v", err)
		}
	}
}

// Memory allocation benchmarks
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("ParseModule", func(b *testing.B) {
		source := `package memory_test
import data.regobrick.default_false
rule1 if { input.test == true }`

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ParseModule("memory_test.rego", source, []string{})
			if err != nil {
				b.Fatalf("ParseModule failed: %v", err)
			}
		}
	})

	b.Run("RegoDecimal", func(b *testing.B) {
		dec := decimal.NewFromFloat(123.456)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rd := NewRegoDecimal(dec)
			_ = NewRegoDecimal(rd.Decimal.Add(NewRegoDecimalFromInt(1).Decimal))
		}
	})

	b.Run("FilterCapabilities", func(b *testing.B) {
		names := []string{"concat", "count"}
		cats := []string{"strings"}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FilterCapabilities(names, cats)
		}
	})
}
