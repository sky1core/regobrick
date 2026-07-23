// Package standard benchmarks policy evaluation with stock OPA builtins.
// It must never call UseDecimalArithmetic (directly or transitively): it is the
// unmodified baseline that internal/bench/decimal is compared against.
package standard

import (
	"testing"

	"github.com/sky1core/regobrick/internal/bench/benchcommon"
)

func BenchmarkArithSum1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.ArithSumPolicy, benchcommon.Inputs(1000))
}

func BenchmarkCompareFilter1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.CompareFilterPolicy, benchcommon.Inputs(1000))
}

func BenchmarkAggregates1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.AggregatesPolicy, benchcommon.Inputs(1000))
}

func BenchmarkDivSum1000(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.DivSumPolicy, benchcommon.Inputs(1000))
}

func BenchmarkScalarGuard(b *testing.B) {
	benchcommon.RunPolicyBenchmark(b, benchcommon.ScalarGuardPolicy, benchcommon.Inputs(1))
}
