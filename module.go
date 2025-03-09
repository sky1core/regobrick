// Package regobrick provides high-level functions for applying RegoBrick
// transformations and adding modules to OPA rego.Rego objects.
package regobrick

import (
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/sky1core/regobrick/internal/module"
)

// ModuleOption is an alias for module.ModuleOption, used for specifying
// Rego module parameters or configuration.
type ModuleOption = module.ModuleOption

// ParseModule parses a Rego source file into an AST module, optionally appending
// additional imports. If the module includes "import data.regobrick.default_false",
// it applies the default_false transform.
func ParseModule(filename, src string, imports []string) (*ast.Module, error) {
	return module.ParseModule(filename, src, imports)
}

// Module returns a rego.Rego option that adds a single Rego module from the given filename, source,
// and optional imports. If you need to handle parse errors directly, parse the module yourself
// (for example, with regobrick.ParseModule or ast.ParseModule) and then pass the *ast.Module
// to rego.ParsedModule(...) in your own Rego configuration.
func Module(filename, src string, imports []string) func(*rego.Rego) {
	return module.Module(filename, src, imports)
}

// Modules returns a rego.Rego option that adds multiple Rego modules in one call,
// each specified via a ModuleOption.
func Modules(opts ...ModuleOption) func(*rego.Rego) {
	return module.Modules(opts...)
}
