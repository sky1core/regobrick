# RegoBrick

RegoBrick provides a straightforward way to parse and transform Rego modules **without** modifying the OPA engine. It applies certain transformations based on special import markers (for example, `import data.regobrick.default_false`) and also offers convenient helpers for custom builtins and Go↔Rego value conversion.

---

## Overview

- **Default False**: If your Rego module imports `data.regobrick.default_false`, RegoBrick will automatically insert a `default` rule that evaluates to `false` for any “if” or boolean rules.
- **Custom Builtins**: Easily register builtins with typed arguments and return values. RegoBrick converts Rego AST terms to Go types and back, so you can write builtins in Go with minimal boilerplate.
- **Rego ↔ Go Conversion**: The `convert` package allows you to map Rego types (e.g. strings, numbers, arrays, objects) to typed Go structs, decimals, time.Time, etc., and vice versa.
- **Parse & Transform**: The `ParseModule` function reads a Rego module, looks for any RegoBrick import markers, and applies the corresponding AST transformations.

---

## Installation

```bash
go get github.com/sky1core/regobrick
```

Make sure you also have OPA in your `go.mod` if you plan to work with the Rego engine.

---

## Usage

### 1. Transforming Modules (e.g. `default_false`)

```go
import (
    "context"
    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    ctx := context.Background()
    code := `
        package example

        import data.regobrick.default_false

        allow if {
            input.user == "admin"
        }
    `

    // RegoBrick parses and transforms the module (detecting 'default_false' import).
    // This injects a 'default allow = false' rule automatically.
    query, err := rego.New(
        regobrick.Module("example.rego", code),
        rego.Query("data.example.allow"),
    ).PrepareForEval(ctx)
    if err != nil {
        panic(err)
    }

    // Evaluate as normal
    rs, err := query.Eval(ctx)
    // ...
}
```

### 2. Writing Custom Builtins

You can register a custom function that OPA calls within your policies:

```go
import (
    "context"
    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

// Example builtin that checks if a user is "admin"
func isAdmin(ctx rego.BuiltinContext, user string) (bool, error) {
    return user == "admin", nil
}

func main() {
    // Register the builtin with 1 string argument, returning bool
    regobrick.RegisterBuiltin1[string, bool]("is_admin", false, isAdmin)

    // Then in Rego, you can write:
//    is_admin(input.user)

    // ...
}
```

RegoBrick automatically converts the Rego argument to a Go string and converts the returned bool back to a Rego boolean.

### 3. Converting Rego Values ↔ Go

If you want to manually convert values, the `convert` package provides:

- **RegoToGo[T any](ast.Value)**: Convert an AST value to a typed Go value.
- **GoToRego(interface{})**: Convert a Go value to an AST term.

These functions support `bool`, `string`, numeric types, `decimal.Decimal`, `time.Time`, slices, maps, structs, and more.

```go
import (
    "fmt"
    "github.com/open-policy-agent/opa/v1/ast"
    "github.com/sky1core/regobrick/convert"
)

func convertExample() {
    // Suppose we have a Rego AST number
    regoNumber := ast.Number("42")
    goVal, err := convert.RegoToGo[int](regoNumber)
    if err != nil {
        panic(err)
    }
    fmt.Println("Converted to Go int:", goVal) // 42

    // Convert back to a Rego term
    term, err := convert.GoToRego(goVal)
    fmt.Println("Converted back to Rego term:", term)
}
```

---

With these features, you can seamlessly integrate custom transformations, builtins, and value conversion into your OPA-based workflows without forking or modifying OPA’s core engine.