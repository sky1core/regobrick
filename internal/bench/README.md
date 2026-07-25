# Benchmarks

End-to-end benchmarks comparing policy evaluation cost across three modes:

| Package | Mode |
|---|---|
| `internal/bench/standard` | Standard OPA (no regobrick) |
| `internal/bench/decimal` | `UseDecimalArithmetic()`, numeric inputs |
| `internal/bench/coercion` | `UseDecimalArithmetic(WithStringCoercion())`, string inputs |

The three modes live in separate test binaries because `UseDecimalArithmetic`
mutates OPA's process-global builtin registry and cannot be undone within a
process. All packages share the same policies, inputs, and benchmark names
(defined in `benchcommon`), so results can be compared directly with
`benchstat`.

Micro-benchmarks for the operator hot path (parse, expand, individual
operations) live in the repository root (`operator_bench_test.go`).

## Scenarios

Each benchmark prepares the query once and measures repeated `Eval` calls.
Every iteration asserts a defined result, so a silently failing evaluation
cannot be measured by mistake.

| Benchmark | Policy shape |
|---|---|
| `ArithSum1000` | `sum([v \| v := xs[i] * ys[i]])` over 1000-element arrays |
| `CompareFilter1000` | `count([1 \| xs[i] > threshold])` over 1000 elements |
| `Aggregates1000` | `[sum(xs), min(xs), max(xs)]` over 1000 elements |
| `DivSum1000` | `sum([v \| v := xs[i] / ys[i]])` over 1000 elements |
| `ScalarGuard` | `price * qty > limit` (single comparison, evaluator-overhead bound) |

Input values have at most 6 fractional digits — well within udecimal's
19-decimal-place limit — so both modes evaluate the same policies over the same
inputs to a defined result. The numeric *results* may differ in low-order
digits (big.Float vs exact decimal); the benchmark compares evaluation cost,
not results.

## Environment

Measured 2026-07-25:

| | |
|---|---|
| CPU / OS | Apple M1 Max, darwin/arm64 |
| Go | go1.26.5 |
| OPA | v1.11.0 |
| udecimal | v1.9.0 |
| Method | `go test -bench . -count 6 -benchtime 1s`, medians via `benchstat` |

All deltas below are statistically significant (n=6; p=0.002 unless a
different p-value is noted inline); rows marked `~` show no significant
difference. **Absolute numbers are machine- and version-specific; the relative
deltas are the meaningful signal.** Re-run on your target hardware before
relying on them.

## Standard OPA vs `UseDecimalArithmetic()`

Time per evaluation (`sec/op`):

| Benchmark | Standard OPA | Decimal | Δ time | Speedup |
|---|---|---|---|---|
| ArithSum1000 | 4.804 ms ± 13% | 2.709 ms ± 9% | −43.6% | 1.8× |
| CompareFilter1000 | 3.316 ms ± 7% | 1.582 ms ± 5% | −52.3% | 2.1× |
| Aggregates1000 | 4.485 ms ± 2% | 0.826 ms ± 7% | −81.6% | 5.4× |
| DivSum1000 | 6.573 ms ± 5% | 2.831 ms ± 6% | −56.9% | 2.3× |
| ScalarGuard | 20.64 µs ± 6% | 14.83 µs ± 12% | −28.1% | 1.4× |
| **geomean** | 1.575 ms | 682.9 µs | **−56.6%** | **2.3×** |

Heap allocations (`allocs/op`; bytes allocated shrink similarly, geomean
−63.1%):

| Benchmark | Standard OPA | Decimal | Δ allocs |
|---|---|---|---|
| ArithSum1000 | 65.7k | 19.7k | −70.0% |
| CompareFilter1000 | 42.2k | 15.2k | −64.0% |
| Aggregates1000 | 62.2k | 6.2k | −90.0% |
| DivSum1000 | 63.7k | 19.7k | −69.1% |
| ScalarGuard | 228 | 168 | −26.3% |

### Why decimal mode is faster

Standard OPA does not use hardware floats for arithmetic: to preserve JSON
number semantics it computes **non-integer** arithmetic on heap-allocated
arbitrary-precision `big.Float` values (`topdown/arithmetic.go`; small
integers take a native fast path), allocating on every parse, operation, and
result. udecimal's `Decimal` is a fixed-width value type (uint128 coefficient +
scale) that parses and computes on the stack — plain-notation parsing is
zero-allocation. Values beyond the u128 coefficient range fall back to exact
`big.Int` arithmetic; all benchmark inputs here stay on the fast path.
Replacing arbitrary-precision software arithmetic with fixed-width software
arithmetic is what yields both the precision contract and the speedup.

The gain scales with how numeric-heavy the policy is. `ScalarGuard` (one
multiply and one compare per evaluation) improves only 1.4× because evaluator
overhead, not arithmetic, dominates its runtime.

## String coercion overhead

`decimal` (numeric inputs) vs `coercion` (the same values as JSON strings,
converted on the per-element hot path by `WithStringCoercion()`):

| Benchmark | Decimal, numeric input | +Coercion, string input | Δ time |
|---|---|---|---|
| ArithSum1000 | 2.709 ms | 2.664 ms | −1.7% (p=0.015) |
| CompareFilter1000 | 1.582 ms | 1.741 ms | ~ (p=0.310) |
| Aggregates1000 | 825.5 µs | 968.8 µs | +17.4% |
| DivSum1000 | 2.831 ms | 2.887 ms | ~ (p=0.937) |
| ScalarGuard | 14.83 µs | 15.53 µs | ~ (p=0.093) |
| **geomean** | 682.9 µs | 725.9 µs | **+6.3%** |

Coercion is within measurement noise for most scenarios. The one significant
regression is `Aggregates1000` (+17.4%): `max`/`min` first scan the collection
to decide whether every element is numeric-like, then parse the elements again
for comparison, so string elements are parsed twice.

## Operator micro-benchmarks

Direct builtin-function cost without rego evaluation overhead
(`operator_bench_test.go`, same environment):

| Benchmark | ns/op | allocs/op |
|---|---|---|
| ExpandExponent_Plain | 27.6 | 0 |
| ExpandExponent_Exponent | 101.9 | 1 |
| ParseDecimal_Plain | 32.9 | 0 |
| ParseDecimal_Exponent | 128.8 | 1 |
| PrecisionPlus | 225.0 | 3 |
| PrecisionMultiply | 235.7 | 3 |
| PrecisionDivide | 258.2 | 3 |
| PrecisionLT | 158.7 | 1 |

## Reproducing

`benchstat` groups rows by package, so strip the differing `pkg:` lines before
comparing across packages:

```bash
go install golang.org/x/perf/cmd/benchstat@latest

go test -bench . -count 6 ./internal/bench/standard | grep -v '^pkg:' > standard.txt
go test -bench . -count 6 ./internal/bench/decimal  | grep -v '^pkg:' > decimal.txt
go test -bench . -count 6 ./internal/bench/coercion | grep -v '^pkg:' > coercion.txt

benchstat standard.txt decimal.txt   # standard OPA vs decimal mode
benchstat decimal.txt coercion.txt   # coercion overhead

go test -bench . -run '^$' -count 6 .   # operator micro-benchmarks
```

CI runs these benchmarks in compile-and-run mode only (single iteration, no
timing); real measurements are taken manually as above.
