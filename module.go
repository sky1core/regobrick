package regobrick

import (
	"fmt"
	"github.com/open-policy-agent/opa/v1/rego"

	"github.com/open-policy-agent/opa/v1/ast"
)

// ParseModule parses and transforms the given Rego source.
//   - If parsing fails, returns an error.
//   - If an import marker like "data.regobrick.default_false" exists,
//     applies the corresponding AST transformation (e.g. addDefaultFalse).
func ParseModule(filename, input string) (*ast.Module, error) {
	mod, err := ast.ParseModule(filename, input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse module %q: %w", filename, err)
	}

	// Check if there's a marker like 'import data.regobrick.default_false
	if hasImportRegobrickFeature(mod, "default_false") {
		addDefaultFalse(mod)
	}
	return mod, nil
}

// Module returns a function conforming to `func(*rego.Rego)`.
// Internally, it tries to parse & transform the Rego code via ParseModule.
// If parsing fails, it falls back to providing the **original code** to OPA,
// so that OPA will raise a compile error at evaluation time.
//
// TODO: If you prefer to detect errors earlier, call ParseModule(...) directly
// and handle them before using Module(...). This design just defers the error
// to OPA's compile phase instead of returning it immediately.
func Module(filename, input string) func(r *rego.Rego) {
	return func(r *rego.Rego) {
		mod, err := ParseModule(filename, input)
		if err != nil {
			// Parsing/transformation failed -> fallback to the original code
			// => OPA compile/eval will throw an error at runtime
			rego.Module(filename, input)(r)
			return
		}
		if mod == nil {
			// If there's no module, do nothing
			return
		}

		// Pass the already-parsed AST to OPA
		rego.ParsedModule(mod)(r)
	}
}

// hasImportRegobrickFeature checks if the module has "import data.regobrick.<feature>".
func hasImportRegobrickFeature(mod *ast.Module, feature string) bool {
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok {
			if ref.String() == "data.regobrick."+feature {
				return true
			}
		}
	}
	return false
}

// addDefaultFalse finds "if-rules" (either Head.Key == "if" or boolean rule (nil)) and
// appends a default rule that sets them to false, if not already declared.
func addDefaultFalse(mod *ast.Module) {
	existing := make(map[string]bool)
	for _, r := range mod.Rules {
		if r.Default {
			existing[r.Head.Ref().String()] = true
		}
	}

	for _, r := range mod.Rules {

		// function rule
		if len(r.Head.Args) > 0 {
			continue
		}

		isIfRule := false
		if r.Head.Key != nil {
			if s, ok := r.Head.Key.Value.(ast.String); ok && string(s) == "if" {
				isIfRule = true
			}
		} else {
			// boolean rule => treated like an if-rule
			isIfRule = true
		}

		if isIfRule {
			refVal := r.Head.Ref()
			if refVal == nil {
				continue
			}
			refStr := refVal.String()

			if existing[refStr] {
				continue
			}

			// Create the default rule, e.g. "default allow = false"
			newHead := &ast.Head{
				Reference: refVal,
				Value:     ast.BooleanTerm(false),
			}
			newRule := &ast.Rule{
				Default: true,
				Head:    newHead,
			}
			mod.Rules = append(mod.Rules, newRule)
			existing[refStr] = true
		}
	}
}
