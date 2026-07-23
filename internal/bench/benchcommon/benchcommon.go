// Package benchcommon holds the policy sources, inputs and runner shared by the
// standard-OPA and decimal-mode benchmark packages.
//
// The two modes must live in separate test binaries because
// UseDecimalArithmetic mutates OPA's process-global builtin registry and cannot
// be undone. Identical benchmark names across the two packages allow direct
// comparison, e.g.:
//
//	go test -bench . -count 10 ./internal/bench/standard > standard.txt
//	go test -bench . -count 10 ./internal/bench/decimal > decimal.txt
//	benchstat standard.txt decimal.txt
package benchcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

const ArithSumPolicy = `package bench
import rego.v1
result := sum([v | some i; v := input.xs[i] * input.ys[i]])`

const CompareFilterPolicy = `package bench
import rego.v1
result := count([1 | some i; input.xs[i] > input.threshold])`

const AggregatesPolicy = `package bench
import rego.v1
result := [sum(input.xs), min(input.xs), max(input.xs)]`

const DivSumPolicy = `package bench
import rego.v1
result := sum([v | some i; v := input.xs[i] / input.ys[i]])`

const ScalarGuardPolicy = `package bench
import rego.v1
default result := false
result if input.price * input.qty > input.limit`

// Inputs builds a deterministic input document with n-element numeric arrays.
// Values are emitted as json.Number with at most 6 fractional digits, well
// within udecimal's 19-decimal-place/range limits, so both standard OPA and
// decimal mode evaluate the same policies over the same inputs to a defined
// result — the benchmark compares evaluation cost, not numeric results, which
// may differ in low-order digits (big.Float vs exact decimal).
// ys values are always non-zero so DivSumPolicy cannot divide by zero.
func Inputs(n int) map[string]any {
	xs := make([]any, n)
	ys := make([]any, n)
	for i := 0; i < n; i++ {
		xs[i] = json.Number(fmt.Sprintf("%d.%06d", i%1000, (i*7919)%1000000))
		ys[i] = json.Number(fmt.Sprintf("%d.%06d", (i%97)+1, (i*104729)%1000000))
	}
	return map[string]any{
		"xs":        xs,
		"ys":        ys,
		"threshold": json.Number("500.5"),
		"price":     json.Number("123.45"),
		"qty":       json.Number("10.5"),
		"limit":     json.Number("1000"),
	}
}

// RunPolicyBenchmark prepares the policy once and measures repeated Eval calls.
// Every iteration asserts a defined result: an undefined result would mean the
// benchmark silently measures a failing evaluation path.
func RunPolicyBenchmark(b *testing.B, policy string, input map[string]any) {
	b.Helper()
	ctx := context.Background()
	pq, err := rego.New(
		rego.Query("data.bench.result"),
		rego.Module("bench.rego", policy),
	).PrepareForEval(ctx)
	if err != nil {
		b.Fatalf("prepare: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs, err := pq.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("eval: %v", err)
		}
		if len(rs) == 0 || len(rs[0].Expressions) == 0 {
			b.Fatal("undefined result: the benchmarked policy did not evaluate")
		}
	}
}
