# RegoBrick

RegoBrick provides a straightforward way to parse and transform Rego modules **without** modifying the OPA engine. It applies certain transformations based on special import markers (for example, `import data.regobrick.default_false`) and also offers convenient helpers for custom builtins and Go↔Rego value conversion.

---

## Overview

- **Default False**  
  If your Rego module imports `data.regobrick.default_false`, RegoBrick will automatically insert a `default` rule that evaluates to `false` for any “if” or boolean rules. This helps ensure you don’t forget to explicitly set them to `false` when not satisfied.

- **Custom Builtins**  
  Easily register builtins with typed arguments and return values. RegoBrick converts Rego AST terms to Go types and back, so you can write builtins in Go with minimal boilerplate.

- **Rego ↔ Go Conversion**  
  The `convert` package allows you to map Rego types (e.g. strings, numbers, arrays, objects) to typed Go structs, decimals, `time.Time`, etc., and vice versa.

- **Parse & Transform**  
  The `ParseModule` function (and other internal logic) reads a Rego module, looks for any RegoBrick import markers, and applies the corresponding AST transformations. A higher-level function, `regobrick.New`, provides a convenient way to load multiple modules, apply transforms, and inject them into OPA.

---

## Installation

```bash
go get github.com/sky1core/regobrick
```

Make sure you also have OPA in your `go.mod` if you plan to work with the Rego engine.

---

## Usage

### 1. Transforming Modules (e.g. `default_false`)

Below is an example of how to apply RegoBrick’s transformations (such as `default_false`) by creating a RegoBrick instance and then passing it to OPA’s `rego.New`. This approach allows you to specify multiple Rego modules—some with RegoBrick features and some without—and optionally add extra imports to each module:

```go
import (
    "context"
    "log/slog"

    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    ctx := context.Background()

    // Example policy for the "sub" package (no special RegoBrick feature here).
    subPolicy := `
        package sub

        some_rule {
            input.value == 123
        }
    `
    // Example policy for the "main" package, which includes the 'default_false' feature.
    mainPolicy := `
        package example

        import data.regobrick.default_false

        allow if {
            input.user == "admin"
        }
    `

    // Create a RegoBrick with the modules. The third argument to WithModule is a list
    // of imports you want to add to that module's AST (in addition to what's in the source).
    brick, err := regobrick.New(
        regobrick.WithModule("sub.rego", subPolicy, []string{"data.some.pkg"}),    // Additional import
        regobrick.WithModule("main.rego", mainPolicy, []string{"data.mycompany.util"}),
    )
    if err != nil {
        slog.Error("Failed to create rego brick: ", "error", err)
        return
    }

    // Build a Rego query using the RegoBrick instance. RegoBrick will detect the
    // 'import data.regobrick.default_false' marker in mainPolicy, and automatically
    // insert a rule like 'default allow = false'.
    query, err := rego.New(
        brick,
        rego.Query("data.example.allow"),
    ).PrepareForEval(ctx)
    if err != nil {
        panic(err)
    }

    // Evaluate as normal
    rs, err := query.Eval(ctx, rego.EvalInput(map[string]interface{}{
        "user":  "admin",
        "value": 123,
    }))
    if err != nil {
        panic(err)
    }

    // The result contains the evaluated value of 'data.example.allow'.
    // Because of 'default_false', if 'allow if ...' isn't satisfied, it defaults to false.
    // ...
}
```

By including `import data.regobrick.default_false` in your policy (`mainPolicy` above), RegoBrick automatically inserts a default rule (for example, `default allow = false`) to ensure that if no other conditions are met, `allow` is set to `false`.

---

### 2. Writing Custom Builtins

You can register a custom function that OPA calls within your policies. RegoBrick provides helper functions (like `RegisterBuiltin1`, `RegisterBuiltin2`, etc.) for builtins that accept typed Go arguments and return typed Go values.

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
    // Register the builtin with 1 string argument, returning bool.
    // The second argument is the categories for this builtin (can be used in FilterCapabilities).
    // The third argument is whether it's nondeterministic (false here).
    regobrick.RegisterBuiltin1[string, bool]("is_admin", []string{"my_custom_category"}, false, isAdmin)

    // Then in Rego, you can write:
    //    is_admin(input.user) => returns true if user == "admin"

    // ...
}
```

RegoBrick automatically converts the Rego argument to a Go string and converts the returned bool back to a Rego boolean. For more complex use cases, you can define builtins with multiple arguments, different Go types (e.g., `[]string`, custom structs), and so on.

---

### 3. Filtering Builtins with `FilterCapabilities`

If you want to restrict which builtins are allowed when evaluating a policy, you can use the `FilterCapabilities` function to include or exclude builtins by name and category. This gives you a way to lock down OPA so that only certain operations are permitted.

```go
import (
    "context"
    "fmt"

    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    // Suppose you want to allow only a small subset of builtins:
    // by specific names or categories.
    allowedNames := []string{"is_admin", "concat"}          // e.g., our custom builtin, plus a standard OPA builtin
    allowedCats := []string{"my_custom_category", "strings"} // categories to allow

    // FilterCapabilities returns an *ast.Capabilities object
    // that includes only the builtins matching the allowed names/infixes/categories.
    caps := regobrick.FilterCapabilities(allowedNames, allowedCats)

    // Now you can build a Rego query with these restricted capabilities:
    ctx := context.Background()
    query, err := rego.New(
        rego.Query("data.example.allow"),
        rego.Capabilities(caps),
        // Possibly other Rego options...
    ).PrepareForEval(ctx)
    if err != nil {
        panic(err)
    }

    // Evaluate the query as usual:
    rs, err := query.Eval(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Println("Query result:", rs)
}
```

In the snippet above, builtins that do not appear in either `allowedNames` or `allowedCats` (and are not in the `coreInfixes` set) will be excluded from the engine’s capabilities, resulting in errors if a policy tries to use them.

---

### 4. Converting Rego Values ↔ Go

If you want to manually convert values, the `convert` package provides:

- **RegoToGo[T any](ast.Value)**  
  Convert an AST value to a typed Go value.
- **GoToRego(interface{})**  
  Convert a Go value to an AST term.

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
    if err != nil {
        panic(err)
    }
    fmt.Println("Converted back to Rego term:", term)
}
```

---

With these features, you can seamlessly integrate custom transformations, builtins, capability filtering, and value conversion into your OPA-based workflows—without forking or modifying OPA’s core engine.