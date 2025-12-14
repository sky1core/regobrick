# RegoBrick

RegoBrick provides a straightforward way to parse and transform Rego modules **without** modifying the OPA engine. It applies certain transformations based on special import markers (for example, `import data.regobrick.default_false`) and also offers convenient helpers for custom builtins and Go↔Rego value conversion.

## Number Type

`regobrick.Number` is an alias for `json.Number`, used to pass numeric values to Rego without floating-point precision loss.

```go
input := map[string]any{
    "price":    regobrick.Number("123.45"),
    "quantity": regobrick.Number("10"),
}
```

**Contract:**
- Exponent notation (`1e-8`, `2.5E10`) is **not supported**
- If exponent notation is used with `RegisterOperatorOverloads()`:
  - Default mode: operation silently fails (rule not satisfied, no result)
  - `StrictBuiltinErrors(true)`: returns `eval_builtin_error`
- Input validation is the caller's responsibility

**Precision Limits (udecimal):**
- Maximum **19 decimal places**
- Range: ±34,028,236,692,093,846,346.3374607431768211455
- Exceeding 19 decimal places results in **truncation** (not rounding)
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

        some_rule {
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

RegoBrick provides operator overloading for precision arithmetic using [udecimal](https://github.com/quagmt/udecimal) internally. Call `RegisterOperatorOverloads()` once at startup to replace Rego's default float-based operators.

```go
func init() {
    regobrick.RegisterOperatorOverloads()
}
```

This overloads:
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- Comparison: `>`, `>=`, `<`, `<=`, `==`, `!=`
- Unary: `abs()`, `round()`, `ceil()`, `floor()`

Notes:
- On error (e.g., divide by zero, invalid number format):
  - Default mode: operation silently fails (rule not satisfied)
  - `StrictBuiltinErrors(true)`: returns `eval_builtin_error`

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
