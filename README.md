# RegoBrick

RegoBrick provides a straightforward way to parse and transform Rego modules **without** modifying the OPA engine. It applies certain transformations based on special import markers (for example, `import data.regobrick.default_false`) and also offers convenient helpers for custom builtins and Go↔Rego value conversion.

## Number Type

`regobrick.Number` is a numeric type based on `json.Number`, used to pass numeric values to Rego without floating-point precision loss. It supports DB operations (sql.Scanner, driver.Valuer) and JSON marshaling.

```go
input := map[string]any{
    "price":    regobrick.Number("123.45"),
    "quantity": regobrick.Number("10"),
}
```

**Contract:**
- Exponent notation (`1e-8`, `2.5E10`) is **not supported**
- If exponent notation is used with `UseDecimalArithmetic()`:
  - Default mode: operation silently fails (rule not satisfied, no result)
  - `StrictBuiltinErrors(true)`: returns `eval_builtin_error`
- Input validation is the caller's responsibility

**Precision Limits (udecimal):**
- Maximum **19 decimal places**
- Range: ±34,028,236,692,093,846,346.3374607431768211455
- Input values with more than 19 decimal places **fail to parse** (default mode: no result; `StrictBuiltinErrors(true)`: eval error) — they are *not* silently truncated
- **Truncation** (not rounding) applies only to operation *results* that exceed 19 decimal places (e.g., `100 / 3` → `33.3333333333333333333`)
- Sufficient for: BTC (8 decimals), ETH (18 decimals), fiat currencies

## Overview

- **Default False**
  If your Rego module imports `data.regobrick.default_false`, RegoBrick will automatically insert a `default` rule that evaluates to `false` for any "if" or boolean rules. This helps ensure you don't forget to explicitly set them to `false` when not satisfied.

- **Custom Builtins**
  Easily register builtins with typed arguments and return values. RegoBrick converts Rego AST terms to Go types and back, so you can write builtins in Go with minimal boilerplate.

- **Operator Overloading**
  Optionally override Rego's arithmetic and comparison operators with precision decimal operations.

## Installation

```bash
go get github.com/sky1core/regobrick
```

Make sure you also have OPA in your `go.mod` if you plan to work with the Rego engine.

## Usage

Below is an example of how to use RegoBrick with Number input data.
By including `import data.regobrick.default_false` in your policy, RegoBrick automatically inserts a default rule (for example, `default allow = false`), ensuring that if the condition isn't met, the rule defaults to `false`.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    ctx := context.Background()

    // Example policy for the "sub" package
    subPolicy := `
        package sub

        some_rule if {
            input.amount == 123.45
        }
    `
    // Example policy for the "main" package
    mainPolicy := `
        package example

        import data.regobrick.default_false

        allow if {
            input.user == "admin"
        }
    `

    // Build a rego.Rego object with your modules and input.
    query, err := rego.New(
        // Add Rego modules (which will apply "default_false" if that import is found):
        regobrick.Module("sub.rego", subPolicy, []string{"data.some.pkg"}),
        regobrick.Module("main.rego", mainPolicy, []string{"data.mycompany.util"}),

        // Specify the query we want to evaluate:
        rego.Query("data.example.allow"),

    ).PrepareForEval(ctx)

    if err != nil {
        log.Fatal(err)
    }

    // Build the input map with Number values to avoid floating-point issues.
    input := map[string]any{
        "user":   "admin",
        "amount": regobrick.Number("123.45"),
    }

    // Evaluate using rego.EvalInput to pass input.
    rs, err := query.Eval(ctx, rego.EvalInput(input))
    if err != nil {
        log.Fatal(err)
    }

    // The result of 'data.example.allow' is in rs.
    // Because 'allow if ...' is accompanied by 'default allow = false',
    // if the condition is not met, it defaults to false.
    fmt.Println("Result:", rs)
}
```

## Precision Arithmetic

RegoBrick provides operator overloading for precision arithmetic using [udecimal](https://github.com/quagmt/udecimal) internally. Call `UseDecimalArithmetic()` once at startup to replace Rego's default float-based operators.

```go
func init() {
    regobrick.UseDecimalArithmetic()
}
```

This overloads:
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- Comparison: `>`, `>=`, `<`, `<=`, `==`, `!=`
- Unary: `abs()`, `round()`, `ceil()`, `floor()`
- Aggregates: `sum()`, `product()`, `max()`, `min()`

Notes:
- On error (e.g., divide by zero, invalid number format):
  - Default mode: operation silently fails (rule not satisfied)
  - `StrictBuiltinErrors(true)`: returns `eval_builtin_error`
- `%` (modulo) supports floating-point operands (standard OPA allows integers only)
- Decimal arithmetic configuration is **process-global**; call `UseDecimalArithmetic(...)` **once at application startup, before any evaluation begins**
- It mutates process-global state without synchronization, so it is **not safe to call concurrently with evaluations**

### String Coercion (opt-in)

Use `WithStringCoercion()` to enable automatic string-to-number conversion. Numeric strings (e.g., `"0.73"`, `"100"`) from `input` or `data` are automatically converted to numbers in arithmetic, comparison, unary, and aggregate operations. This is useful when external systems pass decimal values as JSON strings to preserve precision.

```go
func init() {
    regobrick.UseDecimalArithmetic(regobrick.WithStringCoercion())
}
```

- **Applied to:** `+`, `-`, `*`, `/`, `%`, `>`, `>=`, `<`, `<=`, `abs`, `round`, `ceil`, `floor`, `sum`, `product`, `max`, `min`
- **Not applied to:** `==`, `!=` (different types are always unequal, matching standard OPA behavior)
- Non-numeric strings (e.g., `"abc"`) result in undefined / eval error

```rego
# input: {"qty": "0.73", "pos": 0.5}
remaining := input.qty - input.pos    # 0.23 — string "0.73" auto-converted
can_trade := input.qty > 0            # true
rounded := round(input.qty)           # 1
```

> **Note:** String coercion is primarily for runtime values from `input`/`data`. Arithmetic (`+`, `-`, ...), unary (`abs`, ...), and the `sum`/`product` aggregates declare numeric operand types, so string literals written directly in Rego source (e.g., `"0.73" + 1`, `sum(["0.1"])`) are rejected by OPA's compile-time type checker before runtime coercion can run. This does **not** apply to `max`/`min`, whose operand is an `Any` collection: string literals pass the type checker and reach runtime, where non-numeric (or mixed) collections fall back to the default comparison ordering.

### Comparison with Standard OPA

Below, **Decimal** = `UseDecimalArithmetic()`, **+Coercion** = `UseDecimalArithmetic(WithStringCoercion())`.

**Arithmetic** (number-only — same with or without `WithStringCoercion`):

| Expression | Decimal / +Coercion | Standard OPA (big.Float) |
|---|---|---|
| `1.1 + 2.2` | `3.3` | `3.3000000000000000002` |
| `0.3 - 0.1` | `0.2` | `0.20000000000000000002` |
| `100.25 * 0.03` | `3.0075` | `3.0074999999999998` |
| `100 / 3` | `33.3333333333333333333` (19 dp) | `33.333333333333333332` (20 dp) |
| `10 % 3` | `1` | `1` |
| `10.5 % 3` | `1.5` | error (integers only) |
| `{1,2,3} - {2}` | `{1,3}` (set diff) | `{1,3}` (set diff) |

**Comparison** (number-only — same with or without `WithStringCoercion`):

| Expression | Decimal / +Coercion | Standard OPA |
|---|---|---|
| `0.3 - 0.1 == 0.2` | `true` | `false` |
| `1.1 + 2.2 == 3.3` | `true` | `false` |
| `3.3 > 2.2` | `true` | `true` |
| `3.3 >= 3.3` | `true` | `true` |
| `2.2 < 3.3` | `true` | `true` |
| `2.2 <= 3.3` | `true` | `true` |
| `"a" < "b"` | undefined | `true` (type ordering) |
| `"hello" > 123` | undefined | `true` (type ordering) |

**Unary** (number-only — same with or without `WithStringCoercion`):

| Expression | Decimal / +Coercion | Standard OPA |
|---|---|---|
| `abs(-3.3)` | `3.3` | `3.3` |
| `round(2.5)` | `3` (half away from zero) | `3` |
| `round(-2.5)` | `-3` (half away from zero) | `-3` |
| `ceil(3.1)` | `4` | `4` |
| `floor(3.9)` | `3` | `3` |

**Aggregates** (number-only — same with or without `WithStringCoercion`):

| Expression | Decimal / +Coercion | Standard OPA |
|---|---|---|
| `sum([0.1, 0.2, 0.3])` | `0.6` | `0.6000000000000000003` |
| `sum([])` | `0` | `0` |
| `product([0.1, 0.2, 0.3])` | `0.006` | `0.006000000000000001` |
| `product([])` | `1` | `1` |
| `max([0.1, 0.11, 0.09])` | `0.11` | `0.11` |
| `min([0.1, 0.11, 0.09])` | `0.09` | `0.09` |
| `max(["b", "a", "c"])` | `"c"` | `"c"` |
| `min(["b", "a", "c"])` | `"a"` | `"a"` |

**String coercion** — values from `input`/`data`, only with `WithStringCoercion()`:

| Expression | Decimal | +Coercion | Standard OPA |
|---|---|---|---|
| `input.s + 1` (`{"s":"0.73"}`) | undefined | `1.73` | undefined |
| `input.s - 0.5` (`{"s":"0.73"}`) | undefined | `0.23` | undefined |
| `input.a * input.b` (`{"a":"5.5","b":"2"}`) | undefined | `11` | undefined |
| `input.s / 4` (`{"s":"10"}`) | undefined | `2.5` | undefined |
| `input.s % 3` (`{"s":"10"}`) | undefined | `1` | undefined |
| `input.s > 0.5` (`{"s":"0.73"}`) | undefined | `true` (numeric) | `true` (type ordering) |
| `input.s < 1` (`{"s":"0.73"}`) | undefined | `true` (numeric) | `false` (type ordering) |
| `input.s >= 3.3` (`{"s":"3.3"}`) | undefined | `true` (numeric) | `false` (type ordering) |
| `input.s <= 3.3` (`{"s":"2.2"}`) | undefined | `true` (numeric) | `true` (type ordering) |
| `input.s == 3.3` (`{"s":"3.3"}`) | `false` | `false` (not coerced) | `false` |
| `input.s != 3.3` (`{"s":"3.3"}`) | `true` | `true` (not coerced) | `true` |
| `abs(input.s)` (`{"s":"-3.3"}`) | undefined | `3.3` | undefined |
| `round(input.s)` (`{"s":"3.5"}`) | undefined | `4` | undefined |
| `ceil(input.s)` (`{"s":"3.1"}`) | undefined | `4` | undefined |
| `floor(input.s)` (`{"s":"3.9"}`) | undefined | `3` | undefined |
| `sum(input.arr)` (`{"arr":["0.1","0.2"]}`) | undefined | `0.3` | undefined |
| `product(input.arr)` (`{"arr":["2","3"]}`) | undefined | `6` | undefined |
| `max(input.arr)` (`{"arr":["1","10","2"]}`) | `"2"` (lexicographic) | `"10"` (numeric) | `"2"` (lexicographic) |
| `min(input.arr)` (`{"arr":["1","10","2"]}`) | `"1"` (lexicographic) | `"1"` (numeric) | `"1"` (lexicographic) |
| `input.s + 1` (`{"s":"abc"}`) | undefined | undefined | undefined |

> **Note:** Standard OPA's comparison operators (`>`, `<`, `>=`, `<=`) support all types using type ordering (`null < bool < number < string < ...`). With `UseDecimalArithmetic`, comparison operators become **numeric-only** — non-number comparisons like `"a" < "b"` or `"hello" > 123` result in undefined. With `WithStringCoercion()`, numeric strings are additionally accepted as numbers.

## Writing Custom Builtins

You can register a custom function that OPA calls within your policies. RegoBrick provides helper functions (like `RegisterBuiltin1`, `RegisterBuiltin2`, etc.) for builtins that accept typed Go arguments and return typed Go values.

```go
package main

import (
    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

// Example builtin that checks if a user is "admin"
func isAdmin(ctx rego.BuiltinContext, user string) (bool, error) {
    return user == "admin", nil
}

func init() {
    regobrick.RegisterBuiltin1[string, bool](
        "is_admin",
        isAdmin,
        regobrick.WithCategories("my_custom_category"),
    )
}
```

### Memoization with WithMemoize

For expensive computations, use `WithMemoize()` to cache results for the same arguments within a single evaluation:

```go
regobrick.RegisterBuiltin1[string, int](
    "expensive_lookup",
    expensiveLookup,
    regobrick.WithMemoize(),
)
```

### Advanced Options with ConfigureFunction

For advanced use cases not covered by built-in options, use `ConfigureFunction` to directly configure the underlying `rego.Function`:

```go
regobrick.RegisterBuiltin1[string, int](
    "custom_func",
    customFunc,
    regobrick.ConfigureFunction(func(f *rego.Function) {
        f.Memoize = true
        f.Nondeterministic = true
    }),
)
```

## Filtering Builtins with `FilterCapabilities`

If you want to restrict which builtins are allowed when evaluating a policy, you can use the `FilterCapabilities` function to include or exclude builtins by name and category.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    allowedNames := []string{"is_admin", "concat"}
    allowedCats := []string{"my_custom_category", "strings"}

    caps := regobrick.FilterCapabilities(allowedNames, allowedCats)

    ctx := context.Background()
    query, err := rego.New(
        rego.Query("data.example.allow"),
        rego.Capabilities(caps),
    ).PrepareForEval(ctx)
    if err != nil {
        log.Fatal(err)
    }

    rs, err := query.Eval(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Query result:", rs)
}
```
