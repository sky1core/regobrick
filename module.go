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
// it applies the default_false transform. METADATA annotations are preserved.
//
// ParseModule never panics; it returns an error for parse failures, invalid
// injected import paths, import name conflicts, or unknown regobrick features.
func ParseModule(filename, src string, imports []string) (*ast.Module, error) {
	return module.ParseModule(filename, src, imports)
}

// Module returns a rego.Rego option that adds a single Rego module from the given
// filename, source, and optional imports.
//
// Fail-fast contract: if the module cannot be processed, Module panics with a
// message like `regobrick: cannot process module "<filename>": <cause>` UNLESS the
// caller requested nothing regobrick-specific (imports is empty AND the source does
// not reference any "data.regobrick." feature). In that case Module falls back to
// rego.Module(filename, src), preserving plain (including v0) workflows. If you want
// to handle errors yourself instead of risking a panic, call ParseModule and pass
// the resulting *ast.Module to rego.ParsedModule(...).
func Module(filename, src string, imports []string) func(*rego.Rego) {
	return module.Module(filename, src, imports)
}

// Modules returns a rego.Rego option that adds multiple Rego modules in one call,
// each specified via a ModuleOption. It follows the same fail-fast contract as
// Module: a module that requested regobrick behavior but failed to parse causes a
// panic, while a fully plain module falls back to rego.Module.
func Modules(opts ...ModuleOption) func(*rego.Rego) {
	return module.Modules(opts...)
}
