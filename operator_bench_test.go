package regobrick

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Micro benchmarks for the decimal operator hot path. These measure the direct
// builtin-function cost without rego evaluation overhead. End-to-end benchmarks
// comparing standard OPA and decimal mode live in internal/bench.

func BenchmarkExpandExponent_Plain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		expandExponent("123.45")
	}
}

func BenchmarkExpandExponent_Exponent(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		expandExponent("1.2345e-8")
	}
}

func BenchmarkParseDecimal_Plain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := parseDecimal("12345.6789"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseDecimal_Exponent(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := parseDecimal("1.2345e-8"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkBinaryOp(b *testing.B, fn func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error, lhs, rhs string) {
	b.Helper()
	operands := []*ast.Term{
		ast.NumberTerm(json.Number(lhs)),
		ast.NumberTerm(json.Number(rhs)),
	}
	iter := func(*ast.Term) error { return nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := fn(topdown.BuiltinContext{}, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrecisionPlus(b *testing.B) {
	benchmarkBinaryOp(b, precisionPlus, "123.456", "789.012")
}

func BenchmarkPrecisionMultiply(b *testing.B) {
	benchmarkBinaryOp(b, precisionMultiply, "123.456", "789.012")
}

func BenchmarkPrecisionDivide(b *testing.B) {
	benchmarkBinaryOp(b, precisionDivide, "123.456", "7.89")
}

func BenchmarkPrecisionLT(b *testing.B) {
	benchmarkBinaryOp(b, precisionLT, "123.456", "789.012")
}
