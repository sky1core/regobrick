# RegoBrick

RegoBrick provides a simple way to parse and transform Rego modules **without** modifying the OPA engine.  
It applies certain transformations based on special import markers, such as `import data.regobrick.default_false`.

---

## Overview

- **Default False**: If your Rego module imports `data.regobrick.default_false`, RegoBrick will automatically insert a `default` rule that evaluates to `false` for any “if” or boolean rules.  
- Additional features may be introduced in the future.

---

## Installation

```bash
go get github.com/sky1core/regobrick
```

Ensure you have OPA as well, and that your `go.mod` is properly set up.

---

## Usage

In your Go code, use RegoBrick’s module-transform function before running queries with OPA:

```go
import (
    "github.com/open-policy-agent/opa/v1/rego"
    "github.com/sky1core/regobrick"
)

func main() {
    // Your Rego source
    code := `
        package example

        import data.regobrick.default_false

        allow if {
            input.user == "admin"
        }
    `

    // Apply transformations (e.g. default_false) via RegoBrick,
    // then pass the resulting module to OPA.
    query, err := rego.New(
        regobrick.Module("example.rego", code),
        rego.Query("data.example.allow"),
    ).PrepareForEval(ctx)
    
    // ...
}
```

- If `import data.regobrick.default_false` is present, RegoBrick injects a `default` rule that sets the `if` or boolean rule to `false` if conditions aren’t met.
