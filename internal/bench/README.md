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

Measured 2026-07-25, re-measured the same day after the single-pass `max`/`min`
scan. All four runs below were taken back to back in one session; an earlier
session that day measured ~28% slower on identical unmodified code, so figures
from different sessions must never be compared against each other:

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
| ArithSum1000 | 4.102 ms ± 1% | 1.813 ms ± 1% | −55.8% | 2.3× |
| CompareFilter1000 | 2.395 ms ± 0% | 1.035 ms ± 1% | −56.8% | 2.3× |
| Aggregates1000 | 3.010 ms ± 0% | 528.9 µs ± 1% | −82.4% | 5.7× |
| DivSum1000 | 4.370 ms ± 0% | 1.880 ms ± 1% | −57.0% | 2.3× |
| ScalarGuard | 14.59 µs ± 1% | 11.62 µs ± 2% | −20.3% | 1.26× |
| **geomean** | 1.135 ms | 464.8 µs | **−59.1%** | **2.4×** |

Heap allocations (`allocs/op`; bytes allocated shrink similarly, geomean
−63.4%):

| Benchmark | Standard OPA | Decimal | Δ allocs |
|---|---|---|---|
| ArithSum1000 | 65.7k | 19.7k | −70.0% |
| CompareFilter1000 | 42.2k | 14.2k | −66.4% |
| Aggregates1000 | 62.2k | 6.2k | −90.0% |
| DivSum1000 | 63.7k | 19.7k | −69.1% |
| ScalarGuard | 228 | 167 | −26.8% |

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

Two operator-level details shape the allocation columns. Comparison operators
return one of OPA's two interned boolean terms, so producing a comparison
result allocates nothing. `max`/`min` over a numeric collection parse each
element exactly once in a single scan, and for an array operand that scan runs
on stack-resident state, so it allocates nothing per element either. (A
collection that is not numeric-only is re-scanned under the default ordering,
which is the slower path.)

The gain scales with how numeric-heavy the policy is. `ScalarGuard` (one
multiply and one compare per evaluation) improves only 1.26× because evaluator
overhead, not arithmetic, dominates its runtime.

## String coercion overhead

`decimal` (numeric inputs) vs `coercion` (the same values as JSON strings,
converted on the per-element hot path by `WithStringCoercion()`):

| Benchmark | Decimal, numeric input | +Coercion, string input | Δ time |
|---|---|---|---|
| ArithSum1000 | 1.813 ms | 1.779 ms | −1.9% |
| CompareFilter1000 | 1.035 ms | 1.004 ms | −3.0% |
| Aggregates1000 | 528.9 µs | 487.3 µs | −7.9% |
| DivSum1000 | 1.880 ms | 1.850 ms | −1.6% |
| ScalarGuard | 11.62 µs | 11.49 µs | −1.2% (p=0.026) |
| **geomean** | 464.8 µs | 450.2 µs | **−3.1%** |

Allocation counts are identical between the two modes; string inputs allocate
0.2–1.7% more bytes.

No scenario shows coercion overhead any more. Read the negative deltas as "no
measurable cost", not as a speedup from coercion: the two runs evaluate
different input documents (string terms vs number terms), so their remaining
gap is not attributable to the coercion path.

`Aggregates1000` used to be the one scenario with a real coercion penalty
(+17.4%), because `max`/`min` first scanned the collection to decide whether
every element was numeric-like and then parsed the elements again to compare
them — parsing every string twice. The scan now parses each element once and
falls back to `ast.Compare` ordering only after meeting an element that is not
numeric-like, which removed the penalty.

## Operator micro-benchmarks

Direct builtin-function cost without rego evaluation overhead
(`operator_bench_test.go`, same environment):

| Benchmark | ns/op | allocs/op |
|---|---|---|
| ExpandExponent_Plain | 22.7 | 0 |
| ExpandExponent_Exponent | 77.6 | 1 |
| ParseDecimal_Plain | 24.0 | 0 |
| ParseDecimal_Exponent | 95.5 | 1 |
| PrecisionPlus | 159.7 | 3 |
| PrecisionMultiply | 168.8 | 3 |
| PrecisionDivide | 184.2 | 3 |
| PrecisionLT | 92.7 | 0 |

`PrecisionLT` allocates nothing: the comparison result is an interned boolean
term, and both operands parse without allocating. The arithmetic operators
allocate for their result number instead.

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
