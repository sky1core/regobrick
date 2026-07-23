// Package decimal benchmarks policy evaluation with UseDecimalArithmetic
// enabled. Benchmark names mirror internal/bench/standard one-to-one so the two
// suites can be compared with benchstat.
package decimal

import (
	"os"
	"testing"

	"github.com/sky1core/regobrick"
	"github.com/sky1core/regobrick/internal/bench/benchcommon"
)

func TestMain(m *testing.M) {
	regobrick.UseDecimalArithmetic()
	os.Exit(m.Run())
}

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
