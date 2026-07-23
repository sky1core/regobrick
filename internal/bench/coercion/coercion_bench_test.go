// Package coercion benchmarks policy evaluation with
// UseDecimalArithmetic(WithStringCoercion()) over string-typed numeric inputs.
// Benchmark names mirror internal/bench/standard and internal/bench/decimal so
// the three suites can be compared with benchstat; note that this suite
// measures a different workload by design (numbers arrive as JSON strings and
// go through the coercion path).
package coercion

import (
	"os"
	"testing"

	"github.com/sky1core/regobrick"
	"github.com/sky1core/regobrick/internal/bench/benchcommon"
)

func TestMain(m *testing.M) {
	regobrick.UseDecimalArithmetic(regobrick.WithStringCoercion())
	os.Exit(m.Run())
}

func BenchmarkArithSum1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.ArithSumPolicy, benchcommon.StringInputs(1000))
}

func BenchmarkCompareFilter1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.CompareFilterPolicy, benchcommon.StringInputs(1000))
}

func BenchmarkAggregates1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.AggregatesPolicy, benchcommon.StringInputs(1000))
}

func BenchmarkDivSum1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.DivSumPolicy, benchcommon.StringInputs(1000))
}

func BenchmarkScalarGuard(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.ScalarGuardPolicy, benchcommon.StringInputs(1))
}
