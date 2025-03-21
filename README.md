# RegoBrick

RegoBrick provides a straightforward way to parse and transform Rego modules **without** modifying the OPA engine. It applies certain transformations based on special import markers (for example, `import data.regobrick.default_false`) and also offers convenient helpers for custom builtins and Go↔Rego value conversion.

## Overview

- **Default False**  
  If your Rego module imports `data.regobrick.default_false`, RegoBrick will automatically insert a `default` rule that evaluates to `false` for any “if” or boolean rules. This helps ensure you don’t forget to explicitly set them to `false` when not satisfied.

- **Custom Builtins**  
  Easily register builtins with typed arguments and return values. RegoBrick converts Rego AST terms to Go types and back, so you can write builtins in Go with minimal boilerplate.

- **Parse & Transform**  
  RegoBrick transforms Rego modules by parsing their AST and applying transformations based on special import markers. It integrates seamlessly with OPA's evaluation pipeline without modifying the OPA engine, simplifying the extension of policy logic through modular AST transformations.


## Installation

```bash
go get github.com/sky1core/regobrick
```

Make sure you also have OPA in your `go.mod` if you plan to work with the Rego engine.

## Usage

Below is an example of how to use RegoBrick with decimal input data.  
We create a decimal value from a string to avoid floating-point precision issues, then pass it to RegoBrick as input.

By including `import data.regobrick.default_false` in your policy, RegoBrick automatically inserts a default rule (for example, `default allow = false`), ensuring that if the condition isn’t met, the rule defaults to `false`.

```go
package main

import (
    "context"
    "fmt"

    "github.com/shopspring/decimal"
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
        panic(err)
    }

    // Create a decimal value from string to avoid floating-point issues.
    rawDec, err := decimal.NewFromString("123.45")
    if err != nil {
        panic(err)
    }

    // Convert the decimal to a RegoDecimal so it’s handled as a numeric literal.
    amount := regobrick.NewRegoDecimal(rawDec)

    // Build the input map, including our RegoDecimal.
    input := map[string]interface{}{
        "user":   "admin",
        "amount": amount,
    }

    // Evaluate using rego.EvalInput to pass input.
    rs, err := query.Eval(ctx, rego.EvalInput(input))
    if err != nil {
        panic(err)
    }

    // The result of 'data.example.allow' is in rs.
    // Because 'allow if ...' is accompanied by 'default allow = false',
    // if the condition is not met, it defaults to false.
    fmt.Println("Result:", rs)
}
```

## Writing Custom Builtins

You can register a custom function that OPA calls within your policies. RegoBrick provides helper functions (like `RegisterBuiltin1`, `RegisterBuiltin2`, etc.) for builtins that accept typed Go arguments and return typed Go values.

```go
package main

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
    // RegisterBuiltin1[T1, R] has this signature:
    //   func RegisterBuiltin1[T1 any, R any](
    //       name string,
    //       fn func(rego.BuiltinContext, T1) (R, error),
    //       opts ...BuiltinRegisterOption,
    //   )
    //
    // So we pass:
    //   1) The builtin name ("is_admin")
    //   2) Our Go function (isAdmin)
    //   3) Any number of BuiltinRegisterOption values, such as categories or nondeterminism.

    regobrick.RegisterBuiltin1[string, bool](
        "is_admin",
        isAdmin,
        // We can set the categories (used in FilterCapabilities) or nondeterministic flag, etc.
        regobrick.WithCategories("my_custom_category"),
        // regobrick.WithNondeterministic() // If your builtin is nondeterministic
    )

    // Then in Rego, you can write:
    //    is_admin(input.user) => returns true if user == "admin"

    // ...
}
```

RegoBrick automatically converts the Rego argument to a Go string and converts the returned bool back to a Rego boolean. For more complex use cases, you can define builtins with multiple arguments, different Go types (e.g., `[]string`, custom structs), and so on.

## Filtering Builtins with `FilterCapabilities`

If you want to restrict which builtins are allowed when evaluating a policy, you can use the `FilterCapabilities` function to include or exclude builtins by name and category. This gives you a way to lock down OPA so that only certain operations are permitted.

```go
package main

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
